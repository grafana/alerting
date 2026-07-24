package lokiclient

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// estimatedSize is a cheap under-estimate of the sample's encoded size, counting only the log line
// and metadata (not the timestamp or per-stream labels). It is used to pack batches; encodeAndSplit
// verifies the true encoded size and re-splits if a batch packed too much.
func (r *Sample) estimatedSize() int {
	size := len(r.V)
	for k, v := range r.Metadata {
		size += len(k) + len(v)
	}
	return size
}

// cursor points at a sample in a []Stream: streams[si].Values[vi]. The zero cursor is the first
// sample of the first stream, and si == len(streams) marks the end (no sample left).
type cursor struct {
	si, vi int
}

// window is the half-open range of samples [from, to) over a fixed []Stream. Because samples run in
// order, a window is always a contiguous run.
type window struct {
	from, to cursor
}

// batchIterator walks a read-only []Stream (never copied) and yields encoded batches that each
// satisfy len(batch) <= maxBytes. A batch is tracked as a window over the original streams, so
// slicing never copies the samples. Producing a batch has two steps:
//   - nextCandidate packs samples up to budget (maxBytes * the encoder's expected compression ratio)
//     using the cheap estimate, so the encoded batch lands near maxBytes.
//   - encodeAndSplit encodes the candidate and, if its true size still exceeds maxBytes, halves the
//     window and retries each half. A lone sample that still doesn't fit is emitted with a warning.
type batchIterator struct {
	streams  []Stream
	enc      encoder
	maxBytes int
	budget   int // uncompressed pack target: maxBytes * expected compression ratio
	logger   log.Logger

	pos     cursor   // position of the next unread sample
	pending [][]byte // fitting encoded pieces produced by a split, drained across next() calls
	cur     []byte   // batch yielded by the most recent successful next()
	failure error    // set when encoding fails; makes next() report the end of iteration
}

// newBatchIterator returns an iterator that yields encoded batches no larger than maxBytes.
func newBatchIterator(streams []Stream, enc encoder, maxBytes int, logger log.Logger) *batchIterator {
	ratio := enc.expectedCompressionRatio()
	if ratio < 1 {
		ratio = 1
	}

	it := &batchIterator{
		streams:  streams,
		enc:      enc,
		maxBytes: maxBytes,
		budget:   int(float64(maxBytes) * ratio),
		logger:   logger,
	}
	it.pos = it.skipEmpty(cursor{}) // start on the first real sample, past any leading empty streams
	return it
}

// next advances to the next fitting encoded batch and reports whether one is available. When it
// returns false, iteration is over; call err() to distinguish exhaustion from an encoding failure.
// Read the current batch via batch():
//
//	for it.next() { use(it.batch()) }
//	if it.err() != nil { ... }
func (it *batchIterator) next() bool {
	if it.failure != nil {
		return false
	}
	if len(it.pending) > 0 {
		it.cur, it.pending = it.pending[0], it.pending[1:]
		return true
	}

	candidate, ok := it.nextCandidate()
	if !ok {
		return false
	}

	pieces, err := it.encodeAndSplit(candidate)
	if err != nil {
		it.failure = err
		return false
	}
	it.cur, it.pending = pieces[0], pieces[1:]
	return true
}

// batch returns the encoded batch produced by the most recent successful next() call.
func (it *batchIterator) batch() []byte { return it.cur }

// err returns the encoding error that ended iteration, or nil if the payload was fully consumed.
func (it *batchIterator) err() error { return it.failure }

// nextCandidate greedily takes samples starting at the current position, stopping once another
// sample would push the estimated uncompressed size past budget. It always takes at least one
// sample, so an oversized sample still makes progress. ok is false once no samples remain.
func (it *batchIterator) nextCandidate() (window, bool) {
	from := it.pos
	if from.si >= len(it.streams) {
		return window{}, false
	}

	size := 0
	c := from
	for c.si < len(it.streams) {
		estimatedSize := it.streams[c.si].Values[c.vi].estimatedSize()
		// Always take the first sample (so an oversized sample still makes progress), then stop at budget.
		if c != from && size+estimatedSize > it.budget {
			break
		}
		size += estimatedSize
		c = it.step(c)
	}

	it.pos = c
	return window{from: from, to: c}, true
}

// encodeAndSplit encodes the samples in w and returns encoded pieces that each fit within maxBytes.
// The window was sized from an estimate, so if its true encoded size still exceeds maxBytes it is
// halved by sample count and each half retried. A single sample that alone exceeds the limit is
// returned as-is with a warning.
func (it *batchIterator) encodeAndSplit(w window) ([][]byte, error) {
	enc, err := it.enc.encode(it.slice(w))
	if err != nil {
		return nil, err
	}
	if len(enc) <= it.maxBytes {
		return [][]byte{enc}, nil
	}

	n := it.countSamples(w)
	if n <= 1 {
		level.Warn(it.logger).Log("msg", "Single Loki log entry exceeds the maximum write batch size, sending anyway",
			"bytes", len(enc), "maxBatchSize", it.maxBytes)
		return [][]byte{enc}, nil
	}

	mid := it.advance(w.from, n/2)
	left, err := it.encodeAndSplit(window{from: w.from, to: mid})
	if err != nil {
		return nil, err
	}
	right, err := it.encodeAndSplit(window{from: mid, to: w.to})
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

// slice materializes the samples in w as []Stream. Consecutive samples from the same source stream
// are kept together under that stream's labels, and each Values slice points into the original
// stream (safe because the input is read-only), so no sample data is copied.
func (it *batchIterator) slice(w window) []Stream {
	var out []Stream
	for si := w.from.si; si <= w.to.si && si < len(it.streams); si++ {
		lo, hi := it.streamBounds(w, si)
		if lo < hi {
			out = append(out, Stream{Stream: it.streams[si].Stream, Values: it.streams[si].Values[lo:hi]})
		}
	}
	return out
}

// countSamples returns the number of samples in w. A window is a contiguous run, so this is simply the
// covered length of each stream it spans, summed.
func (it *batchIterator) countSamples(w window) int {
	n := 0
	for si := w.from.si; si <= w.to.si && si < len(it.streams); si++ {
		lo, hi := it.streamBounds(w, si)
		n += hi - lo
	}
	return n
}

// streamBounds returns the [lo, hi) value indices of stream si that fall inside w. Interior streams
// are covered fully; the first and last streams of the window are clamped to its endpoints.
func (it *batchIterator) streamBounds(w window, si int) (lo, hi int) {
	lo, hi = 0, len(it.streams[si].Values)
	if si == w.from.si {
		lo = w.from.vi
	}
	if si == w.to.si {
		hi = w.to.vi
	}
	return lo, hi
}

// skipEmpty advances c past any exhausted or empty streams so it points at a real sample, or at
// si == len(streams) if none remain. A cursor already on a sample is returned unchanged.
func (it *batchIterator) skipEmpty(c cursor) cursor {
	for c.si < len(it.streams) && c.vi >= len(it.streams[c.si].Values) {
		c.si++
		c.vi = 0
	}
	return c
}

// step returns the position of the sample immediately after c, which must point at a sample,
// skipping over any empty streams.
func (it *batchIterator) step(c cursor) cursor {
	c.vi++
	return it.skipEmpty(c)
}

// advance returns the position n samples after c. n must not exceed the samples remaining after c.
func (it *batchIterator) advance(c cursor, n int) cursor {
	for ; n > 0; n-- {
		c = it.step(c)
	}
	return c
}

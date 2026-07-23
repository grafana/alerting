package lokiclient

import (
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestSampleSize(t *testing.T) {
	t.Run("line length plus per-entry overhead", func(t *testing.T) {
		s := Sample{V: strings.Repeat("a", 100)}
		require.Equal(t, 100, s.estimatedSize())
	})

	t.Run("includes structured metadata keys and values", func(t *testing.T) {
		s := Sample{V: "abc", Metadata: map[string]string{"k": "vv", "kk": "v"}}
		// 3 (line) + (1+2) + (2+1) for the two metadata pairs.
		require.Equal(t, 3+3+3, s.estimatedSize())
	})
}

// fakeEncoder encodes to a byte slice whose length equals the uncompressed size estimate, so tests
// can force the exact-size bisection deterministically. Its reported ratio does not affect the
// encoded length; a ratio above 1 makes newBatchIterator pack more than actually fits, exercising
// encodeAndSplit.
type fakeEncoder struct {
	ratio float64
}

func (f fakeEncoder) encode(s []Stream) ([]byte, error) {
	n := 0
	for i := range s {
		for j := range s[i].Values {
			n += s[i].Values[j].estimatedSize()
		}
	}
	return make([]byte, n), nil
}

func (f fakeEncoder) headers() map[string]string        { return nil }
func (f fakeEncoder) expectedCompressionRatio() float64 { return f.ratio }

// sampleOfSize returns a sample whose size() is exactly n bytes.
func sampleOfSize(n int) Sample {
	return Sample{V: strings.Repeat("a", n)}
}

func drainNext(t *testing.T, it *batchIterator) [][]byte {
	t.Helper()
	var out [][]byte
	for it.next() {
		out = append(out, it.batch())
	}
	require.NoError(t, it.err())
	return out
}

func TestNextCandidate(t *testing.T) {
	labelsA := map[string]string{"rule": "A"}
	labelsB := map[string]string{"rule": "B"}

	t.Run("coalesces consecutive samples from one stream and stops at the budget", func(t *testing.T) {
		// Each sample is 1 byte; a 2-byte budget fits exactly two.
		it := newBatchIterator([]Stream{
			{Stream: labelsA, Values: []Sample{{V: "a"}, {V: "b"}, {V: "c"}}},
		}, fakeEncoder{ratio: 1}, 2, log.NewNopLogger())

		w1, ok := it.nextCandidate()
		require.True(t, ok)
		require.Equal(t, 2, it.countSamples(w1))
		c1 := it.slice(w1)
		require.Len(t, c1, 1)
		require.Len(t, c1[0].Values, 2)
		require.Equal(t, labelsA, c1[0].Stream)

		w2, ok := it.nextCandidate()
		require.True(t, ok)
		require.Equal(t, 1, it.countSamples(w2))
		c2 := it.slice(w2)
		require.Len(t, c2, 1)
		require.Len(t, c2[0].Values, 1)

		_, ok = it.nextCandidate()
		require.False(t, ok, "iterator is exhausted")
	})

	t.Run("keeps distinct source streams separate within one candidate", func(t *testing.T) {
		it := newBatchIterator([]Stream{
			{Stream: labelsA, Values: []Sample{{V: "a"}}},
			{Stream: labelsB, Values: []Sample{{V: "b"}}},
		}, fakeEncoder{ratio: 1}, 1<<20, log.NewNopLogger())

		w, ok := it.nextCandidate()
		require.True(t, ok)
		c := it.slice(w)
		require.Len(t, c, 2)
		require.Equal(t, labelsA, c[0].Stream)
		require.Equal(t, labelsB, c[1].Stream)
	})

	t.Run("skips empty streams while spanning multiple non-empty ones", func(t *testing.T) {
		// Empty streams around and between the data exercise the skip-empty-stream path.
		it := newBatchIterator([]Stream{
			{Stream: map[string]string{"rule": "empty-leading"}, Values: nil},
			{Stream: labelsA, Values: []Sample{{V: "a1"}, {V: "a2"}}},
			{Stream: map[string]string{"rule": "empty-middle"}, Values: []Sample{}},
			{Stream: labelsB, Values: []Sample{{V: "b1"}}},
		}, fakeEncoder{ratio: 1}, 1<<20, log.NewNopLogger())

		w, ok := it.nextCandidate()
		require.True(t, ok)
		require.Equal(t, 3, it.countSamples(w), "all three samples fit under the budget")
		c := it.slice(w)
		require.Len(t, c, 2, "empty streams are skipped, leaving only the two with samples")
		require.Equal(t, labelsA, c[0].Stream)
		require.Len(t, c[0].Values, 2)
		require.Equal(t, labelsB, c[1].Stream)
		require.Len(t, c[1].Values, 1)

		_, ok = it.nextCandidate()
		require.False(t, ok, "iterator is exhausted")
	})

	t.Run("always takes at least one sample even when it exceeds the budget", func(t *testing.T) {
		it := newBatchIterator([]Stream{
			{Stream: labelsA, Values: []Sample{{V: strings.Repeat("x", 100)}}},
		}, fakeEncoder{ratio: 1}, 10, log.NewNopLogger())

		w, ok := it.nextCandidate()
		require.True(t, ok)
		require.Equal(t, 1, it.countSamples(w))
		c := it.slice(w)
		require.Len(t, c, 1)
		require.Len(t, c[0].Values, 1)
	})
}

func TestWindowSlicing(t *testing.T) {
	labelsA := map[string]string{"rule": "A"}
	labelsB := map[string]string{"rule": "B"}
	it := newBatchIterator([]Stream{
		{Stream: labelsA, Values: []Sample{{V: "a1"}, {V: "a2"}}},
		{Stream: labelsB, Values: []Sample{{V: "b1"}}},
	}, fakeEncoder{ratio: 1}, 1<<20, log.NewNopLogger())
	end := it.advance(cursor{}, 3)
	require.Equal(t, 3, it.countSamples(window{from: cursor{}, to: end}))

	t.Run("splits on a stream boundary", func(t *testing.T) {
		mid := it.advance(cursor{}, 2)
		left := it.slice(window{from: cursor{}, to: mid})
		right := it.slice(window{from: mid, to: end})
		require.Len(t, left, 1)
		require.Len(t, left[0].Values, 2)
		require.Equal(t, labelsA, left[0].Stream)
		require.Len(t, right, 1)
		require.Len(t, right[0].Values, 1)
		require.Equal(t, labelsB, right[0].Stream)
	})

	t.Run("splits inside a stream, keeping its labels in both halves", func(t *testing.T) {
		mid := it.advance(cursor{}, 1)
		left := it.slice(window{from: cursor{}, to: mid})
		right := it.slice(window{from: mid, to: end})
		require.Len(t, left, 1)
		require.Equal(t, labelsA, left[0].Stream)
		require.Len(t, left[0].Values, 1)
		require.Equal(t, "a1", left[0].Values[0].V)

		require.Len(t, right, 2)
		require.Equal(t, labelsA, right[0].Stream)
		require.Equal(t, "a2", right[0].Values[0].V)
		require.Equal(t, labelsB, right[1].Stream)
	})
}

func TestEncodeAndSplit(t *testing.T) {
	labelsA := map[string]string{"rule": "A"}

	// fullWindow returns a window covering every sample the iterator holds.
	fullWindow := func(it *batchIterator) window {
		return window{from: cursor{}, to: it.advance(cursor{}, countValues(it.streams))}
	}

	t.Run("halves a batch until every piece fits the encoded limit", func(t *testing.T) {
		it := newBatchIterator([]Stream{{Stream: labelsA, Values: []Sample{
			sampleOfSize(100), sampleOfSize(100), sampleOfSize(100), sampleOfSize(100),
		}}}, fakeEncoder{ratio: 1}, 100, log.NewNopLogger())

		pieces, err := it.encodeAndSplit(fullWindow(it))
		require.NoError(t, err)
		require.Len(t, pieces, 4, "400 bytes / 100 limit halves down to four one-sample pieces")
		for _, p := range pieces {
			require.LessOrEqual(t, len(p), 100)
		}
	})

	t.Run("emits a single oversized entry as one piece", func(t *testing.T) {
		it := newBatchIterator([]Stream{{Stream: labelsA, Values: []Sample{sampleOfSize(232)}}},
			fakeEncoder{ratio: 1}, 100, log.NewNopLogger())

		pieces, err := it.encodeAndSplit(fullWindow(it))
		require.NoError(t, err)
		require.Len(t, pieces, 1)
		require.Greater(t, len(pieces[0]), 100, "an unsplittable entry is sent as-is")
	})
}

// countValues is a test helper: total samples across streams (the iterator itself no longer needs it).
func countValues(streams []Stream) int {
	n := 0
	for _, s := range streams {
		n += len(s.Values)
	}
	return n
}

func TestBatchIteratorNext(t *testing.T) {
	labelsA := map[string]string{"rule": "A"}

	t.Run("bisects an over-optimistically packed candidate so every batch fits", func(t *testing.T) {
		// ratio 10 makes the budget 1000, so all five 100-byte samples pack into one candidate that
		// encodes to 500 bytes and must be bisected down to one-sample batches.
		values := make([]Sample, 0, 5)
		for i := 0; i < 5; i++ {
			values = append(values, sampleOfSize(100))
		}
		it := newBatchIterator([]Stream{{Stream: labelsA, Values: values}}, fakeEncoder{ratio: 10}, 100, log.NewNopLogger())

		batches := drainNext(t, it)
		require.Len(t, batches, 5)
		for _, b := range batches {
			require.LessOrEqual(t, len(b), 100)
		}
	})

	t.Run("reports the end of iteration immediately for an empty payload", func(t *testing.T) {
		it := newBatchIterator(nil, fakeEncoder{ratio: 1}, 100, log.NewNopLogger())
		require.False(t, it.next())
		require.NoError(t, it.err())
	})
}

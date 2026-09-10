package lokiclient

// size returns the sample's size the way Loki accounts for ingested bytes: the log line plus its
// structured metadata, before any encoding or compression.
func (r *Sample) size() int {
	size := len(r.V)
	for k, v := range r.Metadata {
		size += len(k) + len(v)
	}
	return size
}

// splitIntoBatches groups the samples of streams into batches of at most maxBytes. Batches
// point into the given streams rather than copying their samples, and consecutive samples
// keep the labels of the stream they came from.
// A sample that alone exceeds maxBytes gets a batch of its own.
func splitIntoBatches(streams []Stream, maxBytes int) [][]Stream {
	var (
		batches      [][]Stream
		currentBatch []Stream
		currentSize  int
	)
	// take adds stream.Values[from:to] to the current batch, under that stream's labels.
	take := func(stream Stream, from, to int) {
		if from < to {
			currentBatch = append(currentBatch, Stream{Stream: stream.Stream, Values: stream.Values[from:to]})
		}
	}
	// flush closes the current batch, if anything was taken into it.
	flush := func() {
		if len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch, currentSize = nil, 0
		}
	}

	for _, stream := range streams {
		runStart := 0
		for i := range stream.Values {
			sampleSize := stream.Values[i].size()
			// An empty batch always takes the sample, so an oversized one still makes progress.
			if currentSize > 0 && currentSize+sampleSize > maxBytes {
				take(stream, runStart, i)
				flush()
				runStart = i
			}
			currentSize += sampleSize
		}
		take(stream, runStart, len(stream.Values))
	}
	flush()
	return batches
}

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

// splitStreams groups the samples of streams into batches of at most maxBytes, each to be pushed as
// its own request. Batches point into the given streams rather than copying their samples, and
// consecutive samples keep the labels of the stream they came from. A sample that alone exceeds
// maxBytes gets a batch of its own.
func splitStreams(streams []Stream, maxBytes int) [][]Stream {
	var (
		batches [][]Stream
		batch   []Stream
		size    int
	)
	for _, stream := range streams {
		from := 0
		for i := range stream.Values {
			sampleSize := stream.Values[i].size()
			if size > 0 && size+sampleSize > maxBytes {
				if i > from {
					batch = append(batch, Stream{Stream: stream.Stream, Values: stream.Values[from:i]})
				}
				batches = append(batches, batch)
				batch, size, from = nil, 0, i
			}
			size += sampleSize
		}
		if len(stream.Values) > from {
			batch = append(batch, Stream{Stream: stream.Stream, Values: stream.Values[from:]})
		}
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

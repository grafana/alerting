package lokiclient

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSampleSize(t *testing.T) {
	t.Run("counts the line length only, ignoring timestamp and labels", func(t *testing.T) {
		s := Sample{V: strings.Repeat("a", 100)}
		require.Equal(t, 100, s.size())
	})

	t.Run("includes structured metadata keys and values", func(t *testing.T) {
		s := Sample{V: "abc", Metadata: map[string]string{"k": "vv", "kk": "v"}}
		// 3 (line) + (1+2) + (2+1) for the two metadata pairs.
		require.Equal(t, 3+3+3, s.size())
	})
}

func TestSplitStreams(t *testing.T) {
	labelsA := map[string]string{"rule": "A"}
	labelsB := map[string]string{"rule": "B"}

	// sample returns a sample whose size() is exactly n bytes.
	sample := func(v string, n int) Sample {
		return Sample{V: v + strings.Repeat("a", n-len(v))}
	}

	t.Run("keeps everything in one batch when it fits", func(t *testing.T) {
		in := []Stream{
			{Stream: labelsA, Values: []Sample{sample("a1", 10), sample("a2", 10)}},
			{Stream: labelsB, Values: []Sample{sample("b1", 10)}},
		}

		batches := splitStreams(in, 100)

		require.Len(t, batches, 1)
		require.Equal(t, in, batches[0])
	})

	t.Run("splits within a stream, repeating its labels in both batches", func(t *testing.T) {
		in := []Stream{{Stream: labelsA, Values: []Sample{sample("a1", 10), sample("a2", 10), sample("a3", 10)}}}

		batches := splitStreams(in, 20)

		require.Len(t, batches, 2)
		require.Equal(t, []Stream{{Stream: labelsA, Values: in[0].Values[0:2]}}, batches[0])
		require.Equal(t, []Stream{{Stream: labelsA, Values: in[0].Values[2:3]}}, batches[1])
	})

	t.Run("spans several streams in one batch and splits on a stream boundary", func(t *testing.T) {
		in := []Stream{
			{Stream: labelsA, Values: []Sample{sample("a1", 10), sample("a2", 10)}},
			{Stream: labelsB, Values: []Sample{sample("b1", 10)}},
		}

		batches := splitStreams(in, 20)

		require.Len(t, batches, 2)
		require.Equal(t, []Stream{in[0]}, batches[0])
		require.Equal(t, []Stream{in[1]}, batches[1])
	})

	t.Run("gives a sample larger than the limit a batch of its own", func(t *testing.T) {
		in := []Stream{{Stream: labelsA, Values: []Sample{sample("a1", 10), sample("big", 500), sample("a3", 10)}}}

		batches := splitStreams(in, 20)

		require.Len(t, batches, 3)
		for i, batch := range batches {
			require.Len(t, batch, 1)
			require.Len(t, batch[0].Values, 1, "batch %d", i)
		}
		require.Equal(t, in[0].Values[1], batches[1][0].Values[0])
	})

	t.Run("drops empty streams and loses no samples", func(t *testing.T) {
		in := []Stream{
			{Stream: map[string]string{"rule": "empty-leading"}, Values: nil},
			{Stream: labelsA, Values: []Sample{sample("a1", 10), sample("a2", 10)}},
			{Stream: map[string]string{"rule": "empty-middle"}, Values: []Sample{}},
			{Stream: labelsB, Values: []Sample{sample("b1", 10)}},
		}

		for _, maxBytes := range []int{10, 20, 1000} {
			batches := splitStreams(in, maxBytes)

			var lines []string
			for _, batch := range batches {
				for _, stream := range batch {
					require.NotEmpty(t, stream.Values)
					for _, v := range stream.Values {
						lines = append(lines, v.V)
					}
				}
			}
			require.Equal(t, []string{in[1].Values[0].V, in[1].Values[1].V, in[3].Values[0].V}, lines,
				"maxBytes %d", maxBytes)
		}
	})

	t.Run("returns no batches for an empty payload", func(t *testing.T) {
		require.Empty(t, splitStreams(nil, 100))
		require.Empty(t, splitStreams([]Stream{{Stream: labelsA}}, 100))
	})
}

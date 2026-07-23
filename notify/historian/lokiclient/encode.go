package lokiclient

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

// snappyExpectedCompressionRatio is a conservative estimate of how much SnappyProtoEncoder shrinks
// the uncompressed size estimate for typical (repetitive) state-history payloads. It is used only
// to size batches: a value below the true ratio simply makes the exact encoded-size check re-split
// a little more often, never producing an oversized request.
const snappyExpectedCompressionRatio = 1.7

type JSONEncoder struct{}

func (e JSONEncoder) encode(s []Stream) ([]byte, error) {
	body := struct {
		Streams []Stream `json:"streams"`
	}{Streams: s}
	enc, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Loki payload: %w", err)
	}
	return enc, nil
}

func (e JSONEncoder) headers() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

// expectedCompressionRatio is 1.0 because JSON payloads are sent uncompressed.
func (e JSONEncoder) expectedCompressionRatio() float64 {
	return 1.0
}

type SnappyProtoEncoder struct{}

func (e SnappyProtoEncoder) encode(s []Stream) ([]byte, error) {
	body := push.PushRequest{
		Streams: make([]push.Stream, 0, len(s)),
	}

	for _, str := range s {
		entries := make([]push.Entry, 0, len(str.Values))
		for _, sample := range str.Values {
			entry := push.Entry{
				Timestamp: sample.T,
				Line:      sample.V,
			}
			if len(sample.Metadata) > 0 {
				entry.StructuredMetadata = make(push.LabelsAdapter, 0, len(sample.Metadata))
				for k, v := range sample.Metadata {
					entry.StructuredMetadata = append(entry.StructuredMetadata, push.LabelAdapter{Name: k, Value: v})
				}
			}
			entries = append(entries, entry)
		}
		body.Streams = append(body.Streams, push.Stream{
			Labels:  labelsMapToString(str.Stream, ""),
			Entries: entries,
			// Hash seems to be mainly used for query responses. Promtail does not seem to calculate this field on push.
		})
	}

	buf, err := proto.Marshal(&body)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Loki payload to proto: %w", err)
	}
	buf = snappy.Encode(nil, buf)
	return buf, nil
}

func (e SnappyProtoEncoder) headers() map[string]string {
	return map[string]string{
		"Content-Type":     "application/x-protobuf",
		"Content-Encoding": "snappy",
	}
}

// expectedCompressionRatio reports the conservative snappy compression estimate used to size
// batches before they are encoded and checked against the true limit.
func (e SnappyProtoEncoder) expectedCompressionRatio() float64 {
	return snappyExpectedCompressionRatio
}

// Copied from promtail.
// Modified slightly to work in terms of plain map[string]string to avoid some unnecessary copies and type casts.
func labelsMapToString(ls map[string]string, without model.LabelName) string {
	var b strings.Builder
	totalSize := 2
	lstrs := make([]string, 0, len(ls))

	for l, v := range ls {
		if l == string(without) {
			continue
		}

		lstrs = append(lstrs, l)
		// guess size increase: 2 for `, ` between labels and 3 for the `=` and quotes around label value
		totalSize += len(l) + 2 + len(v) + 3
	}

	b.Grow(totalSize)
	b.WriteByte('{')
	slices.Sort(lstrs)
	for i, l := range lstrs {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(l)
		b.WriteString(`=`)
		b.WriteString(strconv.Quote(ls[l]))
	}
	b.WriteByte('}')

	return b.String()
}

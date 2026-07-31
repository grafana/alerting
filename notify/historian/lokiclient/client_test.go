package lokiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	alertingInstrument "github.com/grafana/alerting/http/instrument"
	"github.com/grafana/alerting/http/instrument/instrumenttest"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

const lokiClientSpanName = "testLokiClientSpanName"

func TestLokiHTTPClient(t *testing.T) {
	t.Run("push formats expected data", func(t *testing.T) {
		req := instrumenttest.NewFakeRequester()
		client := createTestLokiClient(req)
		now := time.Now().UTC()
		data := []Stream{
			{
				Stream: map[string]string{},
				Values: []Sample{
					{
						T: now,
						V: "some line",
					},
				},
			},
		}

		err := client.Push(context.Background(), data)

		require.NoError(t, err)
		require.Contains(t, "/loki/api/v1/push", req.LastRequest.URL.Path)
		sent := reqBody(t, req.LastRequest)
		exp := fmt.Sprintf(`{"streams": [{"stream": {}, "values": [["%d", "some line"]]}]}`, now.UnixNano())
		require.JSONEq(t, exp, sent)
	})

	t.Run("range query", func(t *testing.T) {
		t.Run("passes along page size", func(t *testing.T) {
			req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
				Status:        "200 OK",
				StatusCode:    200,
				Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
				ContentLength: int64(0),
				Header:        make(http.Header, 0),
			})
			client := createTestLokiClient(req)
			now := time.Now().UTC().UnixNano()
			q := `{from="state-history"}`

			_, err := client.RangeQuery(context.Background(), q, now-100, now, 1100)

			require.NoError(t, err)
			params := req.LastRequest.URL.Query()
			require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
			require.Equal(t, fmt.Sprint(1100), params.Get("limit"))
		})

		t.Run("uses default page size if limit not provided", func(t *testing.T) {
			req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
				Status:        "200 OK",
				StatusCode:    200,
				Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
				ContentLength: int64(0),
				Header:        make(http.Header, 0),
			})
			client := createTestLokiClient(req)
			now := time.Now().UTC().UnixNano()
			q := `{from="state-history"}`

			_, err := client.RangeQuery(context.Background(), q, now-100, now, 0)

			require.NoError(t, err)
			params := req.LastRequest.URL.Query()
			require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
			require.Equal(t, fmt.Sprint(defaultPageSize), params.Get("limit"))
		})

		t.Run("uses default page size if limit invalid", func(t *testing.T) {
			req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
				Status:        "200 OK",
				StatusCode:    200,
				Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
				ContentLength: int64(0),
				Header:        make(http.Header, 0),
			})
			client := createTestLokiClient(req)
			now := time.Now().UTC().UnixNano()
			q := `{from="state-history"}`

			_, err := client.RangeQuery(context.Background(), q, now-100, now, -100)

			require.NoError(t, err)
			params := req.LastRequest.URL.Query()
			require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
			require.Equal(t, fmt.Sprint(defaultPageSize), params.Get("limit"))
		})

		t.Run("uses maximum page size if limit too big", func(t *testing.T) {
			req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
				Status:        "200 OK",
				StatusCode:    200,
				Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
				ContentLength: int64(0),
				Header:        make(http.Header, 0),
			})
			client := createTestLokiClient(req)
			now := time.Now().UTC().UnixNano()
			q := `{from="state-history"}`

			_, err := client.RangeQuery(context.Background(), q, now-100, now, maximumPageSize+1000)

			require.NoError(t, err)
			params := req.LastRequest.URL.Query()
			require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
			require.Equal(t, fmt.Sprint(maximumPageSize), params.Get("limit"))
		})
	})
}

// This function can be used for local testing, just remove the skip call.
func TestLokiHTTPClient_Manual(t *testing.T) {
	t.Skip()

	t.Run("smoke test pinging Loki", func(t *testing.T) {
		url, err := url.Parse("http://localhost:3100")
		require.NoError(t, err)

		bytesWritten := prometheus.NewCounter(prometheus.CounterOpts{})
		writeDuration := instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))

		client := NewLokiClient(LokiConfig{
			ReadPathURL:  url,
			WritePathURL: url,
			Encoder:      JSONEncoder{},
		}, NewRequester(), bytesWritten, writeDuration, log.NewNopLogger(), noop.NewTracerProvider().Tracer("test"), lokiClientSpanName)

		// Authorized request should not fail.
		err = client.Ping(context.Background())
		require.NoError(t, err)
	})

	t.Run("smoke test range querying Loki", func(t *testing.T) {
		url, err := url.Parse("http://localhost:3100")
		require.NoError(t, err)

		bytesWritten := prometheus.NewCounter(prometheus.CounterOpts{})
		writeDuration := instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))

		client := NewLokiClient(LokiConfig{
			ReadPathURL:       url,
			WritePathURL:      url,
			BasicAuthUser:     "<your_username>",
			BasicAuthPassword: "<your_password>",
			Encoder:           JSONEncoder{},
		}, NewRequester(), bytesWritten, writeDuration, log.NewNopLogger(), noop.NewTracerProvider().Tracer("test"), lokiClientSpanName)

		// Define the query time range
		start := time.Now().Add(-30 * time.Minute).UnixNano()
		end := time.Now().UnixNano()

		// Authorized request should not fail.
		res, err := client.RangeQuery(context.Background(), `{probe="Paris"}`, start, end, defaultPageSize)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func TestLokiHTTPClient_MetricsQuery(t *testing.T) {
	okResponse := func() *http.Response {
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
			ContentLength: int64(0),
			Header:        make(http.Header, 0),
		}
	}

	t.Run("hits instant query endpoint", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsQuery(context.Background(), `rate({from="state-history"}[5m])`, now, defaultPageSize)

		require.NoError(t, err)
		require.Contains(t, req.LastRequest.URL.Path, "/loki/api/v1/query")
		require.NotContains(t, req.LastRequest.URL.Path, "query_range")
	})

	t.Run("passes along time parameter", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsQuery(context.Background(), `rate({from="state-history"}[5m])`, now, defaultPageSize)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.True(t, params.Has("time"), "query params did not contain 'time': %#v", params)
		require.Equal(t, fmt.Sprint(now), params.Get("time"))
		require.False(t, params.Has("start"), "metrics query should not have 'start' param")
		require.False(t, params.Has("end"), "metrics query should not have 'end' param")
	})

	t.Run("passes along page size", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsQuery(context.Background(), `rate({from="state-history"}[5m])`, now, 1100)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
		require.Equal(t, fmt.Sprint(1100), params.Get("limit"))
	})

	t.Run("uses default page size if limit not provided", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsQuery(context.Background(), `rate({from="state-history"}[5m])`, now, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.Equal(t, fmt.Sprint(defaultPageSize), params.Get("limit"))
	})

	t.Run("uses maximum page size if limit too big", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsQuery(context.Background(), `rate({from="state-history"}[5m])`, now, maximumPageSize+1000)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.Equal(t, fmt.Sprint(maximumPageSize), params.Get("limit"))
	})

	t.Run("parses metric sample response", func(t *testing.T) {
		body := `{"data":{"result":[{"metric":{"job":"my-app"},"value":[1700000000.123,"42.5"]}]}}`
		req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header, 0),
		})
		client := createTestLokiClient(req)

		res, err := client.MetricsQuery(context.Background(), `rate({job="my-app"}[5m])`, time.Now().UnixNano(), defaultPageSize)

		require.NoError(t, err)
		require.Len(t, res.Data.Result, 1)
		require.Equal(t, map[string]string{"job": "my-app"}, res.Data.Result[0].Metric)
		ts, err := res.Data.Result[0].Value.Timestamp()
		require.NoError(t, err)
		require.InDelta(t, 1700000000.123, ts, 0.001)
		val, err := res.Data.Result[0].Value.Value()
		require.NoError(t, err)
		require.Equal(t, "42.5", val)
	})
}

func TestLokiHTTPClient_MetricsRangeQuery(t *testing.T) {
	okResponse := func() *http.Response {
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewBufferString(`{}`)),
			ContentLength: int64(0),
			Header:        make(http.Header, 0),
		}
	}

	t.Run("hits query_range endpoint", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, defaultPageSize, 0)

		require.NoError(t, err)
		require.Contains(t, req.LastRequest.URL.Path, "/loki/api/v1/query_range")
	})

	t.Run("passes along start and end parameters", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()
		start := now - 100

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, start, now, defaultPageSize, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.True(t, params.Has("start"), "query params did not contain 'start': %#v", params)
		require.True(t, params.Has("end"), "query params did not contain 'end': %#v", params)
		require.Equal(t, fmt.Sprint(start), params.Get("start"))
		require.Equal(t, fmt.Sprint(now), params.Get("end"))
		require.False(t, params.Has("time"), "metrics range query should not have 'time' param")
	})

	t.Run("passes along page size", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, 1100, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.True(t, params.Has("limit"), "query params did not contain 'limit': %#v", params)
		require.Equal(t, fmt.Sprint(1100), params.Get("limit"))
	})

	t.Run("uses default page size if limit not provided", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, 0, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.Equal(t, fmt.Sprint(defaultPageSize), params.Get("limit"))
	})

	t.Run("uses maximum page size if limit too big", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, maximumPageSize+1000, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.Equal(t, fmt.Sprint(maximumPageSize), params.Get("limit"))
	})

	t.Run("returns error if start is after end", func(t *testing.T) {
		req := instrumenttest.NewFakeRequester()
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now, now-100, defaultPageSize, 0)

		require.Error(t, err)
		require.ErrorContains(t, err, "start time cannot be after end time")
	})

	t.Run("passes along step parameter", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()
		step := int64(30)

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, defaultPageSize, step)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.True(t, params.Has("step"), "query params did not contain 'step': %#v", params)
		require.Equal(t, "30", params.Get("step"))
	})

	t.Run("omits step parameter when zero", func(t *testing.T) {
		resp := okResponse()
		t.Cleanup(func() { resp.Body.Close() })
		req := instrumenttest.NewFakeRequester().WithResponse(resp)
		client := createTestLokiClient(req)
		now := time.Now().UTC().UnixNano()

		_, err := client.MetricsRangeQuery(context.Background(), `rate({from="state-history"}[5m])`, now-100, now, defaultPageSize, 0)

		require.NoError(t, err)
		params := req.LastRequest.URL.Query()
		require.False(t, params.Has("step"), "query params should not contain 'step' when zero: %#v", params)
	})

	t.Run("parses metric range sample response", func(t *testing.T) {
		body := `{"data":{"result":[{"metric":{"job":"my-app"},"values":[[1700000000.0,"1.5"],[1700000060.0,"2.0"]]}]}}`
		req := instrumenttest.NewFakeRequester().WithResponse(&http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header, 0),
		})
		client := createTestLokiClient(req)
		now := time.Now().UnixNano()

		res, err := client.MetricsRangeQuery(context.Background(), `rate({job="my-app"}[5m])`, now-int64(time.Minute), now, defaultPageSize, 0)

		require.NoError(t, err)
		require.Len(t, res.Data.Result, 1)
		require.Equal(t, map[string]string{"job": "my-app"}, res.Data.Result[0].Metric)
		require.Len(t, res.Data.Result[0].Values, 2)
		ts, err := res.Data.Result[0].Values[0].Timestamp()
		require.NoError(t, err)
		require.InDelta(t, 1700000000.0, ts, 0.001)
		val, err := res.Data.Result[0].Values[0].Value()
		require.NoError(t, err)
		require.Equal(t, "1.5", val)
	})
}

func TestRow(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		row := Sample{
			T: time.Unix(0, 1234),
			V: "some sample",
		}

		jsn, err := json.Marshal(&row)

		require.NoError(t, err)
		require.JSONEq(t, `["1234", "some sample"]`, string(jsn))
	})

	t.Run("unmarshal", func(t *testing.T) {
		jsn := []byte(`["1234", "some sample"]`)

		row := Sample{}
		err := json.Unmarshal(jsn, &row)

		require.NoError(t, err)
		require.Equal(t, int64(1234), row.T.UnixNano())
		require.Equal(t, "some sample", row.V)
	})

	t.Run("unmarshal invalid", func(t *testing.T) {
		jsn := []byte(`{"key": "wrong shape"}`)

		row := Sample{}
		err := json.Unmarshal(jsn, &row)

		require.ErrorContains(t, err, "failed to deserialize sample")
	})

	t.Run("unmarshal bad timestamp", func(t *testing.T) {
		jsn := []byte(`["not-unix-nano", "some sample"]`)

		row := Sample{}
		err := json.Unmarshal(jsn, &row)

		require.ErrorContains(t, err, "timestamp in Loki sample")
	})
}

func TestStream(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		stream := Stream{
			Stream: map[string]string{"a": "b"},
			Values: []Sample{
				{T: time.Unix(0, 1), V: "one"},
				{T: time.Unix(0, 2), V: "two"},
			},
		}

		jsn, err := json.Marshal(stream)

		require.NoError(t, err)
		require.JSONEq(
			t,
			`{"stream": {"a": "b"}, "values": [["1", "one"], ["2", "two"]]}`,
			string(jsn),
		)
	})
}

func TestClampRange(t *testing.T) {
	tc := []struct {
		name     string
		oldRange []int64
		max      int64
		newRange []int64
	}{
		{
			name:     "clamps start value if max is smaller than range",
			oldRange: []int64{5, 10},
			max:      1,
			newRange: []int64{9, 10},
		},
		{
			name:     "returns same values if max is greater than range",
			oldRange: []int64{5, 10},
			max:      20,
			newRange: []int64{5, 10},
		},
		{
			name:     "returns same values if max is equal to range",
			oldRange: []int64{5, 10},
			max:      5,
			newRange: []int64{5, 10},
		},
		{
			name:     "returns same values if max is zero",
			oldRange: []int64{5, 10},
			max:      0,
			newRange: []int64{5, 10},
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			start, end := ClampRange(c.oldRange[0], c.oldRange[1], c.max)

			require.Equal(t, c.newRange[0], start)
			require.Equal(t, c.newRange[1], end)
		})
	}
}

func createTestLokiClient(req alertingInstrument.Requester) *HTTPLokiClient {
	return createTestLokiClientWithEncoder(req, JSONEncoder{})
}

func createTestLokiClientWithEncoder(req alertingInstrument.Requester, enc encoder) *HTTPLokiClient {
	url, _ := url.Parse("http://some.url")
	cfg := LokiConfig{
		WritePathURL: url,
		ReadPathURL:  url,
		Encoder:      enc,
	}

	bytesWritten := prometheus.NewCounter(prometheus.CounterOpts{})
	writeDuration := instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))
	return NewLokiClient(cfg, req, bytesWritten, writeDuration, log.NewNopLogger(), noop.NewTracerProvider().Tracer("test"), lokiClientSpanName)
}

func reqBody(t *testing.T, req *http.Request) string {
	t.Helper()

	defer func() {
		_ = req.Body.Close()
	}()
	byt, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	return string(byt)
}

// makeStream builds one stream of n samples whose lines are distinct and exactly lineLen bytes long,
// so batch sizes are predictable and lost or duplicated lines are detectable.
func makeStream(n, lineLen int) []Stream {
	now := time.Now().UTC()
	values := make([]Sample, 0, n)
	for i := 0; i < n; i++ {
		values = append(values, Sample{T: now.Add(time.Duration(i)), V: testLine(i, lineLen)})
	}
	return []Stream{{Stream: map[string]string{"from": "state-history"}, Values: values}}
}

func testLine(i, lineLen int) string {
	prefix := fmt.Sprintf("line-%d-", i)
	return prefix + strings.Repeat("a", lineLen-len(prefix))
}

type pushedStream struct {
	Labels map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func parsePush(t *testing.T, body string) []pushedStream {
	t.Helper()
	var parsed struct {
		Streams []pushedStream `json:"streams"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	return parsed.Streams
}

func TestLokiHTTPClientPushSplitting(t *testing.T) {
	t.Run("sends a single request when the payload fits the batch size", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 1 << 20

		require.NoError(t, client.Push(context.Background(), makeStream(10, 100)))
		require.Len(t, req.Bodies(), 1)
	})

	t.Run("sends a single request when splitting is disabled", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 0

		require.NoError(t, client.Push(context.Background(), makeStream(100, 1000)))
		require.Len(t, req.Bodies(), 1)
	})

	t.Run("splits an oversized payload into bounded requests without losing samples", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 2000

		require.NoError(t, client.Push(context.Background(), makeStream(20, 500)))

		bodies := req.Bodies()
		require.Len(t, bodies, 5, "20 lines of 500 bytes fill four lines per 2000-byte batch")
		var lines []string
		for _, body := range bodies {
			sent := 0
			for _, stream := range parsePush(t, body) {
				for _, v := range stream.Values {
					sent += len(v[1])
					lines = append(lines, v[1])
				}
			}
			require.LessOrEqual(t, sent, client.cfg.MaxWriteBatchSize)
		}
		want := make([]string, 0, 20)
		for i := 0; i < 20; i++ {
			want = append(want, testLine(i, 500))
		}
		require.ElementsMatch(t, want, lines)
	})

	t.Run("keeps every line under the labels of the stream it came from", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 2000

		now := time.Now().UTC()
		values := func(prefix string) []Sample {
			out := make([]Sample, 0, 8)
			for i := 0; i < 8; i++ {
				out = append(out, Sample{T: now.Add(time.Duration(i)), V: prefix + strings.Repeat("a", 500)})
			}
			return out
		}
		input := []Stream{
			{Stream: map[string]string{"from": "state-history", "rule": "A"}, Values: values("A-")},
			{Stream: map[string]string{"from": "state-history", "rule": "B"}, Values: values("B-")},
		}

		require.NoError(t, client.Push(context.Background(), input))

		got := map[string]int{}
		for _, body := range req.Bodies() {
			for _, stream := range parsePush(t, body) {
				rule := stream.Labels["rule"]
				for _, v := range stream.Values {
					require.True(t, strings.HasPrefix(v[1], rule+"-"), "line %q sent under rule=%s", v[1], rule)
					got[rule]++
				}
			}
		}
		require.Equal(t, map[string]int{"A": 8, "B": 8}, got)
	})

	t.Run("returns the encoding error", func(t *testing.T) {
		boom := errors.New("encode failed")
		req := newRecordingRequester()
		client := createTestLokiClientWithEncoder(req, failingEncoder{err: boom})
		client.cfg.MaxWriteBatchSize = 2000

		require.ErrorIs(t, client.Push(context.Background(), makeStream(20, 500)), boom)
		require.Empty(t, req.Bodies())
	})

	t.Run("returns an error when one of the split requests is rejected", func(t *testing.T) {
		req := newRecordingRequester()
		req.respond = func(_ int, body string) int {
			if strings.Contains(body, testLine(10, 500)) {
				return http.StatusBadRequest
			}
			return http.StatusOK
		}
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 2000

		require.Error(t, client.Push(context.Background(), makeStream(20, 500)))
	})

	t.Run("keeps at most MaxWriteConcurrency requests in flight", func(t *testing.T) {
		req := newRecordingRequester()
		req.block = make(chan struct{})
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 100
		client.cfg.MaxWriteConcurrency = 2

		done := make(chan error, 1)
		go func() { done <- client.Push(context.Background(), makeStream(10, 500)) }()

		require.Eventually(t, func() bool { return req.MaxInFlight() == 2 }, 10*time.Second, time.Millisecond,
			"expected the pushes to run in parallel up to the limit")
		close(req.block)

		require.NoError(t, <-done)
		require.Equal(t, 2, req.MaxInFlight())
		require.Len(t, req.Bodies(), 10)
	})

	t.Run("returns the context error and sends nothing when the context is already cancelled", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 100

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.Push(ctx, makeStream(50, 500))

		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, req.Bodies())
	})

	t.Run("surfaces cancellation instead of reporting a partial write as success", func(t *testing.T) {
		req := newRecordingRequester()
		ctx, cancel := context.WithCancel(context.Background())
		// Cancel while the first request is being served, mimicking a timeout partway through the payload.
		req.respond = func(_ int, _ string) int {
			cancel()
			return http.StatusOK
		}
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 100

		err := client.Push(ctx, makeStream(50, 500))

		require.ErrorIs(t, err, context.Canceled)
		require.Less(t, len(req.Bodies()), 50, "cancellation must stop the remaining batches")
	})
}

func TestLokiHTTPClientPushRetries(t *testing.T) {
	fastBackoff := backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond, MaxRetries: 2}

	t.Run("retries a rate-limited push until Loki accepts it", func(t *testing.T) {
		req := newRecordingRequester()
		req.respond = func(attempt int, _ string) int {
			if attempt == 1 {
				return http.StatusTooManyRequests
			}
			return http.StatusOK
		}
		client := createTestLokiClient(req)
		client.cfg.WriteBackoff = fastBackoff

		require.NoError(t, client.Push(context.Background(), makeStream(1, 100)))
		require.Len(t, req.Bodies(), 2)
	})

	t.Run("gives up after the configured number of retries", func(t *testing.T) {
		req := newRecordingRequester()
		req.respond = func(_ int, _ string) int { return http.StatusTooManyRequests }
		client := createTestLokiClient(req)
		client.cfg.WriteBackoff = fastBackoff

		err := client.Push(context.Background(), makeStream(1, 100))

		require.ErrorContains(t, err, "429")
		require.Len(t, req.Bodies(), 3, "the initial attempt plus MaxRetries")
	})

	t.Run("retries a server error", func(t *testing.T) {
		req := newRecordingRequester()
		req.respond = func(attempt int, _ string) int {
			if attempt == 1 {
				return http.StatusInternalServerError
			}
			return http.StatusOK
		}
		client := createTestLokiClient(req)
		client.cfg.WriteBackoff = fastBackoff

		require.NoError(t, client.Push(context.Background(), makeStream(1, 100)))
		require.Len(t, req.Bodies(), 2)
	})

	t.Run("does not retry a rejected payload", func(t *testing.T) {
		req := newRecordingRequester()
		req.respond = func(_ int, _ string) int { return http.StatusBadRequest }
		client := createTestLokiClient(req)
		client.cfg.WriteBackoff = fastBackoff

		require.Error(t, client.Push(context.Background(), makeStream(1, 100)))
		require.Len(t, req.Bodies(), 1)
	})
}

type failingEncoder struct {
	err error
}

func (e failingEncoder) encode([]Stream) ([]byte, error) { return nil, e.err }
func (e failingEncoder) headers() map[string]string      { return nil }

// recordingRequester is a concurrency-safe fake requester that records the body of every request it
// serves and tracks how many it serves at once.
type recordingRequester struct {
	mu          sync.Mutex
	bodies      []string
	inFlight    int
	maxInFlight int

	// respond, when set, returns the status to answer the given 1-based attempt with.
	respond func(attempt int, body string) int
	// block, when set, holds every request until the channel is closed.
	block chan struct{}
}

func newRecordingRequester() *recordingRequester {
	return &recordingRequester{}
}

func (r *recordingRequester) Bodies() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.bodies)
}

func (r *recordingRequester) MaxInFlight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxInFlight
}

func (r *recordingRequester) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.bodies = append(r.bodies, string(body))
	attempt := len(r.bodies)
	r.inFlight++
	r.maxInFlight = max(r.maxInFlight, r.inFlight)
	respond := r.respond
	r.mu.Unlock()

	status := http.StatusOK
	if respond != nil {
		status = respond(attempt, string(body))
	}
	if r.block != nil {
		<-r.block
	}

	r.mu.Lock()
	r.inFlight--
	r.mu.Unlock()

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

package lokiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	alertingInstrument "github.com/grafana/alerting/http/instrument"
	"github.com/grafana/alerting/http/instrument/instrumenttest"
	"github.com/grafana/dskit/instrument"
	"github.com/grafana/loki/pkg/push"
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

func TestLokiHTTPClientPushSplitting(t *testing.T) {
	// makeStream builds a single stream of n samples, each line being a repeated character of the
	// given length, so batch sizes are predictable.
	makeStream := func(n, lineLen int) []Stream {
		now := time.Now().UTC()
		values := make([]Sample, 0, n)
		for i := 0; i < n; i++ {
			values = append(values, Sample{T: now.Add(time.Duration(i)), V: strings.Repeat("a", lineLen)})
		}
		return []Stream{{Stream: map[string]string{"from": "state-history"}, Values: values}}
	}

	// collectLines parses recorded JSON push bodies and returns every log line that was sent.
	collectLines := func(t *testing.T, bodies []string) []string {
		t.Helper()
		var lines []string
		for _, body := range bodies {
			var parsed struct {
				Streams []struct {
					Values [][]string `json:"values"`
				} `json:"streams"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &parsed))
			for _, s := range parsed.Streams {
				for _, v := range s.Values {
					lines = append(lines, v[1])
				}
			}
		}
		return lines
	}

	t.Run("sends a single request when payload is under the limit", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 1 << 20 // 1MB, far above the payload

		err := client.Push(context.Background(), makeStream(10, 100))

		require.NoError(t, err)
		require.Len(t, req.Bodies(), 1)
	})

	t.Run("sends a single request when splitting is disabled (size 0)", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 0

		err := client.Push(context.Background(), makeStream(100, 1000))

		require.NoError(t, err)
		require.Len(t, req.Bodies(), 1)
	})

	t.Run("splits oversized payloads into multiple bounded requests", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		maxBatch := 2000
		client.cfg.MaxWriteBatchSize = maxBatch
		input := makeStream(20, 500)

		err := client.Push(context.Background(), input)

		require.NoError(t, err)
		bodies := req.Bodies()
		require.Greater(t, len(bodies), 1, "expected the payload to be split into multiple requests")
		for _, body := range bodies {
			require.LessOrEqual(t, len(body), maxBatch, "each request must stay within the configured limit")
		}
		// No samples are lost or duplicated across the split requests.
		require.ElementsMatch(t, []string{strings.Repeat("a", 500)}, dedup(collectLines(t, bodies)))
		require.Len(t, collectLines(t, bodies), 20)
	})

	t.Run("returns an error when one split request fails", func(t *testing.T) {
		sentinel := "FAIL_ME"
		req := newRecordingRequester()
		req.failIfContains = sentinel
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 2000

		now := time.Now().UTC()
		values := make([]Sample, 0, 20)
		for i := 0; i < 20; i++ {
			v := strings.Repeat("a", 500)
			if i == 10 {
				v = sentinel + strings.Repeat("a", 500)
			}
			values = append(values, Sample{T: now.Add(time.Duration(i)), V: v})
		}
		input := []Stream{{Stream: map[string]string{"from": "state-history"}, Values: values}}

		err := client.Push(context.Background(), input)

		require.Error(t, err)
	})

	t.Run("sends a single oversized entry as one request without splitting forever", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 100 // smaller than the single line below

		err := client.Push(context.Background(), makeStream(1, 5000))

		require.NoError(t, err)
		require.Len(t, req.Bodies(), 1)
	})

	t.Run("preserves per-stream labels when splitting across multiple streams", func(t *testing.T) {
		req := newRecordingRequester()
		client := createTestLokiClient(req)
		client.cfg.MaxWriteBatchSize = 2000

		now := time.Now().UTC()
		mkValues := func(prefix string, n int) []Sample {
			out := make([]Sample, 0, n)
			for i := 0; i < n; i++ {
				out = append(out, Sample{T: now.Add(time.Duration(i)), V: prefix + strings.Repeat("a", 500)})
			}
			return out
		}
		input := []Stream{
			{Stream: map[string]string{"from": "state-history", "rule": "A"}, Values: mkValues("A-", 8)},
			{Stream: map[string]string{"from": "state-history", "rule": "B"}, Values: mkValues("B-", 8)},
		}

		err := client.Push(context.Background(), input)
		require.NoError(t, err)

		// Every line must still travel under its original stream's labels, with none lost.
		got := map[string][]string{} // rule label -> lines
		for _, body := range req.Bodies() {
			var parsed struct {
				Streams []struct {
					Stream map[string]string `json:"stream"`
					Values [][]string        `json:"values"`
				} `json:"streams"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &parsed))
			for _, s := range parsed.Streams {
				for _, v := range s.Values {
					got[s.Stream["rule"]] = append(got[s.Stream["rule"]], v[1])
				}
			}
		}
		require.Len(t, got["A"], 8)
		require.Len(t, got["B"], 8)
		for _, line := range got["A"] {
			require.True(t, strings.HasPrefix(line, "A-"), "line under rule=A must be an A line: %q", line)
		}
		for _, line := range got["B"] {
			require.True(t, strings.HasPrefix(line, "B-"), "line under rule=B must be a B line: %q", line)
		}
	})
}

func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// recordingRequester is a concurrency-safe fake HTTP requester that records every request body and
// can be configured to fail requests whose body contains a sentinel substring.
type recordingRequester struct {
	mu             sync.Mutex
	bodies         []string
	failIfContains string
}

func newRecordingRequester() *recordingRequester {
	return &recordingRequester{}
}

func (r *recordingRequester) Bodies() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.bodies))
	copy(out, r.bodies)
	return out
}

func (r *recordingRequester) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.bodies = append(r.bodies, string(body))
	shouldFail := r.failIfContains != "" && strings.Contains(string(body), r.failIfContains)
	r.mu.Unlock()

	if shouldFail {
		return &http.Response{
			Status:     "400 Bad Request",
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString("payload rejected")),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("")),
		Header:     make(http.Header),
	}, nil
}

func TestLokiHTTPClientPushSplittingSnappy(t *testing.T) {
	// decodeSnappyLines reverses SnappyProtoEncoder: snappy-decompress, proto-unmarshal, collect lines.
	decodeSnappyLines := func(t *testing.T, body string) []string {
		t.Helper()
		raw, err := snappy.Decode(nil, []byte(body))
		require.NoError(t, err)
		var req push.PushRequest
		require.NoError(t, proto.Unmarshal(raw, &req))
		var lines []string
		for _, s := range req.Streams {
			for _, e := range s.Entries {
				lines = append(lines, e.Line)
			}
		}
		return lines
	}

	// A realistic, highly repetitive state-history payload: many near-identical lines that snappy
	// compresses well, so packing against the compressed limit fits far more per request than the
	// uncompressed estimate would.
	const (
		n       = 400
		lineLen = 512
		maxByte = 4096
	)
	now := time.Now().UTC()
	values := make([]Sample, 0, n)
	uncompressed := 0
	for i := 0; i < n; i++ {
		s := Sample{T: now.Add(time.Duration(i)), V: strings.Repeat("a", lineLen)}
		values = append(values, s)
		uncompressed += s.estimatedSize()
	}
	input := []Stream{{Stream: map[string]string{"from": "state-history"}, Values: values}}

	req := newRecordingRequester()
	client := createTestLokiClientWithEncoder(req, SnappyProtoEncoder{})
	client.cfg.MaxWriteBatchSize = maxByte

	require.NoError(t, client.Push(context.Background(), input))

	bodies := req.Bodies()
	require.NotEmpty(t, bodies)

	// Every request stays within the (compressed) limit.
	for _, b := range bodies {
		require.LessOrEqual(t, len(b), maxByte, "each snappy request must stay within the limit")
	}

	// Packing against the compressed size yields materially fewer requests than a naive uncompressed
	// packer, which would need ~uncompressed/maxByte requests.
	uncompressedRequests := (uncompressed + maxByte - 1) / maxByte
	require.Less(t, len(bodies), uncompressedRequests,
		"compression-aware packing should send fewer requests than uncompressed packing (%d)", uncompressedRequests)

	// No lines are lost or duplicated across the split.
	got := make([]string, 0, n)
	for _, b := range bodies {
		got = append(got, decodeSnappyLines(t, b)...)
	}
	require.Len(t, got, n)
}

package lokiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	alertingInstrument "github.com/grafana/alerting/http/instrument"
	"github.com/grafana/dskit/instrument"
	"go.opentelemetry.io/otel/trace"

	"github.com/prometheus/client_golang/prometheus"
)

const defaultPageSize = 1000
const maximumPageSize = 5000

func NewRequester() alertingInstrument.Requester {
	return &http.Client{}
}

// encoder serializes log streams to some byte format.
type encoder interface {
	// encode serializes a set of log streams to bytes.
	encode(s []Stream) ([]byte, error)
	// headers returns a set of HTTP-style headers that describes the encoding scheme used.
	headers() map[string]string
}

type LokiConfig struct {
	ReadPathURL       *url.URL
	WritePathURL      *url.URL
	BasicAuthUser     string
	BasicAuthPassword string
	TenantID          string
	ExternalLabels    map[string]string
	Encoder           encoder
	MaxQueryLength    time.Duration
	MaxQuerySize      int
}

type HTTPLokiClient struct {
	client       alertingInstrument.Requester
	encoder      encoder
	cfg          LokiConfig
	bytesWritten prometheus.Counter
	logger       log.Logger
}

// Kind of Operation (=, !=, =~, !~)
type Operator string

const (
	// Equal operator (=)
	Eq Operator = "="
	// Not Equal operator (!=)
	Neq Operator = "!="
	// Equal operator supporting RegEx (=~)
	EqRegEx Operator = "=~"
	// Not Equal operator supporting RegEx (!~)
	NeqRegEx Operator = "!~"
)

func NewLokiClient(cfg LokiConfig, req alertingInstrument.Requester, bytesWritten prometheus.Counter, writeDuration *instrument.HistogramCollector, logger log.Logger, tracer trace.Tracer, spanName string) *HTTPLokiClient {
	tc := alertingInstrument.NewTimedClient(req, writeDuration)
	trc := alertingInstrument.NewTracedClient(tc, tracer, spanName)
	return &HTTPLokiClient{
		client:       trc,
		encoder:      cfg.Encoder,
		cfg:          cfg,
		bytesWritten: bytesWritten,
		logger:       logger,
	}
}

func (c *HTTPLokiClient) Ping(ctx context.Context) error {
	uri := c.cfg.ReadPathURL.JoinPath("/loki/api/v1/labels")
	req, err := http.NewRequest(http.MethodGet, uri.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	c.setAuthAndTenantHeaders(req)

	req = req.WithContext(ctx)
	res, err := c.client.Do(req)
	if res != nil {
		defer func() {
			if err := res.Body.Close(); err != nil {
				level.Warn(c.logger).Log("msg", "Failed to close response body", "err", err)
			}
		}()
	}
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("ping request to loki endpoint returned a non-200 status code: %d", res.StatusCode)
	}
	level.Debug(c.logger).Log("msg", "Ping request to Loki endpoint succeeded", "status", res.StatusCode)
	return nil
}

type Stream struct {
	Stream map[string]string `json:"stream"`
	Values []Sample          `json:"values"`
}

type Sample struct {
	T        time.Time
	V        string
	Metadata map[string]string
}

func (r *Sample) MarshalJSON() ([]byte, error) {
	if len(r.Metadata) > 0 {
		return json.Marshal([]any{
			fmt.Sprintf("%d", r.T.UnixNano()), r.V, r.Metadata,
		})
	}
	return json.Marshal([2]string{
		fmt.Sprintf("%d", r.T.UnixNano()), r.V,
	})
}

func (r *Sample) UnmarshalJSON(b []byte) error {
	// A Loki stream sample is formatted like a list with two or three elements, [At, Val] or [At, Val, Metadata]
	// At is a string wrapping a timestamp, in nanosecond unix epoch.
	// Val is a string containing the logger line.
	// Metadata is an optional JSON object with string keys and string values.
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("failed to deserialize sample in Loki response: %w", err)
	}
	if len(raw) < 2 {
		return fmt.Errorf("failed to deserialize sample in Loki response: expected at least 2 elements, got %d", len(raw))
	}
	var ts string
	if err := json.Unmarshal(raw[0], &ts); err != nil {
		return fmt.Errorf("failed to deserialize sample timestamp in Loki response: %w", err)
	}
	nano, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("timestamp in Loki sample not convertible to nanosecond epoch: %v", ts)
	}
	r.T = time.Unix(0, nano)
	if err := json.Unmarshal(raw[1], &r.V); err != nil {
		return fmt.Errorf("failed to deserialize sample value in Loki response: %w", err)
	}
	if len(raw) >= 3 {
		if err := json.Unmarshal(raw[2], &r.Metadata); err != nil {
			return fmt.Errorf("failed to deserialize sample metadata in Loki response: %w", err)
		}
	}
	return nil
}

func (c *HTTPLokiClient) Push(ctx context.Context, s []Stream) error {
	enc, err := c.encoder.encode(s)
	if err != nil {
		return err
	}

	uri := c.cfg.WritePathURL.JoinPath("/loki/api/v1/push")
	req, err := http.NewRequest(http.MethodPost, uri.String(), bytes.NewBuffer(enc))
	if err != nil {
		return fmt.Errorf("failed to create Loki request: %w", err)
	}

	c.setAuthAndTenantHeaders(req)
	for k, v := range c.encoder.headers() {
		req.Header.Add(k, v)
	}

	c.bytesWritten.Add(float64(len(enc)))
	req = req.WithContext(ctx)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			level.Warn(c.logger).Log("msg", "Failed to close response body", "err", err)
		}
	}()

	_, err = c.handleLokiResponse(c.logger, resp)
	if err != nil {
		return err
	}

	return nil
}

func (c *HTTPLokiClient) setAuthAndTenantHeaders(req *http.Request) {
	if c.cfg.BasicAuthUser != "" || c.cfg.BasicAuthPassword != "" {
		req.SetBasicAuth(c.cfg.BasicAuthUser, c.cfg.BasicAuthPassword)
	}

	if c.cfg.TenantID != "" {
		req.Header.Add("X-Scope-OrgID", c.cfg.TenantID)
	}
}

func (c *HTTPLokiClient) RangeQuery(ctx context.Context, logQL string, start, end, limit int64) (QueryRes, error) {
	// Run the pre-flight checks for the query.
	if start > end {
		return QueryRes{}, fmt.Errorf("start time cannot be after end time")
	}
	start, end = ClampRange(start, end, c.cfg.MaxQueryLength.Nanoseconds())
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}

	queryURL := c.cfg.ReadPathURL.JoinPath("/loki/api/v1/query_range")

	values := url.Values{}
	values.Set("query", logQL)
	values.Set("start", fmt.Sprintf("%d", start))
	values.Set("end", fmt.Sprintf("%d", end))
	values.Set("limit", fmt.Sprintf("%d", limit))

	queryURL.RawQuery = values.Encode()
	level.Debug(c.logger).Log("msg", "Sending query request", "query", logQL, "start", start, "end", end, "limit", limit)
	req, err := http.NewRequest(http.MethodGet,
		queryURL.String(), nil)
	if err != nil {
		return QueryRes{}, fmt.Errorf("error creating request: %w", err)
	}

	req = req.WithContext(ctx)
	c.setAuthAndTenantHeaders(req)

	res, err := c.client.Do(req)
	if err != nil {
		return QueryRes{}, fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			level.Warn(c.logger).Log("msg", "Failed to close response body", "err", err)
		}
	}()

	data, err := c.handleLokiResponse(c.logger, res)
	if err != nil {
		return QueryRes{}, err
	}

	result := QueryRes{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse response", "err", err, "data", string(data))
		return QueryRes{}, fmt.Errorf("error parsing request response: %w", err)
	}

	return result, nil
}

func (c *HTTPLokiClient) MetricsQuery(ctx context.Context, logQL string, ts int64, limit int64) (MetricsQueryRes, error) {
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}

	queryURL := c.cfg.ReadPathURL.JoinPath("/loki/api/v1/query")

	values := url.Values{}
	values.Set("query", logQL)
	values.Set("time", fmt.Sprintf("%d", ts))
	values.Set("limit", fmt.Sprintf("%d", limit))

	queryURL.RawQuery = values.Encode()
	level.Debug(c.logger).Log("msg", "Sending metrics query request", "query", logQL, "time", ts, "limit", limit)
	req, err := http.NewRequest(http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return MetricsQueryRes{}, fmt.Errorf("error creating request: %w", err)
	}

	req = req.WithContext(ctx)
	c.setAuthAndTenantHeaders(req)

	res, err := c.client.Do(req)
	if err != nil {
		return MetricsQueryRes{}, fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			level.Warn(c.logger).Log("msg", "Failed to close response body", "err", err)
		}
	}()

	data, err := c.handleLokiResponse(c.logger, res)
	if err != nil {
		return MetricsQueryRes{}, err
	}

	result := MetricsQueryRes{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse response", "err", err, "data", string(data))
		return MetricsQueryRes{}, fmt.Errorf("error parsing request response: %w", err)
	}

	return result, nil
}

func (c *HTTPLokiClient) MetricsRangeQuery(ctx context.Context, logQL string, start, end, limit int64) (MetricsRangeQueryRes, error) {
	if start > end {
		return MetricsRangeQueryRes{}, fmt.Errorf("start time cannot be after end time")
	}
	start, end = ClampRange(start, end, c.cfg.MaxQueryLength.Nanoseconds())
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}

	queryURL := c.cfg.ReadPathURL.JoinPath("/loki/api/v1/query_range")

	values := url.Values{}
	values.Set("query", logQL)
	values.Set("start", fmt.Sprintf("%d", start))
	values.Set("end", fmt.Sprintf("%d", end))
	values.Set("limit", fmt.Sprintf("%d", limit))

	queryURL.RawQuery = values.Encode()
	level.Debug(c.logger).Log("msg", "Sending metrics range query request", "query", logQL, "start", start, "end", end, "limit", limit)
	req, err := http.NewRequest(http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return MetricsRangeQueryRes{}, fmt.Errorf("error creating request: %w", err)
	}

	req = req.WithContext(ctx)
	c.setAuthAndTenantHeaders(req)

	res, err := c.client.Do(req)
	if err != nil {
		return MetricsRangeQueryRes{}, fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			level.Warn(c.logger).Log("msg", "Failed to close response body", "err", err)
		}
	}()

	data, err := c.handleLokiResponse(c.logger, res)
	if err != nil {
		return MetricsRangeQueryRes{}, err
	}

	result := MetricsRangeQueryRes{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse response", "err", err, "data", string(data))
		return MetricsRangeQueryRes{}, fmt.Errorf("error parsing request response: %w", err)
	}

	return result, nil
}

func (c *HTTPLokiClient) MaxQuerySize() int {
	return c.cfg.MaxQuerySize
}

type QueryRes struct {
	Data QueryData `json:"data"`
}

type QueryData struct {
	Result []Stream `json:"result"`
}

type MetricsQueryRes struct {
	Data MetricsQueryData `json:"data"`
}

type MetricsQueryData struct {
	Result []MetricSample `json:"result"`
}

type MetricsRangeQueryRes struct {
	Data MetricsRangeQueryData `json:"data"`
}

type MetricsRangeQueryData struct {
	Result []MetricRangeSample `json:"result"`
}

// MetricRangeSample represents a single matrix result from a Loki metrics range query.
type MetricRangeSample struct {
	Metric map[string]string   `json:"metric"`
	Values []MetricSampleValue `json:"values"`
}

// MetricSample represents a single instant vector result from a Loki metrics query.
type MetricSample struct {
	Metric map[string]string `json:"metric"`
	Value  MetricSampleValue `json:"value"`
}

// MetricSampleValue is an [timestamp, value] pair returned by an instant metrics query.
// The timestamp is a Unix epoch in seconds (float64), and the value is a string-encoded number.
type MetricSampleValue [2]json.RawMessage

func (v MetricSampleValue) Timestamp() (float64, error) {
	var ts float64
	if err := json.Unmarshal(v[0], &ts); err != nil {
		return 0, fmt.Errorf("failed to deserialize metric sample timestamp: %w", err)
	}
	return ts, nil
}

func (v MetricSampleValue) Value() (string, error) {
	var val string
	if err := json.Unmarshal(v[1], &val); err != nil {
		return "", fmt.Errorf("failed to deserialize metric sample value: %w", err)
	}
	return val, nil
}

func (c *HTTPLokiClient) handleLokiResponse(logger log.Logger, res *http.Response) ([]byte, error) {
	if res == nil {
		return nil, fmt.Errorf("response is nil")
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if len(data) > 0 {
			level.Error(logger).Log("msg", "Error response from Loki", "response", string(data), "status", res.StatusCode)
		} else {
			level.Error(logger).Log("msg", "Error response from Loki with an empty body", "status", res.StatusCode)
		}
		return nil, fmt.Errorf("received a non-200 response from loki, status: %d", res.StatusCode)
	}

	return data, nil
}

// ClampRange ensures that the time range is within the configured maximum query length.
func ClampRange(start, end, maxTimeRange int64) (newStart int64, newEnd int64) {
	newStart, newEnd = start, end

	if maxTimeRange != 0 && end-start > maxTimeRange {
		newStart = end - maxTimeRange
	}

	return newStart, newEnd
}

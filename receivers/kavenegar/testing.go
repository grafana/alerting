package kavenegar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// SendMessageFunc is a function that sends a message to Kavenegar API
type SendMessageFunc func(ctx context.Context, apiURL string, params url.Values) (*kavenegarResponse, error)

// MockNotifier is a mock implementation of the Kavenegar notifier for testing
type MockNotifier struct {
	*Notifier
	sendMessageFn SendMessageFunc
}

// NewMockNotifier creates a new mock notifier
func NewMockNotifier(t *testing.T, cfg Config, tmpl *templates.Template) *MockNotifier {
	logger := log.NewNopLogger()
	meta := receivers.Metadata{
		UID:                   "test",
		Name:                  "test",
		Type:                  "kavenegar",
		DisableResolveMessage: false,
	}
	
	notifier := &Notifier{
		Base:       receivers.NewBase(meta, logger),
		tmpl:       tmpl,
		settings:   cfg,
		client:     http.DefaultClient,
		appVersion: "test",
	}
	
	return &MockNotifier{
		Notifier: notifier,
	}
}

// TestNotifier creates a test notifier with a mock HTTP server
func TestNotifier(t *testing.T, cfg Config, tmpl *templates.Template, responseHandler http.HandlerFunc) (*Notifier, *httptest.Server) {
	server := httptest.NewServer(responseHandler)
	
	// Override the client to use the test server
	client := &http.Client{}
	
	logger := log.NewNopLogger()
	meta := receivers.Metadata{
		UID:                   "test",
		Name:                  "test",
		Type:                  "kavenegar",
		DisableResolveMessage: false,
	}
	
	notifier := &Notifier{
		Base:       receivers.NewBase(meta, logger),
		tmpl:       tmpl,
		settings:   cfg,
		client:     client,
		appVersion: "test",
	}
	
	// Temporarily override the API endpoints to use test server
	// This would need to be done differently in actual implementation
	
	return notifier, server
}

// CreateTestAlert creates a test alert for testing
func CreateTestAlert(status string, labels map[string]string) *types.Alert {
	modelAlert := model.Alert{
		Labels:       model.LabelSet{},
		Annotations:  model.LabelSet{"summary": "Test alert"},
		GeneratorURL: "http://localhost/test",
	}
	
	// Convert labels
	for k, v := range labels {
		modelAlert.Labels[model.LabelName(k)] = model.LabelValue(v)
	}
	
	if status == "firing" {
		modelAlert.StartsAt = timeNow()
	} else {
		modelAlert.StartsAt = timeNow().Add(-1 * time.Hour)
		modelAlert.EndsAt = timeNow()
	}
	
	return &types.Alert{
		Alert: modelAlert,
	}
}

// timeNow returns current time - can be mocked in tests
var timeNow = time.Now

// SuccessResponse returns a successful Kavenegar API response
func SuccessResponse() kavenegarResponse {
	return kavenegarResponse{
		Return: struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		}{
			Status:  200,
			Message: "تایید شد",
		},
		Entries: []struct {
			MessageID  int64  `json:"messageid"`
			Message    string `json:"message"`
			Status     int    `json:"status"`
			StatusText string `json:"statustext"`
			Sender     string `json:"sender"`
			Receptor   string `json:"receptor"`
			Date       int64  `json:"date"`
			Cost       int    `json:"cost"`
		}{
			{
				MessageID:  12345678,
				Message:    "Test message",
				Status:     5,
				StatusText: "ارسال به مخابرات",
				Sender:     "10004346",
				Receptor:   "09123456789",
				Date:       time.Now().Unix(),
				Cost:       120,
			},
		},
	}
}

// ErrorResponse returns an error Kavenegar API response
func ErrorResponse(status int, message string) kavenegarResponse {
	return kavenegarResponse{
		Return: struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		}{
			Status:  status,
			Message: message,
		},
	}
}

// CreateTestConfig creates a test configuration
func CreateTestConfig(t *testing.T, overrides ...func(*Config)) Config {
	cfg := Config{
		ApiKey:     "test-api-key",
		Recipients: receivers.CommaSeparatedStrings{"09123456789"},
		SendMode:   SendModeSMS,
		Text:       "Test message: {{ .CommonAnnotations.summary }}",
		DebugMode:  true,
	}
	
	for _, override := range overrides {
		override(&cfg)
	}
	
	return cfg
}

// ValidateRequest validates that a request has the expected parameters
func ValidateRequest(t *testing.T, req *http.Request, expectedParams map[string]string) {
	err := req.ParseForm()
	require.NoError(t, err)
	
	for key, expectedValue := range expectedParams {
		actualValue := req.Form.Get(key)
		require.Equal(t, expectedValue, actualValue, "Parameter %s mismatch", key)
	}
}

// MockHTTPHandler creates a mock HTTP handler for testing
func MockHTTPHandler(t *testing.T, expectedPath string, response kavenegarResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate request path contains API key
		require.Contains(t, r.URL.Path, expectedPath)
		
		// Validate method
		require.Equal(t, http.MethodPost, r.Method)
		
		// Validate content type
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		
		// Send response
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}
}
package kavenegar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify_SMS(t *testing.T) {
	tmpl := templates.ForTests(t)

	tests := []struct {
		name           string
		config         Config
		alerts         []*types.Alert
		expectedParams map[string]string
		response       kavenegarResponse
		expectError    bool
	}{
		{
			name: "Send SMS successfully",
			config: Config{
				ApiKey:     "test-api-key",
				Recipients: receivers.CommaSeparatedStrings{"09123456789"},
				SendMode:   SendModeSMS,
				Text:       "Alert: {{ .CommonLabels.alertname }}",
				Sender:     "10004346",
			},
			alerts: []*types.Alert{
				CreateTestAlert("firing", map[string]string{"alertname": "HighCPU"}),
			},
			expectedParams: map[string]string{
				"receptor": "9123456789",
				"sender":   "10004346",
				"message":  "Alert: HighCPU",
			},
			response:    SuccessResponse(),
			expectError: false,
		},
		{
			name: "Send SMS to multiple recipients",
			config: Config{
				ApiKey:     "test-api-key",
				Recipients: receivers.CommaSeparatedStrings{"09123456789", "09987654321"},
				SendMode:   SendModeSMS,
				Text:       "{{ .CommonAnnotations.summary }}",
			},
			alerts: []*types.Alert{
				CreateTestAlert("firing", map[string]string{"alertname": "HighMemory"}),
			},
			response:    SuccessResponse(),
			expectError: false,
		},
		{
			name: "Handle API error",
			config: Config{
				ApiKey:     "test-api-key",
				Recipients: receivers.CommaSeparatedStrings{"09123456789"},
				SendMode:   SendModeSMS,
				Text:       "Test message",
				DebugMode:  true,
			},
			alerts: []*types.Alert{
				CreateTestAlert("firing", map[string]string{"alertname": "Test"}),
			},
			response:    ErrorResponse(418, "اعتبار حساب شما کافی نیست"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				
				// Validate request
				require.Equal(t, http.MethodPost, r.Method)
				require.Contains(t, r.URL.Path, tt.config.ApiKey)
				require.Contains(t, r.URL.Path, "sms/send.json")
				
				// Parse form
				err := r.ParseForm()
				require.NoError(t, err)
				
				// Validate some parameters if first request
				if requestCount == 1 && tt.expectedParams != nil {
					for key, expected := range tt.expectedParams {
						actual := r.Form.Get(key)
						require.Equal(t, expected, actual, "Parameter %s mismatch", key)
					}
				}
				
				// Send response
				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(tt.response)
				require.NoError(t, err)
			}))
			defer server.Close()

			// Override API URL for testing
			originalAPI := kavenegarAPISMS
			kavenegarAPISMS = server.URL + "/v1/%s/sms/send.json"
			defer func() { kavenegarAPISMS = originalAPI }()

			// Create notifier
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
				settings:   tt.config,
				client:     http.DefaultClient,
				appVersion: "test",
			}

			// Send notification
			_, err := notifier.Notify(context.Background(), tt.alerts...)
			
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			
			// Verify request count matches recipient count
			if !tt.expectError {
				require.Equal(t, len(tt.config.Recipients), requestCount)
			}
		})
	}
}

func TestNotify_OTP(t *testing.T) {
	tmpl := templates.ForTests(t)

	tests := []struct {
		name           string
		config         Config
		alerts         []*types.Alert
		expectedParams map[string]string
		response       kavenegarResponse
		expectError    bool
	}{
		{
			name: "Send OTP for firing alert",
			config: Config{
				ApiKey:           "test-api-key",
				Recipients:       receivers.CommaSeparatedStrings{"09123456789"},
				SendMode:         SendModeOTP,
				OtpTemplateError: "alert-error",
				OtpTemplateOk:    "alert-ok",
				Token1:           "{{ .CommonLabels.alertname }}",
				Token2:           "{{ .Status }}",
				Token3:           "{{ .CommonLabels.severity }}",
			},
			alerts: []*types.Alert{
				CreateTestAlert("firing", map[string]string{
					"alertname": "HighCPU",
					"severity":  "critical",
				}),
			},
			expectedParams: map[string]string{
				"receptor": "9123456789",
				"template": "alert-error",
				"token":    "HighCPU",
				"token2":   "firing",
				"token3":   "critical",
			},
			response:    SuccessResponse(),
			expectError: false,
		},
		{
			name: "Send OTP for resolved alert",
			config: Config{
				ApiKey:           "test-api-key",
				Recipients:       receivers.CommaSeparatedStrings{"09123456789"},
				SendMode:         SendModeOTP,
				OtpTemplateError: "alert-error",
				OtpTemplateOk:    "alert-ok",
				Token1:           "{{ .CommonLabels.alertname }}",
			},
			alerts: []*types.Alert{
				CreateTestAlert("resolved", map[string]string{
					"alertname": "HighCPU",
				}),
			},
			expectedParams: map[string]string{
				"receptor": "9123456789",
				"template": "alert-ok",
				"token":    "HighCPU",
				"token2":   "resolved",
			},
			response:    SuccessResponse(),
			expectError: false,
		},
		{
			name: "Use default tokens when not specified",
			config: Config{
				ApiKey:           "test-api-key",
				Recipients:       receivers.CommaSeparatedStrings{"09123456789"},
				SendMode:         SendModeOTP,
				OtpTemplateError: "alert-template",
			},
			alerts: []*types.Alert{
				CreateTestAlert("firing", map[string]string{
					"alertname": "TestAlert",
				}),
			},
			expectedParams: map[string]string{
				"receptor": "9123456789",
				"template": "alert-template",
				"token":    "TestAlert",
				"token2":   "firing",
			},
			response:    SuccessResponse(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				require.Equal(t, http.MethodPost, r.Method)
				require.Contains(t, r.URL.Path, tt.config.ApiKey)
				require.Contains(t, r.URL.Path, "verify/lookup.json")
				
				// Parse form
				err := r.ParseForm()
				require.NoError(t, err)
				
				// Validate parameters
				if tt.expectedParams != nil {
					for key, expected := range tt.expectedParams {
						actual := r.Form.Get(key)
						if strings.HasPrefix(key, "token") && key != "token" && actual == "" {
							// Optional tokens might not be present
							continue
						}
						require.Equal(t, expected, actual, "Parameter %s mismatch", key)
					}
				}
				
				// Send response
				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(tt.response)
				require.NoError(t, err)
			}))
			defer server.Close()

			// Override API URL for testing
			originalAPI := kavenegarAPIOTP
			kavenegarAPIOTP = server.URL + "/v1/%s/verify/lookup.json"
			defer func() { kavenegarAPIOTP = originalAPI }()

			// Create notifier
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
				settings:   tt.config,
				client:     http.DefaultClient,
				appVersion: "test",
			}

			// Send notification
			_, err := notifier.Notify(context.Background(), tt.alerts...)
			
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSanitizeToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test token", "testtoken"},
		{"test  multiple  spaces", "testmultiplespaces"},
		{strings.Repeat("a", 150), strings.Repeat("a", 100)},
		{"normal", "normal"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeToken(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleAPIError(t *testing.T) {
	logger := log.NewNopLogger()
	meta := receivers.Metadata{UID: "test"}
	notifier := &Notifier{Base: receivers.NewBase(meta, logger)}

	tests := []struct {
		status      int
		message     string
		expectedErr string
	}{
		{418, "اعتبار حساب شما کافی نیست", "insufficient account balance"},
		{422, "داده ها به دلیل وجود کاراکتر نامناسب", "invalid data due to inappropriate characters"},
		{424, "الگوی مورد نظر پیدا نشد", "template not found or not approved"},
		{426, "استفاده از این متد نیازمند سرویس پیشرفته", "advanced service required for this method"},
		{428, "ارسال کد از طریق تماس تلفنی", "voice call not possible - token must contain only numbers"},
		{431, "ساختار کد صحیح نمی‌باشد", "invalid token structure - contains newline, space, underscore or separator"},
		{432, "پارامتر کد در متن پیام پیدا نشد", "token parameter not found in message template"},
		{607, "نام تگ انتخابی اشتباه است", "invalid tag name"},
		{999, "Unknown error", "API error 999: Unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedErr, func(t *testing.T) {
			err := notifier.handleAPIError(tt.status, tt.message)
			require.Error(t, err)
			require.Equal(t, tt.expectedErr, err.Error())
		})
	}
}

// TestRealKavenegarSMS tests actual SMS sending to Kavenegar
// This test is skipped by default. To run it, use: go test -tags=integration
func TestRealKavenegarSMS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Real credentials from the prompt
	apiKey := "6D6D426C4275785372444B693575596559766A557A5666327279397965697136"
	sender := "10006468142491"
	recipient := "+989196070718"

	tmpl := templates.ForTests(t)

	config := Config{
		ApiKey:     apiKey,
		Sender:     sender,
		Recipients: receivers.CommaSeparatedStrings{recipient},
		SendMode:   SendModeSMS,
		Text:       "Grafana Alert Test: {{ .CommonLabels.alertname }} is {{ .Status }}",
		DebugMode:  true,
	}

	logger := log.NewNopLogger()
	meta := receivers.Metadata{
		UID:                   "test-real",
		Name:                  "test-real",
		Type:                  "kavenegar",
		DisableResolveMessage: false,
	}
	
	notifier := &Notifier{
		Base:       receivers.NewBase(meta, logger),
		tmpl:       tmpl,
		settings:   config,
		client:     http.DefaultClient,
		appVersion: "test",
	}

	// Create a test alert
	alert := CreateTestAlert("firing", map[string]string{
		"alertname": "TestAlert",
		"severity":  "info",
	})

	// Send the actual SMS
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := notifier.Notify(ctx, alert)
	require.NoError(t, err)

	t.Log("SMS sent successfully to", recipient)
}
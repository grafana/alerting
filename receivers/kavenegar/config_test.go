package kavenegar

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name        string
		settings    string
		secrets     map[string]string
		expectedErr string
		validate    func(t *testing.T, cfg Config)
	}{
		{
			name: "Valid SMS configuration",
			settings: `{
				"recipients": "09123456789,09987654321",
				"sendMode": "sms",
				"text": "Alert: {{ .CommonLabels.alertname }}",
				"sender": "10004346"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key-123",
			},
			validate: func(t *testing.T, cfg Config) {
				require.Equal(t, "test-api-key-123", cfg.ApiKey)
				require.Equal(t, "10004346", cfg.Sender)
				require.Len(t, cfg.Recipients, 2)
				require.Equal(t, "09123456789", cfg.Recipients[0])
				require.Equal(t, "09987654321", cfg.Recipients[1])
				require.Equal(t, SendModeSMS, cfg.SendMode)
				require.Equal(t, "Alert: {{ .CommonLabels.alertname }}", cfg.Text)
				require.False(t, cfg.DebugMode)
			},
		},
		{
			name: "Valid OTP configuration with error template",
			settings: `{
				"recipients": "09123456789",
				"sendMode": "otp",
				"otpTemplateError": "alert-error",
				"token1": "{{ .CommonLabels.alertname }}",
				"token2": "{{ .CommonLabels.severity }}",
				"debugMode": true
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key-456",
			},
			validate: func(t *testing.T, cfg Config) {
				require.Equal(t, "test-api-key-456", cfg.ApiKey)
				require.Equal(t, SendModeOTP, cfg.SendMode)
				require.Equal(t, "alert-error", cfg.OtpTemplateError)
				require.Equal(t, "{{ .CommonLabels.alertname }}", cfg.Token1)
				require.Equal(t, "{{ .CommonLabels.severity }}", cfg.Token2)
				require.True(t, cfg.DebugMode)
			},
		},
		{
			name: "Valid OTP configuration with both templates",
			settings: `{
				"recipients": "09123456789",
				"sendMode": "otp",
				"otpTemplateError": "alert-error",
				"otpTemplateOk": "alert-resolved"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key-789",
			},
			validate: func(t *testing.T, cfg Config) {
				require.Equal(t, "alert-error", cfg.OtpTemplateError)
				require.Equal(t, "alert-resolved", cfg.OtpTemplateOk)
			},
		},
		{
			name: "Default values applied",
			settings: `{
				"recipients": "09123456789"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			validate: func(t *testing.T, cfg Config) {
				require.Equal(t, SendModeSMS, cfg.SendMode)
				require.Equal(t, "{{ template \"default.message\" . }}", cfg.Text)
				require.False(t, cfg.DebugMode)
			},
		},
		{
			name: "International phone number",
			settings: `{
				"recipients": "00989123456789,+989987654321",
				"sendMode": "sms"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			validate: func(t *testing.T, cfg Config) {
				require.Len(t, cfg.Recipients, 2)
			},
		},
		{
			name:     "Missing API key",
			settings: `{"recipients": "09123456789"}`,
			secrets:  map[string]string{},
			expectedErr: "apiKey is required",
		},
		{
			name: "Missing recipients",
			settings: `{}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			expectedErr: "at least one recipient is required",
		},
		{
			name: "Invalid phone number format",
			settings: `{
				"recipients": "12345,09123456789"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			expectedErr: "invalid phone number format: 12345",
		},
		{
			name: "Invalid send mode",
			settings: `{
				"recipients": "09123456789",
				"sendMode": "invalid"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			expectedErr: "invalid sendMode: invalid. Must be 'sms' or 'otp'",
		},
		{
			name: "OTP mode without templates",
			settings: `{
				"recipients": "09123456789",
				"sendMode": "otp"
			}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			expectedErr: "at least one of otpTemplateError or otpTemplateOk is required for OTP mode",
		},
		{
			name: "Invalid JSON",
			settings: `{invalid json}`,
			secrets: map[string]string{
				"apiKey": "test-api-key",
			},
			expectedErr: "failed to unmarshal settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decryptFn := func(key string, fallback string) string {
				if value, ok := tt.secrets[key]; ok {
					return value
				}
				return fallback
			}

			cfg, err := NewConfig(json.RawMessage(tt.settings), decryptFn)
			
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestPhoneNumberValidation(t *testing.T) {
	tests := []struct {
		phone    string
		expected bool
	}{
		// Valid Iranian numbers
		{"09123456789", true},
		{"9123456789", true},
		{"+989123456789", true},
		{"00989123456789", true},
		{"989123456789", true},
		
		// Valid international numbers
		{"00974211234565", true},
		{"001234567890123", true},
		
		// Invalid numbers
		{"12345", false},
		{"091234567", false},     // Too short
		{"0912345678901", false}, // Too long
		{"08123456789", false},   // Invalid Iranian prefix
		{"abcdefghijk", false},   // Letters
		{"", false},              // Empty
		{"0098", false},          // Too short international
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			result := isValidPhoneNumber(tt.phone)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Iranian numbers
		{"09123456789", "9123456789"},
		{"+989123456789", "9123456789"},
		{"00989123456789", "9123456789"},
		{"989123456789", "9123456789"},
		{"9123456789", "9123456789"},
		
		// International numbers
		{"00974211234565", "00974211234565"},
		{"001234567890", "001234567890"},
		
		// With spaces and dashes
		{"0912-345-6789", "9123456789"},
		{"0912 345 6789", "9123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePhoneNumber(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
package kavenegar

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	ApiKey           string                          `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Sender           string                          `json:"sender,omitempty" yaml:"sender,omitempty"`
	Recipients       receivers.CommaSeparatedStrings `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	SendMode         string                          `json:"sendMode,omitempty" yaml:"sendMode,omitempty"` // "sms" or "otp"
	Text             string                          `json:"text,omitempty" yaml:"text,omitempty"`
	OtpTemplateError string                          `json:"otpTemplateError,omitempty" yaml:"otpTemplateError,omitempty"`
	OtpTemplateOk    string                          `json:"otpTemplateOk,omitempty" yaml:"otpTemplateOk,omitempty"`
	Token1           string                          `json:"token1,omitempty" yaml:"token1,omitempty"`
	Token2           string                          `json:"token2,omitempty" yaml:"token2,omitempty"`
	Token3           string                          `json:"token3,omitempty" yaml:"token3,omitempty"`
	DebugMode        bool                            `json:"debugMode,omitempty" yaml:"debugMode,omitempty"`
}

const (
	SendModeSMS = "sms"
	SendModeOTP = "otp"
)

var (
	// Phone number validation regex - supports Iranian and international formats
	phoneRegex = regexp.MustCompile(`^(\+98|0098|98|0)?9\d{9}$|^00\d{10,15}$`)
)

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	var settings Config
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	// Decrypt sensitive fields
	settings.ApiKey = decryptFn("apiKey", settings.ApiKey)
	if settings.ApiKey == "" {
		return Config{}, errors.New("apiKey is required")
	}

	// Validate recipients
	if len(settings.Recipients) == 0 {
		return Config{}, errors.New("at least one recipient is required")
	}

	// Validate phone numbers
	for _, recipient := range settings.Recipients {
		if !isValidPhoneNumber(recipient) {
			return Config{}, fmt.Errorf("invalid phone number format: %s", recipient)
		}
	}

	// Set default send mode
	if settings.SendMode == "" {
		settings.SendMode = SendModeSMS
	}

	// Validate send mode
	if settings.SendMode != SendModeSMS && settings.SendMode != SendModeOTP {
		return Config{}, fmt.Errorf("invalid sendMode: %s. Must be 'sms' or 'otp'", settings.SendMode)
	}

	// Validate mode-specific requirements
	if settings.SendMode == SendModeSMS {
		if settings.Text == "" {
			settings.Text = templates.DefaultMessageEmbed
		}
	} else if settings.SendMode == SendModeOTP {
		if settings.OtpTemplateError == "" && settings.OtpTemplateOk == "" {
			return Config{}, errors.New("at least one of otpTemplateError or otpTemplateOk is required for OTP mode")
		}
	}

	// Set default debug mode
	// DebugMode is already false by default (zero value)

	return settings, nil
}

// isValidPhoneNumber validates phone numbers according to Kavenegar requirements
func isValidPhoneNumber(phone string) bool {
	// Remove spaces and dashes
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	
	return phoneRegex.MatchString(phone)
}

// normalizePhoneNumber converts phone numbers to the format expected by Kavenegar API
func normalizePhoneNumber(phone string) string {
	// Remove spaces and dashes
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	
	// Handle Iranian numbers
	if strings.HasPrefix(phone, "+98") {
		return phone[3:] // Remove +98
	} else if strings.HasPrefix(phone, "0098") {
		return phone[4:] // Remove 0098
	} else if strings.HasPrefix(phone, "98") && len(phone) == 12 {
		return phone[2:] // Remove 98
	} else if strings.HasPrefix(phone, "0") && len(phone) == 11 {
		return phone[1:] // Remove leading 0
	}
	
	// For international numbers starting with 00, keep as is
	return phone
}
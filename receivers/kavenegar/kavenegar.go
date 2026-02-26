package kavenegar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

var (
	// Kavenegar API endpoints (var to allow override in tests)
	kavenegarAPISMS = "https://api.kavenegar.com/v1/%s/sms/send.json"
	kavenegarAPIOTP = "https://api.kavenegar.com/v1/%s/verify/lookup.json"
)

// Notifier is responsible for sending notifications to Kavenegar
type Notifier struct {
	*receivers.Base
	tmpl       *templates.Template
	settings   Config
	client     *http.Client
	appVersion string
}

// New creates a new Kavenegar notifier
func New(cfg Config, meta receivers.Metadata, tmpl *templates.Template, httpClient *http.Client, logger log.Logger, appVersion string) *Notifier {
	return &Notifier{
		Base:       receivers.NewBase(meta, logger),
		tmpl:       tmpl,
		settings:   cfg,
		client:     httpClient,
		appVersion: appVersion,
	}
}

// Notify sends notifications to Kavenegar
func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, data := templates.TmplText(ctx, n.tmpl, as, n.GetLogger(ctx), &tmplErr)
	
	for _, alert := range as {
		if err := n.sendAlert(ctx, alert, tmpl, data); err != nil {
			level.Error(n.GetLogger(ctx)).Log("msg", "Failed to send alert", "alert", alert, "error", err)
			if n.settings.DebugMode {
				return false, err
			}
			// Continue sending other alerts even if one fails
		}
	}
	
	return false, nil
}

func (n *Notifier) sendAlert(ctx context.Context, alert *types.Alert, tmpl func(string) string, data *templates.ExtendedData) error {
	if n.settings.SendMode == SendModeSMS {
		return n.sendSMS(ctx, alert, tmpl)
	}
	return n.sendOTP(ctx, alert, tmpl)
}

func (n *Notifier) sendSMS(ctx context.Context, alert *types.Alert, tmpl func(string) string) error {
	// Prepare SMS text
	text := tmpl(n.settings.Text)

	apiURL := fmt.Sprintf(kavenegarAPISMS, n.settings.ApiKey)
	
	// Send to all recipients
	for _, recipient := range n.settings.Recipients {
		params := url.Values{}
		params.Set("receptor", normalizePhoneNumber(recipient))
		if n.settings.Sender != "" {
			params.Set("sender", n.settings.Sender)
		}
		params.Set("message", text)

		if err := n.sendRequest(ctx, apiURL, params); err != nil {
			return fmt.Errorf("failed to send SMS to %s: %w", recipient, err)
		}
	}
	
	return nil
}

func (n *Notifier) sendOTP(ctx context.Context, alert *types.Alert, tmpl func(string) string) error {
	// Select template based on alert status
	var templateName string
	if alert.Status() == "firing" && n.settings.OtpTemplateError != "" {
		templateName = n.settings.OtpTemplateError
	} else if alert.Status() == "resolved" && n.settings.OtpTemplateOk != "" {
		templateName = n.settings.OtpTemplateOk
	} else {
		// Use whichever template is available
		if n.settings.OtpTemplateError != "" {
			templateName = n.settings.OtpTemplateError
		} else {
			templateName = n.settings.OtpTemplateOk
		}
	}

	apiURL := fmt.Sprintf(kavenegarAPIOTP, n.settings.ApiKey)
	
	// Prepare tokens
	token1 := tmpl(n.settings.Token1)
	token2 := tmpl(n.settings.Token2)
	token3 := tmpl(n.settings.Token3)
	
	// If tokens are empty, use alert information
	if token1 == "" {
		token1 = string(alert.Labels["alertname"])
	}
	if token2 == "" {
		token2 = string(alert.Status())
	}
	if token3 == "" && !alert.StartsAt.IsZero() {
		token3 = alert.StartsAt.Format("15:04:05")
	}

	// Send to all recipients
	for _, recipient := range n.settings.Recipients {
		params := url.Values{}
		params.Set("receptor", normalizePhoneNumber(recipient))
		params.Set("template", templateName)
		params.Set("token", sanitizeToken(token1))
		if token2 != "" {
			params.Set("token2", sanitizeToken(token2))
		}
		if token3 != "" {
			params.Set("token3", sanitizeToken(token3))
		}

		if err := n.sendRequest(ctx, apiURL, params); err != nil {
			return fmt.Errorf("failed to send OTP to %s: %w", recipient, err)
		}
	}
	
	return nil
}

// sanitizeToken removes spaces and ensures token doesn't exceed 100 characters
func sanitizeToken(token string) string {
	// Remove spaces for regular tokens (not token10 or token20)
	token = strings.ReplaceAll(token, " ", "")
	
	// Limit to 100 characters
	if len(token) > 100 {
		token = token[:100]
	}
	
	return token
}

func (n *Notifier) sendRequest(ctx context.Context, apiURL string, params url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", fmt.Sprintf("Grafana/%s", n.appVersion))
	
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			level.Error(n.GetLogger(ctx)).Log("msg", "Failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if n.settings.DebugMode {
		level.Debug(n.GetLogger(ctx)).Log("msg", "Kavenegar API response", "status", resp.StatusCode, "body", string(body))
	}

	// Parse response
	var response kavenegarResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if response.Return.Status != 200 {
		return n.handleAPIError(response.Return.Status, response.Return.Message)
	}

	return nil
}

func (n *Notifier) handleAPIError(status int, message string) error {
	switch status {
	case 418:
		return fmt.Errorf("insufficient account balance")
	case 422:
		return fmt.Errorf("invalid data due to inappropriate characters")
	case 424:
		return fmt.Errorf("template not found or not approved")
	case 426:
		return fmt.Errorf("advanced service required for this method")
	case 428:
		return fmt.Errorf("voice call not possible - token must contain only numbers")
	case 431:
		return fmt.Errorf("invalid token structure - contains newline, space, underscore or separator")
	case 432:
		return fmt.Errorf("token parameter not found in message template")
	case 607:
		return fmt.Errorf("invalid tag name")
	default:
		return fmt.Errorf("API error %d: %s", status, message)
	}
}

// SendResolved returns true if the notifier should send resolved alerts
func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}

// kavenegarResponse represents the API response structure
type kavenegarResponse struct {
	Return struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"return"`
	Entries []struct {
		MessageID  int64  `json:"messageid"`
		Message    string `json:"message"`
		Status     int    `json:"status"`
		StatusText string `json:"statustext"`
		Sender     string `json:"sender"`
		Receptor   string `json:"receptor"`
		Date       int64  `json:"date"`
		Cost       int    `json:"cost"`
	} `json:"entries"`
}
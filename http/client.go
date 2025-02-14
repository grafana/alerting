package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
)

// This is carbon copy of https://github.com/grafana/grafana/blob/71d04a326be9578e2d678f23c1efa61768e0541f/pkg/services/notifications/webhook.go#L38

type NotificationService struct {
	log logging.Logger
}

func (ns *NotificationService) SendWebhook(ctx context.Context, webhook *receivers.SendWebhookSettings) error {
	if webhook.HTTPMethod == "" {
		webhook.HTTPMethod = http.MethodPost
	}
	ns.log.Debug("Sending webhook", "url", webhook.URL, "http method", webhook.HTTPMethod)

	if webhook.HTTPMethod != http.MethodPost && webhook.HTTPMethod != http.MethodPut {
		return fmt.Errorf("webhook only supports HTTP methods PUT or POST")
	}

	request, err := http.NewRequestWithContext(ctx, webhook.HTTPMethod, webhook.URL, bytes.NewReader([]byte(webhook.Body)))
	if err != nil {
		return err
	}
	url, err := url.Parse(webhook.URL)
	if err != nil {
		// Should not be possible - NewRequestWithContext should also err if the URL is bad.
		return err
	}

	if webhook.ContentType == "" {
		webhook.ContentType = "application/json"
	}

	request.Header.Set("Content-Type", webhook.ContentType)
	request.Header.Set("User-Agent", "Grafana")

	if webhook.User != "" && webhook.Password != "" {
		request.Header.Set("Authorization", GetBasicAuthHeader(webhook.User, webhook.Password))
	}

	for k, v := range webhook.HTTPHeader {
		request.Header.Set(k, v)
	}

	resp, err := receivers.NewTLSClient(webhook.TLSConfig).Do(request)
	if err != nil {
		return redactURL(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			ns.log.Warn("Failed to close response body", "err", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if webhook.Validation != nil {
		err := webhook.Validation(body, resp.StatusCode)
		if err != nil {
			ns.log.Debug("Webhook failed validation", "url", url.Redacted(), "statuscode", resp.Status, "body", string(body), "error", err)
			return fmt.Errorf("webhook failed validation: %w", err)
		}
	}

	if resp.StatusCode/100 == 2 {
		ns.log.Debug("Webhook succeeded", "url", url.Redacted(), "statuscode", resp.Status)
		return nil
	}

	ns.log.Debug("Webhook failed", "url", url.Redacted(), "statuscode", resp.Status, "body", string(body))
	return fmt.Errorf("webhook response status %v", resp.Status)
}

func redactURL(err error) error {
	var e *url.Error
	if !errors.As(err, &e) {
		return err
	}
	e.URL = "<redacted>"
	return e
}

func GetBasicAuthHeader(user string, password string) string {
	var userAndPass = user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(userAndPass))
}

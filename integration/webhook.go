package integration

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/e2e"
)

const (
	defaultWebhookImage = "webhook-receiver"
	webhookBinary       = "/bin/main"
	webhookHTTPPort     = 8080
)

type WebhookService struct {
	*e2e.HTTPService
}

func NewWebhookService(name string, flags, envVars map[string]string) *WebhookService {
	svc := &WebhookService{
		HTTPService: e2e.NewHTTPService(
			name,
			"webhook-receiver",
			e2e.NewCommandWithoutEntrypoint(webhookBinary, e2e.BuildArgs(flags)...),
			e2e.NewHTTPReadinessProbe(webhookHTTPPort, "/ready", 200, 299),
			webhookHTTPPort),
	}

	svc.SetEnvVars(envVars)

	return svc
}

type WebhookClient struct {
	c http.Client
	u *url.URL
}

func NewWebhookClient(u string) (*WebhookClient, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	return &WebhookClient{
		c: http.Client{},
		u: pu,
	}, nil
}

// GetAlerts fetches the alerts from the webhook server
func (c *WebhookClient) GetAlerts() (map[string]any, error) {
	u := c.u.ResolveReference(&url.URL{Path: "/alerts"})

	resp, err := c.c.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	res := make(map[string]any)

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CreateAlert creates a new alert
func (c *WebhookClient) CreateAlert(a map[string]any) error {
	u := c.u.ResolveReference(&url.URL{Path: "/alert"})

	d, err := json.Marshal(a)
	if err != nil {
		return err
	}

	resp, err := c.c.Post(u.String(), "application/json", bytes.NewReader(d))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

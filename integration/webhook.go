package integration

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

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

type GetAlertsResponse struct {
	Stats   map[string]int `json:"stats"`
	History []struct {
		Status            string    `json:"status"`
		TimeNow           time.Time `json:"timeNow"`
		StartsAt          time.Time `json:"startsAt"`
		Node              string    `json:"node"`
		DeltaLastSeconds  float64   `json:"deltaLastSeconds"`
		DeltaStartSeconds float64   `json:"deltaStartSeconds"`
	} `json:"history"`
}

// GetAlerts fetches the alerts from the webhook server
func (c *WebhookClient) GetAlerts() (*GetAlertsResponse, error) {
	u := c.u.ResolveReference(&url.URL{Path: "/alerts"})

	resp, err := c.c.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	res := GetAlertsResponse{}

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

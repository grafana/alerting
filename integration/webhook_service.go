package integration

import (
	_ "embed"

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

package notify

import (
	"encoding/json"

	"golang.org/x/time/rate"

	"github.com/grafana/alerting/definition"
)

func PostableAPIReceiverToAPIReceiver(r *definition.PostableApiReceiver) *APIReceiver {
	integrations := GrafanaIntegrations{
		Integrations: make([]*GrafanaIntegrationConfig, 0, len(r.GrafanaManagedReceivers)),
	}
	for _, p := range r.GrafanaManagedReceivers {
		var rl *RateLimits
		if r.RateLimits != nil {
			rl = &RateLimits{
				RateLimit: rate.Limit(r.RateLimits.Limit),
				Burst:     r.RateLimits.Burst,
			}
		}
		integrations.Integrations = append(integrations.Integrations, &GrafanaIntegrationConfig{
			UID:                   p.UID,
			Name:                  p.Name,
			Type:                  p.Type,
			DisableResolveMessage: p.DisableResolveMessage,
			Settings:              json.RawMessage(p.Settings),
			SecureSettings:        p.SecureSettings,
			RateLimits:            rl,
		})
	}

	return &APIReceiver{
		ConfigReceiver:      r.Receiver,
		GrafanaIntegrations: integrations,
	}
}

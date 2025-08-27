package notify

import (
	"encoding/json"

	"golang.org/x/time/rate"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/templates"
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

// PostableAPITemplateToTemplateDefinition converts a definition.PostableApiTemplate to a templates.TemplateDefinition
func PostableAPITemplateToTemplateDefinition(t definition.PostableApiTemplate) templates.TemplateDefinition {
	var kind templates.Kind
	switch t.Kind {
	case definition.GrafanaTemplateKind:
		kind = templates.GrafanaKind
	case definition.MimirTemplateKind:
		kind = templates.MimirKind
	}
	d := templates.TemplateDefinition{
		Name:     t.Name,
		Template: t.Content,
		Kind:     kind,
	}
	return d
}

func PostableAPITemplatesToTemplateDefinitions(ts []definition.PostableApiTemplate) []templates.TemplateDefinition {
	defs := make([]templates.TemplateDefinition, 0, len(ts))
	for _, t := range ts {
		defs = append(defs, PostableAPITemplateToTemplateDefinition(t))
	}
	return defs
}

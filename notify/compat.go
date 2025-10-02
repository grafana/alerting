package notify

import (
	"encoding/json"
	"reflect"
	"slices"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/templates"
)

func PostableAPIReceiversToAPIReceivers(r []*definition.PostableApiReceiver) []*APIReceiver {
	result := make([]*APIReceiver, 0, len(r))
	for _, receiver := range r {
		result = append(result, PostableAPIReceiverToAPIReceiver(receiver))
	}
	return result
}

func PostableAPIReceiverToAPIReceiver(r *definition.PostableApiReceiver) *APIReceiver {
	integrations := models.ReceiverConfig{
		Integrations: make([]*models.IntegrationConfig, 0, len(r.GrafanaManagedReceivers)),
	}
	for _, p := range r.GrafanaManagedReceivers {
		integrations.Integrations = append(integrations.Integrations, PostableGrafanaReceiverToIntegrationConfig(p))
	}

	return &APIReceiver{
		ConfigReceiver: r.Receiver,
		ReceiverConfig: integrations,
	}
}

func PostableGrafanaReceiverToIntegrationConfig(r *definition.PostableGrafanaReceiver) *models.IntegrationConfig {
	return &models.IntegrationConfig{
		UID:                   r.UID,
		Name:                  r.Name,
		Type:                  r.Type,
		DisableResolveMessage: r.DisableResolveMessage,
		Settings:              json.RawMessage(r.Settings),
		SecureSettings:        r.SecureSettings,
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

// ConfigReceiverToIntegrations converts a ConfigReceiver to a list of MimirIntegrationConfig
func ConfigReceiverToIntegrations(receiver ConfigReceiver) ([]MimirIntegrationConfig, error) {
	result := make([]MimirIntegrationConfig, 0)
	receiverVal := reflect.ValueOf(&receiver).Elem()
	receiverType := receiverVal.Type()
	for i := 0; i < receiverType.NumField(); i++ {
		integrationField := receiverType.Field(i)
		if integrationField.Type.Kind() != reflect.Slice {
			continue
		}
		sliceType := integrationField.Type
		elemType := sliceType.Elem()
		sliceVal := receiverVal.Field(i)
		if sliceVal.Len() == 0 {
			continue
		}
		iType, err := IntegrationTypeFromMimirTypeReflect(elemType)
		if err != nil {
			return nil, err
		}
		slices.Grow(result, sliceVal.Len())
		for j := 0; j < sliceVal.Len(); j++ {
			elem := sliceVal.Index(j).Elem().Interface()
			result = append(result, MimirIntegrationConfig{
				Type:   iType,
				Config: elem,
			})
		}
	}
	return result, nil
}

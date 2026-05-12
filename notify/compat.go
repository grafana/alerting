package notify

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"slices"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

func PostableAPIReceiversToAPIReceivers(r []*definition.PostableApiReceiver) []models.ReceiverConfig {
	result := make([]models.ReceiverConfig, 0, len(r))
	for _, receiver := range r {
		result = append(result, PostableAPIReceiverToAPIReceiver(receiver))
	}
	return result
}

func PostableAPIReceiverToReceiverConfig(r *definition.PostableApiReceiver) models.ReceiverConfig {
	result := models.ReceiverConfig{
		Name:         r.Name,
		Integrations: make([]*models.IntegrationConfig, 0, len(r.GrafanaManagedReceivers)),
	}
	for _, p := range r.GrafanaManagedReceivers {
		result.Integrations = append(result.Integrations, PostableGrafanaReceiverToIntegrationConfig(p))
	}
	return result
}

func PostableGrafanaReceiverToIntegrationConfig(r *definition.PostableGrafanaReceiver) *models.IntegrationConfig {
	version := schema.V1
	if r.Version != "" {
		version = schema.Version(r.Version)
	}
	return &models.IntegrationConfig{
		UID:                   r.UID,
		Name:                  r.Name,
		Type:                  schema.IntegrationType(r.Type), // TODO validate type/version here
		Version:               version,
		DisableResolveMessage: r.DisableResolveMessage,
		Settings:              json.RawMessage(r.Settings),
		SecureSettings:        r.SecureSettings,
	}
}

// PostableMimirReceiverToPostableGrafanaReceiver converts all legacy models to apimodels.PostableGrafanaReceiver.
// If receiver does not have any legacy receivers, returns the original receiver.
// Otherwise, returns a copy that contains converted integrations (and shallow copy of existing Grafana integrations).
func PostableMimirReceiverToPostableGrafanaReceiver(r *definition.PostableApiReceiver) (*definition.PostableApiReceiver, error) {
	if !r.HasMimirIntegrations() {
		return r, nil
	}
	v0, err := ConfigReceiverToMimirIntegrations(r.Receiver)
	if err != nil {
		return nil, fmt.Errorf("failed to convert v0 receiver to integrations: %w", err)
	}
	result := &definition.PostableApiReceiver{
		Receiver: definition.Receiver{
			Name: r.Name,
		},
		PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
			GrafanaManagedReceivers: make([]*definition.PostableGrafanaReceiver, 0, len(v0)+len(r.GrafanaManagedReceivers)),
		},
	}
	result.GrafanaManagedReceivers = append(result.GrafanaManagedReceivers, r.GrafanaManagedReceivers...)
	typeCount := make(map[string]int)
	for _, config := range v0 {
		integrationType := string(config.Schema.Type())
		idx := typeCount[integrationType]
		typeCount[integrationType]++
		integration, err := MimirIntegrationConfigToPostableGrafanaReceiver(config, r.Name, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Mimir integration config to PostableGrafanaReceiver: %w", err)
		}
		result.GrafanaManagedReceivers = append(result.GrafanaManagedReceivers, integration)
	}
	return result, nil
}

// MimirIntegrationConfigToPostableGrafanaReceiver Converts a Mimir integration configuration to a PostableGrafanaReceiver. All settings are unencrypted. Needs to be encrypted later.
func MimirIntegrationConfigToPostableGrafanaReceiver(config MimirIntegrationConfig, receiverName string, idx int) (*definition.PostableGrafanaReceiver, error) {
	raw, err := config.ConfigJSON()
	if err != nil {
		return nil, err
	}

	return &definition.PostableGrafanaReceiver{
		// mimirIntegrationUID generates a stable, fixed-length UID for a converted Mimir integration that passes ValidateUID, 40-char limit for long names in particular
		UID:                   mimirIntegrationUID(receiverName, string(config.Schema.Type()), idx),
		Name:                  receiverName,
		Type:                  string(config.Schema.Type()),
		Version:               string(config.Schema.Version),
		DisableResolveMessage: false, // V0 ignore this flag as they have their own SendResolved one.
		Settings:              raw,
		SecureSettings:        nil,
	}, nil
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

// ConfigReceiverToMimirIntegrations converts a ConfigReceiver to a list of MimirIntegrationConfig
func ConfigReceiverToMimirIntegrations(receiver ConfigReceiver) ([]MimirIntegrationConfig, error) {
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

		sch, ok := GetSchemaForIntegration(iType)
		if !ok {
			return nil, fmt.Errorf("cannot find schema by integration type %s", iType)
		}
		var version schema.IntegrationSchemaVersion
		if sch.Type == iType {
			version, ok = sch.GetVersion(schema.V0mimir1)
			if !ok {
				return nil, fmt.Errorf(" integration type %s does not have version %s", iType, schema.V0mimir1)
			}
		} else {
			version, ok = sch.GetVersionByTypeAlias(iType)
			if !ok {
				return nil, fmt.Errorf("cannot find schema version by integration type alias %s", iType)
			}
		}
		result = slices.Grow(result, sliceVal.Len())
		for j := 0; j < sliceVal.Len(); j++ {
			var elem any
			item := sliceVal.Index(j)
			if elemType.Kind() == reflect.Ptr {
				elem = item.Elem().Interface()
			} else {
				elem = item.Interface()
			}
			result = append(result, MimirIntegrationConfig{
				Schema: version,
				Config: elem,
			})
		}
	}
	return result, nil
}

func mimirIntegrationUID(receiverName string, integrationType string, idx int) string {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s-%s-%d", receiverName, integrationType, idx)
	return fmt.Sprintf("%016x", h.Sum64())
}

package ingest

import (
	"context"
	"text/template"

	amv2 "github.com/prometheus/alertmanager/api/v2/models"
)

// IngestFunc parses a raw webhook payload and returns PostableAlerts
// ready for submission to Alertmanager. The plugin handles all
// payload parsing, default field mapping, and label application.
//
// The framework provides per-instance label configuration:
//   - staticLabels: fixed key-value pairs to attach to every alert
//   - labelTemplates: Go templates keyed by label name, to be evaluated
//     by the plugin against the parsed payload
//
// The plugin is responsible for merging these into the final alert
// labels alongside any plugin-derived labels.
type IngestFunc func(
	ctx context.Context,
	payload []byte,
	staticLabels map[string]string,
	labelTemplates map[string]*template.Template,
) ([]amv2.PostableAlert, error)

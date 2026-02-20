package models

const (
	RuleUIDLabel      = "__alert_rule_uid__"
	NamespaceUIDLabel = "__alert_rule_namespace_uid__"

	// Annotations are actually a set of labels, so technically this is the label name of an annotation.
	DashboardUIDAnnotation = "__dashboardUid__"
	PanelIDAnnotation      = "__panelId__"
	OrgIDAnnotation        = "__orgId__"

	// This isn't a hard-coded secret token, hence the nolint.
	//nolint:gosec
	ImageTokenAnnotation = "__alertImageToken__"

	// ImageURLAnnotation is the annotation that will contain the URL of an alert's image.
	ImageURLAnnotation = "__alert_image_url__"

	// GrafanaReservedLabelPrefix contains the prefix for Grafana reserved labels. These differ from "__<label>__" labels
	// in that they are not meant for internal-use only and will be passed-through to AMs and available to users in the same
	// way as manually configured labels.
	GrafanaReservedLabelPrefix = "grafana_"

	// FolderTitleLabel is the label that will contain the title of an alert's folder/namespace.
	FolderTitleLabel = GrafanaReservedLabelPrefix + "folder"

	// StateReasonAnnotation is the name of the annotation that explains the difference between evaluation state and alert state (i.e. changing state when NoData or Error).
	StateReasonAnnotation = GrafanaReservedLabelPrefix + "state_reason"

	ValuesAnnotation      = "__values__"
	ValueStringAnnotation = "__value_string__"
)

// filterAlertmanagerKV returns true if a label or annotation should be excluded
// because its name or value is empty.
func filterAlertmanagerKV(name, value string) bool {
	return name == "" || value == ""
}

// FilterAlertmanagerLabel returns true if the label should be excluded from
// Alertmanager payloads. Labels are filtered when the name is empty,
// the value is empty, or the name matches NamespaceUIDLabel.
func FilterAlertmanagerLabel(name, value string) bool {
	return filterAlertmanagerKV(name, value) || name == NamespaceUIDLabel
}

// FilterAlertmanagerAnnotation returns true if the annotation should be
// excluded from Alertmanager payloads. Annotations are filtered when the
// name or value is empty.
func FilterAlertmanagerAnnotation(name, value string) bool {
	return filterAlertmanagerKV(name, value)
}

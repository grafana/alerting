package ingest

// Standard label and annotation keys for use by Ingester implementations
// and instance config templates.
const (
	// LabelAlertname is the standard Prometheus label for the alert name.
	LabelAlertname = "alertname"

	// LabelTeam is a conventional label for team-based routing.
	LabelTeam = "team"

	// AnnotationMessage is the Grafana convention for alert description text.
	AnnotationMessage = "message"

	// AnnotationImageURL is the reserved Grafana annotation for alert images.
	AnnotationImageURL = "__alert_image_url__"
)

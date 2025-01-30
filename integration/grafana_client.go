package integration

import (
	"fmt"
	"net/url"

	gapi "github.com/grafana/grafana-api-golang-client"
)

type GrafanaClient struct {
	*gapi.Client

	// Alertmanager *amapi.AlertmanagerAPI
}

// NewGrafanaClient creates a client for using the Grafana API. Note we don't bother
// wrapping the client library, and just use it as-is, until we find a reason not to.
func NewGrafanaClient(host string, orgID int64) (*GrafanaClient, error) {
	cfg := gapi.Config{
		BasicAuth: url.UserPassword("admin", "admin"),
		OrgID:     orgID,
		HTTPHeaders: map[string]string{
			"X-Disable-Provenance": "true",
		},
	}

	client, err := gapi.New(fmt.Sprintf("http://%s/", host), cfg)
	if err != nil {
		return nil, err
	}

	return &GrafanaClient{
		Client: client,
		// Alertmanager: amapi.New(transport, nil),
	}, nil
}

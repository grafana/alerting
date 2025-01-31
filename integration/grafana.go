package integration

import (
	_ "embed"
	"fmt"
	"net/url"
	"os"

	"github.com/grafana/e2e"
	gapi "github.com/grafana/grafana-api-golang-client"
	"gopkg.in/yaml.v3"
)

//go:embed grafana-versions.yaml
var grafanaVersionsJson []byte

const (
	defaultGrafanaImage = "grafana/grafana-enterprise-dev:10.1.0-58862pre"
	grafanaBinary       = "/run.sh"
	grafanaHTTPPort     = 3000
)

// GetDefaultImage returns the Docker image to use to run the Grafana..
func GetGrafanaImage() string {
	if img := os.Getenv("GRAFANA_IMAGE"); img != "" {
		return img
	}

	if version := os.Getenv("GRAFANA_VERSION"); version != "" {
		var versions map[string]string
		if err := yaml.Unmarshal(grafanaVersionsJson, &versions); err != nil {
			panic(err)
		}

		tag, ok := versions[version]
		if !ok {
			panic("Unknown GRAFANA_VERSION: " + version)
		}

		return "grafana/grafana-enterprise-dev:" + tag
	}

	panic("Provide GRAFANA_VERSION or GRAFANA_IMAGE")
}

type GrafanaService struct {
	*e2e.HTTPService
}

func NewGrafanaService(name string, flags, envVars map[string]string) *GrafanaService {
	svc := &GrafanaService{
		HTTPService: e2e.NewHTTPService(
			name,
			GetGrafanaImage(),
			e2e.NewCommandWithoutEntrypoint(grafanaBinary, e2e.BuildArgs(flags)...),
			e2e.NewHTTPReadinessProbe(grafanaHTTPPort, "/ready", 200, 299),
			grafanaHTTPPort,
			9094),
	}

	svc.SetEnvVars(envVars)

	return svc
}

type GrafanaClient struct {
	*gapi.Client
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
	}, nil
}

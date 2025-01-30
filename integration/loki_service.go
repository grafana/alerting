package integration

import (
	_ "embed"
	"os"

	"github.com/grafana/e2e"
)

const (
	defaultLokiImage = "grafana/loki:latest"
	lokiBinary       = "/usr/bin/loki"
	lokiHTTPPort     = 3100
)

// GetDefaultImage returns the Docker image to use to run the Loki..
func GetLokiImage() string {
	if img := os.Getenv("LOKI_IMAGE"); img != "" {
		return img
	}

	return defaultLokiImage
}

type LokiService struct {
	*e2e.HTTPService
}

func NewLokiService(name string, flags, envVars map[string]string) *LokiService {
	svc := &LokiService{
		HTTPService: e2e.NewHTTPService(
			name,
			GetLokiImage(),
			e2e.NewCommandWithoutEntrypoint(lokiBinary, e2e.BuildArgs(flags)...),
			e2e.NewHTTPReadinessProbe(lokiHTTPPort, "/ready", 200, 299),
			lokiHTTPPort),
	}

	svc.SetEnvVars(envVars)

	return svc
}

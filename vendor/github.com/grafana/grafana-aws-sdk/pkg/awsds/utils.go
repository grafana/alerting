package awsds

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/grafana/grafana-plugin-sdk-go/build"
)

// GetUserAgentString returns an agent that can be parsed in server logs
func GetUserAgentString(name string) string {
	// Build info is set from compile time flags
	buildInfo, err := build.GetBuildInfo()
	if err != nil {
		buildInfo.Version = "dev"
		buildInfo.Hash = "?"
	}

	if len(buildInfo.Hash) > 8 {
		buildInfo.Hash = buildInfo.Hash[0:8]
	}

	grafanaVersion := os.Getenv("GF_VERSION")
	if grafanaVersion == "" {
		grafanaVersion = "?"
	}

	return fmt.Sprintf("%s/%s (%s; %s;) %s/%s-%s Grafana/%s",
		aws.SDKName,
		aws.SDKVersion,
		runtime.Version(),
		runtime.GOOS,
		name,
		buildInfo.Version,
		buildInfo.Hash,
		grafanaVersion)
}

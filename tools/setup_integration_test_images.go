package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/grafana/alerting/integration"
)

func docker(args []string) {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("docker pull failed: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	var wg sync.WaitGroup

	for _, cmd := range [][]string{
		{"pull", integration.GetGrafanaImage()},
		{"pull", integration.GetLokiImage()},
		{"pull", integration.GetPostgresImage()},
		{"build", "-t", "webhook-receiver", "integration/webhook"},
	} {
		wg.Add(1)

		go func(cmd []string) {
			defer wg.Done()

			docker(cmd)
		}(cmd)
	}

	wg.Wait()
}

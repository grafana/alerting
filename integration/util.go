package integration

import (
	"os"
	"strconv"
)

const (
	defaultNetworkName = "e2e-grafana-am"
)

func getNetworkName() string {
	// If the E2E_NETWORK_NAME is set, use that for the network name.
	// Otherwise, return the default network name.
	if os.Getenv("E2E_NETWORK_NAME") != "" {
		return os.Getenv("E2E_NETWORK_NAME")
	}

	return defaultNetworkName
}

func getInstances(n int) []string {
	is := make([]string, n)

	for i := 0; i < n; i++ {
		is[i] = "grafana-" + strconv.Itoa(i+1)
	}

	return is
}

func getPeers(i string, is []string) []string {
	peers := make([]string, 0, len(is)-1)

	for _, p := range is {
		if p != i {
			peers = append(peers, p+":9094")
		}
	}

	return peers
}

func mapInstancePeers(is []string) map[string][]string {
	mIs := make(map[string][]string, len(is))

	for _, i := range is {
		mIs[i] = getPeers(i, is)
	}

	return mIs
}

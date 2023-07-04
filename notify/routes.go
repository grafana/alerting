package notify

import "github.com/prometheus/alertmanager/config"

type Route = config.Route

// AllReceivers will recursively walk a routing tree and return the set of all the
// referenced receiver names.
func AllReceivers(route *Route) map[string]struct{} {
	return allReceivers(route, nil)
}

func allReceivers(route *Route, res map[string]struct{}) map[string]struct{} {
	if res == nil {
		res = make(map[string]struct{})
	}
	if route == nil {
		return res
	}

	if route.Receiver != "" {
		res[route.Receiver] = struct{}{}
	}

	for _, subRoute := range route.Routes {
		allReceivers(subRoute, res)
	}
	return res
}

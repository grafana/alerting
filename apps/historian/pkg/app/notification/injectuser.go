package notification

import (
	"errors"
	"fmt"
	"net/http"

	authtypes "github.com/grafana/authlib/types"
)

// injectUserTransport sets the basic auth username of each request to the stack
// ID of the caller's namespace, and keeps its password.
type injectUserTransport struct {
	next http.RoundTripper
}

func newInjectUserTransport(next http.RoundTripper) http.RoundTripper {
	return &injectUserTransport{next: next}
}

func (t *injectUserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	info, ok := authtypes.AuthInfoFrom(req.Context())
	if !ok {
		return nil, errors.New("auth info missing from context")
	}
	namespace := info.GetNamespace()
	namespaceInfo, err := authtypes.ParseNamespace(namespace)
	if err != nil {
		return nil, fmt.Errorf("parse namespace %q: %w", namespace, err)
	}
	if namespaceInfo.StackID <= 0 {
		return nil, fmt.Errorf("stack id required: %q", namespace)
	}

	// A transport must not modify the caller's request.
	req = req.Clone(req.Context())
	_, password, _ := req.BasicAuth()
	req.SetBasicAuth(fmt.Sprintf("%d", namespaceInfo.StackID), password)
	return t.next.RoundTrip(req)
}

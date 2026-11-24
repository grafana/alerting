package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	authtypes "github.com/grafana/authlib/types"
	"github.com/grafana/grafana-app-sdk/app"
	"github.com/grafana/grafana-app-sdk/logging"
	"github.com/grafana/grafana-app-sdk/resource"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"k8s.io/client-go/rest"

	"github.com/grafana/alerting/notify/historian/lokiclient"

	"github.com/grafana/alerting/apps/historian/pkg/app/config"
)

// namespaceAuthInfo is an AuthInfo with only a namespace.
type namespaceAuthInfo struct {
	authtypes.AuthInfo
	namespace string
}

func (a namespaceAuthInfo) GetNamespace() string { return a.namespace }

func withNamespace(ctx context.Context, namespace string) context.Context {
	return authtypes.WithAuthInfo(ctx, namespaceAuthInfo{namespace: namespace})
}

type captureTransport struct {
	got *http.Request
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	return &http.Response{StatusCode: http.StatusOK}, nil
}

func TestInjectUserTransport(t *testing.T) {
	send := func(t *testing.T, ctx context.Context) (*captureTransport, *http.Request, error) {
		t.Helper()
		next := &captureTransport{}
		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		req.SetBasicAuth("configured", "mypassword")
		//nolint:bodyclose // The response has no body.
		_, err := newInjectUserTransport(next).RoundTrip(req)
		return next, req, err
	}

	t.Run("sets the stack ID as the username and keeps the password", func(t *testing.T) {
		next, req, err := send(t, withNamespace(context.Background(), "stacks-123"))
		require.NoError(t, err)

		user, password, ok := next.got.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "123", user)
		require.Equal(t, "mypassword", password)

		// The caller's request is left as it was.
		user, _, _ = req.BasicAuth()
		require.Equal(t, "configured", user)
	})

	t.Run("fails without auth info", func(t *testing.T) {
		next, _, err := send(t, context.Background())
		require.ErrorContains(t, err, "auth info missing from context")
		require.Nil(t, next.got)
	})

	for _, namespace := range []string{"default", "org-2", "*", "invalid-namespace", "stacks-0"} {
		t.Run("fails for the namespace "+namespace, func(t *testing.T) {
			next, _, err := send(t, withNamespace(context.Background(), namespace))
			require.Error(t, err)
			require.Nil(t, next.got)
		})
	}
}

// fakeLoki records the basic auth credentials of the requests it gets.
type fakeLoki struct {
	mu    sync.Mutex
	users []string
	pass  []string
}

func (f *fakeLoki) handler(w http.ResponseWriter, r *http.Request) {
	user, password, _ := r.BasicAuth()
	f.mu.Lock()
	f.users = append(f.users, user)
	f.pass = append(f.pass, password)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[]}}`))
}

func TestHandlersInjectUser(t *testing.T) {
	newNotification := func(t *testing.T, injectUser bool) (*Notification, *fakeLoki) {
		t.Helper()
		loki := &fakeLoki{}
		server := httptest.NewServer(http.HandlerFunc(loki.handler))
		t.Cleanup(server.Close)
		u, err := url.Parse(server.URL)
		require.NoError(t, err)

		n := New(config.NotificationConfig{
			Enabled: true,
			Loki: config.LokiConfig{
				LokiConfig: lokiclient.LokiConfig{
					ReadPathURL:       u,
					BasicAuthUser:     "configured",
					BasicAuthPassword: "mypassword",
					MaxQuerySize:      65536,
				},
				InjectUser: injectUser,
			},
		}, rest.Config{}, prometheus.NewRegistry(), logging.DefaultLogger, otel.GetTracerProvider().Tracer("test"))
		return n, loki
	}

	run := func(h handlerFn, namespace string, body any) error {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req := &app.CustomRouteRequest{
			ResourceIdentifier: resource.FullIdentifier{Namespace: namespace},
			Body:               io.NopCloser(bytes.NewReader(raw)),
		}
		return h(withNamespace(context.Background(), namespace), httptest.NewRecorder(), req)
	}

	handlers := map[string]func(*Notification) (handlerFn, any){
		"notification query": func(n *Notification) (handlerFn, any) { return n.QueryHandler, Query{} },
		"alerts query":       func(n *Notification) (handlerFn, any) { return n.QueryAlertsHandler, AlertQuery{} },
	}
	for name, pick := range handlers {
		t.Run(name+" sends the namespace's stack ID to Loki when asked to", func(t *testing.T) {
			n, loki := newNotification(t, true)
			h, body := pick(n)
			require.NoError(t, run(h, "stacks-42", body))

			require.NotEmpty(t, loki.users)
			for i := range loki.users {
				require.Equal(t, "42", loki.users[i])
				require.Equal(t, "mypassword", loki.pass[i])
			}
		})

		t.Run(name+" fails before reaching Loki for a namespace that is not a stack", func(t *testing.T) {
			n, loki := newNotification(t, true)
			h, body := pick(n)
			require.Error(t, run(h, "default", body))
			require.Empty(t, loki.users)
		})

		t.Run(name+" sends the configured credentials otherwise", func(t *testing.T) {
			n, loki := newNotification(t, false)
			h, body := pick(n)
			require.NoError(t, run(h, "default", body))

			require.NotEmpty(t, loki.users)
			for i := range loki.users {
				require.Equal(t, "configured", loki.users[i])
				require.Equal(t, "mypassword", loki.pass[i])
			}
		})
	}
}

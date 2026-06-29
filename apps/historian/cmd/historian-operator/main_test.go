package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain_OperatorComesUp(t *testing.T) {
	// The webhook server needs TLS (the app has custom routes), and the metrics
	// server hosts the /readyz health endpoint on its own port over plain HTTP.
	ports := freePorts(t, 2)
	webhookPort, metricsPort := ports[0], ports[1]
	certPath, keyPath := writeTestCert(t)

	done := make(chan error, 1)
	go func() {
		done <- Main([]string{
			fmt.Sprintf("--webhook.port=%d", webhookPort),
			"--webhook.tls.cert-path=" + certPath,
			"--webhook.tls.key-path=" + keyPath,
			fmt.Sprintf("--metrics.port=%d", metricsPort),
			// Enable the notification API so the /notification/query route is served.
			// The Loki URL is never reached: we send a malformed body, which the
			// handler rejects before contacting Loki.
			"--alerting.historian.notification.enabled=true",
			"--alerting.historian.notification.loki.read-url=http://127.0.0.1:1/",
		})
	}()

	// Poll the readiness endpoint until it reports OK; a 200 proves the operator
	// came up and its runner is serving.
	client := &http.Client{Timeout: time.Second}
	readyz := fmt.Sprintf("http://127.0.0.1:%d/readyz", metricsPort)
	require.Eventually(t, func() bool {
		resp, err := client.Get(readyz)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond, "operator readiness endpoint never reported OK")

	// Hit the /notification/query custom route on the webhook server (TLS). A
	// malformed body proves the route is wired through to the app's handler: it
	// returns 400 before any Loki interaction.
	tlsClient := &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	queryURL := fmt.Sprintf(
		"https://127.0.0.1:%d/apis/historian.alerting.grafana.app/v0alpha1/namespaces/default/notification/query",
		webhookPort,
	)
	var resp *http.Response
	require.Eventually(t, func() bool {
		var err error
		resp, err = tlsClient.Post(queryURL, "application/json", strings.NewReader("not json"))
		return err == nil
	}, 10*time.Second, 50*time.Millisecond, "notification/query endpoint never responded")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()

	// Trigger Main's signal-based shutdown and confirm a clean exit.
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("operator did not shut down after SIGINT")
	}
}

// freePorts returns n distinct free TCP ports. All listeners are held open
// simultaneously so the kernel hands out distinct ports, then closed before
// returning. There is still a small window before the caller binds them, but
// the ports are guaranteed not to collide with each other.
func freePorts(t *testing.T, n int) []int {
	t.Helper()
	listeners := make([]net.Listener, 0, n)
	ports := make([]int, 0, n)
	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	for _, l := range listeners {
		_ = l.Close()
	}
	return ports
}

func writeTestCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}

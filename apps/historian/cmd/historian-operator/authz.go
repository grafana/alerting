package main

import (
	"context"
	"fmt"

	authnlib "github.com/grafana/authlib/authn"
	authzlib "github.com/grafana/authlib/authz"
	authtypes "github.com/grafana/authlib/types"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// authzServiceAudience is the audience requested when exchanging a service token
// for a call to the authz service. It matches the value Grafana uses.
const authzServiceAudience = "authzService"

// authzConfig configures the connection to the multi-tenant authz service used
// to authorize alert.rules:read per folder for notification-history RBAC. It is
// only required when notification RBAC is enabled.
type authzConfig struct {
	// RemoteAddress is the host:port of the authz gRPC service.
	RemoteAddress string
	// Token is the service token presented to the token-exchange endpoint.
	Token string
	// TokenExchangeURL is the endpoint that exchanges Token for a scoped access
	// token accepted by the authz service.
	TokenExchangeURL string
	// TokenNamespace is the namespace claimed when exchanging tokens (e.g. the
	// stack namespace "stacks-<id>", or "*" to act across namespaces).
	TokenNamespace string
	// CertFile, when set, enables TLS to the authz service using this CA/cert
	// file. When empty the connection uses insecure transport credentials.
	CertFile string
}

func (c *authzConfig) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.RemoteAddress, "authz.remote-address", "", "host:port of the authz gRPC service (required when notification RBAC is enabled)")
	flags.StringVar(&c.Token, "authz.token", "", "Service token used to perform the token exchange for authz calls")
	flags.StringVar(&c.TokenExchangeURL, "authz.token-exchange-url", "", "URL of the token-exchange endpoint")
	flags.StringVar(&c.TokenNamespace, "authz.token-namespace", "*", "Namespace claimed when exchanging tokens for authz calls")
	flags.StringVar(&c.CertFile, "authz.tls.cert-path", "", "Path to a TLS CA/cert file for the authz connection; empty uses insecure transport")
}

// newAccessClient builds an authz-backed AccessClient using service-to-service
// token exchange, mirroring how Grafana constructs its remote RBAC client.
func newAccessClient(cfg authzConfig) (authtypes.AccessClient, error) {
	if cfg.RemoteAddress == "" {
		return nil, fmt.Errorf("authz.remote-address is required when notification RBAC is enabled")
	}
	if cfg.Token == "" || cfg.TokenExchangeURL == "" {
		return nil, fmt.Errorf("authz.token and authz.token-exchange-url are required when notification RBAC is enabled")
	}

	tokenClient, err := authnlib.NewTokenExchangeClient(authnlib.TokenExchangeConfig{
		Token:            cfg.Token,
		TokenExchangeURL: cfg.TokenExchangeURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token exchange client: %w", err)
	}

	transportCreds := insecure.NewCredentials()
	if cfg.CertFile != "" {
		transportCreds, err = credentials.NewClientTLSFromFile(cfg.CertFile, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load authz TLS credentials: %w", err)
		}
	}

	conn, err := grpc.NewClient(
		cfg.RemoteAddress,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithPerRPCCredentials(newTokenAuth(authzServiceAudience, cfg.TokenNamespace, tokenClient)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create authz client connection: %w", err)
	}

	return authzlib.NewClient(conn), nil
}

// tokenAuth is a gRPC PerRPCCredentials that attaches a freshly exchanged access
// token to each request as the X-Access-Token header.
type tokenAuth struct {
	audience    string
	namespace   string
	tokenClient authnlib.TokenExchanger
}

func newTokenAuth(audience, namespace string, tc authnlib.TokenExchanger) *tokenAuth {
	return &tokenAuth{audience: audience, namespace: namespace, tokenClient: tc}
}

func (t *tokenAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	token, err := t.tokenClient.Exchange(ctx, authnlib.TokenExchangeRequest{
		Namespace: t.namespace,
		Audiences: []string{t.audience},
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"X-Access-Token": token.Token}, nil
}

func (t *tokenAuth) RequireTransportSecurity() bool { return false }

package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	commoncfg "github.com/prometheus/common/config"

	"github.com/grafana/alerting/receivers"
)

var ErrInvalidMethod = errors.New("webhook only supports HTTP methods PUT or POST")

// ErrWebhookRateLimited is returned from SendWebhook when a rate limiter rejects the call.
// Mirrors ErrEmailRateLimited semantics: reject immediately, do not retry.
var ErrWebhookRateLimited = errors.New("webhook notifications are rate limited")

type clientConfiguration struct {
	userAgent    string
	dialer       net.Dialer // We use Dialer here instead of DialContext as our mqtt client doesn't support DialContext.
	customDialer bool
	// rateLimiter, when non-nil, is consulted once per SendWebhook call.
	// The caller owns the limiter and may mutate its rate/burst at runtime.
	rateLimiter *rate.Limiter
}

// defaultDialTimeout is the default timeout for the dialer, 30 seconds to match http.DefaultTransport.
const defaultDialTimeout = 30 * time.Second

type Client struct {
	cfg               clientConfiguration
	oauth2TokenSource oauth2.TokenSource
}

// NewClient builds a Client for a specific integration type. The integrationType is passed
// through to each ClientOption so options can vary their behavior per integration (e.g.
// WithRateLimiterByType selects a limiter from a per-type map). Pass "" for standalone
// clients that are not tied to a particular integration.
func NewClient(httpClientConfig *HTTPClientConfig, integrationType string, opts ...ClientOption) (*Client, error) {
	cfg := clientConfiguration{
		userAgent: "Grafana",
		dialer:    net.Dialer{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg, integrationType)
		}
	}
	if cfg.dialer.Timeout == 0 {
		// Mostly defensive to ensure that timeout semantics don't change when given a custom dialer without a timeout.
		cfg.dialer.Timeout = defaultDialTimeout
	}

	client := &Client{
		cfg: cfg,
	}

	if httpClientConfig != nil && httpClientConfig.OAuth2 != nil {
		if err := ValidateOAuth2Config(httpClientConfig.OAuth2); err != nil {
			return nil, fmt.Errorf("invalid OAuth2 configuration: %w", err)
		}
		// If the user has provided an OAuth2 config, we need to prepare the OAuth2 token source. This needs to
		// be stored outside of the request so that the token expiration/re-use will work as expected.
		tokenSource, err := NewOAuth2TokenSource(*httpClientConfig.OAuth2, cfg)
		if err != nil {
			return nil, err
		}
		client.oauth2TokenSource = tokenSource
	}

	return client, nil
}

// ClientOption configures a Client. The second argument is the integration type the
// Client is being built for (e.g. "slack", "webhook"); most options ignore it, but
// options like WithRateLimiterByType use it to select per-type behavior. For standalone
// clients not tied to a specific integration, NewClient is called with "".
type ClientOption func(*clientConfiguration, string)

func WithUserAgent(userAgent string) ClientOption {
	return func(c *clientConfiguration, _ string) {
		c.userAgent = userAgent
	}
}

func WithDialer(dialer net.Dialer) ClientOption {
	return func(c *clientConfiguration, _ string) {
		c.dialer = dialer
		c.customDialer = true
	}
}

// WithRateLimiter installs a rate limiter on the client unconditionally. Each SendWebhook
// call consumes one token via Allow(); when no token is available the call is rejected
// with ErrWebhookRateLimited without contacting the upstream. A nil limiter disables
// rate limiting (same as not passing this option).
//
// The caller owns the limiter's lifetime and may reconfigure it in place via
// SetLimit/SetBurst. Use WithRateLimiterByType instead when the limiter should be chosen
// per integration type.
func WithRateLimiter(limiter *rate.Limiter) ClientOption {
	return func(c *clientConfiguration, _ string) {
		c.rateLimiter = limiter
	}
}

// WithRateLimiterByType installs a rate limiter chosen from the given map by the Client's
// integration type. When the type has no entry, the option is a no-op and no rate limiting
// is applied to that Client. The typical use is a per-tenant map of limiters (one bucket
// per integration type) that the factory consults when building Clients for each
// receiver — giving a single shared bucket per integration type independent of how many
// receivers reuse the same integration.
//
// The caller owns each limiter's lifetime and may reconfigure it in place via
// SetLimit/SetBurst. To avoid data races, pass a snapshot of the map when the underlying
// source can mutate concurrently (the option retains the reference).
func WithRateLimiterByType(limiters map[string]*rate.Limiter) ClientOption {
	return func(c *clientConfiguration, integrationType string) {
		if lim, ok := limiters[integrationType]; ok && lim != nil {
			c.rateLimiter = lim
		}
	}
}

func ToHTTPClientOption(option ...ClientOption) []commoncfg.HTTPClientOption {
	cfg := clientConfiguration{}
	for _, opt := range option {
		if opt == nil {
			continue
		}
		// No integration type context at conversion time — options that depend on the type
		// have no effect here, which is correct: ToHTTPClientOption converts only to the
		// subset supported by upstream (user-agent, dialer).
		opt(&cfg, "")
	}
	result := make([]commoncfg.HTTPClientOption, 0, len(option))
	if cfg.userAgent != "" {
		result = append(result, commoncfg.WithUserAgent(cfg.userAgent))
	}
	if cfg.customDialer {
		result = append(result, commoncfg.WithDialContextFunc(cfg.dialer.DialContext))
	}
	return result
}

func (ns *Client) SendWebhook(ctx context.Context, l log.Logger, webhook *receivers.SendWebhookSettings) error {
	if ns.cfg.rateLimiter != nil && !ns.cfg.rateLimiter.Allow() {
		return ErrWebhookRateLimited
	}
	// This method was moved from https://github.com/grafana/grafana/blob/71d04a326be9578e2d678f23c1efa61768e0541f/pkg/services/notifications/webhook.go#L38
	if webhook.HTTPMethod == "" {
		webhook.HTTPMethod = http.MethodPost
	}
	level.Debug(l).Log("msg", "sending webhook", "url", webhook.URL, "http method", webhook.HTTPMethod)

	if webhook.HTTPMethod != http.MethodPost && webhook.HTTPMethod != http.MethodPut {
		return fmt.Errorf("%w: %s", ErrInvalidMethod, webhook.HTTPMethod)
	}

	request, err := http.NewRequestWithContext(ctx, webhook.HTTPMethod, webhook.URL, bytes.NewReader([]byte(webhook.Body)))
	if err != nil {
		return err
	}
	url, err := url.Parse(webhook.URL)
	if err != nil {
		// Should not be possible - NewRequestWithContext should also err if the URL is bad.
		return err
	}

	if webhook.ContentType == "" {
		webhook.ContentType = "application/json"
	}

	request.Header.Set("Content-Type", webhook.ContentType)
	request.Header.Set("User-Agent", ns.cfg.userAgent)

	if webhook.User != "" && webhook.Password != "" {
		request.Header.Set("Authorization", GetBasicAuthHeader(webhook.User, webhook.Password))
	}

	for k, v := range webhook.HTTPHeader {
		request.Header.Set(k, v)
	}

	client := NewTLSClient(webhook.TLSConfig, ns.cfg.dialer.DialContext)

	if webhook.HMACConfig != nil {
		level.Debug(l).Log("msg", "Adding HMAC roundtripper to client")
		client.Transport, err = NewHMACRoundTripper(
			client.Transport,
			clock.New(),
			webhook.HMACConfig.Secret,
			webhook.HMACConfig.Header,
			webhook.HMACConfig.TimestampHeader,
		)
		if err != nil {
			level.Error(l).Log("msg", "Failed to add HMAC roundtripper to client", "err", err)
			return err
		}
	}

	if ns.oauth2TokenSource != nil {
		level.Debug(l).Log("msg", "Adding OAuth2 roundtripper to client")
		client.Transport = NewOAuth2RoundTripper(ns.oauth2TokenSource, client.Transport)
	}

	resp, err := client.Do(request)
	if err != nil {
		return redactURL(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			level.Warn(l).Log("msg", "Failed to close response body", "err", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if webhook.Validation != nil {
		err := webhook.Validation(body, resp.StatusCode)
		if err != nil {
			level.Debug(l).Log("msg", "Webhook failed validation", "url", url.Redacted(), "statuscode", resp.Status, "body", string(body), "err", err)
			return fmt.Errorf("webhook failed validation: %w", err)
		}
	}

	if resp.StatusCode/100 == 2 {
		level.Debug(l).Log("msg", "Webhook succeeded", "url", url.Redacted(), "statuscode", resp.Status)
		return nil
	}

	level.Debug(l).Log("msg", "Webhook failed", "url", url.Redacted(), "statuscode", resp.Status, "body", string(body))
	return fmt.Errorf("webhook response status %v", resp.Status)
}

func redactURL(err error) error {
	var e *url.Error
	if !errors.As(err, &e) {
		return err
	}
	e.URL = "<redacted>"
	return e
}

func GetBasicAuthHeader(user string, password string) string {
	var userAndPass = user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(userAndPass))
}

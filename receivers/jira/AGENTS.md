# Jira receiver notes

`v0mimir1` follows the Prometheus Alertmanager Jira notifier at the fork revision pinned by this repository. Keep `jira.go` and `types.go` close to upstream.

Local adaptations are limited to repository boundaries:

- `Config` uses Grafana receiver types and the local `http/v0mimir.HTTPClientConfig`.
- `NewConfig` supports Grafana JSON settings, secret decryption, and validation. `JiraFieldConfig` adds JSON decoding because stored scalar `summary` and `description` values must remain readable. Every re-save writes the object form, including when `enable_update` is unset. Versions of this module before this change cannot decode the object form; consider this before rolling back any consumer.
- `summary` or `description` set to `null` is zeroed by JSON and yaml.v2. yaml.v3 keeps the default.
- The factory converts the go-kit logger with `logging.NewSlogLogger`; the notifier itself uses `*slog.Logger`.
- HTTP client creation preserves the local nil-default, conversion, and validation behavior before calling `notify.NewClientWithTracing`.
- `SendResolved` remains part of the local notification channel interface.
- Directly constructed configs with a nil `api_url` return an error instead of panicking.

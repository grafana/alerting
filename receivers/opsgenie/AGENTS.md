# OpsGenie receiver notes

`v0mimir1` uses responder validation from Prometheus Alertmanager `v0.34.1`
(`73c6bfe7393929211294c1954f30d8ed78e4d0ad`). The Grafana fork revision is
`fc531ea850e603f3a797d30dfb1cc96000d2577a` (grafana/prometheus-alertmanager#194).
Last sync: 2026-11-19 UTC.

## Upstream deviations

- `v0mimir1` shares the responder checks between YAML loading and local
  validation. The checks match upstream, but the YAML method keeps the local
  config type and delegates to `validate`.
- The notifier keeps the local HTTP constructor without upstream tracing.
  The constructor supplies defaults for nil HTTP config and validates the
  converted config.
- The config uses local HTTP, URL, and secret types. It omits `api_key_file`
  because Grafana accepts an inline or decrypted key, not a file reference.

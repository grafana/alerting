# Microsoft Teams receiver notes

`v0mimir2` follows Prometheus Alertmanager `v0.34.1`
(`73c6bfe7393929211294c1954f30d8ed78e4d0ad`). The Grafana fork revision is
`5b378e13e6a9bb109a6e3194016889a754f907dd` (grafana/prometheus-alertmanager#193).
Last sync: 2026-11-19 UTC.

## Upstream deviations

- The `v0mimir2` notifier retains the local `http/v0mimir` client constructor
  instead of the `v0.34.1` traced constructor. The local path supplies defaults
  for nil HTTP config, converts the native config, and validates it.
- The notifier accepts the local config and go-kit logger. It adapts that logger
  for fork template calls and exposes `SendResolved` for the local interface.
- The config uses local HTTP and secret types and omits `WebhookURLFile`.
  Grafana stores the URL as a secure setting, not a file reference.

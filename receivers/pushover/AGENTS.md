# Pushover receiver notes

`v0mimir1` follows Prometheus Alertmanager `v0.34.1`
(`73c6bfe7393929211294c1954f30d8ed78e4d0ad`). The Grafana fork revision is
`a99cad413f34bd37eee0148b6ce0134b19b6e89b` (grafana/prometheus-alertmanager#199).
Last sync: 2026-11-20 UTC.

## Upstream deviations

- The config retains local `FractionalDuration`, secret, and HTTP types. It
  accepts inline or decrypted credentials, not credential file references.
- YAML decoding delegates the formatting conflict check to the same local
  validator that checks stored JSON and direct configs.
- The notifier retains the local HTTP constructor, logger, retry error
  classification, and URL redaction. It does not use upstream HTTP tracing
  because the Pushover URL query contains credentials and message content.

# Alerting

Go library providing core alerting infrastructure for Grafana and Grafana Mimir.
Extension of Prometheus Alertmanager with Grafana-specific features.

## Build & Test

- `make test` - Run all tests (`go test -tags netgo -timeout 30m -race -count 1 ./...`)
- `make lint` - Run misspell + golangci-lint
- `make mod-check` - Verify go.mod/go.sum are tidy
- `make clean` - Remove local tool installations (.tools/)

## Code Style

- Go 1.24, module: `github.com/grafana/alerting`
- Linter: golangci-lint v2 (config in `.golangci.yml`)
- Formatters: gofmt + goimports
- Import groups: (1) stdlib, (2) third-party, (3) internal (`github.com/grafana/alerting/...`)
- Test assertions: use `github.com/stretchr/testify`

## Receivers

Each receiver (slack, pagerduty, email, etc.) lives in `receivers/<channel>/` with versioned integration subdirs:
- `v0mimir1/` and `v0mimir2` - Mimir-compatible (Prometheus Alertmanager-style) config. Also they are called legacy.
- `v1/` - Grafana-native config.

Each integration has a config struct and a separate schema struct that reflects it.
Schema is used to: (1) determine whether a field is secure, (2) render UI components for editing configurations.
Grafana integration config has two inputs: settings and secure settings. Secure settings are encrypted fields of the config struct, resolved via a decryption function.
**When adding a new secure field to a schema, ensure the decryption function is used to read and populate it in the config.**
**When modifying a config struct, always update the corresponding schema to match.**

## Architecture

- `receivers/` - Notification channels with versioned integrations (see above)
- `notify/` - Core alertmanager orchestration (GrafanaAlertmanager, dispatch, stages)
- `definition/` - Alertmanager config structs (Route, Config, Receiver)
- `templates/` - Go template engine for alert notifications, extended with gomplate functions
- `models/` - Shared data models (Alert, Labels, statuses)
- `images/` - Alert screenshot/image provider interface
- `cluster/` - Re-exports of Prometheus Alertmanager cluster types

## Key Patterns

- Receiver configs use a schema versioning system (`receivers/schema/`): V0mimir1, V0mimir2, V1
- Factory pattern in `notify/factory.go` builds receiver integrations by version
- Notification history persisted to Loki via `notify/historian/`
- Uses Grafana fork of Alertmanager: `github.com/grafana/prometheus-alertmanager`

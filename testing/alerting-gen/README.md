# alerting-gen

A tiny CLI to generate random Grafana Alerting rules and Recording rules using:

- Grafana OpenAPI Go models (`github.com/grafana/grafana-openapi-client-go/models`)
- Property-based generators (`pgregory.net/rapid`) for realistic, reproducible data

It outputs a list of valid `AlertingRuleGroup` JSON that you can inspect. Optionally, it can push the generated rules directly to Grafana via the provisioning API.

## Install

```bash
make build
```

## Usage

```bash
# Generate JSON only
./alerting-gen \
  -alerts=10 \
  -recordings=5 \
  -query-ds=__expr__ \
  -write-ds=prom-uid \
  -rules-per-group=5 \
  -groups-per-folder=2 \
  -seed=123456 \
  -out=rules.json

# Generate and also PUT groups via provisioning API
./alerting-gen \
  -alerts=10 \
  -recordings=5 \
  -query-ds=__expr__ \
  -write-ds=prom-uid \
  -rules-per-group=5 \
  -groups-per-folder=2 \
  -seed=123456 \
  -grafana-url=https://grafana.example.com \
  -username=admin \
  -password=admin \
  -org-id=1 \
  -folder-uids=general,prod,staging
```

Flags:

- `-alerts` number of alerting rules to generate
- `-recordings` number of recording rules to generate
- `-query-ds` datasource UID to query from (e.g., `__expr__` or a Prometheus UID)
- `-write-ds` datasource UID to write recording rule metrics to (Prometheus UID)
- `-rules-per-group` how many rules to pack per group
- `-groups-per-folder` how many groups to place per folder before cycling to the next provided `-folder-uids`
- `-seed` seed for deterministic generation; re-use to reproduce the same output
- `-out` output file; if empty, prints JSON to stdout and the seed to stderr
Provisioning flags (optional):

- `-grafana-url` Grafana base URL; when set, the tool will send rule groups via the provisioning API
- `-username` Grafana admin account user name (required with `-grafana-url`)
- `-password` Grafana admin account password (required with `-grafana-url`)
- `-token` Grafana service account token (alternative to username/password; if set it takes precedence and is sent as `Authorization: Bearer <token>`)
- `-org-id` Grafana organization ID to scope requests (defaults to 1)
- `-folder-uids` Comma-separated list of folder UIDs to distribute groups across (default: `general`)

### Notes

- Rules are generated directly as provisioning models (`models.ProvisionedAlertRule`) bundled into `models.AlertRuleGroup`. The same JSON is printed and, if configured, sent to Grafana via the provisioning API.
- Alerting rules include a `condition` and a single query by default (refIDs are local per rule). Recording rules include a `record` block with `from`, `metric`, and `targetDatasourceUID`.
- Annotations always include `summary` and may include 0–4 extra randomized keys (e.g., `runbook_url`, `dashboard`, `description`, `priority`, `owner`, `ticket`).
- Several fields are randomized for realism: `execErrState`, `noDataState`, `isPaused`, `labels`, and `missingSeriesEvalsToResolve`. The `for` duration is randomized, and `keep_firing_for` is guaranteed to be a multiple of `for` (including zero).
- NotificationSettings is currently not populated by the generator. This requires additional type coordination and is out of scope for now. TODO: revisit and add randomized NotificationSettings when stable types are available.
- OrgID is currently set to 1 in generated rules. TODO: make OrgID configurable via flags and generator config.
- The generated queries default to a simple expression (`1 == 1`) for `__expr__`, or a basic PromQL if `-query-ds` is not `__expr__`.
- When `-grafana-url` is provided, the tool will PUT each generated group to `/api/provisioning/folder/{folderUID}/rule-groups/{group}`.
- Recording rules require `-write-ds` to be set to the destination Prometheus datasource UID.
- Authentication precedence: if `-token` is supplied, it is used (Bearer header) and `-username`/`-password` are ignored for requests. If `-token` is empty, basic auth with `-username`/`-password` is used.

## Reproducibility

Pass a fixed `-seed` to reproduce the same rules. The program also prints the seed when writing to stdout.

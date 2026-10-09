# Caller fields at the Alertmanager fork boundary

The fork uses `log/slog`, while Alerting's public logging API uses go-kit.
`NewSlogLogger` adapts between them. A go-kit caller valuer such as
`log.Caller(N)` depends on stack depth, so the additional adapter frames can
make it report an adapter or standard-library caller instead of the fork's
call site. An opaque go-kit logger does not expose its bound fields, and the
adapter cannot safely remove a caller valuer from it.

Consumers that include a caller field should supply a companion logger with
the same timestamps, context, destination, and level filtering, but without
the caller field. Keep the normal logger unchanged for go-kit calls.

For a direct conversion:

```go
forkLogger := logging.NewSlogLogger(
    normalLogger,
    logging.WithCallerlessLogger(callerlessLogger),
)
```

The adapter uses the companion only for fork records and derives one caller
field from `slog.Record.PC`. `WithDebugEnabled` still overrides debug detection;
otherwise detection uses the normal logger.

For a Grafana Alertmanager, set both `GrafanaAlertmanagerOpts.Logger` and
`GrafanaAlertmanagerOpts.CallerlessLogger`. The Alertmanager mirrors its
component and tenant fields onto the companion and propagates it to receiver
construction and template testing.

For a receiver factory, pass `NotifierOpts.SlogLogger` alongside `Logger`:

```go
opts := receivers.NotifierOpts{
    Logger: normalLogger,
    SlogLogger: logging.NewSlogLogger(callerlessLogger, logging.WithCaller()),
}
```

Built-in receivers retain this logger through `ForkLogger`. Directly
constructed receivers can use `SetSlogLogger` before invoking `Notify`.
The notification integration builders, `templates.TmplText`, and
`notify.TestTemplate` also accept an optional final `*slog.Logger` argument.
`cluster.Create` and `notify.NewGrafanaAlertmanagerMetrics` accept final
`logging.Option` arguments, including `WithCallerlessLogger`.

Omitting a companion preserves default adaptation. Consumers without an
existing caller field can continue using `WithCaller` directly. A companion
that itself adds a caller field still produces duplicate fields and must not
be used with either caller option.

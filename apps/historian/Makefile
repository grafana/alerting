.PHONY: build # Build the historian operator
build:
	go build -tags netgo ./cmd/historian-operator

.PHONY: test # Run unit tests
# NOTE: -race is intentionally omitted for now: grafana-app-sdk's operator.Runner
# has an internal data race on startup that the operator bring-up test would trip.
test:
	go test -tags netgo -timeout 30m -count 1 ./...

include ../sdk.mk

.PHONY: generate # Run Grafana App SDK code generation
generate: install-app-sdk update-app-sdk
	@$(APP_SDK_BIN) generate --source=./kinds/

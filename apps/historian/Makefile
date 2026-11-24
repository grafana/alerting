.PHONY: test # Run unit tests
test:
	go test -tags netgo -timeout 30m -count 1 ./...

include ../sdk.mk

.PHONY: generate # Run Grafana App SDK code generation
generate: install-app-sdk update-app-sdk
	@$(APP_SDK_BIN) generate --source=./kinds/

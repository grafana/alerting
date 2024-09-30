# Put tools at the root of the folder.
PATH := $(CURDIR)/.tools/bin:$(PATH)

.PHONY: clean
clean:
	@# go mod makes the modules read-only, so before deletion we need to make them deleteable
	@chmod -R u+rwX .tools 2> /dev/null || true
	rm -rf .tools/

.PHONY: test
test:
	go test -tags netgo -timeout 30m -race -count 1 ./...

.PHONY: lint
lint: .tools/bin/misspell .tools/bin/faillint .tools/bin/golangci-lint
	misspell -error README.md CONTRIBUTING.md LICENSE

	# Configured via .golangci.yml.
	golangci-lint run

.PHONY: mod-check
mod-check:
	GO111MODULE=on go mod download
	GO111MODULE=on go mod verify
	GO111MODULE=on go mod tidy
	@git diff --exit-code -- go.sum go.mod

.PHONY: drone
drone: .drone/drone.yml

# Drone.
.drone/drone.yml: .drone/drone.jsonnet
	drone jsonnet --source $< --target $@.tmp --stream --format=false
	drone sign --save grafana/alerting $@.tmp
	drone lint --trusted $@.tmp
	mv $@.tmp $@

# Tools needed to run linting.
.tools:
	mkdir -p .tools/

.tools/bin/misspell: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/client9/misspell/cmd/misspell@v0.3.4

.tools/bin/faillint: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/fatih/faillint@v1.10.0

.tools/bin/golangci-lint: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0

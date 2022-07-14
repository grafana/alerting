.PHONY: test
test:
	go test -tags netgo -timeout 30m -race -count 1 ./...

.PHONY: mod-check
mod-check:
	GO111MODULE=on go mod download
	GO111MODULE=on go mod verify
	GO111MODULE=on go mod tidy
	@git diff --exit-code -- go.sum go.mod
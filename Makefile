# Put tools at the root of the folder.
PATH := $(CURDIR)/.tools/bin:$(PATH)
SHELL := /usr/bin/env bash

.PHONY: build
build:
	$(MAKE) -C apps/historian build

.PHONY: clean
clean:
	@# go mod makes the modules read-only, so before deletion we need to make them deleteable
	@chmod -R u+rwX .tools 2> /dev/null || true
	rm -rf .tools/

.PHONY: test
test:
	go test -tags netgo -timeout 30m -race -count 1 ./...
	$(MAKE) -C apps/historian test
	$(MAKE) -C testing/alerting-gen test

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

# Tools needed to run linting.
.tools:
	mkdir -p .tools/

.tools/bin/misspell: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/client9/misspell/cmd/misspell@v0.3.4

.tools/bin/faillint: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/fatih/faillint@v1.10.0

.tools/bin/golangci-lint: .tools
	GOPATH=$(CURDIR)/.tools go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

##
## Docker image targets.
##
## The build context is this repository root so the parent
## github.com/grafana/alerting module and go.work are available and images build
## against this repo's source rather than a pinned published version (see
## apps/historian/Dockerfile).
##

IMAGE_TAG ?= $(shell ./tools/image-tag)
IMAGE_PLATFORM_OS ?= linux
IMAGE_PLATFORM_ARCH ?= amd64
IMAGE_PLATFORM := ${IMAGE_PLATFORM_OS}/${IMAGE_PLATFORM_ARCH}
PROD_IMAGE_PREFIX := us-docker.pkg.dev/grafanalabs-global/docker-alerting-prod/
DEV_IMAGE_PREFIX := us-docker.pkg.dev/grafanalabs-dev/docker-alerting-dev/

DOCKER_BUILD_EXTRA_ARGS ?=

.PHONY: docker-image-tag
docker-image-tag:
	@echo $(IMAGE_TAG)

# Aggregate targets: build or push every image.
.PHONY: build-docker-images
build-docker-images: build-docker-image-historian-operator

.PHONY: push-images-prod-registry
push-images-prod-registry: push-image-prod-historian-operator

.PHONY: push-images-dev-registry
push-images-dev-registry: push-image-dev-historian-operator

## historian-operator

HISTORIAN_OPERATOR_PROD := $(PROD_IMAGE_PREFIX)historian-operator:$(IMAGE_TAG)
HISTORIAN_OPERATOR_DEV := $(DEV_IMAGE_PREFIX)historian-operator:$(IMAGE_TAG)
HISTORIAN_OPERATOR_DEV_LATEST := $(DEV_IMAGE_PREFIX)historian-operator:latest

.PHONY: build-docker-image-historian-operator
build-docker-image-historian-operator:
	@echo
	docker build \
		--platform=$(IMAGE_PLATFORM) \
		-t $(HISTORIAN_OPERATOR_PROD) \
		-t $(HISTORIAN_OPERATOR_DEV) \
		-t $(HISTORIAN_OPERATOR_DEV_LATEST) \
		$(DOCKER_BUILD_EXTRA_ARGS) \
		-f apps/historian/Dockerfile .
	@echo

.PHONY: push-image-prod-historian-operator
push-image-prod-historian-operator: build-docker-image-historian-operator
	docker push $(HISTORIAN_OPERATOR_PROD)

.PHONY: push-image-dev-historian-operator
push-image-dev-historian-operator: build-docker-image-historian-operator
	docker push $(HISTORIAN_OPERATOR_DEV)

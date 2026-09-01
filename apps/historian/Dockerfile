# Build context is the repository root, NOT apps/historian, so that the parent
# github.com/grafana/alerting module and the root go.work are available and the
# operator builds against this repo's source rather than a pinned published
# version. Build with:
#
#   docker build -f apps/historian/Dockerfile .
#
# (run from the repository root; `make build-docker-image-historian-operator`,
# or the `build-docker-images` aggregate, does this for you.)
FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

# make is used to build via the Makefile's `build` target.
RUN apk add --no-cache make

WORKDIR /build

# Copy the workspace and both modules' dependency files first, in their own
# layer, so downloads are cached and only re-run when a go.mod/go.sum/go.work
# changes rather than on every source edit.
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY apps/historian/go.mod apps/historian/go.sum ./apps/historian/
# testing/alerting-gen is listed in go.work, so workspace mode requires its
# module file to be present even though the operator does not depend on it.
COPY testing/alerting-gen/go.mod testing/alerting-gen/go.sum ./testing/alerting-gen/
RUN go mod download

# Copy the parent module and the historian app source.
COPY . .

# `make build` runs `go build -tags netgo ./cmd/historian-operator`, emitting the
# binary at ./historian-operator. Run it from the app dir since the target uses a
# package-relative path. Workspace mode resolves github.com/grafana/alerting to
# the parent module copied above.
WORKDIR /build/apps/historian
RUN make build

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS runtime
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/apps/historian/historian-operator /usr/bin/historian-operator

ENTRYPOINT ["/usr/bin/historian-operator"]

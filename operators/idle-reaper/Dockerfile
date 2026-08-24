# Build the manager binary
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Dependencies first so a source-only change does not re-download them.
COPY go.mod go.mod
COPY go.sum go.sum
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# The scaffolded build passed -a, which forces every package including the
# standard library to be rebuilt from scratch. For a controller-runtime binary
# that turns a build into a fifteen-minute wait for no benefit: CGO_ENABLED=0
# already gives a static binary, and the build cache handles correctness.
#
# The cache mounts keep the module and build caches across image builds, which
# is what makes an incremental rebuild take seconds instead of minutes.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager cmd/main.go

# Distroless: no shell, no package manager, non-root by default.
# Note the binary embeds the timezone database (time/tzdata) because this image
# has no system zoneinfo and schedules are evaluated in a named location.
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]

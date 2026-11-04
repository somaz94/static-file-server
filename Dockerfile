# syntax=docker/dockerfile:1
# Build the static-file-server binary. The builder runs natively and cross-compiles via
# TARGETOS/TARGETARCH, so the arm64 image is not compiled under QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Cache deps in a dedicated layer with mount cache for faster rebuilds
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
COPY cmd/ cmd/
COPY internal/ internal/

# Build with cache mounts for Go build cache and module cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w \
      -X github.com/somaz94/static-file-server/internal/version.Version=${VERSION} \
      -X github.com/somaz94/static-file-server/internal/version.GitCommit=${GIT_COMMIT} \
      -X github.com/somaz94/static-file-server/internal/version.BuildDate=${BUILD_DATE}" \
    -o static-file-server ./cmd/

# Pinned by Debian release and digest: the unversioned static tag moves to a new Debian
# release silently, and Dependabot can only bump what is pinned.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# OCI image labels
LABEL org.opencontainers.image.title="static-file-server" \
      org.opencontainers.image.description="Lightweight static file server with modern directory listing UI" \
      org.opencontainers.image.url="https://github.com/somaz94/static-file-server" \
      org.opencontainers.image.source="https://github.com/somaz94/static-file-server" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /
COPY --from=builder /workspace/static-file-server .

# Default serving directory
VOLUME ["/web"]
EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/static-file-server"]

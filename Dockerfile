# Build the static-file-server binary
FROM golang:1.26 AS builder
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

# Use distroless as minimal base image
FROM gcr.io/distroless/static:nonroot
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# OCI image labels
LABEL org.opencontainers.image.title="static-file-server" \
      org.opencontainers.image.description="Lightweight static file server with modern directory listing UI" \
      org.opencontainers.image.url="https://github.com/somaz94/static-file-server" \
      org.opencontainers.image.source="https://github.com/somaz94/static-file-server" \
      org.opencontainers.image.licenses="MIT" \
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

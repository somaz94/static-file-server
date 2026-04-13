APP_NAME := static-file-server
MODULE := github.com/somaz94/static-file-server

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "\
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE) \
	-s -w"

# Docker
IMG ?= somaz940/$(APP_NAME):$(VERSION)
CONTAINER_TOOL ?= docker

DOCKER_BUILD_ARGS = \
	--build-arg VERSION=$(VERSION) \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg BUILD_DATE=$(BUILD_DATE)

# Platforms for cross-compilation
PLATFORMS ?= linux/amd64,linux/arm64

# Tools
GOLANGCI_LINT_VERSION ?= v2.1.6

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: help build test test-unit test-integration cover clean install fmt vet run \
	lint lint-fix version \
	docker-build docker-push docker-buildx docker-buildx-tag docker-buildx-latest \
	cross-build

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

version: ## Print version information
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

##@ Development

build: ## Build binary to bin/
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/

run: build ## Build and run the server
	./bin/$(APP_NAME)

fmt: ## Format Go source code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	@$(call go-install-tool,golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with --fix
	@$(call go-install-tool,golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	golangci-lint run --fix ./...

##@ Testing

test: ## Run all tests with race detection and coverage
	go test ./... -v -race -cover

test-unit: ## Run unit tests only (internal packages)
	go test ./internal/... -v -race -cover

test-integration: ## Run integration tests only
	go test -v -race -run TestIntegration ./...

cover: ## Generate HTML coverage report
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

##@ Build

install: build ## Install binary to /usr/local/bin
	cp bin/$(APP_NAME) /usr/local/bin/$(APP_NAME)

cross-build: ## Build for multiple OS/arch (output: dist/)
	@mkdir -p dist
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${platform%%/*}; \
		arch=$${platform##*/}; \
		output=dist/$(APP_NAME)-$${os}-$${arch}; \
		if [ "$${os}" = "windows" ]; then output="$${output}.exe"; fi; \
		echo "Building $${os}/$${arch}..."; \
		GOOS=$${os} GOARCH=$${arch} go build $(LDFLAGS) -o $${output} ./cmd/; \
	done
	@echo "Cross-build complete. Binaries in dist/"

##@ Docker

docker-build: ## Build docker image
	$(CONTAINER_TOOL) build $(DOCKER_BUILD_ARGS) -t ${IMG} .

docker-push: ## Push docker image
	$(CONTAINER_TOOL) push ${IMG}

docker-buildx-tag: ## Build and push multi-arch image with version tag
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name $(APP_NAME)-builder
	$(CONTAINER_TOOL) buildx use $(APP_NAME)-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--tag ${IMG} \
		-f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm $(APP_NAME)-builder
	rm Dockerfile.cross

docker-buildx-latest: ## Build and push multi-arch image with latest tag
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name $(APP_NAME)-builder
	$(CONTAINER_TOOL) buildx use $(APP_NAME)-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) \
		$(DOCKER_BUILD_ARGS) \
		--tag $(shell echo ${IMG} | cut -f1 -d:):latest \
		-f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm $(APP_NAME)-builder
	rm Dockerfile.cross

docker-buildx: ## Build and push both version and latest tags
docker-buildx: docker-buildx-tag docker-buildx-latest

##@ Cleanup

clean: ## Remove build artifacts
	rm -rf bin/ dist/ coverage.out coverage.html

# Helper function to install Go tools
define go-install-tool
@[ -f $$(which $(1)) ] || { \
	echo "Installing $(1)@$(3)..." ;\
	go install $(2)@$(3) ;\
}
endef

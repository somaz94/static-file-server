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

.PHONY: build test test-unit cover clean install fmt vet run

build:
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/

test:
	go test ./... -v -race -cover

test-unit:
	go test ./internal/... -v -race -cover

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/ coverage.out coverage.html

install: build
	cp bin/$(APP_NAME) /usr/local/bin/$(APP_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...

run: build
	./bin/$(APP_NAME)

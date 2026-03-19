BINARY=agenterm
GO=/opt/homebrew/bin/go
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean all linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

build:
	$(GO) build $(LDFLAGS) -o bin/$(BINARY) ./cmd/agenterm

all: linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/agenterm

linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./cmd/agenterm

darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/agenterm

darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/agenterm

clean:
	rm -rf bin/

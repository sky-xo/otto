.PHONY: build install test clean

VERSION ?= v0.1.0

# Build the june binary
build:
	go build -ldflags "-X github.com/sky-xo/june/internal/cli.Version=$(VERSION)" -o june ./cmd/june

# Install to $GOPATH/bin
install:
	go install -ldflags "-X github.com/sky-xo/june/internal/cli.Version=$(VERSION)" ./cmd/june

# Run all tests
test:
	go test ./...

# Run tests with coverage
cover:
	go test -cover ./...

# Clean build artifacts
clean:
	rm -f june

# Build and run the TUI watch
watch:
	go build -o june ./cmd/june && ./june watch

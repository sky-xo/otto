.PHONY: build install test clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X github.com/sky-xo/june/internal/cli.version=$(VERSION) -X github.com/sky-xo/june/internal/cli.commit=$(COMMIT)"

# Build the june binary
build:
	go build $(LDFLAGS) -o june .

# Install to $GOPATH/bin
install:
	go install $(LDFLAGS) .

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
	go build $(LDFLAGS) -o june . && ./june watch

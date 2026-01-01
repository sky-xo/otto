.PHONY: build install test clean

# Build the june binary
build:
	go build -o june ./cmd/june

# Install to $GOPATH/bin
install:
	go install ./cmd/june

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

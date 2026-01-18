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

# Superpowers vendoring
SUPERPOWERS_VERSION := v4.0.3
SUPERPOWERS_REPO := https://github.com/obra/superpowers

.PHONY: build-skills
build-skills:
	@# Fetch superpowers if not cached
	@[ -d .skill-cache/superpowers ] || git clone $(SUPERPOWERS_REPO) .skill-cache/superpowers
	@cd .skill-cache/superpowers && git fetch && git checkout $(SUPERPOWERS_VERSION)
	@# Clean and copy superpowers skills
	rm -rf skills/
	cp -r .skill-cache/superpowers/skills skills/
	@# Overlay June's custom skills (override)
	cp -r june-skills/* skills/
	@echo "Skills assembled: superpowers $(SUPERPOWERS_VERSION) + june overrides"

.PHONY: update-superpowers
update-superpowers:
	cd .skill-cache/superpowers && git fetch origin main && git log --oneline HEAD..origin/main
	@echo "Review changes above, then update SUPERPOWERS_VERSION and run 'make build-skills'"

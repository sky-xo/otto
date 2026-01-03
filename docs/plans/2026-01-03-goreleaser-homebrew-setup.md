# GoReleaser + Homebrew Setup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable `brew install sky-xo/tap/june` with pre-built binaries for macOS and Linux.

**Architecture:** GoReleaser builds binaries on tag push via GitHub Actions, uploads to GitHub Releases, and auto-updates the Homebrew tap formula.

**Tech Stack:** GoReleaser, GitHub Actions, Homebrew

---

### Task 1: Create GoReleaser configuration

**Files:**
- Create: `.goreleaser.yaml`

**Step 1: Create the GoReleaser config file**

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
version: 2

project_name: june

before:
  hooks:
    - go mod tidy

builds:
  - id: june
    main: ./main.go
    binary: june
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/sky-xo/june/internal/cli.version={{.Version}}

archives:
  - id: june-archive
    format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - LICENSE*

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "Merge pull request"
      - "Merge branch"

brews:
  - name: june
    repository:
      owner: sky-xo
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/sky-xo/june"
    description: "A read-only TUI for viewing Claude Code subagent activity"
    license: "MIT"
    install: |
      bin.install "june"
    test: |
      system "#{bin}/june", "--help"

release:
  github:
    owner: sky-xo
    name: june
  draft: false
  prerelease: auto
  name_template: "v{{ .Version }}"
```

**Step 2: Commit**

```bash
git add .goreleaser.yaml
git commit -m "chore: add goreleaser configuration"
```

---

### Task 2: Create GitHub Actions workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the workflow file**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release workflow"
```

---

### Task 3: Create Homebrew tap repository

**Step 1: Create the repository**

Go to GitHub and create: `github.com/sky-xo/homebrew-tap`
- Public repository
- Initialize with README.md
- Description: "Homebrew formulae for sky-xo projects"

**Step 2: Create GitHub Personal Access Token**

- Go to GitHub Settings -> Developer Settings -> Personal Access Tokens -> Fine-grained tokens
- Create token with:
  - Token name: `goreleaser-homebrew-tap`
  - Expiration: 1 year (or custom)
  - Repository access: Only select repositories -> `sky-xo/homebrew-tap`
  - Permissions:
    - Contents: Read and write
    - Metadata: Read (auto-selected)
- Copy the token

**Step 3: Add secret to june repository**

- Go to `github.com/sky-xo/june` -> Settings -> Secrets and variables -> Actions
- Click "New repository secret"
- Name: `HOMEBREW_TAP_GITHUB_TOKEN`
- Secret: [paste the token from Step 2]
- Click "Add secret"

---

### Task 4: Test locally

**Step 1: Install GoReleaser**

```bash
brew install goreleaser
```

**Step 2: Validate the configuration**

```bash
goreleaser check
```

Expected: "config is valid"

**Step 3: Test build with snapshot**

```bash
goreleaser release --snapshot --clean
```

Expected: Creates `dist/` folder with:
- `june_<version>-SNAPSHOT-<commit>_darwin_amd64.tar.gz`
- `june_<version>-SNAPSHOT-<commit>_darwin_arm64.tar.gz`
- `june_<version>-SNAPSHOT-<commit>_linux_amd64.tar.gz`
- `june_<version>-SNAPSHOT-<commit>_linux_arm64.tar.gz`
- `checksums.txt`

**Step 4: Clean up**

```bash
rm -rf dist/
```

---

### Task 5: Trigger first release

**Step 1: Push commits to main**

```bash
git push origin main
```

**Step 2: Create and push new tag**

```bash
git tag v0.2.0
git push origin v0.2.0
```

**Step 3: Verify release**

- Check GitHub Actions at `github.com/sky-xo/june/actions` for successful run
- Check GitHub Releases at `github.com/sky-xo/june/releases` for binaries
- Check `github.com/sky-xo/homebrew-tap` for new `Formula/june.rb` file

**Step 4: Test Homebrew install**

```bash
brew tap sky-xo/tap
brew install june
june --help
```

Expected: june runs successfully from Homebrew installation

---

### Troubleshooting

**If GoReleaser check fails:**
- Ensure YAML syntax is valid
- Check that main.go exists at repo root

**If GitHub Actions fails:**
- Check that `HOMEBREW_TAP_GITHUB_TOKEN` secret is set
- Verify the PAT has correct permissions for homebrew-tap repo

**If Homebrew formula is not pushed:**
- Verify the PAT token has Contents write permission
- Check GoReleaser logs for authentication errors

**If brew install fails:**
- Wait a few minutes for tap to sync
- Run `brew update` before install
- Check formula syntax at `sky-xo/homebrew-tap/Formula/june.rb`

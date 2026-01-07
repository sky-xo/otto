# June Config for MCP Servers - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `~/.june/config.yaml` for MCP server configuration, passed to Gemini subagents.

**Architecture:** New config loader in `internal/config/`, enhanced `EnsureGeminiHome()` to copy auth and write `settings.json`, spawn sets `GEMINI_CONFIG_DIR` env var.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, existing patterns from Codex auth copying

---

## Task 1: Add yaml.v3 dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the dependency**

Run: `go get gopkg.in/yaml.v3`

**Step 2: Verify it was added**

Run: `grep yaml go.mod`
Expected: `gopkg.in/yaml.v3 v3.x.x`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add gopkg.in/yaml.v3 dependency"
```

---

## Task 2: Create config types and loader

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing test for LoadConfig**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create config file
	configDir := filepath.Join(tmpHome, ".june")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `mcpServers:
  chrome:
    command: node
    args:
      - /path/to/chrome-mcp.js
  playwright:
    command: npx
    args:
      - "@anthropic/mcp-playwright"
    env:
      DEBUG: "true"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.MCPServers) != 2 {
		t.Errorf("len(MCPServers) = %d, want 2", len(cfg.MCPServers))
	}

	chrome, ok := cfg.MCPServers["chrome"]
	if !ok {
		t.Fatal("missing chrome server")
	}
	if chrome.Command != "node" {
		t.Errorf("chrome.Command = %q, want %q", chrome.Command, "node")
	}
	if len(chrome.Args) != 1 || chrome.Args[0] != "/path/to/chrome-mcp.js" {
		t.Errorf("chrome.Args = %v, want [/path/to/chrome-mcp.js]", chrome.Args)
	}

	playwright := cfg.MCPServers["playwright"]
	if playwright.Env["DEBUG"] != "true" {
		t.Errorf("playwright.Env[DEBUG] = %q, want %q", playwright.Env["DEBUG"], "true")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No config file - should return empty config, not error
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.MCPServers == nil {
		t.Error("MCPServers should be initialized, not nil")
	}
	if len(cfg.MCPServers) != 0 {
		t.Errorf("len(MCPServers) = %d, want 0", len(cfg.MCPServers))
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create invalid config file
	configDir := filepath.Join(tmpHome, ".june")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("mcpServers:\n  chrome:\n    - not a map"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestLoadConfig -v`
Expected: FAIL (LoadConfig not defined)

**Step 3: Implement config types and LoadConfig**

Create `internal/config/config.go` (add to existing file):

```go
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents June's configuration file
type Config struct {
	MCPServers map[string]MCPServer `yaml:"mcpServers"`
}

// MCPServer represents an MCP server configuration
type MCPServer struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// LoadConfig loads the config from ~/.june/config.yaml
// Returns empty config (not error) if file doesn't exist
func LoadConfig() (*Config, error) {
	cfg := &Config{
		MCPServers: make(map[string]MCPServer),
	}

	configPath := filepath.Join(DataDir(), "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // No config file is fine
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure map is initialized even if YAML had no servers
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServer)
	}

	return cfg, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add YAML config loader for MCP servers"
```

---

## Task 3: Add auth file copying to EnsureGeminiHome

**Files:**
- Modify: `internal/gemini/home.go`
- Modify: `internal/gemini/home_test.go`

**Step 1: Write failing test for auth copying**

Add to `internal/gemini/home_test.go`:

```go
func TestEnsureGeminiHomeCopiesAuth(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create mock ~/.gemini/ with auth files
	userGemini := filepath.Join(tmpHome, ".gemini")
	if err := os.MkdirAll(userGemini, 0755); err != nil {
		t.Fatal(err)
	}

	// Create oauth_creds.json
	oauthContent := `{"token": "secret"}`
	if err := os.WriteFile(filepath.Join(userGemini, "oauth_creds.json"), []byte(oauthContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Create google_accounts.json
	accountsContent := `{"active": "user@example.com"}`
	if err := os.WriteFile(filepath.Join(userGemini, "google_accounts.json"), []byte(accountsContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Run EnsureGeminiHome
	home, err := EnsureGeminiHome()
	if err != nil {
		t.Fatalf("EnsureGeminiHome failed: %v", err)
	}

	// Verify oauth_creds.json was copied
	oauthDst := filepath.Join(home, "oauth_creds.json")
	data, err := os.ReadFile(oauthDst)
	if err != nil {
		t.Fatalf("oauth_creds.json not copied: %v", err)
	}
	if string(data) != oauthContent {
		t.Errorf("oauth_creds.json content = %q, want %q", string(data), oauthContent)
	}

	// Verify oauth_creds.json has 0600 permissions
	info, _ := os.Stat(oauthDst)
	if info.Mode().Perm() != 0600 {
		t.Errorf("oauth_creds.json mode = %o, want 0600", info.Mode().Perm())
	}

	// Verify google_accounts.json was copied
	accountsDst := filepath.Join(home, "google_accounts.json")
	data, err = os.ReadFile(accountsDst)
	if err != nil {
		t.Fatalf("google_accounts.json not copied: %v", err)
	}
	if string(data) != accountsContent {
		t.Errorf("google_accounts.json content = %q, want %q", string(data), accountsContent)
	}

	// Verify google_accounts.json has 0600 permissions
	info, _ = os.Stat(accountsDst)
	if info.Mode().Perm() != 0600 {
		t.Errorf("google_accounts.json mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestEnsureGeminiHomeNoAuthFiles(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No ~/.gemini/ directory - should still succeed
	home, err := EnsureGeminiHome()
	if err != nil {
		t.Fatalf("EnsureGeminiHome failed: %v", err)
	}

	// Verify home was created
	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Error("home directory not created")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/gemini/... -run TestEnsureGeminiHomeCopies -v`
Expected: FAIL (oauth_creds.json not copied)

**Step 3: Update EnsureGeminiHome to copy auth files**

Update `internal/gemini/home.go`:

```go
package gemini

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureGeminiHome creates the June gemini home at ~/.june/gemini/
// and the sessions subdirectory. Also copies auth files from ~/.gemini/.
// Returns the path to the gemini home directory.
func EnsureGeminiHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create ~/.june/gemini/
	geminiHome := filepath.Join(home, ".june", "gemini")
	if err := os.MkdirAll(geminiHome, 0755); err != nil {
		return "", err
	}

	// Create sessions subdirectory
	sessionsDir := filepath.Join(geminiHome, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", err
	}

	// Copy auth files from user's ~/.gemini/ if they exist
	userGemini := filepath.Join(home, ".gemini")
	oauthMissing := copyAuthFile(userGemini, geminiHome, "oauth_creds.json", 0600)
	accountsMissing := copyAuthFile(userGemini, geminiHome, "google_accounts.json", 0600)

	// Warn if auth files are missing (user may not have logged into Gemini yet)
	if oauthMissing || accountsMissing {
		fmt.Fprintf(os.Stderr, "warning: gemini auth files not found in ~/.gemini/ - run 'gemini' to authenticate\n")
	}

	return geminiHome, nil
}

// copyAuthFile copies a file from src dir to dst dir if source exists and dest doesn't.
// Uses atomic create-if-not-exists pattern.
// Returns true if source was missing (for warning purposes).
func copyAuthFile(srcDir, dstDir, filename string, perm os.FileMode) bool {
	srcPath := filepath.Join(srcDir, filename)
	dstPath := filepath.Join(dstDir, filename)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return true // Source doesn't exist
	}

	// Try to create the file exclusively - fails if it already exists
	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return false // Already exists or other error
	}
	defer f.Close()
	f.Write(data)
	return false
}

// SessionsDir returns the path to the sessions directory.
func SessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".june", "gemini", "sessions"), nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/gemini/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gemini/home.go internal/gemini/home_test.go
git commit -m "feat(gemini): copy auth files from ~/.gemini/ to isolated home"
```

---

## Task 4: Add settings.json writing with MCP servers

**Files:**
- Modify: `internal/gemini/home.go`
- Modify: `internal/gemini/home_test.go`

**Step 1: Write failing test for WriteSettings**

Add to `internal/gemini/home_test.go`:

```go
import (
	"encoding/json"
	// ... existing imports
)

func TestWriteSettings(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Ensure gemini home exists
	geminiHome, err := EnsureGeminiHome()
	if err != nil {
		t.Fatal(err)
	}

	// Write settings with MCP servers
	mcpServers := map[string]MCPServerConfig{
		"chrome": {
			Command: "node",
			Args:    []string{"/path/to/mcp.js"},
		},
		"test": {
			Command: "echo",
			Args:    []string{"hello"},
			Env:     map[string]string{"FOO": "bar"},
		},
	}

	if err := WriteSettings(geminiHome, mcpServers); err != nil {
		t.Fatalf("WriteSettings failed: %v", err)
	}

	// Read and verify settings.json
	data, err := os.ReadFile(filepath.Join(geminiHome, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var settings struct {
		MCPServers map[string]MCPServerConfig `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(settings.MCPServers) != 2 {
		t.Errorf("len(mcpServers) = %d, want 2", len(settings.MCPServers))
	}

	chrome := settings.MCPServers["chrome"]
	if chrome.Command != "node" {
		t.Errorf("chrome.Command = %q, want %q", chrome.Command, "node")
	}
}

func TestWriteSettingsEmpty(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	geminiHome, _ := EnsureGeminiHome()

	// Write empty settings
	if err := WriteSettings(geminiHome, nil); err != nil {
		t.Fatalf("WriteSettings failed: %v", err)
	}

	// Verify settings.json exists with empty mcpServers
	data, err := os.ReadFile(filepath.Join(geminiHome, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	if settings["mcpServers"] == nil {
		t.Error("mcpServers should exist even if empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/gemini/... -run TestWriteSettings -v`
Expected: FAIL (WriteSettings not defined)

**Step 3: Implement MCPServerConfig and WriteSettings**

Add to `internal/gemini/home.go`:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MCPServerConfig represents an MCP server for Gemini's settings.json
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// WriteSettings writes a settings.json file with MCP server configuration.
// Overwrites any existing settings.json.
func WriteSettings(geminiHome string, mcpServers map[string]MCPServerConfig) error {
	if mcpServers == nil {
		mcpServers = make(map[string]MCPServerConfig)
	}

	settings := map[string]interface{}{
		"mcpServers": mcpServers,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(geminiHome, "settings.json"), data, 0600)
}
```

**Step 4: Run tests**

Run: `go test ./internal/gemini/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gemini/home.go internal/gemini/home_test.go
git commit -m "feat(gemini): add WriteSettings for MCP server configuration"
```

---

## Task 5: Update runSpawnGemini to use config and set env var

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Update runSpawnGemini**

In `internal/cli/spawn.go`, update `runSpawnGemini` to:
1. Load config
2. Only if config has MCP servers: write settings.json and set GEMINI_CONFIG_DIR
3. Otherwise: let Gemini use its normal ~/.gemini/ config

Find the `runSpawnGemini` function and update it:

```go
func runSpawnGemini(prefix, task string, model string, yolo, sandbox bool) error {
	// Check if gemini is installed
	if !geminiInstalled() {
		return fmt.Errorf("gemini CLI not found - install with: npm install -g @google/gemini-cli")
	}

	// Load June config for MCP servers
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Capture git context before spawning
	repoPath := scope.RepoRoot()
	branch := scope.BranchName()

	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Ensure gemini home exists (copies auth files)
	geminiHome, err := gemini.EnsureGeminiHome()
	if err != nil {
		return fmt.Errorf("failed to setup gemini home: %w", err)
	}

	// Build gemini command arguments
	args := buildGeminiArgs(task, model, yolo, sandbox)

	// Start gemini -p ...
	geminiCmd := exec.Command("gemini", args...)
	geminiCmd.Stderr = os.Stderr

	// Only set GEMINI_CONFIG_DIR if we have MCP servers to configure
	// Otherwise, let Gemini use its normal ~/.gemini/ config (preserves user's existing MCP servers)
	if len(cfg.MCPServers) > 0 {
		// Write settings.json with MCP servers from config
		mcpServers := make(map[string]gemini.MCPServerConfig)
		for name, server := range cfg.MCPServers {
			mcpServers[name] = gemini.MCPServerConfig{
				Command: server.Command,
				Args:    server.Args,
				Env:     server.Env,
			}
		}
		if err := gemini.WriteSettings(geminiHome, mcpServers); err != nil {
			return fmt.Errorf("failed to write gemini settings: %w", err)
		}
		geminiCmd.Env = append(os.Environ(), fmt.Sprintf("GEMINI_CONFIG_DIR=%s", geminiHome))
	}

	// ... rest of function unchanged
```

**Step 2: Add config import**

Add to imports at top of `spawn.go`:

```go
"github.com/sky-xo/june/internal/config"
```

**Step 3: Run build**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): use config for MCP servers, set GEMINI_CONFIG_DIR"
```

---

## Task 6: Full integration test

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests PASS

**Step 2: Build binary**

Run: `make build`
Expected: SUCCESS

**Step 3: Manual verification (if gemini CLI installed)**

Create test config:

```bash
mkdir -p ~/.june
cat > ~/.june/config.yaml << 'EOF'
mcpServers:
  test:
    command: echo
    args:
      - "hello from mcp"
EOF
```

Verify settings.json is created after spawn attempt:

```bash
./june spawn gemini "say hello" --name test-mcp
cat ~/.june/gemini/settings.json
```

Expected: settings.json contains the test MCP server config.

**Step 4: Cleanup test config**

```bash
rm ~/.june/config.yaml
```

**Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address integration test issues"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add yaml.v3 dependency | go.mod |
| 2 | Create config loader | config/config.go, config_test.go |
| 3 | Copy auth files | gemini/home.go, home_test.go |
| 4 | Write settings.json | gemini/home.go, home_test.go |
| 5 | Update spawn to use config | cli/spawn.go |
| 6 | Integration test | - |

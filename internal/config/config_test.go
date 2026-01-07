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

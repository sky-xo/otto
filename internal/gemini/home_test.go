package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGeminiHome(t *testing.T) {
	// Use temp dir as home
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	home, err := EnsureGeminiHome()
	if err != nil {
		t.Fatalf("EnsureGeminiHome failed: %v", err)
	}

	expected := filepath.Join(tmpHome, ".june", "gemini")
	if home != expected {
		t.Errorf("home = %q, want %q", home, expected)
	}

	// Verify directory was created
	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Errorf("directory was not created")
	}

	// Verify sessions subdirectory was created
	sessionsDir := filepath.Join(home, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Errorf("sessions directory was not created")
	}
}

func TestEnsureGeminiHomeCopiesAuth(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

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
	t.Setenv("HOME", tmpHome)

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

func TestEnsureGeminiHome_DoesNotOverwriteExistingAuth(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create mock ~/.gemini/ with auth files
	userGemini := filepath.Join(tmpHome, ".gemini")
	if err := os.MkdirAll(userGemini, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userGemini, "oauth_creds.json"), []byte(`{"new": "creds"}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Create existing auth file in dest with different content
	geminiHome := filepath.Join(tmpHome, ".june", "gemini")
	if err := os.MkdirAll(geminiHome, 0755); err != nil {
		t.Fatal(err)
	}
	existingContent := `{"existing": "data"}`
	if err := os.WriteFile(filepath.Join(geminiHome, "oauth_creds.json"), []byte(existingContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Run EnsureGeminiHome - should NOT overwrite existing file
	_, err := EnsureGeminiHome()
	if err != nil {
		t.Fatalf("EnsureGeminiHome failed: %v", err)
	}

	// Verify existing file was NOT overwritten
	data, err := os.ReadFile(filepath.Join(geminiHome, "oauth_creds.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existingContent {
		t.Errorf("existing file was overwritten: got %q, want %q", string(data), existingContent)
	}
}

func TestWriteSettings(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

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
	t.Setenv("HOME", tmpHome)

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

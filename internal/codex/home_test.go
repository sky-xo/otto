package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCodexHome_CreatesDirectory(t *testing.T) {
	// Use temp dir as fake june home
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create fake user codex with auth.json
	userCodex := filepath.Join(tmpDir, ".codex")
	os.MkdirAll(userCodex, 0755)
	os.WriteFile(filepath.Join(userCodex, "auth.json"), []byte(`{"token":"secret"}`), 0600)

	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome failed: %v", err)
	}

	// Should be under ~/.june/codex
	expected := filepath.Join(tmpDir, ".june", "codex")
	if codexHome != expected {
		t.Errorf("codexHome = %q, want %q", codexHome, expected)
	}

	// Directory should exist
	if _, err := os.Stat(codexHome); os.IsNotExist(err) {
		t.Error("codex home directory was not created")
	}
}

func TestEnsureCodexHome_CopiesAuthJson(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create fake user codex with auth.json
	userCodex := filepath.Join(tmpDir, ".codex")
	os.MkdirAll(userCodex, 0755)
	authContent := []byte(`{"token":"test-secret-token"}`)
	os.WriteFile(filepath.Join(userCodex, "auth.json"), authContent, 0600)

	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome failed: %v", err)
	}

	// auth.json should be copied
	copiedAuth := filepath.Join(codexHome, "auth.json")
	data, err := os.ReadFile(copiedAuth)
	if err != nil {
		t.Fatalf("failed to read copied auth.json: %v", err)
	}
	if string(data) != string(authContent) {
		t.Errorf("auth.json content = %q, want %q", string(data), string(authContent))
	}
}

func TestEnsureCodexHome_NoAuthJsonOK(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// No ~/.codex exists at all
	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome should succeed without auth.json: %v", err)
	}

	// Directory should still be created
	if _, err := os.Stat(codexHome); os.IsNotExist(err) {
		t.Error("codex home directory was not created")
	}
}

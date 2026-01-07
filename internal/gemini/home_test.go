package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGeminiHome(t *testing.T) {
	// Use temp dir as home
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

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

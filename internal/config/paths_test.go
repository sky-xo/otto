package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_DefaultsToHomeOtto(t *testing.T) {
	t.Setenv("HOME", "/tmp/otto-home")
	got := DataDir()
	want := filepath.Join("/tmp/otto-home", ".otto")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDataDir_FallsBackToUserHomeDir(t *testing.T) {
	t.Setenv("HOME", "")
	got := DataDir()
	// When HOME is empty, should fall back to os.UserHomeDir()
	// We expect it to return a valid path (not empty and not just ".otto")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("os.UserHomeDir() failed, skipping test")
	}
	want := filepath.Join(home, ".otto")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

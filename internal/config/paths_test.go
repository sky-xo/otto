package config

import (
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

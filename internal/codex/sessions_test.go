package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindSessionFile(t *testing.T) {
	// Create temp directory structure mimicking Codex
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions", "2025", "01", "03")
	os.MkdirAll(sessionsDir, 0755)

	// Create a session file with known ULID
	ulid := "019b825b-b138-7981-898d-2830d3610fc9"
	filename := "rollout-2025-01-03T10-00-00-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{"type":"session_meta"}`), 0644)

	// Find it
	found, err := FindSessionFile(tmpDir, ulid)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileToday(t *testing.T) {
	// Create temp directory with today's date
	tmpDir := t.TempDir()
	now := time.Now()
	sessionsDir := filepath.Join(tmpDir, "sessions",
		now.Format("2006"), now.Format("01"), now.Format("02"))
	os.MkdirAll(sessionsDir, 0755)

	ulid := "test-ulid-12345"
	filename := "rollout-" + now.Format("2006-01-02T15-04-05") + "-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{}`), 0644)

	found, err := FindSessionFile(tmpDir, ulid)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := FindSessionFile(tmpDir, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}

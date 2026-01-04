package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindSessionFile(t *testing.T) {
	// Create temp directory structure with HOME override
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create session file in ~/.june/codex/sessions/2025/01/03/
	juneCodex := filepath.Join(tmpDir, ".june", "codex")
	sessionsDir := filepath.Join(juneCodex, "sessions", "2025", "01", "03")
	os.MkdirAll(sessionsDir, 0755)

	// Create a session file with known ULID
	ulid := "019b825b-b138-7981-898d-2830d3610fc9"
	filename := "rollout-2025-01-03T10-00-00-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{"type":"session_meta"}`), 0644)

	// Find it
	found, err := FindSessionFile(ulid)
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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	now := time.Now()
	juneCodex := filepath.Join(tmpDir, ".june", "codex")
	sessionsDir := filepath.Join(juneCodex, "sessions",
		now.Format("2006"), now.Format("01"), now.Format("02"))
	os.MkdirAll(sessionsDir, 0755)

	ulid := "test-ulid-12345"
	filename := "rollout-" + now.Format("2006-01-02T15-04-05") + "-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{}`), 0644)

	found, err := FindSessionFile(ulid)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := FindSessionFile("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestFindSessionFile_UsesJuneCodexHome(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create session file in ~/.june/codex/sessions/
	juneCodex := filepath.Join(tmpDir, ".june", "codex")
	sessionDir := filepath.Join(juneCodex, "sessions", "2026", "01", "04")
	os.MkdirAll(sessionDir, 0755)

	threadID := "01abc123"
	sessionFile := filepath.Join(sessionDir, threadID+".jsonl")
	os.WriteFile(sessionFile, []byte(`{"test":"data"}`), 0644)

	// FindSessionFile should find it
	found, err := FindSessionFile(threadID)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("FindSessionFile = %q, want %q", found, sessionFile)
	}
}

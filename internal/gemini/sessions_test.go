package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSessionFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create sessions directory and a test file
	sessionsDir := filepath.Join(tmpHome, ".june", "gemini", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	sessionID := "8b6238bf-8332-4fc7-ba9a-2f3323119bb2"
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"type":"init"}`), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindSessionFile(sessionID)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileNotFound(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create empty sessions directory
	sessionsDir := filepath.Join(tmpHome, ".june", "gemini", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := FindSessionFile("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionFilePath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	sessionID := "test-session-123"
	path, err := SessionFilePath(sessionID)
	if err != nil {
		t.Fatalf("SessionFilePath failed: %v", err)
	}

	expected := filepath.Join(tmpHome, ".june", "gemini", "sessions", sessionID+".jsonl")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestSessionsDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir failed: %v", err)
	}

	expected := filepath.Join(tmpHome, ".june", "gemini", "sessions")
	if dir != expected {
		t.Errorf("dir = %q, want %q", dir, expected)
	}
}

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
	}{
		{"valid UUID", "8b6238bf-8332-4fc7-ba9a-2f3323119bb2", false},
		{"valid simple", "test-session-123", false},
		{"empty", "", true},
		{"forward slash", "foo/bar", true},
		{"backslash", "foo\\bar", true},
		{"double dot", "..", true},
		{"leading double dot", "../foo", true},
		{"trailing double dot", "foo/..", true},
		{"middle double dot", "foo..bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionID(tt.sessionID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSessionID(%q) error = %v, wantErr %v", tt.sessionID, err, tt.wantErr)
			}
		})
	}
}

func TestFindSessionFilePathTraversal(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// These should all fail with ErrInvalidSessionID
	invalidIDs := []string{"../etc/passwd", "foo/bar", ".."}
	for _, id := range invalidIDs {
		_, err := FindSessionFile(id)
		if err != ErrInvalidSessionID {
			t.Errorf("FindSessionFile(%q) = %v, want ErrInvalidSessionID", id, err)
		}
	}
}

func TestSessionFilePathPathTraversal(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// These should all fail with ErrInvalidSessionID
	invalidIDs := []string{"../etc/passwd", "foo/bar", ".."}
	for _, id := range invalidIDs {
		_, err := SessionFilePath(id)
		if err != ErrInvalidSessionID {
			t.Errorf("SessionFilePath(%q) = %v, want ErrInvalidSessionID", id, err)
		}
	}
}

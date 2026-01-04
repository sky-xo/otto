package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSessionNotFound is returned when a session file cannot be found
var ErrSessionNotFound = errors.New("session file not found")

// CodexHome returns the Codex home directory
func CodexHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// FindSessionFile finds a session file by thread ID in the June codex home.
// It searches in ~/.june/codex/sessions/YYYY/MM/DD/
func FindSessionFile(threadID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Look in ~/.june/codex/sessions/
	sessionsDir := filepath.Join(home, ".june", "codex", "sessions")

	// First, try today's directory (most common case)
	now := time.Now()
	todayDir := filepath.Join(sessionsDir, now.Format("2006"), now.Format("01"), now.Format("02"))
	if path, found := findInDir(todayDir, threadID); found {
		return path, nil
	}

	// Otherwise, search all directories (less common)
	var foundPath string
	err = filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), threadID) && strings.HasSuffix(info.Name(), ".jsonl") {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if foundPath != "" {
		return foundPath, nil
	}

	return "", ErrSessionNotFound
}

func findInDir(dir string, ulid string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ulid) && strings.HasSuffix(e.Name(), ".jsonl") {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

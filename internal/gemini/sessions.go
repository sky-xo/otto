package gemini

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrSessionNotFound is returned when a session file cannot be found
var ErrSessionNotFound = errors.New("session file not found")

// ErrInvalidSessionID is returned when a session ID contains invalid characters
var ErrInvalidSessionID = errors.New("invalid session ID: must not contain path separators or traversal sequences")

// validateSessionID checks that a session ID does not contain path traversal characters.
func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return ErrInvalidSessionID
	}
	// Reject path separators and traversal sequences
	for _, c := range sessionID {
		if c == '/' || c == '\\' {
			return ErrInvalidSessionID
		}
	}
	// Also reject ".." explicitly
	if sessionID == ".." || len(sessionID) >= 2 && (sessionID[:2] == ".." || sessionID[len(sessionID)-2:] == "..") {
		return ErrInvalidSessionID
	}
	// Check for ".." anywhere in the string
	for i := 0; i < len(sessionID)-1; i++ {
		if sessionID[i] == '.' && sessionID[i+1] == '.' {
			return ErrInvalidSessionID
		}
	}
	return nil
}

// FindSessionFile finds a Gemini session file by session ID.
// Looks in ~/.june/gemini/sessions/{session_id}.jsonl
func FindSessionFile(sessionID string) (string, error) {
	if err := validateSessionID(sessionID); err != nil {
		return "", err
	}

	sessionsDir, err := SessionsDir()
	if err != nil {
		return "", err
	}

	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	if _, err := os.Stat(sessionFile); err != nil {
		if os.IsNotExist(err) {
			return "", ErrSessionNotFound
		}
		return "", err
	}

	return sessionFile, nil
}

// SessionFilePath returns the path where a session file should be written.
func SessionFilePath(sessionID string) (string, error) {
	if err := validateSessionID(sessionID); err != nil {
		return "", err
	}

	sessionsDir, err := SessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(sessionsDir, sessionID+".jsonl"), nil
}

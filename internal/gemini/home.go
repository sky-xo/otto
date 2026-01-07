package gemini

import (
	"os"
	"path/filepath"
)

// EnsureGeminiHome creates the June gemini home at ~/.june/gemini/
// and the sessions subdirectory.
// Returns the path to the gemini home directory.
func EnsureGeminiHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create ~/.june/gemini/
	geminiHome := filepath.Join(home, ".june", "gemini")
	if err := os.MkdirAll(geminiHome, 0755); err != nil {
		return "", err
	}

	// Create sessions subdirectory
	sessionsDir := filepath.Join(geminiHome, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", err
	}

	return geminiHome, nil
}

// SessionsDir returns the path to the sessions directory.
func SessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".june", "gemini", "sessions"), nil
}

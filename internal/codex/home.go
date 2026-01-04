package codex

import (
	"os"
	"path/filepath"
)

// EnsureCodexHome creates an isolated CODEX_HOME at ~/.june/codex/
// and copies auth.json from the user's ~/.codex/ for API access.
// Returns the path to the isolated codex home directory.
func EnsureCodexHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create ~/.june/codex/
	codexHome := filepath.Join(home, ".june", "codex")
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		return "", err
	}

	// Copy auth.json from user's ~/.codex/ if it exists
	userCodex := filepath.Join(home, ".codex")
	authSrc := filepath.Join(userCodex, "auth.json")
	authDst := filepath.Join(codexHome, "auth.json")

	// Only copy if source exists and destination doesn't
	if _, err := os.Stat(authDst); os.IsNotExist(err) {
		if authData, err := os.ReadFile(authSrc); err == nil {
			_ = os.WriteFile(authDst, authData, 0600)
		}
		// Ignore errors - auth.json is optional (user may not have authenticated yet)
	}

	return codexHome, nil
}

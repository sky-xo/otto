package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"june/internal/config"
)

func ensureCodexHome() (string, error) {
	codexHome := filepath.Join(config.DataDir(), "state", "codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		return "", fmt.Errorf("create CODEX_HOME: %w", err)
	}

	realCodexHome := os.Getenv("CODEX_HOME")
	if realCodexHome == "" || realCodexHome == codexHome {
		home, _ := os.UserHomeDir()
		realCodexHome = filepath.Join(home, ".codex")
	}

	authSrc := filepath.Join(realCodexHome, "auth.json")
	authDst := filepath.Join(codexHome, "auth.json")
	// Only copy if destination doesn't exist
	if _, err := os.Stat(authDst); os.IsNotExist(err) {
		if authData, err := os.ReadFile(authSrc); err == nil {
			_ = os.WriteFile(authDst, authData, 0600)
		}
	}

	return codexHome, nil
}

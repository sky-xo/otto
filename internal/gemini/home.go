package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureGeminiHome creates the June gemini home at ~/.june/gemini/
// and the sessions subdirectory. Also copies auth files from ~/.gemini/.
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

	// Copy auth files from user's ~/.gemini/ if they exist
	userGemini := filepath.Join(home, ".gemini")
	oauthMissing := copyAuthFile(userGemini, geminiHome, "oauth_creds.json", 0600)
	accountsMissing := copyAuthFile(userGemini, geminiHome, "google_accounts.json", 0600)

	// Warn if auth files are missing (user may not have logged into Gemini yet)
	if oauthMissing || accountsMissing {
		fmt.Fprintf(os.Stderr, "warning: gemini auth files not found in ~/.gemini/ - run 'gemini' to authenticate\n")
	}

	return geminiHome, nil
}

// copyAuthFile copies a file from src dir to dst dir if source exists and dest doesn't.
// Uses atomic create-if-not-exists pattern.
// Returns true if source was missing (for warning purposes).
func copyAuthFile(srcDir, dstDir, filename string, perm os.FileMode) bool {
	srcPath := filepath.Join(srcDir, filename)
	dstPath := filepath.Join(dstDir, filename)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return true // Source doesn't exist
	}

	// Try to create the file exclusively - fails if it already exists
	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return false // Already exists or other error
	}
	defer f.Close()
	_, _ = f.Write(data)
	return false
}

// SessionsDir returns the path to the sessions directory.
func SessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".june", "gemini", "sessions"), nil
}

// MCPServerConfig represents an MCP server for Gemini's settings.json
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// WriteSettings writes a settings.json file with MCP server configuration.
// Overwrites any existing settings.json.
func WriteSettings(geminiHome string, mcpServers map[string]MCPServerConfig) error {
	if mcpServers == nil {
		mcpServers = make(map[string]MCPServerConfig)
	}

	settings := map[string]interface{}{
		"mcpServers": mcpServers,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(geminiHome, "settings.json"), data, 0600)
}

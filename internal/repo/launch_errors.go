package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"otto/internal/config"
)

// RecordLaunchError writes an error message to a log file for a failed agent launch.
// The error is written to ~/.otto/orchestrators/<scope>/launch-errors/<agent-id>.log
func RecordLaunchError(scopePath, agentID, errorText string) error {
	// Construct path: ~/.otto/orchestrators/<scope>/launch-errors/<agent-id>.log
	errorDir := filepath.Join(config.DataDir(), "orchestrators", scopePath, "launch-errors")
	if err := os.MkdirAll(errorDir, 0o755); err != nil {
		return fmt.Errorf("create launch-errors dir: %w", err)
	}

	errorFile := filepath.Join(errorDir, agentID+".log")
	if err := os.WriteFile(errorFile, []byte(errorText), 0o644); err != nil {
		return fmt.Errorf("write launch error: %w", err)
	}

	return nil
}

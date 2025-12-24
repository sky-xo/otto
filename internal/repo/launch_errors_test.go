package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"otto/internal/config"
	"otto/internal/scope"
)

func TestRecordLaunchError(t *testing.T) {
	// Determine scope
	repoRoot := scope.RepoRoot()
	if repoRoot == "" {
		t.Skip("not in a git repository")
	}

	branch := scope.BranchName()
	if branch == "" {
		branch = "main"
	}

	scopePath := scope.Scope(repoRoot, branch)
	agentID := "test-agent"
	errorText := "failed to start worker: process exited with status 1"

	// Record launch error
	err := RecordLaunchError(scopePath, agentID, errorText)
	if err != nil {
		t.Fatalf("RecordLaunchError failed: %v", err)
	}

	// Verify file was created at correct path
	expectedPath := filepath.Join(config.DataDir(), "orchestrators", scopePath, "launch-errors", agentID+".log")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read launch error file: %v", err)
	}

	// Verify content
	if !strings.Contains(string(content), errorText) {
		t.Errorf("expected error text %q in file, got: %s", errorText, string(content))
	}

	// Cleanup
	os.Remove(expectedPath)
}

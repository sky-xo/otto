package tui

import (
	"testing"

	"github.com/sky-xo/june/internal/codex"
)

func TestConvertCodexEntriesToolUseNormalized(t *testing.T) {
	// Test shell_command -> Bash normalization
	codexEntries := []codex.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: shell_command]",
			ToolName:  "shell_command",
			ToolInput: map[string]interface{}{"command": "go test ./..."},
		},
	}

	entries := convertCodexEntries(codexEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Check that ToolName() returns NORMALIZED name "Bash" (not "shell_command")
	toolName := entries[0].ToolName()
	if toolName != "Bash" {
		t.Errorf("ToolName() = %q, want %q (normalized)", toolName, "Bash")
	}

	// Check that ToolInput() returns the input map with command
	toolInput := entries[0].ToolInput()
	if toolInput == nil {
		t.Fatal("ToolInput() = nil, want map")
	}
	if cmd, ok := toolInput["command"].(string); !ok || cmd != "go test ./..." {
		t.Errorf("ToolInput()[command] = %v, want %q", toolInput["command"], "go test ./...")
	}
}

func TestConvertCodexEntriesReadFileNormalized(t *testing.T) {
	// Test read_file -> Read normalization with path -> file_path
	codexEntries := []codex.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: read_file]",
			ToolName:  "read_file",
			ToolInput: map[string]interface{}{"path": "/tmp/main.go"},
		},
	}

	entries := convertCodexEntries(codexEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Check normalized name
	if entries[0].ToolName() != "Read" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Read")
	}

	// Check normalized parameter key (path -> file_path)
	toolInput := entries[0].ToolInput()
	if fp, ok := toolInput["file_path"].(string); !ok || fp != "/tmp/main.go" {
		t.Errorf("ToolInput()[file_path] = %v, want %q", toolInput["file_path"], "/tmp/main.go")
	}
}

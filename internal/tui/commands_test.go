package tui

import (
	"testing"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/gemini"
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

func TestConvertGeminiEntriesToolUseNormalized(t *testing.T) {
	// Test read_file -> Read normalization with path -> file_path
	geminiEntries := []gemini.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: read_file]",
			ToolName:  "read_file",
			ToolInput: map[string]interface{}{"path": "main.go"},
		},
	}

	entries := convertGeminiEntries(geminiEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Check that ToolName() returns NORMALIZED name "Read" (not "read_file")
	toolName := entries[0].ToolName()
	if toolName != "Read" {
		t.Errorf("ToolName() = %q, want %q (normalized)", toolName, "Read")
	}

	// Check normalized parameter key (path -> file_path)
	toolInput := entries[0].ToolInput()
	if toolInput == nil {
		t.Fatal("ToolInput() = nil, want map")
	}
	if fp, ok := toolInput["file_path"].(string); !ok || fp != "main.go" {
		t.Errorf("ToolInput()[file_path] = %v, want %q", toolInput["file_path"], "main.go")
	}
}

func TestConvertGeminiEntriesShellNormalized(t *testing.T) {
	// Test shell -> Bash normalization
	geminiEntries := []gemini.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: shell]",
			ToolName:  "shell",
			ToolInput: map[string]interface{}{"command": "ls -la"},
		},
	}

	entries := convertGeminiEntries(geminiEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Check normalized name
	if entries[0].ToolName() != "Bash" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Bash")
	}

	// command key stays the same
	toolInput := entries[0].ToolInput()
	if cmd, ok := toolInput["command"].(string); !ok || cmd != "ls -la" {
		t.Errorf("ToolInput()[command] = %v, want %q", toolInput["command"], "ls -la")
	}
}

func TestConvertCodexEntriesRichFormatting(t *testing.T) {
	// Codex shell_command should normalize to Bash and produce rich summary
	codexEntries := []codex.TranscriptEntry{
		{
			Type:      "tool",
			ToolName:  "shell_command",
			ToolInput: map[string]interface{}{
				"command": "go test ./...",
			},
		},
	}

	entries := convertCodexEntries(codexEntries)

	// After normalization: Bash with command
	// ToolSummary should return "Bash: go test ./..."
	summary := entries[0].ToolSummary()
	if summary != "Bash: go test ./..." {
		t.Errorf("ToolSummary() = %q, want %q", summary, "Bash: go test ./...")
	}
}

func TestConvertGeminiEntriesRichFormatting(t *testing.T) {
	// Gemini read_file should normalize to Read with file_path
	geminiEntries := []gemini.TranscriptEntry{
		{
			Type:      "tool",
			ToolName:  "read_file",
			ToolInput: map[string]interface{}{
				"path": "/Users/test/code/project/main.go",
			},
		},
	}

	entries := convertGeminiEntries(geminiEntries)

	// After normalization: Read with file_path
	// ToolSummary should include shortened path
	summary := entries[0].ToolSummary()
	if summary == "" || summary == "Read" {
		t.Errorf("ToolSummary() = %q, want path info like 'Read: project/main.go'", summary)
	}
	if entries[0].ToolName() != "Read" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Read")
	}
}

func TestConvertCodexEntriesNilToolInput(t *testing.T) {
	// Verify that nil ToolInput doesn't cause panic
	codexEntries := []codex.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: shell_command]",
			ToolName:  "shell_command",
			ToolInput: nil, // nil input (can happen with malformed JSON)
		},
	}

	// Should not panic
	entries := convertCodexEntries(codexEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Should still normalize tool name
	if entries[0].ToolName() != "Bash" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Bash")
	}

	// ToolInput should be empty map, not nil
	toolInput := entries[0].ToolInput()
	if toolInput == nil {
		t.Error("ToolInput() = nil, want empty map")
	}
}

func TestConvertCodexEntriesEmptyToolInput(t *testing.T) {
	// Verify that empty ToolInput works correctly
	codexEntries := []codex.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: read_file]",
			ToolName:  "read_file",
			ToolInput: map[string]interface{}{}, // empty map
		},
	}

	// Should not panic
	entries := convertCodexEntries(codexEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Should still normalize tool name
	if entries[0].ToolName() != "Read" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Read")
	}
}

func TestConvertGeminiEntriesNilToolInput(t *testing.T) {
	// Verify that nil ToolInput doesn't cause panic
	geminiEntries := []gemini.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: shell]",
			ToolName:  "shell",
			ToolInput: nil, // nil input (can happen with malformed JSON)
		},
	}

	// Should not panic
	entries := convertGeminiEntries(geminiEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Should still normalize tool name
	if entries[0].ToolName() != "Bash" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Bash")
	}

	// ToolInput should be empty map, not nil
	toolInput := entries[0].ToolInput()
	if toolInput == nil {
		t.Error("ToolInput() = nil, want empty map")
	}
}

func TestConvertGeminiEntriesEmptyToolInput(t *testing.T) {
	// Verify that empty ToolInput works correctly
	geminiEntries := []gemini.TranscriptEntry{
		{
			Type:      "tool",
			Content:   "[tool: read_file]",
			ToolName:  "read_file",
			ToolInput: map[string]interface{}{}, // empty map
		},
	}

	// Should not panic
	entries := convertGeminiEntries(geminiEntries)

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	// Should still normalize tool name
	if entries[0].ToolName() != "Read" {
		t.Errorf("ToolName() = %q, want %q", entries[0].ToolName(), "Read")
	}
}

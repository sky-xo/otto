package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEntryInit(t *testing.T) {
	data := []byte(`{"type":"init","timestamp":"2026-01-07T10:02:12.875Z","session_id":"8b6238bf","model":"auto-gemini-3"}`)
	entry := parseEntry(data)

	if entry.Type != "" {
		t.Errorf("init should be skipped, got Type=%q", entry.Type)
	}
}

func TestParseEntryUserMessage(t *testing.T) {
	data := []byte(`{"type":"message","timestamp":"...","role":"user","content":"Fix the bug"}`)
	entry := parseEntry(data)

	if entry.Type != "user" {
		t.Errorf("Type = %q, want %q", entry.Type, "user")
	}
	if entry.Content != "Fix the bug" {
		t.Errorf("Content = %q, want %q", entry.Content, "Fix the bug")
	}
}

func TestParseEntryAssistantMessage(t *testing.T) {
	data := []byte(`{"type":"message","timestamp":"...","role":"assistant","content":"I fixed it","delta":true}`)
	entry := parseEntry(data)

	if entry.Type != "message" {
		t.Errorf("Type = %q, want %q", entry.Type, "message")
	}
	if entry.Content != "I fixed it" {
		t.Errorf("Content = %q, want %q", entry.Content, "I fixed it")
	}
}

func TestParseEntryToolUse(t *testing.T) {
	data := []byte(`{"type":"tool_use","timestamp":"...","tool_name":"read_file","tool_id":"abc","parameters":{"path":"main.go"}}`)
	entry := parseEntry(data)

	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.Content != "[tool: read_file]" {
		t.Errorf("Content = %q, want %q", entry.Content, "[tool: read_file]")
	}
}

func TestParseEntryToolResult(t *testing.T) {
	data := []byte(`{"type":"tool_result","timestamp":"...","tool_id":"abc","status":"success","output":"file contents here"}`)
	entry := parseEntry(data)

	if entry.Type != "tool_output" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool_output")
	}
	if entry.Content != "file contents here" {
		t.Errorf("Content = %q, want %q", entry.Content, "file contents here")
	}
}

func TestParseEntryResult(t *testing.T) {
	data := []byte(`{"type":"result","timestamp":"...","status":"success","stats":{"total_tokens":100}}`)
	entry := parseEntry(data)

	// Result events are skipped (just stats)
	if entry.Type != "" {
		t.Errorf("result should be skipped, got Type=%q", entry.Type)
	}
}

func TestReadTranscriptAccumulatesDeltas(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")

	content := `{"type":"init","session_id":"abc"}
{"type":"message","role":"user","content":"Hello"}
{"type":"message","role":"assistant","content":"Hi","delta":true}
{"type":"message","role":"assistant","content":" there","delta":true}
{"type":"message","role":"assistant","content":"!","delta":true}
{"type":"tool_use","tool_name":"read_file","tool_id":"t1"}
{"type":"result","status":"success"}
`
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, _, err := ReadTranscript(sessionFile, 0)
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	// Should have: user message, accumulated assistant message, tool
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].Type != "user" || entries[0].Content != "Hello" {
		t.Errorf("entries[0] = %+v, want user/Hello", entries[0])
	}

	if entries[1].Type != "message" || entries[1].Content != "Hi there!" {
		t.Errorf("entries[1] = %+v, want message/'Hi there!'", entries[1])
	}

	if entries[2].Type != "tool" {
		t.Errorf("entries[2].Type = %q, want tool", entries[2].Type)
	}
}

func TestParseEntryToolUseWithParameters(t *testing.T) {
	data := []byte(`{"type":"tool_use","timestamp":"...","tool_name":"read_file","tool_id":"abc","parameters":{"path":"main.go","encoding":"utf-8"}}`)
	entry := parseEntry(data)

	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.ToolName != "read_file" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "read_file")
	}
	if entry.ToolInput == nil {
		t.Fatal("ToolInput is nil, want map with path")
	}
	if path, ok := entry.ToolInput["path"].(string); !ok || path != "main.go" {
		t.Errorf("ToolInput[path] = %v, want %q", entry.ToolInput["path"], "main.go")
	}
}

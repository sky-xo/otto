package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseEntryReasoning(t *testing.T) {
	// Actual Codex format: type is "response_item", payload.type is "reasoning", summary[0].text has content
	data := []byte(`{"type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"**Thinking about this**"}]}}`)

	entry := parseEntry(data)

	if entry.Type != "reasoning" {
		t.Errorf("Type = %q, want %q", entry.Type, "reasoning")
	}
	if entry.Content != "**Thinking about this**" {
		t.Errorf("Content = %q, want %q", entry.Content, "**Thinking about this**")
	}
}

func TestParseEntryFunctionCall(t *testing.T) {
	// Actual Codex format: type is "response_item", payload.type is "function_call"
	data := []byte(`{"type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{}"}}`)

	entry := parseEntry(data)

	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.Content != "[tool: shell_command]" {
		t.Errorf("Content = %q, want %q", entry.Content, "[tool: shell_command]")
	}
}

func TestParseEntryFunctionCallWithArguments(t *testing.T) {
	data := []byte(`{"type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{\"command\":\"go test ./...\",\"workdir\":\"/tmp\"}"}}`)

	entry := parseEntry(data)

	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.ToolName != "shell_command" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "shell_command")
	}
	if entry.ToolInput == nil {
		t.Fatal("ToolInput is nil, want map with command")
	}
	if cmd, ok := entry.ToolInput["command"].(string); !ok || cmd != "go test ./..." {
		t.Errorf("ToolInput[command] = %v, want %q", entry.ToolInput["command"], "go test ./...")
	}
}

func TestParseEntryFunctionCallMalformedArguments(t *testing.T) {
	// Malformed JSON in arguments field - should handle gracefully (not panic)
	data := []byte(`{"type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{invalid json here"}}`)

	entry := parseEntry(data)

	// Should still return a valid tool entry
	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.ToolName != "shell_command" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "shell_command")
	}
	// ToolInput should be nil when JSON parsing fails
	if entry.ToolInput != nil {
		t.Errorf("ToolInput = %v, want nil for malformed JSON", entry.ToolInput)
	}
	// Content should still be populated
	if entry.Content != "[tool: shell_command]" {
		t.Errorf("Content = %q, want %q", entry.Content, "[tool: shell_command]")
	}
}

func TestParseEntryFunctionCallOutput(t *testing.T) {
	// Actual Codex format: type is "response_item", payload.type is "function_call_output"
	data := []byte(`{"type":"response_item","payload":{"type":"function_call_output","output":"Exit code: 0\nOutput: hello"}}`)

	entry := parseEntry(data)

	if entry.Type != "tool_output" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool_output")
	}
	if entry.Content != "Exit code: 0\nOutput: hello" {
		t.Errorf("Content = %q, want %q", entry.Content, "Exit code: 0\nOutput: hello")
	}
}

func TestParseEntryFunctionCallOutputTruncation(t *testing.T) {
	// Long output should be truncated to 200 chars
	longOutput := make([]byte, 300)
	for i := range longOutput {
		longOutput[i] = 'x'
	}
	data := []byte(`{"type":"response_item","payload":{"type":"function_call_output","output":"` + string(longOutput) + `"}}`)

	entry := parseEntry(data)

	if entry.Content == "" {
		t.Fatal("Content is empty, parser not working")
	}
	if len(entry.Content) != 203 { // 200 + "..."
		t.Errorf("Content length = %d, want 203", len(entry.Content))
	}
	if entry.Content[200:] != "..." {
		t.Errorf("Content should end with '...'")
	}
}

func TestParseEntryFunctionCallOutputUTF8Truncation(t *testing.T) {
	// Create output with multi-byte characters that would be split by byte truncation
	// 250 emoji (each is 4 bytes in UTF-8) = 250 runes, 1000 bytes
	longOutput := strings.Repeat("🎉", 250)
	data := []byte(`{"type":"response_item","payload":{"type":"function_call_output","output":"` + longOutput + `"}}`)

	entry := parseEntry(data)

	if entry.Content == "" {
		t.Fatal("Content is empty, parser not working")
	}

	// Should be 200 runes + "..." = 203 runes, all valid UTF-8
	runes := []rune(entry.Content)
	if len(runes) != 203 {
		t.Errorf("rune count = %d, want 203", len(runes))
	}
	// Verify it's valid UTF-8 (no split chars)
	if !utf8.ValidString(entry.Content) {
		t.Error("Content is not valid UTF-8")
	}
	// Verify it ends with "..."
	if !strings.HasSuffix(entry.Content, "...") {
		t.Error("Content should end with '...'")
	}
}

func TestParseEntryMessageWithOutputText(t *testing.T) {
	// Actual Codex format: type is "response_item", payload.type is "message", content[0].type is "output_text"
	data := []byte(`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello! I'm Codex, your coding teammate."}]}}`)

	entry := parseEntry(data)

	if entry.Type != "message" {
		t.Errorf("Type = %q, want %q", entry.Type, "message")
	}
	if entry.Content != "Hello! I'm Codex, your coding teammate." {
		t.Errorf("Content = %q, want %q", entry.Content, "Hello! I'm Codex, your coding teammate.")
	}
}

func TestReadTranscriptWithRealFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")

	content := `{"type":"session_meta","payload":{}}
{"type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"Thinking..."}]}}
{"type":"response_item","payload":{"type":"function_call","name":"shell_command"}}
{"type":"response_item","payload":{"type":"function_call_output","output":"done"}}
`
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	entries, lineCount, err := ReadTranscript(sessionFile, 0)
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	if lineCount != 4 {
		t.Errorf("lineCount = %d, want 4", lineCount)
	}

	// Should have 3 entries (session_meta is ignored)
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].Type != "reasoning" {
		t.Errorf("entries[0].Type = %q, want %q", entries[0].Type, "reasoning")
	}
	if entries[1].Type != "tool" {
		t.Errorf("entries[1].Type = %q, want %q", entries[1].Type, "tool")
	}
	if entries[2].Type != "tool_output" {
		t.Errorf("entries[2].Type = %q, want %q", entries[2].Type, "tool_output")
	}
}

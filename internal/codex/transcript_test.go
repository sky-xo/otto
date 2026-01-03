package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseEntryAgentReasoning(t *testing.T) {
	// Actual Codex format: type is "event_msg", payload.type is "agent_reasoning", payload.text has content
	data := []byte(`{"type":"event_msg","payload":{"type":"agent_reasoning","text":"**Preparing to run shell command**"}}`)

	entry := parseEntry(data)

	if entry.Type != "reasoning" {
		t.Errorf("Type = %q, want %q", entry.Type, "reasoning")
	}
	if entry.Content != "**Preparing to run shell command**" {
		t.Errorf("Content = %q, want %q", entry.Content, "**Preparing to run shell command**")
	}
}

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

func TestParseEntryMessage(t *testing.T) {
	// Actual Codex format for message: type is "response_item", payload.type is "message", payload.text has content
	data := []byte(`{"type":"response_item","payload":{"type":"message","text":"Here is my response to your question"}}`)

	entry := parseEntry(data)

	if entry.Type != "message" {
		t.Errorf("Type = %q, want %q", entry.Type, "message")
	}
	if entry.Content != "Here is my response to your question" {
		t.Errorf("Content = %q, want %q", entry.Content, "Here is my response to your question")
	}
}

func TestReadTranscriptWithRealFormat(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")

	content := `{"type":"session_meta","payload":{}}
{"type":"event_msg","payload":{"type":"agent_reasoning","text":"Thinking..."}}
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

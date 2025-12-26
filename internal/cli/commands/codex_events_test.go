package commands

import (
	"testing"
)

func TestParseCodexEventCompaction(t *testing.T) {
	line := `{"type":"context_compacted"}`
	event := ParseCodexEvent(line)
	if event.Type != "context_compacted" {
		t.Fatalf("expected type = context_compacted, got %q", event.Type)
	}
}

func TestParseCodexEventThreadStarted(t *testing.T) {
	line := `{"type":"thread.started","thread_id":"abc123"}`
	event := ParseCodexEvent(line)
	if event.Type != "thread.started" {
		t.Fatalf("expected type = thread.started, got %q", event.Type)
	}
	if event.ThreadID != "abc123" {
		t.Fatalf("expected thread_id = abc123, got %q", event.ThreadID)
	}
}

func TestParseCodexEventItemCompletedReasoning(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"reasoning","text":"thinking about the problem"}}`
	event := ParseCodexEvent(line)
	if event.Type != "item.completed" {
		t.Fatalf("expected type = item.completed, got %q", event.Type)
	}
	if event.Item == nil {
		t.Fatal("expected item to be non-nil")
	}
	if event.Item.Type != "reasoning" {
		t.Fatalf("expected item.type = reasoning, got %q", event.Item.Type)
	}
	if event.Item.Text != "thinking about the problem" {
		t.Fatalf("expected item.text = 'thinking about the problem', got %q", event.Item.Text)
	}
}

func TestParseCodexEventItemCompletedCommandExecution(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"command_execution","command":"ls -la","aggregated_output":"total 42\ndrwxr-xr-x","exit_code":0}}`
	event := ParseCodexEvent(line)
	if event.Type != "item.completed" {
		t.Fatalf("expected type = item.completed, got %q", event.Type)
	}
	if event.Item == nil {
		t.Fatal("expected item to be non-nil")
	}
	if event.Item.Type != "command_execution" {
		t.Fatalf("expected item.type = command_execution, got %q", event.Item.Type)
	}
	if event.Item.Command != "ls -la" {
		t.Fatalf("expected item.command = 'ls -la', got %q", event.Item.Command)
	}
	if event.Item.AggregatedOutput != "total 42\ndrwxr-xr-x" {
		t.Fatalf("expected aggregated_output, got %q", event.Item.AggregatedOutput)
	}
	if event.Item.ExitCode == nil {
		t.Fatal("expected exit_code to be non-nil")
	}
	if *event.Item.ExitCode != 0 {
		t.Fatalf("expected exit_code = 0, got %d", *event.Item.ExitCode)
	}
}

func TestParseCodexEventItemCompletedAgentMessage(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"agent_message","text":"Hello world"}}`
	event := ParseCodexEvent(line)
	if event.Type != "item.completed" {
		t.Fatalf("expected type = item.completed, got %q", event.Type)
	}
	if event.Item == nil {
		t.Fatal("expected item to be non-nil")
	}
	if event.Item.Type != "agent_message" {
		t.Fatalf("expected item.type = agent_message, got %q", event.Item.Type)
	}
	if event.Item.Text != "Hello world" {
		t.Fatalf("expected item.text = 'Hello world', got %q", event.Item.Text)
	}
}

func TestParseCodexEventTurnFailed(t *testing.T) {
	line := `{"type":"turn.failed","status":"error"}`
	event := ParseCodexEvent(line)
	if event.Type != "turn.failed" {
		t.Fatalf("expected type = turn.failed, got %q", event.Type)
	}
	if event.Status != "error" {
		t.Fatalf("expected status = error, got %q", event.Status)
	}
}

func TestParseCodexEventMalformedJSON(t *testing.T) {
	line := `{not valid json`
	event := ParseCodexEvent(line)
	if event.Type != "" {
		t.Fatalf("expected empty event for malformed JSON, got type %q", event.Type)
	}
}

func TestParseCodexEventRawPreserved(t *testing.T) {
	line := `{"type":"context_compacted"}`
	event := ParseCodexEvent(line)
	if event.Raw != line {
		t.Fatalf("expected Raw to be preserved as %q, got %q", line, event.Raw)
	}
}

func TestParseCodexEventNonJSON(t *testing.T) {
	line := `This is just regular text output`
	event := ParseCodexEvent(line)
	if event.Type != "" {
		t.Fatalf("expected empty event for non-JSON, got type %q", event.Type)
	}
}

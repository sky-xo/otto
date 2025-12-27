package commands

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestRunPeek(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create log entries
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: "line 1", Valid: true}})
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: "line 2", Valid: true}})

	var buf bytes.Buffer

	// First peek should show all entries
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}
	if !strings.Contains(buf.String(), "line 1") {
		t.Errorf("first peek should show line 1")
	}

	// Second peek should show nothing (cursor advanced)
	buf.Reset()
	err = runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No new log entries") {
		t.Errorf("second peek should say no new entries, got: %s", buf.String())
	}

	// Add new entry
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: "line 3", Valid: true}})

	// Third peek should show only new entry
	buf.Reset()
	err = runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}
	if !strings.Contains(buf.String(), "line 3") {
		t.Errorf("third peek should show line 3")
	}
	if strings.Contains(buf.String(), "line 1") {
		t.Errorf("third peek should NOT show line 1")
	}
}

func TestRunPeek_ItemStartedWithCommand(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create item.started log entry with command
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "item.started",
		Command:   sql.NullString{String: "rg -n \"foo\" .", Valid: true},
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[running] rg -n \"foo\" .") {
		t.Errorf("expected '[running] rg -n \"foo\" .', got: %s", output)
	}
}

func TestRunPeek_ItemStartedWithContent(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create item.started log entry with content (non-command item)
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "item.started",
		Content:   sql.NullString{String: "Analyzing code structure", Valid: true},
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[starting] Analyzing code structure") {
		t.Errorf("expected '[starting] Analyzing code structure', got: %s", output)
	}
}

func TestRunPeek_ItemStartedCommandTakesPrecedence(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create item.started log entry with both command and content - command should take precedence
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "item.started",
		Command:   sql.NullString{String: "cat file.go", Valid: true},
		Content:   sql.NullString{String: "Reading file", Valid: true},
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[running] cat file.go") {
		t.Errorf("expected '[running] cat file.go', got: %s", output)
	}
	if strings.Contains(output, "[starting]") {
		t.Errorf("should not contain [starting] when command is present, got: %s", output)
	}
}

func TestRunPeek_TurnEvents(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create turn.started and turn.completed log entries
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "turn.started",
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "output",
		Content:   sql.NullString{String: "Some output", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "turn.completed",
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()

	// Verify sequence appears in order
	expectedSequence := []string{
		"--- turn started ---",
		"Some output",
		"--- turn completed ---",
	}

	lastIndex := -1
	for _, expected := range expectedSequence {
		index := strings.Index(output, expected)
		if index == -1 {
			t.Errorf("expected output to contain %q", expected)
		} else if index <= lastIndex {
			t.Errorf("expected %q to appear after previous item, but found at index %d (last was %d)", expected, index, lastIndex)
		}
		lastIndex = index
	}
}

func TestRunPeek_MixedEventTypes(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create a realistic sequence of events
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "turn.started",
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "item.started",
		Command:   sql.NullString{String: "rg -n \"foo\" .", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "output",
		ToolName:  sql.NullString{String: "Grep", Valid: true},
		Content:   sql.NullString{String: "src/main.go:42: foo", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "item.started",
		Command:   sql.NullString{String: "cat src/main.go", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "output",
		ToolName:  sql.NullString{String: "Read", Valid: true},
		Content:   sql.NullString{String: "package main...", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "output",
		Content:   sql.NullString{String: "**Thinking about the problem**", Valid: true},
	})
	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "turn.completed",
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()

	// Verify sequence appears in order
	expectedSequence := []string{
		"--- turn started ---",
		"[running] rg -n \"foo\" .",
		"[Grep] src/main.go:42: foo",
		"[running] cat src/main.go",
		"[Read] package main...",
		"**Thinking about the problem**",
		"--- turn completed ---",
	}

	lastIndex := -1
	for _, expected := range expectedSequence {
		index := strings.Index(output, expected)
		if index == -1 {
			t.Errorf("expected output to contain %q", expected)
		} else if index <= lastIndex {
			t.Errorf("expected %q to appear after previous item, but found at index %d (last was %d)", expected, index, lastIndex)
		}
		lastIndex = index
	}
}

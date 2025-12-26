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

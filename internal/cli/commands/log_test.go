package commands

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestRunLog(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create agent
	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create some log entries
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: "line 1", Valid: true}})
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: "line 2", Valid: true}})
	repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "error", Content: sql.NullString{String: "error 1", Valid: true}})

	var buf bytes.Buffer
	err := runLog(conn, "test-agent", 0, &buf)
	if err != nil {
		t.Fatalf("runLog failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "line 1") || !strings.Contains(output, "line 2") {
		t.Errorf("expected output to contain log entries, got: %s", output)
	}
}

func TestRunLogWithTail(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create 10 entries
	for i := 0; i < 10; i++ {
		repo.CreateLogEntry(conn, repo.LogEntry{Project: ctx.Project, Branch: ctx.Branch, AgentName: "test-agent", AgentType: "claude", EventType: "output", Content: sql.NullString{String: fmt.Sprintf("line %d", i), Valid: true}})
	}

	var buf bytes.Buffer
	err := runLog(conn, "test-agent", 3, &buf)
	if err != nil {
		t.Fatalf("runLog failed: %v", err)
	}

	output := buf.String()
	// Should only have last 3 lines
	if strings.Contains(output, "line 0") {
		t.Errorf("should not contain line 0 with --tail 3")
	}
	if !strings.Contains(output, "line 9") {
		t.Errorf("should contain line 9")
	}
}

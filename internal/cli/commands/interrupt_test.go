package commands

import (
	"testing"

	"otto/internal/repo"
)

// Note: Full integration test of runInterrupt with real process signaling
// would require spawning a real process. The tests below cover validation logic.
// Manual testing: otto spawn claude "task", then otto interrupt <id>

func TestInterruptAgentNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Test agent not found
	err := runInterrupt(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if err.Error() != `agent "nonexistent" not found` {
		t.Fatalf("expected 'agent not found' error, got: %v", err)
	}
}

func TestInterruptRequiresPID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := testCtx()

	// Create an agent without a PID
	agent := repo.Agent{
		Project: ctx.Project,
		Branch:  ctx.Branch,
		Name:    "nopidagent",
		Type:    "claude",
		Task:    "test task",
		Status:  "busy",
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Should fail because agent has no PID
	err := runInterrupt(db, "nopidagent")
	if err == nil {
		t.Fatal("expected error for agent without PID")
	}
	if err.Error() != `agent "nopidagent" has no PID` {
		t.Fatalf("expected 'has no PID' error, got: %v", err)
	}
}

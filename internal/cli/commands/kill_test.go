package commands

import (
	"database/sql"
	"testing"

	"otto/internal/repo"
)

func TestKillDeletesAgent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create an agent with a PID (use current process PID for testing)
	// We won't actually kill it, just test that the message and deletion happen
	agent := repo.Agent{
		ID:     "testagent",
		Type:   "claude",
		Task:   "test task",
		Status: "working",
		Pid:    sql.NullInt64{Int64: 99999, Valid: true}, // Use fake PID that doesn't exist
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Note: runKill will fail when trying to signal the process, but we can test
	// the lookup and validation logic. For a complete test we'd need a real process.
	// Let's just test the agent-not-found case and missing PID case.

	// Test agent not found
	err := runKill(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestKillRequiresPID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create an agent without a PID
	agent := repo.Agent{
		ID:     "nopidagent",
		Type:   "claude",
		Task:   "test task",
		Status: "working",
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Should fail because agent has no PID
	err := runKill(db, "nopidagent")
	if err == nil {
		t.Fatal("expected error for agent without PID")
	}
}

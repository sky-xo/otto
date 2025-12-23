# Agent Lifecycle Implementation Plan

> **For Claude:** Use TDD to implement this plan task-by-task.

**Goal:** Auto-delete agents on completion/exit, add `otto kill` command.

**Architecture:** DELETE agent rows instead of updating status. Messages table is the history.

**Tech Stack:** Go, Cobra, SQLite

---

### Task 1: Add DeleteAgent function to repo

**Files:**
- Modify: `internal/repo/agents.go`
- Modify: `internal/repo/agents_test.go`

**Step 1: Write failing test**

```go
func TestDeleteAgent(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{ID: "test", Type: "claude", Task: "test task", Status: "working"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := DeleteAgent(conn, "test"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := GetAgent(conn, "test")
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}
```

**Step 2: Implement DeleteAgent**

```go
func DeleteAgent(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}
```

**Step 3: Run tests, commit**

---

### Task 2: Modify complete.go to DELETE agent

After posting completion message, call `repo.DeleteAgent` instead of updating status.

---

### Task 3: Modify spawn.go to DELETE agent on exit

Post exit message, then DELETE agent instead of updating status.

---

### Task 4: Modify watch.go to DELETE dead agents

When dead PID detected, post message then DELETE agent.

---

### Task 5: Create otto kill command

**File:** `internal/cli/commands/kill.go`

- Look up agent, get PID
- SIGTERM the process
- Post "KILLED: by orchestrator" message
- DELETE agent row
- Wire into root.go

---

### Notes

- Messages table is the source of truth for history
- Agents table only contains actively running processes

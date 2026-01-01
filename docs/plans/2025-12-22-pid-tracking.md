# PID Tracking Implementation Plan

> **For Claude:** Use TDD to implement this plan task-by-task.

**Goal:** Add PID tracking to detect and clean up stale "working" agents whose processes have died.

**Architecture:** Store PID when spawning agents. In watch/status, check if PIDs are alive; if dead, update status to "done".

**Tech Stack:** Go, syscall for process checking, SQLite

### Task 1: Add pid column to schema and Agent struct

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/repo/agents.go`

**Step 1: Update schema in db.go**

Add `pid INTEGER` column after `session_id`:

```go
const schemaSQL = `-- agents table
CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  task TEXT NOT NULL,
  status TEXT NOT NULL,
  session_id TEXT,
  pid INTEGER,
  worktree_path TEXT,
  ...
);
```

**Step 2: Update Agent struct in agents.go**

```go
type Agent struct {
	ID        string
	Type      string
	Task      string
	Status    string
	SessionID sql.NullString
	Pid       sql.NullInt64
}
```

**Step 3: Update CreateAgent to include pid**

```go
func CreateAgent(db *sql.DB, a Agent) error {
	_, err := db.Exec(`INSERT INTO agents (id, type, task, status, session_id, pid) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.Type, a.Task, a.Status, a.SessionID, a.Pid)
	return err
}
```

**Step 4: Update GetAgent and ListAgents to read pid**

Update the SELECT and Scan calls to include pid.

**Step 5: Run tests**

```bash
go test ./internal/db ./internal/repo -v
```

### Task 2: Add isProcessAlive helper

**Files:**
- Create: `internal/process/process.go`
- Create: `internal/process/process_test.go`

**Step 1: Write failing test**

```go
func TestIsProcessAlive(t *testing.T) {
	// Current process should be alive
	if !IsProcessAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}

	// PID 0 or negative should return false
	if IsProcessAlive(0) {
		t.Fatal("PID 0 should not be alive")
	}
}
```

**Step 2: Implement helper**

```go
package process

import (
	"os"
	"syscall"
)

// IsProcessAlive checks if a process with the given PID is still running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send anything, just checks if process exists
	return proc.Signal(syscall.Signal(0)) == nil
}
```

**Step 3: Run tests**

```bash
go test ./internal/process -v
```

### Task 3: Store PID when spawning

**Files:**
- Modify: `internal/exec/runner.go`
- Modify: `internal/cli/commands/spawn.go`

**Step 1: Update Runner interface to return PID**

```go
type Runner interface {
	Start(name string, args ...string) (pid int, wait func() error, err error)
}
```

**Step 2: Implement in DefaultRunner**

```go
func (r *DefaultRunner) Start(name string, args ...string) (int, func() error, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return 0, nil, err
	}
	return cmd.Process.Pid, cmd.Wait, nil
}
```

**Step 3: Update spawn.go to use Start and store PID**

```go
pid, wait, err := runner.Start(cmdArgs[0], cmdArgs[1:]...)
if err != nil {
	return fmt.Errorf("spawn %s: %w", agentType, err)
}

// Update agent with PID
_ = repo.UpdateAgentPid(db, agentID, pid)

// Wait for process
if err := wait(); err != nil {
	_ = repo.UpdateAgentStatus(db, agentID, "failed")
	return fmt.Errorf("spawn %s: %w", agentType, err)
}
```

**Step 4: Add UpdateAgentPid to repo**

```go
func UpdateAgentPid(db *sql.DB, id string, pid int) error {
	_, err := db.Exec(`UPDATE agents SET pid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, pid, id)
	return err
}
```

**Step 5: Run tests**

```bash
go test ./internal/cli/commands -v
```

### Task 4: Check PIDs in watch/status and update stale agents

**Files:**
- Modify: `internal/cli/commands/watch.go` or add helper

**Step 1: Add helper function**

```go
func cleanupStaleAgents(db *sql.DB) {
	agents, err := repo.ListAgents(db)
	if err != nil {
		return
	}
	for _, a := range agents {
		if a.Status == "working" && a.Pid.Valid {
			if !process.IsProcessAlive(int(a.Pid.Int64)) {
				_ = repo.UpdateAgentStatus(db, a.ID, "done")
			}
		}
	}
}
```

**Step 2: Call in watch loop**

Add `cleanupStaleAgents(conn)` in the watch polling loop.

**Step 3: Test manually**

1. Spawn an agent
2. Kill the agent process externally
3. Run `june watch` - agent should be marked done

### Notes

- The pid column is nullable for backward compatibility with existing agents
- Agents without PIDs are left as-is (won't be auto-cleaned)
- Process check is cross-platform (os.FindProcess + Signal(0))

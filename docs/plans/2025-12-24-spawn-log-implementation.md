# Spawn & Log Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--detach` flag to spawn, rename transcript_entries to logs, and add `otto peek` and `otto log` commands for CLI access to agent output.

**Architecture:** Schema migration renames table and adds cursor tracking. Repo layer gets renamed functions plus new peek/log queries. Two new CLI commands follow codex-subagent's API exactly.

**Tech Stack:** Go, SQLite, Cobra CLI

---

## Task 1: Schema Migration - Rename transcript_entries to logs

**Files:**
- Modify: `internal/db/db.go:12-55` (schema and migrations)

**Step 1: Write the failing test**

```go
// internal/db/db_test.go - add to existing file
func TestLogsTableExists(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Verify logs table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Errorf("logs table should exist: %v", err)
	}

	// Verify old table doesn't exist (or is aliased)
	_, err = db.QueryRow("SELECT COUNT(*) FROM transcript_entries").Scan(&count)
	// This should still work due to migration handling old DBs
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db -v -run TestLogsTableExists`
Expected: FAIL with "no such table: logs"

**Step 3: Update schema to use logs table name**

In `internal/db/db.go`, update schemaSQL:
- Change `CREATE TABLE IF NOT EXISTS transcript_entries` → `CREATE TABLE IF NOT EXISTS logs`
- Change `CREATE INDEX IF NOT EXISTS idx_transcript_agent ON transcript_entries` → `CREATE INDEX IF NOT EXISTS idx_logs_agent ON logs`

Update cleanupOldData:
- Change `DELETE FROM transcript_entries` → `DELETE FROM logs`

Add migration in ensureSchema:
```go
// Migration: rename transcript_entries to logs
_, _ = conn.Exec(`ALTER TABLE transcript_entries RENAME TO logs`)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db -v -run TestLogsTableExists`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): rename transcript_entries table to logs"
```

---

## Task 2: Schema Migration - Add last_read_log_id to agents

**Files:**
- Modify: `internal/db/db.go:70-81` (ensureSchema)
- Modify: `internal/repo/agents.go` (Agent struct and queries)
- Modify: `internal/repo/agents_test.go`

**Step 1: Write the failing test**

```go
// internal/repo/agents_test.go - add to existing file
func TestAgentLastReadLogID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	agent := Agent{
		ID:     "test-agent",
		Type:   "claude",
		Task:   "test task",
		Status: "busy",
	}
	if err := CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Update last read log ID
	if err := UpdateAgentLastReadLogID(db, "test-agent", "log-123"); err != nil {
		t.Fatalf("update last read: %v", err)
	}

	// Verify it was saved
	got, err := GetAgent(db, "test-agent")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if !got.LastReadLogID.Valid || got.LastReadLogID.String != "log-123" {
		t.Errorf("expected LastReadLogID='log-123', got %v", got.LastReadLogID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -v -run TestAgentLastReadLogID`
Expected: FAIL with undefined: UpdateAgentLastReadLogID

**Step 3: Implement the changes**

Add to `internal/repo/agents.go`:

1. Add field to Agent struct:
```go
type Agent struct {
	ID            string
	Type          string
	Task          string
	Status        string
	SessionID     sql.NullString
	Pid           sql.NullInt64
	CompletedAt   sql.NullTime
	LastReadLogID sql.NullString  // Add this field
}
```

2. Update GetAgent query:
```go
func GetAgent(db *sql.DB, id string) (Agent, error) {
	var a Agent
	err := db.QueryRow(`SELECT id, type, task, status, session_id, pid, completed_at, last_read_log_id FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompletedAt, &a.LastReadLogID)
	return a, err
}
```

3. Update ListAgents query similarly.

4. Add new function:
```go
func UpdateAgentLastReadLogID(db *sql.DB, id, logID string) error {
	_, err := db.Exec(`UPDATE agents SET last_read_log_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, logID, id)
	return err
}
```

Add migration to `internal/db/db.go`:
```go
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN last_read_log_id TEXT`)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -v -run TestAgentLastReadLogID`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/repo/agents.go internal/repo/agents_test.go
git commit -m "feat(repo): add last_read_log_id cursor to agents"
```

---

## Task 3: Rename repo functions - TranscriptEntry to LogEntry

**Files:**
- Rename: `internal/repo/transcripts.go` → `internal/repo/logs.go`
- Rename: `internal/repo/transcripts_test.go` → `internal/repo/logs_test.go`
- Modify: `internal/cli/commands/transcript_capture.go`
- Modify: `internal/tui/watch.go`

**Step 1: Rename files**

```bash
cd /Users/glowy/code/otto/.worktrees/spawn-log
git mv internal/repo/transcripts.go internal/repo/logs.go
git mv internal/repo/transcripts_test.go internal/repo/logs_test.go
```

**Step 2: Update type and function names in logs.go**

In `internal/repo/logs.go`:
- `TranscriptEntry` → `LogEntry`
- `CreateTranscriptEntry` → `CreateLogEntry`
- `ListTranscriptEntries` → `ListLogs`
- Update SQL to use `logs` table

```go
package repo

import (
	"database/sql"

	"github.com/google/uuid"
)

type LogEntry struct {
	ID        string
	AgentID   string
	Direction string
	Stream    sql.NullString
	Content   string
	CreatedAt string
}

func CreateLogEntry(db *sql.DB, agentID, direction, stream, content string) error {
	var streamValue sql.NullString
	if stream != "" {
		streamValue = sql.NullString{String: stream, Valid: true}
	}
	_, err := db.Exec(
		`INSERT INTO logs (id, agent_id, direction, stream, content) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(),
		agentID,
		direction,
		streamValue,
		content,
	)
	return err
}

func ListLogs(db *sql.DB, agentID, sinceID string) ([]LogEntry, error) {
	query := `SELECT id, agent_id, direction, stream, content, created_at FROM logs WHERE agent_id = ?`
	args := []interface{}{agentID}
	// ... rest of existing logic with updated table name
}
```

**Step 3: Update test file**

In `internal/repo/logs_test.go`:
- Update test function names and references

**Step 4: Update callers**

In `internal/cli/commands/transcript_capture.go`:
- `repo.CreateTranscriptEntry` → `repo.CreateLogEntry`

In `internal/tui/watch.go`:
- `repo.ListTranscriptEntries` → `repo.ListLogs`
- `repo.TranscriptEntry` → `repo.LogEntry`

**Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor(repo): rename TranscriptEntry to LogEntry, transcript_entries to logs"
```

---

## Task 4: Add ListLogsWithTail function

**Files:**
- Modify: `internal/repo/logs.go`
- Modify: `internal/repo/logs_test.go`

**Step 1: Write the failing test**

```go
// internal/repo/logs_test.go
func TestListLogsWithTail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create 10 log entries
	for i := 0; i < 10; i++ {
		if err := CreateLogEntry(db, "agent-1", "out", "stdout", fmt.Sprintf("line %d", i)); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	// Get last 3
	logs, err := ListLogsWithTail(db, "agent-1", 3)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}

	// Should be the last 3 (lines 7, 8, 9)
	if logs[0].Content != "line 7" {
		t.Errorf("expected first to be 'line 7', got %q", logs[0].Content)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -v -run TestListLogsWithTail`
Expected: FAIL with undefined: ListLogsWithTail

**Step 3: Implement ListLogsWithTail**

```go
func ListLogsWithTail(db *sql.DB, agentID string, n int) ([]LogEntry, error) {
	query := `SELECT id, agent_id, direction, stream, content, created_at
		FROM logs WHERE agent_id = ?
		ORDER BY created_at DESC, id DESC LIMIT ?`

	rows, err := db.Query(query, agentID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.AgentID, &entry.Direction, &entry.Stream, &entry.Content, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to get chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -v -run TestListLogsWithTail`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/repo/logs.go internal/repo/logs_test.go
git commit -m "feat(repo): add ListLogsWithTail for otto log --tail"
```

---

## Task 5: Create otto log command

**Files:**
- Create: `internal/cli/commands/log.go`
- Create: `internal/cli/commands/log_test.go`
- Modify: `internal/cli/root.go`

**Step 1: Write the failing test**

```go
// internal/cli/commands/log_test.go
package commands

import (
	"bytes"
	"database/sql"
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestRunLog(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()

	// Create agent
	agent := repo.Agent{ID: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create some log entries
	repo.CreateLogEntry(conn, "test-agent", "out", "stdout", "line 1")
	repo.CreateLogEntry(conn, "test-agent", "out", "stdout", "line 2")
	repo.CreateLogEntry(conn, "test-agent", "out", "stderr", "error 1")

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

	agent := repo.Agent{ID: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create 10 entries
	for i := 0; i < 10; i++ {
		repo.CreateLogEntry(conn, "test-agent", "out", "stdout", fmt.Sprintf("line %d", i))
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v -run TestRunLog`
Expected: FAIL with undefined: runLog

**Step 3: Implement otto log command**

```go
// internal/cli/commands/log.go
package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

var logTail int

func NewLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <agent-id>",
		Short: "Show agent log history",
		Long:  "Show full log history for an agent. Does not advance read cursor.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("id") {
				return errors.New("log is an orchestrator command and does not accept --id flag")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runLog(conn, args[0], logTail, os.Stdout)
		},
	}
	cmd.Flags().IntVar(&logTail, "tail", 0, "Show only last N entries")
	return cmd
}

func runLog(db *sql.DB, agentID string, tail int, w io.Writer) error {
	// Verify agent exists
	if _, err := repo.GetAgent(db, agentID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	var logs []repo.LogEntry
	var err error

	if tail > 0 {
		logs, err = repo.ListLogsWithTail(db, agentID, tail)
	} else {
		logs, err = repo.ListLogs(db, agentID, "")
	}
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No log entries for %s\n", agentID)
		return nil
	}

	for _, entry := range logs {
		stream := ""
		if entry.Stream.Valid {
			stream = fmt.Sprintf("[%s] ", entry.Stream.String)
		}
		fmt.Fprintf(w, "%s%s\n", stream, entry.Content)
	}
	return nil
}
```

**Step 4: Register command in root.go**

Add to `internal/cli/root.go`:
```go
rootCmd.AddCommand(commands.NewLogCmd())
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v -run TestRunLog`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/cli/commands/log.go internal/cli/commands/log_test.go internal/cli/root.go
git commit -m "feat(cli): add otto log command"
```

---

## Task 6: Create otto peek command

**Files:**
- Create: `internal/cli/commands/peek.go`
- Create: `internal/cli/commands/peek_test.go`
- Modify: `internal/cli/root.go`

**Step 1: Write the failing test**

```go
// internal/cli/commands/peek_test.go
package commands

import (
	"bytes"
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestRunPeek(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()

	agent := repo.Agent{ID: "test-agent", Type: "claude", Task: "test", Status: "busy"}
	repo.CreateAgent(conn, agent)

	// Create log entries
	repo.CreateLogEntry(conn, "test-agent", "out", "stdout", "line 1")
	repo.CreateLogEntry(conn, "test-agent", "out", "stdout", "line 2")

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
	repo.CreateLogEntry(conn, "test-agent", "out", "stdout", "line 3")

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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v -run TestRunPeek`
Expected: FAIL with undefined: runPeek

**Step 3: Implement otto peek command**

```go
// internal/cli/commands/peek.go
package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek <agent-id>",
		Short: "Show unread agent logs",
		Long:  "Show unread log entries for an agent and advance read cursor.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("id") {
				return errors.New("peek is an orchestrator command and does not accept --id flag")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runPeek(conn, args[0], os.Stdout)
		},
	}
	return cmd
}

func runPeek(db *sql.DB, agentID string, w io.Writer) error {
	// Get agent to check cursor
	agent, err := repo.GetAgent(db, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	// Get logs since last read
	sinceID := ""
	if agent.LastReadLogID.Valid {
		sinceID = agent.LastReadLogID.String
	}

	logs, err := repo.ListLogs(db, agentID, sinceID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No new log entries for %s\n", agentID)
		return nil
	}

	// Print entries
	for _, entry := range logs {
		stream := ""
		if entry.Stream.Valid {
			stream = fmt.Sprintf("[%s] ", entry.Stream.String)
		}
		fmt.Fprintf(w, "%s%s\n", stream, entry.Content)
	}

	// Advance cursor to last entry
	lastID := logs[len(logs)-1].ID
	return repo.UpdateAgentLastReadLogID(db, agentID, lastID)
}
```

**Step 4: Register command in root.go**

Add to `internal/cli/root.go`:
```go
rootCmd.AddCommand(commands.NewPeekCmd())
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v -run TestRunPeek`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/cli/commands/peek.go internal/cli/commands/peek_test.go internal/cli/root.go
git commit -m "feat(cli): add otto peek command with cursor tracking"
```

---

## Task 7: Add --detach flag to spawn command

**Design Decision:** Detach mode skips output capture entirely. Agent self-reports via `otto complete` / `otto ask`. No stdout/stderr logged for detached agents. Future: add PID reconciliation to mark dead agents as "failed".

**Files:**
- Modify: `internal/cli/commands/spawn.go`
- Modify: `internal/cli/commands/spawn_test.go`

**Step 1: Write the failing test**

```go
// internal/cli/commands/spawn_test.go - add test
func TestSpawnDetach(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()

	runner := &mockRunner{
		startDetachedFunc: func(name string, args ...string) (int, error) {
			return 12345, nil
		},
	}

	var buf bytes.Buffer
	err := runSpawnWithOptions(conn, runner, "claude", "test task", "", "", "", true, &buf)
	if err != nil {
		t.Fatalf("spawn detach failed: %v", err)
	}

	// Should print agent ID
	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Errorf("expected agent ID in output, got: %s", output)
	}

	// Agent should exist and be busy
	agents, _ := repo.ListAgents(conn)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Status != "busy" {
		t.Errorf("expected status busy, got %s", agents[0].Status)
	}
	if !agents[0].Pid.Valid || agents[0].Pid.Int64 != 12345 {
		t.Errorf("expected PID 12345, got %v", agents[0].Pid)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v -run TestSpawnDetach`
Expected: FAIL with undefined: runSpawnWithOptions

**Step 3: Add StartDetached to Runner interface**

In `internal/exec/exec.go`, add to Runner interface:
```go
StartDetached(name string, args ...string) (pid int, err error)
```

Implement in DefaultRunner:
```go
func (r *DefaultRunner) StartDetached(name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil  // No capture
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// Don't wait - process runs independently
	go cmd.Wait() // Reap zombie when done
	return cmd.Process.Pid, nil
}
```

**Step 4: Implement --detach flag in spawn.go**

Add flag variable and registration:
```go
var spawnDetach bool

// In NewSpawnCmd:
cmd.Flags().BoolVar(&spawnDetach, "detach", false, "Return immediately (no output capture)")
```

Refactor to runSpawnWithOptions:
```go
func runSpawnWithOptions(..., detach bool, w io.Writer) error {
	// ... agent creation code (unchanged) ...

	if detach {
		// Start process without waiting or capturing output
		cmdArgs := buildSpawnCommand(agentType, prompt, sessionID)
		pid, err := runner.StartDetached(cmdArgs[0], cmdArgs[1:]...)
		if err != nil {
			return fmt.Errorf("spawn %s: %w", agentType, err)
		}
		_ = repo.UpdateAgentPid(db, agentID, pid)
		fmt.Fprintln(w, agentID)
		return nil
	}

	// ... existing blocking spawn code ...
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v -run TestSpawnDetach`
Expected: PASS

**Step 6: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/exec/exec.go internal/cli/commands/spawn.go internal/cli/commands/spawn_test.go
git commit -m "feat(spawn): add --detach flag for non-blocking spawn"
```

---

## Task 8: Integration test and cleanup

**Files:**
- Run all tests
- Verify build

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests PASS

**Step 2: Build and verify**

Run: `go build -o otto ./cmd/otto`
Expected: Build succeeds

**Step 3: Manual smoke test**

```bash
./otto spawn claude "say hello" --detach
./otto status
./otto peek <agent-id>
./otto log <agent-id> --tail 5
```

**Step 4: Commit any final fixes**

If any issues found, fix and commit.

**Step 5: Final commit**

```bash
git add -A
git commit -m "test: integration tests for spawn-log feature"
```

---

## Pre-Release TODO

- [ ] Remove all schema migrations - consolidate into clean schema before public release

## Future Improvements

- [ ] PID reconciliation: on `db.Open()`, check "busy" agents with PIDs - if dead, mark as "failed"

---

## Summary

| Task | Description | Estimated Complexity |
|------|-------------|---------------------|
| 1 | Schema: rename transcript_entries → logs | Low |
| 2 | Schema: add last_read_log_id to agents | Low |
| 3 | Repo: rename TranscriptEntry → LogEntry | Medium |
| 4 | Repo: add ListLogsWithTail | Low |
| 5 | CLI: add otto log command | Medium |
| 6 | CLI: add otto peek command | Medium |
| 7 | CLI: add --detach to spawn | High |
| 8 | Integration test and cleanup | Low |

Total: 8 tasks, ~45 bite-sized steps

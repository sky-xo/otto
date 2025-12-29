# Agent Messaging Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Simplify agent messaging by adding `otto dm`, enhancing `otto peek` for completed agents, and removing `otto say`.

**Architecture:** Add direct messaging between agents via `otto dm --from --to`. Modify `peek` to show full log (capped at 100 lines) when agent status is complete/failed. Remove `say` command entirely.

**Tech Stack:** Go, Cobra CLI, SQLite

---

### Task 1: Add `otto dm` command

**Files:**
- Create: `internal/cli/commands/dm.go`
- Create: `internal/cli/commands/dm_test.go`
- Modify: `internal/cli/root.go`

**Step 1: Write failing test for dm command**

```go
// internal/cli/commands/dm_test.go
package commands

import (
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestRunDM(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create sender and recipient agents
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "impl-1", Type: "codex", Task: "test", Status: "busy"})
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "reviewer", Type: "codex", Task: "test", Status: "busy"})

	err := runDM(conn, ctx.Project, ctx.Branch, "impl-1", "reviewer", "API contract is ready")
	if err != nil {
		t.Fatalf("runDM failed: %v", err)
	}

	// Verify message was created
	msgs, err := repo.ListMessages(conn, repo.MessageFilter{Project: ctx.Project, Branch: ctx.Branch, Type: "dm"})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].FromAgent != "impl-1" {
		t.Errorf("expected from_agent 'impl-1', got %q", msgs[0].FromAgent)
	}
	if !msgs[0].ToAgent.Valid || msgs[0].ToAgent.String != "reviewer" {
		t.Errorf("expected to_agent 'reviewer', got %v", msgs[0].ToAgent)
	}
	if msgs[0].Content != "API contract is ready" {
		t.Errorf("expected content 'API contract is ready', got %q", msgs[0].Content)
	}
}

func TestRunDM_CrossBranch(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create sender in current branch
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "impl-1", Type: "codex", Task: "test", Status: "busy"})
	// Create recipient in different branch
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: "feature/login", Name: "frontend", Type: "codex", Task: "test", Status: "busy"})

	err := runDM(conn, ctx.Project, ctx.Branch, "impl-1", "feature/login:frontend", "your branch needs rebase")
	if err != nil {
		t.Fatalf("runDM failed: %v", err)
	}

	// Verify message was created with resolved address
	msgs, err := repo.ListMessages(conn, repo.MessageFilter{Project: ctx.Project, Branch: ctx.Branch, Type: "dm"})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	// to_agent should be the full resolved address
	if !msgs[0].ToAgent.Valid || msgs[0].ToAgent.String != "feature/login:frontend" {
		t.Errorf("expected to_agent 'feature/login:frontend', got %v", msgs[0].ToAgent)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestRunDM -v`
Expected: FAIL (runDM not defined)

**Step 3: Implement dm command**

```go
// internal/cli/commands/dm.go
package commands

import (
	"database/sql"
	"fmt"

	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var dmFrom string
var dmTo string

func NewDMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dm [message]",
		Short: "Send a direct message to another agent",
		Long:  "Send a direct message from one agent to another. Wakes the target agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dmFrom == "" {
				return fmt.Errorf("--from flag is required")
			}
			if dmTo == "" {
				return fmt.Errorf("--to flag is required")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx := scope.CurrentContext()
			return runDM(conn, ctx.Project, ctx.Branch, dmFrom, dmTo, args[0])
		},
	}
	cmd.Flags().StringVar(&dmFrom, "from", "", "Sender agent name (required)")
	cmd.Flags().StringVar(&dmTo, "to", "", "Recipient agent (supports branch:agent format)")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}

func runDM(db *sql.DB, project, branch, from, to, content string) error {
	msg := repo.Message{
		ID:           uuid.New().String(),
		Project:      project,
		Branch:       branch,
		FromAgent:    from,
		ToAgent:      sql.NullString{String: to, Valid: true},
		Type:         "dm",
		Content:      content,
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}

	return repo.CreateMessage(db, msg)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/commands -run TestRunDM -v`
Expected: PASS

**Step 5: Wire up command in root.go**

Add to `internal/cli/root.go`:
```go
rootCmd.AddCommand(commands.NewDMCmd())
```

**Step 6: Run all tests**

Run: `go test ./internal/cli/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/cli/commands/dm.go internal/cli/commands/dm_test.go internal/cli/root.go
git commit -m "feat(otto): add dm command for agent-to-agent messaging"
```

---

### Task 2: Add CountLogs repo helper

**Files:**
- Modify: `internal/repo/logs.go`
- Modify: `internal/repo/logs_test.go` (create if doesn't exist)

**Step 1: Write failing test for CountLogs**

```go
// internal/repo/logs_test.go
package repo

import (
	"database/sql"
	"testing"

	"otto/internal/db"
)

func TestCountLogs(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()

	project := "test-project"
	branch := "main"
	agentName := "test-agent"

	// Create some log entries
	for i := 0; i < 5; i++ {
		CreateLogEntry(conn, LogEntry{
			Project:   project,
			Branch:    branch,
			AgentName: agentName,
			AgentType: "codex",
			EventType: "output",
			Content:   sql.NullString{String: "line", Valid: true},
		})
	}

	count, err := CountLogs(conn, project, branch, agentName)
	if err != nil {
		t.Fatalf("CountLogs failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -run TestCountLogs -v`
Expected: FAIL (CountLogs not defined)

**Step 3: Implement CountLogs**

Add to `internal/repo/logs.go`:
```go
func CountLogs(db *sql.DB, project, branch, agentName string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM logs WHERE project = ? AND branch = ? AND agent_name = ?`,
		project, branch, agentName).Scan(&count)
	return count, err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -run TestCountLogs -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/repo/logs.go internal/repo/logs_test.go
git commit -m "feat(repo): add CountLogs helper"
```

---

### Task 3: Enhance `otto peek` for completed agents

**Files:**
- Modify: `internal/cli/commands/peek.go`
- Modify: `internal/cli/commands/peek_test.go`

**Step 1: Write failing test for peek on completed agent**

Add to `internal/cli/commands/peek_test.go`:
```go
func TestRunPeek_CompletedAgentShowsFullLog(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create COMPLETED agent
	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "complete"}
	repo.CreateAgent(conn, agent)

	// Create log entries
	for i := 1; i <= 5; i++ {
		repo.CreateLogEntry(conn, repo.LogEntry{
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			AgentName: "test-agent",
			AgentType: "codex",
			EventType: "output",
			Content:   sql.NullString{String: fmt.Sprintf("line %d", i), Valid: true},
		})
	}

	// Advance cursor past first 3 entries (simulating earlier peeks)
	logs, _ := repo.ListLogs(conn, ctx.Project, ctx.Branch, "test-agent", "")
	repo.UpdateAgentPeekCursor(conn, ctx.Project, ctx.Branch, "test-agent", logs[2].ID)

	var buf bytes.Buffer

	// Peek on completed agent should show FULL log, not just since cursor
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	// Should contain header
	if !strings.Contains(output, "[agent complete") {
		t.Errorf("expected '[agent complete' header, got: %s", output)
	}
	// Should contain ALL lines, not just since cursor
	if !strings.Contains(output, "line 1") {
		t.Errorf("expected 'line 1' (full log), got: %s", output)
	}
	if !strings.Contains(output, "line 5") {
		t.Errorf("expected 'line 5', got: %s", output)
	}
}

func TestRunPeek_CompletedAgentCapsAt100(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "complete"}
	repo.CreateAgent(conn, agent)

	// Create 150 log entries
	for i := 1; i <= 150; i++ {
		repo.CreateLogEntry(conn, repo.LogEntry{
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			AgentName: "test-agent",
			AgentType: "codex",
			EventType: "output",
			Content:   sql.NullString{String: fmt.Sprintf("line %d", i), Valid: true},
		})
	}

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	// Should show capped message
	if !strings.Contains(output, "showing last 100 lines") {
		t.Errorf("expected 'showing last 100 lines', got: %s", output)
	}
	// Should show footer with full count
	if !strings.Contains(output, "full log: 150 lines") {
		t.Errorf("expected 'full log: 150 lines', got: %s", output)
	}
	// Should NOT contain early lines (line 1-50)
	if strings.Contains(output, "line 1\n") {
		t.Errorf("should not contain line 1 (capped), got: %s", output)
	}
	// Should contain later lines (line 100+)
	if !strings.Contains(output, "line 150") {
		t.Errorf("expected 'line 150', got: %s", output)
	}
}

func TestRunPeek_FailedAgentShowsFullLog(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "test-agent", Type: "codex", Task: "test", Status: "failed"}
	repo.CreateAgent(conn, agent)

	repo.CreateLogEntry(conn, repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: "test-agent",
		AgentType: "codex",
		EventType: "output",
		Content:   sql.NullString{String: "error happened", Valid: true},
	})

	var buf bytes.Buffer
	err := runPeek(conn, "test-agent", &buf)
	if err != nil {
		t.Fatalf("runPeek failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[agent failed") {
		t.Errorf("expected '[agent failed' header, got: %s", output)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/commands -run "TestRunPeek_Completed|TestRunPeek_Failed" -v`
Expected: FAIL

**Step 3: Modify peek.go to handle completed/failed agents**

Replace `internal/cli/commands/peek.go`:
```go
package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/spf13/cobra"
)

const peekCapLines = 100

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek <agent-id>",
		Short: "Show unread agent logs",
		Long:  "Show unread log entries for an agent and advance read cursor. For completed/failed agents, shows full log (capped at 100 lines).",
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
	ctx := scope.CurrentContext()

	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	// For completed/failed agents, show full log (capped)
	if agent.Status == "complete" || agent.Status == "failed" {
		return runPeekFullLog(db, ctx, agent, w)
	}

	// For active agents, show incremental logs
	return runPeekIncremental(db, ctx, agent, w)
}

func runPeekFullLog(db *sql.DB, ctx scope.Context, agent repo.Agent, w io.Writer) error {
	// Count total logs
	totalCount, err := repo.CountLogs(db, ctx.Project, ctx.Branch, agent.Name)
	if err != nil {
		return err
	}

	if totalCount == 0 {
		fmt.Fprintf(w, "No log entries for %s\n", agent.Name)
		return nil
	}

	// Get logs (capped)
	var logs []repo.LogEntry
	if totalCount > peekCapLines {
		logs, err = repo.ListLogsWithTail(db, ctx.Project, ctx.Branch, agent.Name, peekCapLines)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "[agent %s - showing last %d lines]\n\n", agent.Status, peekCapLines)
	} else {
		logs, err = repo.ListLogs(db, ctx.Project, ctx.Branch, agent.Name, "")
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "[agent %s - showing all %d lines]\n\n", agent.Status, totalCount)
	}

	// Render logs
	for _, entry := range logs {
		renderLogEntry(w, entry)
	}

	// Show footer if capped
	if totalCount > peekCapLines {
		fmt.Fprintf(w, "\n[full log: %d lines - run 'otto log %s' for complete history]\n", totalCount, agent.Name)
	}

	return nil
}

func runPeekIncremental(db *sql.DB, ctx scope.Context, agent repo.Agent, w io.Writer) error {
	// Get logs since the last peek cursor
	sinceID := ""
	if agent.PeekCursor.Valid {
		sinceID = agent.PeekCursor.String
	}
	logs, err := repo.ListLogs(db, ctx.Project, ctx.Branch, agent.Name, sinceID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No new log entries for %s\n", agent.Name)
		return nil
	}

	for _, entry := range logs {
		renderLogEntry(w, entry)
	}

	// Update the peek cursor to the last log entry ID
	lastLogID := logs[len(logs)-1].ID
	if err := repo.UpdateAgentPeekCursor(db, ctx.Project, ctx.Branch, agent.Name, lastLogID); err != nil {
		return err
	}

	return nil
}

func renderLogEntry(w io.Writer, entry repo.LogEntry) {
	// Handle item.started events
	if entry.EventType == "item.started" {
		if entry.Command.Valid && entry.Command.String != "" {
			fmt.Fprintf(w, "[running] %s\n", entry.Command.String)
		} else if entry.Content.Valid && entry.Content.String != "" {
			fmt.Fprintf(w, "[starting] %s\n", entry.Content.String)
		}
		return
	}

	// Handle turn events
	if entry.EventType == "turn.started" {
		fmt.Fprintf(w, "--- turn started ---\n")
		return
	}
	if entry.EventType == "turn.completed" {
		fmt.Fprintf(w, "--- turn completed ---\n")
		return
	}

	// Handle all other events
	stream := ""
	if entry.ToolName.Valid {
		stream = fmt.Sprintf("[%s] ", entry.ToolName.String)
	}
	if entry.Content.Valid {
		fmt.Fprintf(w, "%s%s\n", stream, entry.Content.String)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/commands -run TestRunPeek -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/commands/peek.go internal/cli/commands/peek_test.go
git commit -m "feat(peek): show full log for completed/failed agents (capped at 100)"
```

---

### Task 4: Remove `otto say` command

**Files:**
- Delete: `internal/cli/commands/say.go`
- Modify: `internal/cli/root.go`

**Step 1: Remove say command from root.go**

Remove this line from `internal/cli/root.go`:
```go
rootCmd.AddCommand(commands.NewSayCmd())
```

**Step 2: Delete say.go**

Run: `rm internal/cli/commands/say.go`

**Step 3: Run all tests to ensure nothing breaks**

Run: `go test ./internal/cli/... -v`
Expected: PASS

**Step 4: Build to verify compilation**

Run: `make build`
Expected: Success

**Step 5: Commit**

```bash
git add -A
git commit -m "feat(otto): remove say command (use dm instead)"
```

---

### Task 5: Update skill documentation

**Files:**
- Modify: `.claude/skills/otto-orchestrate/SKILL.md`
- Modify: `.codex/skills/otto-orchestrate/SKILL.md`

**Step 1: Update .claude/skills/otto-orchestrate/SKILL.md**

Key changes:
1. Remove references to `block: true` (doesn't exist)
2. Remove `otto say` documentation
3. Add `otto dm` documentation
4. Update workflow examples
5. Document enhanced `peek` behavior for completed agents

**Step 2: Update .codex/skills/otto-orchestrate/SKILL.md**

Same changes as above.

**Step 3: Commit**

```bash
git add .claude/skills/otto-orchestrate/SKILL.md .codex/skills/otto-orchestrate/SKILL.md
git commit -m "docs: update skill docs with dm command and remove say"
```

---

### Task 6: Update TODO.md

**Files:**
- Modify: `TODO.md`

**Step 1: Add note about name collision bug**

Ensure the bug is documented:
```markdown
## Bugs

- Agent names conflict across different projects. The unique constraint on the db should be tied to project/branch-name/agent-name not just to agent-name
```

**Step 2: Mark feedback items as addressed**

Update the feedback section to note which items are resolved.

**Step 3: Commit**

```bash
git add TODO.md
git commit -m "docs: update TODO with messaging redesign status"
```

---

Plan complete and saved to `docs/plans/2025-12-29-agent-messaging-impl-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?

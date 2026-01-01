# Super Orchestrator V0 Phase 2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the V0 TUI experience: activity feed + orchestrator chat + parsed agent transcripts using structured Codex events.

**Architecture:** Reuse the normalized logs from Phase 1 (event_type + command/content/raw_json). Add a parser/formatter layer that converts log events into renderable blocks, then update the Bubble Tea TUI to render a two-pane main view and a per-agent transcript view with formatted entries (reasoning shown as-is, command output in code blocks).

**Tech Stack:** Go, Bubble Tea, Lip Gloss.

### Task 1: Add log event formatting helpers

**Files:**
- Create: `internal/tui/formatting.go`
- Test: `internal/tui/formatting_test.go`

**Step 1: Write the failing test**

```go
func TestFormatLogEntryReasoning(t *testing.T) {
	entry := repo.LogEntry{EventType: "reasoning", Content: "I will do X"}
	out := FormatLogEntry(entry)
	if out != "[reasoning] I will do X" {
		t.Fatalf("unexpected output: %q", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -v`
Expected: FAIL with "undefined: FormatLogEntry"

**Step 3: Write minimal implementation**

```go
// internal/tui/formatting.go
package tui

// Reasoning labels are added at render time based on EventType; DB stores only event_type + content.
// Render reasoning lines with a dim style in the TUI.
func FormatLogEntry(entry repo.LogEntry) string {
	switch entry.EventType {
	case "reasoning":
		return "[reasoning] " + entry.Content
	case "command_execution":
		if entry.Command == "" {
			return entry.Content
		}
		if entry.Content == "" {
			return entry.Command
		}
		return entry.Command + "\n" + entry.Content
	case "agent_message":
		return entry.Content
	default:
		return entry.Content
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/formatting.go internal/tui/formatting_test.go
git commit -m "feat: add log entry formatting helpers"
```

### Task 2: Render activity feed + orchestrator chat in main view

**Files:**
- Modify: `internal/tui/watch.go`
- Test: `internal/tui/watch_test.go`

**Step 1: Write the failing test**

```go
func TestRenderMainViewIncludesActivityFeed(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	m := NewModel(db)
	m.messages = []repo.Message{{Type: "complete", Content: "@impl-1 done"}}
	view := m.View()
	if !strings.Contains(view, "Activity Feed") {
		t.Fatalf("missing activity feed header")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -v`
Expected: FAIL with "missing activity feed header"

**Step 3: Write minimal implementation**

Update the layout to render a split right panel (top = activity feed, bjunem = orchestrator chat):

```go
func (m model) renderRightPanel() string {
	top := renderActivityFeed(m.messages)
	bjunem := renderChat(m.messages)
	return lipgloss.JoinVertical(lipgloss.Left, top, bjunem)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/watch.go internal/tui/watch_test.go
git commit -m "feat: split activity feed and chat in tui"
```

### Task 3: Agent transcript view with structured formatting

**Files:**
- Modify: `internal/tui/watch.go`
- Test: `internal/tui/watch_test.go`

**Step 1: Write the failing test**

```go
func TestTranscriptViewFormatsReasoning(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	m := NewModel(db)
	m.transcripts["impl-1"] = []repo.LogEntry{{EventType: "reasoning", Content: "thinking"}}
	m.activeChannelID = "impl-1"
	view := m.View()
	if !strings.Contains(view, "thinking") {
		t.Fatalf("expected reasoning formatting")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -v`
Expected: FAIL with "expected reasoning formatting"

**Step 3: Write minimal implementation**

```go
func renderTranscript(entries []repo.LogEntry) string {
	var lines []string
	for _, entry := range entries {
		lines = append(lines, FormatLogEntry(entry))
	}
	return strings.Join(lines, "\n")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/watch.go internal/tui/watch_test.go
git commit -m "feat: format agent transcript entries"
```

### Task 4: Load logs with event types for transcript view

**Files:**
- Modify: `internal/repo/logs.go`
- Test: `internal/repo/logs_test.go`

**Step 1: Write the failing test**

```go
func TestListLogsIncludesType(t *testing.T) {
	db := openTestDB(t)
	_ = repo.CreateLogEntry(db, repo.LogEntry{Project: "app", Branch: "main", AgentName: "impl-1", AgentType: "codex", EventType: "reasoning", Content: "thinking"})
	entries, _ := repo.ListLogs(db, "app", "main", "impl-1", "")
	if entries[0].EventType != "reasoning" {
		t.Fatalf("event_type = %q", entries[0].EventType)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -v`
Expected: FAIL with "unknown field Type" or mismatch

**Step 3: Write minimal implementation**

```go
// internal/repo/logs.go

type LogEntry struct {
	ID        string
	Project   string
	Branch    string
	AgentName string
	AgentType string
	EventType string
	ToolName  sql.NullString
	Content   string
	RawJSON   sql.NullString
	Command   sql.NullString
	ExitCode  sql.NullInt64
	Status    sql.NullString
	ToolUseID sql.NullString
	CreatedAt string
}

func CreateLogEntry(db *sql.DB, entry LogEntry) error {
	_, err := db.Exec(`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, tool_name, content, raw_json, command, exit_code, status, tool_use_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), entry.Project, entry.Branch, entry.AgentName, entry.AgentType, entry.EventType, entry.ToolName, entry.Content, entry.RawJSON, entry.Command, entry.ExitCode, entry.Status, entry.ToolUseID)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/repo/logs.go internal/repo/logs_test.go
git commit -m "feat: include log event types in repo"
```

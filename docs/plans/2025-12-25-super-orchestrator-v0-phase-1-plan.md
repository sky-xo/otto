# Super Orchestrator V0 Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the V0 backend orchestration core: global DB with project/branch-aware agents/messages/tasks, Codex event parsing with compaction detection, and daemon wake-ups.

**Architecture:** Migrate SQLite schema to add project/branch columns, tasks table, and normalized log events. Update repo layer to require scope, parse @mentions into fully-qualified addresses, and extend transcript capture to parse Codex JSON events (including context_compacted). The watch loop becomes the daemon: it detects mentions/completions/failures and wakes the orchestrator with a context bundle built from tasks + new messages.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Cobra CLI.

### Task 1: Introduce scope context + global DB path

**Files:**
- Modify: `internal/cli/commands/say.go`
- Modify: `internal/scope/scope.go`
- Create: `internal/scope/context.go`
- Test: `internal/scope/scope_test.go`

**Step 1: Write the failing test**

```go
func TestCurrentContextFromRepoRoot(t *testing.T) {
	ctx := scope.ContextFromRepoRoot("/Users/alice/code/my-app", "feature-login")
	if ctx.Project != "my-app" {
		t.Fatalf("project = %q", ctx.Project)
	}
	if ctx.Branch != "feature-login" {
		t.Fatalf("branch = %q", ctx.Branch)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/scope -v`
Expected: FAIL with "undefined: scope.ContextFromRepoRoot"

**Step 3: Write minimal implementation**

```go
// internal/scope/context.go
package scope

import "path/filepath"

type Context struct {
	Project string
	Branch  string
}

func ContextFromRepoRoot(repoRoot, branch string) Context {
	project := filepath.Base(repoRoot)
	if branch == "" {
		branch = "main"
	}
	return Context{Project: project, Branch: branch}
}

func CurrentContext() Context {
	repoRoot := RepoRoot()
	branch := BranchName()
	if repoRoot == "" {
		return Context{Project: "unknown", Branch: "main"}
	}
	return ContextFromRepoRoot(repoRoot, branch)
}
```

Update `internal/cli/commands/say.go` to use a single global DB path:

```go
func openDB() (*sql.DB, error) {
	dbPath := filepath.Join(config.DataDir(), "june.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	return db.Open(dbPath)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/scope -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/scope/context.go internal/scope/scope_test.go internal/cli/commands/say.go
git commit -m "feat: add scope context helper and global db path"
```

### Task 2: Migrate schema to project/branch-aware tables + tasks

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`
- Create: `internal/repo/tasks.go`
- Test: `internal/repo/tasks_test.go`

**Step 1: Write the failing test**

```go
func TestTasksTableExists(t *testing.T) {
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	if !columnExists(t, conn, "tasks", "project") {
		t.Fatalf("tasks.project column missing")
	}
	if !columnExists(t, conn, "logs", "event_type") {
		t.Fatalf("logs.event_type column missing")
	}
	if !columnExists(t, conn, "logs", "agent_type") {
		t.Fatalf("logs.agent_type column missing")
	}
	if !columnExists(t, conn, "agents", "project") {
		t.Fatalf("agents.project column missing")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db -v`
Expected: FAIL with "tasks table missing" or "column missing"

**Step 3: Write minimal implementation**

Update `internal/db/db.go` to:

```sql
CREATE TABLE IF NOT EXISTS agents (
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  task TEXT NOT NULL,
  status TEXT NOT NULL,
  session_id TEXT,
  pid INTEGER,
  compacted_at DATETIME,
  last_seen_message_id TEXT,
  completed_at DATETIME,
  archived_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (project, branch, name)
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  from_agent TEXT NOT NULL,
  to_agent TEXT,
  type TEXT NOT NULL,
  content TEXT NOT NULL,
  mentions TEXT,
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS logs (
  id TEXT PRIMARY KEY,
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  agent_name TEXT NOT NULL,
  agent_type TEXT NOT NULL,
  event_type TEXT NOT NULL,
  tool_name TEXT,
  content TEXT,
  raw_json TEXT,
  command TEXT,
  exit_code INTEGER,
  status TEXT,
  tool_use_id TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  id TEXT NOT NULL,
  parent_id TEXT,
  name TEXT NOT NULL,
  sort_index INTEGER NOT NULL DEFAULT 0,
  assigned_agent TEXT,
  result TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (project, branch, id)
);
```

Add the initial tasks repo helpers in `internal/repo/tasks.go`:

```go
package repo

type Task struct {
	Project       string
	Branch        string
	ID            string
	ParentID      sql.NullString
	Name          string
	SortIndex     int
	AssignedAgent sql.NullString
	Result        sql.NullString
}

func CreateTask(db *sql.DB, t Task) error {
	_, err := db.Exec(`INSERT INTO tasks (project, branch, id, parent_id, name, sort_index, assigned_agent, result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Project, t.Branch, t.ID, t.ParentID, t.Name, t.SortIndex, t.AssignedAgent, t.Result)
	return err
}

func ListTasks(db *sql.DB, project, branch string) ([]Task, error) {
	rows, err := db.Query(`SELECT project, branch, id, parent_id, name, sort_index, assigned_agent, result
		FROM tasks WHERE project = ? AND branch = ? ORDER BY sort_index ASC`, project, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.Project, &t.Branch, &t.ID, &t.ParentID, &t.Name, &t.SortIndex, &t.AssignedAgent, &t.Result); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

Add a migration path in `ensureSchema`:

```go
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN project TEXT`)
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN branch TEXT`)
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN name TEXT`)
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN compacted_at DATETIME`)
_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN last_seen_message_id TEXT`)
_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN project TEXT`)
_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN branch TEXT`)
_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN from_agent TEXT`)
_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN to_agent TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN project TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN branch TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN agent_name TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN agent_type TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN event_type TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN tool_name TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN raw_json TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN command TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN exit_code INTEGER`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN status TEXT`)
_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN tool_use_id TEXT`)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/repo/tasks.go internal/repo/tasks_test.go
git commit -m "feat: add project-aware schema and tasks table"
```

### Task 3: Update repo layer to be project/branch-aware

**Files:**
- Modify: `internal/repo/agents.go`
- Modify: `internal/repo/messages.go`
- Modify: `internal/repo/logs.go`
- Test: `internal/repo/agents_test.go`
- Test: `internal/repo/messages_test.go`
- Test: `internal/repo/logs_test.go`

**Step 1: Write the failing test**

```go
func TestListAgentsFilteredByProjectBranch(t *testing.T) {
	db := openTestDB(t)
	_ = repo.CreateAgent(db, repo.Agent{Project: "alpha", Branch: "main", Name: "a1", Type: "codex", Task: "t", Status: "busy"})
	_ = repo.CreateAgent(db, repo.Agent{Project: "beta", Branch: "main", Name: "a1", Type: "codex", Task: "t", Status: "busy"})

	agents, err := repo.ListAgents(db, repo.AgentFilter{Project: "alpha", Branch: "main"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("agents = %d", len(agents))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -v`
Expected: FAIL with "unknown field Project" or similar

**Step 3: Write minimal implementation**

Update structs and queries to include scope fields:

```go
// internal/repo/agents.go

type Agent struct {
	Project       string
	Branch        string
	Name          string
	Type          string
	Task          string
	Status        string
	SessionID     sql.NullString
	Pid           sql.NullInt64
	CompactedAt   sql.NullTime
	LastSeenMsgID sql.NullString
	CompletedAt   sql.NullTime
	ArchivedAt    sql.NullTime
}

func CreateAgent(db *sql.DB, a Agent) error {
	_, err := db.Exec(`INSERT INTO agents (project, branch, name, type, task, status, session_id, pid, compacted_at, last_seen_message_id, completed_at, archived_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Project, a.Branch, a.Name, a.Type, a.Task, a.Status, a.SessionID, a.Pid, a.CompactedAt, a.LastSeenMsgID, a.CompletedAt, a.ArchivedAt)
	return err
}
```

Mirror the same shape for messages/logs:

```go
// internal/repo/messages.go

type Message struct {
	ID            string
	Project       string
	Branch        string
	FromAgent     string
	ToAgent       sql.NullString
	Type          string
	Content       string
	MentionsJSON  string
	RequiresHuman bool
	ReadByJSON    string
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/repo/agents.go internal/repo/messages.go internal/repo/logs.go internal/repo/agents_test.go internal/repo/messages_test.go internal/repo/logs_test.go
git commit -m "feat: scope repo queries by project and branch"
```

### Task 4: Parse @mentions into fully-qualified addresses

Agent names are normalized to lowercase; project and branch casing is preserved.

**Files:**
- Modify: `internal/cli/commands/say.go`
- Test: `internal/cli/commands/commands_test.go`

**Step 1: Write the failing test**

```go
func TestParseMentionsWithScope(t *testing.T) {
	ctx := scope.Context{Project: "app", Branch: "feature/login"}
	mentions := parseMentions("ping @Impl-1 and @backend:main:June", ctx)
	want := []string{"app:feature/login:impl-1", "backend:main:june"}
	if !reflect.DeepEqual(mentions, want) {
		t.Fatalf("mentions = %#v", mentions)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v`
Expected: FAIL with "parseMentions undefined" or mismatch

**Step 3: Write minimal implementation**

```go
var mentionRe = regexp.MustCompile(`@([A-Za-z0-9._/-]+(?::[A-Za-z0-9._/-]+){0,2})`)

func parseMentions(content string, ctx scope.Context) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var out []string
	for _, match := range matches {
		parts := strings.Split(match[1], ":")
		resolved := ""
		switch len(parts) {
		case 1:
			resolved = fmt.Sprintf("%s:%s:%s", ctx.Project, ctx.Branch, strings.ToLower(parts[0]))
		case 2:
			resolved = fmt.Sprintf("%s:%s:%s", ctx.Project, parts[0], strings.ToLower(parts[1]))
		case 3:
			resolved = fmt.Sprintf("%s:%s:%s", parts[0], parts[1], strings.ToLower(parts[2]))
		}
		if resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	return out
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/commands/say.go internal/cli/commands/commands_test.go
git commit -m "feat: resolve mentions to project/branch scope"
```

### Task 5: Codex JSON event parsing + compaction handling

**Files:**
- Modify: `internal/cli/commands/transcript_capture.go`
- Modify: `internal/cli/commands/spawn.go`
- Create: `internal/cli/commands/codex_events.go`
- Test: `internal/cli/commands/codex_events_test.go`

**Step 1: Write the failing test**

```go
func TestParseCodexEventCompaction(t *testing.T) {
	line := `{"type":"context_compacted"}`
	event := ParseCodexEvent(line)
	if event.Type != "context_compacted" {
		t.Fatalf("type = %q", event.Type)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v`
Expected: FAIL with "undefined: ParseCodexEvent"

**Step 3: Write minimal implementation**

```go
// internal/cli/commands/codex_events.go
package commands

import "encoding/json"

type CodexItem struct {
	Type             string `json:"type"`
	Text             string `json:"text,omitempty"`
	Command          string `json:"command,omitempty"`
	AggregatedOutput string `json:"aggregated_output,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	Status           string `json:"status,omitempty"`
}

type CodexEvent struct {
	Type     string    `json:"type"`
	ThreadID string    `json:"thread_id"`
	Item     *CodexItem `json:"item,omitempty"`
	Raw      string
}

func ParseCodexEvent(line string) CodexEvent {
	var payload CodexEvent
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return CodexEvent{}
	}
	payload.Raw = line
	return payload
}
```

Wire it into transcript capture:

```go
func consumeTranscriptEntries(db *sql.DB, agentName string, output <-chan juneexec.TranscriptChunk, onEvent func(CodexEvent)) <-chan error {
	...
	if onEvent != nil && chunk.Stream == "stdout" {
		line := strings.TrimSpace(stdoutBuffer[:newline])
		event := ParseCodexEvent(line)
		if event.Type != "" {
			onEvent(event)
		}
	}
	...
}
```

And in `runCodexSpawn`:

```go
onEvent := func(event CodexEvent) {
	if event.Type == "thread.started" && event.ThreadID != "" {
		_ = repo.UpdateAgentSessionID(db, ctx, agentName, event.ThreadID)
	}
	if event.Type == "context_compacted" {
		_ = repo.MarkAgentCompacted(db, ctx, agentName)
	}
	if event.Type == "turn.failed" {
		_ = repo.SetAgentFailed(db, ctx, agentName)
	}
	if event.Type == "item.completed" && event.Item != nil {
		logEntry := repo.LogEntry{
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			AgentName: agentName,
			AgentType: "codex",
			EventType: event.Item.Type,
			ToolName:  "",
			Content:   event.Item.Text,
			RawJSON:   event.Raw,
			Command:   event.Item.Command,
			ExitCode:  event.Item.ExitCode,
			Status:    event.Item.Status,
		}
		if event.Item.Type == "command_execution" {
			logEntry.Content = event.Item.AggregatedOutput
		}
		_ = repo.CreateLogEntry(db, logEntry)
	}
}
```

For non-item events you still care about later, write a log row with `event_type = event.Type` and `raw_json = event.Raw`, leaving content fields empty.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/commands/codex_events.go internal/cli/commands/codex_events_test.go internal/cli/commands/transcript_capture.go internal/cli/commands/spawn.go
git commit -m "feat: parse codex events and track compaction"
```

### Task 6: Daemon wake-ups and failure detection

Wake-ups are triggered on any @mention or any agent process exit; V0 does not rely on `june complete`.

**Files:**
- Modify: `internal/cli/commands/watch.go`
- Modify: `internal/cli/commands/watch_test.go`
- Create: `internal/cli/commands/wakeup.go`

**Step 1: Write the failing test**

```go
func TestWakeupOnMention(t *testing.T) {
	db := setupTestDB(t)
	ctx := scope.Context{Project: "app", Branch: "main"}
	_ = repo.CreateMessage(db, repo.Message{
		ID: "m1", Project: ctx.Project, Branch: ctx.Branch,
		FromAgent: "impl-1", Type: "say", Content: "@impl-1 status?",
		MentionsJSON: `["app:main:impl-1"]`, ReadByJSON: "[]",
	})

	w := newWakeupTracker()
	err := processWakeups(db, ctx, w)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if !w.Woke("app:main:impl-1") {
		t.Fatalf("expected wakeup for impl-1")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -v`
Expected: FAIL with "undefined: processWakeups"

**Step 3: Write minimal implementation**

```go
// internal/cli/commands/wakeup.go
package commands

func processWakeups(db *sql.DB, ctx scope.Context, w wakeupSender) error {
	msgs, err := repo.ListMessages(db, repo.MessageFilter{Project: ctx.Project, Branch: ctx.Branch})
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}

	contextText, err := buildContextBundle(db, ctx, msgs)
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		for _, mention := range parseMentionsFromJSON(msg.MentionsJSON) {
			if err := w.SendTo(mention, contextText); err != nil {
				return err
			}
		}
	}
	return nil
}
```

Update `cleanupStaleAgents` to mark failed and notify orchestrator:

```go
if !process.IsProcessAlive(int(a.Pid.Int64)) {
	_ = repo.SetAgentFailed(db, ctx, a.Name)
	_ = repo.CreateMessage(db, repo.Message{
		ID: uuid.NewString(), Project: ctx.Project, Branch: ctx.Branch,
		FromAgent: a.Name, Type: "exit",
		Content: "process died unexpectedly",
		MentionsJSON: "[]",
		ReadByJSON: "[]",
	})
	_ = w.SendTo(fmt.Sprintf("%s:%s:june", ctx.Project, ctx.Branch), buildExitContext(...))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/commands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/commands/watch.go internal/cli/commands/watch_test.go internal/cli/commands/wakeup.go
git commit -m "feat: daemon wakeups and failure notifications"
```

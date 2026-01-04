# TUI Codex Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Codex agents appear in the TUI sidebar alongside Claude agents, grouped by channel.

**Architecture:** Create shared `internal/agent/` package with unified Agent type. Both claude and db packages convert to this type. Channel scanning merges both sources.

**Tech Stack:** Go, SQLite, Bubbletea TUI

**Design Doc:** `docs/plans/2026-01-03-tui-codex-integration-design.md`

---

## Phase 1: Create Unified Agent Package

### Task 1: Create agent package with Agent type

**Files:**
- Create: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

**Step 1: Write the failing test**

```go
// internal/agent/agent_test.go
package agent

import (
	"testing"
	"time"
)

func TestAgent_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		agent    Agent
		expected string
	}{
		{
			name:     "uses Name when set",
			agent:    Agent{ID: "abc", Name: "fix-auth"},
			expected: "fix-auth",
		},
		{
			name:     "falls back to ID when Name empty",
			agent:    Agent{ID: "abc123"},
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.agent.DisplayName(); got != tt.expected {
				t.Errorf("DisplayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAgent_IsActive(t *testing.T) {
	active := Agent{LastActivity: time.Now().Add(-5 * time.Second)}
	if !active.IsActive() {
		t.Error("agent modified 5s ago should be active")
	}

	inactive := Agent{LastActivity: time.Now().Add(-30 * time.Second)}
	if inactive.IsActive() {
		t.Error("agent modified 30s ago should not be active")
	}
}

func TestAgent_IsRecent(t *testing.T) {
	recent := Agent{LastActivity: time.Now().Add(-1 * time.Hour)}
	if !recent.IsRecent() {
		t.Error("agent modified 1h ago should be recent")
	}

	old := Agent{LastActivity: time.Now().Add(-3 * time.Hour)}
	if old.IsRecent() {
		t.Error("agent modified 3h ago should not be recent")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/... -v`
Expected: FAIL - package does not exist

**Step 3: Write minimal implementation**

```go
// internal/agent/agent.go
package agent

import "time"

const (
	activeThreshold = 10 * time.Second
	recentThreshold = 2 * time.Hour
)

// Source identifies which system spawned an agent.
const (
	SourceClaude = "claude"
	SourceCodex  = "codex"
)

// Agent represents any AI coding agent (Claude, Codex, etc.)
type Agent struct {
	// Identity
	ID     string // ULID or extracted from filename
	Name   string // Display name (user-given for Codex, extracted from transcript for Claude)
	Source string // "claude" or "codex"

	// Channel grouping
	RepoPath string // Git repo path
	Branch   string // Git branch

	// Transcript
	TranscriptPath string // Path to JSONL or session file

	// Activity
	LastActivity time.Time
	PID          int // Process ID if running, 0 otherwise
}

// DisplayName returns the best name for UI display.
// Falls back to ID if Name is empty.
func (a Agent) DisplayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// IsActive returns true if the agent was recently modified.
func (a Agent) IsActive() bool {
	return time.Since(a.LastActivity) < activeThreshold
}

// IsRecent returns true if the agent was modified within 2 hours.
func (a Agent) IsRecent() bool {
	return time.Since(a.LastActivity) < recentThreshold
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): add unified Agent type

Creates internal/agent/ package with shared Agent struct that
represents both Claude and Codex agents with common interface."
```

---

## Phase 2: Add Git Context to Database

### Task 2: Add repo_path and branch columns to DB schema

**Files:**
- Modify: `internal/db/db.go:27-36`
- Test: `internal/db/db_test.go`

**Step 1: Write the failing test**

Add to `internal/db/db_test.go`:

```go
func TestCreateAgent_WithGitContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "test-agent",
		ULID:        "01234567890",
		SessionFile: "/tmp/session.jsonl",
		PID:         1234,
		RepoPath:    "/Users/test/code/myproject",
		Branch:      "main",
	}

	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := db.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if got.RepoPath != agent.RepoPath {
		t.Errorf("RepoPath = %q, want %q", got.RepoPath, agent.RepoPath)
	}
	if got.Branch != agent.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, agent.Branch)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v -run TestCreateAgent_WithGitContext`
Expected: FAIL - Agent struct has no field RepoPath

**Step 3: Write minimal implementation**

Update `internal/db/db.go`:

```go
// Agent represents a spawned Codex agent
type Agent struct {
	Name        string
	ULID        string
	SessionFile string
	Cursor      int
	PID         int
	SpawnedAt   time.Time
	RepoPath    string // Git repo path for channel grouping
	Branch      string // Git branch for channel grouping
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name TEXT PRIMARY KEY,
	ulid TEXT NOT NULL,
	session_file TEXT NOT NULL,
	cursor INTEGER DEFAULT 0,
	pid INTEGER,
	spawned_at TEXT NOT NULL,
	repo_path TEXT DEFAULT '',
	branch TEXT DEFAULT ''
);
`

// CreateAgent inserts a new agent record
func (db *DB) CreateAgent(a Agent) error {
	_, err := db.Exec(
		`INSERT INTO agents (name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch)
		 VALUES (?, ?, ?, 0, ?, ?, ?, ?)`,
		a.Name, a.ULID, a.SessionFile, a.PID, time.Now().UTC().Format(time.RFC3339),
		a.RepoPath, a.Branch,
	)
	return err
}

// GetAgent retrieves an agent by name
func (db *DB) GetAgent(name string) (*Agent, error) {
	var a Agent
	var spawnedAt string
	err := db.QueryRow(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch)
	if err == sql.ErrNoRows {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	var parseErr error
	a.SpawnedAt, parseErr = time.Parse(time.RFC3339, spawnedAt)
	if parseErr != nil {
		log.Printf("warning: failed to parse spawned_at for agent %s: %v", name, parseErr)
	}
	return &a, nil
}

// ListAgents returns all agents
func (db *DB) ListAgents() ([]Agent, error) {
	rows, err := db.Query(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch
		 FROM agents ORDER BY spawned_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var spawnedAt string
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch); err != nil {
			return nil, err
		}
		var parseErr error
		a.SpawnedAt, parseErr = time.Parse(time.RFC3339, spawnedAt)
		if parseErr != nil {
			log.Printf("warning: failed to parse spawned_at for agent %s: %v", a.Name, parseErr)
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return agents, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat(db): add repo_path and branch columns for channel grouping"
```

---

### Task 3: Add DB migration for existing databases

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Step 1: Write the failing test**

```go
func TestMigration_AddsNewColumns(t *testing.T) {
	// Create a DB with the OLD schema (no repo_path, branch)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Manually create old schema
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE agents (
			name TEXT PRIMARY KEY,
			ulid TEXT NOT NULL,
			session_file TEXT NOT NULL,
			cursor INTEGER DEFAULT 0,
			pid INTEGER,
			spawned_at TEXT NOT NULL
		);
		INSERT INTO agents (name, ulid, session_file, pid, spawned_at)
		VALUES ('old-agent', 'ulid123', '/tmp/session.jsonl', 0, '2025-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
	rawDB.Close()

	// Now open with our Open() which should migrate
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Old agent should still be readable with empty repo_path/branch
	agent, err := db.GetAgent("old-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent.RepoPath != "" {
		t.Errorf("expected empty RepoPath for migrated agent, got %q", agent.RepoPath)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v -run TestMigration_AddsNewColumns`
Expected: FAIL - column repo_path does not exist

**Step 3: Write minimal implementation**

Add migration logic to `internal/db/db.go`:

```go
const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name TEXT PRIMARY KEY,
	ulid TEXT NOT NULL,
	session_file TEXT NOT NULL,
	cursor INTEGER DEFAULT 0,
	pid INTEGER,
	spawned_at TEXT NOT NULL,
	repo_path TEXT DEFAULT '',
	branch TEXT DEFAULT ''
);
`

// migrate runs schema migrations for existing databases
func migrate(db *sql.DB) error {
	// Check if repo_path column exists
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='repo_path'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		// Add missing columns
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN repo_path TEXT DEFAULT ''`); err != nil {
			return err
		}
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN branch TEXT DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// Open opens or creates the SQLite database at the given path
func Open(path string) (*DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Create schema (for new DBs)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	// Run migrations (for existing DBs)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat(db): add migration for repo_path and branch columns"
```

---

## Phase 3: Capture Git Context at Spawn Time

### Task 4: Update spawn command to capture git context

**Files:**
- Modify: `internal/cli/spawn.go`
- Uses: `internal/scope/` (existing git detection)

**Step 1: Read current spawn.go to understand structure**

Run: `cat internal/cli/spawn.go | head -100`

**Step 2: Write the implementation**

Find where `db.CreateAgent` is called and add git context:

```go
// Near the top of the spawn command's Run function, after getting cwd:
gitCtx, err := scope.DetectGitContext(cwd)
if err != nil {
	// Non-fatal: continue without git context
	gitCtx = &scope.GitContext{}
}

// When creating the agent, include git context:
agent := db.Agent{
	Name:        name,
	ULID:        ulid,
	SessionFile: sessionFile,
	PID:         pid,
	RepoPath:    gitCtx.RepoPath,
	Branch:      gitCtx.Branch,
}
```

**Step 3: Run tests**

Run: `go test ./internal/cli/... -v`
Expected: PASS (or adjust if there are spawn tests)

**Step 4: Manual verification**

Run: `go build -o june . && ./june spawn codex "test task" --name test-git-ctx`
Then: `sqlite3 ~/.june/june.db "SELECT name, repo_path, branch FROM agents WHERE name='test-git-ctx'"`
Expected: Shows current repo path and branch

**Step 5: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(spawn): capture git context when spawning Codex agents"
```

---

## Phase 4: Add Conversion Functions

### Task 5: Add ToUnifiedAgent function in db package

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Step 1: Write the failing test**

```go
func TestAgent_ToUnified(t *testing.T) {
	dbAgent := Agent{
		Name:        "my-agent",
		ULID:        "ulid123",
		SessionFile: "/path/to/session.jsonl",
		Cursor:      100,
		PID:         1234,
		SpawnedAt:   time.Now(),
		RepoPath:    "/Users/test/code/project",
		Branch:      "feature",
	}

	unified := dbAgent.ToUnified()

	if unified.ID != dbAgent.ULID {
		t.Errorf("ID = %q, want %q", unified.ID, dbAgent.ULID)
	}
	if unified.Name != dbAgent.Name {
		t.Errorf("Name = %q, want %q", unified.Name, dbAgent.Name)
	}
	if unified.Source != agent.SourceCodex {
		t.Errorf("Source = %q, want %q", unified.Source, agent.SourceCodex)
	}
	if unified.TranscriptPath != dbAgent.SessionFile {
		t.Errorf("TranscriptPath = %q, want %q", unified.TranscriptPath, dbAgent.SessionFile)
	}
	if unified.RepoPath != dbAgent.RepoPath {
		t.Errorf("RepoPath = %q, want %q", unified.RepoPath, dbAgent.RepoPath)
	}
	if unified.Branch != dbAgent.Branch {
		t.Errorf("Branch = %q, want %q", unified.Branch, dbAgent.Branch)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v -run TestAgent_ToUnified`
Expected: FAIL - ToUnified not defined

**Step 3: Write minimal implementation**

Add to `internal/db/db.go`:

```go
import (
	"github.com/sky-xo/june/internal/agent"
)

// ToUnified converts a db.Agent to the unified agent.Agent type.
func (a Agent) ToUnified() agent.Agent {
	return agent.Agent{
		ID:             a.ULID,
		Name:           a.Name,
		Source:         agent.SourceCodex,
		RepoPath:       a.RepoPath,
		Branch:         a.Branch,
		TranscriptPath: a.SessionFile,
		LastActivity:   a.SpawnedAt, // TODO: use session file mod time
		PID:            a.PID,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat(db): add ToUnified conversion to agent.Agent"
```

---

### Task 6: Add ToUnifiedAgent function in claude package

**Files:**
- Modify: `internal/claude/agents.go`
- Test: `internal/claude/agents_test.go`

**Step 1: Write the failing test**

```go
func TestAgent_ToUnified(t *testing.T) {
	claudeAgent := Agent{
		ID:          "abc123",
		FilePath:    "/path/to/agent-abc123.jsonl",
		LastMod:     time.Now(),
		Description: "Fix the auth bug",
	}

	// Channel context would be provided by the caller
	unified := claudeAgent.ToUnified("/Users/test/project", "main")

	if unified.ID != claudeAgent.ID {
		t.Errorf("ID = %q, want %q", unified.ID, claudeAgent.ID)
	}
	if unified.Name != claudeAgent.Description {
		t.Errorf("Name = %q, want %q (from Description)", unified.Name, claudeAgent.Description)
	}
	if unified.Source != agent.SourceClaude {
		t.Errorf("Source = %q, want %q", unified.Source, agent.SourceClaude)
	}
	if unified.RepoPath != "/Users/test/project" {
		t.Errorf("RepoPath = %q, want %q", unified.RepoPath, "/Users/test/project")
	}
	if unified.Branch != "main" {
		t.Errorf("Branch = %q, want %q", unified.Branch, "main")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/... -v -run TestAgent_ToUnified`
Expected: FAIL - ToUnified not defined

**Step 3: Write minimal implementation**

Add to `internal/claude/agents.go`:

```go
import (
	"github.com/sky-xo/june/internal/agent"
)

// ToUnified converts a claude.Agent to the unified agent.Agent type.
// repoPath and branch come from the channel context.
func (a Agent) ToUnified(repoPath, branch string) agent.Agent {
	return agent.Agent{
		ID:             a.ID,
		Name:           a.Description, // Use extracted description as display name
		Source:         agent.SourceClaude,
		RepoPath:       repoPath,
		Branch:         branch,
		TranscriptPath: a.FilePath,
		LastActivity:   a.LastMod,
		PID:            0, // Claude agents don't track PID
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/
git commit -m "feat(claude): add ToUnified conversion to agent.Agent"
```

---

## Phase 5: Merge Agent Sources in Channel Scanning

### Task 7: Add ListAgentsByRepo to db package

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Step 1: Write the failing test**

```go
func TestListAgentsByRepo(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create agents in different repos
	db.CreateAgent(Agent{Name: "a1", ULID: "1", SessionFile: "/tmp/1.jsonl", RepoPath: "/code/project", Branch: "main"})
	db.CreateAgent(Agent{Name: "a2", ULID: "2", SessionFile: "/tmp/2.jsonl", RepoPath: "/code/project", Branch: "feature"})
	db.CreateAgent(Agent{Name: "a3", ULID: "3", SessionFile: "/tmp/3.jsonl", RepoPath: "/code/other", Branch: "main"})

	agents, err := db.ListAgentsByRepo("/code/project")
	if err != nil {
		t.Fatalf("ListAgentsByRepo failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v -run TestListAgentsByRepo`
Expected: FAIL - ListAgentsByRepo not defined

**Step 3: Write minimal implementation**

```go
// ListAgentsByRepo returns agents matching the given repo path.
func (db *DB) ListAgentsByRepo(repoPath string) ([]Agent, error) {
	rows, err := db.Query(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch
		 FROM agents WHERE repo_path = ? ORDER BY spawned_at DESC`,
		repoPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var spawnedAt string
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch); err != nil {
			return nil, err
		}
		a.SpawnedAt, _ = time.Parse(time.RFC3339, spawnedAt)
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat(db): add ListAgentsByRepo for filtering by repo path"
```

---

### Task 8: Create unified Channel type in agent package

**Files:**
- Modify: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

**Step 1: Write the implementation**

Add to `internal/agent/agent.go`:

```go
// Channel represents a group of agents from a branch/worktree.
type Channel struct {
	Name   string  // Display name like "june:main"
	Agents []Agent // Mixed Claude and Codex agents
}

// HasRecentActivity returns true if any agent is active or recent.
func (c Channel) HasRecentActivity() bool {
	for _, a := range c.Agents {
		if a.IsActive() || a.IsRecent() {
			return true
		}
	}
	return false
}
```

**Step 2: Add test**

```go
func TestChannel_HasRecentActivity(t *testing.T) {
	recent := Channel{
		Agents: []Agent{
			{LastActivity: time.Now().Add(-1 * time.Hour)},
		},
	}
	if !recent.HasRecentActivity() {
		t.Error("channel with recent agent should have recent activity")
	}

	old := Channel{
		Agents: []Agent{
			{LastActivity: time.Now().Add(-24 * time.Hour)},
		},
	}
	if old.HasRecentActivity() {
		t.Error("channel with old agent should not have recent activity")
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/agent/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): add Channel type for grouping agents"
```

---

### Task 9: Update ScanChannels to merge Codex agents

**Files:**
- Modify: `internal/claude/channels.go`
- Test: `internal/claude/channels_test.go`

**Step 1: Update ScanChannels signature and implementation**

This is a larger change - update `ScanChannels` to:
1. Accept a db connection
2. Load Codex agents for the repo
3. Merge them into channels by branch
4. Return `[]agent.Channel` instead of `[]Channel`

```go
// ScanChannels scans Claude project directories and merges Codex agents.
func ScanChannels(claudeProjectsDir, basePath, repoName string, codexDB *db.DB) ([]agent.Channel, error) {
	relatedDirs := FindRelatedProjectDirs(claudeProjectsDir, basePath)

	// Map branch -> agents
	channelMap := make(map[string][]agent.Agent)

	baseDir := strings.ReplaceAll(basePath, "/", "-")

	// 1. Scan Claude agents
	for _, dir := range relatedDirs {
		dirName := filepath.Base(dir)
		channelName := ExtractChannelName(baseDir, dirName, repoName)

		claudeAgents, err := ScanAgents(dir)
		if err != nil {
			continue
		}

		// Extract branch from channel name (e.g., "june:main" -> "main")
		branch := strings.TrimPrefix(channelName, repoName+":")

		for _, ca := range claudeAgents {
			channelMap[channelName] = append(channelMap[channelName], ca.ToUnified(basePath, branch))
		}
	}

	// 2. Load Codex agents for this repo
	if codexDB != nil {
		codexAgents, err := codexDB.ListAgentsByRepo(basePath)
		if err == nil {
			for _, ca := range codexAgents {
				channelName := repoName + ":" + ca.Branch
				if ca.Branch == "" {
					channelName = repoName + ":main"
				}
				channelMap[channelName] = append(channelMap[channelName], ca.ToUnified())
			}
		}
	}

	// 3. Build and sort channels
	var channels []agent.Channel
	for name, agents := range channelMap {
		channels = append(channels, agent.Channel{Name: name, Agents: agents})
	}

	// Sort: recent activity first, then alphabetically
	sort.Slice(channels, func(i, j int) bool {
		iRecent := channels[i].HasRecentActivity()
		jRecent := channels[j].HasRecentActivity()
		if iRecent != jRecent {
			return iRecent
		}
		return channels[i].Name < channels[j].Name
	})

	return channels, nil
}
```

**Step 2: Update tests**

Update existing channel tests to use new signature.

**Step 3: Run tests**

Run: `go test ./internal/claude/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/claude/
git commit -m "feat(claude): merge Codex agents into channel scanning"
```

---

## Phase 6: Update TUI

### Task 10: Update TUI model to use agent.Channel

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/commands.go`

**Step 1: Update imports and types**

Replace `claude.Channel` with `agent.Channel` throughout.

**Step 2: Update scanChannelsCmd to pass DB**

The TUI needs access to the Codex database to merge agents.

**Step 3: Update rendering to use DisplayName()**

Replace `agent.Description` with `agent.DisplayName()` in sidebar rendering.

**Step 4: Run and verify**

Run: `go build -o june . && ./june`
Expected: TUI shows both Claude and Codex agents in sidebar

**Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): display Codex agents in sidebar alongside Claude agents"
```

---

## Final Verification

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Manual verification**

1. Spawn a Codex agent: `./june spawn codex "test task" --name test-integration`
2. Open TUI: `./june`
3. Verify Codex agent appears in sidebar under current branch channel

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat: complete TUI Codex integration

Codex agents now appear in TUI sidebar alongside Claude agents,
grouped by git branch. Includes:
- Unified agent.Agent type
- DB schema migration for git context
- Git context capture at spawn time
- Merged channel scanning
- Updated TUI rendering"
```

---

Plan complete and saved to `docs/plans/2026-01-03-tui-codex-integration-impl.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**

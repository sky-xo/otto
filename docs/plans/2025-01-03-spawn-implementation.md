# Spawn Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `june spawn codex`, `june peek`, and `june logs` commands to spawn and monitor Codex agents.

**Architecture:** New `internal/db` package for SQLite, new `internal/codex` package for Codex session parsing, extend CLI with subcommands.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Cobra CLI

---

## Task 1: Add SQLite dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add SQLite dependency**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go get modernc.org/sqlite
```

**Step 2: Verify dependency added**

Run:
```bash
grep sqlite /Users/glowy/code/june/.worktrees/spawn/go.mod
```
Expected: `modernc.org/sqlite` appears in require block

**Step 3: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add go.mod go.sum && git commit -m "chore: add sqlite dependency"
```

---

## Task 2: Create database package with schema

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

**Step 1: Write the failing test**

Create `internal/db/db_test.go`:

```go
package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenCreatesAgentsTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify agents table exists by querying it
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		t.Errorf("agents table does not exist: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/db/...
```
Expected: FAIL - package does not exist

**Step 3: Write minimal implementation**

Create `internal/db/db.go`:

```go
package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name TEXT PRIMARY KEY,
	ulid TEXT NOT NULL,
	session_file TEXT NOT NULL,
	cursor INTEGER DEFAULT 0,
	pid INTEGER,
	spawned_at TEXT NOT NULL
);
`

// DB wraps a SQLite database connection
type DB struct {
	*sql.DB
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

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/db/...
```
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/db/ && git commit -m "feat(db): add sqlite database package with schema"
```

---

## Task 3: Add agent CRUD operations

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

**Step 1: Write the failing tests**

Add to `internal/db/db_test.go`:

```go
func TestCreateAndGetAgent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "impl-1",
		ULID:        "019b825b-b138-7981-898d-2830d3610fc9",
		SessionFile: "/path/to/session.jsonl",
		PID:         12345,
	}

	err := db.CreateAgent(agent)
	if err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := db.GetAgent("impl-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if got.Name != agent.Name {
		t.Errorf("Name = %q, want %q", got.Name, agent.Name)
	}
	if got.ULID != agent.ULID {
		t.Errorf("ULID = %q, want %q", got.ULID, agent.ULID)
	}
	if got.SessionFile != agent.SessionFile {
		t.Errorf("SessionFile = %q, want %q", got.SessionFile, agent.SessionFile)
	}
	if got.PID != agent.PID {
		t.Errorf("PID = %d, want %d", got.PID, agent.PID)
	}
	if got.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", got.Cursor)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestUpdateCursor(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "impl-1",
		ULID:        "test-ulid",
		SessionFile: "/path/to/session.jsonl",
		PID:         12345,
	}
	db.CreateAgent(agent)

	err := db.UpdateCursor("impl-1", 42)
	if err != nil {
		t.Fatalf("UpdateCursor failed: %v", err)
	}

	got, _ := db.GetAgent("impl-1")
	if got.Cursor != 42 {
		t.Errorf("Cursor = %d, want 42", got.Cursor)
	}
}

func TestListAgents(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	db.CreateAgent(Agent{Name: "a", ULID: "ulid-a", SessionFile: "/a.jsonl", PID: 1})
	db.CreateAgent(Agent{Name: "b", ULID: "ulid-b", SessionFile: "/b.jsonl", PID: 2})

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("len(agents) = %d, want 2", len(agents))
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	return db
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/db/...
```
Expected: FAIL - Agent type undefined, methods undefined

**Step 3: Write implementation**

Add to `internal/db/db.go`:

```go
import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrAgentNotFound is returned when an agent is not found
var ErrAgentNotFound = errors.New("agent not found")

// Agent represents a spawned Codex agent
type Agent struct {
	Name        string
	ULID        string
	SessionFile string
	Cursor      int
	PID         int
	SpawnedAt   time.Time
}

// CreateAgent inserts a new agent record
func (db *DB) CreateAgent(a Agent) error {
	_, err := db.Exec(
		`INSERT INTO agents (name, ulid, session_file, cursor, pid, spawned_at)
		 VALUES (?, ?, ?, 0, ?, ?)`,
		a.Name, a.ULID, a.SessionFile, a.PID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetAgent retrieves an agent by name
func (db *DB) GetAgent(name string) (*Agent, error) {
	var a Agent
	var spawnedAt string
	err := db.QueryRow(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	a.SpawnedAt, _ = time.Parse(time.RFC3339, spawnedAt)
	return &a, nil
}

// UpdateCursor updates the cursor position for an agent
func (db *DB) UpdateCursor(name string, cursor int) error {
	result, err := db.Exec(`UPDATE agents SET cursor = ? WHERE name = ?`, cursor, name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// ListAgents returns all agents
func (db *DB) ListAgents() ([]Agent, error) {
	rows, err := db.Query(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at
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
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt); err != nil {
			return nil, err
		}
		a.SpawnedAt, _ = time.Parse(time.RFC3339, spawnedAt)
		agents = append(agents, a)
	}
	return agents, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/db/...
```
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/db/ && git commit -m "feat(db): add agent CRUD operations"
```

---

## Task 4: Create Codex session file finder

**Files:**
- Create: `internal/codex/sessions.go`
- Create: `internal/codex/sessions_test.go`

**Step 1: Write the failing test**

Create `internal/codex/sessions_test.go`:

```go
package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindSessionFile(t *testing.T) {
	// Create temp directory structure mimicking Codex
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions", "2025", "01", "03")
	os.MkdirAll(sessionsDir, 0755)

	// Create a session file with known ULID
	ulid := "019b825b-b138-7981-898d-2830d3610fc9"
	filename := "rollout-2025-01-03T10-00-00-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{"type":"session_meta"}`), 0644)

	// Find it
	found, err := FindSessionFile(tmpDir, ulid)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileToday(t *testing.T) {
	// Create temp directory with today's date
	tmpDir := t.TempDir()
	now := time.Now()
	sessionsDir := filepath.Join(tmpDir, "sessions",
		now.Format("2006"), now.Format("01"), now.Format("02"))
	os.MkdirAll(sessionsDir, 0755)

	ulid := "test-ulid-12345"
	filename := "rollout-" + now.Format("2006-01-02T15-04-05") + "-" + ulid + ".jsonl"
	sessionFile := filepath.Join(sessionsDir, filename)
	os.WriteFile(sessionFile, []byte(`{}`), 0644)

	found, err := FindSessionFile(tmpDir, ulid)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := FindSessionFile(tmpDir, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/codex/...
```
Expected: FAIL - package does not exist

**Step 3: Write implementation**

Create `internal/codex/sessions.go`:

```go
package codex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSessionNotFound is returned when a session file cannot be found
var ErrSessionNotFound = errors.New("session file not found")

// CodexHome returns the Codex home directory
func CodexHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

// FindSessionFile finds a Codex session file by ULID
// It searches in the sessions directory structure: ~/.codex/sessions/YYYY/MM/DD/
func FindSessionFile(codexHome string, ulid string) (string, error) {
	sessionsDir := filepath.Join(codexHome, "sessions")

	// First, try today's directory (most common case)
	now := time.Now()
	todayDir := filepath.Join(sessionsDir, now.Format("2006"), now.Format("01"), now.Format("02"))
	if path, found := findInDir(todayDir, ulid); found {
		return path, nil
	}

	// Otherwise, search all directories (less common)
	var foundPath string
	err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), ulid) && strings.HasSuffix(info.Name(), ".jsonl") {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if foundPath != "" {
		return foundPath, nil
	}

	return "", ErrSessionNotFound
}

func findInDir(dir string, ulid string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ulid) && strings.HasSuffix(e.Name(), ".jsonl") {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./internal/codex/...
```
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/codex/ && git commit -m "feat(codex): add session file finder"
```

---

## Task 5: Add spawn command

**Files:**
- Create: `internal/cli/spawn.go`
- Modify: `internal/cli/root.go`

**Step 1: Create spawn command**

Create `internal/cli/spawn.go`:

```go
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/spf13/cobra"
)

func newSpawnCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn an agent",
		Long:  "Spawn a Codex agent to perform a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentType := args[0]
			task := args[1]

			if agentType != "codex" {
				return fmt.Errorf("unsupported agent type: %s (only 'codex' is supported)", agentType)
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			return runSpawnCodex(name, task)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the agent (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func runSpawnCodex(name, task string) error {
	// Open database
	dbPath := filepath.Join(juneHome(), "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Check if agent already exists
	if _, err := database.GetAgent(name); err == nil {
		return fmt.Errorf("agent %q already exists", name)
	}

	// Start codex exec --json
	codexCmd := exec.Command("codex", "exec", "--json", task)
	codexCmd.Stderr = os.Stderr

	stdout, err := codexCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := codexCmd.Start(); err != nil {
		return fmt.Errorf("failed to start codex: %w", err)
	}

	// Read first line to get thread_id
	scanner := bufio.NewScanner(stdout)
	var threadID string
	if scanner.Scan() {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			if event.Type == "thread.started" {
				threadID = event.ThreadID
			}
		}
		// Echo this line to stdout
		fmt.Println(scanner.Text())
	}

	if threadID == "" {
		codexCmd.Process.Kill()
		return fmt.Errorf("failed to get thread_id from codex output")
	}

	// Find the session file
	sessionFile, err := codex.FindSessionFile(codex.CodexHome(), threadID)
	if err != nil {
		// Session file might not exist yet, construct expected path
		// For now, we'll store it and look it up later
		sessionFile = "" // Will be populated later
	}

	// Create agent record
	agent := db.Agent{
		Name:        name,
		ULID:        threadID,
		SessionFile: sessionFile,
		PID:         codexCmd.Process.Pid,
	}
	if err := database.CreateAgent(agent); err != nil {
		return fmt.Errorf("failed to create agent record: %w", err)
	}

	// Stream remaining output
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	// Wait for process to finish
	if err := codexCmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "codex exited with error: %v\n", err)
	}

	// Update session file if we didn't have it
	if sessionFile == "" {
		if found, err := codex.FindSessionFile(codex.CodexHome(), threadID); err == nil {
			// Update the agent record with the session file
			database.Exec("UPDATE agents SET session_file = ? WHERE name = ?", found, name)
		}
	}

	return nil
}

func juneHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".june")
}
```

**Step 2: Add spawn command to root**

Modify `internal/cli/root.go` - add to Execute() after rootCmd creation:

```go
rootCmd.AddCommand(newSpawnCmd())
```

**Step 3: Build and test manually**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go build -o june .
./june spawn --help
```
Expected: Shows spawn command help

**Step 4: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/cli/ && git commit -m "feat(cli): add spawn command"
```

---

## Task 6: Add peek command

**Files:**
- Create: `internal/cli/peek.go`
- Create: `internal/codex/transcript.go`
- Modify: `internal/cli/root.go`

**Step 1: Create transcript reader**

Create `internal/codex/transcript.go`:

```go
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TranscriptEntry represents a parsed entry from a Codex session file
type TranscriptEntry struct {
	Type    string
	Content string
}

// ReadTranscript reads a Codex session file from the given line offset
// Returns entries and the new line count
func ReadTranscript(path string, fromLine int) ([]TranscriptEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fromLine, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Set larger buffer for long lines
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024)

	var entries []TranscriptEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= fromLine {
			continue
		}

		entry := parseEntry(scanner.Bytes())
		if entry.Content != "" {
			entries = append(entries, entry)
		}
	}

	return entries, lineNum, scanner.Err()
}

func parseEntry(data []byte) TranscriptEntry {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return TranscriptEntry{}
	}

	entryType, _ := raw["type"].(string)

	switch entryType {
	case "response_item":
		// Extract message content
		if payload, ok := raw["payload"].(map[string]interface{}); ok {
			if content, ok := payload["content"].(string); ok {
				return TranscriptEntry{Type: "message", Content: content}
			}
		}
	case "agent_reasoning":
		if content, ok := raw["content"].(string); ok {
			return TranscriptEntry{Type: "reasoning", Content: content}
		}
	case "function_call":
		if name, ok := raw["name"].(string); ok {
			return TranscriptEntry{Type: "tool", Content: fmt.Sprintf("[tool: %s]", name)}
		}
	case "function_call_output":
		if output, ok := raw["output"].(string); ok {
			// Truncate long outputs
			if len(output) > 200 {
				output = output[:200] + "..."
			}
			return TranscriptEntry{Type: "tool_output", Content: output}
		}
	}

	return TranscriptEntry{}
}

// FormatEntries formats transcript entries for display
func FormatEntries(entries []TranscriptEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case "message":
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "reasoning":
			sb.WriteString("[thinking] ")
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "tool":
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		case "tool_output":
			sb.WriteString("  → ")
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
```

**Step 2: Create peek command**

Create `internal/cli/peek.go`:

```go
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/spf13/cobra"
)

func newPeekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peek <name>",
		Short: "Show new output from an agent",
		Long:  "Show output since last peek and advance the cursor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return runPeek(name)
		},
	}
}

func runPeek(name string) error {
	// Open database
	dbPath := filepath.Join(juneHome(), "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Get agent
	agent, err := database.GetAgent(name)
	if err == db.ErrAgentNotFound {
		return fmt.Errorf("agent %q not found", name)
	}
	if err != nil {
		return err
	}

	// Find session file if not set
	sessionFile := agent.SessionFile
	if sessionFile == "" {
		found, err := codex.FindSessionFile(codex.CodexHome(), agent.ULID)
		if err != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
		sessionFile = found
		// Update in database
		database.Exec("UPDATE agents SET session_file = ? WHERE name = ?", found, name)
	}

	// Read from cursor
	entries, newCursor, err := codex.ReadTranscript(sessionFile, agent.Cursor)
	if err != nil {
		return fmt.Errorf("failed to read transcript: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("(no new output)")
		return nil
	}

	// Update cursor
	if err := database.UpdateCursor(name, newCursor); err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	// Print entries
	fmt.Print(codex.FormatEntries(entries))
	return nil
}
```

**Step 3: Add to root.go**

Add after spawn command:
```go
rootCmd.AddCommand(newPeekCmd())
```

**Step 4: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/cli/ internal/codex/ && git commit -m "feat(cli): add peek command with transcript reader"
```

---

## Task 7: Add logs command

**Files:**
- Create: `internal/cli/logs.go`
- Modify: `internal/cli/root.go`

**Step 1: Create logs command**

Create `internal/cli/logs.go`:

```go
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Show full transcript from an agent",
		Long:  "Show full transcript without advancing the cursor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return runLogs(name)
		},
	}
}

func runLogs(name string) error {
	// Open database
	dbPath := filepath.Join(juneHome(), "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Get agent
	agent, err := database.GetAgent(name)
	if err == db.ErrAgentNotFound {
		return fmt.Errorf("agent %q not found", name)
	}
	if err != nil {
		return err
	}

	// Find session file if not set
	sessionFile := agent.SessionFile
	if sessionFile == "" {
		found, err := codex.FindSessionFile(codex.CodexHome(), agent.ULID)
		if err != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
		sessionFile = found
	}

	// Read from beginning (cursor 0), don't update cursor
	entries, _, err := codex.ReadTranscript(sessionFile, 0)
	if err != nil {
		return fmt.Errorf("failed to read transcript: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("(no output)")
		return nil
	}

	fmt.Print(codex.FormatEntries(entries))
	return nil
}
```

**Step 2: Add to root.go**

Add after peek command:
```go
rootCmd.AddCommand(newLogsCmd())
```

**Step 3: Build and verify all commands work**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go build -o june . && ./june --help
```
Expected: Shows spawn, peek, logs subcommands

**Step 4: Commit**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add internal/cli/ && git commit -m "feat(cli): add logs command"
```

---

## Task 8: Run all tests and verify build

**Step 1: Run all tests**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go test ./...
```
Expected: All tests pass

**Step 2: Build**

Run:
```bash
cd /Users/glowy/code/june/.worktrees/spawn && go build -o june .
```
Expected: Binary builds successfully

**Step 3: Manual smoke test**

Run:
```bash
./june --help
./june spawn --help
./june peek --help
./june logs --help
```
Expected: All help outputs display correctly

**Step 4: Commit any final fixes**

If any fixes needed, commit them.

---

## Task 9: Update CLAUDE.md and commit design doc

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update CLAUDE.md with new commands**

Add section about spawn commands:

```markdown
## Spawn Commands

Spawn and monitor Codex agents:

```bash
june spawn codex "task" --name <name>   # Spawn a Codex agent
june peek <name>                         # Show new output since last peek
june logs <name>                         # Show full transcript
```

Agent state is stored in `~/.june/june.db` (SQLite).
```

**Step 2: Commit design doc and CLAUDE.md**

```bash
cd /Users/glowy/code/june/.worktrees/spawn && git add docs/plans/ CLAUDE.md && git commit -m "docs: add spawn design and update CLAUDE.md"
```

---

## Summary

After completing all tasks, you will have:

1. SQLite database at `~/.june/june.db` for agent state
2. `june spawn codex "task" --name <name>` - spawns Codex agent, streams output
3. `june peek <name>` - shows new output, advances cursor
4. `june logs <name>` - shows full transcript

TUI integration (showing Codex agents alongside Claude agents) is deferred to a follow-up task.

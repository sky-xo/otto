# Gemini Spawn Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `june spawn gemini` with feature parity to Codex spawning.

**Architecture:** New `internal/gemini/` package mirrors `internal/codex/`. Database gets `type` column. peek/logs dispatch based on agent type.

**Tech Stack:** Go, SQLite, Cobra CLI, bufio for stream capture

---

## Task 1: Add SourceGemini constant

**Files:**
- Modify: `internal/agent/agent.go:12-15`

**Step 1: Add the constant**

In `internal/agent/agent.go`, add `SourceGemini` to the constants:

```go
// Source identifies which system spawned an agent.
const (
	SourceClaude = "claude"
	SourceCodex  = "codex"
	SourceGemini = "gemini"
)
```

**Step 2: Run tests to verify no regressions**

Run: `go test ./internal/agent/...`
Expected: PASS (or no tests exist yet)

**Step 3: Commit**

```bash
git add internal/agent/agent.go
git commit -m "feat(agent): add SourceGemini constant"
```

---

## Task 2: Add type column to database

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Step 1: Write failing test for type field**

Add to `internal/db/db_test.go`:

```go
func TestAgentTypeField(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Create agent with type
	agent := Agent{
		Name:        "test-agent",
		ULID:        "test-ulid",
		SessionFile: "/tmp/session.jsonl",
		Type:        "gemini",
	}
	if err := database.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	// Retrieve and verify type
	got, err := database.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Type != "gemini" {
		t.Errorf("Type = %q, want %q", got.Type, "gemini")
	}
}

func TestAgentTypeDefaultsToCodex(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Create agent without type (should default to codex)
	agent := Agent{
		Name:        "test-agent",
		ULID:        "test-ulid",
		SessionFile: "/tmp/session.jsonl",
	}
	if err := database.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := database.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Type != "codex" {
		t.Errorf("Type = %q, want %q", got.Type, "codex")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -run TestAgentType -v`
Expected: FAIL (Type field doesn't exist)

**Step 3: Add Type field to Agent struct**

In `internal/db/db.go`, update the Agent struct:

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
	Type        string // "codex" or "gemini"
}
```

**Step 4: Update schema constant**

Update the schema to include type column:

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
	branch TEXT DEFAULT '',
	type TEXT DEFAULT 'codex'
);
`
```

**Step 5: Add migration for type column**

In the `migrate` function, add:

```go
	// Check if type column exists and add if missing
	var typeCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='type'`).Scan(&typeCount)
	if err != nil {
		return err
	}
	if typeCount == 0 {
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN type TEXT DEFAULT 'codex'`); err != nil {
			return err
		}
	}
```

**Step 6: Update CreateAgent to include type**

```go
func (db *DB) CreateAgent(a Agent) error {
	agentType := a.Type
	if agentType == "" {
		agentType = "codex"
	}
	_, err := db.Exec(
		`INSERT INTO agents (name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type)
		 VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		a.Name, a.ULID, a.SessionFile, a.PID, time.Now().UTC().Format(time.RFC3339),
		a.RepoPath, a.Branch, agentType,
	)
	return err
}
```

**Step 7: Update GetAgent to read type**

Update the SELECT and Scan in GetAgent:

```go
func (db *DB) GetAgent(name string) (*Agent, error) {
	var a Agent
	var spawnedAt string
	err := db.QueryRow(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch, &a.Type)
	// ... rest unchanged
}
```

**Step 8: Update ListAgents and ListAgentsByRepo similarly**

Add `type` to SELECT and Scan in both functions.

**Step 9: Update ToUnified to use Type**

```go
func (a Agent) ToUnified() agent.Agent {
	source := agent.SourceCodex
	if a.Type == "gemini" {
		source = agent.SourceGemini
	}
	// ... rest, using source instead of hardcoded agent.SourceCodex
}
```

**Step 10: Run tests**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 11: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add type column to agents table"
```

---

## Task 3: Create gemini package with home.go

**Files:**
- Create: `internal/gemini/home.go`
- Create: `internal/gemini/home_test.go`

**Step 1: Write failing test**

Create `internal/gemini/home_test.go`:

```go
package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGeminiHome(t *testing.T) {
	// Use temp dir as home
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	home, err := EnsureGeminiHome()
	if err != nil {
		t.Fatalf("EnsureGeminiHome failed: %v", err)
	}

	expected := filepath.Join(tmpHome, ".june", "gemini")
	if home != expected {
		t.Errorf("home = %q, want %q", home, expected)
	}

	// Verify directory was created
	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Errorf("directory was not created")
	}

	// Verify sessions subdirectory was created
	sessionsDir := filepath.Join(home, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Errorf("sessions directory was not created")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/gemini/... -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement EnsureGeminiHome**

Create `internal/gemini/home.go`:

```go
package gemini

import (
	"os"
	"path/filepath"
)

// EnsureGeminiHome creates the June gemini home at ~/.june/gemini/
// and the sessions subdirectory.
// Returns the path to the gemini home directory.
func EnsureGeminiHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create ~/.june/gemini/
	geminiHome := filepath.Join(home, ".june", "gemini")
	if err := os.MkdirAll(geminiHome, 0755); err != nil {
		return "", err
	}

	// Create sessions subdirectory
	sessionsDir := filepath.Join(geminiHome, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", err
	}

	return geminiHome, nil
}

// SessionsDir returns the path to the sessions directory.
func SessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".june", "gemini", "sessions"), nil
}
```

**Step 4: Run test**

Run: `go test ./internal/gemini/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gemini/home.go internal/gemini/home_test.go
git commit -m "feat(gemini): add home.go with EnsureGeminiHome"
```

---

## Task 4: Create gemini sessions.go

**Files:**
- Create: `internal/gemini/sessions.go`
- Create: `internal/gemini/sessions_test.go`

**Step 1: Write failing test**

Create `internal/gemini/sessions_test.go`:

```go
package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSessionFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create sessions directory and a test file
	sessionsDir := filepath.Join(tmpHome, ".june", "gemini", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	sessionID := "8b6238bf-8332-4fc7-ba9a-2f3323119bb2"
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"type":"init"}`), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindSessionFile(sessionID)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}

	if found != sessionFile {
		t.Errorf("found = %q, want %q", found, sessionFile)
	}
}

func TestFindSessionFileNotFound(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create empty sessions directory
	sessionsDir := filepath.Join(tmpHome, ".june", "gemini", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := FindSessionFile("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/gemini/... -run TestFindSession -v`
Expected: FAIL

**Step 3: Implement FindSessionFile**

Create `internal/gemini/sessions.go`:

```go
package gemini

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrSessionNotFound is returned when a session file cannot be found
var ErrSessionNotFound = errors.New("session file not found")

// FindSessionFile finds a Gemini session file by session ID.
// Looks in ~/.june/gemini/sessions/{session_id}.jsonl
func FindSessionFile(sessionID string) (string, error) {
	sessionsDir, err := SessionsDir()
	if err != nil {
		return "", err
	}

	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	if _, err := os.Stat(sessionFile); err != nil {
		if os.IsNotExist(err) {
			return "", ErrSessionNotFound
		}
		return "", err
	}

	return sessionFile, nil
}

// SessionFilePath returns the path where a session file should be written.
func SessionFilePath(sessionID string) (string, error) {
	sessionsDir, err := SessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(sessionsDir, sessionID+".jsonl"), nil
}
```

**Step 4: Run test**

Run: `go test ./internal/gemini/... -run TestFindSession -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gemini/sessions.go internal/gemini/sessions_test.go
git commit -m "feat(gemini): add sessions.go with FindSessionFile"
```

---

## Task 5: Create gemini transcript.go

**Files:**
- Create: `internal/gemini/transcript.go`
- Create: `internal/gemini/transcript_test.go`

**Step 1: Write failing tests for parseEntry**

Create `internal/gemini/transcript_test.go`:

```go
package gemini

import (
	"testing"
)

func TestParseEntryInit(t *testing.T) {
	data := []byte(`{"type":"init","timestamp":"2026-01-07T10:02:12.875Z","session_id":"8b6238bf","model":"auto-gemini-3"}`)
	entry := parseEntry(data)

	if entry.Type != "" {
		t.Errorf("init should be skipped, got Type=%q", entry.Type)
	}
}

func TestParseEntryUserMessage(t *testing.T) {
	data := []byte(`{"type":"message","timestamp":"...","role":"user","content":"Fix the bug"}`)
	entry := parseEntry(data)

	if entry.Type != "user" {
		t.Errorf("Type = %q, want %q", entry.Type, "user")
	}
	if entry.Content != "Fix the bug" {
		t.Errorf("Content = %q, want %q", entry.Content, "Fix the bug")
	}
}

func TestParseEntryAssistantMessage(t *testing.T) {
	data := []byte(`{"type":"message","timestamp":"...","role":"assistant","content":"I fixed it","delta":true}`)
	entry := parseEntry(data)

	if entry.Type != "message" {
		t.Errorf("Type = %q, want %q", entry.Type, "message")
	}
	if entry.Content != "I fixed it" {
		t.Errorf("Content = %q, want %q", entry.Content, "I fixed it")
	}
}

func TestParseEntryToolUse(t *testing.T) {
	data := []byte(`{"type":"tool_use","timestamp":"...","tool_name":"read_file","tool_id":"abc","parameters":{"path":"main.go"}}`)
	entry := parseEntry(data)

	if entry.Type != "tool" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool")
	}
	if entry.Content != "[tool: read_file]" {
		t.Errorf("Content = %q, want %q", entry.Content, "[tool: read_file]")
	}
}

func TestParseEntryToolResult(t *testing.T) {
	data := []byte(`{"type":"tool_result","timestamp":"...","tool_id":"abc","status":"success","output":"file contents here"}`)
	entry := parseEntry(data)

	if entry.Type != "tool_output" {
		t.Errorf("Type = %q, want %q", entry.Type, "tool_output")
	}
	if entry.Content != "file contents here" {
		t.Errorf("Content = %q, want %q", entry.Content, "file contents here")
	}
}

func TestParseEntryResult(t *testing.T) {
	data := []byte(`{"type":"result","timestamp":"...","status":"success","stats":{"total_tokens":100}}`)
	entry := parseEntry(data)

	// Result events are skipped (just stats)
	if entry.Type != "" {
		t.Errorf("result should be skipped, got Type=%q", entry.Type)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/gemini/... -run TestParseEntry -v`
Expected: FAIL

**Step 3: Implement parseEntry**

Create `internal/gemini/transcript.go`:

```go
package gemini

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TranscriptEntry represents a parsed entry from a Gemini session file
type TranscriptEntry struct {
	Type    string
	Content string
}

// ReadTranscript reads a Gemini session file from the given line offset.
// Returns entries and the new line count.
// Note: Assistant messages with delta:true are accumulated into single entries.
func ReadTranscript(path string, fromLine int) ([]TranscriptEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fromLine, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024)

	var entries []TranscriptEntry
	var pendingMessage strings.Builder
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= fromLine {
			continue
		}

		entry := parseEntry(scanner.Bytes())

		// Accumulate assistant message deltas
		if entry.Type == "message" {
			pendingMessage.WriteString(entry.Content)
			continue
		}

		// Flush pending message when we hit a non-message entry
		if pendingMessage.Len() > 0 {
			entries = append(entries, TranscriptEntry{
				Type:    "message",
				Content: pendingMessage.String(),
			})
			pendingMessage.Reset()
		}

		if entry.Content != "" {
			entries = append(entries, entry)
		}
	}

	// Flush any remaining message
	if pendingMessage.Len() > 0 {
		entries = append(entries, TranscriptEntry{
			Type:    "message",
			Content: pendingMessage.String(),
		})
	}

	return entries, lineNum, scanner.Err()
}

func parseEntry(data []byte) TranscriptEntry {
	var raw struct {
		Type     string `json:"type"`
		Role     string `json:"role"`
		Content  string `json:"content"`
		ToolName string `json:"tool_name"`
		Output   string `json:"output"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return TranscriptEntry{}
	}

	switch raw.Type {
	case "message":
		if raw.Role == "user" {
			return TranscriptEntry{Type: "user", Content: raw.Content}
		}
		// Assistant message (may be delta)
		return TranscriptEntry{Type: "message", Content: raw.Content}

	case "tool_use":
		return TranscriptEntry{Type: "tool", Content: fmt.Sprintf("[tool: %s]", raw.ToolName)}

	case "tool_result":
		output := raw.Output
		// Truncate long outputs
		runes := []rune(output)
		if len(runes) > 200 {
			output = string(runes[:200]) + "..."
		}
		return TranscriptEntry{Type: "tool_output", Content: output}

	case "init", "result":
		// Skip these - init is metadata, result is just stats
		return TranscriptEntry{}
	}

	return TranscriptEntry{}
}

// FormatEntries formats transcript entries for display
func FormatEntries(entries []TranscriptEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case "user":
			sb.WriteString("[user] ")
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "message":
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "tool":
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		case "tool_output":
			sb.WriteString("  -> ")
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
```

**Step 4: Run tests**

Run: `go test ./internal/gemini/... -v`
Expected: PASS

**Step 5: Write test for delta accumulation**

Add to `transcript_test.go`:

```go
func TestReadTranscriptAccumulatesDeltas(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")

	content := `{"type":"init","session_id":"abc"}
{"type":"message","role":"user","content":"Hello"}
{"type":"message","role":"assistant","content":"Hi","delta":true}
{"type":"message","role":"assistant","content":" there","delta":true}
{"type":"message","role":"assistant","content":"!","delta":true}
{"type":"tool_use","tool_name":"read_file","tool_id":"t1"}
{"type":"result","status":"success"}
`
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, _, err := ReadTranscript(sessionFile, 0)
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	// Should have: user message, accumulated assistant message, tool
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].Type != "user" || entries[0].Content != "Hello" {
		t.Errorf("entries[0] = %+v, want user/Hello", entries[0])
	}

	if entries[1].Type != "message" || entries[1].Content != "Hi there!" {
		t.Errorf("entries[1] = %+v, want message/'Hi there!'", entries[1])
	}

	if entries[2].Type != "tool" {
		t.Errorf("entries[2].Type = %q, want tool", entries[2].Type)
	}
}
```

**Step 6: Run test**

Run: `go test ./internal/gemini/... -run TestReadTranscript -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/gemini/transcript.go internal/gemini/transcript_test.go
git commit -m "feat(gemini): add transcript.go with delta accumulation"
```

---

## Task 6: Add runSpawnGemini function

**Files:**
- Modify: `internal/cli/spawn.go`
- Modify: `internal/cli/spawn_test.go`

**Step 1: Add buildGeminiArgs function**

Add to `spawn.go`:

```go
// buildGeminiArgs constructs the argument slice for the gemini command.
func buildGeminiArgs(task, model string, yolo, sandbox bool) []string {
	args := []string{"-p", task, "--output-format", "stream-json"}

	if yolo {
		args = append(args, "--yolo")
	} else {
		args = append(args, "--approval-mode", "auto_edit")
	}

	if model != "" {
		args = append(args, "-m", model)
	}

	if sandbox {
		args = append(args, "--sandbox")
	}

	return args
}
```

**Step 2: Add test for buildGeminiArgs**

Add to `spawn_test.go`:

```go
func TestBuildGeminiArgs(t *testing.T) {
	tests := []struct {
		name    string
		task    string
		model   string
		yolo    bool
		sandbox bool
		want    []string
	}{
		{
			name:    "basic task with defaults",
			task:    "fix the bug",
			model:   "",
			yolo:    false,
			sandbox: false,
			want:    []string{"-p", "fix the bug", "--output-format", "stream-json", "--approval-mode", "auto_edit"},
		},
		{
			name:    "with yolo mode",
			task:    "refactor code",
			model:   "",
			yolo:    true,
			sandbox: false,
			want:    []string{"-p", "refactor code", "--output-format", "stream-json", "--yolo"},
		},
		{
			name:    "with model",
			task:    "write tests",
			model:   "gemini-2.5-pro",
			yolo:    false,
			sandbox: false,
			want:    []string{"-p", "write tests", "--output-format", "stream-json", "--approval-mode", "auto_edit", "-m", "gemini-2.5-pro"},
		},
		{
			name:    "with sandbox",
			task:    "dangerous task",
			model:   "",
			yolo:    true,
			sandbox: true,
			want:    []string{"-p", "dangerous task", "--output-format", "stream-json", "--yolo", "--sandbox"},
		},
		{
			name:    "all options",
			task:    "full task",
			model:   "gemini-2.5-flash",
			yolo:    true,
			sandbox: true,
			want:    []string{"-p", "full task", "--output-format", "stream-json", "--yolo", "-m", "gemini-2.5-flash", "--sandbox"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGeminiArgs(tt.task, tt.model, tt.yolo, tt.sandbox)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildGeminiArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 3: Run test to verify buildGeminiArgs**

Run: `go test ./internal/cli/... -run TestBuildGeminiArgs -v`
Expected: PASS

**Step 4: Add geminiInstalled helper**

Add to `spawn.go`:

```go
// geminiInstalled checks if the gemini CLI is available in PATH.
func geminiInstalled() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}
```

**Step 5: Add runSpawnGemini function with buffer sizing and gemini check**

Add to `spawn.go`:

```go
func runSpawnGemini(prefix, task string, model string, yolo, sandbox bool) error {
	// Check if gemini is installed
	if !geminiInstalled() {
		return fmt.Errorf("gemini CLI not found - install with: npm install -g @google/gemini-cli")
	}

	// Capture git context before spawning
	repoPath := scope.RepoRoot()
	branch := scope.BranchName()

	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Ensure gemini home exists
	if _, err := gemini.EnsureGeminiHome(); err != nil {
		return fmt.Errorf("failed to setup gemini home: %w", err)
	}

	// Build gemini command arguments
	args := buildGeminiArgs(task, model, yolo, sandbox)

	// Start gemini -p ...
	geminiCmd := exec.Command("gemini", args...)
	geminiCmd.Stderr = os.Stderr

	stdout, err := geminiCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := geminiCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gemini: %w", err)
	}

	// Read first line to get session_id
	// Use large buffer to handle long lines (tool outputs can be large)
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size

	var sessionID string
	var firstLine []byte
	if scanner.Scan() {
		firstLine = make([]byte, len(scanner.Bytes()))
		copy(firstLine, scanner.Bytes())

		var event struct {
			Type      string `json:"type"`
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(firstLine, &event); err == nil {
			if event.Type == "init" {
				sessionID = event.SessionID
			}
		}
	}

	if sessionID == "" {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to get session_id from gemini output")
	}

	// Get session file path and create it
	sessionFile, err := gemini.SessionFilePath(sessionID)
	if err != nil {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to get session file path: %w", err)
	}

	// Create session file and write first line
	f, err := os.Create(sessionFile)
	if err != nil {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to create session file: %w", err)
	}

	// Write the buffered first line
	f.Write(firstLine)
	f.Write([]byte("\n"))

	// Resolve agent name using session ID
	name, err := resolveAgentNameWithULID(database, prefix, sessionID)
	if err != nil {
		f.Close()
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to resolve agent name: %w", err)
	}

	// Create agent record
	agent := db.Agent{
		Name:        name,
		ULID:        sessionID,
		SessionFile: sessionFile,
		PID:         geminiCmd.Process.Pid,
		RepoPath:    repoPath,
		Branch:      branch,
		Type:        "gemini",
	}
	if err := database.CreateAgent(agent); err != nil {
		f.Close()
		return fmt.Errorf("failed to create agent record: %w", err)
	}

	// Stream remaining output to session file
	for scanner.Scan() {
		f.Write(scanner.Bytes())
		f.Write([]byte("\n"))
	}
	f.Close()

	// Wait for process to finish
	if err := geminiCmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "gemini exited with error: %v\n", err)
	}

	// Print the agent name
	fmt.Println(name)

	return nil
}
```

**Step 6: Add import for gemini package**

Add to imports in `spawn.go`:

```go
"github.com/sky-xo/june/internal/gemini"
```

**Step 7: Run build to verify**

Run: `go build ./...`
Expected: SUCCESS

**Step 8: Commit**

```bash
git add internal/cli/spawn.go internal/cli/spawn_test.go
git commit -m "feat(cli): add runSpawnGemini function with tests"
```

---

## Task 7: Update spawn command to dispatch gemini

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Update newSpawnCmd to support gemini with separate sandbox flags**

Replace the command setup with:

```go
func newSpawnCmd() *cobra.Command {
	var (
		name          string
		model         string
		yolo          bool
		geminiSandbox bool
		// Codex-specific
		codexSandbox    string
		reasoningEffort string
		maxTokens       int
	)

	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn an agent",
		Long:  "Spawn a Codex or Gemini agent to perform a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentType := args[0]
			task := args[1]

			switch agentType {
			case "codex":
				return runSpawnCodex(name, task, model, reasoningEffort, codexSandbox, maxTokens)
			case "gemini":
				return runSpawnGemini(name, task, model, yolo, geminiSandbox)
			default:
				return fmt.Errorf("unsupported agent type: %s (supported: codex, gemini)", agentType)
			}
		},
	}

	// Shared flags
	cmd.Flags().StringVar(&name, "name", "", "Name prefix for the agent (auto-generated if omitted)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use")

	// Codex-specific flags
	cmd.Flags().StringVar(&codexSandbox, "sandbox", "", "Sandbox mode (codex: read-only|workspace-write|danger-full-access)")
	cmd.Flags().StringVar(&reasoningEffort, "reasoning-effort", "", "Reasoning effort (codex only)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max output tokens (codex only)")

	// Gemini-specific flags
	cmd.Flags().BoolVar(&yolo, "yolo", false, "Auto-approve all actions (gemini only, default is auto_edit)")
	cmd.Flags().BoolVar(&geminiSandbox, "gemini-sandbox", false, "Run in Docker sandbox (gemini only)")

	return cmd
}
```

**Step 2: Run build**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): dispatch spawn to codex or gemini with proper flags"
```

---

## Task 8: Update peek.go for multi-agent support

**Files:**
- Modify: `internal/cli/peek.go`

**Step 1: Update runPeek to dispatch based on type**

Replace `runPeek` function:

```go
func runPeek(name string) error {
	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
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
		var findErr error
		if agent.Type == "gemini" {
			sessionFile, findErr = gemini.FindSessionFile(agent.ULID)
		} else {
			sessionFile, findErr = codex.FindSessionFile(agent.ULID)
		}
		if findErr != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
		if err := database.UpdateSessionFile(name, sessionFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update session file in database: %v\n", err)
		}
	}

	// Read transcript based on agent type
	var output string
	var newCursor int

	if agent.Type == "gemini" {
		entries, cursor, err := gemini.ReadTranscript(sessionFile, agent.Cursor)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		newCursor = cursor
		output = gemini.FormatEntries(entries)
	} else {
		entries, cursor, err := codex.ReadTranscript(sessionFile, agent.Cursor)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		newCursor = cursor
		output = codex.FormatEntries(entries)
	}

	if output == "" {
		fmt.Println("(no new output)")
		return nil
	}

	// Update cursor
	if err := database.UpdateCursor(name, newCursor); err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	fmt.Print(output)
	return nil
}
```

**Step 2: Add gemini import**

Add to imports:

```go
"github.com/sky-xo/june/internal/gemini"
```

**Step 3: Run build**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/cli/peek.go
git commit -m "feat(cli): update peek to support gemini agents"
```

---

## Task 9: Update logs.go for multi-agent support

**Files:**
- Modify: `internal/cli/logs.go`

**Step 1: Update runLogs to dispatch based on type**

Replace `runLogs` function:

```go
func runLogs(name string) error {
	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
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
		var findErr error
		if agent.Type == "gemini" {
			sessionFile, findErr = gemini.FindSessionFile(agent.ULID)
		} else {
			sessionFile, findErr = codex.FindSessionFile(agent.ULID)
		}
		if findErr != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
	}

	// Read transcript based on agent type
	var output string

	if agent.Type == "gemini" {
		entries, _, err := gemini.ReadTranscript(sessionFile, 0)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		output = gemini.FormatEntries(entries)
	} else {
		entries, _, err := codex.ReadTranscript(sessionFile, 0)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		output = codex.FormatEntries(entries)
	}

	if output == "" {
		fmt.Println("(no output)")
		return nil
	}

	fmt.Print(output)
	return nil
}
```

**Step 2: Add gemini import**

Add to imports:

```go
"github.com/sky-xo/june/internal/gemini"
```

**Step 3: Run build**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/cli/logs.go
git commit -m "feat(cli): update logs to support gemini agents"
```

---

## Task 10: Add TUI support for Gemini agents

**Files:**
- Modify: `internal/tui/commands.go`

**Step 1: Add gemini import**

Add to imports in `commands.go`:

```go
"github.com/sky-xo/june/internal/gemini"
```

**Step 2: Update loadTranscriptCmd to handle Gemini**

Update the switch statement in `loadTranscriptCmd`:

```go
// loadTranscriptCmd loads a transcript from a file.
func loadTranscriptCmd(a agent.Agent) tea.Cmd {
	return func() tea.Msg {
		var entries []claude.Entry
		var err error

		switch a.Source {
		case agent.SourceGemini:
			// Parse Gemini format and convert to claude.Entry for display
			var geminiEntries []gemini.TranscriptEntry
			geminiEntries, _, err = gemini.ReadTranscript(a.TranscriptPath, 0)
			if err != nil {
				return errMsg(err)
			}
			entries = convertGeminiEntries(geminiEntries)
		case agent.SourceCodex:
			// Parse Codex format and convert to claude.Entry for display
			var codexEntries []codex.TranscriptEntry
			codexEntries, _, err = codex.ReadTranscript(a.TranscriptPath, 0)
			if err != nil {
				return errMsg(err)
			}
			entries = convertCodexEntries(codexEntries)
		default:
			// Default to Claude format
			entries, err = claude.ParseTranscript(a.TranscriptPath)
			if err != nil {
				return errMsg(err)
			}
		}

		return transcriptMsg{
			agentID: a.ID,
			entries: entries,
		}
	}
}
```

**Step 3: Add convertGeminiEntries function**

Add after `convertCodexEntries`:

```go
// convertGeminiEntries converts Gemini transcript entries to Claude entry format for TUI display.
func convertGeminiEntries(geminiEntries []gemini.TranscriptEntry) []claude.Entry {
	entries := make([]claude.Entry, 0, len(geminiEntries))
	for _, ge := range geminiEntries {
		var entry claude.Entry
		switch ge.Type {
		case "user":
			// Gemini user message -> Claude user
			entry = claude.Entry{
				Type: "user",
				Message: claude.Message{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content,
						},
					},
				},
			}
		case "message":
			// Gemini assistant message -> Claude assistant
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content,
						},
					},
				},
			}
		case "tool":
			// Gemini tool call -> Claude assistant with tool info
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content, // Already formatted as "[tool: name]"
						},
					},
				},
			}
		case "tool_output":
			// Gemini tool output -> Claude user with tool_result
			entry = claude.Entry{
				Type: "user",
				Message: claude.Message{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "tool_result",
							"text": "  -> " + ge.Content,
						},
					},
				},
			}
		default:
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}
```

**Step 4: Run build**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/tui/commands.go
git commit -m "feat(tui): add support for Gemini agent transcripts"
```

---

## Task 11: Run full test suite and verify

**Step 1: Run all tests**

Run: `make test`
Expected: All tests PASS

**Step 2: Build binary**

Run: `make build`
Expected: SUCCESS

**Step 3: Manual smoke test (if gemini CLI installed)**

```bash
./june spawn gemini "Say hello" --name test-gemini
./june peek test-gemini
./june logs test-gemini
./june  # Check TUI shows Gemini agent
```

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address test failures"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add SourceGemini constant | agent/agent.go |
| 2 | Add type column to database | db/db.go, db_test.go |
| 3 | Create gemini home.go | gemini/home.go, home_test.go |
| 4 | Create gemini sessions.go | gemini/sessions.go, sessions_test.go |
| 5 | Create gemini transcript.go | gemini/transcript.go, transcript_test.go |
| 6 | Add runSpawnGemini function | cli/spawn.go, spawn_test.go |
| 7 | Update spawn command dispatch | cli/spawn.go |
| 8 | Update peek for multi-agent | cli/peek.go |
| 9 | Update logs for multi-agent | cli/logs.go |
| 10 | Add TUI support for Gemini | tui/commands.go |
| 11 | Full test suite verification | - |

## Fresh Eyes Review Fixes Applied

This plan was updated based on a fresh eyes review that identified:

1. **TUI Integration Missing** - Added Task 10 to update `internal/tui/commands.go`
2. **Sandbox flag mismatch** - Task 7 now uses `--gemini-sandbox` (bool) separate from `--sandbox` (string for Codex)
3. **Scanner buffer limit** - Task 6 now sets 1MB buffer limit to prevent truncation
4. **Missing tests** - Task 6 now includes `TestBuildGeminiArgs`
5. **Missing "gemini not installed" error** - Task 6 now includes `geminiInstalled()` check

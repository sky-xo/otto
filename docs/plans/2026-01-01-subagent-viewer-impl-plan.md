# Subagent Viewer MVP Implementation Plan

**Status:** ✅ MVP COMPLETE (2026-01-01)

> **For Claude:** Use superpowers:subagent-driven-development to implement this plan with parallel subagents where possible.

**Goal:** Build a read-only TUI that displays Claude Code subagent sessions from `agent-*.jsonl` files.

**Design Doc:** See `docs/plans/2026-01-01-subagent-viewer-mvp.md` for full context on what we're building and why.

**Architecture:** New `internal/claude/` package reads Claude Code's project files. Existing TUI is gutted to remove SQLite/messaging/spawning, then rewired to use the new file-based data source.

**Tech Stack:** Go, Bubbletea TUI, file watching via polling

**Module:** `june` (go.mod)

## Implementation Summary

All phases complete. The TUI now:
- ✅ Displays subagents from `agent-*.jsonl` files
- ✅ Two-panel layout with borders (sidebar + transcript)
- ✅ Mouse click to select agents
- ✅ Keyboard navigation (j/k, Tab to switch panels)
- ✅ Scroll wheel support
- ✅ Smart timestamp in header (Today/Yesterday/Weekday/Date)
- ✅ Full row highlight on selected agent
- ✅ Auto-refresh without scroll position reset
- ✅ Active indicator (green dot) for recently modified agents

**Key files created:**
- `internal/claude/projects.go` - Path mapping
- `internal/claude/agents.go` - Agent file scanning
- `internal/claude/transcript.go` - JSONL parsing
- `internal/tui/model.go` - Bubbletea TUI
- `internal/tui/commands.go` - TUI commands
- `internal/tui/model_test.go` - Tests

---

## Phase 0: Clean Slate ✅

Delete old SQLite-based code before building new file-based viewer. This prevents type conflicts and keeps the build clean.

### Task 0.1: Delete old TUI, repo, db, and unused commands

**Files to delete:**
- `internal/tui/` (entire directory - 5200+ lines)
- `internal/repo/` (entire directory - database operations)
- `internal/db/` (entire directory - SQLite schema)
- `internal/process/` (if exists - process management for spawning)
- Most of `internal/cli/commands/` except `watch.go` and `common.go`

**Files to keep:**
- `internal/cli/root.go` (will simplify)
- `internal/cli/commands/watch.go` (will rewrite)
- `internal/cli/commands/common.go` (shared utilities)
- `internal/scope/` (git detection - we need this)
- `internal/exec/` (if basic exec utilities needed)

**Steps:**
1. `git rm -r internal/tui/`
2. `git rm -r internal/repo/`
3. `git rm -r internal/db/`
4. `git rm` unused commands (spawn, dm, ask, complete, messages, status, etc.)
5. Simplify `internal/cli/root.go` to only register watch command
6. Stub out `internal/cli/commands/watch.go` to just print "TODO"
7. Verify `go build ./...` passes
8. Commit: "chore: remove SQLite-based orchestration code for clean slate"

**Preserving patterns:** The old `renderPanelWithTitle` function is useful - we'll recreate it in the new TUI.

---

## Phase 1: Create `internal/claude/` Package

Core file parsing logic, independent of TUI.

### Task 1.1: Create projects.go with path mapping

**Files:**
- Create: `internal/claude/projects.go`
- Create: `internal/claude/projects_test.go`

**Step 1: Write the failing test**

```go
// internal/claude/projects_test.go
package claude

import (
	"testing"
)

func TestPathToProjectDir(t *testing.T) {
	tests := []struct {
		absPath  string
		expected string
	}{
		{"/Users/glowy/code/otto", "-Users-glowy-code-otto"},
		{"/home/user/project", "-home-user-project"},
	}

	for _, tt := range tests {
		got := PathToProjectDir(tt.absPath)
		if got != tt.expected {
			t.Errorf("PathToProjectDir(%q) = %q, want %q", tt.absPath, got, tt.expected)
		}
	}
}

func TestClaudeProjectsDir(t *testing.T) {
	dir := ClaudeProjectsDir()
	if dir == "" {
		t.Error("ClaudeProjectsDir() returned empty string")
	}
	// Should end with .claude/projects
	if !strings.HasSuffix(dir, ".claude/projects") {
		t.Errorf("ClaudeProjectsDir() = %q, should end with .claude/projects", dir)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/... -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write minimal implementation**

```go
// internal/claude/projects.go
package claude

import (
	"os"
	"path/filepath"
	"strings"
)

// ClaudeProjectsDir returns ~/.claude/projects
func ClaudeProjectsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// PathToProjectDir converts an absolute path to Claude's directory format.
// Example: /Users/glowy/code/otto -> -Users-glowy-code-otto
func PathToProjectDir(absPath string) string {
	return strings.ReplaceAll(absPath, "/", "-")
}

// ProjectDir returns the full path to a project's Claude directory.
func ProjectDir(absPath string) string {
	return filepath.Join(ClaudeProjectsDir(), PathToProjectDir(absPath))
}
```

**Step 4: Add missing import to test**

```go
// Add to imports in projects_test.go
import (
	"strings"
	"testing"
)
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/claude/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/claude/
git commit -m "feat(claude): add project path mapping"
```

---

### Task 1.2: Create agents.go with file scanning

**Files:**
- Create: `internal/claude/agents.go`
- Create: `internal/claude/agents_test.go`

**Step 1: Write the failing test**

```go
// internal/claude/agents_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAgents(t *testing.T) {
	// Create temp directory with fake agent files
	dir := t.TempDir()

	// Create agent-abc123.jsonl
	f1, _ := os.Create(filepath.Join(dir, "agent-abc123.jsonl"))
	f1.WriteString(`{"type":"user","message":{"content":"test"}}`)
	f1.Close()

	// Create agent-def456.jsonl
	f2, _ := os.Create(filepath.Join(dir, "agent-def456.jsonl"))
	f2.WriteString(`{"type":"user","message":{"content":"test2"}}`)
	f2.Close()

	// Create non-agent file (should be ignored)
	f3, _ := os.Create(filepath.Join(dir, "session-xyz.jsonl"))
	f3.WriteString(`{}`)
	f3.Close()

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("got %d agents, want 2", len(agents))
	}

	// Check IDs were extracted correctly
	ids := make(map[string]bool)
	for _, a := range agents {
		ids[a.ID] = true
	}
	if !ids["abc123"] || !ids["def456"] {
		t.Errorf("expected agents abc123 and def456, got %v", ids)
	}
}

func TestAgentIsActive(t *testing.T) {
	agent := Agent{
		LastMod: time.Now().Add(-5 * time.Second),
	}
	if !agent.IsActive() {
		t.Error("agent modified 5s ago should be active")
	}

	agent.LastMod = time.Now().Add(-30 * time.Second)
	if agent.IsActive() {
		t.Error("agent modified 30s ago should not be active")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/... -v`
Expected: FAIL (Agent type and ScanAgents don't exist)

**Step 3: Write minimal implementation**

```go
// internal/claude/agents.go
package claude

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const activeThreshold = 10 * time.Second

// Agent represents a Claude Code subagent session.
type Agent struct {
	ID       string    // Extracted from filename: agent-{id}.jsonl
	FilePath string    // Full path to jsonl file
	LastMod  time.Time // File modification time
}

// IsActive returns true if the agent was modified within the active threshold.
func (a Agent) IsActive() bool {
	return time.Since(a.LastMod) < activeThreshold
}

// ScanAgents finds all agent-*.jsonl files in a directory.
// Returns agents sorted by LastMod descending (most recent first).
func ScanAgents(dir string) ([]Agent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No agents yet
		}
		return nil, err
	}

	var agents []Agent
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Extract ID: agent-abc123.jsonl -> abc123
		id := strings.TrimPrefix(name, "agent-")
		id = strings.TrimSuffix(id, ".jsonl")

		info, err := e.Info()
		if err != nil {
			continue
		}

		agents = append(agents, Agent{
			ID:       id,
			FilePath: filepath.Join(dir, name),
			LastMod:  info.ModTime(),
		})
	}

	// Sort by LastMod descending
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].LastMod.After(agents[j].LastMod)
	})

	return agents, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/agents.go internal/claude/agents_test.go
git commit -m "feat(claude): add agent file scanning"
```

---

### Task 1.3: Create transcript.go with JSONL parsing

**Files:**
- Create: `internal/claude/transcript.go`
- Create: `internal/claude/transcript_test.go`

**Step 1: Write the failing test**

```go
// internal/claude/transcript_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTranscript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-test.jsonl")

	content := `{"type":"user","message":{"role":"user","content":"Hello"},"timestamp":"2026-01-01T12:00:00Z","agentId":"test"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi there"}]},"timestamp":"2026-01-01T12:00:01Z","agentId":"test"}`

	os.WriteFile(path, []byte(content), 0644)

	entries, err := ParseTranscript(path)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	if entries[0].Type != "user" {
		t.Errorf("entries[0].Type = %q, want user", entries[0].Type)
	}
	if entries[1].Type != "assistant" {
		t.Errorf("entries[1].Type = %q, want assistant", entries[1].Type)
	}
}

func TestEntryContent(t *testing.T) {
	// User message with string content
	e1 := Entry{
		Type: "user",
		Message: Message{
			Role:    "user",
			Content: "Hello world",
		},
	}
	if got := e1.TextContent(); got != "Hello world" {
		t.Errorf("TextContent() = %q, want %q", got, "Hello world")
	}

	// Assistant message with content blocks
	e2 := Entry{
		Type: "assistant",
		Message: Message{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Response here"},
			},
		},
	}
	if got := e2.TextContent(); got != "Response here" {
		t.Errorf("TextContent() = %q, want %q", got, "Response here")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/... -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// internal/claude/transcript.go
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// Entry represents a single line in an agent transcript.
type Entry struct {
	Type      string    `json:"type"` // "user" or "assistant"
	Message   Message   `json:"message"`
	AgentID   string    `json:"agentId"`
	Timestamp time.Time `json:"timestamp"`
}

// Message represents the message content.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentBlock
	StopReason *string     `json:"stop_reason"`
}

// ContentBlock represents a content block in assistant messages.
type ContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Name  string `json:"name,omitempty"`  // for tool_use
	Input any    `json:"input,omitempty"` // for tool_use
}

// TextContent extracts the text content from an entry.
func (e Entry) TextContent() string {
	switch c := e.Message.Content.(type) {
	case string:
		return c
	case []interface{}:
		for _, block := range c {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}

// ToolName returns the tool name if this is a tool_use entry.
func (e Entry) ToolName() string {
	if blocks, ok := e.Message.Content.([]interface{}); ok {
		for _, block := range blocks {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "tool_use" {
					if name, ok := m["name"].(string); ok {
						return name
					}
				}
			}
		}
	}
	return ""
}

// IsToolResult returns true if this is a tool_result entry.
func (e Entry) IsToolResult() bool {
	if blocks, ok := e.Message.Content.([]interface{}); ok {
		for _, block := range blocks {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "tool_result" {
					return true
				}
			}
		}
	}
	return false
}

// ParseTranscript reads a JSONL file and returns all entries.
func ParseTranscript(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)

	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line

	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // Skip malformed lines
		}
		entries = append(entries, e)
	}

	return entries, scanner.Err()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/transcript.go internal/claude/transcript_test.go
git commit -m "feat(claude): add transcript JSONL parsing"
```

---

## Phase 2: Simplify the TUI

Remove unused features, prepare for new data source.

### Task 2.1: Create a new simplified model

**Files:**
- Create: `internal/tui/model.go`

**Step 1: Extract and simplify the model**

Create a new model that doesn't depend on SQLite:

```go
// internal/tui/model.go
package tui

import (
	"june/internal/claude"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the TUI state.
type Model struct {
	projectDir  string          // Claude project directory we're watching
	agents      []claude.Agent  // List of agents
	transcripts map[string][]claude.Entry // Agent ID -> transcript entries

	selectedIdx int             // Currently selected agent index
	width       int
	height      int
	viewport    viewport.Model
	err         error
}

// NewModel creates a new TUI model.
func NewModel(projectDir string) Model {
	return Model{
		projectDir:  projectDir,
		agents:      []claude.Agent{},
		transcripts: make(map[string][]claude.Entry),
		viewport:    viewport.New(0, 0),
	}
}

// SelectedAgent returns the currently selected agent, or nil if none.
func (m Model) SelectedAgent() *claude.Agent {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.agents) {
		return nil
	}
	return &m.agents[m.selectedIdx]
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "refactor(tui): create simplified model without SQLite"
```

---

### Task 2.2: Create new fetch commands

**Files:**
- Create: `internal/tui/commands.go`

**Step 1: Write the fetch commands**

```go
// internal/tui/commands.go
package tui

import (
	"time"

	"june/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

// Messages for the TUI
type (
	tickMsg       time.Time
	agentsMsg     []claude.Agent
	transcriptMsg struct {
		agentID string
		entries []claude.Entry
	}
	errMsg error
)

// tickCmd returns a command that ticks every second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// scanAgentsCmd scans for agent files.
func scanAgentsCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		agents, err := claude.ScanAgents(dir)
		if err != nil {
			return errMsg(err)
		}
		return agentsMsg(agents)
	}
}

// loadTranscriptCmd loads a transcript from a file.
func loadTranscriptCmd(agent claude.Agent) tea.Cmd {
	return func() tea.Msg {
		entries, err := claude.ParseTranscript(agent.FilePath)
		if err != nil {
			return errMsg(err)
		}
		return transcriptMsg{
			agentID: agent.ID,
			entries: entries,
		}
	}
}
```

**Step 2: Commit**

```bash
git add internal/tui/commands.go
git commit -m "feat(tui): add file-based fetch commands"
```

---

### Task 2.3: Implement Init and Update

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add Init and Update methods**

```go
// Add to internal/tui/model.go

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		scanAgentsCmd(m.projectDir),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				if agent := m.SelectedAgent(); agent != nil {
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
			}
		case "down", "j":
			if m.selectedIdx < len(m.agents)-1 {
				m.selectedIdx++
				if agent := m.SelectedAgent(); agent != nil {
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - sidebarWidth - 4
		m.viewport.Height = msg.Height - 4

	case tickMsg:
		cmds = append(cmds, tickCmd(), scanAgentsCmd(m.projectDir))

	case agentsMsg:
		m.agents = msg
		// Load transcript for selected agent
		if agent := m.SelectedAgent(); agent != nil {
			cmds = append(cmds, loadTranscriptCmd(*agent))
		}

	case transcriptMsg:
		m.transcripts[msg.agentID] = msg.entries
		m.updateViewport()

	case errMsg:
		m.err = msg
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	agent := m.SelectedAgent()
	if agent == nil {
		m.viewport.SetContent("")
		return
	}
	entries := m.transcripts[agent.ID]
	m.viewport.SetContent(formatTranscript(entries))
}

const sidebarWidth = 20
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): implement Init and Update for file-based model"
```

---

### Task 2.4: Implement View

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add View method and helpers**

```go
// Add to internal/tui/model.go

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	doneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")) // cyan
)

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	// Left panel: agent list
	sidebar := m.renderSidebar()

	// Right panel: transcript
	content := m.viewport.View()

	// Combine
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", content)
}

func (m Model) renderSidebar() string {
	var lines []string
	lines = append(lines, "Subagents")
	lines = append(lines, strings.Repeat("─", sidebarWidth-2))

	for i, agent := range m.agents {
		var indicator string
		if agent.IsActive() {
			indicator = activeStyle.Render("●")
		} else {
			indicator = doneStyle.Render("✓")
		}

		name := agent.ID
		if len(name) > sidebarWidth-4 {
			name = name[:sidebarWidth-4]
		}

		line := fmt.Sprintf(" %s %s", indicator, name)
		if i == m.selectedIdx {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	// Pad to full height
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func formatTranscript(entries []claude.Entry) string {
	var lines []string
	for _, e := range entries {
		switch e.Type {
		case "user":
			content := e.TextContent()
			if content != "" {
				lines = append(lines, fmt.Sprintf("> %s", content))
				lines = append(lines, "")
			}
		case "assistant":
			if tool := e.ToolName(); tool != "" {
				lines = append(lines, fmt.Sprintf("  [%s]", tool))
			} else if text := e.TextContent(); text != "" {
				lines = append(lines, text)
				lines = append(lines, "")
			}
		}
	}
	return strings.Join(lines, "\n")
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): implement View with sidebar and transcript"
```

---

## Phase 3: Wire Up and Test

### Task 3.1: Update watch command entry point

**Files:**
- Modify: `internal/cli/commands/watch.go`

**Step 1: Simplify watch.go**

Replace the content with a simpler version:

```go
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"june/internal/claude"
	"june/internal/scope"
	"june/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch subagent activity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunWatch()
		},
	}
}

// RunWatch starts the TUI.
func RunWatch() error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("watch requires a terminal")
	}

	// Get current git project root
	repoRoot := scope.RepoRoot()
	if repoRoot == "" {
		return fmt.Errorf("not in a git repository")
	}

	// Find Claude project directory
	absPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	projectDir := claude.ProjectDir(absPath)

	// Check if directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("no Claude Code sessions found for this project\n\nExpected: %s", projectDir)
	}

	// Run TUI
	model := tui.NewModel(projectDir)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
```

**Step 2: Commit**

```bash
git add internal/cli/commands/watch.go
git commit -m "refactor(watch): simplify to use file-based TUI"
```

---

### Task 3.2: Update root command

**Files:**
- Modify: `internal/cli/root.go`

**Step 1: Simplify root.go to only include watch**

Keep only the essential commands for MVP:

```go
// In internal/cli/root.go, update the command list to remove DB-dependent commands
// Keep: watch (and version if exists)
// Remove: spawn, complete, ask, dm, messages, status, etc.
```

For now, just ensure `june` (no args) runs watch:

```go
// Update the root command's RunE to call watch
RunE: func(cmd *cobra.Command, args []string) error {
    return commands.RunWatch()
},
```

**Step 2: Commit**

```bash
git add internal/cli/root.go
git commit -m "refactor(cli): simplify root to just watch for MVP"
```

---

### Task 3.3: Test with real agent files

**Step 1: Build and run**

```bash
make build
./june
```

**Step 2: Verify behavior**

- [ ] Agent list shows in sidebar
- [ ] Most recent agent at top
- [ ] Active indicator (●) for recently modified
- [ ] Done indicator (✓) for older files
- [ ] Arrow keys navigate
- [ ] Transcript shows in right panel
- [ ] Auto-refreshes (new content appears)

**Step 3: Fix any issues found**

**Step 4: Commit**

```bash
git add .
git commit -m "test: verify TUI works with real agent files"
```

---

## Phase 4: Polish

### Task 4.1: Improve transcript formatting

**Files:**
- Modify: `internal/tui/model.go` (formatTranscript function)

Enhance to show:
- Tool calls with better formatting
- Truncate long outputs
- Color coding

**Step 1: Update formatTranscript**

```go
func formatTranscript(entries []claude.Entry) string {
	var lines []string

	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsToolResult() {
				// Skip tool results in display (too verbose)
				continue
			}
			content := e.TextContent()
			if content != "" {
				// Show first 200 chars of prompt
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				lines = append(lines, promptStyle.Render("> "+content))
				lines = append(lines, "")
			}
		case "assistant":
			if tool := e.ToolName(); tool != "" {
				lines = append(lines, toolStyle.Render("  "+tool))
			} else if text := e.TextContent(); text != "" {
				// Show first 500 chars of response
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				lines = append(lines, text)
				lines = append(lines, "")
			}
		}
	}
	return strings.Join(lines, "\n")
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "polish(tui): improve transcript formatting"
```

---

### Task 4.2: Add panel borders

Use the existing `renderPanelWithTitle` helper or similar to add nice borders.

**Step 1: Update View to use bordered panels**

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "polish(tui): add panel borders"
```

---

### Task 4.3: Clean up old code

**Files:**
- Delete or archive: Old TUI code in `internal/tui/watch.go`
- Delete: Unused CLI commands
- Delete: `internal/repo/` (no longer needed)
- Delete: `internal/db/` (no longer needed)

**Step 1: Remove old files**

```bash
# Move old TUI to backup (or just delete if confident)
git rm internal/tui/watch.go
git rm internal/tui/formatting.go
git rm -r internal/repo/
git rm -r internal/db/
# Remove unused commands
git rm internal/cli/commands/spawn.go
git rm internal/cli/commands/complete.go
# ... etc
```

**Step 2: Update imports and fix compilation**

**Step 3: Run tests**

```bash
go test ./...
```

**Step 4: Commit**

```bash
git add .
git commit -m "chore: remove unused SQLite and orchestration code"
```

---

## Summary

| Phase | Tasks | Purpose |
|-------|-------|---------|
| 1 | 1.1-1.3 | Create `internal/claude/` package |
| 2 | 2.1-2.4 | Simplify TUI model |
| 3 | 3.1-3.3 | Wire up and test |
| 4 | 4.1-4.3 | Polish and cleanup |

**Estimated tasks:** 10 tasks, each 5-15 minutes

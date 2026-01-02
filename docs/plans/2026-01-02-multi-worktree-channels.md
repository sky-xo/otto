# Multi-Worktree Channels Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show subagents from all worktrees/branches in the sidebar, grouped under headers like `june:main`, `june:channels`, `june:select-mode`

**Architecture:** Add a `Channel` struct to represent a branch grouping. Modify `ScanAgents` to scan multiple Claude project directories. Update the TUI sidebar to render collapsible channel headers with agents nested underneath.

**Tech Stack:** Go, Bubbletea TUI framework, existing lipgloss styling

---

## Task 1: Add Channel struct and ScanAllChannels function

**Files:**
- Create: `internal/claude/channels.go`
- Test: `internal/claude/channels_test.go`

**Step 1: Write the failing test**

```go
// internal/claude/channels_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindRelatedProjectDirs(t *testing.T) {
	// Create temp Claude projects directory structure
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")
	os.MkdirAll(claudeProjects, 0755)

	// Create project dirs
	dirs := []string{
		"-Users-test-code-myproject",
		"-Users-test-code-myproject--worktrees-feature1",
		"-Users-test-code-myproject--worktrees-feature2",
		"-Users-test-code-other", // unrelated
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(claudeProjects, d), 0755)
	}

	// Test finding related dirs
	basePath := "/Users/test/code/myproject"
	related := FindRelatedProjectDirs(claudeProjects, basePath)

	if len(related) != 3 {
		t.Errorf("expected 3 related dirs, got %d: %v", len(related), related)
	}
}

func TestExtractChannelName(t *testing.T) {
	tests := []struct {
		baseDir    string
		projectDir string
		repoName   string
		want       string
	}{
		{
			baseDir:    "-Users-test-code-myproject",
			projectDir: "-Users-test-code-myproject",
			repoName:   "myproject",
			want:       "myproject:main",
		},
		{
			baseDir:    "-Users-test-code-myproject",
			projectDir: "-Users-test-code-myproject--worktrees-feature1",
			repoName:   "myproject",
			want:       "myproject:feature1",
		},
		{
			baseDir:    "-Users-test-code-june",
			projectDir: "-Users-test-code-june--worktrees--worktrees-channels",
			repoName:   "june",
			want:       "june:channels",
		},
	}

	for _, tt := range tests {
		got := ExtractChannelName(tt.baseDir, tt.projectDir, tt.repoName)
		if got != tt.want {
			t.Errorf("ExtractChannelName(%q, %q, %q) = %q, want %q",
				tt.baseDir, tt.projectDir, tt.repoName, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude -run TestFindRelatedProjectDirs -v`
Expected: FAIL with "undefined: FindRelatedProjectDirs"

**Step 3: Write minimal implementation**

```go
// internal/claude/channels.go
package claude

import (
	"os"
	"path/filepath"
	"strings"
)

// Channel represents a group of agents from a branch/worktree.
type Channel struct {
	Name   string  // Display name like "june:main" or "june:channels"
	Dir    string  // Full path to Claude project directory
	Agents []Agent // Agents in this channel
}

// FindRelatedProjectDirs finds all Claude project directories that share
// the same base project path (main repo + worktrees).
func FindRelatedProjectDirs(claudeProjectsDir, basePath string) []string {
	// Convert base path to Claude's dash format
	basePrefix := strings.ReplaceAll(basePath, "/", "-")

	entries, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		return nil
	}

	var related []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Match exact base or base with worktree suffix
		if name == basePrefix || strings.HasPrefix(name, basePrefix+"-") {
			related = append(related, filepath.Join(claudeProjectsDir, name))
		}
	}
	return related
}

// ExtractChannelName creates a display name like "june:main" or "june:feature".
// baseDir is the main repo's Claude dir name (e.g., "-Users-test-code-june")
// projectDir is the current dir name (e.g., "-Users-test-code-june--worktrees-channels")
// repoName is the repository name (e.g., "june")
func ExtractChannelName(baseDir, projectDir, repoName string) string {
	if projectDir == baseDir {
		return repoName + ":main"
	}

	// Extract worktree name from suffix
	suffix := strings.TrimPrefix(projectDir, baseDir)
	// Remove leading dashes and "worktrees" segments
	suffix = strings.TrimLeft(suffix, "-")

	// Handle nested worktrees like "--worktrees--worktrees-channels"
	// Split by "-" and find the last meaningful segment
	parts := strings.Split(suffix, "-")

	// Filter out "worktrees" and empty parts, keep last segment
	var lastPart string
	for _, p := range parts {
		if p != "" && p != "worktrees" {
			lastPart = p
		}
	}

	if lastPart == "" {
		return repoName + ":unknown"
	}
	return repoName + ":" + lastPart
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude -run "TestFindRelatedProjectDirs|TestExtractChannelName" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/channels.go internal/claude/channels_test.go
git commit -m "feat: add Channel struct and helper functions for multi-worktree support"
```

---

## Task 2: Add ScanChannels function

**Files:**
- Modify: `internal/claude/channels.go`
- Test: `internal/claude/channels_test.go`

**Step 1: Write the failing test**

Add to `internal/claude/channels_test.go`:

```go
func TestScanChannels(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")

	// Create two project dirs with agent files
	mainDir := filepath.Join(claudeProjects, "-Users-test-code-myproject")
	worktreeDir := filepath.Join(claudeProjects, "-Users-test-code-myproject--worktrees-feature1")
	os.MkdirAll(mainDir, 0755)
	os.MkdirAll(worktreeDir, 0755)

	// Create agent files
	os.WriteFile(filepath.Join(mainDir, "agent-abc123.jsonl"), []byte(`{"type":"user","message":{"role":"user","content":"Main task"}}`+"\n"), 0644)
	os.WriteFile(filepath.Join(worktreeDir, "agent-def456.jsonl"), []byte(`{"type":"user","message":{"role":"user","content":"Feature task"}}`+"\n"), 0644)

	// Touch the feature file to make it more recent
	futureTime := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(worktreeDir, "agent-def456.jsonl"), futureTime, futureTime)

	channels, err := ScanChannels(claudeProjects, "/Users/test/code/myproject", "myproject")
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	// Check channel names (sorted by most recent agent)
	if channels[0].Name != "myproject:feature1" {
		t.Errorf("expected first channel to be myproject:feature1, got %s", channels[0].Name)
	}
	if channels[1].Name != "myproject:main" {
		t.Errorf("expected second channel to be myproject:main, got %s", channels[1].Name)
	}

	// Check agents are present
	if len(channels[0].Agents) != 1 || channels[0].Agents[0].ID != "def456" {
		t.Errorf("unexpected agents in feature1 channel: %v", channels[0].Agents)
	}
	if len(channels[1].Agents) != 1 || channels[1].Agents[0].ID != "abc123" {
		t.Errorf("unexpected agents in main channel: %v", channels[1].Agents)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude -run TestScanChannels -v`
Expected: FAIL with "undefined: ScanChannels"

**Step 3: Write minimal implementation**

Add to `internal/claude/channels.go`:

```go
import (
	"sort"
)

// ScanChannels scans all related project directories and returns channels
// sorted by most recent agent activity (channels with active/recent agents first).
func ScanChannels(claudeProjectsDir, basePath, repoName string) ([]Channel, error) {
	relatedDirs := FindRelatedProjectDirs(claudeProjectsDir, basePath)
	if len(relatedDirs) == 0 {
		return nil, nil
	}

	baseDir := strings.ReplaceAll(basePath, "/", "-")
	var channels []Channel

	for _, dir := range relatedDirs {
		dirName := filepath.Base(dir)
		channelName := ExtractChannelName(baseDir, dirName, repoName)

		agents, err := ScanAgents(dir)
		if err != nil {
			continue // Skip dirs we can't read
		}

		if len(agents) == 0 {
			continue // Skip empty channels
		}

		channels = append(channels, Channel{
			Name:   channelName,
			Dir:    dir,
			Agents: agents,
		})
	}

	// Sort channels by most recent agent (first agent in each channel is most recent due to ScanAgents sorting)
	sort.Slice(channels, func(i, j int) bool {
		// Channels with active agents come first
		iHasActive := len(channels[i].Agents) > 0 && channels[i].Agents[0].IsActive()
		jHasActive := len(channels[j].Agents) > 0 && channels[j].Agents[0].IsActive()
		if iHasActive != jHasActive {
			return iHasActive
		}

		// Then by most recent agent modification time
		var iTime, jTime time.Time
		if len(channels[i].Agents) > 0 {
			iTime = channels[i].Agents[0].LastMod
		}
		if len(channels[j].Agents) > 0 {
			jTime = channels[j].Agents[0].LastMod
		}
		return iTime.After(jTime)
	})

	return channels, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude -run TestScanChannels -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/channels.go internal/claude/channels_test.go
git commit -m "feat: add ScanChannels to aggregate agents from all worktrees"
```

---

## Task 3: Update TUI Model for channels

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/commands.go`

**Step 1: Update Model struct**

In `internal/tui/model.go`, update the Model struct:

```go
// Model is the TUI state.
type Model struct {
	claudeProjectsDir string                    // Base Claude projects directory (~/.claude/projects)
	basePath          string                    // Git repo base path
	repoName          string                    // Repository name (e.g., "june")
	channels          []claude.Channel          // Channels with their agents
	transcripts       map[string][]claude.Entry // Agent ID -> transcript entries

	selectedIdx        int  // Currently selected item index (across all channels + headers)
	sidebarOffset      int  // Scroll offset for the sidebar
	lastNavWasKeyboard bool
	focusedPanel       int
	width              int
	height             int
	viewport           viewport.Model
	err                error
}
```

**Step 2: Update NewModel**

```go
// NewModel creates a new TUI model.
func NewModel(claudeProjectsDir, basePath, repoName string) Model {
	return Model{
		claudeProjectsDir: claudeProjectsDir,
		basePath:          basePath,
		repoName:          repoName,
		channels:          []claude.Channel{},
		transcripts:       make(map[string][]claude.Entry),
		viewport:          viewport.New(0, 0),
	}
}
```

**Step 3: Update commands.go**

Replace `scanAgentsCmd` in `internal/tui/commands.go`:

```go
// Message types
type (
	tickMsg       time.Time
	channelsMsg   []claude.Channel
	transcriptMsg struct {
		agentID string
		entries []claude.Entry
	}
	errMsg error
)

// scanChannelsCmd scans for channels and their agents.
func scanChannelsCmd(claudeProjectsDir, basePath, repoName string) tea.Cmd {
	return func() tea.Msg {
		channels, err := claude.ScanChannels(claudeProjectsDir, basePath, repoName)
		if err != nil {
			return errMsg(err)
		}
		return channelsMsg(channels)
	}
}
```

**Step 4: Update Init and message handling in model.go**

```go
// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		scanChannelsCmd(m.claudeProjectsDir, m.basePath, m.repoName),
	)
}
```

Update the `Update` function to handle `channelsMsg` instead of `agentsMsg`.

**Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/commands.go
git commit -m "refactor: update TUI model to use channels instead of flat agents"
```

---

## Task 4: Add sidebar item abstraction

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add helper types and methods**

Add to `internal/tui/model.go`:

```go
// sidebarItem represents either a channel header or an agent in the sidebar.
type sidebarItem struct {
	isHeader     bool
	channelName  string       // Only set for headers
	channelIdx   int          // Index into m.channels
	agent        *claude.Agent // Only set for agents
	agentIdx     int          // Index within channel's agents slice
}

// sidebarItems returns a flat list of all items to display in the sidebar.
func (m Model) sidebarItems() []sidebarItem {
	var items []sidebarItem
	for ci, ch := range m.channels {
		// Add channel header
		items = append(items, sidebarItem{
			isHeader:    true,
			channelName: ch.Name,
			channelIdx:  ci,
		})
		// Add agents
		for ai := range ch.Agents {
			items = append(items, sidebarItem{
				isHeader:   false,
				channelIdx: ci,
				agent:      &m.channels[ci].Agents[ai],
				agentIdx:   ai,
			})
		}
	}
	return items
}

// SelectedAgent returns the currently selected agent, or nil if a header is selected.
func (m Model) SelectedAgent() *claude.Agent {
	items := m.sidebarItems()
	if m.selectedIdx < 0 || m.selectedIdx >= len(items) {
		return nil
	}
	item := items[m.selectedIdx]
	if item.isHeader {
		return nil
	}
	return item.agent
}

// totalSidebarItems returns the total count of items (headers + agents).
func (m Model) totalSidebarItems() int {
	count := 0
	for _, ch := range m.channels {
		count++ // header
		count += len(ch.Agents)
	}
	return count
}
```

**Step 2: Update navigation logic**

Update `up`/`down` key handling to work with the new item model, skipping headers when selecting.

**Step 3: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: add sidebar item abstraction for channel headers and agents"
```

---

## Task 5: Update sidebar rendering

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Update renderSidebarContent**

```go
func (m Model) renderSidebarContent(width, height int) string {
	items := m.sidebarItems()
	if len(items) == 0 || height <= 0 {
		return "No agents found"
	}

	// Calculate scroll indicators
	hiddenAbove := m.sidebarOffset
	totalItems := len(items)

	availableLines := height
	showTopIndicator := hiddenAbove > 0
	if showTopIndicator {
		availableLines--
	}

	visibleEnd := m.sidebarOffset + availableLines
	if visibleEnd > totalItems {
		visibleEnd = totalItems
	}

	hiddenBelow := totalItems - visibleEnd
	showBottomIndicator := hiddenBelow > 0
	if showBottomIndicator && availableLines > 0 {
		availableLines--
		visibleEnd = m.sidebarOffset + availableLines
		if visibleEnd > totalItems {
			visibleEnd = totalItems
		}
		hiddenBelow = totalItems - visibleEnd
	}

	var lines []string

	// Top scroll indicator
	if showTopIndicator {
		indicator := fmt.Sprintf("↑ %d more", hiddenAbove)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, doneStyle.Render(indicator))
	}

	// Header style
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "6"})

	// Render visible items
	for i := m.sidebarOffset; i < visibleEnd; i++ {
		item := items[i]

		if item.isHeader {
			// Render channel header
			header := item.channelName
			if len(header) > width {
				header = header[:width]
			}
			if i == m.selectedIdx {
				// Selected header
				selectedBg := lipgloss.AdaptiveColor{Light: "254", Dark: "8"}
				rest := header
				if len(rest) < width {
					rest = rest + strings.Repeat(" ", width-len(rest))
				}
				lines = append(lines, headerStyle.Background(selectedBg).Render(rest))
			} else {
				lines = append(lines, headerStyle.Render(header))
			}
		} else {
			// Render agent (indented under header)
			agent := item.agent
			var indicator string
			indicatorChar := "✓"
			if agent.IsActive() {
				indicatorChar = "●"
			}

			name := agent.Description
			if name == "" {
				name = agent.ID
			}
			maxNameLen := width - 5 // "  " indent + indicator + space + name
			if len(name) > maxNameLen {
				name = name[:maxNameLen]
			}

			if i == m.selectedIdx {
				selectedBg := lipgloss.AdaptiveColor{Light: "254", Dark: "8"}
				var styledIndicator string
				if agent.IsActive() {
					styledIndicator = activeStyle.Background(selectedBg).Render(indicatorChar)
				} else {
					styledIndicator = doneStyle.Background(selectedBg).Render(indicatorChar)
				}
				rest := fmt.Sprintf(" %s", name)
				if len("  ")+len(indicatorChar)+len(rest) < width {
					rest = rest + strings.Repeat(" ", width-len("  ")-len(indicatorChar)-len(rest))
				}
				lines = append(lines, "  "+styledIndicator+selectedBgStyle.Render(rest))
			} else {
				if agent.IsActive() {
					indicator = activeStyle.Render(indicatorChar)
				} else {
					indicator = doneStyle.Render(indicatorChar)
				}
				lines = append(lines, fmt.Sprintf("  %s %s", indicator, name))
			}
		}
	}

	// Bottom scroll indicator
	if showBottomIndicator {
		indicator := fmt.Sprintf("↓ %d more", hiddenBelow)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, doneStyle.Render(indicator))
	}

	return strings.Join(lines, "\n")
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: render channel headers with indented agents in sidebar"
```

---

## Task 6: Update CLI to pass new parameters

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/scope/git.go` (if needed for repo name extraction)

**Step 1: Update root.go**

Update the CLI to compute and pass the new parameters to `NewModel`:

```go
// Extract repo name from base path
repoName := filepath.Base(basePath)

// Get Claude projects base directory
homeDir, _ := os.UserHomeDir()
claudeProjectsDir := filepath.Join(homeDir, ".claude", "projects")

m := tui.NewModel(claudeProjectsDir, basePath, repoName)
```

**Step 2: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: update CLI to pass channel parameters to TUI"
```

---

## Task 7: Integration testing

**Files:**
- Test: `internal/claude/channels_test.go`

**Step 1: Add integration test with real-world-like structure**

```go
func TestScanChannels_Integration(t *testing.T) {
	// Create a structure mimicking real June worktrees
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")

	// Mimic: june main + 2 worktrees
	dirs := []struct {
		name   string
		agents []string
	}{
		{"-Users-test-code-june", []string{"agent-main1.jsonl", "agent-main2.jsonl"}},
		{"-Users-test-code-june--worktrees-channels", []string{"agent-ch1.jsonl"}},
		{"-Users-test-code-june--worktrees-select-mode", []string{"agent-sel1.jsonl", "agent-sel2.jsonl"}},
	}

	for _, d := range dirs {
		dir := filepath.Join(claudeProjects, d.name)
		os.MkdirAll(dir, 0755)
		for _, a := range d.agents {
			os.WriteFile(filepath.Join(dir, a), []byte(`{"type":"user","message":{"role":"user","content":"Test task"}}`+"\n"), 0644)
		}
	}

	channels, err := ScanChannels(claudeProjects, "/Users/test/code/june", "june")
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	// Verify all channels found
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}

	// Verify channel names contain expected patterns
	names := make(map[string]bool)
	for _, ch := range channels {
		names[ch.Name] = true
	}
	if !names["june:main"] {
		t.Error("missing june:main channel")
	}
	if !names["june:channels"] {
		t.Error("missing june:channels channel")
	}
	if !names["june:select-mode"] {
		t.Error("missing june:select-mode channel")
	}
}
```

**Step 2: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/claude/channels_test.go
git commit -m "test: add integration test for multi-worktree channel scanning"
```

---

## Task 8: Manual testing and polish

**Step 1: Build and run**

```bash
make build
./june
```

**Step 2: Verify expected behavior**

- [ ] Channels appear with headers like `june:main`, `june:channels`
- [ ] Agents are indented under their channel headers
- [ ] Navigation with j/k works across headers and agents
- [ ] Selecting an agent shows its transcript
- [ ] Selecting a header shows nothing (or first agent in that channel)
- [ ] Scroll indicators work correctly

**Step 3: Final commit if any polish needed**

```bash
git add -A
git commit -m "polish: minor adjustments for multi-worktree display"
```

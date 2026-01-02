# Smart Agent Filtering Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce sidebar clutter by showing only recent/active agents by default, with expand-to-show-all functionality, and sort channels by activity.

**Architecture:** Add filtering logic to `sidebarItems()` that limits visible agents per channel to active + recent (2 hours). Add an "expander" item type that shows hidden count and toggles expansion. Sort channels: active-recent first (alphabetical), then inactive (alphabetical).

**Tech Stack:** Go, Bubbletea TUI framework, existing lipgloss styling

---

## Task 1: Add recent threshold constant and helper

**Files:**
- Modify: `internal/claude/agents.go`
- Test: `internal/claude/agents_test.go`

**Step 1: Write the failing test**

Add to `internal/claude/agents_test.go`:

```go
func TestAgent_IsRecent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		lastMod  time.Time
		expected bool
	}{
		{"1 hour ago", now.Add(-1 * time.Hour), true},
		{"2 hours ago", now.Add(-2 * time.Hour), true},
		{"3 hours ago", now.Add(-3 * time.Hour), false},
		{"1 day ago", now.Add(-24 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := Agent{LastMod: tt.lastMod}
			if got := agent.IsRecent(); got != tt.expected {
				t.Errorf("IsRecent() = %v, want %v", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude -run TestAgent_IsRecent -v`
Expected: FAIL with "agent.IsRecent undefined"

**Step 3: Write minimal implementation**

Add to `internal/claude/agents.go`:

```go
const recentThreshold = 2 * time.Hour

// IsRecent returns true if the agent was modified within the recent threshold (2 hours).
func (a Agent) IsRecent() bool {
	return time.Since(a.LastMod) < recentThreshold
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude -run TestAgent_IsRecent -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/agents.go internal/claude/agents_test.go
git commit -m "feat: add IsRecent helper for 2-hour activity threshold"
```

---

## Task 2: Add Channel.HasRecentActivity helper

**Files:**
- Modify: `internal/claude/channels.go`
- Test: `internal/claude/channels_test.go`

**Step 1: Write the failing test**

Add to `internal/claude/channels_test.go`:

```go
func TestChannel_HasRecentActivity(t *testing.T) {
	now := time.Now()

	recentAgent := Agent{ID: "recent", LastMod: now.Add(-1 * time.Hour)}
	oldAgent := Agent{ID: "old", LastMod: now.Add(-24 * time.Hour)}
	activeAgent := Agent{ID: "active", LastMod: now.Add(-5 * time.Second)}

	tests := []struct {
		name     string
		agents   []Agent
		expected bool
	}{
		{"has active agent", []Agent{activeAgent, oldAgent}, true},
		{"has recent agent", []Agent{recentAgent, oldAgent}, true},
		{"only old agents", []Agent{oldAgent}, false},
		{"empty", []Agent{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := Channel{Agents: tt.agents}
			if got := ch.HasRecentActivity(); got != tt.expected {
				t.Errorf("HasRecentActivity() = %v, want %v", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude -run TestChannel_HasRecentActivity -v`
Expected: FAIL with "ch.HasRecentActivity undefined"

**Step 3: Write minimal implementation**

Add to `internal/claude/channels.go`:

```go
// HasRecentActivity returns true if any agent is active or was modified recently.
func (c Channel) HasRecentActivity() bool {
	for _, a := range c.Agents {
		if a.IsActive() || a.IsRecent() {
			return true
		}
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude -run TestChannel_HasRecentActivity -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/channels.go internal/claude/channels_test.go
git commit -m "feat: add HasRecentActivity helper for channels"
```

---

## Task 3: Update channel sorting in ScanChannels

**Files:**
- Modify: `internal/claude/channels.go`
- Test: `internal/claude/channels_test.go`

**Step 1: Write the failing test**

Add to `internal/claude/channels_test.go`:

```go
func TestScanChannels_SortsByActivityThenAlphabetical(t *testing.T) {
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")

	now := time.Now()
	recentTime := now.Add(-1 * time.Hour)
	oldTime := now.Add(-24 * time.Hour)

	// Create channels: zebra (recent), alpha (old), beta (recent), gamma (old)
	dirs := []struct {
		name    string
		modTime time.Time
	}{
		{"-Users-test-code-proj--worktrees-zebra", recentTime},
		{"-Users-test-code-proj--worktrees-alpha", oldTime},
		{"-Users-test-code-proj--worktrees-beta", recentTime},
		{"-Users-test-code-proj--worktrees-gamma", oldTime},
	}

	for _, d := range dirs {
		dir := filepath.Join(claudeProjects, d.name)
		os.MkdirAll(dir, 0755)
		agentFile := filepath.Join(dir, "agent-test.jsonl")
		os.WriteFile(agentFile, []byte(`{"type":"user","message":{"role":"user","content":"Test"}}`+"\n"), 0644)
		os.Chtimes(agentFile, d.modTime, d.modTime)
	}

	channels, err := ScanChannels(claudeProjects, "/Users/test/code/proj", "proj")
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	// Expected order: recent channels alphabetically (beta, zebra), then old alphabetically (alpha, gamma)
	expectedOrder := []string{"proj:beta", "proj:zebra", "proj:alpha", "proj:gamma"}

	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}

	for i, ch := range channels {
		if ch.Name != expectedOrder[i] {
			t.Errorf("channel[%d] = %s, want %s", i, ch.Name, expectedOrder[i])
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude -run TestScanChannels_SortsByActivityThenAlphabetical -v`
Expected: FAIL (current sorting is by recency, not activity+alphabetical)

**Step 3: Update implementation**

Modify the sort in `ScanChannels` in `internal/claude/channels.go`:

```go
// Sort channels: recent-activity first (alphabetical), then inactive (alphabetical)
sort.Slice(channels, func(i, j int) bool {
	iRecent := channels[i].HasRecentActivity()
	jRecent := channels[j].HasRecentActivity()
	if iRecent != jRecent {
		return iRecent // recent channels come first
	}
	// Within same activity group, sort alphabetically by name
	return channels[i].Name < channels[j].Name
})
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/claude -run TestScanChannels_SortsByActivityThenAlphabetical -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/channels.go internal/claude/channels_test.go
git commit -m "feat: sort channels by activity then alphabetically"
```

---

## Task 4: Add expandedChannels state to Model

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add field to Model struct**

In `internal/tui/model.go`, add to the Model struct (around line 65):

```go
expandedChannels map[int]bool // Channel indices that are expanded to show all agents
```

**Step 2: Initialize in NewModel**

Update `NewModel` to initialize the map:

```go
func NewModel(claudeProjectsDir, basePath, repoName string) Model {
	return Model{
		claudeProjectsDir: claudeProjectsDir,
		basePath:          basePath,
		repoName:          repoName,
		channels:          []claude.Channel{},
		transcripts:       make(map[string][]claude.Entry),
		expandedChannels:  make(map[int]bool),
		viewport:          viewport.New(0, 0),
	}
}
```

**Step 3: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: add expandedChannels state to Model"
```

---

## Task 5: Add expander item type to sidebarItem

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Update sidebarItem struct**

Update the `sidebarItem` struct to support expander items:

```go
type sidebarItem struct {
	isHeader    bool
	isExpander  bool          // True for "show N more" items
	channelName string        // Only set for headers
	channelIdx  int           // Index into m.channels
	agent       *claude.Agent // Only set for agents
	agentIdx    int           // Index within channel's agents slice
	hiddenCount int           // Only set for expanders: how many agents are hidden
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: add expander item type to sidebarItem"
```

---

## Task 6: Update sidebarItems to filter agents and add expanders

**Files:**
- Modify: `internal/tui/model.go`
- Test: `internal/tui/model_test.go`

**Step 1: Write the failing test**

Add to `internal/tui/model_test.go`:

```go
func TestSidebarItems_FiltersOldAgents(t *testing.T) {
	now := time.Now()
	agents := []claude.Agent{
		{ID: "active", LastMod: now.Add(-5 * time.Second)},   // active
		{ID: "recent", LastMod: now.Add(-1 * time.Hour)},     // recent
		{ID: "old1", LastMod: now.Add(-24 * time.Hour)},      // old
		{ID: "old2", LastMod: now.Add(-48 * time.Hour)},      // old
	}

	m := createModelWithAgents(agents, 80, 40)
	items := m.sidebarItems()

	// Should have: 1 header + 2 visible agents (active, recent) + 1 expander
	// Total: 4 items
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	// First item is header
	if !items[0].isHeader {
		t.Error("first item should be header")
	}

	// Last item should be expander with count 2
	expander := items[3]
	if !expander.isExpander {
		t.Error("last item should be expander")
	}
	if expander.hiddenCount != 2 {
		t.Errorf("expander.hiddenCount = %d, want 2", expander.hiddenCount)
	}
}

func TestSidebarItems_ExpandedShowsAll(t *testing.T) {
	now := time.Now()
	agents := []claude.Agent{
		{ID: "active", LastMod: now.Add(-5 * time.Second)},
		{ID: "old1", LastMod: now.Add(-24 * time.Hour)},
		{ID: "old2", LastMod: now.Add(-48 * time.Hour)},
	}

	m := createModelWithAgents(agents, 80, 40)
	m.expandedChannels[0] = true // Expand channel 0
	items := m.sidebarItems()

	// Should have: 1 header + 3 agents (all shown), no expander
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	// No expander when expanded
	for _, item := range items {
		if item.isExpander {
			t.Error("should not have expander when channel is expanded")
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run "TestSidebarItems_FiltersOldAgents|TestSidebarItems_ExpandedShowsAll" -v`
Expected: FAIL

**Step 3: Update sidebarItems implementation**

Replace `sidebarItems()` in `internal/tui/model.go`:

```go
// sidebarItems returns a flat list of all items to display in the sidebar.
// Filters agents to show only active/recent unless channel is expanded.
func (m Model) sidebarItems() []sidebarItem {
	var items []sidebarItem
	for ci, ch := range m.channels {
		// Add channel header
		items = append(items, sidebarItem{
			isHeader:    true,
			channelName: ch.Name,
			channelIdx:  ci,
		})

		expanded := m.expandedChannels[ci]
		var visibleAgents, hiddenAgents []int

		for ai, agent := range ch.Agents {
			if expanded || agent.IsActive() || agent.IsRecent() {
				visibleAgents = append(visibleAgents, ai)
			} else {
				hiddenAgents = append(hiddenAgents, ai)
			}
		}

		// Add visible agents
		for _, ai := range visibleAgents {
			items = append(items, sidebarItem{
				isHeader:   false,
				channelIdx: ci,
				agent:      &m.channels[ci].Agents[ai],
				agentIdx:   ai,
			})
		}

		// Add expander if there are hidden agents
		if len(hiddenAgents) > 0 {
			items = append(items, sidebarItem{
				isExpander:  true,
				channelIdx:  ci,
				hiddenCount: len(hiddenAgents),
			})
		}
	}
	return items
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run "TestSidebarItems_FiltersOldAgents|TestSidebarItems_ExpandedShowsAll" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "feat: filter old agents in sidebar, add expander items"
```

---

## Task 7: Render expander items in sidebar

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Update renderSidebarContent**

In the render loop in `renderSidebarContent`, add handling for expander items after the agent rendering block:

```go
} else if item.isExpander {
	// Render expander: "↓ N older"
	text := fmt.Sprintf("↓ %d older", item.hiddenCount)
	if len(text) > width {
		text = text[:width]
	}
	if i == m.selectedIdx {
		selectedBg := lipgloss.AdaptiveColor{Light: "254", Dark: "8"}
		padded := text
		if len(padded) < width {
			padded = padded + strings.Repeat(" ", width-len(padded))
		}
		lines = append(lines, selectedBgStyle.Background(selectedBg).Render(padded))
	} else {
		lines = append(lines, doneStyle.Render(text))
	}
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: render expander items in sidebar"
```

---

## Task 8: Handle Enter/Space to toggle expansion

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add key handling in Update**

In the `Update` function's key handling section, add a case for Enter/Space:

```go
case "enter", " ":
	if m.focusedPanel == panelLeft {
		items := m.sidebarItems()
		if m.selectedIdx >= 0 && m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.isExpander {
				// Toggle expansion for this channel
				if m.expandedChannels[item.channelIdx] {
					delete(m.expandedChannels, item.channelIdx)
				} else {
					m.expandedChannels[item.channelIdx] = true
				}
			}
		}
	}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: handle Enter/Space to toggle channel expansion"
```

---

## Task 9: Update totalSidebarItems for expanders

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Update totalSidebarItems**

The current `totalSidebarItems()` counts headers + all agents. Update it to account for filtering:

```go
// totalSidebarItems returns the total count of visible items.
func (m Model) totalSidebarItems() int {
	return len(m.sidebarItems())
}
```

Note: This is simpler but slightly less efficient. For a TUI with reasonable data sizes, this is fine.

**Step 2: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/tui/model.go
git commit -m "refactor: simplify totalSidebarItems to use sidebarItems length"
```

---

## Task 10: Update SelectedAgent for expanders

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Update SelectedAgent**

Ensure `SelectedAgent` returns nil for expander items (they're not agents):

```go
func (m Model) SelectedAgent() *claude.Agent {
	items := m.sidebarItems()
	if m.selectedIdx < 0 || m.selectedIdx >= len(items) {
		return nil
	}
	item := items[m.selectedIdx]
	if item.isHeader || item.isExpander {
		return nil
	}
	return item.agent
}
```

**Step 2: Commit**

```bash
git add internal/tui/model.go
git commit -m "fix: SelectedAgent returns nil for expander items"
```

---

## Task 11: Final testing and polish

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Build and manual test**

Run: `make build && ./june`

Verify:
- [ ] Channels with recent activity appear first, alphabetically sorted
- [ ] Channels without recent activity appear below, alphabetically sorted
- [ ] Only active/recent agents shown by default
- [ ] "↓ N older" expander appears when there are hidden agents
- [ ] Pressing Enter/Space on expander shows all agents
- [ ] Pressing Enter/Space again collapses back (or stays expanded - your choice)
- [ ] Navigation works correctly with expanders
- [ ] Right panel still shows last viewed agent when expander is selected

**Step 3: Final commit if needed**

```bash
git add -A
git commit -m "polish: final adjustments for smart agent filtering"
```

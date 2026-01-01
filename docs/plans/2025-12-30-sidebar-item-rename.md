# SidebarItem Rename Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename the `channel` struct to `SidebarItem` to clarify that it's a UI view model, not a domain concept.

**Architecture:** The current `channel` struct conflates "chat channel" with "sidebar row". We rename to `SidebarItem` with typed `Kind` enum to make the UI-layer nature explicit. The domain layer (`repo.Agent`) stays unchanged.

**Tech Stack:** Go, Bubbletea TUI framework

---

## Task 1: Add SidebarItemKind Enum

**Files:**
- Modify: `internal/tui/watch.go:115-123`

**Step 1: Add the typed enum above the channel struct**

Find this code:
```go
type channel struct {
	ID      string
	Name    string
	Kind    string
```

Add BEFORE it:
```go
// SidebarItemKind identifies the type of item in the sidebar
type SidebarItemKind int

const (
	SidebarChannelHeader  SidebarItemKind = iota // Project/branch header (e.g., "june/main")
	SidebarAgentRow                              // An agent in the channel
	SidebarArchivedSection                       // "N archived" collapsible section
	SidebarDivider                               // Visual separator between channels
)
```

**Step 2: Verify it compiles**

Run: `go build ./internal/tui/...`
Expected: Success (no errors)

**Step 3: Commit**

```bash
git add internal/tui/watch.go
git commit -m "feat(tui): add SidebarItemKind enum"
```

---

## Task 2: Rename channel to SidebarItem

**Files:**
- Modify: `internal/tui/watch.go` (struct definition and all usages)

**Step 1: Rename the struct and change Kind type**

Replace:
```go
type channel struct {
	ID      string
	Name    string
	Kind    string
	Status  string
	Level   int
	Project string
	Branch  string
}
```

With:
```go
// SidebarItem represents a single row in the TUI sidebar.
// This is a view model - the domain model is repo.Agent.
type SidebarItem struct {
	ID      string
	Name    string
	Kind    SidebarItemKind
	Status  string
	Level   int
	Project string
	Branch  string
}
```

**Step 2: Update all references in watch.go**

Use find/replace:
- `channel{` → `SidebarItem{`
- `channel struct` → `SidebarItem struct` (already done)
- `[]channel` → `[]SidebarItem`
- `func (m model) channels()` → `func (m model) sidebarItems()`
- All calls to `m.channels()` → `m.sidebarItems()`

**Step 3: Update Kind string literals to enum values**

Replace throughout watch.go:
- `Kind: "project_header"` → `Kind: SidebarChannelHeader`
- `Kind: "agent"` → `Kind: SidebarAgentRow`
- `Kind: "archived_count"` → `Kind: SidebarArchivedSection`
- `Kind: "separator"` → `Kind: SidebarDivider`

Replace Kind comparisons:
- `ch.Kind == "project_header"` → `ch.Kind == SidebarChannelHeader`
- `ch.Kind == "agent"` → `ch.Kind == SidebarAgentRow`
- `ch.Kind == "archived_count"` → `ch.Kind == SidebarArchivedSection`
- `ch.Kind == "separator"` → `ch.Kind == SidebarDivider`
- `selected.Kind == "project_header"` → `selected.Kind == SidebarChannelHeader`
- `selected.Kind == "archived_count"` → `selected.Kind == SidebarArchivedSection`

**Step 4: Verify it compiles**

Run: `go build ./internal/tui/...`
Expected: Success

**Step 5: Commit**

```bash
git add internal/tui/watch.go
git commit -m "refactor(tui): rename channel to SidebarItem"
```

---

## Task 3: Update Test Files

**Files:**
- Modify: `internal/tui/watch_channels_test.go`
- Modify: `internal/tui/watch_selection_test.go`
- Modify: `internal/tui/watch_panels_test.go`

**Step 1: Update watch_channels_test.go**

Replace all occurrences:
- `channels()` → `sidebarItems()`
- `channels[` → `items[` (local var rename for clarity)
- `ch.Kind != "project_header"` → `ch.Kind != SidebarChannelHeader`
- `ch.Kind == "project_header"` → `ch.Kind == SidebarChannelHeader`
- `ch.Kind == "archived_count"` → `ch.Kind == SidebarArchivedSection`
- `ch.Kind == "separator"` → `ch.Kind == SidebarDivider`
- `ch.Kind == "agent"` → `ch.Kind == SidebarAgentRow`
- `Kind != "separator"` → `Kind != SidebarDivider`

**Step 2: Update watch_selection_test.go**

Same replacements as Step 1.

**Step 3: Update watch_panels_test.go**

Same replacements as Step 1.

**Step 4: Run all TUI tests**

Run: `go test ./internal/tui/... -v`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/tui/watch_*_test.go
git commit -m "test(tui): update tests for SidebarItem rename"
```

---

## Task 4: Add String() Method for Debugging

**Files:**
- Modify: `internal/tui/watch.go`

**Step 1: Add String method to SidebarItemKind**

Add after the const block:
```go
func (k SidebarItemKind) String() string {
	switch k {
	case SidebarChannelHeader:
		return "channel_header"
	case SidebarAgentRow:
		return "agent"
	case SidebarArchivedSection:
		return "archived_section"
	case SidebarDivider:
		return "divider"
	default:
		return "unknown"
	}
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/tui/...`
Expected: Success

**Step 3: Run tests**

Run: `go test ./internal/tui/... -v`
Expected: All pass

**Step 4: Commit**

```bash
git add internal/tui/watch.go
git commit -m "feat(tui): add String() method to SidebarItemKind"
```

---

## Task 5: Final Cleanup and Verification

**Files:**
- All `internal/tui/*.go` files

**Step 1: Search for any remaining "channel" references**

Run: `grep -n "channel" internal/tui/watch*.go | grep -v "activeChannelID" | grep -v "mainChannelID"`

Expected: No matches (activeChannelID and mainChannelID are fine - they refer to chat channels)

**Step 2: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass

**Step 3: Commit any stragglers**

```bash
git add -A
git commit -m "refactor(tui): complete SidebarItem rename cleanup"
```

---

## Summary

| Task | Description | Est. Time |
|------|-------------|-----------|
| 1 | Add SidebarItemKind enum | 5 min |
| 2 | Rename channel → SidebarItem | 20 min |
| 3 | Update test files | 15 min |
| 4 | Add String() method | 5 min |
| 5 | Final cleanup | 5 min |

**Total: ~50 minutes**

After completion:
- `channel` struct → `SidebarItem` (clear it's a view model)
- `Kind string` → `Kind SidebarItemKind` (type-safe, no magic strings)
- `channels()` → `sidebarItems()` (clear it returns UI items)
- Domain layer (`repo.Agent`) unchanged

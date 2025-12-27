# TUI Project/Branch Grouping Implementation Plan

> **For Claude:** Use TDD to implement this plan task-by-task.

**Goal:** Group agents under project/branch headers in the TUI sidebar, make headers selectable for orchestrator chat, and enable sending messages to spawn/prompt the orchestrator.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, SQLite (repo + scope).

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Orchestrator name | `@otto` | Short, brand-consistent, matches tool name |
| Orchestrator lifecycle | On-demand (lazy) | Spawned when user sends first message, not pre-created |
| TUI restart behavior | Resume if running, spawn new if complete | Check agent status, prompt if busy, spawn if not |
| Click project header | Show `@otto` chat for that project/branch | Right panel switches to orchestrator messages |
| Click agent | Show agent transcript | Current behavior, unchanged |
| Empty state | "No conversation yet" + input | Show message history if any, input always visible |

### Orchestrator Spawning Logic

When user sends a message to a project header:

1. Check if `@otto` agent exists for this project/branch
2. **If otto is busy** → Submit button disabled, user must wait (no queuing in P1)
3. **If otto exists and finished** (`status = complete` or `failed`) → `otto prompt otto "message"` (resumes session)
4. **If otto doesn't exist** → `otto spawn codex "message" --name otto`

**spawn vs prompt distinction:**
- `spawn` - Creates new agent row + new session
- `prompt` - Resumes existing agent's session (agent must exist and not be busy)

**Rationale:** Orchestrator is session-scoped, not task-scoped. We want to resume conversations, not start fresh each time.

### Message Queuing (Deferred to P3)

In P1, there is no message queuing:
- **User → Otto:** TUI disables submit button while otto is busy
- **Otto → Subagent:** `otto prompt` fails with error if agent is busy

In P3, the daemon will handle queuing:
- Messages stored in DB while agent is busy
- Daemon detects agent completion and delivers queued messages (batch delivery - all at once, like catching up on chat)

### Not In Scope (Deferred)

- Compaction handling / skill re-injection (P3)
- Activity feed split (P4)
- Daemon wake-ups (P3)

---

## Implementation Tasks

### Task 1: Add project/branch grouping to channel list

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing test for grouped channels**

Add a test that seeds agents across multiple projects/branches and asserts channel ordering/structure:
- Main first.
- Project header per `project/branch` in stable order.
- Agents under headers.
- Archived section still last.

Example test skeleton:

```go
func TestChannelsGroupByProjectBranch(t *testing.T) {
    m := NewModel(nil)
    m.agents = []repo.Agent{
        {Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
        {Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
        {Project: "other", Branch: "branch", Name: "worker", Status: "complete"},
    }

    channels := m.channels()
    // expect: Main, otto/main header, impl-1, reviewer, other/branch header, worker
    if got := channels[0].ID; got != mainChannelID { t.Fatalf("got %q", got) }
    // add asserts for header kinds + ordering
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestChannelsGroupByProjectBranch -v`
Expected: FAIL (no grouping behavior).

**Step 3: Implement grouping in channels()**

- Extend `channel` with `Project`, `Branch`, and an `Indent` or `Level` field.
- Add a `projectHeader` kind (or similar constant) to represent headers.
- Build a map of active agents by `project/branch`, sort group keys, then append header + agents.
- Ensure archived agents are excluded in this task (archived handled in Task 2).

**Step 4: Run tests**

Run: `go test ./internal/tui -run TestChannelsGroupByProjectBranch -v`
Expected: PASS

---

### Task 2: Add per-project collapse state and archived grouping

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing tests for collapse + archived grouping**

Add tests covering:
- Project headers default expanded (or auto-expanded for current scope) and show agents.
- Collapsed headers hide agents (but header remains).
- Archived section contains grouped archived agents under the same header style when expanded.

Example test idea:

```go
func TestProjectHeaderCollapse(t *testing.T) {
    m := NewModel(nil)
    m.projectExpanded = map[string]bool{"otto/main": false}
    m.agents = []repo.Agent{{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"}}

    channels := m.channels()
    // expect Main, header only (no agent entry)
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui -run ProjectHeaderCollapse -v`
Expected: FAIL

**Step 3: Implement expanded state + archived grouping**

- Add `projectExpanded map[string]bool` to `model`.
- Implement a helper key like `projectBranchKey(project, branch)` (e.g., `"otto/main"`).
- In `channels()`, only include agents for a group if expanded.
- For archived agents: under `archived_header` include project headers + agents when archived is expanded.

**Step 4: Run tests**

Run: `go test ./internal/tui -run ProjectHeaderCollapse -v`
Expected: PASS

---

### Task 3: Selection behavior for project headers

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing tests for selection semantics**

Add tests to assert:
- Selecting a project header sets `activeChannelID` to the project/branch key.
- Selecting an agent still sets `activeChannelID` to agent name.
- Selecting a header expands it if collapsed (optional spec: collapse via second activation or a dedicated key).

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui -run ProjectHeaderSelection -v`
Expected: FAIL

**Step 3: Implement selection logic**

- Update `activateSelection()` to:
  - Toggle `projectExpanded` for header rows when selected.
  - Set `activeChannelID` to the header key so the content panel switches to orchestrator chat for that project/branch.
- Ensure archived header behavior remains unchanged.

**Step 4: Run tests**

Run: `go test ./internal/tui -run ProjectHeaderSelection -v`
Expected: PASS

---

### Task 4: Rendering + indentation for grouped rows

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing rendering test**

Add a test that calls `renderChannelLine` for:
- A project header (no status indicator, uses header styling).
- An indented agent row (prefix + indentation, still shows status dot).

Use `lipgloss.NewStyle()` stripping or string contains for assertion to avoid ANSI fragility.

**Step 2: Run test to verify failure**

Run: `go test ./internal/tui -run RenderChannelLine -v`
Expected: FAIL

**Step 3: Implement rendering changes**

- In `renderChannelLine`, add indentation for agent rows under a header.
- For headers, render label without status dot and with muted/bold style distinct from agents.
- Keep cursor background behavior consistent.

**Step 4: Run tests**

Run: `go test ./internal/tui -run RenderChannelLine -v`
Expected: PASS

---

### Task 5: Fetch and display orchestrator chat per project/branch

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing test for message filtering**

Add a test that sets the active channel to a project header key and verifies `fetchMessagesCmd` uses that project/branch (not the global scope).

**Step 2: Run test to verify failure**

Run: `go test ./internal/tui -run ProjectHeaderMessages -v`
Expected: FAIL

**Step 3: Implement project-aware message fetching**

- Store `activeProject` and `activeBranch` in the model (or derive from `activeChannelID` via a lookup table built in `channels()`).
- Update `fetchMessagesCmd` calls to use the active project/branch instead of only `scope.CurrentContext()`.
- Ensure agent transcript fetching continues to use the agent name + active project/branch.

**Step 4: Run tests**

Run: `go test ./internal/tui -run ProjectHeaderMessages -v`
Expected: PASS

---

### Task 6: Mouse/keyboard navigation polish

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing tests**

Add tests covering:
- Mouse click on header selects project and updates cursor index.
- Up/down navigation skips hidden (collapsed) agent rows and respects new channel list length.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui -run ProjectHeaderMouse -v`
Expected: FAIL

**Step 3: Implement navigation updates**

- Ensure click logic indexes into the new channel list (already computed via `channels()`), and header selection triggers expansion + chat.
- Confirm `ensureSelection()` handles missing/hidden rows after collapse.

**Step 4: Run tests**

Run: `go test ./internal/tui -run ProjectHeaderMouse -v`
Expected: PASS

---

### Task 7: Chat input and orchestrator spawning

**Files:**
- Modify: `internal/tui/watch.go`
- Modify: `internal/tui/watch_test.go`

**Step 1: Write failing tests for chat input**

Add tests covering:
- Text input component appears at bottom of right panel when project header selected.
- Submit button/action disabled when `@otto` is `busy`.
- Submitting message when no `@otto` exists spawns new orchestrator.
- Submitting message when `@otto` is `complete` or `failed` prompts existing agent (resumes session).

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui -run ChatInput -v`
Expected: FAIL

**Step 3: Implement chat input**

- Add `textinput.Model` from Bubble Tea to the model.
- Render input at bottom of right panel when project header is active.
- On submit:
  - Look up `@otto` agent for active project/branch via `repo.GetAgent()`
  - If `status == busy`: ignore submit (button should be disabled)
  - If exists and finished (`complete`/`failed`): `otto prompt otto "message"` (resume)
  - If not found: `otto spawn codex "message" --name otto`
- Clear input after submit.
- Show visual indicator when otto is busy (grayed input, "Otto is working..." hint).

**Step 4: Run tests**

Run: `go test ./internal/tui -run ChatInput -v`
Expected: PASS

---

### Notes / Decisions

- Use `project/branch` as the header label and ID key to avoid collisions and align with design spec.
- Archived grouping mirrors active grouping for consistency.
- Default expansion: expand groups for the current `scope.CurrentContext()` and keep others collapsed unless previously expanded.
- Orchestrator is always named `@otto` (reserved name).
- Chat input only appears when a project header is selected, not when viewing agent transcripts.
- **CLI change:** `otto prompt` should check agent status and fail with error if agent is busy (prevents double-prompting same session).

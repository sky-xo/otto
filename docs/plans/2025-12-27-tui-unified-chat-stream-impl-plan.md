# TUI Unified Chat Stream Implementation Plan

**Source design:** `docs/plans/2025-12-27-unified-chat-stream-design.md`
**Scope:** Planning only. No code changes.

## Assumptions and Notes
- The plan stays centered on `internal/tui/` and `internal/repo/`, with callouts to other packages only where required for status or message creation behavior.
- “chat” message type is additive and should not break existing `prompt`, `complete`, `exit`, or `error` flows.
- The “finishing” agent status requires both persistence (repo) and UX updates (TUI), even if the status transition is triggered elsewhere.

## Phase 1: Three-Focus Keyboard Model

### Task 1.1: Introduce explicit focus enum and three-panel routing
- **Description:** Replace `focusedPanel` behavior with a three-state focus model (Agents, Stream viewport, Chat input). Ensure Tab cycles all three areas and that focus is distinct from “selected channel.”
- **Dependencies:** None.
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** Add/extend focus-cycle tests in `internal/tui/watch_test.go` for Tab order and focus-specific behavior.
- **Complexity:** Medium

### Task 1.2: Chat input captures all keys while focused
- **Description:** When focus is Chat input, route all keypresses to `textinput` except Tab/Esc. Prevent viewport scrolling and global shortcuts in this state.
- **Dependencies:** Task 1.1
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** Add regression tests in `internal/tui/watch_test.go` for key capture (e.g., `g/j/k` are inserted, not handled as shortcuts) and Esc/Tab behavior.
- **Complexity:** Medium

### Task 1.3: Focus visuals and cursor behavior
- **Description:** Ensure focused border highlights the three areas and cursor visibility is tied to Chat input focus only.
- **Dependencies:** Task 1.1
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** View-level checks in `internal/tui/watch_test.go` (snapshot-style or targeted string assertions for border styling if already used).
- **Complexity:** Small

## Phase 2: User Message Storage (`chat` type)

### Task 2.1: Define and persist `chat` message type
- **Description:** Add a new message type for user-entered text, and store it before spawning/prompting an agent. Ensure message fields follow design (`FromAgent: "you"`, `ToAgent: "june"`, `Content: raw input`).
- **Dependencies:** None.
- **Files to modify:**
  - `internal/tui/watch.go` (store `chat` messages on submit)
  - `internal/repo/messages.go` (optional: type constants or helper for `chat` messages)
  - `internal/repo/messages_test.go` (new tests validating `chat` inserts and filters)
- **Tests:** Add repo tests for `chat` messages in `internal/repo/messages_test.go`. Add TUI tests in `internal/tui/watch_test.go` to confirm `handleChatSubmit` writes a `chat` message and clears input.
- **Complexity:** Medium

### Task 2.2: Error visibility for spawn/prompt failures
- **Description:** If spawn/prompt fails after storing user chat, insert an error line into the stream (as a message of type `error` or a new error-renderable type) with the failure detail.
- **Dependencies:** Task 2.1
- **Files to modify:**
  - `internal/tui/watch.go` (failure path in `handleChatSubmit`)
  - `internal/repo/messages.go` (if adding helper for error messages)
  - `internal/tui/watch_test.go` (failure path coverage)
- **Tests:** Add TUI tests for failure path (assert new error message stored and rendered).
- **Complexity:** Medium

## Phase 3: Unified Stream Rendering

### Task 3.1: Message formatting and filtering rules
- **Description:** Implement Slack-style formatting for `chat` and `complete` (from june) and compact activity lines for prompts/completions from sub-agents. Hide `exit` and orchestrator-to-june `prompt` messages.
- **Dependencies:** Task 2.1
- **Files to modify:**
  - `internal/tui/watch.go` (message filtering and formatting in `mainContentLines`/`formatMessage`)
  - `internal/tui/formatting.go` (optional: extract formatting helpers)
  - `internal/tui/formatting_test.go` (add format expectations)
- **Tests:** Add formatting tests in `internal/tui/formatting_test.go` for each message type and hidden cases.
- **Complexity:** Large

### Task 3.2: Layout changes for chat-style entries
- **Description:** Render author on its own line, wrap message body on next line(s), and add a blank line after every entry (chat or activity) while preserving full-width wrapping.
- **Dependencies:** Task 3.1
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** Add or extend tests in `internal/tui/watch_test.go` to validate line structure and spacing.
- **Complexity:** Medium

### Task 3.3: Stream selection behavior
- **Description:** Enter on activity lines (prompt from june to sub-agent) should select the agent in the left panel and open transcript view (if implemented in current UI). If not currently supported, define a follow-up stub or no-op.
- **Dependencies:** Task 3.1
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** Add behavioral tests in `internal/tui/watch_test.go` (selection on activity line).
- **Complexity:** Medium

## Phase 4: Polish and Status Semantics

### Task 4.1: Color scheme updates
- **Description:** Apply color styling to `you`, `june`, sub-agents, and muted activity lines according to the design palette. Keep timestamps omitted.
- **Dependencies:** Task 3.1
- **Files to modify:** `internal/tui/watch.go`, `internal/tui/formatting.go`
- **Tests:** Optional snapshot-style formatting tests in `internal/tui/formatting_test.go`.
- **Complexity:** Small

### Task 4.2: Word wrapping improvements
- **Description:** Ensure wrap behavior is consistent with Slack-style layout and does not break on narrow widths.
- **Dependencies:** Task 3.2
- **Files to modify:** `internal/tui/watch.go` (or `internal/tui/formatting.go` if helpers extracted)
- **Tests:** Add wrap regression tests in `internal/tui/formatting_test.go` and `internal/tui/watch_test.go`.
- **Complexity:** Small

### Task 4.3: Scroll-to-bjunem on new messages
- **Description:** When new messages arrive and viewport is at bjunem, keep it pinned after update; otherwise preserve scroll position. Apply to both chat stream and transcript view.
- **Dependencies:** Task 3.2
- **Files to modify:** `internal/tui/watch.go`
- **Tests:** Add TUI tests in `internal/tui/watch_test.go` for at-bjunem vs not-at-bjunem cases.
- **Complexity:** Medium

### Task 4.4: Introduce `finishing` agent status
- **Description:** Add new status `finishing` between busy and complete; block new chat input when status is finishing. Update status indicator colors and semantics.
- **Dependencies:** Phase 1 (focus handling) and Phase 3 (stream rendering) can proceed independently; this task should align before final polish.
- **Files to modify:**
  - `internal/repo/agents.go` (helper to set `finishing` status, or update status handling)
  - `internal/tui/watch.go` (status display and input blocking logic)
  - `internal/tui/watch_test.go` (status indicator and input blocking tests)
  - Additional callouts if needed: `internal/cli/commands/complete.go`, `internal/cli/commands/spawn.go` to set `finishing` on `june complete` and `complete` on process exit
- **Tests:** Add/adjust repo tests in `internal/repo/agents_test.go` and TUI tests in `internal/tui/watch_test.go` for `finishing` state; add CLI tests if status transitions are updated.
- **Complexity:** Large

## Suggested Test Plan by Phase
- **Phase 1:** `go test ./internal/tui -run Focus` (or full `internal/tui` suite)
- **Phase 2:** `go test ./internal/repo -run Message` and `go test ./internal/tui -run ChatSubmit`
- **Phase 3:** `go test ./internal/tui -run Formatting` and relevant watch tests
- **Phase 4:** `go test ./internal/tui -run Status|Scroll` and `go test ./internal/repo -run Agent`

## Dependencies Summary
- Phase 1 is independent and should be completed first to stabilize keyboard routing.
- Phase 2 depends only on storage and can proceed in parallel with Phase 1 if desired, but should land before Phase 3 rendering changes.
- Phase 3 depends on Phase 2 because `chat` types are rendered distinctly.
- Phase 4 depends on Phase 3 for final rendering and on status semantics from Phase 2/3 if using `complete` as chat output.

# Otto Roadmap

What's next. For detailed design, see `docs/plans/`.

---

## Up Next (Implementation Order)

### ~~1. Codex Event Logging~~ ✅ DONE

Completed - see commits `727ef67`, `00d4d8e`, etc.

### 2. Unified Chat Stream ⬅️ IN PROGRESS

**Status:** Design complete, implementation plan ready

**Next Action:** Implement Phase 1 (Two-Focus Keyboard Model) using subagent-driven development

**Implementation Plan:** `docs/plans/2025-12-27-unified-chat-stream-design.md` - has detailed TDD tasks

**Phases:**
- Phase 1: Two-focus keyboard model (3 tasks) ← START HERE
- Phase 2: User message storage (2 tasks)
- Phase 3: Message rendering (Slack-style)
- Phase 4: Polish

### 3. Agent Chat in TUI

**Why:** Currently only orchestrator chat works. This adds chat with individual agents.

**Scope:**
- When agent selected: show input, send via `otto prompt <agent>`
- Builds on unified chat stream's keyboard model

### 4. Daemon Wake-ups (Superorchestrator Core)

**Why:** This is what makes Otto an orchestrator vs just a spawner.

**Scope:**
- Wire `processWakeups` into TUI tick loop
- On @mention: wake mentioned agent with context
- On agent exit: notify orchestrator
- After compaction: re-inject skills

### 5. File Diffs in Agent Transcripts

**Why:** TUI transcript shows file changes but not the actual diffs.

**Design:** `docs/plans/2025-12-27-agent-diff-capture-design.md` (DRAFT)

**Research:**
- `codex app-server` provides `turn/diff/updated` events with unified diffs
- JSON-RPC over stdio, experimental but used by VS Code extension

### 6. Transcript Replace-on-Complete

**Why:** Transcript view is noisy - shows every thinking step and command as permanent entries.

**Design:** `docs/plans/2025-12-28-tui-replace-on-complete-design.md` (DRAFT)

**Key ideas:**
- Ephemeral status (thinking + command) replaces in-place during turn
- Collapses when turn completes, leaving only durable output
- Spinner + shimmer animation for liveness

---

## Backlog (Deferred Items)

Issues identified during implementation, deferred for future work.

### From TUI Project Grouping (`docs/plans/2025-12-27-tui-project-grouping-plan.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| 1 | Global message state when switching projects | `m.messages` and `m.lastMessageID` are global but fetch scope changes per project header. Switching projects may show stale messages. | Scope message lists per `project/branch`, or reset when selecting different project header |
| 2 | `isProjectHeader()` misclassifies agent names with `/` | Any channel ID containing `/` is treated as project header. Agent names with `/` would route to orchestrator chat. | Check against actual channel list or use explicit `Kind` field |
| 3 | Unicode width uses `len()` not display width | `▼`/`▶` are multibyte but display as single char. Width calculations may cause visual misalignment. | Use `lipgloss.Width()` or `runewidth` for sizing |

### From Unified Chat Stream (`docs/plans/2025-12-27-unified-chat-stream-design.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| 4 | Spawn failure visibility | If spawn fails after storing `chat` message, user sees their message but no error. | Show error line in stream: `⚠ Failed to start otto: ...` |
| 5 | Two-phase completion ("finishing" status) | Agent calls `otto complete` but process continues outputting 10-30s more. Status shows "complete" while still talking. | Add `finishing` status: busy→finishing→complete. Visual: ● gray filled. |

### From Codex Event Logging (`docs/plans/2025-12-27-codex-event-logging-plan.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| 6 | TUI shows both `item.started` and `item.completed` | Verbose transcript - shows pending line then result line for same action. | Replace-on-complete: group by `item.id`, only show completed when both exist, show ⏳ indicator while running. |

---

## Future (Not V0)

- Hard gates on flow transitions
- Claude as orchestrator
- Multiple root tasks per branch
- Headless mode (`otto --headless`)
- Cross-project coordination patterns

---

## Docs

**Design:**
- `docs/plans/2025-12-25-super-orchestrator-v0-design.md` - Main design doc
- `docs/plans/2025-12-27-codex-event-logging-plan.md` - Codex event logging (Ready)
- `docs/plans/2025-12-27-unified-chat-stream-design.md` - Unified chat stream (Ready)
- `docs/plans/2025-12-27-agent-diff-capture-design.md` - File diffs (DRAFT)
- `docs/plans/2025-12-28-tui-replace-on-complete-design.md` - Replace-on-complete (DRAFT)

**Reference:**
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios

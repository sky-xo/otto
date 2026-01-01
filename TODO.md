# June Roadmap

What's next. For detailed design, see `docs/plans/`.

---

## Up Next (Implementation Order)

### ~~1. Codex Event Logging~~ ✅ DONE

Completed - see commits `727ef67`, `00d4d8e`, etc.

### ~~2. Unified Chat Stream~~ ✅ DONE

**Status:** All 4 phases complete

**Progress:**
- ✅ Task 1.1: Right panel sends all keys to chat input (commit `ea21dd7`)
- ✅ Task 1.2: Esc/Tab from right panel returns to sidebar (commit `93a05e3`)
- ✅ Task 1.3: Remove keyboard scrolling from right panel (commit `3207675`)
- ✅ Task 2.1: Add chat message type constant (commit `8855749`)
- ✅ Task 2.2: Store user chat message before spawn (commit `3113fce`)
- ✅ Task 3.1: Render chat and june completions as Slack-style blocks (commit `9c6a64b`)
- ✅ Task 3.2: Render activity lines and hide noise (commit `e0621a0`)
- ✅ Task 4.1: Apply color styles for chat and activity lines (commit `d53e1de`)
- ✅ Task 4.2: Improve word wrapping for chat blocks (commit `5ed007d`)
- ✅ Task 4.3: Scroll to bjunem on new messages (commit `c374a4b`)
- ✅ Bugfix: Wire Enter key to handleChatSubmit (commit `5b5986e`)

**Implementation Plan:** `docs/plans/2025-12-27-unified-chat-stream-design.md`

**Phases:**
- ✅ Phase 1: Two-focus keyboard model (3 tasks) - COMPLETE
- ✅ Phase 2: User message storage (2 tasks) - COMPLETE
- ✅ Phase 3: Message rendering (Slack-style) - COMPLETE
- ✅ Phase 4: Polish (3 tasks) - COMPLETE

### 3. Daemon Wake-ups (Superorchestrator Core) ⬅️ NEXT

**Why:** This is what makes June an orchestrator vs just a spawner.

**Scope:**
- Wire `processWakeups` into TUI tick loop
- On @mention: wake mentioned agent with context
- On agent exit: notify orchestrator
- After compaction: re-inject skills

### 4. Agent Chat in TUI

**Why:** Currently only orchestrator chat works. This adds chat with individual agents.

**Scope:**
- When agent selected: show input, send via `june prompt <agent>`
- Builds on unified chat stream's keyboard model

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

## Feedback from Claude about stumbling blocks when trying to use june to run codex subagents:

Stumbling Blocks

1. ~~Name collision on spawn~~ ✅ RESOLVED

First spawn failed with UNIQUE constraint failed because plan-reviewer already existed. Had to guess a unique name db-workflow-review-1.

**Status:** Tested and confirmed working. Auto-increment (`-2`, `-3`, etc.) prevents collisions. Original bug likely from older version.

2. ~~block parameter doesn't exist~~ ✅ ADDRESSED (was documentation bug)

The skill doc says to use BashOutput with block: true to wait - but that parameter doesn't exist on the tool. Got an error.

**Fix:** Corrected skill documentation - removed incorrect `block: true` reference.

3. ~~Finding the actual review was confusing~~ ✅ ADDRESSED

- june status → showed "busy" then "complete", no content
- june peek → showed agent reading files, not the final findings
- june messages → one-line summary: "Reviewed... reported issues"
- BashOutput on spawn → raw JSON stream, not human-readable
- june log --tail 100 → finally found it, but buried after 600+ lines of file contents

**Fix:** Enhanced `june peek` to show completion messages for completed agents. See `docs/plans/2025-12-29-agent-messaging-redesign.md`.

4. ~~No clean "get the result" command~~ ✅ ADDRESSED

The review findings were embedded in the agent's reasoning text within the JSON log. I had to parse through command output to find them.

**Fix:** Enhanced `june peek` serves this purpose. Also added `june dm <agent> "<message>"` for direct messaging.

---
What Would Be Ideal

1. ~~june result <agent>~~ ✅ ADDRESSED - `june peek <agent>` now shows completion message
2. ~~june status with preview~~ ✅ ADDRESSED - `june peek` provides this
3. ~~Clearer output separation~~ ✅ ADDRESSED - `june peek` shows completion messages separately
4. ~~Waiting mechanism~~ ✅ ADDRESSED - Documentation corrected (agents notify on completion automatically)
5. ✅ Name uniqueness help - Auto-suffix (`-2`, `-3`, etc.) already implemented and working

## Bugs

1. **Multiple june agents spawned** - Prompting in a project that already has an june agent spawns `june-2` instead of reusing the existing one. There should only ever be one june per project/branch.

Previous issues resolved:
- ~~Agent name collisions~~ - Fixed: Primary key is `(project, branch, name)` and auto-increment prevents duplicates
- ~~Chat shows summaries, not actual responses~~ - Fixed: Main view now shows june's actual responses from `agent_message` log entries. See `docs/plans/2025-12-31-june-chat-ux-fix.md`.

## Backlog (Deferred Items)

Issues identified during implementation, deferred for future work.

### From TUI Project Grouping (`docs/plans/2025-12-27-tui-project-grouping-plan.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| ~~1~~ | ~~Global message state when switching projects~~ | ~~`m.messages` and `m.lastMessageID` are global but fetch scope changes per project header. Switching projects may show stale messages.~~ | ✅ Fixed: Added `messagesProject` tracking, clear messages when switching projects (commit `089b037`) |
| 2 | `isProjectHeader()` misclassifies agent names with `/` | Any channel ID containing `/` is treated as project header. Agent names with `/` would route to orchestrator chat. | Check against actual channel list or use explicit `Kind` field |
| ~~3~~ | ~~Unicode width uses `len()` not display width~~ | ~~`▼`/`▶` are multibyte but display as single char. Width calculations may cause visual misalignment.~~ | ✅ Fixed: Use `len([]rune(...))` and `lipgloss.Width()` (commits `1d20454`, `fb9a1c2`) |

### From Unified Chat Stream (`docs/plans/2025-12-27-unified-chat-stream-design.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| 4 | Spawn failure visibility | If spawn fails after storing `chat` message, user sees their message but no error. | Show error line in stream: `⚠ Failed to start june: ...` |
| 5 | Two-phase completion ("finishing" status) | Agent calls `june complete` but process continues outputting 10-30s more. Status shows "complete" while still talking. | Add `finishing` status: busy→finishing→complete. Visual: ● gray filled. |

### From Codex Event Logging (`docs/plans/2025-12-27-codex-event-logging-plan.md`)

| # | Issue | Impact | Fix |
|---|-------|--------|-----|
| 6 | TUI shows both `item.started` and `item.completed` | Verbose transcript - shows pending line then result line for same action. | Replace-on-complete: group by `item.id`, only show completed when both exist, show ⏳ indicator while running. |

---

## Future (Not V0)

- Use Anthropic's Bloom to evaluate june vs claude + superpowers
- Hard gates on flow transitions
- Claude as orchestrator
- Multiple root tasks per branch
- Headless mode (`june --headless`)
- Cross-project coordination patterns

---

## Docs

**Design:**
- `docs/plans/2025-12-25-super-orchestrator-v0-design.md` - Main design doc
- `docs/plans/2025-12-27-codex-event-logging-plan.md` - Codex event logging (Ready)
- `docs/plans/2025-12-27-unified-chat-stream-design.md` - Unified chat stream (Ready)
- `docs/plans/2025-12-29-agent-messaging-redesign.md` - Agent messaging redesign (Ready)
- `docs/plans/2025-12-27-agent-diff-capture-design.md` - File diffs (DRAFT)
- `docs/plans/2025-12-28-tui-replace-on-complete-design.md` - Replace-on-complete (DRAFT)

**Reference:**
- `docs/ARCHITECTURE.md` - How June works
- `docs/SCENARIOS.md` - Usage scenarios

# Otto Roadmap

Quick overview of features and status. For detailed design, see `docs/plans/`.

## Implementation Status

Based on `docs/plans/2025-12-25-super-orchestrator-v0-design.md`.

### Backend (Phase 1) - COMPLETE

| Feature | Status | Notes |
|---------|--------|-------|
| Global DB at `~/.otto/otto.db` | ✅ Done | |
| Project/branch-aware schema | ✅ Done | agents, messages, logs, tasks tables |
| Scope context helper | ✅ Done | `scope.CurrentContext()` |
| @mention parsing | ✅ Done | Resolves to `project:branch:agent` |
| Codex event parsing | ✅ Done | `codex_events.go` |
| Compaction detection | ✅ Done | Parses `context_compacted` event |
| Session ID capture | ✅ Done | From `thread.started` event |
| Structured log entries | ✅ Done | event_type, command, exit_code, etc. |
| Tasks table | ✅ Done | Schema exists, repo functions work |
| Git worktree support | ✅ Done | `RepoRoot()` uses `--git-common-dir` for main repo |

### TUI (Phase 2) - COMPLETE

| Feature | Status | Notes |
|---------|--------|-------|
| Agent list with status indicators | ✅ Done | busy/complete/failed/waiting |
| Archived agents section | ✅ Done | Collapsible |
| Transcript view | ✅ Done | Formatting for reasoning, commands, input |
| Mouse support | ✅ Done | Click to select, scroll wheel |
| Keyboard navigation | ✅ Done | j/k/up/down move cursor; Enter/h/l toggle/expand/collapse |
| **Project/branch grouping** | ✅ Done | Sidebar groups agents by project:branch with headers |
| **Global agent view** | ✅ Done | TUI shows ALL agents across all projects |
| **Separators between groups** | ✅ Done | Visual separation between project groups |
| **Chat panel** | ✅ Done | Bottom panel for orchestrator chat with @otto |

### Daemon/Wake-ups - NOT STARTED

| Feature | Status | Notes |
|---------|--------|-------|
| Auto-wake on @mention | ❌ Not done | `wakeup.go` has helpers, not wired in |
| Auto-wake on agent exit | ❌ Not done | Should notify orchestrator |
| Context injection | ❌ Not done | `buildContextBundle` exists, unused |
| Skill re-injection after compaction | ❌ Not done | Design specifies this |

### CLI Commands - COMPLETE

All commands working: `spawn`, `status`, `peek`, `log`, `prompt`, `say`, `ask`, `complete`, `kill`, `interrupt`, `archive`, `messages`, `attach`, `install-skills`.

---

## Session Notes (2025-12-27)

### Just Completed
- **TUI global view**: Shows all agents across all projects (not just current git context)
- **Navigation UX fix**: Up/down just moves cursor, Enter/left/right toggles expand/collapse
- **Removed "Main" channel**: Redundant now that each project header acts as orchestrator entry point
- **Added separators**: Empty lines between project groups for visual clarity
- **Git worktree fix**: `RepoRoot()` now correctly returns main repo root for worktrees

### Commits (unpushed)
```
383e0ba feat(tui): show all projects globally, improve navigation
3959084 fix(scope): handle git worktrees correctly in RepoRoot()
```

---

## Remaining Work (Priority Order)

### P2: Agent Chat in TUI

**Why:** P1 covers orchestrator chat. This adds chat with individual agents.

**Scope:**
- When agent selected: show input, send via `otto prompt <agent>`

### P3: Daemon Wake-ups (Superorchestrator Core)

**Why:** This is what makes Otto an orchestrator vs just a spawner.

**Scope:**
- Wire `processWakeups` into TUI tick loop
- On @mention: wake mentioned agent with context
- On agent exit: notify orchestrator
- After compaction: re-inject skills

### P4: Activity Feed + Chat Split

**Why:** Design shows two-panel right side (feed top, chat bottom). Polish after core works.

**Scope:**
- Split right panel into top (activity) and bottom (chat)
- Activity: status changes, completions, agent spawns
- Chat: messages mentioning @otto, user input

### P5: File Diffs in Agent Transcripts

**Why:** TUI transcript shows file changes but not the actual diffs.

**Design:** `docs/plans/2025-12-27-agent-diff-capture-design.md` (DRAFT)

**Research (2025-12-27):**
- `codex app-server` provides `turn/diff/updated` events with unified diffs
- JSON-RPC over stdio, experimental but used by VS Code extension
- Uses same OAuth as `exec`
- Consider for richer diff capture vs git-based approach

---

## Future (Not V0)

- Hard gates on flow transitions
- Claude as orchestrator
- Multiple root tasks per branch
- Headless mode (`otto --headless`)
- Cross-project coordination patterns

## Docs

**Design (current):**
- `docs/plans/2025-12-25-super-orchestrator-v0-design.md` - Main design doc
- `docs/plans/2025-12-27-tui-project-grouping-plan.md` - P1 implementation (COMPLETE)
- `docs/plans/2025-12-27-agent-diff-capture-design.md` - Capturing file diffs (DRAFT)

**Reference:**
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios

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

### TUI (Phase 2) - PARTIAL

| Feature | Status | Notes |
|---------|--------|-------|
| Agent list with status indicators | ✅ Done | busy/complete/failed/waiting |
| Archived agents section | ✅ Done | Collapsible |
| Transcript view | ✅ Done | Formatting for reasoning, commands, input |
| Mouse support | ✅ Done | Click to select, scroll wheel |
| Keyboard navigation | ✅ Done | j/k, tab, enter, escape |
| **Project/branch grouping** | ❌ Not done | Sidebar should group agents by project:branch |
| **Activity feed panel** | ❌ Not done | Design shows top panel for status updates |
| **Chat panel** | ❌ Not done | Design shows bottom panel for orchestrator chat |

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

## Remaining Work (Priority Order)

### P1: Project/Branch Grouping in TUI (BLOCKING)

**Why:** The sidebar needs project:branch headers as clickable targets. "Click project = orchestrator chat, click agent = transcript" (design doc line 350). Without headers, there's no way to select which orchestrator to chat with.

**Design:** (from super-orchestrator-v0-design.md:331-350)
```
│ Projects     │
│              │
│ otto/main    │  ← Click this = chat with otto/main orchestrator
│   @impl-1 *  │  ← Click this = view impl-1 transcript
│   @reviewer  │
│ other/branch │  ← Click this = chat with other/branch orchestrator
│   @worker    │
```

**Scope:**
- Modify `channels()` to group agents by project:branch
- Add collapsible project:branch headers
- Indent agents under their group
- Clicking header selects that branch (for chat target)
- Clicking agent selects that agent (for transcript view)

### P2: Chat Input in TUI

**Why:** TUI is currently read-only. Depends on P1 - needs a selected target to send messages to.

**Scope:**
- Add `textinput` component from Bubble Tea
- Input area at bottom of right panel
- When project:branch selected: send to orchestrator (@otto for that branch)
- When agent selected: send to that agent (via `otto prompt`)
- Messages appear in chat/transcript view

### P3: Daemon Wake-ups (Superorchestrator Core)

**Why:** This is what makes Otto an orchestrator vs just a spawner. Depends on P1+P2.

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
- `docs/plans/2025-12-25-super-orchestrator-v0-phase-1-plan.md` - Backend implementation
- `docs/plans/2025-12-25-super-orchestrator-v0-phase-2-plan.md` - TUI implementation
- `docs/plans/2025-12-24-skill-injection-design.md` - Skill re-injection

**Reference:**
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios

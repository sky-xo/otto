# Super Orchestrator V0 Implementation Plan Review

**Date:** 2025-12-26
**Status:** In Progress
**Plans Reviewed:**
- `2025-12-25-super-orchestrator-v0-phase-1-plan.md`
- `2025-12-25-super-orchestrator-v0-phase-2-plan.md`

## Batch Approval Status

| Batch | Topic | Status |
|-------|-------|--------|
| 1 | Database Architecture | ✓ Approved |
| 2 | Addressing & Mentions | ✓ Approved |
| 3 | Codex Event Parsing | ✓ Approved |
| 4 | Daemon Wake-ups | ✓ Approved |
| 5 | TUI Changes (Phase 2) | ✓ Approved |

---

## Batch 1: Database Architecture — APPROVED

**Decisions:**
- Global DB at `~/.june/june.db` (via `config.DataDir()`) — verified in code
- Agent PK changes to `(project, branch, name)` — breaking change OK (greenfield)
- Tasks table with `parent_id` + `result` for hierarchical state
- **Skip legacy import helper** — not needed for V0

---

## Batch 2: Addressing & Mentions — APPROVED

**Decisions:**
- Regex updated to: `@([A-Za-z0-9._/-]+(?::[A-Za-z0-9._/-]+){0,2})`
- Supports: uppercase, underscores, dots, slashes (for branch names like `feature/login`)
- **Agent names normalized to lowercase; project/branch casing preserved**

**Example:**
```
@Impl-1 → app:feature/login:impl-1 (agent lowercased)
@backend:main:June → backend:main:june (agent lowercased)
```

---

## Batch 3: Codex Event Parsing — APPROVED

**Decisions:**
- **Minimal parsing approach** — only parse what we need:
  1. Session ID (from `thread.started`)
  2. Compaction signal (from `context_compacted`)
  3. Failure signal (for marking agent failed)
- Store everything else as raw content with type tag
- **Defer structured item parsing** (exit_code, file_change details) — not V0
- This approach is agent-type agnostic (works for Claude later)

**Logs table design:**
```sql
logs (
  agent_type TEXT,     -- 'codex' or 'claude'
  event_type TEXT,     -- normalized event type (reasoning, command_execution, etc.)
  raw_json TEXT,       -- full event for future parsing
  content TEXT,        -- extracted displayable content
  command TEXT,        -- for command_execution: the command string
  exit_code INTEGER,   -- for command_execution: exit code
  status TEXT,         -- for command_execution: in_progress/completed/failed
)
```

---

## Batch 4: Daemon Wake-ups — APPROVED

### Core Principle

**Daemon is dumb, orchestrator is smart.**
- Daemon: Detect events, report facts, inject context
- Orchestrator: Interpret status, make decisions, take action

### Wake-up Triggers

| Trigger | Action |
|---------|--------|
| @mention in any message | Wake the mentioned agent |
| Agent process exits | Notify orchestrator with context |

**Cross-project:** Yes, @mentions wake agents in any project/branch.

### Process Exit Handling

On any agent process exit, daemon notifies orchestrator with:
- Agent name and exit code
- Agent's current status (`busy`, `blocked`, `complete`, `failed`)
- All unread messages (via `last_seen_message_id`)
- Current task state

Orchestrator interprets and decides:
- Status `blocked` → answer the question
- Status `complete` → move to next task
- Status `failed` → handle error (retry, reassign, escalate)

### `june ask` Flow

```
1. Agent runs: june ask "which approach?"
   → Message stored, status = blocked
   → Agent process exits

2. Daemon detects exit, sees status = blocked
   → Notifies orchestrator with question context

3. Orchestrator answers: june prompt impl-1 "use approach A"
   → Agent respawned with answer
```

No implicit @june mention needed — daemon notifies on any exit.

### Compaction Re-injection

- On first wake-up after compaction: re-inject full skills + task state
- Subsequent wake-ups: only new messages (until next compaction)
- Track via `compacted_at` timestamp in agents table

### Message Deduplication

- Each agent has `last_seen_message_id`
- On wake-up: inject all messages since that ID
- Update ID after successful wake-up

### V0 Scope

- **Skip `june complete`** — process exit is sufficient
- **No auto-retry on failure** — orchestrator decides

---

## Batch 5: TUI Changes (Phase 2) — APPROVED

**Decisions:**

### Codex Event Structure (from research)
Based on OpenAI Codex CLI docs, events use these structs:

```go
// ReasoningItem - Text field IS the summary (full reasoning stays internal)
type ReasoningItem struct {
    ID   string `json:"id"`
    Type string `json:"type"`  // "reasoning"
    Text string `json:"text"`  // Summary from Codex
}

// CommandExecutionItem - has command AND output as separate fields
type CommandExecutionItem struct {
    ID               string `json:"id"`
    Type             string `json:"type"`  // "command_execution"
    Command          string `json:"command"`
    AggregatedOutput string `json:"aggregated_output"`
    ExitCode         *int   `json:"exit_code,omitempty"`
    Status           string `json:"status"`  // in_progress/completed/failed
}
```

### Log Storage
| Codex Field | What We Store | DB Column |
|-------------|---------------|-----------|
| `ReasoningItem.Text` | Summary (as-is from Codex) | `content` |
| `CommandExecutionItem.Command` | Command string | `command` (new) |
| `CommandExecutionItem.AggregatedOutput` | Full output | `content` |
| `CommandExecutionItem.ExitCode` | Exit code | `exit_code` (new) |
| `CommandExecutionItem.Status` | Status | `status` (new) |

### TUI Formatting
| Event Type | Display Format |
|------------|----------------|
| `reasoning` | Show summary text as-is (no prefix) |
| `command_execution` | Show command + output (NO `$` prefix) |
| `agent_message` | Show content as-is |

### Layout
- **Main view split:** Activity Feed (top) + Orchestrator Chat (bjunem)
- **Navigation:** Click agent in sidebar → transcript view; Escape or click project → main view
- **Activity Feed:** Status changes, completions, agent spawns
- **Orchestrator Chat:** Messages mentioning @june, ask/say content

---

## Open Items / Notes

1. **`june complete` command** — May be unnecessary for V0. Process exit detection is sufficient. Keep for future (multi-prompt agents) but don't require it.

2. **Plan updates needed:**
   - Task 4 regex already updated ✓
   - Task 6 (daemon wake-ups) needs update: wake any mentioned agent, not just `@june`
   - Task 6 needs update: auto-detect completion via process exit, not explicit `june complete`

---

## Agent Output Format Research (2025-12-26)

**Full details moved to design doc:** See `2025-12-25-super-orchestrator-v0-design.md` → "Agent Output Formats" section.

**Summary of decisions:**
- Codex and Claude Code both provide reasoning **summaries** (not full reasoning)
- Normalized schema covers: `thinking`, `message`, `tool_call`, `tool_result` event types
- New logs table columns: `agent_type`, `event_type`, `tool_name`, `command`, `exit_code`, `status`, `tool_use_id`, `raw_json`
- **V0:** Poll completed blocks only (no character-by-character streaming)
- **V1:** Add live streaming via delta forwarding to TUI

**Implementation plans should reference the design doc for schema details.**

---

## Next Steps

1. ~~Review Batch 5 (TUI Changes)~~ ✓
2. ~~Review Batch 4 (Daemon Wake-ups)~~ ✓
3. **Update implementation plans** with approved changes (delegate to Codex)
4. Begin implementation with subagent-driven development

## Review Complete

All batches approved. Ready for implementation.

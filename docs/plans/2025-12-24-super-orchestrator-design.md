# Super Orchestrator Design

**Status:** Draft (Updated)
**Created:** 2025-12-23
**Updated:** 2025-12-25

**Related docs:**
- [Flow Engine Design](./2025-12-25-otto-flow-engine-design.md) - Harness-driven flow, skill resolution, config
- [Tasks Design](./2025-12-24-tasks-design.md) - Task tracking and state
- [Skill Injection Design](./2025-12-24-skill-injection-design.md) - Re-injecting skills on wake-up

## Problem Statement

When using Claude Code or Codex as an orchestrator for multi-agent work, three problems emerge:

1. **Skill enforcement fails** - Despite "YOU MUST" instructions, Claude rationalizes skipping skills. Prompting doesn't work.

2. **Manual polling burden** - When Codex orchestrates and spawns agents, it must manually poll `otto messages` to see responses. Agents can't wake each other or the orchestrator.

3. **Compaction destroys context** - After compaction, the orchestrator loses bootstrapped skills, todo state, and plan progress. It becomes "dumb" and has to be re-taught everything.

**Core insight:** The orchestrator needs to be *event-driven* and *compaction-resilient*. When an agent reports back, the orchestrator should be automatically woken up with its full context re-injected.

## Design Principles

1. **Orchestrator is an agent** - Not outside the system. Has the ID `@otto`, uses `say` like everyone else.

2. **One privilege: spawning** - Only the orchestrator can spawn new agents. Other agents request help via `say @otto I need a specialist for X`.

3. **@mentions trigger wake-ups** - When agent A posts `say @B ...`, agent B gets woken up automatically. No manual polling.

4. **Wake-up = injection point** - Every time an agent (including orchestrator) is woken up, we re-inject relevant context: skills, todos, plan state.

5. **State lives externally** - State survives compaction because it's stored in files and SQLite, not LLM memory.

6. **Claude or Codex** - Either can be the orchestrator. User chooses based on task (Codex for reliability, Claude for conversation).

## Decisions Made

| Topic | Decision |
|-------|----------|
| Orchestrator ID | `@otto` - short, matches tool name, `@` prefix disambiguates from CLI |
| Branches vs worktrees | Branches are the logical unit. Worktrees are implementation detail. |
| Database | Single global DB for all projects/branches |
| TUI + Daemon | `otto` is both - single process, always running |
| Human @mention | Through orchestrator - agents talk to `@otto`, it escalates to `@human` |
| Cross-project | Permissive but blind - agents can message any project, but only know what orchestrator tells them |
| Gate/capability system | Deferred - revisit if skill enforcement remains a problem after Phase 1-2 |

## Work Tracking

See [Tasks Design](./2025-12-24-tasks-design.md) for details on:
- TODO.md vs tasks table distinction
- Tasks schema with derived state
- Hierarchical task structure

## State & Re-injection

After compaction, orchestrators and agents lose context. The wake-up mechanism re-hydrates them:

| State | Where it lives | How it's re-injected |
|-------|----------------|---------------------|
| **Skills** | Files (`.claude/commands/`, skill files) | Derived from task position, injected on wake-up |
| **Tasks/progress** | SQLite (`otto.db` tasks table) | Queried and injected on wake-up |
| **Plans/design docs** | Files (`docs/plans/*.md`) | Re-read and injected on wake-up |
| **Messages/history** | SQLite (`otto.db` messages table) | Summarize recent activity |

Example wake-up injection:

```
You are the orchestrator. Here's your current state:

## Active Plan
[contents of docs/plans/2025-12-24-xyz.md]

## Current Tasks
- [x] Task 1: Add user model (@backend, completed)
- [ ] Task 2: Add login endpoint (@backend, in spec review)  <-- current
- [ ] Task 3: Add JWT middleware (pending)

## Recent Activity
- Agent @backend completed Task 1
- Agent @backend submitted Task 2 for spec review

## Current Skill
You are using `subagent-driven-development`. Task 2 implementation is complete,
spec review is next. Dispatch spec-reviewer subagent.
```

## Compaction Resilience

Both Claude Code and Codex compact in headless/detached mode:

- **Claude Code:** Triggers around ~75% context usage. Has `PreCompact` hook we can use to snapshot state before compaction.
- **Codex CLI:** Token-based threshold (`model_auto_compact_token_limit`). Emits compaction events in structured output (v0.64.0+).

**Strategy:**
1. Use Claude Code's `PreCompact` hook to snapshot state before compaction
2. Detect session resumption after compaction
3. Re-inject full context on wake-up (skills, todos, plan)

## Communication Model

### Unified `say` command

All agents (including orchestrator) communicate via `say`:

```bash
# Agent posts to shared channel
otto say "I've finished the mockups"

# Agent mentions another agent (triggers wake-up)
otto say "@backend the API spec is ready"

# Agent asks orchestrator for help
otto say "@otto I need a specialist for database work"
```

### Cross-project addressing

Agents can communicate across branches and projects:

```bash
# Within same project, different branch
otto say "@feature-x/designer check this"

# Across projects
otto say "@other-project/main/otto need your input"
```

Format: `@project/branch/agent` or `@branch/agent` (same project) or `@agent` (same branch).

### `prompt` becomes internal

The `prompt` command becomes an internal mechanism - how the event bus wakes agents:

```bash
# Internal: event bus wakes agent with context injection
otto prompt backend "[injected context]\n\nMessage from @designer: the API spec is ready"
```

Users don't need to use `prompt` directly - they use `say` with @mentions.

## Agent Lifecycle

### Spawning
Only `@otto` (orchestrator) can spawn agents. Other agents request via `say @otto`.

### Status transitions
```
spawn → busy → blocked (via ask) → busy (via answer) → complete
                  ↓
               failed (process dies)
                  ↓
              archived (via archive command)
```

### Killing agents
Orchestrator can forcibly kill agents:
```bash
otto kill <agent-id>      # Terminate agent process
otto kill <agent-id> -f   # Force kill (SIGKILL)
```

Hung/stuck agents detected via PID monitoring (existing `cleanupStaleAgents`).

### Interrupting agents
Wake-ups queue by default. Orchestrator can interrupt mid-task:
- Kill agent
- Respawn with new context/task injected

## Event Bus Architecture

SQLite-backed event log enables:

1. **@mention detection** - Parse messages for mentions, trigger wake-ups
2. **Status change notifications** - Agent goes blocked/complete, orchestrator notified
3. **Cross-project awareness** - Single event bus spans all projects/branches

### Event Types

```
message_posted    - Agent posted a message
agent_blocked     - Agent called `ask`, waiting for input
agent_completed   - Agent called `complete`
agent_failed      - Agent process died unexpectedly
compaction_start  - Agent about to compact (Claude Code PreCompact hook)
```

### Watcher Process

`otto` itself is the watcher - both TUI and daemon in one:
- Monitors global DB for events
- Triggers wake-ups on @mentions
- No TUI running = no automatic wake-ups (fall back to manual)

## Unified TUI Vision

One `otto` command as the control center for all projects/branches:

```
+---------------------+---------------------------------------------+
| Channels            | otto/main                                   |
|                     |---------------------------------------------|
| otto/main           | @otto: Spawning designer agent              |
|   @otto *           | @designer: Working on mockups...            |
|   @designer *       | @designer: Mockups complete! See figma link |
|   @backend o        | @backend: I'm blocked, need API spec        |
|                     |                                             |
| otto/feature-x      |                                             |
|   @otto *           |                                             |
|   @migrator *       |                                             |
|                     |                                             |
| other-project/main  |---------------------------------------------|
|   @otto o           | > Type message here...                      |
+---------------------+---------------------------------------------+
```

**Features:**
- Left sidebar shows `project/branch` with agents underneath
- Click channel to view its messages
- Chat input to message orchestrator or @mention specific agents
- Status indicators: `*` busy, `?` blocked, `o` complete, `x` failed
- Human can be a participant in any channel
- Archived agents in collapsible section (already implemented)

## Human Participation

### Human identity
Future: On first run, `otto` prompts for human's name. Stored in config.

### @mentioning the human
For now: Orchestrator can `@human` to escalate to the human. Later we'll add configurable names.

Agents talk to `@otto`, orchestrator decides whether to escalate to `@human`.

## Database Schema

### Global DB location
```
~/.otto/otto.db   # Single global database
```

### Schema updates

Add `project` and `branch` columns to existing tables:

```sql
ALTER TABLE agents ADD COLUMN project TEXT;
ALTER TABLE agents ADD COLUMN branch TEXT;

ALTER TABLE messages ADD COLUMN project TEXT;
ALTER TABLE messages ADD COLUMN branch TEXT;
```

### Tasks table

See [Tasks Design](./2025-12-24-tasks-design.md) for the full tasks schema. Key points:
- Hierarchical with `parent_id`
- State derived from `assigned_agent` + `completed_at` (no status field)
- Linked to plan files

## Implementation Phases

### Phase 1: Event Bus + Wake-up

- Add event logging to message posting
- Implement @mention detection and parsing
- Create wake-up mechanism via `otto prompt` with context injection
- Wire into existing `say`, `ask`, `complete` commands
- Add `kill` command for forcible termination

### Phase 2: Compaction Resilience

- Add todos table to schema
- Implement hierarchical todos with `parent_id`
- Implement state snapshot on PreCompact hook
- Build context re-injection for wake-ups
- Test with both Claude and Codex as orchestrator

### Phase 3: Unified TUI + Global DB

- Migrate to global DB (`~/.otto/otto.db`)
- Add `project` and `branch` columns
- Extend scope detection to handle multiple projects/branches
- Update TUI to show cross-project channels
- Add chat input for human participation
- Single `otto` command (no `watch` subcommand)

## Skill Flow & Injection

See [Skill Injection Design](./2025-12-24-skill-injection-design.md) for details on:
- The superpowers skill chain (brainstorming → planning → execution → finishing)
- How skills are re-injected on wake-up
- Mapping tasks to skills
- Wake-up context assembly

Key insight: The task hierarchy encodes skill flow position. "Which skill am I in?" is derived from "what's my current task?"

## Open Questions

See open questions in the related design docs:
- [Tasks Design - Open Questions](./2025-12-24-tasks-design.md#open-questions)
- [Skill Injection Design - Open Questions](./2025-12-24-skill-injection-design.md#open-questions)

## Appendix: Previous Approaches Considered

### A. Stricter prompting
Add more emphatic instructions to CLAUDE.md. Testing shows this doesn't work—Claude already has "YOU MUST" instructions and ignores them when convenient.

### B. Codex as orchestrator
Replace Claude with Codex for the orchestrator role. Codex follows instructions more reliably. Trade-off: less conversational, may feel less delightful for design discussions.

### C. Injection layer (Claude + enforcer)
Keep Claude as the conversational interface. A thin layer preprocesses input and injects skill prompts before Claude sees it. Claude doesn't know it's being managed. Preserves delight, adds reliability.

### D. Structural enforcement (hooks/gates)
Use Claude Code's hook system to block tool calls until skills are invoked. Hard gate, but may be annoying for simple tasks.

### E. Event-driven system with lightweight classifiers
No "super orchestrator" brain. Instead: event bus + small classifiers that detect intent and inject/route accordingly. Deterministic enforcement, zero LLM token cost for the enforcement layer.

**Current direction:** Start with event bus and wake-up mechanism (Phase 1), layer in compaction resilience (Phase 2), then unified TUI (Phase 3). Gate/capability system deferred - revisit if needed.

## Appendix: Compaction Research (2025-12-24)

**Claude Code:**
- Triggers ~75% context usage (varies by version)
- `PreCompact` hook fires before compaction
- `SessionStart(compact)` matcher detects post-compaction resume
- Headless mode (`claude -p`) still compacts

**Codex CLI:**
- Token-based threshold (`model_auto_compact_token_limit`)
- Event stream notifications for compaction (v0.64.0+)
- `notify` command for turn completion (limited event coverage)
- `codex exec` (headless) still compacts

Both compact in headless/detached mode - this is a real problem that needs solving.

## Appendix: Existing Infrastructure (as of 2025-12-24)

Recent merge from `agent-archiving` branch provides:
- `otto archive <agent-id>` - Soft-delete completed/failed agents
- `ArchivedAt` field on agents
- `--all` and `--archive` flags on `status` command
- TUI collapsible "Archived" section
- Auto-unarchive on `prompt`/`attach`

This provides foundation for agent lifecycle management.

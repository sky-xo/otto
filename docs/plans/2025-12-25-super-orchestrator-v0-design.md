# Super Orchestrator V0 Design

**Status:** Ready for Review
**Created:** 2025-12-25

## Summary

This document captures the refined architecture decisions from the 2025-12-25 brainstorming session. It resolves the "split control planes" issue identified in architectural reviews and defines a simpler V0 approach.

## Core Problems Being Solved

1. **Claude skips rules/steps** - Despite strict instructions, Claude rationalizes skipping skills
2. **Codex forgets to poll** - When orchestrating, Codex must manually check for agent responses
3. **Compaction destroys context** - Skills, todos, and progress lost after compaction
4. **Need reliable multi-agent orchestration** - Coordinate Claude and Codex agents

## V0 Architecture Decision

### The Key Insight

The earlier designs had **two control planes** in tension:
- Super Orchestrator: `@otto` as central agent with event bus
- Flow Engine: Go harness controlling the flow

**V0 Resolution:** Use Codex as orchestrator (follows instructions), daemon handles wake-ups and context injection. No hard gates in V0 - just soft reminders.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ otto (TUI + Daemon)                                             │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Event Bus                                                   ││
│  │  - Detects @mentions in messages                            ││
│  │  - Detects agent completion/failure                         ││
│  │  - Detects compaction events                                ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Wake-up Handler                                             ││
│  │  - Builds context injection (state + new messages)          ││
│  │  - Calls `otto prompt @otto "[context]"`                    ││
│  │  - Re-injects full skills after compaction                  ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Orchestrator Agent (Codex, detached mode)                   ││
│  │  - Runs the skill flow (brainstorm → plan → impl → ...)     ││
│  │  - Spawns subagents via `otto spawn`                        ││
│  │  - Communicates via `otto say`                              ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Key Decisions

| Topic | Decision |
|-------|----------|
| **Primary interface** | TUI (`otto`) - will rename `otto watch` to just `otto` |
| **V0 orchestrator model** | Codex (reliable, follows instructions) |
| **Enforcement approach** | Soft reminders (Option C) - inject context, no hard gates |
| **Compaction handling** | Detect `context_compacted` event, re-inject full skills |
| **Agent wake-ups** | Daemon auto-wakes on @mentions and completions |
| **Headless mode** | `otto --headless` for future API/integrations (not V0 priority) |

## Modes of Operation

| Mode | How it works |
|------|--------------|
| **TUI mode** (`otto`) | Primary interface. Daemon + TUI in one process. Shows channels, chat with orchestrator. |
| **CLI mode** (from orchestrator) | Orchestrator uses `otto spawn`, `otto say`, etc. These are the API. |
| **Headless** (`otto --headless`) | Future: daemon without TUI, for Slack/API integrations. |

## Context Injection

### On Normal Wake-up (no compaction)

Only inject NEW information the orchestrator hasn't seen:

```
[New messages since last wake]
@impl-1: implementation complete, tests passing
@reviewer: LGTM, approved

[State summary - always included, cheap]
Phase: implementation | Task 4/7: Add password validation
```

**Message count is naturally small.** Between wake-ups, there are typically only 2-5 messages:
- 1 completion/failure message (the trigger)
- Maybe 1-2 status updates via `otto say`
- Maybe a question if agent got stuck

No message limits needed for V0.

### After Compaction

Codex loses skills after compaction. Re-inject full context:

```
[Skills - re-injected after compaction]
You are the orchestrator using subagent-driven-development.
Follow this flow: brainstorming → planning → implementation → review → finishing
...

[State summary]
Phase: implementation | Task 4/7: Add password validation

[New messages]
@impl-1: implementation complete
```

### Detecting Compaction (Codex)

Parse the JSON output stream for `context_compacted` event:

```json
{"type":"context_compacted"}
```

When detected, set flag in DB. Next wake-up re-injects full skills.

## Codex JSON Output Format

From research of `github.com/openai/codex` source:

### Event Types

| Event | Description |
|-------|-------------|
| `thread.started` | Session start, includes `thread_id` |
| `turn.started` | Turn begins |
| `turn.completed` | Turn ends, includes token usage |
| `turn.failed` | Turn failed with error |
| `item.started/updated/completed` | Item lifecycle |
| `context_compacted` | **Compaction happened** |
| `warning` | General warning (e.g., post-compaction warning) |
| `error` | Fatal error |

### Item Types (in `item.*` events)

| Item Type | Fields | Display As |
|-----------|--------|------------|
| `reasoning` | `text` | Collapsed thinking (gray) |
| `agent_message` | `text` | Message bubble |
| `command_execution` | `command`, `aggregated_output`, `exit_code`, `status` | Code block |
| `file_change` | file details | Diff view |
| `mcp_tool_call` | tool call details | Tool badge |
| `todo_list` | plan items | Checklist |

### Implementation: Parse and Store

Currently we store raw JSON. Improve to:

1. Parse each JSON line as it arrives
2. Detect event types (especially `context_compacted`)
3. Extract content for nice display
4. Store structured data in DB

## TUI Design

### Main View (click project name)

```
┌──────────────┬──────────────────────────────────────────────┐
│ Projects     │ Activity Feed (agent-to-agent, status)       │
│              │ @impl-1 completed task 3                     │
│ otto/main    │ @reviewer: LGTM on the API changes           │
│   @impl-1 *  ├──────────────────────────────────────────────┤
│   @reviewer  │ Chat with Orchestrator                       │
│              │                                              │
│              │ You: build a login page                      │
│              │ @otto: Let's brainstorm. What auth...        │
│              │                                              │
│              │ > [type here]                                │
└──────────────┴──────────────────────────────────────────────┘
```

- **Top panel:** Activity feed (agent status, agent-to-agent messages)
- **Bottom panel (larger):** Chat with orchestrator
- **Sidebar:** Click project = orchestrator chat, click agent = transcript

### Agent Transcript View (click agent)

Shows parsed, nicely formatted output:
- Reasoning as collapsed gray text
- Commands as code blocks with output
- Messages as bubbles

## Event Routing

| Event | Routes to... |
|-------|--------------|
| `@mention` of specific agent | Daemon auto-wakes that agent |
| Agent `complete` | Daemon notifies orchestrator |
| Agent `failed` (crash/timeout) | Daemon notifies orchestrator with error details |
| Agent `ask` (blocked) | Orchestrator decides: answer, escalate, or spawn specialist |
| `@human` | Human via TUI notification |
| `@otto` | Orchestrator |

### Agent Failure Handling

When an agent crashes, times out, or exits with an error, the **daemon detects it** (not the orchestrator). The daemon then notifies the orchestrator with details:

```
@otto: @impl-1 failed (exit code 1, error: process killed after 10min timeout)
```

The orchestrator then decides what to do:
- Retry with the same agent
- Spawn a new agent
- Escalate to human
- Mark the task as blocked and move on

This keeps reliable detection in the daemon (code) and smart decisions in the orchestrator (LLM).

## CLI Commands

### Core Commands

```bash
# Prompt orchestrator (default) or specific agent
otto prompt "build a login page"              # → orchestrator
otto prompt impl-1 "try a different approach"  # → specific agent

# Agent communication
otto say "status update"                       # Post to channel
otto say "@otto I need help"                   # Mention triggers wake-up
otto ask "what should I do next?"              # Block until answered
otto complete "finished the task"              # Mark agent complete

# Spawning
otto spawn codex "implement feature X"         # Spawn agent (orchestrator only)
otto spawn codex "task" --name impl-1          # With custom name

# Management
otto kill impl-1                               # Terminate agent
otto kill impl-1 -f                            # Force kill (SIGKILL)
otto status                                    # Show all agents
otto messages                                  # Show message stream
```

### Agent Lifecycle

```
spawn → busy → blocked (via ask) → busy (via answer) → complete
                  ↓
               failed (process dies)
                  ↓
              archived (via archive command)
```

## TODO.md vs Tasks

Two distinct concepts:

| Concept | TODO.md | tasks table |
|---------|---------|-------------|
| **Purpose** | What work exists (backlog) | What's happening now (execution) |
| **Maintained by** | Human | Orchestrator/agents |
| **Storage** | File, version-controlled | SQLite, ephemeral |
| **Lifespan** | Long-lived, per-branch | Per-execution |
| **Example** | "Add authentication feature" | "Task 2: Add login endpoint" |

**The flow:**
1. Human picks item from TODO.md: "Add authentication"
2. Orchestrator brainstorms → creates design doc
3. Orchestrator writes plan → creates task breakdown in `tasks` table
4. Orchestrator executes → updates task status via agent assignments
5. Orchestrator finishes → human updates TODO.md

TODO.md is the durable record. Tasks table is ephemeral execution state.

## V0 Implementation Scope

### What V0 Includes

1. **Tasks table** - Hierarchical tree of tasks (root = workflow, children = phases/items)
2. **Event detection** - Parse messages for @mentions, detect completions and failures
3. **Auto-wake** - On event, call `otto prompt "[context]"` (defaults to orchestrator)
4. **Failure detection** - Daemon monitors agent processes, notifies orchestrator on crash/timeout
5. **Context builder** - Assemble: state summary (from tasks DB) + new messages
6. **Compaction detection** - Parse `context_compacted` from JSON stream
7. **Skill re-injection** - On compaction, re-inject full orchestrator skills + state from DB
8. **Codex output parsing** - Store structured data, display nicely in TUI

### What V0 Does NOT Include

- Hard gates on flow transitions (add later if soft reminders fail)
- Claude as orchestrator (add later with gates)
- Multiple root tasks per branch (V0 uses single root task per branch)
- Artifact checks by daemon (orchestrator handles)
- External skill resolution (hardcode superpowers flow for V0)

## Data Model

### Core Concept: Tasks as a Tree

Everything is a **task**. A "workflow" is just a root-level task:

```
"Build auth system"           (parent_id: NULL)  ← root task = "workflow"
├── "brainstorming"           (parent_id: above)
├── "planning"                (parent_id: above)
├── "implementation"          (parent_id: above)
│   ├── "Add login form"      (parent_id: implementation)
│   └── "Add password reset"  (parent_id: implementation)
├── "review"                  (parent_id: above)
└── "finishing"               (parent_id: above)
```

This simplifies the model:
- **No separate workflows table** - a workflow is just a root task
- **Workflow state** survives agent crashes (lives in DB, not agent context)
- **After compaction**, we query tasks to reconstruct state

### Schema

```sql
-- Tasks: hierarchical tree (root tasks are "workflows")
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    parent_id TEXT,               -- NULL for root tasks (workflows)
    name TEXT NOT NULL,           -- 'Build auth system', 'brainstorming', etc.
    assigned_agent TEXT,          -- FK to agents (nullable)
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Track compaction per agent
ALTER TABLE agents ADD COLUMN compacted_at TIMESTAMP;

CREATE INDEX idx_tasks_parent ON tasks(parent_id);
```

### Derived State

**Root tasks** = `SELECT * FROM tasks WHERE parent_id IS NULL` (these are "workflows")
**Current phase** = first child of root task that's not completed
**Task status** = derived from `assigned_agent` + `completed_at`:
- `pending` = assigned_agent IS NULL, completed_at IS NULL
- `active` = assigned_agent NOT NULL, completed_at IS NULL
- `completed` = completed_at NOT NULL

### V0 Pattern

V0 assumes one root task per branch:
- `GetOrCreateRootTask(branch, name)` returns the single root task
- Child tasks are phases, grandchildren are implementation items
- Later, we can support multiple root tasks per branch for parallel workstreams

## Open Questions for V1

1. **Claude orchestrator support** - Add hard gates when Claude is orchestrator?
2. **Multiple workflows per branch** - UI for managing parallel workstreams
3. **Artifact verification** - Should daemon verify artifacts before advancing?
4. **Cross-project addressing** - Format: `@project/branch/agent` or `@branch/agent` (same project) or `@agent` (same branch)
5. **Agent preferences config** - Which agent type for which task type (e.g., always use Claude for brainstorming)
6. **Harness-driven flow** - If soft reminders fail, add YAML-defined flow with hard gates

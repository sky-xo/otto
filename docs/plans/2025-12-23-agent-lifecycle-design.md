# Agent Lifecycle & Kill Command Design

> **Status:** Approved (Updated with interrupt/status changes)

## Overview

Simplify agent lifecycle: agents table only contains actively running processes. When agents complete or exit, they're deleted immediately. Messages table is the source of truth for history.

## Agent Statuses

Three statuses for active agents:

| Status | Meaning | Orchestrator Action |
|--------|---------|---------------------|
| `busy` | Process running, actively thinking | Wait or interrupt |
| `idle` | Ready for input (interrupted or between turns) | Send prompt if needed |
| `blocked` | Asked a question, needs reply to continue | Must respond |

### Status Transitions

```
spawn ──────────────────────────────────────┐
                                            ▼
                                    ┌──────────────┐
         ┌─────────────────────────▶│     busy     │◀──────────────────┐
         │                          └──────┬───────┘                   │
         │                                 │                           │
         │      ┌──────────────────────────┼────────────────────┐      │
         │      │                          │                    │      │
         │      ▼                          ▼                    ▼      │
   ┌───────────────┐              ┌──────────────┐       ┌───────────────┐
   │    blocked    │              │     idle     │       │   (deleted)   │
   │  (otto ask)   │              │ (interrupted │       │  on complete  │
   └───────────────┘              │  or finished)│       │   or kill     │
         │                        └──────────────┘       └───────────────┘
         │                                │
         │         otto prompt            │
         └────────────────────────────────┘
```

## Mental Model

```
agents table  = running processes (ephemeral)
messages table = full history (permanent)
```

Like Claude subagents via Task tool - they do work, return results, and disappear. The result (completion message) is what matters, not the agent metadata.

## Lifecycle Changes

### Current Behavior
```
spawn → working → complete → status='done' (row persists)
                → exit     → status='done'/'failed' (row persists)
```

### New Behavior
```
spawn → working → complete → POST message → DELETE row
                → exit     → POST message → DELETE row
                → kill     → POST message → DELETE row
```

## Changes Required

### 1. `otto complete` (complete.go)
After posting completion message, DELETE agent row instead of updating status.

### 2. spawn.go exit handling
When process exits (success or failure):
- Post exit message to stream
- DELETE agent row

### 3. watch.go dead PID detection
When detecting dead PID:
- Post "process died" message
- DELETE agent row

### 4. New command: `otto interrupt <agent-id>`
```bash
otto interrupt foo
```
- Look up agent, get PID
- Send SIGINT to process (graceful stop)
- Process exits cleanly, session preserved
- Update status to `idle`
- Agent can be resumed with `otto prompt`

**Why SIGINT works:** Codex handles SIGINT gracefully - stops current turn but preserves the thread. Resume with `codex exec resume <thread_id>`.

### 5. New command: `otto kill <agent-id>`
```bash
otto kill foo
```
- Look up agent, get PID
- Send SIGTERM to process (hard stop)
- Post "[foo] KILLED by orchestrator" message
- DELETE agent row
- Print confirmation

### Interrupt vs Kill

| Command | Signal | Session | Agent Row | Use Case |
|---------|--------|---------|-----------|----------|
| `otto interrupt` | SIGINT | Preserved | Kept (status=idle) | "Pause, I want to redirect you" |
| `otto kill` | SIGTERM | Destroyed | Deleted | "Stop completely, you're done" |

## Message Format

```
[agent-id] COMPLETE: summary text     # from otto complete
[agent-id] EXITED: success            # process exit code 0
[agent-id] FAILED: error message      # process exit code != 0
[agent-id] DIED: process not found    # PID gone unexpectedly
[agent-id] INTERRUPTED                # otto interrupt (no deletion)
[agent-id] KILLED: by orchestrator    # otto kill command
```

## Edge Cases

### Resuming completed agents
Not supported. If agent completed, that task is done. Start a new agent for new tasks.

### Orphaned messages
Messages from deleted agents remain in messages table. This is correct - they're history.

### Race conditions
If agent completes while being killed, either path results in deletion. Message posted will be whichever happened first.

## What We're NOT Building (YAGNI)

- `otto clean` - Not needed if auto-cleanup works
- `otto list` - Not needed for current workflows
- Soft delete - Messages table has history
- Agent status history - Not needed

## Files to Modify

- `internal/cli/commands/complete.go` - DELETE after posting
- `internal/cli/commands/spawn.go` - DELETE on exit, use new statuses
- `internal/cli/commands/watch.go` - DELETE on dead PID
- `internal/cli/commands/ask.go` - Set status to `blocked`
- `internal/cli/commands/interrupt.go` - New command (SIGINT, status=idle)
- `internal/cli/commands/kill.go` - New command (SIGTERM, DELETE)
- `internal/repo/agents.go` - Add DeleteAgent function

## Codex Improvements

Based on analysis of external-subagents, also add to Codex invocation:
- `--skip-git-repo-check` flag - reduces startup overhead

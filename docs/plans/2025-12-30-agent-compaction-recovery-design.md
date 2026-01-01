# Agent Compaction Recovery Design

**Status:** Draft
**Created:** 2025-12-30
**Related:** Hypothesis 6 in docs/HYPOTHESIS.md

## Problem Statement

When Claude Code subagents compact mid-task, recovery is painful:

1. Subagent loses internal context (reasoning, intermediate state)
2. In vanilla Claude, recovery requires manual intervention:
   - `/export` the conversation
   - `/clear` to start fresh
   - Tell new Claude to read export "from the bjunem up"
   - New Claude figures out where things left off
3. This workflow is tedious and error-prone

**Source:** Vibes Coding Group discussion about common subagent compaction failures.

## Proposed Solution

June can provide automatic recovery by leveraging:
- Task list as checkpoint (agents mark tasks complete as they go)
- Activity logs persisted to SQLite
- Orchestrator process independent of subagent processes

### Recovery Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Normal Execution                          │
├─────────────────────────────────────────────────────────────┤
│  1. Orchestrator assigns tasks to agent                      │
│  2. Agent works through tasks, marking complete as it goes   │
│  3. June logs agent activity to SQLite                       │
│  4. Task list in SQLite = persistent checkpoint              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ (agent compacts/crashes)
┌─────────────────────────────────────────────────────────────┐
│                    Recovery Triggered                        │
├─────────────────────────────────────────────────────────────┤
│  5. Orchestrator detects agent failure                       │
│  6. Queries task list: which tasks complete vs pending       │
│  7. Queries logs: last N lines of agent activity             │
│  8. Spawns new agent with recovery context                   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    New Agent Continues                       │
├─────────────────────────────────────────────────────────────┤
│  9. New agent receives: pending tasks + recent logs          │
│  10. Checks codebase state (what's already done)             │
│  11. Continues from first incomplete task                    │
│  12. Retry limits prevent infinite loops                     │
└─────────────────────────────────────────────────────────────┘
```

## Design Details

### 1. Agent Health Monitoring

**How do we detect compaction/failure?**

Options:
- **Process status:** Check if agent process is still running (`june status` already does this)
- **Heartbeat:** Agent periodically reports "still alive" (adds complexity)
- **Timeout:** If agent hasn't reported progress in N minutes, assume dead (risk of false positives)
- **Exit detection:** Monitor for process exit, check if tasks incomplete

**Recommendation:** Start with process exit detection + incomplete tasks check. If process exits and tasks remain incomplete, trigger recovery.

### 2. Task List as Checkpoint

The existing task system (`repo.Tasks`) already tracks:
- Task name and assignment
- Task status (pending, in_progress, complete)
- Task result

**Required behavior:** Agents must mark tasks complete as they finish each one. This is already the intended workflow, but the injected skill should emphasize it.

**Recovery query:**
```sql
SELECT * FROM tasks
WHERE project = ? AND branch = ? AND assigned_agent = ?
ORDER BY sort_index
```

This gives us: what's done, what's pending, what was in progress when agent died.

### 3. Log Retrieval for Recovery Context

The existing logs table captures agent activity. For recovery, we need the last N lines from the dead agent.

**Recovery query:**
```sql
SELECT * FROM logs
WHERE project = ? AND branch = ? AND agent_name = ?
ORDER BY id DESC
LIMIT ?
```

**How much context?** Start with last 200 lines. This should capture recent tool calls, decisions, and errors without overwhelming the new agent.

### 4. Recovery Prompt Generation

When spawning a recovery agent, construct a prompt like:

```
You are continuing work that a previous agent started. The previous agent
encountered an issue and stopped.

## Task Status
Completed:
- [x] Task 1: Implement auth module
- [x] Task 2: Add unit tests

In Progress (continue from here):
- [ ] Task 3: Integration tests

Pending:
- [ ] Task 4: Documentation

## Recent Activity from Previous Agent
[Last 200 lines of logs]

## Instructions
1. Check the current codebase state to see what's already been done
2. Continue from Task 3
3. Mark tasks complete as you finish them
```

### 5. Retry Limits and Backoff

To prevent infinite loops on tasks that consistently cause compaction:

- **Max retries per task:** 3 attempts
- **Backoff strategy:** On retry, suggest breaking the task into smaller subtasks
- **Escalation:** After max retries, mark task as failed and alert orchestrator/human

**Tracking retries:** Add `retry_count` column to tasks table, or track in a separate recovery_attempts table.

### 6. Triggering Recovery

**Automatic (recommended):**
- Orchestrator monitors spawned agents
- On agent exit with incomplete tasks, automatically spawn recovery agent
- No human intervention required for normal recovery

**Manual fallback:**
- If auto-recovery fails (max retries), notify human
- Provide command: `june recover <agent-name>` to manually trigger recovery

## Implementation Plan

### Phase 1: Foundation
1. Add agent exit detection to orchestrator
2. Add `june logs <agent> --tail N` command for retrieving recent logs
3. Add retry tracking (column or table)

### Phase 2: Recovery Logic
4. Implement recovery prompt generation (tasks + logs)
5. Add auto-spawn on agent failure
6. Add retry limits and max-retry handling

### Phase 3: Polish
7. TUI indication of recovery events
8. Manual `june recover` command as fallback
9. Configurable retry limits and log context size

## Open Questions

1. **How do we distinguish compaction from normal completion?**
   - Compaction: process exits, tasks incomplete
   - Normal: process exits, all tasks complete
   - Edge case: process exits mid-task but task was nearly done

2. **Should recovery be opt-in or default?**
   - Recommendation: default on, with flag to disable

3. **How much log context is right?**
   - Too little: new agent lacks context
   - Too much: overwhelms new agent's context window
   - Start with 200 lines, make configurable

4. **What if the task itself is too big?**
   - Repeated compaction on same task suggests task is too large
   - Recovery could suggest: "Break this task into smaller subtasks"
   - Or: orchestrator could attempt automatic task decomposition

## Alternatives Considered

**1. Full transcript capture**
- Capture entire agent conversation, replay to new agent
- Pros: Perfect context recovery
- Cons: Huge context, may not fit, expensive
- Verdict: Task list + recent logs is sufficient and practical

**2. Periodic checkpointing to file**
- Agent writes checkpoint file every N minutes
- Pros: Fine-grained recovery
- Cons: Extra complexity, agents must implement checkpoint logic
- Verdict: Task completion is already a natural checkpoint

**3. No automatic recovery**
- Just provide better tools for manual recovery
- Pros: Simpler, less magic
- Cons: Doesn't solve the core pain point
- Verdict: Automatic is the goal, manual is the fallback

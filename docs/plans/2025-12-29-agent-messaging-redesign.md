# Agent Messaging Redesign

**Status:** Ready
**Date:** 2025-12-29

## Problem

Feedback from Claude using Otto to run Codex subagents revealed several UX issues:

1. **Finding results is confusing** - Agent conclusions are buried in 600+ lines of logs
2. **No clean "get result" command** - Had to parse through raw output
3. **`otto say` purpose is unclear** - Rarely used, overlaps with stdout
4. **Skill doc has wrong info** - References `block: true` param that doesn't exist

## Solution

Simplify agent messaging and make results easy to retrieve:

1. **Add `otto dm`** - Explicit agent-to-agent direct messaging
2. **Remove `otto say`** - Consolidate into `dm`, stdout handles broadcast
3. **Enhance `otto peek`** - Show full log (capped) when agent completes
4. **Update skill doc** - Fix incorrect info, document new commands

## Design

### `otto dm` Command

Direct message between agents. Wakes the target agent.

```bash
otto dm --from impl-1 --to reviewer "API contract is ready"
otto dm --from impl-1 --to feature/login:frontend "your branch needs rebase"
```

**Flags:**
- `--from` (required): Sender agent name
- `--to` (required): Recipient, supports cross-branch addressing

**Addressing formats for `--to`:**
- `agent` → same project/branch
- `branch:agent` → same project, different branch
- `project:branch:agent` → different project

**Behavior:**
1. Resolve addresses using existing `parseMentions()` logic
2. Store in `messages` table (type: `dm`)
3. Wake target agent (daemon handles this)

### Enhanced `otto peek`

Behavior changes based on agent status:

| Agent Status | `peek` Behavior |
|--------------|-----------------|
| busy/blocked | Incremental (cursor-based, existing behavior) |
| complete/failed | Full log, capped at 100 lines |

When showing capped output:
```
[agent complete - showing last 100 lines]

... log content ...

[full log: 347 lines - run 'otto log reviewer' for complete history]
```

### Remove `otto say`

Delete the command entirely. Use cases migrate to:
- Agent-to-agent comms → `otto dm`
- Status updates → stdout (captured in logs)
- @mentions for wake-ups → `otto dm`

### Skill Doc Updates

Fix `.claude/skills/otto-orchestrate/SKILL.md`:
- Remove references to `block: true` (doesn't exist)
- Remove `otto say` documentation
- Add `otto dm` documentation
- Update workflow examples

## Out of Scope

- **Name collision bug** - Agent names conflict across projects. Tracked in TODO.md, separate fix needed.
- **`otto wait` command** - Future consideration, not needed with current notification model.

## Implementation Order

1. Add `otto dm` command
2. Enhance `otto peek` for complete agents
3. Remove `otto say` command
4. Update skill doc

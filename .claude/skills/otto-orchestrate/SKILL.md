---
name: otto-orchestrate
description: Use when spawning Codex agents via Otto to delegate implementation work.
---

# Otto Orchestration

Spawn and coordinate Codex agents for implementation work.

## Orchestrator Role

You coordinate, you don't implement. Dispatch agents to do the work.

## Spawning Agents (IMPORTANT)

**Use `run_in_background: true` with the Bash tool - NOT `--detach`:**

```
Bash tool call:
  command: otto spawn codex "task description" --name <name>
  run_in_background: true
```

This gives you **automatic notification** when the agent completes. No polling needed.

Then use `BashOutput` to retrieve results when ready:
```
BashOutput tool call:
  bash_id: <id from spawn>
  block: true  # waits for completion
```

**Why not --detach?** With `--detach` you must manually poll with `otto status` and `otto peek`. With `run_in_background`, Claude Code notifies you automatically.

### Spawn Options

- `--name <name>` - Readable ID (e.g., `planner`, `impl-1`)
- `--files <paths>` - Attach relevant files (keep minimal)
- `--context <text>` - Extra context

## Monitoring (IMPORTANT)

**DO NOT repeatedly poll `BashOutput`** - it returns verbose JSON and may burn context quickly.

Instead, use Otto's incremental monitoring:

```bash
otto status                  # Check if agent is done (busy/complete/failed)
otto peek <agent>            # Incremental output since last peek (cursor-based)
otto log <agent>             # Full history (use sparingly)
otto log <agent> --tail 20   # Last 20 entries
```

**Recommended pattern:**

1. Spawn with `run_in_background: true`
2. Check `otto status` to see when agent completes
3. Use `otto peek <agent>` for incremental progress (not BashOutput)
4. Only use `BashOutput block: true` once at the end to confirm completion

**Why?** BashOutput returns raw Codex JSON which is verbose. `otto peek` returns parsed, human-readable transcript and uses a cursor so you never see the same content twice.

## Re-awakening Agents (IMPORTANT)

**Use `otto prompt` to continue work with the SAME agent for the SAME role.**

```bash
# ✅ GOOD: Re-awaken spec reviewer for re-review after fixes
otto prompt spec-1 "Issues were fixed. Re-review the changes."

# ✅ GOOD: Re-awaken quality reviewer for re-review after fixes
otto prompt quality-1 "Issues were fixed. Re-review the changes."

# ❌ BAD: Spawning new agent for re-review of same role
otto spawn codex "Re-review..." --name spec-1-re  # Wasteful!

# ❌ BAD: Prompting agent to change roles
otto prompt spec-1 "Now do quality review"  # Wrong! Different role = different agent
```

**When to prompt vs spawn:**
- **Prompt**: Re-reviews after fixes (same agent, same role)
- **Spawn**: Different task OR different role (spec vs quality vs implementation)

**Key insight:** Spec reviewer and quality reviewer are DIFFERENT ROLES requiring SEPARATE agents.
The re-awakening pattern is for when the SAME reviewer needs to verify fixes.

**Benefits of prompting for re-reviews:**
- Agent retains context from initial review
- Knows what issues it found previously
- Can verify fixes were actually made
- More efficient than explaining context to new agent

## Communication

```bash
otto prompt <agent> "message"   # Send followup to agent (also re-awakens completed agents)
otto say "status update"        # Post to shared channel
```

## Lifecycle

```bash
otto kill <agent>            # Terminate agent process
otto interrupt <agent>       # Pause agent (can resume later)
otto archive <agent>         # Archive completed/failed agent
```

## Agent Communication

Spawned agents use these to communicate back:
- `otto say "update"` - Post status to channel
- `otto ask "question?"` - Set status to WAITING, block for answer
- `otto complete` - Signal task is done

## Typical Flow

```
# 1. Spawn with run_in_background (Bash tool)
command: otto spawn codex "Implement Task 1: Add user model.

Rules:
- Do not read or act on other tasks
- Stop after Task 1 and report" --name task-1
run_in_background: true

# 2. Check status periodically (Bash tool)
otto status

# 3. Get incremental progress if needed (Bash tool)
otto peek task-1

# 4. When complete, review and answer questions if needed
otto prompt task-1 "Use UUID for the ID field"

# 5. Clean up when done
otto archive task-1
```

**Avoid:** Repeatedly calling `BashOutput` on the spawn command - use `otto peek` instead.

## Scope Control

**Do NOT attach full plan files.** Paste only the specific task text.

**Always include guardrails:**
- "Do not read or act on other tasks"
- "Stop after this task and report"

## Quick Reference

| Action | How |
|--------|-----|
| Spawn | Bash tool: `otto spawn codex "task" --name x` with `run_in_background: true` |
| Check status | `otto status` |
| Incremental progress | `otto peek <agent>` (preferred over BashOutput) |
| Full log | `otto log <agent>` |
| Send message | `otto prompt <agent> "msg"` |
| Check channel | `otto messages` |
| Kill | `otto kill <agent>` |
| Archive | `otto archive <agent>` |

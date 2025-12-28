---
name: otto-orchestrate
description: Use when spawning Codex agents via Otto to delegate implementation work.
---

# Otto Orchestration

Spawn and coordinate Codex agents for implementation work.

## Orchestrator Role

You coordinate, you don't implement. Dispatch agents to do the work.

## Spawning Agents

**Use `run_in_background: true` - you'll be notified automatically when the agent completes:**

```
Bash tool call:
  command: otto spawn codex "task description" --name <name>
  run_in_background: true
```

**That's it.** Claude Code notifies you when the agent finishes. No polling needed.

When notified (or when you're ready to wait), retrieve results:
```
BashOutput tool call:
  bash_id: <id from spawn>
  block: true  # waits if still running
```

### Spawn Options

- `--name <name>` - Readable ID (e.g., `planner`, `impl-1`)
- `--files <paths>` - Attach relevant files (keep minimal)
- `--context <text>` - Extra context

## You Don't Need to Poll

You'll be **automatically notified** when agents complete. So there's no need to:
- Run `otto status` to check if an agent is done
- Run `otto peek` repeatedly while waiting

Just wait, or do other work. The notification will come.

## Checking Progress (Optional)

If things seem to be taking a while and you want to see what's happening:

```bash
otto peek <agent>            # Incremental output since last peek
otto status                  # See all agent states
otto log <agent> --tail 20   # Recent log entries
```

This is fine for curiosity or if something feels stuck—just know it's not required.

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

# 2. Do other work or just wait - you'll be notified when done

# 3. When notified, retrieve results (BashOutput tool)
bash_id: <id from step 1>
block: true

# 4. If agent needs guidance, respond and wait again
otto prompt task-1 "Use UUID for the ID field"

# 5. Clean up when done
otto archive task-1
```

## Scope Control

**Do NOT attach full plan files.** Paste only the specific task text.

**Always include guardrails:**
- "Do not read or act on other tasks"
- "Stop after this task and report"

## Quick Reference

| Action | How |
|--------|-----|
| Spawn | Bash tool: `otto spawn codex "task" --name x` with `run_in_background: true` |
| Wait for completion | Do nothing—you'll be notified automatically |
| Get results | `BashOutput` with `bash_id` from spawn, `block: true` |
| Send message | `otto prompt <agent> "msg"` |
| Check channel | `otto messages` |
| Kill | `otto kill <agent>` |
| Archive | `otto archive <agent>` |
| *(Optional)* See progress | `otto peek <agent>` or `otto status` |

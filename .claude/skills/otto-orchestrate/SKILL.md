---
name: june-orchestrate
description: Use when spawning Codex agents via June to delegate implementation work.
---

# June Orchestration

Spawn and coordinate Codex agents for implementation work.

## Orchestrator Role

You coordinate, you don't implement. Dispatch agents to do the work.

## Spawning Agents

**Use `run_in_background: true` - you'll be notified automatically when the agent completes:**

```
Bash tool call:
  command: june spawn codex "task description" --name <name>
  run_in_background: true
```

**That's it.** Claude Code notifies you when the agent finishes. No polling needed.

When notified (or when you're ready to wait), retrieve results:
```
BashOutput tool call:
  bash_id: <id from spawn>
```

### Spawn Options

- `--name <name>` - Readable ID (e.g., `planner`, `impl-1`)
- `--files <paths>` - Attach relevant files (keep minimal)
- `--context <text>` - Extra context

## You Don't Need to Poll

You'll be **automatically notified** when agents complete. So there's no need to:
- Run `june status` to check if an agent is done
- Run `june peek` repeatedly while waiting

Just wait, or do other work. The notification will come.

## Checking Progress (Optional)

If things seem to be taking a while and you want to see what's happening:

```bash
june peek <agent>            # Incremental output since last peek
                             # For completed/failed agents, shows full log (capped at 100 lines)
june status                  # See all agent states
june log <agent> --tail 20   # Recent log entries
```

This is fine for curiosity or if something feels stuck—just know it's not required.

## Re-awakening Agents (IMPORTANT)

**Use `june prompt` to continue work with the SAME agent for the SAME role.**

```bash
# ✅ GOOD: Re-awaken spec reviewer for re-review after fixes
june prompt spec-1 "Issues were fixed. Re-review the changes."

# ✅ GOOD: Re-awaken quality reviewer for re-review after fixes
june prompt quality-1 "Issues were fixed. Re-review the changes."

# ❌ BAD: Spawning new agent for re-review of same role
june spawn codex "Re-review..." --name spec-1-re  # Wasteful!

# ❌ BAD: Prompting agent to change roles
june prompt spec-1 "Now do quality review"  # Wrong! Different role = different agent
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
june prompt <agent> "message"                           # Send followup to agent (also re-awakens completed agents)
june dm --from <sender> --to <recipient> "message"      # Send direct message between agents
                                                        # Supports cross-branch: --to feature/login:frontend
```

## Lifecycle

```bash
june kill <agent>            # Terminate agent process
june interrupt <agent>       # Pause agent (can resume later)
june archive <agent>         # Archive completed/failed agent
```

## When to Archive

**Don't archive during active work.** Keep agents around while:
- Still working through a multi-step task
- Reviewing results or debugging issues
- Might need follow-up (`june prompt` for fixes or clarification)

**Archive when work is complete:**
1. **User signals satisfaction** - "looks good", "ship it", "done", ready to push/merge
2. **Starting unrelated work** - Before pivoting to a new task, clean up agents from the previous one

```bash
# After user approves the work and you're ready to commit/push:
june archive planner
june archive impl-1
june archive reviewer
# ... archive all agents from this task
```

**Why this matters:** Archiving too early loses context you might need. Archiving too late clutters the agent list. Archive at natural boundaries when work is truly done.

## Agent Communication

Spawned agents use these to communicate back:
- `june dm --from <self> --to <recipient> "message"` - Send direct message to another agent or orchestrator
- `june ask "question?"` - Set status to WAITING, block for answer
- `june complete` - Signal task is done

## Typical Flow

```
# 1. Spawn with run_in_background (Bash tool)
command: june spawn codex "Implement Task 1: Add user model.

Rules:
- Do not read or act on other tasks
- Stop after Task 1 and report" --name task-1
run_in_background: true

# 2. Do other work or just wait - you'll be notified when done

# 3. When notified, retrieve results (BashOutput tool)
bash_id: <id from step 1>

# 4. If agent needs guidance, respond and wait again
june prompt task-1 "Use UUID for the ID field"

# 5. Continue with more tasks, reviews, etc.
# Keep agents around - you might need to reference or prompt them

# 6. When user is satisfied and ready to push/merge:
june archive task-1
june archive reviewer
# ... archive all agents from this work
```

## Scope Control

**Do NOT attach full plan files.** Paste only the specific task text.

**Always include guardrails:**
- "Do not read or act on other tasks"
- "Stop after this task and report"

## Quick Reference

| Action | How |
|--------|-----|
| Spawn | Bash tool: `june spawn codex "task" --name x` with `run_in_background: true` |
| Wait for completion | Do nothing—you'll be notified automatically |
| Get results | `BashOutput` with `bash_id` from spawn |
| Send message | `june prompt <agent> "msg"` |
| Agent-to-agent msg | `june dm --from <sender> --to <recipient> "msg"` |
| Check channel | `june messages` |
| Kill | `june kill <agent>` |
| Archive | `june archive <agent>` |
| *(Optional)* See progress | `june peek <agent>` or `june status` |

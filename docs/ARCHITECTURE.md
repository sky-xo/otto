# Otto Architecture

How Otto works under the hood. For feature status, see `TODO.md`. For usage examples, see `SCENARIOS.md`.

## Overview

Otto is a CLI tool that enables a single Claude Code session to orchestrate multiple AI agents (Claude Code and Codex), allowing them to work on tasks in parallel and communicate with each other.

**The core idea:** You chat with Claude Code as the "orchestrator." It spawns background agents, monitors their progress, surfaces questions to you, and coordinates handoffs between agents.

## Why Otto?

- **Unified interface** for both Claude Code and Codex
- **Cross-tool communication** - design in Claude Code, implement in Codex, review in Claude Code
- **Persistent agents** that survive session restarts (via native `--resume`)
- **Escalation system** - agents ask questions when stuck, orchestrator bubbles up to you
- **Fire-and-forget or interactive** - agents decide when they need help

## System Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  You ←→ Claude Code (orchestrator)                          │
│         │                                                   │
│         │ calls otto CLI                                    │
│         ▼                                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  otto CLI                                            │   │
│  │  - spawn agents                                      │   │
│  │  - check status                                      │   │
│  │  - send/receive messages                             │   │
│  │  - manage agent lifecycle                            │   │
│  └─────────────────────────────────────────────────────┘   │
│         │                                                   │
│         ├──────────────────┬───────────────────┐           │
│         ▼                  ▼                   ▼           │
│  ┌─────────────┐   ┌─────────────┐    ┌─────────────┐     │
│  │ Claude Code │   │ Codex       │    │ Claude Code │     │
│  │ (design)    │   │ (implement) │    │ (review)    │     │
│  │ agent-abc   │   │ agent-def   │    │ agent-ghi   │     │
│  └─────────────┘   └─────────────┘    └─────────────┘     │
│         │                  │                   │           │
│         └──────────────────┴───────────────────┘           │
│                            │                                │
│                   ~/.otto/ (SQLite messaging)               │
└─────────────────────────────────────────────────────────────┘
```

## Core Concepts

### The Ephemeral Orchestrator Model

A key architectural insight: **the orchestrator conversation is ephemeral, but state is durable.**

```
┌─────────────────────────────────────────────────────────────┐
│ DURABLE STATE (persists forever)                            │
│                                                             │
│  otto.db           → agents, messages, status               │
│  docs/plans/*.md   → what we're building, why, progress     │
│  .worktrees/       → isolated agent workspaces              │
│  git history       → what was actually done                 │
└─────────────────────────────────────────────────────────────┘
                              ↑
                              │ reads/writes
                              │
┌─────────────────────────────────────────────────────────────┐
│ EPHEMERAL ORCHESTRATOR (current Claude Code session)        │
│                                                             │
│  - Reads plan documents to understand context               │
│  - Queries otto for current state                           │
│  - Makes decisions, spawns agents, answers questions        │
│  - Can end anytime - state persists without it              │
│  - New session picks up from durable state                  │
└─────────────────────────────────────────────────────────────┘
```

**Implications:**

1. **Conversations can be short** - End a session, start a new one, lose nothing important.
2. **Context comes from docs, not chat** - Plan documents are the source of truth.
3. **No special "resume" needed** - A new session just reads the plan and runs `otto status`.
4. **Agents are truly independent** - They don't need the orchestrator session to exist.

### Orchestrator

The orchestrator is just Claude Code with knowledge of otto commands. It:
- Spawns agents via `otto spawn`
- Checks for messages via `otto messages`
- Sends responses via `otto prompt`
- Tracks agent status via `otto status`

There's no separate UI - the conversation with Claude Code IS the interface.

### Agents

Agents are Claude Code or Codex sessions running in non-interactive mode:
- **Claude Code:** `claude -p "task" --session-id <id>`
- **Codex:** `codex exec "task"`

Both support session resume:
- **Claude Code:** `claude --resume <session-id>` (interactive) or `claude --continue --print "<message>"` (headless)
- **Codex:** `codex resume <session-id>` (interactive) or `codex exec resume <session-id> "<message>"` (headless)

### Agent States

```
BUSY      - actively processing a task
IDLE      - ready for input (interrupted or between turns)
BLOCKED   - asked a question, needs reply to continue
```

Note: Agents are deleted from the database when they complete or exit. The `messages` table preserves history.

### Orchestrator Scoping

Orchestrators are auto-scoped by project and branch:

```bash
cd ~/code/my-app  # on branch feature-auth
otto spawn codex "build login"
# → orchestrator: my-app/feature-auth
```

## Storage

### Directory Structure

```
~/.otto/
  orchestrators/
    <project>/
      <branch>/
        otto.db             # SQLite database
        agents/
          <agent-id>/
            context.md      # handoff context
            output.log      # captured output (optional)
```

### Database Schema

```sql
CREATE TABLE agents (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,           -- 'claude' or 'codex'
  task TEXT NOT NULL,
  status TEXT NOT NULL,         -- 'working', 'waiting', 'done', 'failed'
  session_id TEXT,              -- for resume
  worktree_path TEXT,
  branch_name TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,        -- agent ID or 'orchestrator' or 'human'
  type TEXT NOT NULL,           -- 'dm', 'question', 'complete'
  content TEXT NOT NULL,
  mentions TEXT,                -- JSON array: ["agent-def", "orchestrator"]
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',    -- JSON array of readers
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_created ON messages(created_at);
CREATE INDEX idx_agents_status ON agents(status);
```

### Communication Model

All agents share a single message stream - like a shared chat room:

```
┌─────────────────────────────────────────────────────────┐
│ [agent-abc] Finished auth backend. @agent-def ready     │
│ [agent-def] Got it. What's the token format?            │
│ [agent-abc] JWT, expires in 7 days. See src/auth/jwt.ts │
│ [orchestrator] @agent-ghi start review when ready.      │
└─────────────────────────────────────────────────────────┘
```

- All messages visible to everyone
- @mentions direct attention to specific agents
- Agents filter to messages that @mention them

### Message Types

- `dm` - direct message to specific recipient(s)
- `question` - needs a response (sets agent to WAITING)
- `complete` - task finished, here's the result

### Why SQLite?

- **Queryable:** Filter by type, agent, unread status
- **Atomic:** No race conditions on concurrent writes
- **Single file:** Easy to backup
- **Debuggable:** `sqlite3 otto.db "SELECT * FROM messages"`

## Agent Prompt Template

When otto spawns an agent, it includes these instructions:

```markdown
You are an agent working on: <task>
Your agent ID: <agent-id>

## Communication

You're part of a team. All agents share a message stream.
Use @mentions to direct attention to specific agents.

IMPORTANT: Always include your ID (--id <agent-id>) in every command.

### Commands
otto messages --id <agent-id>                           # check unread
otto dm --from <agent-id> --to <recipient> "message"    # post message
otto ask --id <agent-id> "question"                     # ask (sets WAITING)
otto complete --id <agent-id> "summary"                 # mark done

## Guidelines

- Check messages regularly
- Use @mentions when you need specific agents
- Escalate with --human for UX, architecture, security decisions
```

### Escalation Flow

```
Agent hits a blocker
        │
        ▼
Can I resolve with current context? ──YES──→ Continue
        │
        NO
        │
        ▼
Requires human judgment? ──YES──→ otto ask --human "..."
        │
        NO
        │
        ▼
otto ask "..." → Orchestrator answers
```

## Tech Stack

- **Language:** Go
- **CLI:** Cobra
- **TUI:** Bubbletea + Lipgloss
- **Database:** modernc.org/sqlite (pure Go, no CGO)
- **Process:** os/exec

## Compatibility

### packnplay

Otto works inside [packnplay](https://github.com/obra/packnplay) containers - `claude` and `codex` are available, project is mounted, credentials configured.

### superpowers

Otto complements superpowers skills:
- Orchestrator uses `brainstorming`, `writing-plans`
- Implementation agents use `executing-plans`, `test-driven-development`
- All agents use `verification-before-completion`

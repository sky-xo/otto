# Otto: Multi-Agent Orchestrator for Claude Code and Codex

## Overview

Otto is a CLI tool that enables a single Claude Code session to orchestrate multiple AI agents (Claude Code and Codex), allowing them to work on tasks in parallel and communicate with each other.

**The core idea:** You chat with Claude Code as the "orchestrator." It spawns background agents, monitors their progress, surfaces questions to you, and coordinates handoffs between agents.

## Why Otto?

- **Unified interface** for both Claude Code and Codex
- **Cross-tool communication** - design in Claude Code, implement in Codex, review in Claude Code
- **Persistent agents** that survive session restarts (via native `--resume`)
- **Escalation system** - agents ask questions when stuck, orchestrator bubbles up to you
- **Fire-and-forget or interactive** - agents decide when they need help

## Architecture

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

1. **Conversations can be short** - End a session, start a new one, lose nothing important. The plan document and otto.db have everything.

2. **Context comes from docs, not chat** - Plan documents (`docs/plans/*.md`) are the source of truth for what you're building and why.

3. **No special "resume" needed** - A new session just reads the plan and runs `otto status`. Full context restored.

4. **Agents are truly independent** - They don't need the orchestrator session to exist. They write to otto.db, any future session can pick them up.

### Orchestrator

The orchestrator is just Claude Code with knowledge of otto commands. It:
- Spawns agents via `otto spawn`
- Checks for messages via `otto messages`
- Sends responses via `otto send`
- Tracks agent status via `otto status`

There's no separate UI - the conversation with Claude Code IS the interface.

**Starting a new session:**
```bash
# Read the plan to understand context
cat docs/plans/current-feature.md

# Check agent states
otto status

# Check for pending messages
otto messages
```

This gives the orchestrator everything it needs to continue where the previous session left off.

### Agents

Agents are Claude Code or Codex sessions running in non-interactive mode:
- **Claude Code:** `claude -p "task" --session-id <id>`
- **Codex:** `codex exec "task"`

Both support session resume:
- **Claude Code:** `claude --resume <session-id>`
- **Codex:** `codex resume <session-id>`

This enables the "attach" pattern - you can jump into any agent's session interactively.

### Agent States

```
WORKING   - actively processing a task
WAITING   - blocked, has a question for orchestrator
DONE      - completed its task
FAILED    - crashed or errored out
```

### Orchestrator Scoping

Orchestrators are auto-scoped by project and branch:

```bash
cd ~/code/my-app  # on branch feature-auth
otto spawn codex "build login"
# → orchestrator: my-app/feature-auth
```

Override with `--in` for cross-branch or cross-repo work:

```bash
otto spawn --in my-app codex "coordinate release"
# → orchestrator: my-app (project-level, no branch)

otto spawn --in mobile-rewrite codex "sync iOS and Android"
# → orchestrator: mobile-rewrite (custom name)
```

## Storage

### Directory Structure

```
~/.otto/
  orchestrators/
    <project>/
      <branch>/
        otto.db             # SQLite database (agents, messages, state)
        agents/
          <agent-id>/
            context.md      # handoff context from orchestrator
            output.log      # captured output (optional)
```

### Database Schema

```sql
-- Agents table
CREATE TABLE agents (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,           -- 'claude' or 'codex'
  task TEXT NOT NULL,
  status TEXT NOT NULL,         -- 'working', 'waiting', 'done', 'failed'
  session_id TEXT,              -- claude/codex session ID for resume
  worktree_path TEXT,           -- path to worktree if using --worktree
  branch_name TEXT,             -- git branch name
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Messages table
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,        -- agent ID or 'orchestrator' or 'human'
  type TEXT NOT NULL,           -- 'say', 'question', 'complete'
  content TEXT NOT NULL,
  mentions TEXT,                -- JSON array: ["agent-def", "orchestrator"]
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',    -- JSON array of agent IDs who have read this
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX idx_messages_created ON messages(created_at);
CREATE INDEX idx_agents_status ON agents(status);
```

### Communication Model

All agents share a single message stream - like a shared log or chat room:

```
┌─────────────────────────────────────────────────────────┐
│ [agent-abc] Finished auth backend. @agent-def ready     │
│             for frontend integration.                   │
│                                                         │
│ [agent-def] Got it. What's the token format?            │
│                                                         │
│ [agent-abc] JWT, expires in 7 days. See src/auth/jwt.ts │
│                                                         │
│ [orchestrator] @agent-ghi start review when ready.      │
└─────────────────────────────────────────────────────────┘
```

**Simple model:**
- All messages visible to everyone
- @mentions direct attention to specific agents
- Agents can filter to just messages that @mention them

**Why this model:**
- Full visibility for orchestrator and human
- Shared context across all agents
- @mentions make it clear who should respond
- Agents choose how much of the stream to read

### Message Types

- `say` - general message to the channel
- `question` - needs a response (sets agent to WAITING)
- `complete` - task finished, here's the result

### Why SQLite?

- **Queryable:** "show unread messages", "messages from agent-x", "agents that are waiting"
- **Atomic:** No race conditions on concurrent writes
- **Single file:** One `otto.db` per orchestrator, easy to backup
- **No file proliferation:** Avoids hundreds of small JSON files
- **Debuggable:** `sqlite3 otto.db "SELECT * FROM messages"` or `otto messages --debug`

## CLI Commands

### otto spawn

Spawn a new agent:

```bash
otto spawn <type> "<task>"

# Examples:
otto spawn claude "design the auth system UX"
otto spawn codex "implement OAuth login flow"

# Options:
otto spawn --in <orchestrator> codex "task"  # specify orchestrator
otto spawn --files src/auth/ codex "task"    # hint relevant files
otto spawn --context "use Redis for sessions" codex "task"  # extra context
otto spawn --worktree oauth-backend codex "task"  # work in isolated worktree
```

#### Worktree Isolation (--worktree)

When multiple agents work in parallel on code changes, they need isolated workspaces.
The `--worktree` flag creates a git worktree for the agent:

```bash
otto spawn codex "Implement backend" --worktree backend
otto spawn codex "Implement frontend" --worktree frontend
```

This creates:
```
my-project/
  .worktrees/           # gitignored
    backend/            # agent 1's isolated workspace
    frontend/           # agent 2's isolated workspace
  src/
  ...
```

**What otto does:**
1. Ensures `.worktrees/` exists and is in `.gitignore`
2. Creates worktree: `git worktree add .worktrees/<name> -b feature/<name>`
3. Copies env files if project has conventions (detects .env, .dev.vars, etc.)
4. Spawns agent with cwd set to the worktree

**On agent completion:**
- Agent uses `finishing-a-development-branch` skill
- Merges to main or creates PR
- Otto cleans up: `git worktree remove .worktrees/<name>`

**Integrates with superpowers:** Follows same conventions as `superpowers:using-git-worktrees` skill.

### otto status

Check agent status:

```bash
otto status              # all agents in current orchestrator
otto status --all        # all agents across all orchestrators
otto status <agent-id>   # specific agent details
```

Output:
```
my-app/feature-auth:
  agent-abc (claude)  WORKING   "design auth UX"
  agent-def (codex)   WAITING   "implement OAuth" - needs input
  agent-ghi (codex)   DONE      "write tests"
```

### otto messages

View messages:

```bash
otto messages                        # unread messages (default)
otto messages --all                  # all messages
otto messages --last 20              # last 20 messages
otto messages --from agent-abc       # from specific agent
otto messages --mentions agent-def   # messages that @mention an agent
otto messages --questions            # only questions needing answers
```

Output:
```
[agent-abc] Finished auth backend. @agent-def ready for you
[agent-def] QUESTION: What's the token format?
[agent-abc] JWT, 7 days. See src/auth/jwt.ts
[orchestrator] @agent-ghi start review when ready
```

### otto say

Post a message to chat:

```bash
# Orchestrator posts (no --id flag)
otto say "Schema changed, see docs/schema.md"
otto say "@all new requirement: support OAuth"

# Agent posts (with --id flag)
otto say --id <agent-id> "message"
otto say --id agent-abc "Finished auth backend, @agent-def ready for frontend"
```

Use `@all` to mention all active agents. The daemon will wake them.

### otto prompt

Wake up an agent with new instructions (used by orchestrator):

```bash
otto prompt <agent-id> "message"
otto prompt agent-abc "Use argon2 for password hashing"
otto prompt agent-def "Backend is ready, start frontend integration"
```

This resumes the agent's session with the given prompt. Use this to:
- Answer a WAITING agent's question
- Give a DONE agent new work
- Redirect an agent mid-task

Note: Prompts are direct to the agent, not posted to chat. This keeps the chat clean for agent-to-agent communication.

### otto watch

Watch messages in real-time (Seed/V0 - simple polling):

```bash
otto watch
```

Polls the message stream and prints new messages as they arrive:

```
[agent-abc] Backend done. @agent-def ready for frontend
[agent-def] Got it, starting now
[agent-def] QUESTION: What auth lib?
[orchestrator] @all schema changed, see docs/schema.md
```

Run this in a separate terminal tab to monitor your agents.

**Future (Tree/V2+):** Upgrade to full TUI dashboard with agent sidebar and daemon that auto-wakes agents on @mentions.

### otto attach

Print the command to attach to an agent:

```bash
otto attach <agent-id>

# Output:
# Claude Code agent. To attach, run:
#   claude --resume abc123
```

Future: could auto-open in new terminal tab.

### otto kill

Stop an agent:

```bash
otto kill <agent-id>
otto kill --all          # kill all agents in current orchestrator
```

### otto clean

Clean up finished agents:

```bash
otto clean               # remove DONE and FAILED agents
otto clean --all         # clean across all orchestrators
```

### otto list

List orchestrators:

```bash
otto list

# Output:
# my-app/feature-auth    3 agents (1 working, 1 waiting, 1 done)
# my-app/main            1 agent (working)
# mobile-rewrite         2 agents (2 working)
```

## Agent Behavior

### Spawned Agent Prompt Template

When otto spawns an agent, it includes instructions for messaging:

```markdown
You are an agent working on: <task>

Your agent ID: <agent-id>
Relevant files: <files>
Additional context: <context>

## Communication

You're part of a team. All agents share a message stream where everyone can
see everything. Use @mentions to direct attention to specific agents.

IMPORTANT: Always include your ID (--id <agent-id>) in every command.

### Check for messages
otto messages --id <agent-id>              # unread messages
otto messages --mentions <agent-id>        # just messages that @mention you

### Post a message
otto say --id <agent-id> "Finished auth backend, @agent-def ready for frontend"

### Ask a question (sets you to WAITING)
otto ask --id <agent-id> "Should auth tokens expire after 24h or 7d?"

### Ask a question requiring human input
otto ask --id <agent-id> --human "What should the error message say?"

### Mark task as complete
otto complete --id <agent-id> "Auth system implemented. PR ready."

## Guidelines

**Check messages regularly** - other agents or the orchestrator may have
questions or updates for you. Prioritize messages that @mention you.

**Use @mentions** - when you need a specific agent's attention, @mention them.

**Escalate with --human when:**
- UX decisions
- Major architectural choices
- External service selection
- Cost/billing decisions
- Security-sensitive decisions

**Can ask without --human:**
- Code style questions
- Where to find files
- Testing approach
- Implementation details
```

### Escalation Flow

```
Agent hits a blocker
        │
        ▼
Can I resolve this with my current context?
        │
    YES │           NO
        │           │
        ▼           ▼
   Continue    Is this something requiring human judgment?
                    │
                YES │           NO
                    │           │
                    ▼           ▼
            Write message   Write message
            requires_human: true    requires_human: false
                    │           │
                    ▼           ▼
            Orchestrator    Orchestrator
            asks human      tries to answer
                            (from earlier context)
```

## Orchestrator Behavior

The Claude Code orchestrator should:

1. **Periodically check messages** - run `otto messages` to see if agents need help
2. **Triage questions** - answer from context or escalate to user
3. **Coordinate handoffs** - when one agent finishes, spawn the next
4. **Track overall progress** - know what's working, waiting, done

### Example Orchestrator Flow

```
User: "Let's build a user auth system"

Orchestrator: "I'll break this down:
1. Design the UX flow (I'll do this with you)
2. Implement backend (Codex agent)
3. Implement frontend (Codex agent)
4. Review and test (Claude Code agent)

Let's start with the design..."

[design conversation happens]

Orchestrator: "Design looks good. Spawning implementation agents."
→ otto spawn codex "Implement auth backend: OAuth, JWT tokens, 7-day expiry..."
→ otto spawn codex "Implement auth frontend: login form, token storage..."

Orchestrator: "Two agents working. I'll check in periodically."

[later]
→ otto messages

Orchestrator: "Backend agent asks: 'Should I use bcrypt or argon2 for password hashing?'
This is a security decision - what's your preference?"

User: "argon2"

Orchestrator: → otto prompt agent-abc "Use argon2"
"Sent. Agent resumed."

[later]

Orchestrator: "Both agents done. Spawning review agent."
→ otto spawn claude "Review auth implementation for security issues..."
```

## Implementation Plan

We follow a **Seed → Sprout → Tree** scope model.

### Seed (V0) - Core Loop

The minimal viable orchestration system. Validate the idea before adding polish.

- [ ] SQLite database setup (agents, messages tables)
- [ ] `otto spawn` - spawn Claude Code and Codex agents
- [ ] `otto status` - list agents and their states
- [ ] `otto messages` - check for pending messages
- [ ] `otto prompt` - wake up an agent with new instructions
- [ ] `otto say` - post to chat (orchestrator or agent with --id)
- [ ] `otto ask` - agent asks a question (sets to WAITING)
- [ ] `otto complete` - agent marks task done
- [ ] `otto attach` - print resume command
- [ ] `otto watch` - simple message tail for debugging (poll + print, no TUI)
- [ ] Auto-detect project/branch scoping
- [ ] Agent prompt templates with escalation instructions

**Not in Seed:** No daemon, no worktrees, no TUI dashboard. Orchestrator manually polls with `otto messages`.

### Sprout (V1) - Friction Reducers

Add features that reduce friction once core loop is validated.

- [ ] Worktree support (`--worktree`) for parallel agents
- [ ] `otto kill` and `otto clean`
- [ ] `otto list` for orchestrators
- [ ] `--in` flag for custom orchestrator names
- [ ] Optional message streaming view (upgrade `otto watch`)

### Tree (V2+) - Full Experience

The polished multi-agent experience.

- [ ] Full dashboard TUI with Bubbletea (chat + agent sidebar)
- [ ] Daemon with auto-wakeup on @mentions
- [ ] Super-orchestrator: attention router across multiple orchestrators
- [ ] Auto-open terminal for attach
- [ ] Web dashboard for visualization

## Technical Details

### Distribution

Single binary via:
```bash
# Go install
go install github.com/youruser/otto@latest

# Or download binary from GitHub releases
curl -L https://github.com/youruser/otto/releases/latest/download/otto-darwin-arm64 -o otto
chmod +x otto

# Or homebrew (future)
brew install youruser/tap/otto
```

### Tech Stack

- **Language:** Go
- **CLI framework:** Cobra (standard for Go CLIs)
- **Terminal UI:** Bubbletea + Lipgloss (what lazygit uses) - for `otto start` dashboard
- **Database:** go-sqlite3 or modernc.org/sqlite (pure Go, no CGO)
- **Process management:** os/exec

### Session Management

Claude Code sessions:
- Stored in `~/.claude/`
- Resume with `--resume <session-id>` or `--continue`

Codex sessions:
- Stored in `~/.codex/sessions/`
- Resume with `codex resume <session-id>` or `--last`

Otto tracks the mapping: `agent-id → session-id` in `config.json`.

## Compatibility

### packnplay

Otto works seamlessly inside [packnplay](https://github.com/obra/packnplay) containers with no modifications needed.

packnplay creates persistent Docker containers with AI CLI tools pre-installed. When running inside a packnplay container:
- `claude` and `codex` commands are available
- Project directory is mounted at the same host path
- Credentials are pre-configured
- Git worktrees work normally (created on mounted filesystem, persist on host)

Typical workflow:
```bash
# On host - start packnplay session
packnplay run claude

# Inside container - use otto normally
otto spawn codex "implement backend" --worktree backend
otto status
otto messages
```

The two tools are complementary:
- **packnplay**: Container isolation, credential management, reproducible environment
- **otto**: Multi-agent orchestration within that environment

### superpowers

Otto complements the superpowers plugin:
- **superpowers**: Skills that guide agent behavior (TDD, debugging, code review, etc.)
- **otto**: Spawns and coordinates multiple agents

Agents spawned by otto can use superpowers skills. The orchestrator uses skills like `brainstorming` and `writing-plans`, while implementation agents use `executing-plans` and `test-driven-development`.

## Decisions Made

1. **Package name:** `otto` (may prefix with GitHub org/user later if needed)
2. **No `otto handoff`:** Orchestrator stays in control of spawning new agents. This gives the orchestrator visibility into agent context usage and keeps coordination centralized.
3. **No channels within orchestrators:** Stick with one orchestrator per branch. Multiple work streams = multiple branches/orchestrators.
4. **Simplified agent commands:** Just `say`, `ask`, `complete`. Dropped `update` (use `say` instead).
5. **No DMs:** Single shared message stream. All messages visible to everyone. Agents use `--mentions` to filter for messages directed at them. Simpler model, full visibility.
6. **`prompt` not `reply`:** Orchestrator uses `otto prompt` to wake up agents with new instructions. Works for answering questions OR giving new work. Direct to agent, not posted to chat.
7. **Chat is for agent-to-agent:** The message stream is where agents talk to each other. Orchestrator prompts are direct (not in chat) because the orchestrator already has its own context.
8. **`otto watch` for v0, full dashboard later:** Simple message tail for debugging in Seed. Full Bubbletea TUI + daemon in Tree (v2+).
9. **Orchestrator can post to chat:** `otto say "..."` without `--id` posts as orchestrator. Useful for broadcasts. `@all` mentions all active agents.
10. **`--id` flag determines role:** Agent commands require `--id <agent>`. Orchestrator commands (`spawn`, `prompt`, `kill`) reject `--id` with an error. Simple enforcement without token complexity.
11. **Seed → Sprout → Tree scoping:** Start minimal (no daemon, no worktrees), add friction reducers, then polish.

## Open Questions

1. **Agent timeout:** Should agents auto-terminate after inactivity?
2. **Cross-machine sync:** Future feature - sync orchestrator state across machines?

## Summary

Otto is a lightweight CLI that turns Claude Code into a multi-agent orchestrator. It leverages the native session resume capabilities of both Claude Code and Codex to enable persistent, interruptible agents that can communicate through a SQLite-backed message bus.

The key insight: we don't need to build complex infrastructure. Claude Code and Codex already have the primitives we need (session persistence, non-interactive mode). Otto just wires them together with a simple, queryable database.

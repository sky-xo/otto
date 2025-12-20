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
│                   ~/.otto/ (file-based messaging)           │
└─────────────────────────────────────────────────────────────┘
```

## Core Concepts

### Orchestrator

The orchestrator is just Claude Code with knowledge of otto commands. It:
- Spawns agents via `otto spawn`
- Checks for messages via `otto messages`
- Sends responses via `otto send`
- Tracks agent status via `otto status`

There's no separate UI - the conversation with Claude Code IS the interface.

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

## File Structure

```
~/.otto/
  orchestrators/
    <project>/
      <branch>/
        state.json          # orchestrator state
        agents/
          <agent-id>/
            config.json     # agent type, task, status
            context.md      # handoff context from orchestrator
            output.log      # captured output (optional)
        messages/
          inbox/            # messages TO orchestrator
            <timestamp>-<from>.json
          agents/
            <agent-id>/     # messages TO this agent
              <timestamp>-<from>.json
```

### Message Format

```json
{
  "id": "msg-abc123",
  "from": "agent-def",
  "to": "orchestrator",
  "type": "question",
  "content": "Should auth tokens expire after 24h or 7d?",
  "timestamp": "2025-01-15T10:30:00Z",
  "requires_human": false
}
```

Message types:
- `question` - agent needs input
- `update` - status update, no response needed
- `handoff` - passing work to another agent
- `complete` - task finished, here's the result

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
```

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

Check for pending messages:

```bash
otto messages            # messages for current orchestrator
otto messages --all      # messages across all orchestrators
```

Output:
```
[agent-def] QUESTION: Should tokens expire after 24h or 7d?
[agent-ghi] COMPLETE: Tests written. 15 passing, 0 failing.
```

### otto send

Send a message to an agent:

```bash
otto send <agent-id> "<message>"
otto send agent-def "Use 7 day expiration"
```

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

Relevant files: <files>
Additional context: <context>

## Working Style

Complete this task autonomously if possible.

If you need to escalate (questions, decisions, blockers), write to:
  ~/.otto/orchestrators/<orch>/messages/inbox/

Message format:
{
  "from": "<your-agent-id>",
  "type": "question",
  "content": "your question here",
  "requires_human": true/false
}

When complete, write a "complete" message with your summary.

## Escalation Guidelines

ALWAYS escalate:
- UX decisions
- Major architectural choices
- External service selection
- Anything involving cost/billing
- Security-sensitive decisions

CAN answer yourself (if you have context):
- Code style questions
- Where to find files
- Testing approach
- Implementation details within given constraints
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

Orchestrator: → otto send agent-abc "Use argon2"
"Sent. Agents continuing..."

[later]

Orchestrator: "Both agents done. Spawning review agent."
→ otto spawn claude "Review auth implementation for security issues..."
```

## Implementation Plan

### Phase 1: MVP

- [ ] `otto spawn` - spawn Claude Code and Codex agents
- [ ] `otto status` - list agents and their states
- [ ] `otto messages` - check for pending messages
- [ ] `otto send` - send message to agent
- [ ] `otto attach` - print resume command
- [ ] File-based messaging in `~/.otto/`
- [ ] Auto-detect project/branch scoping
- [ ] Agent prompt templates with escalation instructions

### Phase 2: Polish

- [ ] `otto kill` and `otto clean`
- [ ] `otto list` for orchestrators
- [ ] Better output formatting
- [ ] Agent output capture/logging
- [ ] `--in` flag for custom orchestrator names

### Phase 3: Nice-to-haves

- [ ] Auto-open terminal for attach
- [ ] Hooks integration for auto-checking messages
- [ ] Web dashboard for visualization
- [ ] Agent-to-agent direct messaging

## Technical Details

### Distribution

npm package:
```bash
npm install -g otto-agent
# or
npx otto-agent spawn ...
```

### Tech Stack

- **Language:** TypeScript (Node.js)
- **CLI framework:** Commander or Yargs
- **File watching:** chokidar (for future real-time features)
- **Process management:** child_process, with output capture

### Session Management

Claude Code sessions:
- Stored in `~/.claude/`
- Resume with `--resume <session-id>` or `--continue`

Codex sessions:
- Stored in `~/.codex/sessions/`
- Resume with `codex resume <session-id>` or `--last`

Otto tracks the mapping: `agent-id → session-id` in `config.json`.

## Open Questions

1. **Package name:** `otto-agent`? `otto-cli`? Just `otto`?
2. **Message polling vs hooks:** Should orchestrator manually check, or use Claude Code hooks?
3. **Agent timeout:** Should agents auto-terminate after inactivity?
4. **Cross-machine sync:** Future feature - sync orchestrator state across machines?

## Summary

Otto is a lightweight CLI that turns Claude Code into a multi-agent orchestrator. It leverages the native session resume capabilities of both Claude Code and Codex to enable persistent, interruptible agents that can communicate through a simple file-based message bus.

The key insight: we don't need to build complex infrastructure. Claude Code and Codex already have the primitives we need (session persistence, non-interactive mode). Otto just wires them together.

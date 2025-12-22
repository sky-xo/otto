# Hierarchical Todos Design

> **Status:** Draft - initial ideas from brainstorming session

## Overview

A persistent todo system in otto.db that both Claude and Codex agents can use. Replaces ephemeral TodoWrite with durable, shared tracking.

## Why Not TodoWrite?

- **TodoWrite is session-bound** - if conversation ends, todos are gone
- **TodoWrite is Claude-only** - Codex agents can't use it
- **Otto agents persist** - they survive session restarts, work across days

## Core Concept

Hierarchical todos where:
- **Orchestrator** creates high-level todos (from the plan)
- **Agents** create detailed subtodos as they work

```
Orchestrator creates:
├── [ ] Implement OAuth backend
├── [ ] Implement OAuth frontend
└── [ ] Review and test

Agent breaks down their assigned todo:
├── [→] Implement OAuth backend
│   ├── [✓] Add passport.js dependency
│   ├── [→] Create auth routes
│   │   ├── [✓] POST /login
│   │   └── [ ] POST /logout
│   └── [ ] Add session middleware
├── [ ] Implement OAuth frontend
└── [ ] Review and test
```

## Database Schema

```sql
CREATE TABLE todos (
  id TEXT PRIMARY KEY,
  agent_id TEXT,              -- who owns this (null = orchestrator-level)
  parent_id TEXT,             -- for hierarchy
  content TEXT NOT NULL,
  status TEXT NOT NULL,       -- pending, in_progress, completed
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_todos_agent ON todos(agent_id);
CREATE INDEX idx_todos_parent ON todos(parent_id);
CREATE INDEX idx_todos_status ON todos(status);
```

## CLI Commands

```bash
# View todos
otto todos                      # all todos across agents
otto todos --agent authbackend  # just this agent's todos

# Agent commands (require --id)
otto todo add --id agent "implement login"
otto todo start --id agent "implement login"
otto todo done --id agent "implement login"

# Orchestrator commands (no --id)
otto todo add "Implement OAuth backend"
otto todo assign authbackend "Implement OAuth backend"
```

## TUI Integration

The watch TUI could show todos alongside agents:

```
┌─ Agents ─────────────┐┌─ Todos ──────────────────────────┐
│ authbackend [working]││ [→] Implement OAuth backend      │
│ authfrontend [waiting]│   [✓] Add passport.js           │
│                      ││   [→] Create routes              │
│                      ││ [ ] Implement OAuth frontend     │
└──────────────────────┘└──────────────────────────────────┘
```

## Relationship to Plan Files

- **Plan file** = The specification (what needs to be built)
- **SQLite todos** = Execution tracking (progress, dynamic)

Agent reads plan → creates todos from it → might add subtasks during work.
Plan stays as reference, todos track live state.

## Benefits

- **Persistent** - survives session restarts
- **Shared** - Claude and Codex both access via otto CLI or SQL
- **Visible** - orchestrator sees all agent progress
- **Hierarchical** - high-level view and detailed breakdown

## Open Questions

- Should todos auto-complete parent when all children done?
- How to handle blocked/waiting todos?
- Should completed todos be archived or deleted?
- Integration with messages (todo completion → message)?

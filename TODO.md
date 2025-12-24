# Otto Roadmap

Quick overview of features and status. For detailed design, see `docs/plans/`.

## Current Focus

The one list of what we're working on now.

### Super Orchestrator - Phase 1: Event Bus + Wake-up

Design: `docs/plans/2025-12-24-super-orchestrator-design.md`

- [ ] Event logging on message posting
- [ ] @mention detection and parsing
- [ ] Wake-up mechanism via `otto prompt` with context injection
- [ ] Wire into `say`, `ask`, `complete` commands

Related design docs:
- `docs/plans/2025-12-24-tasks-design.md` - Tasks table with derived state
- `docs/plans/2025-12-24-skill-injection-design.md` - Re-injecting skills on wake-up

## Up Next

Queued after current focus. Will become "Current Focus" when ready.

### Agent Reliability
- Improve agent failure diagnostics (exit codes, stderr, failure reason)
- Permissions model: currently all subagents are fully permissive (bad). Look at codex-subagents package for inspiration

### CLI Polish
- `otto status` should list most recent agents first
- Archive polish: `--all --archive` and `--archive` batch operations

### TUI
- Display errors in UI (m.err stored but never shown)
- Composite indexes for pagination performance

## Completed

### V1 - Friction Reducers
Agent lifecycle, kill/interrupt, TUI agents panel, Codex as orchestrator, PID tracking, session resume.

### V0 - Core Loop
SQLite database, spawn, status, messages, prompt, say/ask/complete, attach, watch, project/branch scoping.

## Docs

**Design (current):**
- `docs/plans/2025-12-24-super-orchestrator-design.md` - Event-driven orchestration
- `docs/plans/2025-12-24-tasks-design.md` - Tasks table design
- `docs/plans/2025-12-24-skill-injection-design.md` - Skill re-injection
- `docs/plans/2025-12-22-orchestration-skill-design.md` - Otto vs subagents

**Design (implemented):**
- `docs/plans/2025-12-23-agent-lifecycle-design.md` - Agent lifecycle
- `docs/plans/2025-12-23-interrupt-command-plan.md` - Interrupt command
- `docs/plans/2025-12-22-otto-v0.md` - Original V0 plan

**Reference:**
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios / test cases

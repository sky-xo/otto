# Otto Roadmap

Quick overview of features and their status. For detailed design, see individual plan files in `docs/plans/`.

## V0 - Core Loop (Complete)

The minimal viable orchestration system.

- [x] SQLite database (agents, messages tables)
- [x] `otto spawn` - spawn Claude Code and Codex agents
- [x] `otto status` - list agents and states
- [x] `otto messages` - check pending messages
- [x] `otto prompt` - wake up agent with instructions
- [x] `otto say` / `otto ask` / `otto complete` - agent messaging
- [x] `otto attach` - print resume command
- [x] `otto watch` - real-time message stream (TUI when terminal, plain text when piped)
- [x] Auto-detect project/branch scoping
- [x] Agent prompt templates with communication instructions

Plan: `docs/plans/2025-12-22-otto-v0.md`

## V1 - Friction Reducers

Features that reduce friction once core loop is validated.

- [x] Orchestration skill (`otto install-skills`, otto-orchestrate skill)
- [x] Auto-detect agent process exit (PID tracking + cleanup in watch)
- [x] Codex session resume (capture real thread_id from JSON)
- [x] Agent lifecycle: auto-delete on completion/exit (messages table is history)
- [x] `otto kill` - stop an agent (SIGTERM, deletes agent)
- [x] `otto interrupt` - pause an agent (SIGINT, preserves session for resume)
- [x] Agent statuses: busy/idle/blocked (replace working/waiting)
- [x] CODEX_HOME bypass for Codex agents (skip superpowers loading)
- [x] Add `--skip-git-repo-check` to Codex invocation
- [ ] TUI: 3-line task descriptions (less truncation)
- [ ] TUI: agents panel on left side
- [ ] otto should also work with codex being the orchestrator

## V2 - Full Experience

The polished multi-agent experience.

- [ ] Clickable TUI - click agent to view conversation history
- [ ] Mouse support (scroll, select, expand/collapse)
- [ ] Hierarchical todos in SQLite (orchestrator + agent level)
- [ ] Daemon with auto-wakeup on @mentions
- [ ] Super-orchestrator: attention router across multiple orchestrators
- [ ] Auto-open terminal for attach
- [ ] Web dashboard for visualization
- [ ] `--in` flag for custom orchestrator names
- [ ] `--worktree` flag for parallel agent isolation

## Design Drafts

Features being explored (not yet ready for implementation):

- `docs/plans/2025-12-23-super-orchestrator-design.md` - Event bus + gate architecture for skill enforcement, attention routing, and inter-agent communication
- `docs/plans/2025-12-22-orchestration-skill-design.md` - When to use Otto vs subagents
- `docs/plans/2025-12-22-todos-design.md` - Hierarchical todos system

## Recently Implemented

Design docs for recently completed work:

- `docs/plans/2025-12-23-agent-lifecycle-design.md` - Agent lifecycle, statuses, kill/interrupt (APPROVED)
- `docs/plans/2025-12-23-interrupt-command-plan.md` - Interrupt command implementation

## Reference Docs

- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios / test cases

## Next Up

Priority items for next session:
1. **Codex flag** - Add `--skip-git-repo-check` to reduce startup time
2. **Test interrupt with Codex** - Verify interrupt/resume flow works end-to-end
3. **TUI improvements** - 3-line task descriptions, agents panel

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

- [ ] Orchestration skill
- [ ] TUI: 3-line task descriptions (less truncation)
- [ ] TUI: agents panel on left side
- [ ] `otto kill` - stop an agent
- [ ] `otto clean` - remove DONE/FAILED agents
- [ ] `otto list` - list orchestrators
- [ ] Auto-detect agent process exit (mark done/failed when process ends)
- [ ] otto should also work for with codex being the orchestrator

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

- `docs/plans/2025-12-22-orchestration-skill-design.md` - When to use Otto vs subagents
- `docs/plans/2025-12-22-todos-design.md` - Hierarchical todos system

## Reference Docs

- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios / test cases

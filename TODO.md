# June - Subagent Viewer

A TUI for watching Claude Code subagent sessions.

## Done

- [x] Cleanup old 'otto' code (removed exec/, skills/, watch command, updated docs)
- [x] Selection mode - Click and drag in content area to select text for copy/paste

## Next Up

- [ ] See projects/branches as "channels" on left-hand panel with subagents for each project grouped underneath
- [ ] Star channel to sticky at top, rest sort by most recent activity, but any that have been active within the last 10 min or so don't change order so that they aren't constantly jumping around. Something like that... brainstorm this UX
- [ ] Character-level diff highlighting within changed lines (show specific changes, not just whole line)
- [ ] Full-width background - extend red/green background to right edge of panel
- [ ] Create a consistent color palette for the UI (centralize colors used for borders, indicators, etc.)

## Docs

- `docs/plans/2026-01-01-subagent-viewer-mvp.md` - Design doc
- `docs/plans/2026-01-01-subagent-viewer-impl-plan.md` - Implementation plan

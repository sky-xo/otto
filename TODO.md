# June - Subagent Viewer

A read-only TUI for watching Claude Code subagent sessions.

## Status: MVP Complete

The core viewer is working. See `docs/plans/2026-01-01-subagent-viewer-mvp.md` for design.

**What works:**
- Watch `~/.claude/projects/{project}/agent-*.jsonl` files
- Left panel: agent list with active/done indicators
- Right panel: transcript with markdown rendering
- Keyboard navigation (j/k, u/d page, g/G top/bottom)
- Mouse support (click, scroll)
- Auto-refresh with follow mode
- Smart timestamps

## Recent Polish (2026-01-01)

- Left panel scrolling with `↑ N more` / `↓ N more` indicators
- Hybrid vim-style `u`/`d` paging
- Markdown rendering via glamour
- User prompt styling with `▐` indicator
- Auto-scroll follow mode for active agents
- Status icon colors preserved when highlighted

## Next Up

- [ ] Cleanup old 'otto' code that isn't being used
- [ ] Selection mode - Click and drag in content area to select text for copy/paste
- [ ] Character-level diff highlighting within changed lines (show specific changes, not just whole line)
- [ ] Full-width background - extend red/green background to right edge of panel

## Docs

- `docs/plans/2026-01-01-subagent-viewer-mvp.md` - Design doc
- `docs/plans/2026-01-01-subagent-viewer-impl-plan.md` - Implementation plan

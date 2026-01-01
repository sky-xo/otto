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

## Next Up: Diff Rendering Improvements

### Batch 1: Display polish ✓
- [x] Line numbers on each diff line
- [x] Background tint (green for additions, red for deletions)
- [x] Summary line (`└ Added N lines`)
- [x] Header format: `Update(path)` instead of `Edit: path`

### Batch 2: Context and hunks ✓
- [x] Context lines - show unchanged surrounding lines (dimmed)
- [x] Proper line-by-line diffing to identify changed vs unchanged
- [x] Hunks - group changes with `...` between sections

### Batch 3: Syntax highlighting
- [ ] Syntax highlighting for code in diffs (consider chroma library)

### Batch 4: Inline diff and full-width backgrounds
- [ ] Character-level diff highlighting within changed lines (show specific changes, not just whole line)
- [ ] Full-width background - extend red/green background to right edge of panel

## Future Ideas

- **Agent naming** - Extract task description from first user message
- **Selection mode** - Click and drag in content area to select text for copy/paste
- **TodoWrite details** - Show todo items/count instead of just "TodoWrite"
- ~~**Show tool details**~~ - ✓ Done (Bash shows description + command, Edit shows diffs)

## Docs

- `docs/plans/2026-01-01-subagent-viewer-mvp.md` - Design doc
- `docs/plans/2026-01-01-subagent-viewer-impl-plan.md` - Implementation plan

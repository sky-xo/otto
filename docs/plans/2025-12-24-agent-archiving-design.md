# Agent Archiving Design

**Status:** Draft

## Summary

Make `otto status` an inbox by hiding archived agents by default. Agents move to an archived state only when the orchestrator explicitly archives them (single agent or bulk). Archived agents can be expanded in the TUI and listed with `--all`. Retention cleanup remains time-based (7 days) and independent of archive state.

## Goals

- Keep `otto status` focused on active and unacknowledged agents.
- Allow quick bulk archiving after reviewing status.
- Preserve a short history for reference without clutter.
- Support expanding/collapsing archived agents in the TUI list.

## Non-Goals

- Automatic archiving just by running `otto status`.
- Agent-driven unarchiving (e.g., `ask`/`say` from agents).
- Long-term history storage beyond retention cleanup.

## Definitions

- **Active agent:** Status in {busy, blocked} or complete/failed but not archived yet.
- **Archived agent:** Status in {complete, failed} with `archived_at` set.
- **Acknowledged:** The orchestrator explicitly archived the agent.

## Data Model

Add `archived_at` to `agents`:

```
ALTER TABLE agents ADD COLUMN archived_at DATETIME;
CREATE INDEX idx_agents_archived ON agents(archived_at) WHERE archived_at IS NOT NULL;
```

## CLI Behavior

### Status

- `otto status`: Show active agents (busy/blocked + complete/failed without `archived_at`).
- `otto status --all`: Show all agents, including archived.
- `otto status --archive`: Archive all complete/failed agents in the current status output.

Output should mark archived agents when `--all` is used.

### Archive

New command:

```
otto archive <agent-id>
```

Rules:
- Only allowed if agent status is `complete` or `failed`.
- Otherwise, return a clear error ("agent is still active" or similar).

### Unarchive (implicit)

- `otto prompt <agent-id>` and `otto attach <agent-id>` should clear `archived_at`.
- This only applies to orchestrator actions, not agent messages.

## TUI Behavior

- Left panel shows active agents by default.
- Add a selectable row: `Archived (N)`.
- Press Enter on that row to toggle expand/collapse.
- When expanded, show archived agents sorted by last activity.
- Mouse click toggles the archived row when terminal mouse is enabled.

No key commands specifically for archiving in the UI.

## Sorting

- Active list: existing status order (busy -> blocked -> complete -> failed).
- Archived list: sort by last activity descending.
- Initial implementation can use `updated_at` as last activity.
- Optionally add `last_activity_at` later if needed for higher fidelity.

## Retention

Delete agents (and their logs/messages) only after they have been archived for
7 days. Do not delete completed agents that are not archived yet.

## Implementation Notes

- Repo: add `archived_at` column + helper methods.
- Status queries: filter by `archived_at IS NULL` unless `--all` is set.
- Archive command: update `archived_at = CURRENT_TIMESTAMP`.
- Prompt/attach: clear `archived_at` on action.
- TUI: treat archived as a secondary list with a collapsible row.

## Tests

- Repo: create agent with `complete`, verify `archive` sets `archived_at`.
- Status: `--all` includes archived; default excludes.
- Archive: refuses on `busy/blocked/idle` status.
- Prompt/attach: clears `archived_at`.
- TUI: archived row toggles and list visibility updates.

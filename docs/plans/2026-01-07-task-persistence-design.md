# Task Persistence Design

> **For Claude:** This is a design doc, not an implementation plan. Use superpowers:writing-plans to create the implementation plan.

**Goal:** Persist task state so orchestrators can resume after context compaction.

**Problem:** When orchestrator compacts, it loses track of which task it was on. User has to manually remind it. This requires vigilance and is error-prone.

**Solution:** SQLite-backed task store that survives compaction. Orchestrator writes at task boundaries, reads on restart.

---

## Schema

```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,        -- e.g., "t-a3f8" (flat hash, not hierarchical)
    parent_id TEXT,             -- NULL for root tasks, references parent for children
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',  -- "open" | "in_progress" | "closed"
    notes TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,            -- soft delete (NULL = not deleted)

    FOREIGN KEY (parent_id) REFERENCES tasks(id)
);
```

**ID format:** `t-` prefix + 4-char hash. Flat IDs (not hierarchical). Database tracks parent relationship.

**Status values:** `open` (not started) → `in_progress` (working) → `closed` (done)

**Soft delete:** `deleted_at` timestamp. Deleted tasks excluded from `list` by default.

**Scoping:** Tasks scoped to git repo + branch automatically (like existing June agent channels).

---

## CLI Commands

### Create

```bash
# Create root task (parent)
june task create "Implement auth feature"
# Output: t-a3f8

# Create child task
june task create "Add middleware" --parent t-a3f8
# Output: t-7bc2

# Create multiple children
june task create "Write tests" "Update docs" --parent t-a3f8
# Output: t-9de1
# Output: t-3fg5
```

### List

Single command that shows task details + children summary:

```bash
# List root tasks
june task list
# Output:
# t-a3f8  Implement auth feature  [in_progress]  (3 children)
# t-b2c3  Fix bug #123            [open]         (2 children)

# List specific task (shows details + children)
june task list t-a3f8
# Output:
# t-a3f8 "Implement auth feature" [in_progress]
#   Note: Started work on this
#
# Children:
#   t-7bc2  Add middleware      [closed]
#   t-9de1  Write tests         [in_progress]
#   t-3fg5  Update docs         [open]

# List leaf task (shows details, no children)
june task list t-7bc2
# Output:
# t-7bc2 "Add middleware" [closed]
#   Parent: t-a3f8
#   Note: Mock server needs HTTPS
#
# No children.
```

### Update

All mutations via `update` command:

```bash
# Status
june task update t-7bc2 --status in_progress
june task update t-7bc2 --status closed
june task update t-7bc2 --status open

# Notes
june task update t-7bc2 --note "Mock server needs HTTPS"

# Title
june task update t-7bc2 --title "Add auth middleware"

# Atomic multi-field update
june task update t-7bc2 --status closed --note "Done, tested locally"
```

### Delete

```bash
# Soft delete (sets deleted_at, auto-cascades to children)
june task delete t-7bc2
june task delete t-a3f8   # deletes parent + all children
```

---

## Output Formats

Human-readable by default. Add `--json` for machine-readable output.

```bash
june task list t-a3f8 --json
june task create "New task" --json  # returns {"id": "t-xyz1"}
```

---

## Typical Workflow

### 1. Start plan

Orchestrator reads plan file, creates tasks:

```bash
june task create "Implement auth feature"   # t-a3f8
june task create "Add middleware" --parent t-a3f8
june task create "Write tests" --parent t-a3f8
june task create "Update docs" --parent t-a3f8
```

### 2. Work through tasks

```bash
june task update t-7bc2 --status in_progress
# (subagent works...)
june task update t-7bc2 --note "Mock server needs HTTPS"
june task update t-7bc2 --status closed
june task update t-9de1 --status in_progress
```

### 3. Compaction happens

Context clears. Orchestrator restarts.

### 4. Resume

```bash
june task list t-a3f8
# Shows parent task details + children with statuses
# Orchestrator sees which tasks are closed/in_progress/open
```

Orchestrator continues from where it left off.

---

## What's NOT Included (YAGNI)

- `blockedBy` dependencies (add later if needed)
- Labels/tags
- Assignee/ownership
- Date filtering
- Archive commands
- Import from markdown
- Semantic shortcuts (`start`, `complete`, `reset`) - can add as aliases later

---

## Design Decisions

**Why SQLite, not Git-backed?**
- Survives compaction (the whole point)
- Atomic writes
- No merge conflicts
- Query flexibility
- Already have `~/.june/june.db` for agents

**Why flat IDs, not hierarchical?**
- Simpler to generate (no parsing parent ID)
- Can reparent without changing ID
- Database handles relationships

**Why `t-` prefix?**
- Distinguishes from agent names
- Grep-friendly
- Low cost (2 chars)

**Why hash IDs, not sequential?**
- No coordination needed between concurrent orchestrators
- Unique across sessions

**Why parent/child hierarchy?**
- Scopes tasks to a plan/feature
- Two orchestrators on same branch don't collide
- Natural grouping for `list`

**Why unified `update` instead of `start`/`complete`?**
- Single command for all mutations
- Atomic multi-field updates (`--status closed --note "Done"`)
- Extensible (add new statuses without new commands)
- AI orchestrators don't benefit from semantic shortcuts

**Why unified `list` instead of separate `show`?**
- One command that adapts to context
- Shows task details + children summary together
- Natural hierarchy traversal

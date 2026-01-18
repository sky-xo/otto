# June Plugin Architecture Design

> **For Claude:** This is a design doc, not an implementation plan. Use superpowers:writing-plans to create the implementation plan.

**Goal:** Make June a Claude Code plugin that provides task-aware workflow skills, replacing superpowers for users who want persistent task state.

**Problem:** Claude Code's TodoWrite doesn't survive context compaction. When context resets, Claude loses track of which task it was on. June's SQLite-backed tasks persist, but there's no integration with planning/execution workflows.

**Solution:** June becomes a plugin that vendors superpowers skills and modifies the planning/execution skills to use `june task` commands instead of TodoWrite.

---

## Plugin Structure

**plugin.json:**
```json
{
  "name": "june",
  "description": "Task-aware workflow skills with persistent state",
  "version": "1.0.0"
}
```

**Directory layout:**
```
june/
├── .claude-plugin/
│   └── plugin.json
│
├── skills/                      # ASSEMBLED (committed to git)
│   │
│   │ # === JUNE CUSTOMIZATIONS ===
│   ├── writing-plans/           # fresheyes integration + june tasks
│   ├── executing-plans/         # june task status updates
│   ├── subagent-driven-development/  # model selection + june tasks
│   ├── fresheyes/               # multi-agent review
│   ├── design-review/
│   ├── review-pr-comments/
│   ├── tool-scout/
│   ├── webresearcher/
│   │
│   │ # === VENDORED FROM SUPERPOWERS ===
│   ├── brainstorming/
│   ├── dispatching-parallel-agents/
│   ├── finishing-a-development-branch/
│   ├── receiving-code-review/
│   ├── requesting-code-review/
│   ├── systematic-debugging/
│   ├── test-driven-development/
│   ├── using-git-worktrees/
│   ├── using-superpowers/
│   ├── verification-before-completion/
│   └── writing-skills/
│
├── june-skills/                 # SOURCE OF TRUTH for customizations
│   ├── writing-plans/
│   ├── executing-plans/
│   ├── subagent-driven-development/
│   ├── fresheyes/
│   ├── design-review/
│   ├── review-pr-comments/
│   ├── tool-scout/
│   └── webresearcher/
│
├── .skill-cache/                # add to .gitignore
│   └── superpowers/             # cloned superpowers repo
│
├── internal/                    # existing Go code
├── cmd/
└── Makefile
```

---

## Task Integration

When a plan is created, tasks are persisted to June. When execution progresses, task status updates.

### writing-plans changes

After saving the plan file, before the execution handoff:

```bash
# Create parent task for the plan
june task create "Implement auth feature" --json
# Returns: {"id": "t-a3f8"}

# Create child tasks for each step
june task create "Add middleware" --parent t-a3f8 --note "Step 1 of plan"
june task create "Write tests" --parent t-a3f8
june task create "Update docs" --parent t-a3f8
```

The skill outputs the parent task ID so execution can reference it:

```
Plan saved to docs/plans/2026-01-10-auth-feature.md
Tasks created: t-a3f8 (3 children)

Run `june task list t-a3f8` to see task breakdown.
```

### subagent-driven-development changes

Currently says "Read plan, extract tasks, create TodoWrite". Changes to:

```bash
# On start: read existing tasks or create from plan
june task list t-a3f8 --json

# Before each task: mark in_progress
june task update t-7bc2 --status in_progress

# After task + reviews pass: mark closed
june task update t-7bc2 --status closed --note "Implemented, tests pass"
```

### Resume after compaction

```
june task list t-a3f8

t-a3f8  "Implement auth feature"  [in_progress]
  t-7bc2  "Add middleware"        [closed]
  t-9de1  "Write tests"           [in_progress]  ← resume here
  t-3fg5  "Update docs"           [open]
```

---

## Build Process

The Makefile assembles skills from two sources:

```makefile
SUPERPOWERS_VERSION := v4.0.3
SUPERPOWERS_REPO := https://github.com/obra/superpowers

.PHONY: build-skills
build-skills:
	@# Fetch superpowers if not cached
	@[ -d .skill-cache/superpowers ] || git clone $(SUPERPOWERS_REPO) .skill-cache/superpowers
	@cd .skill-cache/superpowers && git fetch && git checkout $(SUPERPOWERS_VERSION)

	@# Clean and copy superpowers skills
	rm -rf skills/
	cp -r .skill-cache/superpowers/skills skills/

	@# Overlay June's custom skills (override)
	cp -r june-skills/* skills/

	@echo "Skills assembled: superpowers $(SUPERPOWERS_VERSION) + june overrides"

.PHONY: update-superpowers
update-superpowers:
	cd .skill-cache/superpowers && git fetch origin main && git log --oneline HEAD..origin/main
	@echo "Review changes above, then update SUPERPOWERS_VERSION and run 'make build-skills'"
```

### Workflow for editing skills

1. Edit skills in `june-skills/`
2. Run `make build-skills` to assemble
3. Commit `skills/` directory

### Workflow for updating superpowers

1. Run `make update-superpowers` to see what's new
2. Update `SUPERPOWERS_VERSION` in Makefile
3. Run `make build-skills`
4. Review `git diff skills/`
5. Commit

---

## CLI Enhancement

Add `--note` flag to `june task create`:

```bash
# Current
june task create "Add middleware" --parent t-a3f8

# With note support
june task create "Add middleware" --parent t-a3f8 --note "Check auth flow first"
```

**Implementation:** Add `--note` flag to `newTaskCreateCmd()` in `internal/cli/task.go`, pass to `db.Task{}` struct at creation.

---

## Distribution

### Phase 1: --plugin-dir

Users clone and use directly:

```bash
git clone https://github.com/sky-xo/june
cd june && make build-skills
claude --plugin-dir ./june
```

### Phase 2: Marketplace (later)

Create a marketplace for easier install:

```bash
/plugin marketplace add https://github.com/sky-xo/june-marketplace
/plugin install june@june-marketplace
```

### User migration

Uninstall superpowers before using June to avoid skill conflicts:

```bash
/plugin uninstall superpowers@superpowers-marketplace
```

---

## TodoWrite Safety Net (Optional, Defer)

A hook that syncs stray TodoWrite calls to June tasks, catching cases where Claude uses TodoWrite directly instead of a skill.

```json
// hooks/hooks.json
{
  "hooks": [{
    "event": "PostToolUse",
    "matcher": { "tool_name": "TodoWrite" },
    "command": "./scripts/sync-todo-to-june.sh"
  }]
}
```

**Behavior:**
- Parse TodoWrite JSON from stdin
- Create/update a scratch parent task
- Sync todo items as children
- Map status: `pending`→`open`, `in_progress`→`in_progress`, `completed`→`closed`

**Recommendation:** Skip for v1. Add later if needed.

---

## What's Modified vs Vendored

| Skill | Source | Changes |
|-------|--------|---------|
| `writing-plans` | june-skills/ | Fresheyes for 200+ lines, june task create |
| `executing-plans` | june-skills/ | june task update for status |
| `subagent-driven-development` | june-skills/ | Model selection + june task updates |
| `fresheyes` | june-skills/ | Custom multi-agent review |
| `design-review` | june-skills/ | Custom |
| `review-pr-comments` | june-skills/ | Custom |
| `tool-scout` | june-skills/ | Custom |
| `webresearcher` | june-skills/ | Custom |
| All others | superpowers | Vendored unchanged |

---

## Design Decisions

**Why vendor superpowers instead of depending on it?**
- Single plugin for users to install
- Clean `june:*` namespace
- Control over skill versions
- Avoids user confusion about which skill to invoke

**Why commit assembled skills/ instead of .gitignore?**
- Users get a working plugin immediately on clone
- Diff shows exactly what changed on superpowers updates
- No build step required for users

**Why june-skills/ as separate directory?**
- Clear separation of "your code" vs "vendored"
- Easy to see what you maintain
- Build script is simple overlay

**Why defer the TodoWrite hook?**
- Adds complexity
- Skills should be sufficient if used correctly
- Can add later if we see Claude bypassing skills frequently

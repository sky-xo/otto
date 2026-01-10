---
name: writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
# Based on: superpowers v4.0.3
# Customization: Fresheyes review integration, june task persistence
---

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.

Assume they are a skilled developer, but know almost nothing about our toolset or problem domain. Assume they don't know good test design very well.

**Announce at start:** "I'm using the writing-plans skill to create the implementation plan."

**Context:** This should be run in a dedicated worktree (created by brainstorming skill).

**Save plans to:** `docs/plans/YYYY-MM-DD-<feature-name>.md`

## Bite-Sized Task Granularity

**Each step is one action (2-5 minutes):**
- "Write the failing test" - step
- "Run it to make sure it fails" - step
- "Implement the minimal code to make the test pass" - step
- "Run the tests and make sure they pass" - step
- "Commit" - step

## Plan Document Header

**Every plan MUST start with this header:**

```markdown
# [Feature Name] Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

**Tech Stack:** [Key technologies/libraries]

---
```

## Task Structure

```markdown
### Task N: [Component Name]

**Files:**
- Create: `exact/path/to/file.py`
- Modify: `exact/path/to/existing.py:123-145`
- Test: `tests/exact/path/to/test.py`

**Step 1: Write the failing test**

```python
def test_specific_behavior():
    result = function(input)
    assert result == expected
```

**Step 2: Run test to verify it fails**

Run: `pytest tests/path/test.py::test_name -v`
Expected: FAIL with "function not defined"

**Step 3: Write minimal implementation**

```python
def function(input):
    return expected
```

**Step 4: Run test to verify it passes**

Run: `pytest tests/path/test.py::test_name -v`
Expected: PASS

**Step 5: Commit**

```bash
git add tests/path/test.py src/path/file.py
git commit -m "feat: add specific feature"
```
```

## Remember
- Exact file paths always
- Complete code in plan (not "add validation")
- Exact commands with expected output
- Reference relevant skills with @ syntax
- DRY, YAGNI, TDD, frequent commits

## Plan Review

After saving the plan:

**Step 1: Run quick fresheyes**

Always run a quick fresheyes self-review (baseline sanity check). Invoke the fresheyes skill with "quick" mode:

Use the Skill tool: `skill: "fresheyes", args: "quick on the plan"`

This runs a structured self-review checklist without spawning external agents.

**Step 2: Suggest based on plan size**

Check the plan line count and present options:

**If < 400 lines:**

"Plan saved to `docs/plans/<filename>.md` (N lines). Quick fresheyes complete.

Ready to proceed to execution?"

**If >= 400 lines:**

"Plan saved to `docs/plans/<filename>.md` (N lines). Quick fresheyes complete.

This is a larger plan. Recommend full fresheyes with 2x Claude for independent review.

1. **Run full fresheyes** (2x Claude, ~15 min) - recommended for plans this size
2. **Proceed to execution** - skip full review

Which approach?"

User can override either way.

## Task Persistence

After fresheyes review (if proceeding to execution), create June tasks:

```bash
# Create parent task for the plan
june task create "<Plan Title>" --json
# Returns: {"id": "t-xxxx"}

# Create child tasks for each numbered task in the plan
june task create "Task 1: <title>" --parent t-xxxx
june task create "Task 2: <title>" --parent t-xxxx
# ... for each task
```

Output to user:

```
Tasks created: t-xxxx (N children)

Run `june task list t-xxxx` to see task breakdown.
```

**Important:** The parent task ID (e.g., `t-xxxx`) must be provided to the execution skill (executing-plans or subagent-driven-development). Include it in the handoff message so the executor knows which task tree to read.

## Execution Handoff

After saving the plan, offer execution choice:

**"Plan complete and saved to `docs/plans/<filename>.md`. Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?"**

**If Subagent-Driven chosen:**
- **REQUIRED SUB-SKILL:** Use superpowers:subagent-driven-development
- Stay in this session
- Fresh subagent per task + code review

**If Parallel Session chosen:**
- Guide them to open new session in worktree
- **REQUIRED SUB-SKILL:** New session uses superpowers:executing-plans

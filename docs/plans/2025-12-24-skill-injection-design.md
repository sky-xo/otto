# Skill Injection Design

**Status:** Draft
**Created:** 2025-12-24
**Related:** [Super Orchestrator Design](./2025-12-24-super-orchestrator-design.md), [Tasks Design](./2025-12-24-tasks-design.md)

## Overview

Design for re-injecting skills and context when an orchestrator or agent wakes up after compaction or @mention.

## The Problem

When an LLM compacts (Claude Code at ~75% context, Codex at token threshold):
- It loses all bootstrapped skills
- It forgets where it was in a workflow
- It doesn't know what task it was working on
- It has to be re-taught everything

**Solution:** Store state externally (tasks table, files), re-inject on every wake-up.

## The Superpowers Skill Chain

Skills form a workflow chain. Each skill references the next:

```
using-superpowers (entry point)
       │
       ▼
brainstorming ──────────────────────────────────────────┐
  - Ask questions, present design                       │
  - Write docs/plans/YYYY-MM-DD-<topic>-design.md       │
  - Calls: using-git-worktrees, writing-plans           │
       │                                                │
       ▼                                                │
using-git-worktrees                                     │
  - Create isolated workspace                           │
  - Verify .gitignore, run setup, verify tests          │
       │                                                │
       ▼                                                │
writing-plans                                           │
  - Create bite-sized implementation plan               │
  - Save to docs/plans/YYYY-MM-DD-<feature>.md          │
  - Offer: subagent-driven OR parallel session          │
       │                                                │
       ├──────────────────┬─────────────────────────────┘
       ▼                  ▼
subagent-driven-dev  OR  executing-plans
  │                        │
  │ Per task:              │ Per batch (3 tasks):
  │ - implementer          │ - Execute steps
  │ - spec-reviewer        │ - Report for review
  │ - code-quality-rev     │ - Continue on feedback
  │                        │
  └──────────┬─────────────┘
             ▼
   finishing-a-development-branch
     - Verify tests pass
     - Present options (merge/PR/keep/discard)
     - Cleanup worktree
```

### Cross-cutting skills (used throughout)

- **test-driven-development** - RED/GREEN/REFACTOR for all implementation
- **verification-before-completion** - Evidence before claims
- **requesting-code-review** - Dispatch reviewer subagent

## How Skill Injection Works

### 1. Wake-up Trigger

Agent gets woken up via `june prompt` (triggered by @mention or event):

```bash
june prompt backend "Message from @june: Check on Task 2 progress"
```

### 2. Context Assembly

Before injecting the prompt, June assembles context:

```go
func AssembleWakeUpContext(db *sql.DB, agentID string) string {
    // 1. Get agent's current task
    task := GetActiveTaskForAgent(db, agentID)

    // 2. Determine which skill applies
    skill := DetermineSkillForTask(task)

    // 3. Read the skill file
    skillContent := ReadSkillFile(skill)

    // 4. Render task state
    taskState := RenderTasksForInjection(db, task.PlanFile)

    // 5. Get recent messages
    recentMessages := GetRecentMessages(db, agentID, 10)

    // 6. Assemble
    return fmt.Sprintf(`
## Your Current Skill
%s

## Your Current Task
%s

## Recent Messages
%s

## New Message
%s
`, skillContent, taskState, recentMessages, newMessage)
}
```

### 3. Skill Determination

The task hierarchy encodes which skill applies:

```go
func DetermineSkillForTask(task Task) string {
    content := strings.ToLower(task.Content)

    // Pattern matching on task content
    switch {
    case strings.Contains(content, "brainstorm"):
        return "superpowers:brainstorming"
    case strings.Contains(content, "worktree"):
        return "superpowers:using-git-worktrees"
    case strings.Contains(content, "write plan") || strings.Contains(content, "implementation plan"):
        return "superpowers:writing-plans"
    case strings.Contains(content, "spec review"):
        return "superpowers:subagent-driven-development"
    case strings.Contains(content, "code review") || strings.Contains(content, "quality review"):
        return "superpowers:subagent-driven-development"
    case strings.Contains(content, "finish") || strings.Contains(content, "merge") || strings.Contains(content, "pr"):
        return "superpowers:finishing-a-development-branch"
    default:
        // For implementation tasks, use TDD
        return "superpowers:test-driven-development"
    }
}
```

### 4. Alternative: Explicit Skill Field

Instead of pattern matching, tasks could have an explicit skill field:

```sql
CREATE TABLE tasks (
    -- ... existing fields ...
    skill TEXT,  -- e.g., "superpowers:brainstorming"
);
```

Then the orchestrator sets the skill when creating tasks:

```go
func CreateTaskWithSkill(db *sql.DB, content, skill string) {
    // ...
}
```

**Trade-offs:**
- Explicit: More reliable, no ambiguity
- Pattern matching: Less to track, more flexible

## Wake-up Injection Template

Full template for orchestrator wake-up:

```markdown
# Orchestrator Wake-up Context

You are @june, the orchestrator for this project.

## Current Skill
You are using `subagent-driven-development` to execute the implementation plan.

[Full skill content here]

## Active Plan
**File:** docs/plans/2025-12-24-authentication.md

[Summarized plan content or key sections]

## Current Tasks
- [x] Task 1: Add user model (@backend, completed)
- [ ] Task 2: Add login endpoint (@backend, in spec review)  <-- CURRENT
    - [x] Implementation (completed)
    - [ ] Spec review (in progress)
    - [ ] Code quality review (pending)
- [ ] Task 3: Add JWT middleware (pending)

**Current position:** Task 2 implementation is complete. Spec review is in progress.
Next action: Check if spec reviewer has completed, then proceed to code quality review.

## Recent Activity
- 10:32 - @backend: "Task 2 implementation complete, ready for review"
- 10:33 - You dispatched spec-reviewer subagent
- 10:45 - @spec-reviewer: "Review complete, approved with no issues"

## New Message
@spec-reviewer: "Spec review complete. All requirements met, no extras added."

## Your Next Action
The spec review passed. You should:
1. Mark spec review subtask complete
2. Dispatch code-quality-reviewer subagent for Task 2
3. Continue with the subagent-driven-development workflow
```

## Skill File Locations

Skills are read from the superpowers plugin cache:

```
~/.claude/plugins/cache/superpowers-marketplace/superpowers/<version>/skills/<skill-name>/SKILL.md
```

Or from project-local skills:

```
.claude/skills/<skill-name>.md
```

## Subagent Skill Injection

Subagents (implementer, spec-reviewer, code-reviewer) get injected with:

1. Their specific prompt template (e.g., `implementer-prompt.md`)
2. The task they're working on
3. Relevant context from the orchestrator

```markdown
# Implementer Subagent

You are implementing Task 2: Add login endpoint.

## Your Instructions
[Contents of implementer-prompt.md]

## The Task
**From plan:** docs/plans/2025-12-24-authentication.md

### Task 2: Add login endpoint

**Files:**
- Create: `internal/auth/login.go`
- Test: `internal/auth/login_test.go`

**Step 1:** Write failing test for login endpoint
**Step 2:** Implement minimal code to pass
**Step 3:** Verify tests pass
**Step 4:** Commit

## Context from Orchestrator
Task 1 (user model) is already complete. You can import from `internal/auth/user.go`.

## Guidelines
- Follow TDD strictly (superpowers:test-driven-development)
- Self-review before completing
- Commit after each step
```

## Implementation Phases

### Phase 1: Basic Injection
- Read skill file on wake-up
- Inject with task state
- No smart skill detection (explicit skill field on task)

### Phase 2: Smart Detection
- Pattern matching on task content
- Fall back to parent task's skill
- Skill history tracking

### Phase 3: Optimized Injection
- Only inject relevant sections of skill
- Token-aware context assembly
- Cache compiled skill content

## Open Questions

1. **Skill versioning** - What if skill changes between compactions? Use pinned version?

2. **Token budget** - Skills can be long. How much context budget for skill vs task vs messages?

3. **Skill composition** - Some tasks need multiple skills (e.g., TDD + implementation). How to handle?

4. **Subagent skills** - Do subagents get the same skills as orchestrator, or task-specific only?

---
name: otto-orchestrate
description: Use when implementation planning OR implementation work begins. Wraps superpowers skills and routes to appropriate agents (Codex via Otto, or Claude subagents).
---

# Orchestrate

Route implementation work to the right agents by wrapping superpowers skills and spawning via Otto.

**Core principle:** Superpowers defines the process, Otto decides who executes it.

**Dependency:** Requires superpowers skills to be installed.

## Settings

Agent preferences come from settings (project `.claude/settings.json` or `~/.otto/settings.yaml`):

```yaml
planning_agent: codex      # or claude
implementation_agent: codex  # or claude
```

## The Flow

```
PHASE 1: PLANNING
├── Read superpowers:writing-plans
├── Spawn agent (settings default) via Otto
├── Interactive: Agent checks in on architecture decisions
└── Output: docs/plans/YYYY-MM-DD-feature-plan.md

PHASE 2: EXECUTION
├── Read superpowers:subagent-driven-development
├── Choose agent based on complexity:
│   ├── Complex → Spawn via Otto (settings default)
│   └── Simple → Claude subagent (no Otto needed)
├── Agent follows superpowers process:
│   ├── Per task: Implement (TDD)
│   ├── Spec Compliance Review
│   ├── Code Quality Review
│   └── Next task
└── After all tasks: Final review

PHASE 3: COMPLETION
└── superpowers:finishing-a-development-branch
```

## Phase 1: Planning

1. Read `superpowers:writing-plans` skill content
2. Spawn planning agent via Otto:
   ```bash
   otto spawn codex "Write an implementation plan for [feature].

   IMPORTANT - Interactive planning:
   - Check in with me about high-level architecture decisions
   - Use 'otto ask' to get my input on key decisions
   - Handle details yourself once direction is confirmed
   - Present the final plan document when complete

   Follow this planning process:
   [writing-plans skill content]

   Design doc: docs/plans/YYYY-MM-DD-feature-design.md"
   ```
3. Monitor via `otto watch`, respond to checkpoints via `otto prompt`
4. Agent outputs plan following superpowers format

## Phase 2: Execution

**Always use `superpowers:subagent-driven-development` process.**

The process is the same regardless of agent - TDD, spec review, code quality review after each task. The only decision is WHO executes it.

**Choose agent based on complexity:**

| Complexity | Agent | How |
|------------|-------|-----|
| Simple (single file, obvious fix) | Claude subagent | Task tool directly |
| Complex (see criteria below) | Settings default | Otto spawn |

**Use Otto/Codex when:**
- Task is intricate and benefits from Codex's rigor
- Tasks can run in parallel (independent work streams)
- You want to detach and come back later
- Task complexity is high

**To spawn implementation agent:**
```bash
otto spawn codex "Implement this plan following the subagent-driven-development process:

[subagent-driven-development skill content]

Plan: docs/plans/YYYY-MM-DD-feature-plan.md

For each task:
1. Implement using TDD
2. Run spec compliance review
3. Run code quality review
4. Fix any issues, re-review until approved
5. Move to next task"
```

Monitor via `otto watch`, answer questions via `otto prompt`.

## Phase 3: Completion

After all tasks complete:
- Agent uses `superpowers:finishing-a-development-branch`
- Final verification, tests passing
- Present merge/PR options

## Example Workflow

```
User: "Implement the auth feature we designed"

Orchestrator: Design doc exists. Using otto-orchestrate skill.

[Phase 1: Planning]
→ otto spawn codex "Write implementation plan..."

Codex (via otto ask): "Key decision: passport.js or direct JWT?"
→ otto prompt planningagent "Use passport.js"

Codex: "Plan complete. See docs/plans/2025-01-15-auth-plan.md"

[Phase 2: Execution]
Orchestrator: Plan has 6 tasks, multi-file. Using Codex via Otto.
→ otto spawn codex "Implement following subagent-driven-development..."

[Agent works through tasks with built-in reviews]

Codex: "All 6 tasks complete. Tests passing."

[Phase 3: Completion]
Codex: "Using finishing-a-development-branch. Ready to merge?"
```

## Red Flags

**Never:**
- Skip the plan phase
- Forget to feed superpowers skill content to spawned agents
- Ignore agent questions (check `otto messages` regularly)

**Prefer Otto/Codex when:**
- Task is intricate and benefits from Codex's rigor
- Tasks can run in parallel
- You want to detach and come back later
- Task complexity is high

**If agent has questions:**
- Answer via `otto prompt <agent> "answer"`
- Don't rush past blockers

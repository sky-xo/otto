# Orchestration Skill Design

> **Status:** Draft - initial ideas from brainstorming session

## Overview

A Claude skill that helps decide when to use different orchestration approaches:
- **Superpowers skills** (brainstorming, writing-plans, code-review, etc.)
- **Basic Claude subagents** (Task tool, simple delegation)
- **Otto** (spawn Codex agents, persistent agents, coordination)

## Core Insight

It's not Otto OR Claude subagents - it's a hybrid. Use each tool where it's strongest.

## Decision Factors

| Factor | → Otto | → Claude subagents |
|--------|--------|-------------------|
| Need Codex? | Yes | No |
| Agents need to talk to each other? | Yes | No |
| Might need mid-task guidance? | Yes | No (requirements clear) |
| Want persistent agents across sessions? | Yes | No |

## Default Workflow

```
1. DESIGN (Claude - superpowers:brainstorming)
   Conversational exploration with user
   Output: docs/plans/YYYY-MM-DD-<feature>-design.md

2. IMPLEMENTATION PLAN (Codex via Otto)
   → otto spawn codex "Write detailed implementation plan based on design"
   Output: Detailed bite-sized tasks

3. PLAN REVIEW (Claude subagent)
   Use superpowers:code-reviewer template to review plan
   Output: Feedback on gaps, risks, improvements

4. PLAN REVISION (Codex via Otto)
   → otto spawn codex "Update plan based on review feedback"
   Output: Revised plan

5. IMPLEMENTATION (Fresh Codex via Otto)
   → otto spawn codex "Implement per plan" (fresh agent)
   Output: Working code

6. CODE REVIEW (Claude subagent or Otto)
   Could be parallel reviewers (security, architecture, tests)
```

## Configurable Preferences

Users could configure which agent type handles which task type.

**Supported agent types:**
- `claude` - Claude Code
- `codex` - OpenAI Codex
- `gemini` - Google Gemini (future)
- `all` - Run all configured agents in parallel
- `[claude, codex]` - Specify multiple agents

```yaml
# ~/.otto/preferences.yaml or project .claude/settings

agent_preferences:
  design: claude                    # high-level architecture
  implementation_plan: codex        # detailed task breakdown
  plan_review: [claude, codex]      # both review the plan
  implementation: codex             # writing code
  code_review: all                  # all agents review in parallel
  security_review: claude           # specific review types
  debugging: claude                 # systematic investigation

# Default agents available
available_agents:
  - claude
  - codex
  # - gemini  # uncomment when available
```

**When multiple agents review:**
- Spawn each in parallel via Otto
- Collect all feedback
- Orchestrator synthesizes into unified feedback

## When to Use What

**Use superpowers skills directly:**
- brainstorming - conversational design works great
- code-review templates - reusable for any review

**Use basic Claude subagents:**
- Quick research/exploration
- Single focused task with clear output
- When you need result before continuing

**Use Otto:**
- Need Codex for implementation
- Multiple agents that might coordinate
- Agents might have questions mid-task
- Want to detach and come back later

## Integration with Superpowers

This skill doesn't replace superpowers - it helps decide when to use:
- superpowers skills (where they fit)
- Otto (where Codex or persistence is needed)
- Basic subagents (simple delegation)

## Open Questions

- How does this skill get invoked? Auto at start of tasks?
- Should preferences be per-project or global?
- How to handle tasks that could go either way?

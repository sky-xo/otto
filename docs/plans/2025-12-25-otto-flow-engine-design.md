# Otto Flow Engine Design

**Status:** Draft
**Created:** 2025-12-25

**Related docs:**
- [Super Orchestrator Design](./2025-12-24-super-orchestrator-design.md) - Event bus, wake-ups, state persistence, tasks table
- [Tasks Design](./2025-12-24-tasks-design.md) - Task tracking and derived state
- [Skill Injection Design](./2025-12-24-skill-injection-design.md) - Re-injecting skills on wake-up

## Overview

This document describes the design for Otto's flow engine - the system that drives multi-agent workflows through a configurable sequence of steps, each assigned to a specific agent type (Claude or Codex).

## Problem Statement

1. **Claude skips rules** - Despite strict instructions, Claude rationalizes skipping skills and steps
2. **Codex forgets to poll** - When orchestrating, Codex forgets to check for agent responses
3. **Need reliability** - The orchestration layer must be deterministic and never skip steps

## Design Principles

1. **Harness-driven, not LLM-driven** - Otto (Go binary) controls the flow, not an LLM
2. **Data-driven flow** - Flow is defined in YAML config, not hardcoded in Go
3. **Artifact-based verification** - Check for expected outputs, not just agent claims
4. **Configurable per project** - Users can define custom flows
5. **Superpowers-aware** - Designed for future integration, not required for V0

## Architecture

### The Flow Engine

Otto acts as a workflow interpreter. The Go code is a generic engine; the flow is data:

```
┌─────────────────────────────────────────────────────────────────┐
│ OTTO FLOW ENGINE (Go binary)                                    │
│                                                                 │
│ 1. Load flow config (project → user → built-in)                 │
│ 2. Load skills (project → user → external → built-in)           │
│ 3. For each step in flow:                                       │
│    a. Inject skill + preamble into agent prompt                 │
│    b. Spawn agent (claude or codex)                             │
│    c. Wait for completion event                                 │
│    d. Run checks (artifacts, output, tests)                     │
│    e. On fail: retry or go to on_fail step                      │
│    f. On pass: proceed to next step                             │
│ 4. Complete workflow                                            │
└─────────────────────────────────────────────────────────────────┘
```

### Modes of Operation

**Daemon mode** (full orchestration):
- Run `otto` to start TUI + event bus
- Auto wake-ups on @mentions and completions
- Full flow execution with state persistence

**Standalone mode** (CLI only):
- No daemon running
- `otto spawn` still works from Claude/Codex
- Agent commands (`say`, `complete`) still work
- Orchestrator polls manually via `otto messages`
- Same agent code works either way

## Flow Configuration

### Flow Definition

```yaml
# .otto/flow.yaml (project) or ~/.otto/flow.yaml (user)

flow:
  - step: brainstorming
    agent: claude
    check:
      artifact: "docs/plans/*-design.md"

  - step: writing-plans
    agent: codex
    check:
      artifact: "docs/plans/*-plan.md"

  - step: implement
    agent: codex
    loop: tasks-from-plan
    check:
      tests: true

  - step: spec-review
    agent: claude
    check:
      output: "APPROVED"
    on_fail: implement

  - step: code-quality
    agent: claude
    check:
      output: "APPROVED"
    on_fail: implement

  - step: final-review
    agent: codex
    check:
      output: "APPROVED"

  - step: finishing
    agent: codex
```

### Step Properties

| Property | Description |
|----------|-------------|
| `step` | Step identifier (also used as skill name if `skill` not specified) |
| `agent` | Agent type: `claude` or `codex` |
| `skill` | Skill file to inject (defaults to step name) |
| `loop` | Loop source for multi-task steps (e.g., `tasks-from-plan`) |
| `check` | Verification to run after completion |
| `on_fail` | Step to go to if check fails (default: error) |
| `next` | Explicit next step (default: next in list) |

### Check Types

| Check | Description |
|-------|-------------|
| `artifact: <glob>` | File matching pattern must exist |
| `output: <keyword>` | Agent completion message must contain keyword |
| `tests: true` | Test suite must pass |
| `command: <cmd>` | Command must exit 0 |

## Skill System

### Resolution Order

```
1. Project skills      (.otto/skills/<name>.md)
2. User skills         (~/.otto/skills/<name>.md)
3. External skills     (paths from config, e.g., superpowers)
4. Built-in skills     (embedded in Otto binary)
```

First match wins. Users can override any skill by placing it higher in the chain.

### External Skills

```yaml
# ~/.otto/config.yaml
external_skills:
  - ~/.claude/commands/
  - ~/.claude/plugins/cache/superpowers-marketplace/superpowers/4.0.0/skills
```

Otto only looks for skills matching its flow steps. External sources don't inject arbitrary skills into the flow.

### Eject Workflow

```bash
otto skills eject brainstorming           # → ~/.otto/skills/brainstorming.md
otto skills eject brainstorming --project # → .otto/skills/brainstorming.md
otto skills list                          # shows where each skill resolves from
```

### Agent Preambles

Single preamble per agent type, injected at top of all skills:

```yaml
# ~/.otto/config.yaml
preambles:
  claude: ~/.otto/preamble.claude.md   # Strict: "YOU MUST..."
  codex: ~/.otto/preamble.codex.md     # Flexible: "Use judgment..."
```

This addresses the personality difference:
- Claude needs strict language to avoid skipping steps
- Codex needs flexible language to allow human overrides

## Model Assignment

Default flow with recommended agent types:

| Step | Agent | Rationale |
|------|-------|-----------|
| brainstorming | claude | Conversational, explores ideas |
| writing-plans | codex | Structured, reliable formatting |
| implement | codex | Thorough, follows TDD |
| spec-review | claude | Can reason about intent |
| code-quality | claude | Flexible judgment |
| final-review | codex | Comprehensive, thorough |
| finishing | either | Simple decision tree |

## Native Skills Bypass

- **Codex agents**: Bypass AGENTS.md via temp CODEX_HOME (current behavior)
- **Claude agents**: TBD - figure out when we add Claude orchestrator support
- **V0**: No config option, just sensible defaults

## Config File

### Location

- Project: `.otto/config.yaml`
- User: `~/.otto/config.yaml`

### Structure

```yaml
# ~/.otto/config.yaml

# External skill sources (optional)
external_skills:
  - /path/to/skills

# Agent preambles
preambles:
  claude: ~/.otto/preamble.claude.md
  codex: ~/.otto/preamble.codex.md

# Default agent for each step (can override in flow.yaml)
defaults:
  brainstorming: claude
  writing-plans: codex
  implement: codex
  spec-review: claude
  code-quality: claude
  final-review: codex
  finishing: codex
```

### Validation

- Typed schema (JSON Schema) for IDE autocomplete
- Validated on load - warn users about invalid settings
- Project config overrides user config

## Future Considerations

### Superpowers Integration

Designed with superpowers in mind but not implemented for V0:
- `external_skills` can point to superpowers folder
- Skill names may need mapping (superpowers uses different names)
- "Best effort" compatibility - users responsible for name matching

### Classifiers

V0 uses structured output parsing (e.g., "APPROVED" keyword). Future versions could add:
- LLM classifiers for fuzzy checks ("is this good enough?")
- Second-opinion verification
- Confidence scoring

### Parallel Steps

V0 is sequential. Future versions could support:
```yaml
- step: reviews
  parallel:
    - { skill: spec-review, agent: claude }
    - { skill: code-quality, agent: codex }
```

### Visual Flow Editor

Future TUI feature to visually edit flow.yaml.

## Implementation Phases

### Phase 1: Core Flow Engine
- Flow config parsing
- Step execution loop
- Basic checks (artifact, output)
- Skill resolution (built-in only)

### Phase 2: Skill System
- User/project skill overrides
- External skills support
- Eject command
- Preamble injection

### Phase 3: Advanced Features
- Loop steps (tasks-from-plan)
- Command checks
- Test checks
- State persistence across restarts

## Open Questions

1. **Skill name mapping**: How to handle superpowers skill names that don't match Otto's?
2. **Loop implementation**: How exactly does `tasks-from-plan` parse and iterate?
3. **State persistence**: Where does flow state live during execution?
4. **Error recovery**: How does user resume after a failure?

## Appendix: Alternatives Considered

### A) Full LLM Super Orchestrator
Spawn an LLM to run the whole flow. Rejected because:
- Token cost
- Compaction issues
- Step-skipping risk (the problem we're solving)

### B) Orchestrator Agent IS the Loop
Otto as event bus only, LLM orchestrator runs the loop. Rejected because:
- Same issues as A
- Codex forgets to poll (the problem we're solving)

### C) Harness + Lightweight Classifiers
Otto runs loop, uses small LLM calls for decisions. Deferred to future:
- V0 works without classifiers
- Add if structured output proves unreliable

### D) Hardcoded Go Flow
Flow logic in Go code, not config. Rejected because:
- Not configurable by users
- Can't share/customize flows


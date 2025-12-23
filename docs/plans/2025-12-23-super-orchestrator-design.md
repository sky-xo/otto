# Super Orchestrator Design

**Status:** Draft
**Created:** 2025-12-23

## Problem Statement

When using Claude Code as an orchestrator for multi-agent work, three problems emerge:

1. **Skill enforcement fails** - Despite strict instructions ("YOU MUST use skills"), Claude rationalizes skipping them, substitutes familiar tools, or takes "the path of least resistance." Even explicit user instructions get ignored.

2. **Attention routing is manual** - With multiple orchestrators across projects/branches, the human must manually track which need input, switch contexts, and route messages.

3. **Delight vs reliability** - Claude is pleasant to converse with but unreliable at following rigid workflows. Codex follows instructions better but is less conversational.

**Core insight:** Orchestration needs *obedience over intelligence*. The layer that routes and enforces doesn't need to be charming—it needs to be reliable. Enforcement must be code, not prompts.

## Approaches Considered

### A. Stricter prompting
Add more emphatic instructions to CLAUDE.md. Testing shows this doesn't work—Claude already has "YOU MUST" instructions and ignores them when convenient.

### B. Codex as orchestrator
Replace Claude with Codex for the orchestrator role. Codex follows instructions more reliably. Trade-off: less conversational, may feel less delightful for design discussions.

### C. Injection layer (Claude + enforcer)
Keep Claude as the conversational interface. A thin layer preprocesses input and injects skill prompts before Claude sees it. Claude doesn't know it's being managed. Preserves delight, adds reliability.

### D. Structural enforcement (hooks/gates)
Use Claude Code's hook system to block tool calls until skills are invoked. Hard gate, but may be annoying for simple tasks.

### E. Event-driven system with lightweight classifiers
No "super orchestrator" brain. Instead: event bus + small classifiers that detect intent and inject/route accordingly. Deterministic enforcement, zero LLM token cost for the enforcement layer.

**Recommendation:** Start with parallel V0 experiments (B and D). Build toward E (event-driven) based on learnings.

## Architecture

Based on brainstorming from both Claude and Codex agents, the converged architecture:

```
┌─────────────────────────────────────────────────────────────┐
│  Gate (deterministic enforcer)                              │
│  - Intercepts tool calls                                    │
│  - Issues capability tokens when skills invoked             │
│  - Validates tokens before allowing execution               │
│  - Zero LLM token cost                                      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Event Bus                                                  │
│  - Agents emit state (blocked, done, needs input)           │
│  - Lightweight classifiers tag events                       │
│  - Attention router aggregates across projects              │
│  - Wakes agents on @mentions (enables inter-agent chat)     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Agents (Claude/Codex)                                      │
│  - Stay smart and conversational                            │
│  - Operate within enforced boundaries                       │
│  - Can't bypass gate no matter how clever                   │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

**Gate** (`internal/gate/`)
- Intercepts all tool calls from agents
- Maintains capability table: what skills must be called for what operations
- Issues time-limited, scoped capability tokens when skills are invoked
- Validates tokens before allowing tool execution
- No LLM involved—pure code enforcement

**Capability** (`internal/gate/capability.go`)
- Permission object issued when skill is invoked
- Scoped (e.g., "file:*.md"), time-limited (e.g., 5 minutes)
- Must be presented with tool calls
- Can't be forged—validated by gate

**Event Bus** (`internal/eventbus/`)
- Pub/sub system for state changes
- Events: message_sent, agent_blocked, agent_completed, needs_attention
- Subscribers: attention router, inter-agent wake-up, metrics
- Enables agent wake-up on @mention (via `otto prompt`)

**Classifier** (`internal/classifier/`)
- Lightweight intent detection (regex or tiny model)
- Detects: "is this a brainstorm request?", "is this implementation work?"
- Injects appropriate skill prompts before agent sees request
- No heavy LLM—pattern matching or Haiku-class model

### Inter-Agent Communication

The event bus enables peer-to-peer agent communication:

1. Agent A posts message @B via `otto say`
2. Event emitted: `{type: "message", mentions: ["B"]}`
3. Event bus detects B mentioned
4. Event bus calls `otto prompt B "Message from A: ..."`
5. Agent B wakes up, sees message, can respond
6. Cycle continues

This transforms Otto from hub-and-spoke to supporting peer-to-peer coordination.

## Implementation Roadmap

### V0: Validate the hypothesis (parallel experiments)

**Experiment A: Codex as orchestrator**
- No code changes required
- Human uses `codex` as main session instead of `claude`
- Track: Does it follow skills? Spawn agents correctly? Feel usable?
- Success criteria: Noticeably better skill compliance

**Experiment B: Hook enforcement**
- Add pre-tool hook to Claude Code config
- If turn contains implementation intent but no Skill tool call → inject warning or block
- Simple keyword/pattern matching to start
- Success criteria: Catches skill-skipping before it happens

### V1: Event bus + basic enforcement

- Implement `internal/eventbus/` with SQLite-backed event log
- Implement basic attention routing (which orchestrators need input)
- Add @mention wake-up via `otto prompt`
- Wire into `otto watch` TUI

### V2: Full gate + capabilities

- Implement `internal/gate/` with capability validation
- Skills issue capability tokens
- Tool calls require valid tokens
- Classifier for intent detection and skill injection

### V3: Multi-project attention routing

- Track orchestrators across all projects/branches
- Single `otto` command as unified entry point
- TUI shows all orchestrators, highlights which need attention
- Notifications when orchestrators become blocked

## Design Considerations

### Token efficiency

Concern: Enforcement layer could multiply token usage.

Mitigations:
- **Route once, then direct** - Classifier handles initial routing, subsequent messages go directly to worker
- **Stateless classification** - Classifier doesn't maintain conversation history, just classifies current message
- **Cheap model for classification** - Use Haiku or pattern matching, reserve full models for actual work
- **Zero-cost enforcement** - Gate/capability validation is pure code, no LLM

Target: ~100-200 tokens per routing decision, not per message exchange.

### Delight preservation

If enforcement feels too rigid:
- Codex handles routing/enforcement (reliable)
- Claude workers handle conversational tasks (delightful)
- User mostly interacts with Claude, just routed through system

### Failure modes to watch

- Gate becomes bottleneck (everything waits for validation)
- Over-classification (simple questions get heavy skill injection)
- Under-classification (complex tasks slip through without skills)
- Capability expiration frustrates legitimate work

## Package Structure

```
internal/
├── gate/           # enforcement, capability validation
│   ├── gate.go
│   └── capability.go
├── eventbus/       # pub/sub for state changes
│   └── bus.go
├── classifier/     # lightweight intent detection
│   └── classifier.go
├── attention/      # multi-orchestrator attention routing
│   └── router.go
```

## Open Questions

1. **Capability granularity** - Per-file? Per-operation? Per-session? Need to find right balance between security and usability.

2. **Classifier accuracy** - How good does intent detection need to be? What's the cost of false positives vs false negatives?

3. **Event bus persistence** - How long to keep events? Memory-only vs SQLite? Need for replay?

4. **Multi-project scope** - How does attention routing work across different git repos? Shared DB or federation?

5. **Codex vs Claude routing** - When should system automatically choose Codex vs Claude for a task?

## Appendix: Architecture Brainstorm Summary

Two agents (Codex and Claude) independently brainstormed architectures. Both converged on similar recommendations:

**Codex recommended:**
1. Policy Engine + Capability Tokens - hard enforcement with minimal token overhead
2. Event Bus + Micro-Classifier - lowest token cost, "obedience over intelligence"

**Claude recommended:**
1. Kernel/Userspace + Capabilities hybrid - bulletproof enforcement, zero token cost
2. Compiler IR + Optimization Passes - can auto-inject missing skills

**Key convergence:** Both emphasized that enforcement must be deterministic code, not LLM prompts. Both proposed capability-based access control. Both suggested event-driven architecture for coordination.

Full brainstorm outputs available in conversation history.

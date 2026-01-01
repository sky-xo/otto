# June Hypotheses

This document captures the core bets June is making. These are testable claims, not finished conclusions. As we validate or invalidate each hypothesis, we'll update this doc.

## Context

June exists in a space where vanilla Claude Code + superpowers skills + hooks is already "pretty good." The question is: does building a dedicated orchestration layer provide enough value to justify the complexity?

## Hypothesis 1: Persistent Workflow State

**Claim:** Tracking flow state (brainstorm → plan → implement → review) in SQLite produces better outcomes than ad-hoc skill invocation.

**Vanilla:** No persistent state. Skills invoked ad-hoc. Agents drift.
**June:** Flow state tracked, survives restarts, stages configurable per-model.

**Test:** Do agents stay on track better with explicit flow state?

---

## Hypothesis 2: Unified Control Plane

**Claim:** One TUI with visibility into all projects/branches/agents beats managing multiple terminal tabs.

**Pain:** "Which Claude terminal is done?" Context-switching. Missing notifications.
**June:** Single pane of glass. See all agents. Know when any completes.

**Test:** Work on 3+ features across 2+ projects. Compare cognitive overhead.

---

## Hypothesis 3: Codex as Orchestrator (+ Preamble System)

**Claim:** Codex can't orchestrate in vanilla—June enables it. Making multi-model orchestration work well requires a preamble system.

**The problem:** Bootstrapping Codex with Claude's prompts doesn't work well. Different models (and different skills) need different framing.
**June enables:** Preamble system—base prompts + per-model and per-skill modifiers.

**Test:** Compare orchestrators and subagents with vs without tailored preambles.

---

## Hypothesis 4: Agent-to-Agent Communication

**Claim:** Direct peer-to-peer agent communication may beat hub-and-spoke for certain tasks.

**Hub-and-spoke:** All communication flows through orchestrator.
**Peer-to-peer:** Agents debate directly. Richer back-and-forth. Risk: divergence, false consensus.

**Test:** Two agents make plans → compare orchestrator-picks vs agents-debate-and-converge.

---

## Hypothesis 5: Resilient Subagent Execution

**Claim:** Subagents working on long tasks can hit context limits and lose state ("compaction"). An automatic recovery system makes development more pleasant.

**Vanilla:** Agent compacts mid-task → loses internal context → manual recovery: /export, /clear, re-read, figure out where it left off.
**June:** Task progress tracked in SQLite externally. Agent fails → auto-spawn replacement with incomplete tasks + recent logs. No manual intervention.

**Test:** Trigger compaction mid-task. Compare recovery effort: vanilla workflow vs June automatic.

---

## Hypothesis 6: Hierarchical Orchestration

**Claim:** Orchestrators spawning orchestrators enables workflows impractical with single-level orchestration.

**Enables:** Parallel approach exploration (two orchestrators tackle same problem differently). Domain separation (frontend/backend orchestrators). Branch-isolated experimentation.
**June:** Sub-orchestrators are just agents—messageable, visible. Cross-branch visibility via unified database.

**Test:** Two orchestrators implement same feature with different approaches. Compare and synthesize results.

---

## Hypothesis 7: Human-Subagent Collaboration

**Claim:** Allowing humans to see subagent activity and collaborate with them directly—not just the top-level orchestrator—produces better results.

**Vanilla:** Subagents are black-box workers. Sessions ephemeral—once complete, they're gone.
**June:** See any agent's activity directly. Talk to the agent to ask for follow up work. Orchestrator sees all interactions.

**Test:** Try interacting with subagents directly and see if it's a good experience.

---

## The Meta-Hypothesis

**June's overall bet:** An integrated, opinionated system for multi-agent development—with tracked flows, unified visibility, and model-aware orchestration—beats assembling parts (Claude Code + beads + superpowers) for developers doing complex, multi-project work.

**This bet wins if:**

- The conventions June encodes are actually good
- Multi-project/multi-agent workflows are common enough to justify the system
- The integration saves more time than vanilla composition costs

**This bet loses if:**

- Everyone's workflow is too different (conventions don't fit)
- Claude Code + plugins evolves faster than June can keep up
- The orchestration overhead exceeds the benefits

---

## Validation Plan

1. **Finish current TUI sprint** - Get basic visibility and control working
2. **Use June for a real external project feature** - Parallel Codex agents, review, ship
3. **Compare honestly** - Would vanilla Claude have been faster? What friction did June add or remove?
4. **Share with vibez community** - Get feedback from practitioners hitting the same problems
5. **Update this doc** - Mark hypotheses as validated, invalidated, or refined

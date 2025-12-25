# Fresh Architecture Critique: Super Orchestrator + Flow Engine

**Status:** Review Feedback
**Created:** 2025-12-25
**Reviewer:** Codex (fresh-reviewer agent)

**Context:** This is an unbiased architectural review of both design docs, conducted without seeing previous critiques. The reviewer read:
- `2025-12-24-super-orchestrator-design.md` (lines 12-350, excluding cross-references)
- `2025-12-25-otto-flow-engine-design.md` (lines 1-290, core design only, excluding previous Codex critique appendix)

---

## Overall Fit

- The event bus + wake-up model and the flow engine are directionally compatible, but they currently look like **two control planes**: SO focuses on async agent-to-agent messaging and state re-hydration, FE is a deterministic step engine. Without a clear arbitration layer, you risk **double-orchestration and conflicting sources of truth**.

- The docs assume SO wakes the orchestrator on events; FE assumes the Go harness drives the sequence. If the orchestrator is itself an agent (SO), where does FE live? Is it a separate process in the same binary? If yes, its authority should be explicit: **FE should be the one to issue spawns and gate progress**, while the LLM orchestrator becomes a conversational UI only.

## Gaps / Contradictions / Redundancy

1. **Wake-up vs polling**: FE standalone mode explicitly allows manual polling and agent commands without daemon, but SO depends on the event bus + wake-ups. You will get divergent behavior depending on whether the daemon is running. If deterministic flow is the goal, standalone mode should either be read-only or clearly marked as degraded (no guarantee of step ordering).

2. **Skill enforcement is split**: SO relies on skill injection on wake-up; FE relies on step-driven skill injection and checks. These can conflict (e.g., wake-up injects skill A while FE expects step B). Decide a single source of truth for skill selection (prefer FE step context).

3. **Tasks table derived state vs FE loop tasks-from-plan**: SO says tasks live in DB; FE says parse tasks from plan. You need a canonical tasks source (DB) and a clear flow for bootstrapping the DB from plan artifacts. Otherwise, loop behavior can diverge and compaction reinjection may not match flow execution.

4. **Agent lifecycle**: SO has spawn/busy/blocked/complete; FE treats step completion as event and runs checks. There is no explicit transition for a failed check. Is it a retry (same agent?) or a new agent? If not aligned, status reporting will be misleading.

5. **"Unified say" vs FE check output keyword**: output checks are brittle if "say" gets summarized or paraphrased by a higher layer. The check should read the agent completion payload (structured) or a local artifact.

## Highest-Risk Failure Modes

1. **Dual orchestration**: LLM orchestrator issues new spawns or step changes while FE is mid-step. Leads to race conditions, duplicated agents, and incoherent task state.

2. **Event storms and wake-up loops**: mention-based wake-ups can create repeated re-injections and re-entrant flows (agent mentions @otto during an FE step, rehydrating and potentially restarting).

3. **Compaction reinjection mismatched with flow state**: if FE does not persist its current step and loop position, rehydration can put the orchestrator in a different phase than the engine, causing out-of-order steps or repeated work.

4. **Artifact checks are too weak**: a glob existence check can be satisfied by stale files from previous runs. This can mask failures and allow flow to advance incorrectly.

5. **Multi-project DB with permissive cross-project messaging**: a bad mention or malformed address can wake the wrong agent or project, causing context leakage.

## Simpler or More Robust Alternatives

1. **Single authority**: Make FE the only entity that spawns agents and advances steps. The LLM orchestrator becomes an advisor that can request changes via a structured API, but cannot directly spawn. This eliminates double-orchestration.

2. **State machine first**: Formalize the flow as a persisted state machine (step + attempt + loop index + artifacts) in SQLite. Wake-ups always reconstruct from DB, not from the LLM summary. Compaction then becomes a no-op for correctness.

3. **Replace keyword checks with structured completion**: Require agents to emit a strict JSON completion (step_id, status, artifacts, notes). The harness validates; no LLM interpretation.

4. **Use deterministic enforcement hooks instead of skill prompts**: For Claude skip issues, block tool calls until the current step is acknowledged or a specific command is issued. This gives hard guarantees without relying on prompts.

5. **Collapse tasks-from-plan into DB**: Use a single "tasks" source and generate plan docs as a view, not the source. That also fixes compaction reinjection and FE loops.

## Fundamental Approach to Core Problems

| Problem | Proposed Solution |
|---------|-------------------|
| **Skill enforcement** | Prompting is weak. Use a tool-level gate that refuses to proceed unless the step/skill is acknowledged or a structured command is used. FE should own this, with the orchestrator having no direct tool access unless in a gated override mode. |
| **Compaction resilience** | Persist the flow state and agent state in DB. Rehydration should be a deterministic replay of state, not a summary. Let agents be stateless workers; the engine reconstructs everything. |
| **Reliable orchestration** | Combine event bus with the state machine. Events mutate DB state; the engine reacts to state transitions. That unifies SO and FE and removes the need for manual polling entirely. |

## Questions / Clarifications for Design Team

1. Should the LLM orchestrator be allowed to spawn agents directly, or should all spawns go through the flow engine?

2. Is "standalone mode" meant for production reliability, or just debugging? If the latter, consider restricting it to avoid behavioral drift.

3. What is the canonical source of tasks: plan docs or DB? If DB, how is it bootstrapped and edited?

---

## Key Takeaway

The core insight is the **split control planes problem**. Both reviewers (this fresh review and the previous biased review) independently identified this as the highest-risk architectural issue. The solution is to make the **Flow Engine the single authority** for spawning and step progression, with the event bus providing notifications and wake-ups, but not control flow.

---

# Here is the old (biased) review that didn't have the context of the superorchestrator design

Ignore this its basically just here for historical reasons

## Codex Architectural Critique (2025-12-25)

Codex was asked to provide a wide architectural critique of this design.

**Note:** Codex only reviewed this doc, not the related super orchestrator design docs. Some critiques (e.g., "state under-specified") may already be addressed by the tasks table and event bus designs in those docs.

Here are its findings:

### High Severity Issues

| Issue | Problem |
|-------|---------|
| **Brittle verification** | Artifact globs can match stale files from previous runs. `output: APPROVED` can be gamed. No run-scoped namespacing or provenance. |
| **State under-specified** | No explicit state machine, persistence model, or replay semantics. Restarts will be lossy and retries unsafe. |
| **Infinite loops possible** | `on_fail` + `loop` with no retry limits or circuit breakers = infinite cycles on flaky tests |

### Medium Severity Issues

| Issue | Problem |
|-------|---------|
| **Skill resolution ambiguity** | "First match wins" can mask unexpected skills. No version pinning or trust model. |
| **Preambles are weak** | Single preamble per agent is coarse; won't prevent step-skipping without hard verification |
| **AGENTS.md bypass default** | Bypassing by default may violate user intent/security expectations |

### Fundamental Alternatives Suggested

1. **Use a proven workflow engine** (Temporal, Argo, Prefect) - Otto as thin LLM adapter
2. **Typed state machine with event sourcing** instead of list interpreter
3. **Artifacts as source of truth** with run IDs and manifests
4. **DAG instead of linear list** - compile YAML to DAG, schedule properly
5. **LLM steps as pure functions** - inputs in, outputs out, side effects isolated

### V0 Simplification Suggestions

- Drop external skills, preambles, per-step agent defaults
- Limit checks to artifact existence + manifest
- Remove daemon mode - single CLI with `run` and `resume`
- Single retry limit, no loops, manual re-run for failures

### Prior Art to Study

- **Temporal/Cadence** - durable workflows, retries, deterministic execution
- **Argo Workflows / Tekton / GitHub Actions** - YAML workflow semantics, artifact gating
- **Prefect / Airflow** - task orchestration patterns, retries, mapping
- **Bazel / Buck / DVC** - artifact hashing, provenance, cache
- **LangGraph** - LLM graph orchestration patterns

### Controversial Opinion

> Bypassing AGENTS.md by default is a mistake. It contradicts project intent and shifts trust from user policy to the orchestrator; this should be opt-in per project.

## Claude's Take on Codex's Feedback

### The high-severity issues are real:

1. **State persistence** is the biggest gap. We hand-waved "wait for completion event" but didn't define what happens on crash/restart. This needs explicit design work.

2. **Retry limits** are an obvious miss. Should have included `max_retries` on steps. Easy fix.

3. **Artifact scoping** is smart - artifacts from run A shouldn't satisfy run B. Run IDs in paths or a manifest would fix this.

### The simplification suggestions are aggressive but maybe right for V0:

Cutting external skills and preambles for V0 would let us focus on the core engine. Add them in Phase 2. This aligns with YAGNI.

### The Temporal suggestion is interesting:

Using a real workflow engine would give us durability, retries, and replay for free. The question is: is the complexity worth it, or is a simple SQLite-backed state machine enough for V0?

### Recommendations:

1. **Add to design:** Retry limits, run-scoped artifacts, explicit state persistence model
2. **Simplify V0:** Cut external skills and preambles, focus on core engine
3. **Architecture:** Keep simple list-based engine for now, but design so we *could* swap in Temporal later if needed
4. **Revisit AGENTS.md bypass:** Consider making it opt-in rather than default

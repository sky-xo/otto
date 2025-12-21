# Otto Scenarios: Testing the Design

This document explores how otto would be used in real-world scenarios, testing our API design and identifying which superpowers skills each agent would use.

## Superpowers Skills Reference

| Skill | Purpose | Typically Used By |
|-------|---------|-------------------|
| brainstorming | Turn ideas into designs through dialogue | Orchestrator (Claude Code) |
| writing-plans | Create detailed implementation plans | Orchestrator or Planning Agent |
| executing-plans | Execute plans in batches with checkpoints | Implementation Agents |
| test-driven-development | Red-green-refactor cycle | Implementation Agents |
| systematic-debugging | Root cause investigation | Debugging Agents |
| requesting-code-review | Dispatch reviewer | Any agent after completing work |
| finishing-a-development-branch | Verify, merge/PR, cleanup | Agent finishing a feature |
| subagent-driven-development | Fresh subagent per task | Orchestrator managing sub-tasks |
| dispatching-parallel-agents | Parallel independent work | Orchestrator |
| verification-before-completion | Verify before claiming done | All agents |

---

## Scenario 1: Greenfield Feature (Design → Implement → Review → Ship)

**Context:** User wants to add OAuth authentication to their app.

### Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ #main channel                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [human] Let's add OAuth authentication                              │
│                                                                     │
│ [orchestrator] I'll use brainstorming to design this with you.      │
│                                                                     │
│ ... design conversation happens ...                                 │
│                                                                     │
│ [orchestrator] Design complete. I'll write the implementation plan  │
│                and spawn agents.                                    │
│                                                                     │
│ [orchestrator] Spawning backend and frontend agents:                │
│   → otto spawn codex "Implement OAuth backend..."                   │
│   → otto spawn codex "Implement OAuth frontend..."                  │
│                                                                     │
│ [agent-backend] Starting OAuth backend. Using executing-plans.      │
│                                                                     │
│ [agent-frontend] Starting OAuth frontend. Using executing-plans.    │
│                                                                     │
│ [agent-backend] QUESTION: Should I use passport.js or custom JWT?   │
│                 (--human flagged)                                   │
│                                                                     │
│ [orchestrator] @human - backend agent needs OAuth library decision  │
│                                                                     │
│ [human] Use passport.js with the Google and GitHub strategies       │
│                                                                     │
│ [orchestrator] → otto reply msg-123 "Use passport.js with Google    │
│                  and GitHub strategies"                             │
│                                                                     │
│ [agent-backend] Got it. Continuing with passport.js.                │
│                                                                     │
│ [agent-frontend] UPDATE: Login form complete, starting token flow.  │
│                                                                     │
│ [agent-backend] UPDATE: Passport configured, routes done.           │
│                 @agent-frontend - backend ready for integration.    │
│                                                                     │
│ [agent-frontend] Thanks! Connecting to endpoints now.               │
│                                                                     │
│ [agent-backend] COMPLETE: Backend done. PR ready.                   │
│                                                                     │
│ [agent-frontend] COMPLETE: Frontend done. PR ready.                 │
│                                                                     │
│ [orchestrator] Both agents done. Spawning review agent.             │
│   → otto spawn claude "Review OAuth implementation..."              │
│                                                                     │
│ [agent-review] Using requesting-code-review skill.                  │
│                Reviewing commits abc123..def456                     │
│                                                                     │
│ [agent-review] Review complete:                                     │
│                ✓ Strengths: Good test coverage, clean separation    │
│                ⚠ Important: Missing CSRF protection on callback     │
│                ⚠ Minor: Token refresh could use exponential backoff │
│                                                                     │
│ [orchestrator] @agent-backend - reviewer found CSRF issue.          │
│                Can you fix?                                         │
│                                                                     │
│ [agent-backend] On it. → otto say --agent agent-backend "Fixed      │
│                 CSRF, added token to callback"                      │
│                                                                     │
│ [agent-review] Re-reviewed. ✓ CSRF fixed. Approved.                 │
│                                                                     │
│ [orchestrator] Ready to merge. @human - approve?                    │
│                                                                     │
│ [human] Ship it!                                                    │
│                                                                     │
│ [orchestrator] Using finishing-a-development-branch. Merging...     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Skills Used

| Agent | Skills |
|-------|--------|
| Orchestrator | brainstorming, writing-plans, dispatching-parallel-agents |
| agent-backend | executing-plans, test-driven-development, verification-before-completion |
| agent-frontend | executing-plans, test-driven-development, verification-before-completion |
| agent-review | requesting-code-review |
| Orchestrator (final) | finishing-a-development-branch |

### Otto Commands Used

```bash
# Orchestrator spawns agents
otto spawn codex "Implement OAuth backend: passport.js, Google+GitHub..."
otto spawn codex "Implement OAuth frontend: login form, token storage..."
otto spawn claude "Review OAuth implementation for security issues"

# Agents communicate
otto say --agent agent-backend "@agent-frontend backend ready for integration"
otto ask --agent agent-backend --human "Should I use passport.js or custom JWT?"
otto update --agent agent-frontend "Login form complete, starting token flow"
otto complete --agent agent-backend "Backend done. PR ready."

# Orchestrator routes messages
otto messages
otto reply msg-123 "Use passport.js with Google and GitHub strategies"
otto status
```

### API Assessment

✅ Works well:
- @mentions for agent-to-agent coordination
- --human flag for escalation
- Parallel agents posting updates to shared channel
- Orchestrator routing messages naturally

⚠️ Potential issues:
- Agents need to know to check messages periodically (in prompt template)
- Long-running tasks might need "heartbeat" updates

---

## Scenario 2: Bug Triage & Fix

**Context:** User reports a bug - "Login sometimes fails with 500 error."

### Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ #main channel                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [human] Users are reporting intermittent 500 errors on login        │
│                                                                     │
│ [orchestrator] I'll spawn a debugging agent to investigate.         │
│   → otto spawn claude "Investigate login 500 errors..."             │
│                                                                     │
│ [agent-debug] Using systematic-debugging skill.                     │
│               Phase 1: Root cause investigation.                    │
│                                                                     │
│ [agent-debug] UPDATE: Found error in logs - race condition in       │
│               session creation. DB connection pool exhausted.       │
│                                                                     │
│ [agent-debug] UPDATE: Root cause identified:                        │
│               - Connection pool max=5, but peak load=20 concurrent  │
│               - Sessions not released on auth failure path          │
│               Spawning fix agent with specific instructions.        │
│                                                                     │
│ [orchestrator] Good investigation. @agent-debug proceed with fix    │
│                or should I spawn a separate implementation agent?   │
│                                                                     │
│ [agent-debug] I'll hand off to an implementation agent with         │
│               specific fix instructions.                            │
│   → (sends handoff message with root cause details)                 │
│                                                                     │
│ [orchestrator] → otto spawn codex "Fix connection pool leak..."     │
│                  --context "Root cause: [agent-debug's findings]"   │
│                                                                     │
│ [agent-fix] Using test-driven-development. Writing failing test     │
│             for connection leak scenario.                           │
│                                                                     │
│ [agent-fix] UPDATE: Test written, reproduces issue. Implementing    │
│             fix now.                                                │
│                                                                     │
│ [agent-fix] COMPLETE: Fixed. Pool size increased to 20, added       │
│             finally block to release connections. Tests pass.       │
│                                                                     │
│ [orchestrator] → otto spawn claude "Review connection pool fix"     │
│                                                                     │
│ [agent-review] Reviewed. ✓ Fix is correct. Good test coverage.      │
│                One suggestion: add connection pool monitoring.      │
│                                                                     │
│ [orchestrator] @human - fix ready. Reviewer suggests adding         │
│                monitoring. Want to include that or ship as-is?      │
│                                                                     │
│ [human] Ship the fix now, monitoring can be a follow-up             │
│                                                                     │
│ [orchestrator] Merging fix. Creating issue for monitoring.          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Skills Used

| Agent | Skills |
|-------|--------|
| Orchestrator | dispatching-parallel-agents |
| agent-debug | systematic-debugging |
| agent-fix | test-driven-development, verification-before-completion |
| agent-review | requesting-code-review |

### Otto Commands Used

```bash
otto spawn claude "Investigate login 500 errors. Use systematic-debugging."
otto spawn codex "Fix connection pool leak" --context "Root cause: ..."
otto spawn claude "Review connection pool fix"
otto messages
otto status
```

### API Assessment

✅ Works well:
- Clear handoff from debug agent to fix agent
- --context flag passes investigation findings
- Orchestrator asks human for go/no-go decisions

⚠️ Consider adding:
- `otto handoff --agent X --to Y "context"` for explicit handoffs?
- Or is the current say + spawn pattern sufficient?

---

## Scenario 3: Parallel Comparison ("Build-off")

**Context:** User wants to see two different approaches to implementing a feature.

### Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ #main channel                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [human] I want to add real-time notifications. Not sure whether     │
│         to use WebSockets or Server-Sent Events. Can we prototype   │
│         both and compare?                                           │
│                                                                     │
│ [orchestrator] Great idea. I'll spawn two agents in parallel.       │
│                They'll build the same feature with different        │
│                approaches, then we'll compare.                      │
│                                                                     │
│   → otto spawn codex "Implement notifications with WebSockets..."   │
│   → otto spawn codex "Implement notifications with SSE..."          │
│                                                                     │
│ [agent-ws] Starting WebSocket implementation.                       │
│                                                                     │
│ [agent-sse] Starting SSE implementation.                            │
│                                                                     │
│ [agent-ws] UPDATE: Basic connection working. Adding reconnection.   │
│                                                                     │
│ [agent-sse] UPDATE: EventSource connected. Simpler than expected!   │
│                                                                     │
│ [agent-ws] QUESTION: Should I implement binary message support      │
│            or stick to JSON? (WebSockets can do both)               │
│                                                                     │
│ [orchestrator] JSON only - we just need text notifications.         │
│                @agent-sse FYI, JSON only for fair comparison.       │
│                                                                     │
│ [agent-sse] Noted, keeping it simple.                               │
│                                                                     │
│ [agent-ws] COMPLETE: WebSocket impl done.                           │
│            - 180 lines of code                                      │
│            - Reconnection logic: 45 lines                           │
│            - Browser support: All modern                            │
│                                                                     │
│ [agent-sse] COMPLETE: SSE impl done.                                │
│            - 60 lines of code                                       │
│            - Reconnection: Built into EventSource (0 lines)         │
│            - Browser support: All modern except IE                  │
│                                                                     │
│ [orchestrator] Both complete. Spawning comparison reviewer.         │
│   → otto spawn claude "Compare WebSocket vs SSE implementations..." │
│                                                                     │
│ [agent-compare] Comparison:                                         │
│                                                                     │
│                 | WebSocket | SSE |                                 │
│                 |-----------|-----|                                 │
│                 | 180 LOC   | 60 LOC |                              │
│                 | Bidirectional | Server→Client only |              │
│                 | Manual reconnect | Auto reconnect |               │
│                 | More complex | Simpler |                          │
│                                                                     │
│                 Recommendation: SSE for this use case.              │
│                 Notifications are server→client only, SSE is        │
│                 simpler and handles reconnection automatically.     │
│                                                                     │
│ [orchestrator] @human - SSE recommended. Agree?                     │
│                                                                     │
│ [human] Makes sense. Let's go with SSE.                             │
│                                                                     │
│ [orchestrator] Merging SSE branch. Discarding WebSocket branch.     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Skills Used

| Agent | Skills |
|-------|--------|
| Orchestrator | dispatching-parallel-agents |
| agent-ws | executing-plans, test-driven-development |
| agent-sse | executing-plans, test-driven-development |
| agent-compare | (custom comparison analysis) |

### API Assessment

✅ Works well:
- Parallel spawns work naturally
- Shared channel keeps both agents aligned on constraints
- Comparison agent can read both implementations

⚠️ Consider:
- Each agent should work in separate git branch (--branch flag?)
- `otto discard <agent>` to cleanly remove rejected approach?

---

## Scenario 4: Long-Running Migration

**Context:** Migrating from REST to GraphQL across many files, over multiple sessions.

### Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ #main channel                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [human] We need to migrate our API from REST to GraphQL.            │
│         This is big - will take multiple days.                      │
│                                                                     │
│ [orchestrator] I'll create a migration plan and we'll work through  │
│                it incrementally. Using writing-plans skill.         │
│                                                                     │
│ ... planning conversation ...                                       │
│                                                                     │
│ [orchestrator] Plan saved to docs/plans/graphql-migration.md        │
│                10 phases, ~40 tasks. I'll spawn agents for          │
│                Phase 1 (schema design).                             │
│                                                                     │
│ [orchestrator] → otto spawn claude "Design GraphQL schema..."       │
│                                                                     │
│ [agent-schema] Using brainstorming for schema design.               │
│                                                                     │
│ ... Day 1 work happens ...                                          │
│                                                                     │
│ [agent-schema] COMPLETE: Schema designed. Types defined.            │
│                Phase 1 done.                                        │
│                                                                     │
│ [orchestrator] Phase 1 complete. Saving state. See you tomorrow!    │
│                                                                     │
│ --- Session ends, human comes back next day ---                     │
│                                                                     │
│ [human] Let's continue the GraphQL migration                        │
│                                                                     │
│ [orchestrator] Loading migration state...                           │
│                Phase 1: ✓ Complete                                  │
│                Phase 2: Pending (Implement User resolver)           │
│                                                                     │
│                Spawning Phase 2 agents.                             │
│                                                                     │
│ [orchestrator] → otto spawn codex "Implement User GraphQL resolver" │
│                → otto spawn codex "Implement Post GraphQL resolver" │
│                                                                     │
│ [agent-user] Starting User resolver. Using executing-plans.         │
│                                                                     │
│ [agent-post] Starting Post resolver. Using executing-plans.         │
│                                                                     │
│ ... work continues across multiple days ...                         │
│                                                                     │
│ [orchestrator] All 10 phases complete! Migration finished.          │
│                Using finishing-a-development-branch.                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Skills Used

| Agent | Skills |
|-------|--------|
| Orchestrator | brainstorming, writing-plans, dispatching-parallel-agents |
| agent-schema | brainstorming |
| agent-* (resolvers) | executing-plans, test-driven-development |
| Orchestrator (final) | finishing-a-development-branch |

### API Assessment

✅ Works well:
- Orchestrator scoping (project/branch) persists state across sessions
- Plan lives in docs/, agents reference it
- Phases can be tracked in plan document

⚠️ Consider:
- State tracking beyond just "what's done" - maybe store in otto.db?
- `otto resume` to pick up where we left off?
- Integration with TodoWrite for progress tracking

---

## Scenario 5: Ad-Hoc Code Review Across Team

**Context:** Multiple developers working, one wants review from "the AI team."

### Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ #main channel                                                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [human] I just pushed a PR for the payment integration.             │
│         Can you review it?                                          │
│                                                                     │
│ [orchestrator] I'll spawn specialized reviewers.                    │
│                                                                     │
│ → otto spawn claude "Security review of payment PR #142"            │
│ → otto spawn claude "Architecture review of payment PR #142"        │
│ → otto spawn claude "Test coverage review of payment PR #142"       │
│                                                                     │
│ [agent-security] Reviewing for security issues...                   │
│                                                                     │
│ [agent-arch] Reviewing architecture and patterns...                 │
│                                                                     │
│ [agent-tests] Reviewing test coverage...                            │
│                                                                     │
│ [agent-security] COMPLETE: Security review                          │
│   ❌ Critical: API key hardcoded in payment.ts:45                   │
│   ⚠️ Important: Missing rate limiting on payment endpoint           │
│   ✓ Good: Proper input validation on amounts                        │
│                                                                     │
│ [agent-arch] COMPLETE: Architecture review                          │
│   ✓ Good: Clean separation of payment logic                         │
│   ⚠️ Suggestion: Consider strategy pattern for payment providers    │
│                                                                     │
│ [agent-tests] COMPLETE: Test coverage review                        │
│   ⚠️ Important: No tests for refund flow                            │
│   ⚠️ Important: Missing edge case for zero-amount payments          │
│   ✓ Good: Happy path well covered                                   │
│                                                                     │
│ [orchestrator] Reviews complete. Summary:                           │
│                                                                     │
│                CRITICAL (must fix):                                 │
│                - Hardcoded API key (security)                       │
│                                                                     │
│                IMPORTANT (should fix):                              │
│                - Missing rate limiting                              │
│                - No refund tests                                    │
│                - No zero-amount tests                               │
│                                                                     │
│                SUGGESTIONS (consider):                              │
│                - Strategy pattern for providers                     │
│                                                                     │
│ [human] Oh damn, good catch on the API key. I'll fix these.         │
│                                                                     │
│ [orchestrator] Want me to spawn an agent to help with the fixes?    │
│                                                                     │
│ [human] Nah, I'll do these myself. Thanks!                          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Skills Used

| Agent | Skills |
|-------|--------|
| Orchestrator | dispatching-parallel-agents |
| agent-security | requesting-code-review (security focus) |
| agent-arch | requesting-code-review (architecture focus) |
| agent-tests | requesting-code-review (testing focus) |

### API Assessment

✅ Works well:
- Parallel specialized reviewers
- All results visible in shared channel
- Orchestrator synthesizes across reviewers

⚠️ Consider:
- Review agents need PR context (--pr flag?)
- Could use `gh pr diff` in agent prompt
- Aggregation is manual - could be automated

---

## API Refinements Based on Scenarios

### Working Well

1. **Group chat model** - Shared visibility works great for coordination
2. **@mentions** - Natural way to direct attention
3. **--human flag** - Clear escalation path
4. **Parallel spawns** - Easy to parallelize independent work
5. **COMPLETE/UPDATE messages** - Good status visibility

### Consider Adding

1. **`--branch <name>`** - Each agent works in its own branch
   ```bash
   otto spawn codex "..." --branch feature/oauth-backend
   ```

2. **`--pr <number>`** - Give agent context about a PR
   ```bash
   otto spawn claude "Review PR" --pr 142
   ```

3. **`otto resume <orchestrator>`** - Explicitly resume previous work
   ```bash
   otto resume  # resume current project/branch orchestrator
   ```

4. **`otto handoff --to <agent>`** - Explicit handoff with context
   ```bash
   otto handoff --agent agent-debug --to agent-fix "Root cause: ..."
   ```

5. **State tracking** - Track phase/progress in otto.db, not just messages

### Guidelines Refinements

1. **Check messages frequency:** Agents should check `otto messages --unread` after each major step, not just periodically.

2. **When to use #main vs DM:**
   - #main: Status updates, questions, completions, anything others might need
   - DM: Quick clarifications between two agents that don't need visibility

3. **Heartbeat updates:** For long-running tasks (>5 min), agents should post UPDATE every few minutes so orchestrator knows they're alive.

4. **Branch discipline:** Each agent should work in its own branch when making code changes, merge only after review.

---

## Summary

The otto API handles these scenarios well. Main refinements:
- Add `--branch` and `--pr` flags for common patterns
- Consider explicit `otto handoff` command
- Add heartbeat guidance to agent prompts
- State tracking beyond messages for long-running work

The group chat model with @mentions provides natural coordination without complex routing logic.

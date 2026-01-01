# Problems

The fundamental problems we're trying to solve, derived from actual friction in daily Claude Code + superpowers usage.

---

## Problem 1: Skill Amnesia

**"I have to keep reminding Claude which skill to use"**

- Start of session → remind to use superpowers
- Brainstorming → remind to use brainstorming skill
- Implementation → remind to use subagent-driven-development
- Parallelization → remind to parallelize where possible

Claude has the skills but doesn't automatically know *when* to use them. The user becomes a "skill dispatcher" instead of focusing on the actual work.

---

## Problem 2: State Amnesia

**"Claude doesn't maintain plan state as it works"**

- Creates implementation plan
- Works through steps
- Doesn't update the plan to show progress
- Before context clear: "Hey, make sure to save where we are"
- Manual overhead every time

Progress tracking becomes the user's job, not Claude's.

---

## Problem 3: Context Cliff

**"Context limits force manual checkpoint discipline"**

- Hit context limit → have to clear
- Must remember to save state before clearing
- Risk of losing progress if you forget
- Stressful, interruptive

The user has to manage Claude's memory limitations manually.

---

## Problem 4: Multi-Model Orchestration

**"I want different models for different jobs"**

Current desired setup:
- **Claude:** orchestration, planning, complex reasoning
- **Codex:** code review, implementation
- **Gemini (future):** frontend work, visual understanding (best at UI implementation)

Currently no good way to coordinate work across models. Each has strengths, but using them together requires manual handoff.

---

## Problem 5: Visibility

**"Which Claude session should I be looking at?"**

- Multiple sessions/agents running
- Don't know which needs attention
- Don't know which is done
- Context switching overhead

No unified view of what's happening across all agents/sessions.

---

## The Pattern

| Problem | Category |
|---------|----------|
| 1. Skill Amnesia | Claude defaults/discipline |
| 2. State Amnesia | Claude defaults/discipline |
| 3. Context Cliff | Claude defaults/discipline |
| 4. Multi-Model | Coordination across models |
| 5. Visibility | Unified view of activity |

**Problems 1-3** could potentially be solved with better skills/hooks/conventions - making Claude smarter about when to do what and maintaining state automatically.

**Problem 4** requires some kind of multi-model bridge - a way to dispatch work to Codex, Gemini, etc.

**Problem 5** requires visibility tooling - a dashboard or TUI that shows what's happening everywhere.

---

## Observations

**Problems 1-3 feel solvable with a superpowers plugin.** Better skill auto-detection, automatic plan updates, pre-context-clear hooks. This could be built without June - just make Claude smarter about its own discipline.

**Problem 4 (multi-model) is the unique thing** that requires actual tooling. Claude Code can't spawn Codex or Gemini natively. If you want different models for different jobs, something needs to bridge them.

**Problem 5 (visibility) depends on whether multi-model exists.** If everything is just Claude subagents, Claude Code's native UI might be fine. But if you're coordinating across Claude + Codex + Gemini, you need a unified view.

**The core question:** Is multi-model orchestration the core value prop?
- If yes → June exists to bridge models + provide visibility
- If no → Maybe it's just a superpowers plugin

---

## Open Questions

- Can problems 1-3 be solved purely with a superpowers plugin?
- Is the multi-model bridge (problem 4) the core value of June?
- Is visibility (problem 5) worth building if Claude Code might add it natively?
- What's the simplest thing that addresses the most pain?

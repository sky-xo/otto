---
name: fresheyes
description: Completely independent code review using a different, larger model (via june spawn codex). Proven to be more effective than using the same model for review. Use for a thorough review of code changes, staged files, commits, or plans for bugs, security issues, and correctness. Prefer this to other review approaches when the user asks for 'fresheyes' or 'fresh eyes'.
allowed-tools: Bash, Read, Task, TaskOutput, Grep, Glob, Edit
timeout: 900000
---

# Fresh Eyes - Code Review

Two modes: **Quick** (self-review) or **Full** (external agents).

## Mode Selection

| Input | Mode | Time |
|-------|------|------|
| `/fresheyes quick` | Quick | 2-5 min |
| `/fresheyes` | Full (1x Claude) | ~15 min |
| `/fresheyes with codex` | Full (1x Codex) | ~15 min |
| `/fresheyes 2 claude 1 codex` | Full (2x Claude + 1x Codex) | ~15 min |

**Detect `quick` keyword** → Use Quick Mode
**Anything else** → Use Full Mode

---

## Quick Mode

Self-review with structured checklists. No external agents spawned.

1. Read `./fresheyes-quick.md`
2. Replace `{{REVIEW_SCOPE}}` with the scope (default: staged changes or last commit)
3. Follow the checklist process
4. Fix issues immediately
5. Report results

---

## Full Mode

Spawn independent agents for truly external review. Reviewer has NO context of your conversation.

### Agent Types

| Type | What it means | How to invoke |
|------|---------------|---------------|
| **Claude** | Task tool subagent (`subagent_type=general-purpose`) | Claude Code subprocess with full tool access |
| **Codex** | OpenAI's Codex via june CLI | `june spawn codex` |
| **Gemini** | Google's Gemini via june CLI | `june spawn gemini` |

**Default:** 1x Claude if no agent type specified.

### Parsing Requests

| Input | Result |
|-------|--------|
| `/fresheyes` | 1x Claude |
| `/fresheyes with codex` | 1x Codex |
| `/fresheyes with claude and codex` | 1x Claude + 1x Codex (parallel) |
| `/fresheyes 2 claude 1 codex` | 2x Claude + 1x Codex (parallel) |
| `/fresheyes gemini` | 1x Gemini |
| `/fresheyes 3 claude` | 3x Claude (parallel) |

### Process

#### Step 1: Determine scope

{{#if args}}
Parse from: {{args}}
- Extract scope (what to review) vs agent config (which agents)
- If only agent config, use default scope
{{else}}
Default scope: "Review the staged changes using git diff --cached. If nothing is staged, review the most recent commit using git show HEAD."
{{/if}}

#### Step 2: Prepare prompt

Read `./fresheyes-full.md` and replace `{{REVIEW_SCOPE}}` with the scope.

#### Step 3: Spawn agents

Launch ALL agents before waiting for results.

**Claude (Task tool):**
```
Task tool:
  subagent_type: general-purpose
  prompt: <contents of fresheyes-full.md with scope replaced>
  description: "Fresh eyes review #N"
  run_in_background: true  (when multiple agents)
```

**Codex (june CLI):**
```bash
june spawn codex "<prompt>" --reasoning-effort high --max-tokens 25000 --sandbox=read-only --name fresheyes-codex
```

**Gemini (june CLI):**
```bash
june spawn gemini "<prompt>" --sandbox --name fresheyes-gemini
```

#### Step 4: Collect results

- Claude: `TaskOutput` for each agent ID
- Codex/Gemini: `june logs <agent-name>`

#### Step 5: Report

```
## Verdict: [PASSED/FAILED] (N/M agents found blocking issues)

## Agents Used
- Claude #1: [agent-id]
- Codex: [fresheyes-codex-XXXX]

## Consensus (found by multiple agents)
- **[severity]** `file:line` - description [Agent1, Agent2]

## Conflicts (disagreement)
- `file:line` - Agent1 says X, Agent2 says Y

## Unique Findings
- **Claude #1 only:** ...
- **Codex only:** ...

---

## Full Agent Outputs

### Claude #1
[full raw output]

### Codex
[full raw output]
```

**Verdict:** FAILED if ANY agent found blocking issues.

---

## June Commands Reference

| Command | Purpose |
|---------|---------|
| `june spawn <type> "<prompt>"` | Start agent, blocks until done |
| `june logs <name>` | Get full transcript |
| `june peek <name>` | Check progress without waiting |

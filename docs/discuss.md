# Otto: Topics for Further Discussion

This file captures topics we haven't fully resolved yet, for continuation in future sessions.

## Resolved Topics

### 1. `otto handoff` Command - SKIPPED

**Decision:** Don't add this. The orchestrator stays in control of spawning agents.

**Rationale:** The orchestrator has visibility that agents don't have (e.g., could monitor agent context usage). Keeping the orchestrator in the loop for all spawns is a feature, not a limitation. Current flow works fine:
1. Agent posts findings to #main
2. Orchestrator reads them
3. Orchestrator spawns next agent with `--context`

---

### 2. Channels Within Orchestrators - SKIPPED

**Decision:** Stick with one orchestrator per branch. Multiple work streams = multiple branches.

**Rationale:** Simpler model. If checking multiple orchestrators becomes painful, can add `otto attention --all` later to aggregate across them.

---

### 3. Message Filtering - INCLUDED IN V0

**Decision:** Keep as designed:
```bash
otto messages              # unread only (default)
otto messages --all        # everything
otto messages --last 20    # recent
otto messages --from agent-abc  # from specific agent
otto messages --questions  # only questions needing answers
```

---

### 4. Package Name - DECIDED

**Decision:** `otto`. May prefix with GitHub org/user later if npm conflicts.

---

### 5. API Simplification - DECIDED

**Decision:** Dropped `otto update`. Agents have three commands:
- `otto say` - post to channel
- `otto ask` - ask a question (sets agent to WAITING)
- `otto complete` - mark task done

---

## Key Decisions Summary

1. **Architecture:** CLI tool called via Bash, not MCP server.
2. **Storage:** SQLite in `~/.otto/orchestrators/<project>/<branch>/otto.db`
3. **Messaging:** Single shared message stream. @mentions for attention. No DMs.
4. **Agent identification:** Explicit `--id <agent>` flag on agent commands. Orchestrator commands reject `--id`.
5. **Worktrees:** `--worktree <name>` flag creates isolated workspace in `.worktrees/`.
6. **Orchestrator scoping:** Auto-detect from project dir + git branch. Override with `--in <name>`.
7. **Ephemeral orchestrator model:** Conversations are disposable, state lives in otto.db and plan documents.
8. **Compatibility:** Works inside packnplay containers. Complements superpowers skills.
9. **Session resume:** Both Claude Code (`--resume`) and Codex (`codex resume`) support session resume.
10. **No handoff command:** Orchestrator controls all spawns.
11. **No channels:** One orchestrator per branch.
12. **Simplified agent commands:** `say`, `ask`, `complete` (no `update`).
13. **`otto prompt` for orchestrator→agent:** Direct to agent, not in chat. Wakes up idle agents.
14. **Chat is for agent-to-agent:** Orchestrator doesn't need to post prompts to chat (already has context).
15. **`otto watch` for v0:** Simple message tail. Full TUI dashboard + daemon in v2+.
16. **Go over Node:** Single binary, fast startup, great for CLI. Bubbletea for TUI (v2+).
17. **Orchestrator can post to chat:** `otto say "..."` without `--id`. Useful for broadcasts. `@all` wakes all agents (when daemon exists in v2+).
18. **`--id` flag determines role:** Agent commands require `--id`. Orchestrator commands reject `--id`. Simple enforcement.
19. **Seed → Sprout → Tree scoping:** V0 = core loop + manual polling. V1 = worktrees, kill/clean. V2+ = daemon, dashboard, super-orchestrator.

---

## Superpowers Integration

| Agent Role | Skills Used |
|------------|-------------|
| Orchestrator | brainstorming, writing-plans, dispatching-parallel-agents |
| Implementation agent | executing-plans, test-driven-development |
| Debug agent | systematic-debugging |
| Review agent | requesting-code-review |
| All agents | verification-before-completion |

---

## Files in This Project

- `docs/design.md` - Main design document (comprehensive)
- `docs/scenarios.md` - 5 usage scenarios testing the API
- `docs/discuss.md` - This file (resolved topics + decisions)

---

## Next Steps

1. Initialize npm package
2. Implement Phase 1 MVP (see design.md)
3. Test against scenarios
4. Iterate based on real usage

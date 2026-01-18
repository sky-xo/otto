# June Plugin Follow-up: Open Questions & Future Work

## Testing Needed

### Does june task persistence actually help?

**Status:** Untested in real conditions

**The theory:** June tasks survive context compaction, unlike TodoWrite. After compaction, `june task list <parent-id>` shows exactly where you left off.

**What we observed:** In our first implementation session using the system, context never compacted. We used both june tasks AND TodoWrite, which was redundant. The june tasks added overhead without demonstrating their value.

**To validate:**
1. Run a long session (3+ hours) that triggers compaction
2. Observe: Can Claude resume correctly from `june task list` output?
3. Compare: What happens with TodoWrite alone after compaction?

---

## Open Questions

### 1. Is the overhead worth it?

Current overhead per task:
- `june task update <id> --status in_progress` (start)
- `june task update <id> --status closed --note "..."` (end)
- Plus the initial `june task create` calls

**vs TodoWrite:** Single tool call to update status

**Question:** Is 2x the commands worth it for compaction resilience?

### 2. Should we auto-sync TodoWrite → june tasks?

The design doc mentions a hook that catches TodoWrite calls and syncs to june tasks. This would:
- Eliminate duplicate commands
- Let Claude use familiar TodoWrite
- Persist state transparently

**Tradeoff:** More magic, harder to debug

### 3. What makes june tasks better than TodoWrite?

| Feature | TodoWrite | june task |
|---------|-----------|-----------|
| Survives compaction | No | Yes |
| UI visibility | Built-in | None (CLI only) |
| Notes per task | No | Yes |
| Hierarchical | No | Yes (parent/child) |
| Cross-session | No | Yes |
| Command overhead | Low | Higher |

**The pitch:** june tasks are for *serious* multi-session work. TodoWrite is fine for simple stuff.

**But is that true?** We need to test.

---

## Ideas to Explore

### 0. Full tree dump flag

Add a flag like `--full` or `--verbose` to `june task list` that shows the entire task tree with all notes:

```bash
june task list t-a6891 --full
```

Output:
```
t-a6891 "Implement June Plugin" [closed]
  Note: All 9 tasks completed

  t-7a70d "Task 1: Add --note flag" [closed]
    Note: Implemented, spec compliant, code quality approved

  t-2a8e9 "Task 2: Create plugin infrastructure" [closed]
    Note: Implemented, spec compliant, code quality approved

  t-a88d0 "Task 3: Create june-skills directory" [closed]
    Note: Implemented, spec compliant, code quality approved

  ... (all children with all notes)
```

**Why:** After compaction, Claude needs the full context dump to understand where things stand. Current output is too terse - you'd need multiple commands to get the full picture.

**Implementation:** Recursively fetch all children, include notes, format with indentation.

### 1. TUI integration
Show june task progress in the TUI alongside agent activity. Would make the overhead feel more worthwhile.

### 2. Automatic task creation from plans
Parse the plan file and auto-create the task tree instead of manual `june task create` calls.

### 3. Status line integration
Show current task in Claude Code's status line (if possible).

### 4. Compaction-aware prompting
When context compacts, automatically inject `june task list` output into the resumed context.

---

## Action Items

- [ ] Run a real long session to test compaction recovery
- [ ] Measure: How often does compaction actually happen in typical workflows?
- [ ] Decide: Keep both systems or pick one?
- [ ] Consider: Is the TodoWrite sync hook worth building?

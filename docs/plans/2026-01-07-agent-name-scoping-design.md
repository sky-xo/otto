# Agent Naming: Auto-Generate with Optional Override

## Problem

Agent names are globally unique, but the current behavior is suboptimal:
1. `--name` is required - friction when you just want to spawn quickly
2. On collision: unclear error message
3. On success: silent (no confirmation of what was created)

## Design

### Make --name Optional

If `--name` is not provided, auto-generate a name like `task-f3WlaB`:
- Prefix: `task`
- Suffix: 6-character alphanumeric (base62: a-z, A-Z, 0-9)

```bash
$ june spawn codex "fix the auth bug"
task-f3WlaB
```

### Keep Global Uniqueness

Names remain globally unique. No scoping to repo/branch - it adds complexity without enough benefit. The TUI already shows branch context visually.

### Error on Collision (when --name provided)

When a user-specified name already exists, error with a helpful suggestion:

```
Error: agent "bugfix" already exists (spawned 2 hours ago)
Hint: use --name bugfix-2 or another unique name
```

Auto-generated names retry with a new random suffix if collision occurs (extremely unlikely).

### Output Name on Success

Always print the assigned name to stdout:

```bash
$ june spawn codex "fix auth" --name fix-auth
fix-auth

$ june spawn codex "fix auth"
task-k9Xm2P
```

Confirms what was created. Useful for scripting and for Claude to capture the name.

### Exact Match for peek/logs

`june peek bugfix` finds only "bugfix", not "bugfix-2". Simple and predictable.

## Summary

| Aspect | Current | New |
|--------|---------|-----|
| --name flag | Required | Optional (auto-generate if omitted) |
| Auto-generated format | N/A | `task-{6-char-base62}` |
| Uniqueness scope | Global | Global (no change) |
| On collision (user name) | Unclear error | Helpful error with suggestion |
| On collision (auto name) | N/A | Retry with new suffix |
| Success output | Silent | Print assigned name |
| peek/logs lookup | Exact match | Exact match (no change) |

## Implementation

1. Add `generateName()` function that creates `task-{random6}`
2. Make `--name` optional in spawn command
3. Update collision handling: error with suggestion for user names, retry for auto names
4. Print name on successful spawn

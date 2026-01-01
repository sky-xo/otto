# Codex Subscription Authentication Design

Status: Draft

## Overview

Enable June to leverage users' existing ChatGPT subscription (Plus/Pro/Max) for Codex agents, avoiding the need for separate OpenAI API billing.

---

## Background

OpenAI's Codex CLI supports two authentication methods:

1. **ChatGPT OAuth** - Uses subscription tokens (Plus: $20/mo, Pro: $200/mo)
2. **API Key** - Pay-as-you-go credits

When users run `codex login`, the CLI:
- Opens browser for OAuth flow (PKCE)
- Performs token exchange to obtain an API key
- Stores credentials in `~/.codex/auth.json`
- Auto-refreshes tokens every 28 days

This means June can spawn Codex processes that automatically inherit the user's subscription auth.

---

## Credential Storage

Codex stores credentials in one of two locations:

| Mode | Location | Notes |
|------|----------|-------|
| File | `~/.codex/auth.json` | Default fallback, mode 0600 |
| Keyring | OS keyring | Service: "Codex MCP Credentials" |

Controlled by `credential_store_mode` in `~/.codex/config.toml`:
- `auto` - Try keyring, fall back to file
- `keyring` - Keyring only (error if unavailable)
- `file` - File only

---

## Subscription Limits

| Plan | 5-Hour Limit | Weekly Cap | Notes |
|------|--------------|------------|-------|
| Plus ($20/mo) | 30-150 messages | ~6-7 sessions | Users report hitting limits quickly |
| Pro ($200/mo) | 300-1500 messages | Higher | Still has caps despite marketing |
| Max | Higher | Higher | Enterprise tier |

Limits are enforced server-side through OAuth tokens. When exhausted:
- User can wait for reset (5-hour rolling window)
- User can purchase additional API credits
- User can switch to API key auth (pay-as-you-go)

---

## Proposed Implementation

### Phase 1: Detection

June detects if user has authenticated Codex:

```go
func HasCodexAuth() bool {
    // Check auth.json exists
    authPath := filepath.Join(os.Getenv("HOME"), ".codex", "auth.json")
    if _, err := os.Stat(authPath); err == nil {
        return true
    }

    // Alternatively, use CLI status check
    // `codex login status` exits 0 if authenticated
    cmd := exec.Command("codex", "login", "status")
    return cmd.Run() == nil
}
```

### Phase 2: Transparent Spawn

When spawning Codex agents, June simply invokes `codex` normally. The CLI handles auth automatically:

```go
func SpawnCodexAgent(task string) error {
    // Codex inherits auth from ~/.codex/auth.json
    cmd := exec.Command("codex", "--quiet", task)
    // ... spawn logic
}
```

### Phase 3: Limit Monitoring

Expose subscription status to orchestrator:

```go
func GetCodexStatus() (*CodexStatus, error) {
    // Parse output of `codex /status` or similar
    // Return remaining limits, reset time, etc.
}
```

### Phase 4: Fallback Strategy

When subscription limits are hit:

1. **Notify orchestrator** - Signal that Codex agents are rate-limited
2. **Queue pending tasks** - Don't fail, just wait
3. **Offer API fallback** - Prompt user to enable API key billing
4. **Auto-switch** (optional) - If user has API key configured, switch automatically

---

## User Experience

### First Run (No Auth)

```
$ june watch --ui

June detected Codex is not authenticated.
Run `codex login` to use your ChatGPT subscription,
or set OPENAI_API_KEY for API billing.
```

### With Subscription Auth

```
$ june watch --ui

Using ChatGPT Pro subscription for Codex agents
Remaining: 847/1500 messages (5h window)
Weekly: 62% remaining
```

### Limit Hit

```
$ june watch --ui

Codex subscription limit reached. Options:
1. Wait 2h 34m for reset
2. Switch to API billing (requires OPENAI_API_KEY)
3. Continue with Claude agents only
```

---

## Configuration

Add to June config:

```toml
[codex]
# Auth preference: "subscription", "api", "auto"
# auto = use subscription if available, fall back to API
auth_mode = "auto"

# Show subscription status in UI
show_limits = true

# Auto-fallback to API when subscription exhausted
api_fallback = false
```

---

## Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| No headless auth | Initial login requires browser | Document requirement, detect and prompt |
| Rate limits vary | Hard to predict availability | Monitor and surface limits in UI |
| Token refresh | 28-day expiry | Codex handles automatically |
| Limit detection | No official API | Parse CLI output or watch for errors |

---

## Security Considerations

- June never reads or modifies `~/.codex/auth.json` directly
- Auth is handled entirely by Codex CLI
- Spawned processes inherit auth through normal Codex mechanisms
- No credentials are logged or exposed

---

## References

- [Codex CLI and Sign in with ChatGPT](https://help.openai.com/en/articles/11381614-codex-cli-and-sign-in-with-chatgpt)
- [Using Codex with your ChatGPT plan](https://help.openai.com/en/articles/11369540-using-codex-with-your-chatgpt-plan)
- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference/)
- [Codex Configuration](https://developers.openai.com/codex/local-config/)
- [Headless Auth Issue #3820](https://github.com/openai/codex/issues/3820)

---

## Next Steps

1. Validate auth detection approach works reliably
2. Determine best way to surface limit status (CLI parsing vs error detection)
3. Design orchestrator behavior when limits are hit
4. Implement Phase 1 (detection) as proof of concept

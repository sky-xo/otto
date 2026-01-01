# June Interrupt Command Implementation Plan

> **For Claude:** Use subagent-driven-development to implement.

**Goal:** Add `june interrupt <agent-id>` command that sends SIGINT to gracefully stop an agent while preserving its session for resume.

**Tech Stack:** Go, Cobra, syscall

---

### Task 1: Add interrupt command

**Files:**
- Create: `internal/cli/commands/interrupt.go`
- Modify: `internal/cli/root.go`

**Requirements:**
1. New command `june interrupt <agent-id>`
2. Look up agent by ID, get PID
3. Send SIGINT to process (syscall.SIGINT)
4. Update agent status to `idle`
5. Post "[agent-id] INTERRUPTED" message to stream
6. Print confirmation

**Implementation:**

```go
func NewInterruptCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "interrupt <agent-id>",
        Short: "Interrupt an agent (can be resumed later)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // Reject --id flag
            if cmd.Flags().Changed("id") {
                return errors.New("interrupt is an orchestrator command")
            }

            conn, err := openDB()
            if err != nil {
                return err
            }
            defer conn.Close()

            return runInterrupt(conn, args[0])
        },
    }
    return cmd
}

func runInterrupt(db *sql.DB, agentID string) error {
    // Get agent
    agent, err := repo.GetAgent(db, agentID)
    if err != nil {
        return fmt.Errorf("agent not found: %s", agentID)
    }

    // Check PID exists
    if !agent.Pid.Valid {
        return fmt.Errorf("agent %s has no PID", agentID)
    }

    // Send SIGINT
    proc, err := os.FindProcess(int(agent.Pid.Int64))
    if err != nil {
        return fmt.Errorf("find process: %w", err)
    }
    if err := proc.Signal(syscall.SIGINT); err != nil {
        return fmt.Errorf("send SIGINT: %w", err)
    }

    // Update status to idle
    if err := repo.UpdateAgentStatus(db, agentID, "idle"); err != nil {
        return fmt.Errorf("update status: %w", err)
    }

    // Post message
    msg := repo.Message{
        ID:           uuid.New().String(),
        FromID:       agentID,
        Type:         "system",
        Content:      "INTERRUPTED",
        MentionsJSON: "[]",
        ReadByJSON:   "[]",
    }
    _ = repo.CreateMessage(db, msg)

    fmt.Printf("Interrupted agent %s\n", agentID)
    return nil
}
```

**Wire into root.go:**
```go
rootCmd.AddCommand(commands.NewInterruptCmd())
```

**Test approach:**
- Unit test runInterrupt with mock DB
- Test error cases: agent not found, no PID

---

### Notes

- SIGINT (not SIGTERM) preserves Codex session
- Agent stays in DB with status=idle for resume
- Different from kill which deletes the agent

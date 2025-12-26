package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"otto/internal/process"
	"otto/internal/repo"
	"otto/internal/scope"
	"otto/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for new messages in real-time",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunWatchDefault(cmd.Context())
		},
	}

	return cmd
}

// RunWatchDefault runs the watch command with default settings.
// Used by both "otto watch" and "otto" (no subcommand).
func RunWatchDefault(ctx context.Context) error {
	conn, err := openDB()
	if err != nil {
		return err
	}
	defer conn.Close()

	if term.IsTerminal(int(os.Stdout.Fd())) {
		return tui.Run(conn)
	}

	return runWatch(ctx, conn)
}

func runWatch(ctx context.Context, db *sql.DB) error {
	scopeCtx := scope.CurrentContext()
	fmt.Println("Watching for messages... (Ctrl+C to stop)")

	var lastSeenID string

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Clean up stale agents
		cleanupStaleAgents(db, scopeCtx)

		// Build filter
		filter := repo.MessageFilter{
			Project: scopeCtx.Project,
			Branch:  scopeCtx.Branch,
		}
		if lastSeenID != "" {
			filter.SinceID = lastSeenID
		}

		// Fetch new messages
		messages, err := repo.ListMessages(db, filter)
		if err != nil {
			return err
		}

		// Print new messages
		for _, m := range messages {
			fmt.Printf("[%s] %s: %s\n", m.Type, m.FromAgent, m.Content)
			lastSeenID = m.ID
		}

		// Sleep before next poll
		time.Sleep(1 * time.Second)
	}
}

func cleanupStaleAgents(db *sql.DB, scopeCtx scope.Context) {
	filter := repo.AgentFilter{
		Project:         scopeCtx.Project,
		Branch:          scopeCtx.Branch,
		IncludeArchived: false,
	}
	agents, err := repo.ListAgents(db, filter)
	if err != nil {
		return
	}
	for _, a := range agents {
		if a.Status == "busy" && a.Pid.Valid {
			if !process.IsProcessAlive(int(a.Pid.Int64)) {
				// Mark agent as failed
				_ = repo.SetAgentFailed(db, a.Project, a.Branch, a.Name)

				// Post exit message mentioning orchestrator for wakeup
				orchestratorMention := fmt.Sprintf("%s:%s:otto", scopeCtx.Project, scopeCtx.Branch)
				mentionsJSON := fmt.Sprintf(`["%s"]`, orchestratorMention)

				msg := repo.Message{
					ID:           fmt.Sprintf("%s-exit-%d", a.Name, time.Now().Unix()),
					Project:      scopeCtx.Project,
					Branch:       scopeCtx.Branch,
					FromAgent:    a.Name,
					Type:         "exit",
					Content:      "process died unexpectedly",
					MentionsJSON: mentionsJSON,
					ReadByJSON:   "[]",
				}
				_ = repo.CreateMessage(db, msg)
			}
		}
	}
}

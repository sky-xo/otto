package commands

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

func NewWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for new messages in real-time",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runWatch(cmd.Context(), conn)
		},
	}
	return cmd
}

func runWatch(ctx context.Context, db *sql.DB) error {
	fmt.Println("Watching for messages... (Ctrl+C to stop)")

	var lastSeenID string

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Build filter
		filter := repo.MessageFilter{}
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
			fmt.Printf("[%s] %s: %s\n", m.Type, m.FromID, m.Content)
			lastSeenID = m.ID
		}

		// Sleep before next poll
		time.Sleep(1 * time.Second)
	}
}

// nextSince returns the ID to use for the next poll.
// This helper exists to support testing.
func nextSince(lastID string) string {
	return lastID
}

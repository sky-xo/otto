package commands

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"otto/internal/repo"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var completeID string

func NewCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete [summary]",
		Short: "Mark task as done (sets agent to DONE)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if completeID == "" {
				return errors.New("--id is required for complete command")
			}

			content := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runComplete(conn, completeID, content)
		},
	}
	cmd.Flags().StringVar(&completeID, "id", "", "Agent ID completing the task (required)")
	cmd.MarkFlagRequired("id")
	return cmd
}

func runComplete(db *sql.DB, fromID, content string) error {
	mentions := parseMentions(content)
	mentionsJSON, err := json.Marshal(mentions)
	if err != nil {
		return fmt.Errorf("marshal mentions: %w", err)
	}

	msg := repo.Message{
		ID:           uuid.New().String(),
		FromID:       fromID,
		Type:         "complete",
		Content:      content,
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}

	if err := repo.CreateMessage(db, msg); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	if err := repo.SetAgentComplete(db, fromID); err != nil {
		return fmt.Errorf("set agent complete: %w", err)
	}

	return nil
}

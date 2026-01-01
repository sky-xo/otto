package commands

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"june/internal/repo"
	"june/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var completeID string

func NewCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete [summary]",
		Short: "Mark task as done (sets agent to DONE)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if completeID == "" {
				return errors.New("--id is required for complete command")
			}

			content := ""
			if len(args) > 0 {
				content = args[0]
			}

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
	ctx := scope.CurrentContext()

	// Only create completion message if content is provided
	if content != "" {
		mentions := parseMentions(content, ctx)
		mentionsJSON, err := json.Marshal(mentions)
		if err != nil {
			return fmt.Errorf("marshal mentions: %w", err)
		}

		msg := repo.Message{
			ID:           uuid.New().String(),
			Project:      ctx.Project,
			Branch:       ctx.Branch,
			FromAgent:    fromID,
			Type:         "complete",
			Content:      content,
			MentionsJSON: string(mentionsJSON),
			ReadByJSON:   "[]",
		}

		if err := repo.CreateMessage(db, msg); err != nil {
			return fmt.Errorf("create message: %w", err)
		}
	}

	if err := repo.SetAgentComplete(db, ctx.Project, ctx.Branch, fromID); err != nil {
		return fmt.Errorf("set agent complete: %w", err)
	}

	return nil
}

package commands

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var askID string

func NewAskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask a question (sets agent to WAITING)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if askID == "" {
				return errors.New("--id is required for ask command")
			}

			content := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runAsk(conn, askID, content)
		},
	}
	cmd.Flags().StringVar(&askID, "id", "", "Agent ID asking the question (required)")
	cmd.MarkFlagRequired("id")
	return cmd
}

func runAsk(db *sql.DB, fromID, content string) error {
	ctx := scope.CurrentContext()
	mentions := parseMentions(content, ctx)
	mentionsJSON, err := json.Marshal(mentions)
	if err != nil {
		return fmt.Errorf("marshal mentions: %w", err)
	}

	msg := repo.Message{
		ID:        uuid.New().String(),
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		FromAgent: fromID,
		Type:      "question",
		Content:   content,
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}

	if err := repo.CreateMessage(db, msg); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	if err := repo.UpdateAgentStatus(db, ctx.Project, ctx.Branch, fromID, "blocked"); err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}

	return nil
}

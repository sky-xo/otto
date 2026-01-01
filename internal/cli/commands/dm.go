package commands

import (
	"database/sql"
	"fmt"

	"june/internal/repo"
	"june/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var dmFrom string
var dmTo string

func NewDMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dm [message]",
		Short: "Send a direct message to another agent",
		Long:  "Send a direct message from one agent to another. Wakes the target agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dmFrom == "" {
				return fmt.Errorf("--from flag is required")
			}
			if dmTo == "" {
				return fmt.Errorf("--to flag is required")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx := scope.CurrentContext()
			return runDM(conn, ctx.Project, ctx.Branch, dmFrom, dmTo, args[0])
		},
	}
	cmd.Flags().StringVar(&dmFrom, "from", "", "Sender agent name (required)")
	cmd.Flags().StringVar(&dmTo, "to", "", "Recipient agent (supports branch:agent format)")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}

func runDM(db *sql.DB, project, branch, from, to, content string) error {
	msg := repo.Message{
		ID:           uuid.New().String(),
		Project:      project,
		Branch:       branch,
		FromAgent:    from,
		ToAgent:      sql.NullString{String: to, Valid: true},
		Type:         "dm",
		Content:      content,
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}

	return repo.CreateMessage(db, msg)
}

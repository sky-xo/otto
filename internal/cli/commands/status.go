package commands

import (
	"database/sql"
	"fmt"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

var (
	statusAll     bool
	statusArchive bool
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List agents and their statuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runStatus(conn, statusAll, statusArchive)
		},
	}
	cmd.Flags().BoolVar(&statusAll, "all", false, "Include archived agents")
	cmd.Flags().BoolVar(&statusArchive, "archive", false, "Archive complete/failed agents shown")
	return cmd
}

func runStatus(db *sql.DB, includeArchived, archive bool) error {
	agents, err := repo.ListAgentsFiltered(db, includeArchived)
	if err != nil {
		return err
	}

	for _, a := range agents {
		suffix := ""
		if includeArchived && a.ArchivedAt.Valid {
			suffix = " (archived)"
		}
		fmt.Printf("%s [%s]: %s - %s%s\n", a.ID, a.Type, a.Status, a.Task, suffix)
	}

	if archive {
		for _, a := range agents {
			if a.ArchivedAt.Valid {
				continue
			}
			if a.Status != "complete" && a.Status != "failed" {
				continue
			}
			if err := repo.ArchiveAgent(db, a.ID); err != nil {
				return fmt.Errorf("archive agent %q: %w", a.ID, err)
			}
		}
	}

	return nil
}

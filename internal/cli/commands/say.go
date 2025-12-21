package commands

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"otto/internal/config"
	"otto/internal/db"
	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var sayID string

func NewSayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "say [message]",
		Short: "Post a message to the shared channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := args[0]
			fromID := sayID
			if fromID == "" {
				fromID = "orchestrator"
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runSay(conn, fromID, content)
		},
	}
	cmd.Flags().StringVar(&sayID, "id", "", "Agent ID posting the message")
	return cmd
}

func runSay(db *sql.DB, fromID, content string) error {
	mentions := parseMentions(content)
	mentionsJSON, err := json.Marshal(mentions)
	if err != nil {
		return fmt.Errorf("marshal mentions: %w", err)
	}

	msg := repo.Message{
		ID:           uuid.New().String(),
		FromID:       fromID,
		Type:         "say",
		Content:      content,
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}

	return repo.CreateMessage(db, msg)
}

func parseMentions(content string) []string {
	re := regexp.MustCompile(`@([a-z0-9-]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		if len(match) > 1 {
			mention := match[1]
			if !seen[mention] {
				seen[mention] = true
				result = append(result, mention)
			}
		}
	}

	return result
}

func openDB() (*sql.DB, error) {
	repoRoot := scope.RepoRoot()
	if repoRoot == "" {
		return nil, errors.New("not in a git repository")
	}

	branch := scope.BranchName()
	if branch == "" {
		branch = "main"
	}

	scopePath := scope.Scope(repoRoot, branch)
	if scopePath == "" {
		return nil, errors.New("could not determine scope")
	}

	dbPath := filepath.Join(config.DataDir(), "orchestrators", scopePath, "otto.db")
	return db.Open(dbPath)
}

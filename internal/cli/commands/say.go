package commands

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	ctx := scope.CurrentContext()
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
		Type:         "say",
		Content:      content,
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}

	return repo.CreateMessage(db, msg)
}

var mentionRe = regexp.MustCompile(`@([A-Za-z0-9._/-]+(?::[A-Za-z0-9._/-]+){0,2})`)

func parseMentions(content string, ctx scope.Context) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var out []string
	for _, match := range matches {
		parts := strings.Split(match[1], ":")
		resolved := ""
		switch len(parts) {
		case 1:
			// @agent -> project:branch:agent
			resolved = fmt.Sprintf("%s:%s:%s", ctx.Project, ctx.Branch, strings.ToLower(parts[0]))
		case 2:
			// @branch:agent -> project:branch:agent
			resolved = fmt.Sprintf("%s:%s:%s", ctx.Project, parts[0], strings.ToLower(parts[1]))
		case 3:
			// @project:branch:agent -> project:branch:agent
			resolved = fmt.Sprintf("%s:%s:%s", parts[0], parts[1], strings.ToLower(parts[2]))
		}
		if resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	return out
}

func openDB() (*sql.DB, error) {
	dbPath := filepath.Join(config.DataDir(), "otto.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	return db.Open(dbPath)
}

package commands

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"june/internal/config"
	"june/internal/db"
	"june/internal/scope"
)

func openDB() (*sql.DB, error) {
	dbPath := filepath.Join(config.DataDir(), "june.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	return db.Open(dbPath)
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

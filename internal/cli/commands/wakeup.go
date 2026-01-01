package commands

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"june/internal/repo"
	"june/internal/scope"
)

// wakeupSender is an interface for sending wakeup notifications to agents
type wakeupSender interface {
	SendTo(agent, context string) error
}

// parseMentionsFromJSON extracts mentions from a JSON array string
func parseMentionsFromJSON(mentionsJSON string) []string {
	var mentions []string
	if err := json.Unmarshal([]byte(mentionsJSON), &mentions); err != nil {
		return []string{}
	}
	return mentions
}

// buildContextBundle creates a context string from messages for wakeup notifications
func buildContextBundle(db *sql.DB, ctx scope.Context, msgs []repo.Message) (string, error) {
	if len(msgs) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("Recent messages:\n\n")

	for _, msg := range msgs {
		builder.WriteString(fmt.Sprintf("[%s] %s: %s\n", msg.Type, msg.FromAgent, msg.Content))
	}

	return builder.String(), nil
}

// processWakeups scans for mentions in messages and triggers wakeups for mentioned agents
func processWakeups(db *sql.DB, ctx scope.Context, w wakeupSender) error {
	// Get all messages for this context
	msgs, err := repo.ListMessages(db, repo.MessageFilter{
		Project: ctx.Project,
		Branch:  ctx.Branch,
	})
	if err != nil {
		return err
	}

	if len(msgs) == 0 {
		return nil
	}

	// Build context bundle once for all wakeups
	contextText, err := buildContextBundle(db, ctx, msgs)
	if err != nil {
		return err
	}

	// Track which agents we've already woken up to avoid duplicates
	wokenAgents := make(map[string]bool)

	// Process each message for mentions
	for _, msg := range msgs {
		mentions := parseMentionsFromJSON(msg.MentionsJSON)
		for _, mention := range mentions {
			// Skip if we've already woken this agent
			if wokenAgents[mention] {
				continue
			}

			// Send wakeup
			if err := w.SendTo(mention, contextText); err != nil {
				return err
			}

			wokenAgents[mention] = true
		}
	}

	return nil
}

package commands

import (
	"database/sql"
	"fmt"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

var (
	messagesFromID   string
	messagesQuestions bool
	messagesLast     int
	messagesMention  string
	messagesID       string
)

func NewMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "List messages from the shared channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runMessages(conn)
		},
	}
	cmd.Flags().StringVar(&messagesFromID, "from", "", "Filter by sender agent ID")
	cmd.Flags().BoolVarP(&messagesQuestions, "questions", "q", false, "Filter to only questions")
	cmd.Flags().IntVar(&messagesLast, "last", 0, "Return last N messages (read + unread)")
	cmd.Flags().StringVar(&messagesMention, "mentions", "", "Filter to messages that @mention an agent")
	cmd.Flags().StringVar(&messagesID, "id", "", "Reader ID for unread filtering")
	return cmd
}

func runMessages(db *sql.DB) error {
	// Parse flags to build filter
	msgType := ""
	if messagesQuestions {
		msgType = "question"
	}

	// Only apply unread filtering if --id is provided AND --last is not set
	readerID := messagesID
	if messagesLast > 0 {
		readerID = "" // --last means show all messages, ignore read status
	}
	filter := parseMessagesFlags(messagesFromID, msgType, messagesLast, readerID)
	filter.Mention = messagesMention

	// List messages
	messages, err := repo.ListMessages(db, filter)
	if err != nil {
		return err
	}

	// Print messages
	for _, m := range messages {
		fmt.Printf("[%s] %s: %s\n", m.Type, m.FromID, m.Content)
	}

	// Mark as read if reader ID provided and messages were returned
	if messagesID != "" && len(messages) > 0 {
		var messageIDs []string
		for _, m := range messages {
			messageIDs = append(messageIDs, m.ID)
		}
		if err := repo.MarkMessagesRead(db, messageIDs, messagesID); err != nil {
			return fmt.Errorf("mark messages read: %w", err)
		}
	}

	return nil
}

func parseMessagesFlags(fromID, msgType string, last int, readerID string) repo.MessageFilter {
	return repo.MessageFilter{
		FromID:   fromID,
		Type:     msgType,
		Limit:    last,
		ReaderID: readerID,
	}
}

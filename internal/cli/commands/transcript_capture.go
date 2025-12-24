package commands

import (
	"database/sql"
	"strings"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"

	"github.com/google/uuid"
)

func storePrompt(db *sql.DB, agentID, prompt string) error {
	msg := repo.Message{
		ID:           uuid.NewString(),
		FromID:       "orchestrator",
		ToID:         sql.NullString{String: agentID, Valid: true},
		Type:         "prompt",
		Content:      prompt,
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		return err
	}
	return repo.CreateLogEntry(db, agentID, "in", "", prompt)
}

func consumeTranscriptEntries(db *sql.DB, agentID string, output <-chan ottoexec.TranscriptChunk, onStdoutLine func(string)) <-chan error {
	done := make(chan error, 1)
	go func() {
		var stdoutBuffer string
		for chunk := range output {
			if err := repo.CreateLogEntry(db, agentID, "out", chunk.Stream, chunk.Data); err != nil {
				done <- err
				return
			}
			if onStdoutLine != nil && chunk.Stream == "stdout" {
				stdoutBuffer += chunk.Data
				for {
					newline := strings.IndexByte(stdoutBuffer, '\n')
					if newline == -1 {
						break
					}
					line := strings.TrimSpace(stdoutBuffer[:newline])
					stdoutBuffer = stdoutBuffer[newline+1:]
					if line != "" {
						onStdoutLine(line)
					}
				}
			}
		}
		if onStdoutLine != nil {
			line := strings.TrimSpace(stdoutBuffer)
			if line != "" {
				onStdoutLine(line)
			}
		}
		done <- nil
	}()
	return done
}

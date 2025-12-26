package commands

import (
	"database/sql"
	"strings"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
)

// storePrompt stores a prompt message and log entry.
// summary is displayed in the main channel, fullPrompt is stored in the agent's transcript.
func storePrompt(db *sql.DB, ctx scope.Context, agentID, summary, fullPrompt string) error {
	msg := repo.Message{
		ID:        uuid.NewString(),
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		FromAgent: "orchestrator",
		ToAgent:   sql.NullString{String: agentID, Valid: true},
		Type:      "prompt",
		Content:   summary,
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		return err
	}

	// Get agent to get type
	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
	if err != nil {
		return err
	}

	entry := repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: agentID,
		AgentType: agent.Type,
		EventType: "input",
		Content:   sql.NullString{String: fullPrompt, Valid: true},
	}
	return repo.CreateLogEntry(db, entry)
}

func consumeTranscriptEntries(db *sql.DB, ctx scope.Context, agentID string, output <-chan ottoexec.TranscriptChunk, onEvent func(CodexEvent)) <-chan error {
	done := make(chan error, 1)
	go func() {
		// Get agent type once at the start
		agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
		if err != nil {
			done <- err
			return
		}

		var stdoutBuffer string
		for chunk := range output {
			entry := repo.LogEntry{
				Project:   ctx.Project,
				Branch:    ctx.Branch,
				AgentName: agentID,
				AgentType: agent.Type,
				EventType: "output",
				ToolName:  sql.NullString{String: chunk.Stream, Valid: true},
				Content:   sql.NullString{String: chunk.Data, Valid: true},
			}
			if err := repo.CreateLogEntry(db, entry); err != nil {
				done <- err
				return
			}
			if onEvent != nil && chunk.Stream == "stdout" {
				stdoutBuffer += chunk.Data
				for {
					newline := strings.IndexByte(stdoutBuffer, '\n')
					if newline == -1 {
						break
					}
					line := strings.TrimSpace(stdoutBuffer[:newline])
					stdoutBuffer = stdoutBuffer[newline+1:]
					if line != "" {
						event := ParseCodexEvent(line)
						if event.Type != "" {
							onEvent(event)
						}
					}
				}
			}
		}
		if onEvent != nil {
			line := strings.TrimSpace(stdoutBuffer)
			if line != "" {
				event := ParseCodexEvent(line)
				if event.Type != "" {
					onEvent(event)
				}
			}
		}
		done <- nil
	}()
	return done
}

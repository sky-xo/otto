package commands

import (
	"database/sql"
	"testing"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"
)

func TestPromptStoresPromptAndResumesAgent(t *testing.T) {
	db := openTestDB(t)

	agent := repo.Agent{
		ID:        "researcher",
		Type:      "claude",
		Task:      "task",
		Status:    "complete",
		SessionID: sql.NullString{String: "session-1", Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := repo.SetAgentComplete(db, agent.ID); err != nil {
		t.Fatalf("set agent complete: %v", err)
	}

	chunks := make(chan ottoexec.TranscriptChunk)
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 1234, chunks, func() error { return nil }, nil
		},
	}

	if err := runPrompt(db, runner, "researcher", "Continue the task"); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}

	updated, err := repo.GetAgent(db, "researcher")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if updated.Status != "complete" {
		t.Fatalf("expected status complete, got %q", updated.Status)
	}
	if !updated.CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}

	msgs, err := repo.ListMessages(db, repo.MessageFilter{Type: "prompt", ToID: "researcher"})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 prompt message, got %d", len(msgs))
	}
	if msgs[0].Content != "Continue the task" {
		t.Fatalf("expected prompt message content, got %q", msgs[0].Content)
	}

	entries, err := repo.ListLogs(db, "researcher", "")
	if err != nil {
		t.Fatalf("list transcript entries: %v", err)
	}

	var inCount int
	for _, entry := range entries {
		switch entry.Direction {
		case "in":
			inCount++
			if entry.Content != "Continue the task" {
				t.Fatalf("expected input transcript to match prompt, got %q", entry.Content)
			}
		}
	}

	if inCount != 1 {
		t.Fatalf("expected 1 input transcript entry, got %d", inCount)
	}
}

func TestPromptCapturesOutput(t *testing.T) {
	db := openTestDB(t)

	agent := repo.Agent{
		ID:        "writer",
		Type:      "claude",
		Task:      "task",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-2", Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	chunks := make(chan ottoexec.TranscriptChunk, 1)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: "done\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 1234, chunks, func() error { return nil }, nil
		},
	}

	if err := runPrompt(db, runner, "writer", "Continue"); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}

	entries, err := repo.ListLogs(db, "writer", "")
	if err != nil {
		t.Fatalf("list transcript entries: %v", err)
	}

	var outCount int
	for _, entry := range entries {
		if entry.Direction == "out" {
			outCount++
		}
	}

	if outCount != 1 {
		t.Fatalf("expected 1 output transcript entry, got %d", outCount)
	}
}

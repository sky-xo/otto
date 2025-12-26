package commands

import (
	"database/sql"
	"strings"
	"testing"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"

	"github.com/google/uuid"
)

func TestWorkerSpawnCapturesPromptAndLogs(t *testing.T) {
	// 1) Set up temp DB, create agent row, store prompt message
	db := openTestDB(t)

	agent := repo.Agent{
		Project:   "test-project",
		Branch:    "main",
		Name:      "test-worker",
		Type:      "claude",
		Task:      "test task",
		Status:    "busy",
		SessionID: sql.NullString{String: uuid.New().String(), Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Store prompt message
	promptMsg := repo.Message{
		ID:           uuid.New().String(),
		Project:      "test-project",
		Branch:       "main",
		FromAgent:    "orchestrator",
		ToAgent:      sql.NullString{String: "test-worker", Valid: true},
		Type:         "prompt",
		Content:      "Test prompt content",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, promptMsg); err != nil {
		t.Fatalf("create prompt message: %v", err)
	}

	// 2) Run worker spawn with a fake runner that emits transcript chunks
	chunks := make(chan ottoexec.TranscriptChunk, 3)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: "worker output line 1\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stderr", Data: "worker stderr\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: "worker output line 2\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 9999, chunks, func() error { return nil }, nil
		},
	}

	// Run the worker spawn
	err := runWorkerSpawn(db, runner, "test-worker")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// 3) Assert logs contain prompt (in) + output (out)
	entries, err := repo.ListLogs(db, "test-project", "main", "test-worker", "")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}

	// Count entries by event type
	var inputCount, outputCount int
	var foundPrompt bool
	for _, entry := range entries {
		if entry.EventType == "input" || entry.EventType == "prompt" {
			inputCount++
			if entry.Content.Valid && strings.Contains(entry.Content.String, "Test prompt content") {
				foundPrompt = true
			}
		} else if entry.EventType == "output" || entry.EventType == "tool_use" {
			outputCount++
		}
	}

	if inputCount < 1 {
		t.Fatalf("expected at least 1 input log entry (prompt), got %d", inputCount)
	}
	if !foundPrompt {
		t.Fatal("expected to find prompt content in input logs")
	}

	// Verify agent status was updated to complete
	updatedAgent, err := repo.GetAgent(db, "test-project", "main", "test-worker")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if updatedAgent.Status != "complete" {
		t.Fatalf("expected status 'complete', got %q", updatedAgent.Status)
	}

	// Verify exit message was created
	exitMsgs, err := repo.ListMessages(db, repo.MessageFilter{Type: "exit", FromAgent: "test-worker"})
	if err != nil {
		t.Fatalf("list exit messages: %v", err)
	}
	if len(exitMsgs) != 1 {
		t.Fatalf("expected 1 exit message, got %d", len(exitMsgs))
	}
}

func TestWorkerSpawnCapturesThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   "test-project",
		Branch:    "main",
		Name:      "test-codex-worker",
		Type:      "codex",
		Task:      "test codex task",
		Status:    "busy",
		SessionID: sql.NullString{String: placeholderID, Valid: true}, // placeholder
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Store prompt message
	promptMsg := repo.Message{
		ID:           uuid.New().String(),
		Project:      "test-project",
		Branch:       "main",
		FromAgent:    "orchestrator",
		ToAgent:      sql.NullString{String: "test-codex-worker", Valid: true},
		Type:         "prompt",
		Content:      "Test codex prompt",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, promptMsg); err != nil {
		t.Fatalf("create prompt message: %v", err)
	}

	// Mock runner that simulates Codex JSON output with thread.started event
	chunks := make(chan ottoexec.TranscriptChunk, 5)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"thread.started","thread_id":"thread_xyz789"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 8888, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn for Codex agent
	err := runWorkerSpawn(db, runner, "test-codex-worker")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify thread_id was captured and stored as session_id
	updatedAgent, err := repo.GetAgent(db, "test-project", "main", "test-codex-worker")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if updatedAgent.SessionID.String != "thread_xyz789" {
		t.Fatalf("expected session_id to be 'thread_xyz789', got %q", updatedAgent.SessionID.String)
	}
	if updatedAgent.Status != "complete" {
		t.Fatalf("expected status 'complete', got %q", updatedAgent.Status)
	}
}

func TestWorkerSpawnCodexWithoutThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   "test-project",
		Branch:    "main",
		Name:      "test-codex-worker-no-thread",
		Type:      "codex",
		Task:      "test codex task without thread_id",
		Status:    "busy",
		SessionID: sql.NullString{String: placeholderID, Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Store prompt message
	promptMsg := repo.Message{
		ID:           uuid.New().String(),
		Project:      "test-project",
		Branch:       "main",
		FromAgent:    "orchestrator",
		ToAgent:      sql.NullString{String: "test-codex-worker-no-thread", Valid: true},
		Type:         "prompt",
		Content:      "Test codex prompt",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, promptMsg); err != nil {
		t.Fatalf("create prompt message: %v", err)
	}

	// Mock runner with NO thread.started event
	chunks := make(chan ottoexec.TranscriptChunk, 3)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 7777, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn - should succeed despite missing thread_id
	err := runWorkerSpawn(db, runner, "test-codex-worker-no-thread")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify agent completed successfully (but session_id should still be placeholder)
	updatedAgent, err := repo.GetAgent(db, "test-project", "main", "test-codex-worker-no-thread")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if updatedAgent.SessionID.String != placeholderID {
		t.Fatalf("expected session_id to remain placeholder %q, got %q", placeholderID, updatedAgent.SessionID.String)
	}
	if updatedAgent.Status != "complete" {
		t.Fatalf("expected status 'complete', got %q", updatedAgent.Status)
	}
}

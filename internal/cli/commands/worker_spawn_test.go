package commands

import (
	"database/sql"
	"strings"
	"testing"

	juneexec "june/internal/exec"
	"june/internal/repo"

	"github.com/google/uuid"
)

func TestWorkerSpawnCapturesPromptAndLogs(t *testing.T) {
	// 1) Set up temp DB, create agent row, store prompt message
	db := openTestDB(t)
	ctx := testCtx()

	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
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
		Project:      ctx.Project,
		Branch:       ctx.Branch,
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
	chunks := make(chan juneexec.TranscriptChunk, 3)
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: "worker output line 1\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stderr", Data: "worker stderr\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: "worker output line 2\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan juneexec.TranscriptChunk, func() error, error) {
			return 9999, chunks, func() error { return nil }, nil
		},
	}

	// Run the worker spawn
	err := runWorkerSpawn(db, runner, "test-worker")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// 3) Assert logs contain prompt (in) + output (out)
	entries, err := repo.ListLogs(db, ctx.Project, ctx.Branch, "test-worker", "")
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
	updatedAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "test-worker")
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
	ctx := testCtx()

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
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
		Project:      ctx.Project,
		Branch:       ctx.Branch,
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
	chunks := make(chan juneexec.TranscriptChunk, 5)
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"thread.started","thread_id":"thread_xyz789"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan juneexec.TranscriptChunk, func() error, error) {
			return 8888, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn for Codex agent
	err := runWorkerSpawn(db, runner, "test-codex-worker")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify thread_id was captured and stored as session_id
	updatedAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "test-codex-worker")
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
	ctx := testCtx()

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
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
		Project:      ctx.Project,
		Branch:       ctx.Branch,
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
	chunks := make(chan juneexec.TranscriptChunk, 3)
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan juneexec.TranscriptChunk, func() error, error) {
			return 7777, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn - should succeed despite missing thread_id
	err := runWorkerSpawn(db, runner, "test-codex-worker-no-thread")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify agent completed successfully (but session_id should still be placeholder)
	updatedAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "test-codex-worker-no-thread")
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

func TestWorkerCodexSpawnLogsItemStarted(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		Name:      "test-codex-worker",
		Type:      "codex",
		Task:      "test codex task",
		Status:    "busy",
		SessionID: sql.NullString{String: placeholderID, Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Store prompt message
	promptMsg := repo.Message{
		ID:           uuid.New().String(),
		Project:      ctx.Project,
		Branch:       ctx.Branch,
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

	// Mock runner that simulates Codex JSON output with item.started events
	chunks := make(chan juneexec.TranscriptChunk, 5)
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"thread.started","thread_id":"thread_xyz789"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.started","item":{"type":"command_execution","command":"echo hello","text":""}}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.completed","item":{"type":"command_execution","command":"echo hello","aggregated_output":"hello","exit_code":0}}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.started","item":{"type":"output","text":"Working on task..."}}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.completed","item":{"type":"output","text":"Task complete"}}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan juneexec.TranscriptChunk, func() error, error) {
			return 8888, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn for Codex agent
	err := runWorkerSpawn(db, runner, "test-codex-worker")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify item.started events were logged
	entries, err := repo.ListLogs(db, ctx.Project, ctx.Branch, "test-codex-worker", "")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}

	var startedCount int
	var foundCommandStarted, foundTextStarted bool
	for _, entry := range entries {
		if entry.EventType == "item.started" {
			startedCount++
			// Verify command was used as content fallback when text is empty
			if entry.Command.Valid && entry.Command.String == "echo hello" {
				foundCommandStarted = true
				if !entry.Content.Valid || entry.Content.String != "echo hello" {
					t.Errorf("expected content to be 'echo hello' (fallback from command), got %q", entry.Content.String)
				}
			}
			// Verify text is used when available
			if entry.Content.Valid && entry.Content.String == "Working on task..." {
				foundTextStarted = true
			}
		}
	}

	if startedCount != 2 {
		t.Fatalf("expected 2 item.started log entries, got %d", startedCount)
	}
	if !foundCommandStarted {
		t.Fatal("expected to find item.started event with command 'echo hello'")
	}
	if !foundTextStarted {
		t.Fatal("expected to find item.started event with text 'Working on task...'")
	}
}

func TestWorkerCodexSpawnLogsTurnEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()

	// Create Codex agent with placeholder session_id
	placeholderID := uuid.New().String()
	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		Name:      "test-codex-worker-turn",
		Type:      "codex",
		Task:      "test codex turn events",
		Status:    "busy",
		SessionID: sql.NullString{String: placeholderID, Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Store prompt message
	promptMsg := repo.Message{
		ID:           uuid.New().String(),
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "orchestrator",
		ToAgent:      sql.NullString{String: "test-codex-worker-turn", Valid: true},
		Type:         "prompt",
		Content:      "Test codex turn prompt",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, promptMsg); err != nil {
		t.Fatalf("create prompt message: %v", err)
	}

	// Mock runner that simulates Codex JSON output with turn events
	chunks := make(chan juneexec.TranscriptChunk, 5)
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"thread.started","thread_id":"thread_turn_worker"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"turn.started"}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.started","item":{"type":"output","text":"Processing..."}}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"item.completed","item":{"type":"output","text":"Done processing"}}` + "\n"}
	chunks <- juneexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"turn.completed"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan juneexec.TranscriptChunk, func() error, error) {
			return 8888, chunks, func() error { return nil }, nil
		},
	}

	// Run worker spawn for Codex agent
	err := runWorkerSpawn(db, runner, "test-codex-worker-turn")
	if err != nil {
		t.Fatalf("runWorkerSpawn failed: %v", err)
	}

	// Verify turn events were logged
	entries, err := repo.ListLogs(db, ctx.Project, ctx.Branch, "test-codex-worker-turn", "")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}

	var turnStartedCount, turnCompletedCount int
	for _, entry := range entries {
		if entry.EventType == "turn.started" {
			turnStartedCount++
			if entry.AgentType != "codex" {
				t.Errorf("expected agent_type 'codex', got %q", entry.AgentType)
			}
		}
		if entry.EventType == "turn.completed" {
			turnCompletedCount++
			if entry.AgentType != "codex" {
				t.Errorf("expected agent_type 'codex', got %q", entry.AgentType)
			}
		}
	}

	if turnStartedCount != 1 {
		t.Fatalf("expected 1 turn.started log entry, got %d", turnStartedCount)
	}
	if turnCompletedCount != 1 {
		t.Fatalf("expected 1 turn.completed log entry, got %d", turnCompletedCount)
	}
}

package commands

import (
	"encoding/json"
	"testing"

	"june/internal/repo"
	"june/internal/scope"

	"github.com/google/uuid"
)

// wakeupTracker is a test implementation of wakeupSender
type wakeupTracker struct {
	wakeups map[string]string // agent -> context
}

func newWakeupTracker() *wakeupTracker {
	return &wakeupTracker{
		wakeups: make(map[string]string),
	}
}

func (w *wakeupTracker) SendTo(agent, context string) error {
	w.wakeups[agent] = context
	return nil
}

func (w *wakeupTracker) Woke(agent string) bool {
	_, ok := w.wakeups[agent]
	return ok
}

func (w *wakeupTracker) GetContext(agent string) string {
	return w.wakeups[agent]
}

func TestWakeupOnMention(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := scope.Context{Project: "app", Branch: "main"}

	// Create a message with a mention
	mentions := []string{"app:main:impl-1"}
	mentionsJSON, _ := json.Marshal(mentions)

	msg := repo.Message{
		ID:           uuid.New().String(),
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "orchestrator",
		Type:         "say",
		Content:      "@impl-1 status?",
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	// Process wakeups
	w := newWakeupTracker()
	err := processWakeups(db, ctx, w)
	if err != nil {
		t.Fatalf("process wakeups: %v", err)
	}

	// Verify wakeup was triggered
	if !w.Woke("app:main:impl-1") {
		t.Fatalf("expected wakeup for impl-1")
	}
}

func TestWakeupOnMultipleMentions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := scope.Context{Project: "app", Branch: "main"}

	// Create a message with multiple mentions
	mentions := []string{"app:main:impl-1", "app:main:impl-2"}
	mentionsJSON, _ := json.Marshal(mentions)

	msg := repo.Message{
		ID:           uuid.New().String(),
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "orchestrator",
		Type:         "say",
		Content:      "@impl-1 @impl-2 collaborate on this",
		MentionsJSON: string(mentionsJSON),
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	// Process wakeups
	w := newWakeupTracker()
	err := processWakeups(db, ctx, w)
	if err != nil {
		t.Fatalf("process wakeups: %v", err)
	}

	// Verify both wakeups were triggered
	if !w.Woke("app:main:impl-1") {
		t.Fatalf("expected wakeup for impl-1")
	}
	if !w.Woke("app:main:impl-2") {
		t.Fatalf("expected wakeup for impl-2")
	}
}

func TestWakeupNoMentions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := scope.Context{Project: "app", Branch: "main"}

	// Create a message with no mentions
	msg := repo.Message{
		ID:           uuid.New().String(),
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "orchestrator",
		Type:         "say",
		Content:      "just a regular message",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	// Process wakeups
	w := newWakeupTracker()
	err := processWakeups(db, ctx, w)
	if err != nil {
		t.Fatalf("process wakeups: %v", err)
	}

	// Verify no wakeups were triggered
	if len(w.wakeups) != 0 {
		t.Fatalf("expected no wakeups, got %d", len(w.wakeups))
	}
}

func TestParseMentionsFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected []string
	}{
		{
			name:     "empty array",
			json:     "[]",
			expected: []string{},
		},
		{
			name:     "single mention",
			json:     `["app:main:impl-1"]`,
			expected: []string{"app:main:impl-1"},
		},
		{
			name:     "multiple mentions",
			json:     `["app:main:impl-1","app:main:impl-2"]`,
			expected: []string{"app:main:impl-1", "app:main:impl-2"},
		},
		{
			name:     "invalid json",
			json:     "not-json",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMentionsFromJSON(tt.json)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d mentions, got %d", len(tt.expected), len(result))
			}
			for i, mention := range tt.expected {
				if result[i] != mention {
					t.Fatalf("expected mention[%d]=%s, got %s", i, mention, result[i])
				}
			}
		})
	}
}

func TestBuildContextBundle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := scope.Context{Project: "app", Branch: "main"}

	// Create some messages
	msg1 := repo.Message{
		ID:           "msg1",
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "orchestrator",
		Type:         "say",
		Content:      "first message",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	msg2 := repo.Message{
		ID:           "msg2",
		Project:      ctx.Project,
		Branch:       ctx.Branch,
		FromAgent:    "impl-1",
		Type:         "say",
		Content:      "second message",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}

	if err := repo.CreateMessage(db, msg1); err != nil {
		t.Fatalf("create msg1: %v", err)
	}
	if err := repo.CreateMessage(db, msg2); err != nil {
		t.Fatalf("create msg2: %v", err)
	}

	// Get all messages
	msgs, err := repo.ListMessages(db, repo.MessageFilter{
		Project: ctx.Project,
		Branch:  ctx.Branch,
	})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}

	// Build context bundle
	contextText, err := buildContextBundle(db, ctx, msgs)
	if err != nil {
		t.Fatalf("build context: %v", err)
	}

	// Verify context contains message content
	if contextText == "" {
		t.Fatalf("expected non-empty context")
	}
	// Context should contain the message content
	if len(contextText) < 10 {
		t.Fatalf("context seems too short: %s", contextText)
	}
}

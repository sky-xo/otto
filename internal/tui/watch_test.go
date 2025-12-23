package tui

import (
	"database/sql"
	"testing"

	"otto/internal/repo"

	_ "modernc.org/sqlite"
)

func TestNewModel(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model
	m := NewModel(db)

	// Verify initial state
	if m.db == nil {
		t.Error("expected db to be set")
	}
	if len(m.messages) != 0 {
		t.Error("expected empty messages list")
	}
	if len(m.agents) != 0 {
		t.Error("expected empty agents list")
	}
	if m.activeChannelID != mainChannelID {
		t.Errorf("expected activeChannelID to be %q", mainChannelID)
	}
	if len(m.transcripts) != 0 {
		t.Error("expected empty transcripts map")
	}
}

func TestModelView(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model with size
	m := NewModel(db)
	m.width = 80
	m.height = 24

	// Should render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestChannelOrdering(t *testing.T) {
	agents := []repo.Agent{
		{ID: "agent-3", Status: "failed"},
		{ID: "agent-2", Status: "blocked"},
		{ID: "agent-1", Status: "busy"},
		{ID: "agent-4", Status: "complete"},
	}

	ordered := sortAgentsByStatus(agents)
	if len(ordered) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(ordered))
	}

	expected := []string{"agent-1", "agent-2", "agent-4", "agent-3"}
	for i, id := range expected {
		if ordered[i].ID != id {
			t.Fatalf("expected %q at index %d, got %q", id, i, ordered[i].ID)
		}
	}
}

func TestChannelsIncludeMainFirst(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{ID: "agent-2", Status: "complete"},
		{ID: "agent-1", Status: "busy"},
	}

	channels := m.channels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].ID != mainChannelID {
		t.Fatalf("expected main channel first, got %q", channels[0].ID)
	}
	if channels[1].ID != "agent-1" || channels[2].ID != "agent-2" {
		t.Fatalf("unexpected channel order: %q, %q", channels[1].ID, channels[2].ID)
	}
}

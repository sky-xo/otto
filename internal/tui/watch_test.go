package tui

import (
	"database/sql"
	"testing"
	"time"

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
		{Name: "agent-3", Status: "failed"},
		{Name: "agent-2", Status: "blocked"},
		{Name: "agent-1", Status: "busy"},
		{Name: "agent-4", Status: "complete"},
	}

	ordered := sortAgentsByStatus(agents)
	if len(ordered) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(ordered))
	}

	expected := []string{"agent-1", "agent-2", "agent-4", "agent-3"}
	for i, id := range expected {
		if ordered[i].Name != id {
			t.Fatalf("expected %q at index %d, got %q", id, i, ordered[i].Name)
		}
	}
}

func TestChannelsIncludeMainFirst(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Name: "agent-2", Status: "complete"},
		{Name: "agent-1", Status: "busy"},
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

func TestArchivedAgentsHiddenByDefault(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].ID != mainChannelID {
		t.Fatalf("expected main channel first, got %q", channels[0].ID)
	}
	if channels[1].ID != "agent-1" {
		t.Fatalf("expected active agent second, got %q", channels[1].ID)
	}
	if channels[2].ID != archivedChannelID {
		t.Fatalf("expected archived header last, got %q", channels[2].ID)
	}
}

func TestArchivedAgentsAppearWhenExpanded(t *testing.T) {
	m := NewModel(nil)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
		{
			Name:       "agent-3",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
	}
	m.archivedExpanded = true

	channels := m.channels()
	if len(channels) != 5 {
		t.Fatalf("expected 5 channels, got %d", len(channels))
	}
	if channels[2].ID != archivedChannelID {
		t.Fatalf("expected archived header at index 2, got %q", channels[2].ID)
	}
	if channels[2].Name != "Archived (2)" {
		t.Fatalf("expected archived header label, got %q", channels[2].Name)
	}
	if channels[3].ID != "agent-3" || channels[4].ID != "agent-2" {
		t.Fatalf("unexpected archived order: %q, %q", channels[3].ID, channels[4].ID)
	}
}

func TestArchivedEnterTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == archivedChannelID {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected archived header to exist")
	}

	m.cursorIndex = headerIndex
	_ = m.activateSelection()
	if !m.archivedExpanded {
		t.Fatal("expected archived section to expand on enter")
	}
	if m.activeChannelID != mainChannelID {
		t.Fatalf("expected active channel to remain main, got %q", m.activeChannelID)
	}

	_ = m.activateSelection()
	if m.archivedExpanded {
		t.Fatal("expected archived section to collapse on enter")
	}
}

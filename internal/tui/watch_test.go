package tui

import (
	"database/sql"
	"testing"
	"time"

	"june/internal/repo"

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
	// Default activeChannelID is still mainChannelID even though Main channel no longer exists
	// This is handled by ensureSelection() when agents are loaded
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

func TestSortArchivedAgents(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	t.Run("sorts by activity map (most recent first)", func(t *testing.T) {
		agents := []repo.Agent{
			{Name: "agent-old"},
			{Name: "agent-new"},
			{Name: "agent-mid"},
		}
		lastActivity := map[string]time.Time{
			"agent-old": baseTime.Add(-2 * time.Hour),
			"agent-new": baseTime,
			"agent-mid": baseTime.Add(-1 * time.Hour),
		}

		sorted := sortArchivedAgents(agents, lastActivity)

		expected := []string{"agent-new", "agent-mid", "agent-old"}
		for i, name := range expected {
			if sorted[i].Name != name {
				t.Errorf("position %d: expected %q, got %q", i, name, sorted[i].Name)
			}
		}
	})

	t.Run("falls back to ArchivedAt when no activity in map", func(t *testing.T) {
		agents := []repo.Agent{
			{Name: "agent-a", ArchivedAt: sql.NullTime{Time: baseTime.Add(-2 * time.Hour), Valid: true}},
			{Name: "agent-b", ArchivedAt: sql.NullTime{Time: baseTime, Valid: true}},
			{Name: "agent-c", ArchivedAt: sql.NullTime{Time: baseTime.Add(-1 * time.Hour), Valid: true}},
		}
		lastActivity := map[string]time.Time{} // empty map

		sorted := sortArchivedAgents(agents, lastActivity)

		expected := []string{"agent-b", "agent-c", "agent-a"}
		for i, name := range expected {
			if sorted[i].Name != name {
				t.Errorf("position %d: expected %q, got %q", i, name, sorted[i].Name)
			}
		}
	})

	t.Run("alphabetical tiebreaker when times are equal", func(t *testing.T) {
		agents := []repo.Agent{
			{Name: "charlie"},
			{Name: "alice"},
			{Name: "bob"},
		}
		lastActivity := map[string]time.Time{
			"charlie": baseTime,
			"alice":   baseTime,
			"bob":     baseTime,
		}

		sorted := sortArchivedAgents(agents, lastActivity)

		expected := []string{"alice", "bob", "charlie"}
		for i, name := range expected {
			if sorted[i].Name != name {
				t.Errorf("position %d: expected %q, got %q", i, name, sorted[i].Name)
			}
		}
	})

	t.Run("mixed activity map and ArchivedAt fallback", func(t *testing.T) {
		agents := []repo.Agent{
			{Name: "agent-with-activity"},
			{Name: "agent-archived-only", ArchivedAt: sql.NullTime{Time: baseTime.Add(-30 * time.Minute), Valid: true}},
			{Name: "agent-no-time", ArchivedAt: sql.NullTime{Valid: false}}, // invalid ArchivedAt
		}
		lastActivity := map[string]time.Time{
			"agent-with-activity": baseTime,
		}

		sorted := sortArchivedAgents(agents, lastActivity)

		// agent-with-activity (most recent), agent-archived-only (30min ago), agent-no-time (zero time)
		expected := []string{"agent-with-activity", "agent-archived-only", "agent-no-time"}
		for i, name := range expected {
			if sorted[i].Name != name {
				t.Errorf("position %d: expected %q, got %q", i, name, sorted[i].Name)
			}
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		agents := []repo.Agent{
			{Name: "agent-b"},
			{Name: "agent-a"},
		}
		lastActivity := map[string]time.Time{
			"agent-b": baseTime.Add(-1 * time.Hour),
			"agent-a": baseTime,
		}

		_ = sortArchivedAgents(agents, lastActivity)

		// Original should be unchanged
		if agents[0].Name != "agent-b" || agents[1].Name != "agent-a" {
			t.Error("original slice was modified")
		}
	})
}


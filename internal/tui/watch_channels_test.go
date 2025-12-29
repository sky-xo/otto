package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"otto/internal/repo"
)

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

func TestChannelsIncludeProjectHeaderFirst(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-2", Status: "complete"},
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
	}

	channels := m.sidebarItems()
	// Expected: test/main header, agent-1 (busy first), agent-2
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[0].ID != "test/main" {
		t.Fatalf("expected test/main header first, got %q", channels[0].ID)
	}
	// Agents should be sorted by status: busy before complete
	if channels[1].ID != "agent-1" || channels[2].ID != "agent-2" {
		t.Fatalf("unexpected agent order: %q, %q", channels[1].ID, channels[2].ID)
	}
}

func TestArchivedAgentsHiddenByDefault(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.sidebarItems()
	// Expected: test/main header, agent-1, archived_count indicator (collapsed)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[1].ID != "agent-1" {
		t.Fatalf("expected active agent at index 1, got %q", channels[1].ID)
	}
	if channels[2].Kind != SidebarArchivedSection {
		t.Fatalf("expected archived_count at index 2, got %q", channels[2].Kind)
	}
	if channels[2].Name != "1 archived" {
		t.Fatalf("expected '1 archived' label, got %q", channels[2].Name)
	}
}

func TestArchivedAgentsAppearWhenExpanded(t *testing.T) {
	m := NewModel(nil)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-3",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
	}
	m.archivedExpanded["test/main"] = true

	channels := m.sidebarItems()
	// Expected: test/main header, agent-1, archived_count indicator, agent-3, agent-2
	if len(channels) != 5 {
		t.Fatalf("expected 5 channels, got %d", len(channels))
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[1].ID != "agent-1" {
		t.Fatalf("expected active agent at index 1, got %q", channels[1].ID)
	}
	if channels[2].Kind != SidebarArchivedSection {
		t.Fatalf("expected archived_count at index 2, got %q", channels[2].Kind)
	}
	if channels[2].Name != "2 archived" {
		t.Fatalf("expected '2 archived' label, got %q", channels[2].Name)
	}
	if channels[3].ID != "agent-3" || channels[4].ID != "agent-2" {
		t.Fatalf("unexpected archived order: %q, %q", channels[3].ID, channels[4].ID)
	}
}

func TestArchivedEnterTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.sidebarItems()
	archivedCountIndex := -1
	for i, ch := range channels {
		if ch.Kind == SidebarArchivedSection {
			archivedCountIndex = i
			break
		}
	}
	if archivedCountIndex == -1 {
		t.Fatal("expected archived_count indicator to exist")
	}

	m.cursorIndex = archivedCountIndex
	_ = m.toggleSelection()
	if !m.archivedExpanded["test/main"] {
		t.Fatal("expected archived section to expand on enter")
	}
	if m.activeChannelID != mainChannelID {
		t.Fatalf("expected active channel to remain main, got %q", m.activeChannelID)
	}

	_ = m.toggleSelection()
	if m.archivedExpanded["test/main"] {
		t.Fatal("expected archived section to collapse on enter")
	}
}

func TestChannelsGroupByProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
		{Project: "app", Branch: "dev", Name: "tester", Status: "busy"},
	}

	channels := m.sidebarItems()

	// Expected structure:
	// 0: app/dev header
	// 1:   tester (indented)
	// 2: separator
	// 3: other/feature header
	// 4:   worker (indented)
	// 5: separator
	// 6: otto/main header
	// 7:   impl-1 (indented)
	// 8:   reviewer (indented)

	expectedCount := 9
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Test headers are present with correct IDs and kinds
	// app/dev header
	if channels[0].ID != "app/dev" {
		t.Fatalf("expected 'app/dev' header at index 0, got %q", channels[0].ID)
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected 'project_header' kind at index 0, got %q", channels[0].Kind)
	}
	if channels[0].Name != "app/dev" {
		t.Fatalf("expected 'app/dev' name at index 0, got %q", channels[0].Name)
	}
	if channels[0].Level != 0 {
		t.Fatalf("expected Level 0 for header at index 0, got %d", channels[0].Level)
	}

	// separator at index 2
	if channels[2].Kind != SidebarDivider {
		t.Fatalf("expected 'separator' kind at index 2, got %q", channels[2].Kind)
	}

	// other/feature header
	if channels[3].ID != "other/feature" {
		t.Fatalf("expected 'other/feature' header at index 3, got %q", channels[3].ID)
	}
	if channels[3].Kind != SidebarChannelHeader {
		t.Fatalf("expected 'project_header' kind at index 3, got %q", channels[3].Kind)
	}

	// separator at index 5
	if channels[5].Kind != SidebarDivider {
		t.Fatalf("expected 'separator' kind at index 5, got %q", channels[5].Kind)
	}

	// otto/main header
	if channels[6].ID != "otto/main" {
		t.Fatalf("expected 'otto/main' header at index 6, got %q", channels[6].ID)
	}
	if channels[6].Kind != SidebarChannelHeader {
		t.Fatalf("expected 'project_header' kind at index 6, got %q", channels[6].Kind)
	}

	// Test agents under headers have correct Level and properties
	// tester under app/dev
	if channels[1].ID != "tester" {
		t.Fatalf("expected 'tester' at index 1, got %q", channels[1].ID)
	}
	if channels[1].Kind != SidebarAgentRow {
		t.Fatalf("expected 'agent' kind at index 1, got %q", channels[1].Kind)
	}
	if channels[1].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 1, got %d", channels[1].Level)
	}
	if channels[1].Status != "busy" {
		t.Fatalf("expected 'busy' status at index 1, got %q", channels[1].Status)
	}

	// worker under other/feature
	if channels[4].ID != "worker" {
		t.Fatalf("expected 'worker' at index 4, got %q", channels[4].ID)
	}
	if channels[4].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 4, got %d", channels[4].Level)
	}

	// impl-1 under otto/main (sorted by status: busy before blocked)
	if channels[7].ID != "impl-1" {
		t.Fatalf("expected 'impl-1' at index 7, got %q", channels[7].ID)
	}
	if channels[7].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 7, got %d", channels[7].Level)
	}

	// reviewer under otto/main
	if channels[8].ID != "reviewer" {
		t.Fatalf("expected 'reviewer' at index 8, got %q", channels[8].ID)
	}
	if channels[8].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 8, got %d", channels[8].Level)
	}
}

func TestChannelsGroupingWithArchived(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "old-agent",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.sidebarItems()

	// Expected structure (with per-project archived sections):
	// 0: other/feature header
	// 1:   worker
	// 2: separator
	// 3: otto/main header
	// 4:   impl-1
	// 5:   1 archived (archived_count indicator)

	expectedCount := 6
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Verify active agents are grouped
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project header at index 0, got %q", channels[0].Kind)
	}
	if channels[2].Kind != SidebarDivider {
		t.Fatalf("expected separator at index 2, got %q", channels[2].Kind)
	}
	if channels[3].Kind != SidebarChannelHeader {
		t.Fatalf("expected project header at index 3, got %q", channels[3].Kind)
	}

	// Verify archived count indicator is shown for otto/main
	if channels[5].Kind != SidebarArchivedSection {
		t.Fatalf("expected 'archived_count' kind at index 5, got %q", channels[5].Kind)
	}
	if channels[5].Name != "1 archived" {
		t.Fatalf("expected '1 archived' label, got %q", channels[5].Name)
	}
}

func TestProjectHeaderCollapseHidesAgents(t *testing.T) {
	m := NewModel(nil)
	m.projectExpanded = map[string]bool{"otto/main": false}
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
	}

	channels := m.sidebarItems()

	// Expected structure when collapsed:
	// 0: otto/main header (collapsed)
	// No agents shown under header

	expectedCount := 1
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (header only), got %d", expectedCount, len(channels))
	}

	if channels[0].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 0, got %q", channels[0].ID)
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header kind, got %q", channels[0].Kind)
	}
}

func TestProjectHeaderExpandedShowsAgents(t *testing.T) {
	m := NewModel(nil)
	m.projectExpanded = map[string]bool{"otto/main": true}
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
	}

	channels := m.sidebarItems()

	// Expected structure when expanded:
	// 0: otto/main header (expanded)
	// 1:   impl-1
	// 2:   reviewer

	expectedCount := 3
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	if channels[0].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 0, got %q", channels[0].ID)
	}
	if channels[1].ID != "impl-1" {
		t.Fatalf("expected impl-1 at index 1, got %q", channels[1].ID)
	}
	if channels[2].ID != "reviewer" {
		t.Fatalf("expected reviewer at index 2, got %q", channels[2].ID)
	}
}

func TestProjectHeaderDefaultExpanded(t *testing.T) {
	m := NewModel(nil)
	// No explicit projectExpanded state - should default to expanded
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()

	// Expected structure (default expanded):
	// 0: otto/main header
	// 1:   impl-1

	expectedCount := 2
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (default expanded), got %d", expectedCount, len(channels))
	}

	if channels[1].ID != "impl-1" {
		t.Fatalf("expected impl-1 agent visible by default, got %q", channels[1].ID)
	}
}

func TestArchivedSectionGroupsByProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.archivedExpanded["otto/main"] = true
	m.archivedExpanded["other/feature"] = true
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "active-1", Status: "busy"},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-1",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
		{
			Project:    "other",
			Branch:     "feature",
			Name:       "archived-2",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
	}

	channels := m.sidebarItems()

	// Expected structure (with per-project archived sections):
	// 0: other/feature header
	// 1:   1 archived (archived_count)
	// 2:     archived-2 (expanded)
	// 3: separator
	// 4: otto/main header
	// 5:   active-1
	// 6:   1 archived (archived_count)
	// 7:     archived-1 (expanded)

	expectedCount := 8
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Verify other/feature section
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[1].Kind != SidebarArchivedSection {
		t.Fatalf("expected archived_count at index 1, got %q", channels[1].Kind)
	}
	if channels[2].ID != "archived-2" {
		t.Fatalf("expected archived-2 at index 2, got %q", channels[2].ID)
	}

	// Verify separator
	if channels[3].Kind != SidebarDivider {
		t.Fatalf("expected separator at index 3, got %q", channels[3].Kind)
	}

	// Verify otto/main section
	if channels[4].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header at index 4, got %q", channels[4].Kind)
	}
	if channels[5].ID != "active-1" {
		t.Fatalf("expected active-1 at index 5, got %q", channels[5].ID)
	}
	if channels[6].Kind != SidebarArchivedSection {
		t.Fatalf("expected archived_count at index 6, got %q", channels[6].Kind)
	}
	if channels[7].ID != "archived-1" {
		t.Fatalf("expected archived-1 at index 7, got %q", channels[7].ID)
	}
}

func TestArchivedSectionRespectsProjectCollapse(t *testing.T) {
	m := NewModel(nil)
	m.archivedExpanded["otto/main"] = true
	m.projectExpanded = map[string]bool{"otto/main": false}
	archivedAt := time.Now().Add(-time.Hour)

	m.agents = []repo.Agent{
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-1",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-2",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.sidebarItems()

	// Expected structure (project collapsed, so no archived section shows):
	// 0: otto/main header (collapsed)
	// When project is collapsed, nothing under it shows (including archived count)

	expectedCount := 1
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (project collapsed), got %d", expectedCount, len(channels))
	}

	if channels[0].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 0, got %q", channels[0].ID)
	}
	if channels[0].Kind != SidebarChannelHeader {
		t.Fatalf("expected project_header kind at index 0, got %q", channels[0].Kind)
	}
}

func TestPerProjectArchivedIndicator(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)

	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "active-1", Status: "busy"},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-1",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-2",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.sidebarItems()

	// Find the archived_count indicator
	archivedCountIndex := -1
	for i, ch := range channels {
		if ch.Kind == SidebarArchivedSection {
			archivedCountIndex = i
			break
		}
	}
	if archivedCountIndex == -1 {
		t.Fatal("expected archived_count indicator to exist")
	}

	ch := channels[archivedCountIndex]
	if ch.Name != "2 archived" {
		t.Fatalf("expected '2 archived' label, got %q", ch.Name)
	}
	if ch.Level != 1 {
		t.Fatalf("expected Level 1 for archived_count, got %d", ch.Level)
	}
	if !strings.HasPrefix(ch.ID, "archived:") {
		t.Fatalf("expected ID to start with 'archived:', got %q", ch.ID)
	}

	// Verify archived agents are NOT shown by default (collapsed)
	archivedAgentCount := 0
	for _, ch := range channels {
		if ch.ID == "archived-1" || ch.ID == "archived-2" {
			archivedAgentCount++
		}
	}
	if archivedAgentCount != 0 {
		t.Fatalf("expected archived agents to be hidden by default, found %d", archivedAgentCount)
	}

	// Expand the archived section
	m.cursorIndex = archivedCountIndex
	_ = m.toggleSelection()

	// Verify archived section is now expanded
	if !m.archivedExpanded["otto/main"] {
		t.Fatal("expected archived section to be expanded after toggle")
	}

	// Re-fetch channels and verify archived agents are shown
	channels = m.sidebarItems()
	archivedAgentCount = 0
	for _, ch := range channels {
		if ch.ID == "archived-1" || ch.ID == "archived-2" {
			archivedAgentCount++
			if ch.Level != 2 {
				t.Fatalf("expected Level 2 for archived agent %q, got %d", ch.ID, ch.Level)
			}
		}
	}
	if archivedAgentCount != 2 {
		t.Fatalf("expected 2 archived agents to be shown after expand, found %d", archivedAgentCount)
	}
}


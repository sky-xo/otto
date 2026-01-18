package cli

import (
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/sky-xo/june/internal/db"
)

func TestGenerateTaskID(t *testing.T) {
	id, err := generateTaskID()
	if err != nil {
		t.Fatalf("generateTaskID failed: %v", err)
	}

	// Must match pattern: t-[5 hex chars]
	pattern := regexp.MustCompile(`^t-[a-f0-9]{5}$`)
	if !pattern.MatchString(id) {
		t.Errorf("ID %q doesn't match pattern t-[a-f0-9]{5}", id)
	}
}

func TestGenerateTaskIDUniqueness(t *testing.T) {
	// Generate multiple IDs - should all be unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateTaskID()
		if err != nil {
			t.Fatalf("generateTaskID failed: %v", err)
		}
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateUniqueTaskID(t *testing.T) {
	// Setup temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Generate multiple IDs - should all be unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateUniqueTaskID(database)
		if err != nil {
			t.Fatalf("generateUniqueTaskID failed: %v", err)
		}
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateUniqueTaskIDRetriesOnCollision(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Create a task with a known ID
	now := time.Now()
	if err := database.CreateTask(db.Task{
		ID: "t-12345", Title: "Existing", Status: "open",
		RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Generate new ID - should not collide with existing
	id, err := generateUniqueTaskID(database)
	if err != nil {
		t.Fatalf("generateUniqueTaskID failed: %v", err)
	}
	if id == "t-12345" {
		t.Error("Generated ID should not match existing task")
	}
}

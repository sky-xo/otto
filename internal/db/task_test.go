package db

import (
	"testing"
	"time"
)

func TestCreateTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	task := Task{
		ID:        "t-a3f8b",
		Title:     "Implement auth feature",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Verify by reading back
	got, err := db.GetTask("t-a3f8b")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if got.Title != task.Title {
		t.Errorf("Title = %q, want %q", got.Title, task.Title)
	}
	if got.Status != task.Status {
		t.Errorf("Status = %q, want %q", got.Status, task.Status)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.GetTask("t-nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

func TestCreateTaskWithParent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()

	// Create parent task
	parent := Task{
		ID:        "t-parent",
		Title:     "Parent task",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateTask(parent); err != nil {
		t.Fatalf("CreateTask parent failed: %v", err)
	}

	// Create child task
	parentID := "t-parent"
	child := Task{
		ID:        "t-child",
		ParentID:  &parentID,
		Title:     "Child task",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateTask(child); err != nil {
		t.Fatalf("CreateTask child failed: %v", err)
	}

	// Verify child has parent
	got, err := db.GetTask("t-child")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != "t-parent" {
		t.Errorf("ParentID = %v, want %q", got.ParentID, "t-parent")
	}
}

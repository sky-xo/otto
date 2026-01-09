package db

import (
	"errors"
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
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

func TestCreateTaskEmptyTitle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Test empty title
	task := Task{
		ID:        "t-empty1",
		Title:     "",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.CreateTask(task)
	if err == nil {
		t.Error("Expected error for empty title, got nil")
	}

	// Test whitespace-only title
	task.ID = "t-empty2"
	task.Title = "   "
	err = db.CreateTask(task)
	if err == nil {
		t.Error("Expected error for whitespace-only title, got nil")
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

func TestUpdateTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create initial task
	task := Task{
		ID:        "t-b2c3d",
		Title:     "Original title",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Update status and notes
	newStatus := "in_progress"
	newNotes := "Started working"
	err := db.UpdateTask("t-b2c3d", TaskUpdate{
		Status: &newStatus,
		Notes:  &newNotes,
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	// Verify
	got, err := db.GetTask("t-b2c3d")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", got.Status, "in_progress")
	}
	if got.Notes == nil || *got.Notes != "Started working" {
		t.Errorf("Notes = %v, want %q", got.Notes, "Started working")
	}
}

func TestUpdateTaskTitle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	task := Task{
		ID:        "t-c3d4e",
		Title:     "Original",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	newTitle := "Updated title"
	err := db.UpdateTask("t-c3d4e", TaskUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	got, err := db.GetTask("t-c3d4e")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Title != "Updated title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated title")
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	newStatus := "closed"
	err := db.UpdateTask("t-nonexistent", TaskUpdate{Status: &newStatus})
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

func TestUpdateTaskEmptyTitle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create a task first
	now := time.Now()
	task := Task{
		ID:        "t-update-empty",
		Title:     "Original title",
		Status:    "open",
		RepoPath:  "/app",
		Branch:    "main",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Try to update with empty title
	emptyTitle := ""
	err := db.UpdateTask("t-update-empty", TaskUpdate{Title: &emptyTitle})
	if err == nil {
		t.Error("Expected error for empty title, got nil")
	}

	// Try to update with whitespace-only title
	whitespaceTitle := "   "
	err = db.UpdateTask("t-update-empty", TaskUpdate{Title: &whitespaceTitle})
	if err == nil {
		t.Error("Expected error for whitespace-only title, got nil")
	}

	// Verify original title is preserved
	got, err := db.GetTask("t-update-empty")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Title != "Original title" {
		t.Errorf("Title changed unexpectedly: got %q, want %q", got.Title, "Original title")
	}
}

func TestListRootTasks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()

	// Create root tasks
	if err := db.CreateTask(Task{ID: "t-root1", Title: "Root 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-root2", Title: "Root 2", Status: "in_progress", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Create child task (should not appear in root list)
	parent := "t-root1"
	if err := db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Create task in different scope (should not appear)
	if err := db.CreateTask(Task{ID: "t-other", Title: "Other", Status: "open", RepoPath: "/other", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	tasks, err := db.ListRootTasks("/app", "main")
	if err != nil {
		t.Fatalf("ListRootTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Got %d tasks, want 2", len(tasks))
	}
}

func TestListChildTasks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	parent := "t-parent"

	if err := db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "closed", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	tasks, err := db.ListChildTasks("t-parent")
	if err != nil {
		t.Fatalf("ListChildTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Got %d children, want 2", len(tasks))
	}
}

func TestListTasksExcludesDeleted(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	if err := db.CreateTask(Task{ID: "t-active", Title: "Active", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Manually soft-delete a task
	if err := db.CreateTask(Task{ID: "t-deleted", Title: "Deleted", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if _, err := db.Exec("UPDATE tasks SET deleted_at = ? WHERE id = ?", now.Format(time.RFC3339), "t-deleted"); err != nil {
		t.Fatalf("Exec soft-delete failed: %v", err)
	}

	tasks, err := db.ListRootTasks("/app", "main")
	if err != nil {
		t.Fatalf("ListRootTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Got %d tasks, want 1 (deleted should be excluded)", len(tasks))
	}
}

func TestCountChildren(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	parent := "t-parent"

	if err := db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	count, err := db.CountChildren("t-parent")
	if err != nil {
		t.Fatalf("CountChildren failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}
}

func TestDeleteTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	if err := db.CreateTask(Task{ID: "t-todel", Title: "To Delete", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	err := db.DeleteTask("t-todel")
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Should not appear in list
	tasks, err := db.ListRootTasks("/app", "main")
	if err != nil {
		t.Fatalf("ListRootTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Deleted task still appears in list")
	}

	// GetTask should still work (returns deleted task)
	task, err := db.GetTask("t-todel")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.DeletedAt == nil {
		t.Error("DeletedAt should be set")
	}
}

func TestDeleteTaskCascadesToChildren(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	parent := "t-parent"

	if err := db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if err := db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Delete parent
	err := db.DeleteTask("t-parent")
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Children should also be deleted (not in list)
	children, err := db.ListChildTasks("t-parent")
	if err != nil {
		t.Fatalf("ListChildTasks failed: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("Children still exist after parent delete: %d", len(children))
	}

	// Verify children have DeletedAt set
	child1, err := db.GetTask("t-child1")
	if err != nil {
		t.Fatalf("GetTask child1 failed: %v", err)
	}
	if child1.DeletedAt == nil {
		t.Error("child1 DeletedAt should be set")
	}

	child2, err := db.GetTask("t-child2")
	if err != nil {
		t.Fatalf("GetTask child2 failed: %v", err)
	}
	if child2.DeletedAt == nil {
		t.Error("child2 DeletedAt should be set")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := db.DeleteTask("t-nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

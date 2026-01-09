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
	got, _ := db.GetTask("t-b2c3d")
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
	db.CreateTask(task)

	newTitle := "Updated title"
	err := db.UpdateTask("t-c3d4e", TaskUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	got, _ := db.GetTask("t-c3d4e")
	if got.Title != "Updated title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated title")
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	newStatus := "closed"
	err := db.UpdateTask("t-nonexistent", TaskUpdate{Status: &newStatus})
	if err != ErrTaskNotFound {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

func TestListRootTasks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()

	// Create root tasks
	db.CreateTask(Task{ID: "t-root1", Title: "Root 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-root2", Title: "Root 2", Status: "in_progress", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

	// Create child task (should not appear in root list)
	parent := "t-root1"
	db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

	// Create task in different scope (should not appear)
	db.CreateTask(Task{ID: "t-other", Title: "Other", Status: "open", RepoPath: "/other", Branch: "main", CreatedAt: now, UpdatedAt: now})

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

	db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "closed", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

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
	db.CreateTask(Task{ID: "t-active", Title: "Active", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

	// Manually soft-delete a task
	db.CreateTask(Task{ID: "t-deleted", Title: "Deleted", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.Exec("UPDATE tasks SET deleted_at = ? WHERE id = ?", now.Format(time.RFC3339), "t-deleted")

	tasks, _ := db.ListRootTasks("/app", "main")
	if len(tasks) != 1 {
		t.Errorf("Got %d tasks, want 1 (deleted should be excluded)", len(tasks))
	}
}

func TestCountChildren(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	now := time.Now()
	parent := "t-parent"

	db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

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
	db.CreateTask(Task{ID: "t-todel", Title: "To Delete", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

	err := db.DeleteTask("t-todel")
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Should not appear in list
	tasks, _ := db.ListRootTasks("/app", "main")
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

	db.CreateTask(Task{ID: "t-parent", Title: "Parent", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child1", ParentID: &parent, Title: "Child 1", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})
	db.CreateTask(Task{ID: "t-child2", ParentID: &parent, Title: "Child 2", Status: "open", RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now})

	// Delete parent
	err := db.DeleteTask("t-parent")
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Children should also be deleted
	children, _ := db.ListChildTasks("t-parent")
	if len(children) != 0 {
		t.Errorf("Children still exist after parent delete: %d", len(children))
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := db.DeleteTask("t-nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("Expected ErrTaskNotFound, got: %v", err)
	}
}

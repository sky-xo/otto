# Task Persistence Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add SQLite-backed task persistence so orchestrators can resume after context compaction.

**Architecture:** New `tasks` table in existing `~/.june/june.db`. New `june task` subcommand with create/list/update/delete operations. Tasks scoped to git repo+branch like existing agents. IDs use `t-` prefix + 5-char hex hash (~1M possibilities, collision retry makes this safe up to 100K+ tasks).

**Tech Stack:** Go, Cobra CLI, modernc.org/sqlite, existing scope package for git detection

**Design Doc:** `docs/plans/2026-01-07-task-persistence-design.md`

---

## Task 1: Add tasks table schema

**Files:**
- Modify: `internal/db/db.go:58-69` (add schema)
- Modify: `internal/db/db.go:104-143` (add migrations)
- Test: `internal/db/db_test.go`

**Step 1: Write failing test for task table existence**

Add to `internal/db/db_test.go`:

```go
func TestTasksTableExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Query pragma to verify table structure
	// Note: DB embeds *sql.DB, so methods are promoted directly
	rows, err := db.Query("PRAGMA table_info(tasks)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		columns[name] = typ
	}

	expected := map[string]string{
		"id":         "TEXT",
		"parent_id":  "TEXT",
		"title":      "TEXT",
		"status":     "TEXT",
		"notes":      "TEXT",
		"created_at": "TEXT",
		"updated_at": "TEXT",
		"deleted_at": "TEXT",
		"repo_path":  "TEXT",
		"branch":     "TEXT",
	}

	for col, typ := range expected {
		if columns[col] != typ {
			t.Errorf("Column %s: got type %q, want %q", col, columns[col], typ)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/db -run TestTasksTableExists -v
```

Expected: FAIL - no tasks table

**Step 3: Add tasks table to schema**

In `internal/db/db.go`, find the schema constant (around line 58) and add after the `agents` table:

```go
const schema = `
CREATE TABLE IF NOT EXISTS agents (
    -- existing agents table...
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    notes TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    repo_path TEXT NOT NULL,
    branch TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_scope ON tasks(repo_path, branch);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
`
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/db -run TestTasksTableExists -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add tasks table schema"
```

---

## Task 2: Add Task type and CRUD operations

**Files:**
- Create: `internal/db/task.go`
- Test: `internal/db/task_test.go`

### Step 1: Write failing test for CreateTask

Create `internal/db/task_test.go`:

```go
package db

import (
	"testing"
	"time"
)

func TestCreateTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	task := Task{
		ID:        "t-a3f8",
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
	got, err := db.GetTask("t-a3f8")
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
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/db -run TestCreateTask -v
```

Expected: FAIL - Task type undefined

### Step 3: Create Task type and CreateTask method

Create `internal/db/task.go`:

```go
package db

import (
	"fmt"
	"time"
)

// Task represents a persistent task in the database
type Task struct {
	ID        string
	ParentID  *string // nil for root tasks
	Title     string
	Status    string // "open", "in_progress", "closed"
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // nil = not deleted
	RepoPath  string
	Branch    string
}

// CreateTask inserts a new task into the database
func (d *DB) CreateTask(t Task) error {
	_, err := d.db.Exec(`
		INSERT INTO tasks (id, parent_id, title, status, notes, created_at, updated_at, deleted_at, repo_path, branch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ParentID, t.Title, t.Status, t.Notes,
		t.CreatedAt.Format(time.RFC3339),
		t.UpdatedAt.Format(time.RFC3339),
		nil, // deleted_at
		t.RepoPath, t.Branch,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetTask retrieves a task by ID
func (d *DB) GetTask(id string) (*Task, error) {
	row := d.db.QueryRow(`
		SELECT id, parent_id, title, status, notes, created_at, updated_at, deleted_at, repo_path, branch
		FROM tasks WHERE id = ?`, id)

	var t Task
	var parentID, notes, deletedAt *string
	var createdAt, updatedAt string

	err := row.Scan(&t.ID, &parentID, &t.Title, &t.Status, &notes,
		&createdAt, &updatedAt, &deletedAt, &t.RepoPath, &t.Branch)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("scan task: %w", err)
	}

	t.ParentID = parentID
	t.Notes = notes
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if deletedAt != nil {
		dt, _ := time.Parse(time.RFC3339, *deletedAt)
		t.DeletedAt = &dt
	}

	return &t, nil
}

// ErrTaskNotFound is returned when a task doesn't exist
var ErrTaskNotFound = fmt.Errorf("task not found")
```

### Step 4: Run test to verify it passes

```bash
go test ./internal/db -run TestCreateTask -v
```

Expected: PASS

### Step 5: Commit

```bash
git add internal/db/task.go internal/db/task_test.go
git commit -m "feat(db): add Task type with Create and Get operations"
```

---

## Task 3: Add UpdateTask operation

**Files:**
- Modify: `internal/db/task.go`
- Test: `internal/db/task_test.go`

### Step 1: Write failing test for UpdateTask

Add to `internal/db/task_test.go`:

```go
func TestUpdateTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create initial task
	task := Task{
		ID:        "t-b2c3",
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
	err := db.UpdateTask("t-b2c3", TaskUpdate{
		Status: &newStatus,
		Notes:  &newNotes,
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	// Verify
	got, _ := db.GetTask("t-b2c3")
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
		ID:        "t-c3d4",
		Title:     "Original",
		Status:    "open",
		RepoPath:  "/home/user/myapp",
		Branch:    "main",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.CreateTask(task)

	newTitle := "Updated title"
	err := db.UpdateTask("t-c3d4", TaskUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	got, _ := db.GetTask("t-c3d4")
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
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/db -run TestUpdateTask -v
```

Expected: FAIL - TaskUpdate type undefined

### Step 3: Implement UpdateTask

Add to `internal/db/task.go`:

```go
// TaskUpdate holds optional fields for updating a task
type TaskUpdate struct {
	Title  *string
	Status *string
	Notes  *string
}

// UpdateTask updates specified fields of a task
func (d *DB) UpdateTask(id string, update TaskUpdate) error {
	// Build dynamic query based on which fields are set
	setParts := []string{"updated_at = ?"}
	args := []any{time.Now().Format(time.RFC3339)}

	if update.Title != nil {
		setParts = append(setParts, "title = ?")
		args = append(args, *update.Title)
	}
	if update.Status != nil {
		setParts = append(setParts, "status = ?")
		args = append(args, *update.Status)
	}
	if update.Notes != nil {
		setParts = append(setParts, "notes = ?")
		args = append(args, *update.Notes)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ? AND deleted_at IS NULL",
		strings.Join(setParts, ", "))

	result, err := d.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}
```

Add `"strings"` to the imports at the top of the file.

### Step 4: Run tests to verify they pass

```bash
go test ./internal/db -run TestUpdateTask -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/db/task.go internal/db/task_test.go
git commit -m "feat(db): add UpdateTask operation with partial updates"
```

---

## Task 4: Add ListTasks operations

**Files:**
- Modify: `internal/db/task.go`
- Test: `internal/db/task_test.go`

### Step 1: Write failing tests for ListTasks

Add to `internal/db/task_test.go`:

```go
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
	// Note: DB embeds *sql.DB, so Exec is called directly on db
	db.Exec("UPDATE tasks SET deleted_at = ? WHERE id = ?", now.Format(time.RFC3339), "t-deleted")

	tasks, _ := db.ListRootTasks("/app", "main")
	if len(tasks) != 1 {
		t.Errorf("Got %d tasks, want 1 (deleted should be excluded)", len(tasks))
	}
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/db -run TestList -v
```

Expected: FAIL - ListRootTasks undefined

### Step 3: Implement ListTasks operations

Add to `internal/db/task.go`:

```go
// ListRootTasks returns all non-deleted root tasks (no parent) for a scope
func (d *DB) ListRootTasks(repoPath, branch string) ([]Task, error) {
	rows, err := d.db.Query(`
		SELECT id, parent_id, title, status, notes, created_at, updated_at, deleted_at, repo_path, branch
		FROM tasks
		WHERE parent_id IS NULL
		  AND deleted_at IS NULL
		  AND repo_path = ?
		  AND branch = ?
		ORDER BY created_at DESC`,
		repoPath, branch)
	if err != nil {
		return nil, fmt.Errorf("query root tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// ListChildTasks returns all non-deleted children of a task
func (d *DB) ListChildTasks(parentID string) ([]Task, error) {
	rows, err := d.db.Query(`
		SELECT id, parent_id, title, status, notes, created_at, updated_at, deleted_at, repo_path, branch
		FROM tasks
		WHERE parent_id = ?
		  AND deleted_at IS NULL
		ORDER BY created_at ASC`,
		parentID)
	if err != nil {
		return nil, fmt.Errorf("query child tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// CountChildren returns the number of non-deleted children for a task
func (d *DB) CountChildren(parentID string) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM tasks
		WHERE parent_id = ? AND deleted_at IS NULL`,
		parentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count children: %w", err)
	}
	return count, nil
}

// scanTasks converts rows to Task slice
func scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var t Task
		var parentID, notes, deletedAt *string
		var createdAt, updatedAt string

		err := rows.Scan(&t.ID, &parentID, &t.Title, &t.Status, &notes,
			&createdAt, &updatedAt, &deletedAt, &t.RepoPath, &t.Branch)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		t.ParentID = parentID
		t.Notes = notes
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if deletedAt != nil {
			dt, _ := time.Parse(time.RFC3339, *deletedAt)
			t.DeletedAt = &dt
		}

		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
```

Add `"database/sql"` to imports.

### Step 4: Run tests to verify they pass

```bash
go test ./internal/db -run TestList -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/db/task.go internal/db/task_test.go
git commit -m "feat(db): add ListRootTasks, ListChildTasks, and CountChildren"
```

---

## Task 5: Add DeleteTask operation (soft delete with cascade)

**Files:**
- Modify: `internal/db/task.go`
- Test: `internal/db/task_test.go`

### Step 1: Write failing tests for DeleteTask

Add to `internal/db/task_test.go`:

```go
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
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/db -run TestDeleteTask -v
```

Expected: FAIL - DeleteTask undefined

### Step 3: Implement DeleteTask with cascade

Add to `internal/db/task.go`:

```go
// DeleteTask soft-deletes a task and all its children
func (d *DB) DeleteTask(id string) error {
	now := time.Now().Format(time.RFC3339)

	// First check if task exists
	_, err := d.GetTask(id)
	if err != nil {
		return err
	}

	// Soft delete children first (recursive via CTE)
	_, err = d.db.Exec(`
		WITH RECURSIVE descendants AS (
			SELECT id FROM tasks WHERE parent_id = ?
			UNION ALL
			SELECT t.id FROM tasks t
			INNER JOIN descendants d ON t.parent_id = d.id
		)
		UPDATE tasks SET deleted_at = ?
		WHERE id IN (SELECT id FROM descendants)`,
		id, now)
	if err != nil {
		return fmt.Errorf("delete children: %w", err)
	}

	// Soft delete the task itself
	result, err := d.db.Exec(`UPDATE tasks SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, now, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./internal/db -run TestDeleteTask -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/db/task.go internal/db/task_test.go
git commit -m "feat(db): add DeleteTask with recursive cascade to children"
```

---

## Task 6: Add task ID generation with collision retry

**Files:**
- Create: `internal/cli/taskid.go`
- Test: `internal/cli/taskid_test.go`

### Step 1: Write failing test for task ID generation

Create `internal/cli/taskid_test.go`:

```go
package cli

import (
	"regexp"
	"testing"

	"github.com/sky-xo/june/internal/db"
)

func TestGenerateTaskID(t *testing.T) {
	id := generateTaskID()

	// Must match pattern: t-[7 hex chars]
	pattern := regexp.MustCompile(`^t-[a-f0-9]{5}$`)
	if !pattern.MatchString(id) {
		t.Errorf("ID %q doesn't match pattern t-[a-f0-9]{5}", id)
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
	database.CreateTask(db.Task{
		ID: "t-12345", Title: "Existing", Status: "open",
		RepoPath: "/app", Branch: "main", CreatedAt: now, UpdatedAt: now,
	})

	// Generate new ID - should not collide with existing
	id, err := generateUniqueTaskID(database)
	if err != nil {
		t.Fatalf("generateUniqueTaskID failed: %v", err)
	}
	if id == "t-12345" {
		t.Error("Generated ID should not match existing task")
	}
}
```

Add imports: `"path/filepath"`, `"time"`

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestGenerateTaskID -v
go test ./internal/cli -run TestGenerateUniqueTaskID -v
```

Expected: FAIL - generateTaskID undefined

### Step 3: Implement task ID generation with collision retry

Create `internal/cli/taskid.go`:

```go
package cli

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/sky-xo/june/internal/db"
)

// generateTaskID creates a task ID with format "t-xxxxx"
// where xxxxx is 5 random hex characters (~1M possibilities)
// With collision retry, this is safe up to 100K+ tasks
func generateTaskID() string {
	b := make([]byte, 3) // 3 bytes = 6 hex chars, we use 5
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	// Use first 5 hex chars (20 bits = ~1M possibilities)
	return fmt.Sprintf("t-%05x", b)[:7] // "t-" + 5 chars = 7 total
}

// generateUniqueTaskID generates a task ID that doesn't exist in the database.
// Retries up to 10 times on collision (following pattern from name.go).
func generateUniqueTaskID(database *db.DB) (string, error) {
	for i := 0; i < 10; i++ {
		id := generateTaskID()
		_, err := database.GetTask(id)
		if err == db.ErrTaskNotFound {
			return id, nil // ID is available
		}
		if err != nil {
			return "", fmt.Errorf("check task existence: %w", err)
		}
		// ID exists, retry
	}
	return "", errors.New("failed to generate unique task ID after 10 attempts")
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestGenerateTaskID -v
go test ./internal/cli -run TestGenerateUniqueTaskID -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/taskid.go internal/cli/taskid_test.go
git commit -m "feat(cli): add task ID generation with 5-char hex and collision retry"
```

---

## Task 7: Add `june task` root command

**Files:**
- Create: `internal/cli/task.go`
- Modify: `internal/cli/root.go` (add subcommand)
- Test: `internal/cli/task_test.go`

### Step 1: Write failing test for task command structure

Create `internal/cli/task_test.go`:

```go
package cli

import (
	"testing"
)

func TestTaskCommandExists(t *testing.T) {
	cmd := newTaskCmd()
	if cmd.Use != "task" {
		t.Errorf("Use = %q, want %q", cmd.Use, "task")
	}

	// Check subcommands exist
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}

	expected := []string{"create", "list", "update", "delete"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("Missing subcommand: %s", name)
		}
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/cli -run TestTaskCommandExists -v
```

Expected: FAIL - newTaskCmd undefined

### Step 3: Create task command skeleton

Create `internal/cli/task.go`:

```go
package cli

import (
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage persistent tasks",
		Long:  "Create, list, update, and delete tasks that persist across context compaction.",
	}

	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskUpdateCmd())
	cmd.AddCommand(newTaskDeleteCmd())

	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <title> [titles...]",
		Short: "Create one or more tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runTaskCreate,
	}
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [task-id]",
		Short: "List tasks or show task details",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTaskList,
	}
}

func newTaskUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE:  runTaskUpdate,
	}
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task and its children",
		Args:  cobra.ExactArgs(1),
		RunE:  runTaskDelete,
	}
}

// Placeholder implementations - will be filled in subsequent tasks
func runTaskCreate(cmd *cobra.Command, args []string) error {
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	return nil
}

func runTaskUpdate(cmd *cobra.Command, args []string) error {
	return nil
}

func runTaskDelete(cmd *cobra.Command, args []string) error {
	return nil
}
```

### Step 4: Add task command to root

In `internal/cli/root.go`, find where other commands are added (around `rootCmd.AddCommand`) and add:

```go
rootCmd.AddCommand(newTaskCmd())
```

### Step 5: Run test to verify it passes

```bash
go test ./internal/cli -run TestTaskCommandExists -v
```

Expected: PASS

### Step 6: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go internal/cli/root.go
git commit -m "feat(cli): add june task command skeleton with subcommands"
```

---

## Task 8: Implement `june task create`

**Files:**
- Modify: `internal/cli/task.go`
- Test: `internal/cli/task_test.go`

### Step 1: Write integration test for task create

Add to `internal/cli/task_test.go`:

```go
func TestTaskCreateSingle(t *testing.T) {
	// Setup temp DB
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	// Create a mock git repo context
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	// Should output task ID like "t-a3f8b" (7 hex chars)
	pattern := regexp.MustCompile(`t-[a-f0-9]{5}`)
	if !pattern.MatchString(output) {
		t.Errorf("Output %q doesn't contain task ID", output)
	}
}

func TestTaskCreateWithParent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create parent first
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Parent task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()

	parentID := strings.TrimSpace(stdout.String())

	// Create child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Child task", "--parent", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	childID := strings.TrimSpace(stdout.String())
	if childID == parentID {
		t.Error("Child should have different ID than parent")
	}
}

func TestTaskCreateMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 1", "Task 2", "Task 3"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 task IDs, got %d", len(lines))
	}
}
```

Add these imports at the top:

```go
import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestTaskCreate -v
```

Expected: FAIL - tests fail because runTaskCreate does nothing

### Step 3: Implement runTaskCreate

Update `internal/cli/task.go`:

```go
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/scope"
	"github.com/spf13/cobra"
)

var taskCreateParent string
var taskOutputJSON bool

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage persistent tasks",
		Long:  "Create, list, update, and delete tasks that persist across context compaction.",
	}

	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskUpdateCmd())
	cmd.AddCommand(newTaskDeleteCmd())

	// Global flag for JSON output
	cmd.PersistentFlags().BoolVar(&taskOutputJSON, "json", false, "Output in JSON format")

	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <title> [titles...]",
		Short: "Create one or more tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runTaskCreate,
	}

	cmd.Flags().StringVar(&taskCreateParent, "parent", "", "Parent task ID for creating child tasks")

	return cmd
}

func runTaskCreate(cmd *cobra.Command, args []string) error {
	// Get git scope
	// Note: scope.RepoRoot() and scope.BranchName() return string only (not error)
	// They return empty string if not in a git repo
	repoPath := scope.RepoRoot()
	if repoPath == "" {
		return fmt.Errorf("not in a git repository")
	}
	branch := scope.BranchName()
	if branch == "" {
		branch = "main" // fallback
	}

	// Open database
	dbPath, err := juneDBPath()
	if err != nil {
		return err
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Validate parent exists if specified
	var parentID *string
	if taskCreateParent != "" {
		_, err := database.GetTask(taskCreateParent)
		if err != nil {
			return fmt.Errorf("parent task %q not found", taskCreateParent)
		}
		parentID = &taskCreateParent
	}

	// Create tasks
	now := time.Now()
	for _, title := range args {
		id, err := generateUniqueTaskID(database)
		if err != nil {
			return fmt.Errorf("generate task ID: %w", err)
		}
		task := db.Task{
			ID:        id,
			ParentID:  parentID,
			Title:     title,
			Status:    "open",
			RepoPath:  repoPath,
			Branch:    branch,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := database.CreateTask(task); err != nil {
			return fmt.Errorf("create task: %w", err)
		}

		if taskOutputJSON {
			fmt.Fprintf(cmd.OutOrStdout(), `{"id":"%s"}`+"\n", id)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), id)
		}
	}

	return nil
}

// juneDBPath returns the path to the june database
func juneDBPath() (string, error) {
	home, err := juneHome()
	if err != nil {
		return "", err
	}
	return home + "/june.db", nil
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestTaskCreate -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): implement june task create command"
```

---

## Task 9: Implement `june task list`

**Files:**
- Modify: `internal/cli/task.go`
- Test: `internal/cli/task_test.go`

### Step 1: Write tests for task list

Add to `internal/cli/task_test.go`:

```go
func TestTaskListRoot(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create tasks
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 2"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// List root tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 2") {
		t.Errorf("List output should contain both tasks: %s", output)
	}
}

func TestTaskListSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create parent
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Parent task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	parentID := strings.TrimSpace(stdout.String())

	// Create children
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Child 1", "Child 2", "--parent", parentID})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// List specific task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Parent task") {
		t.Errorf("Should show task title: %s", output)
	}
	if !strings.Contains(output, "Child 1") || !strings.Contains(output, "Child 2") {
		t.Errorf("Should show children: %s", output)
	}
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestTaskList -v
```

Expected: FAIL - runTaskList returns nothing

### Step 3: Implement runTaskList

Add to `internal/cli/task.go`:

```go
func runTaskList(cmd *cobra.Command, args []string) error {
	// Get git scope
	// Note: scope.RepoRoot() and scope.BranchName() return string only (not error)
	repoPath := scope.RepoRoot()
	if repoPath == "" {
		return fmt.Errorf("not in a git repository")
	}
	branch := scope.BranchName()
	if branch == "" {
		branch = "main"
	}

	// Open database
	dbPath, err := juneDBPath()
	if err != nil {
		return err
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	out := cmd.OutOrStdout()

	// If task ID provided, show that task + its children
	if len(args) == 1 {
		taskID := args[0]
		return listSpecificTask(database, taskID, out)
	}

	// Otherwise list root tasks
	return listRootTasks(database, repoPath, branch, out)
}

func listRootTasks(database *db.DB, repoPath, branch string, out *os.File) error {
	tasks, err := database.ListRootTasks(repoPath, branch)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, "No tasks.")
		return nil
	}

	for _, t := range tasks {
		childCount, _ := database.CountChildren(t.ID)
		childSuffix := ""
		if childCount > 0 {
			childSuffix = fmt.Sprintf("  (%d children)", childCount)
		}
		fmt.Fprintf(out, "%s  %s  [%s]%s\n", t.ID, t.Title, t.Status, childSuffix)
	}

	return nil
}

func listSpecificTask(database *db.DB, taskID string, out *os.File) error {
	task, err := database.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	// Show task details
	fmt.Fprintf(out, "%s %q [%s]\n", task.ID, task.Title, task.Status)

	if task.ParentID != nil {
		fmt.Fprintf(out, "  Parent: %s\n", *task.ParentID)
	}

	if task.Notes != nil && *task.Notes != "" {
		fmt.Fprintf(out, "  Note: %s\n", *task.Notes)
	}

	// Show children
	children, err := database.ListChildTasks(taskID)
	if err != nil {
		return fmt.Errorf("list children: %w", err)
	}

	fmt.Fprintln(out)
	if len(children) == 0 {
		fmt.Fprintln(out, "No children.")
	} else {
		fmt.Fprintln(out, "Children:")
		for _, c := range children {
			fmt.Fprintf(out, "  %s  %s  [%s]\n", c.ID, c.Title, c.Status)
		}
	}

	return nil
}
```

Note: The function signature uses `*os.File` but should use `io.Writer`. Fix this:

```go
import "io"

func listRootTasks(database *db.DB, repoPath, branch string, out io.Writer) error {
    // ... same implementation
}

func listSpecificTask(database *db.DB, taskID string, out io.Writer) error {
    // ... same implementation
}
```

And update `runTaskList` to use `cmd.OutOrStdout()` which returns `io.Writer`.

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestTaskList -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): implement june task list command"
```

---

## Task 10: Implement `june task update`

**Files:**
- Modify: `internal/cli/task.go`
- Test: `internal/cli/task_test.go`

### Step 1: Write tests for task update

Add to `internal/cli/task_test.go`:

```go
func TestTaskUpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	taskID := strings.TrimSpace(stdout.String())

	// Update status
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", taskID, "--status", "in_progress"})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify via list
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	if !strings.Contains(stdout.String(), "[in_progress]") {
		t.Errorf("Status not updated: %s", stdout.String())
	}
}

func TestTaskUpdateNote(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	taskID := strings.TrimSpace(stdout.String())

	// Update note
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", taskID, "--note", "Important note"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// Verify
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	if !strings.Contains(stdout.String(), "Important note") {
		t.Errorf("Note not shown: %s", stdout.String())
	}
}

func TestTaskUpdateMultipleFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	taskID := strings.TrimSpace(stdout.String())

	// Update multiple fields atomically
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", taskID, "--status", "closed", "--note", "Done", "--title", "Updated title"})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify all fields
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output := stdout.String()
	if !strings.Contains(output, "[closed]") {
		t.Errorf("Status not updated: %s", output)
	}
	if !strings.Contains(output, "Done") {
		t.Errorf("Note not updated: %s", output)
	}
	if !strings.Contains(output, "Updated title") {
		t.Errorf("Title not updated: %s", output)
	}
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestTaskUpdate -v
```

Expected: FAIL

### Step 3: Implement runTaskUpdate

Update `newTaskUpdateCmd` and add `runTaskUpdate`:

```go
var (
	taskUpdateStatus string
	taskUpdateNote   string
	taskUpdateTitle  string
)

func newTaskUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE:  runTaskUpdate,
	}

	cmd.Flags().StringVar(&taskUpdateStatus, "status", "", "Set status (open, in_progress, closed)")
	cmd.Flags().StringVar(&taskUpdateNote, "note", "", "Set note")
	cmd.Flags().StringVar(&taskUpdateTitle, "title", "", "Set title")

	return cmd
}

func runTaskUpdate(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Build update struct
	update := db.TaskUpdate{}
	if taskUpdateStatus != "" {
		// Validate status
		valid := map[string]bool{"open": true, "in_progress": true, "closed": true}
		if !valid[taskUpdateStatus] {
			return fmt.Errorf("invalid status %q (use: open, in_progress, closed)", taskUpdateStatus)
		}
		update.Status = &taskUpdateStatus
	}
	if taskUpdateNote != "" {
		update.Notes = &taskUpdateNote
	}
	if taskUpdateTitle != "" {
		update.Title = &taskUpdateTitle
	}

	// Check if any update provided
	if update.Status == nil && update.Notes == nil && update.Title == nil {
		return fmt.Errorf("no update provided (use --status, --note, or --title)")
	}

	// Open database
	dbPath, err := juneDBPath()
	if err != nil {
		return err
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Perform update
	err = database.UpdateTask(taskID, update)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("update task: %w", err)
	}

	return nil
}
```

Note: Reset flag values in tests or use `cmd.ResetFlags()`. A better pattern is to use local variables captured by the closure. Let's refactor:

```go
func newTaskUpdateCmd() *cobra.Command {
	var status, note, title string

	cmd := &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskUpdate(cmd, args, status, note, title)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Set status (open, in_progress, closed)")
	cmd.Flags().StringVar(&note, "note", "", "Set note")
	cmd.Flags().StringVar(&title, "title", "", "Set title")

	return cmd
}

func runTaskUpdate(cmd *cobra.Command, args []string, status, note, title string) error {
	// ... use the passed parameters instead of globals
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestTaskUpdate -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): implement june task update command"
```

---

## Task 11: Implement `june task delete`

**Files:**
- Modify: `internal/cli/task.go`
- Test: `internal/cli/task_test.go`

### Step 1: Write tests for task delete

Add to `internal/cli/task_test.go`:

```go
func TestTaskDelete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	taskID := strings.TrimSpace(stdout.String())

	// Delete task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"delete", taskID})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify not in list
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	if strings.Contains(stdout.String(), taskID) {
		t.Errorf("Deleted task still appears in list")
	}
}

func TestTaskDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"delete", "t-nonexistent"})
	cmd.SetOut(&bytes.Buffer{})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for nonexistent task")
	}
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestTaskDelete -v
```

Expected: FAIL

### Step 3: Implement runTaskDelete

```go
func runTaskDelete(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Open database
	dbPath, err := juneDBPath()
	if err != nil {
		return err
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Delete task (soft delete with cascade)
	err = database.DeleteTask(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}
```

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestTaskDelete -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): implement june task delete command"
```

---

## Task 12: Add JSON output support

**Files:**
- Modify: `internal/cli/task.go`
- Test: `internal/cli/task_test.go`

### Step 1: Write tests for JSON output

Add to `internal/cli/task_test.go`:

```go
func TestTaskListJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()

	// List with JSON
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", "--json"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	// Should be valid JSON array
	var tasks []map[string]any
	err := json.Unmarshal(stdout.Bytes(), &tasks)
	if err != nil {
		t.Fatalf("Invalid JSON: %v\nOutput: %s", err, stdout.String())
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestTaskCreateJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task", "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()

	var result map[string]string
	err := json.Unmarshal(stdout.Bytes(), &result)
	if err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if result["id"] == "" {
		t.Error("Expected id in JSON output")
	}
}
```

Add `"encoding/json"` to imports.

### Step 2: Run tests to verify they fail

```bash
go test ./internal/cli -run TestTask.*JSON -v
```

Expected: FAIL - JSON output not implemented for list

### Step 3: Implement JSON output for list

Update `listRootTasks` and `listSpecificTask` to check `taskOutputJSON`:

```go
import "encoding/json"

func listRootTasks(database *db.DB, repoPath, branch string, out io.Writer, asJSON bool) error {
	tasks, err := database.ListRootTasks(repoPath, branch)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if asJSON {
		type taskOutput struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Status   string `json:"status"`
			Children int    `json:"children"`
		}
		output := make([]taskOutput, len(tasks))
		for i, t := range tasks {
			childCount, _ := database.CountChildren(t.ID)
			output[i] = taskOutput{
				ID:       t.ID,
				Title:    t.Title,
				Status:   t.Status,
				Children: childCount,
			}
		}
		enc := json.NewEncoder(out)
		return enc.Encode(output)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, "No tasks.")
		return nil
	}

	for _, t := range tasks {
		childCount, _ := database.CountChildren(t.ID)
		childSuffix := ""
		if childCount > 0 {
			childSuffix = fmt.Sprintf("  (%d children)", childCount)
		}
		fmt.Fprintf(out, "%s  %s  [%s]%s\n", t.ID, t.Title, t.Status, childSuffix)
	}

	return nil
}

func listSpecificTask(database *db.DB, taskID string, out io.Writer, asJSON bool) error {
	task, err := database.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	children, err := database.ListChildTasks(taskID)
	if err != nil {
		return fmt.Errorf("list children: %w", err)
	}

	if asJSON {
		type childOutput struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}
		type taskOutput struct {
			ID       string        `json:"id"`
			ParentID *string       `json:"parent_id,omitempty"`
			Title    string        `json:"title"`
			Status   string        `json:"status"`
			Notes    *string       `json:"notes,omitempty"`
			Children []childOutput `json:"children"`
		}

		output := taskOutput{
			ID:       task.ID,
			ParentID: task.ParentID,
			Title:    task.Title,
			Status:   task.Status,
			Notes:    task.Notes,
			Children: make([]childOutput, len(children)),
		}
		for i, c := range children {
			output.Children[i] = childOutput{ID: c.ID, Title: c.Title, Status: c.Status}
		}

		enc := json.NewEncoder(out)
		return enc.Encode(output)
	}

	// Human-readable output (existing code)
	fmt.Fprintf(out, "%s %q [%s]\n", task.ID, task.Title, task.Status)
	// ... rest of human-readable output
}
```

Update `runTaskList` to pass `taskOutputJSON` to these functions.

### Step 4: Run tests to verify they pass

```bash
go test ./internal/cli -run TestTask.*JSON -v
```

Expected: All PASS

### Step 5: Commit

```bash
git add internal/cli/task.go internal/cli/task_test.go
git commit -m "feat(cli): add --json flag for machine-readable output"
```

---

## Task 13: Add database migration for existing installations

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

### Step 1: Write migration test

Add to `internal/db/db_test.go`:

```go
func TestMigrationAddsTasksTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create DB with only agents table (simulating old schema)
	oldDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_, err = oldDB.Exec(`
		CREATE TABLE agents (
			name TEXT PRIMARY KEY,
			ulid TEXT,
			session_file TEXT,
			cursor INTEGER DEFAULT 0,
			pid INTEGER,
			spawned_at TEXT,
			repo_path TEXT DEFAULT '',
			branch TEXT DEFAULT '',
			type TEXT DEFAULT 'codex'
		)
	`)
	if err != nil {
		t.Fatalf("Create agents table failed: %v", err)
	}
	oldDB.Close()

	// Open with our DB package (should migrate)
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with migration failed: %v", err)
	}
	defer database.Close()

	// Verify tasks table exists
	// Note: DB embeds *sql.DB, so Query is called directly
	rows, err := database.Query("PRAGMA table_info(tasks)")
	if err != nil {
		t.Fatalf("Query pragma failed: %v", err)
	}
	defer rows.Close()

	hasTasksTable := false
	for rows.Next() {
		hasTasksTable = true
		break
	}

	if !hasTasksTable {
		t.Error("tasks table not created during migration")
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/db -run TestMigrationAddsTasksTable -v
```

Expected: FAIL - tasks table not created

### Step 3: Update migration logic

In `internal/db/db.go`, update the `migrate` function to add tasks table if missing:

```go
func migrate(db *sql.DB) error {
	// Existing agent migrations...

	// Add tasks table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			parent_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			notes TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT,
			repo_path TEXT NOT NULL,
			branch TEXT NOT NULL,
			FOREIGN KEY (parent_id) REFERENCES tasks(id)
		);
		CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_scope ON tasks(repo_path, branch);
		CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	`)
	if err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}

	return nil
}
```

### Step 4: Run test to verify it passes

```bash
go test ./internal/db -run TestMigrationAddsTasksTable -v
```

Expected: PASS

### Step 5: Commit

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add migration for tasks table on existing installations"
```

---

## Task 14: Integration testing and documentation

**Files:**
- Test: `internal/cli/task_integration_test.go`
- Modify: `CLAUDE.md` (update docs)

### Step 1: Write end-to-end integration test

Create `internal/cli/task_integration_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskWorkflowE2E(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	// Setup git repo
	repoDir := filepath.Join(tmpDir, "myproject")
	os.MkdirAll(repoDir, 0755)
	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "feature-auth").Run()

	var stdout bytes.Buffer

	// 1. Create root task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Implement auth feature"})
	cmd.SetOut(&stdout)
	cmd.Execute()
	parentID := strings.TrimSpace(stdout.String())
	t.Logf("Created parent: %s", parentID)

	// 2. Create child tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Add middleware", "Write tests", "Update docs", "--parent", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()
	childIDs := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	t.Logf("Created children: %v", childIDs)

	// 3. Start work on first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--status", "in_progress"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 4. Add note to first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--note", "Mock server needs HTTPS"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 5. Complete first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--status", "closed"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 6. List parent to see progress
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output := stdout.String()
	t.Logf("Task list output:\n%s", output)

	// Verify expected state
	if !strings.Contains(output, "Implement auth feature") {
		t.Error("Parent title not shown")
	}
	if !strings.Contains(output, "[closed]") {
		t.Error("First child should be closed")
	}
	if !strings.Contains(output, "[open]") {
		t.Error("Other children should be open")
	}
	if !strings.Contains(output, "Mock server needs HTTPS") {
		t.Error("Note should be shown")
	}

	// 7. List root tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output = stdout.String()
	if !strings.Contains(output, parentID) {
		t.Error("Parent should appear in root list")
	}
	if !strings.Contains(output, "(3 children)") {
		t.Error("Should show children count")
	}
}
```

### Step 2: Run integration test

```bash
go test ./internal/cli -run TestTaskWorkflowE2E -v
```

Expected: PASS

### Step 3: Update CLAUDE.md documentation

Add to `CLAUDE.md` under the "Usage" section:

```markdown
## Task Commands

Persist tasks across context compaction:

```bash
# Create root task
june task create "Implement auth feature"   # Output: t-a3f8b

# Create child tasks
june task create "Add middleware" --parent t-a3f8b

# List root tasks
june task list

# Show task details and children
june task list t-a3f8b

# Update task
june task update t-a3f8b --status in_progress
june task update t-a3f8b --note "Started work"
june task update t-a3f8b --status closed --note "Done"

# Delete task (soft delete, cascades to children)
june task delete t-a3f8b
```

Task state is stored in `~/.june/june.db` (same as agent state).
```

### Step 4: Commit

```bash
git add internal/cli/task_integration_test.go CLAUDE.md
git commit -m "test: add task workflow E2E test and update docs"
```

---

## Task 15: Final verification

### Step 1: Run all tests

```bash
make test
```

Expected: All tests pass

### Step 2: Run build

```bash
make build
```

Expected: Binary builds successfully

### Step 3: Manual smoke test

```bash
cd /tmp && git init test-tasks && cd test-tasks
../path/to/june task create "Test task"
../path/to/june task list
../path/to/june task list --json
```

### Step 4: Commit any final fixes

If any issues found, fix and commit.

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add tasks table schema | `internal/db/db.go`, tests |
| 2 | Task type + Create/Get | `internal/db/task.go`, tests |
| 3 | UpdateTask operation | `internal/db/task.go`, tests |
| 4 | ListTasks operations | `internal/db/task.go`, tests |
| 5 | DeleteTask with cascade | `internal/db/task.go`, tests |
| 6 | Task ID generation (5-char + collision retry) | `internal/cli/taskid.go`, tests |
| 7 | `june task` root command | `internal/cli/task.go`, `root.go`, tests |
| 8 | `june task create` | `internal/cli/task.go`, tests |
| 9 | `june task list` | `internal/cli/task.go`, tests |
| 10 | `june task update` | `internal/cli/task.go`, tests |
| 11 | `june task delete` | `internal/cli/task.go`, tests |
| 12 | JSON output support | `internal/cli/task.go`, tests |
| 13 | Database migration | `internal/db/db.go`, tests |
| 14 | Integration tests + docs | `internal/cli/task_integration_test.go`, `CLAUDE.md` |
| 15 | Final verification | All files |

**Total commits:** ~15 (one per task)

**Key dependencies:** None - uses existing packages (db, scope, cobra)

---

## Implementation Progress

> **For Claude resuming this work:** This section tracks implementation progress.

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 1 | ✅ Complete | `f722707` | Schema added, tests pass |
| 2 | ✅ Complete | `f7a1b54` | Task type with Create/Get |
| 3 | ✅ Complete | `ae45444` | UpdateTask with partial updates |
| 4 | ✅ Complete | `7dd86b5` | ListRootTasks, ListChildTasks, CountChildren |
| 5 | ✅ Complete | `f0d6a85` | DeleteTask with recursive cascade |
| 6 | ✅ Complete | `be341fd` | Task ID generation (5-char hex) |
| 7-12 | ✅ Complete | `2577a0f` | june task command with all subcommands |
| 13 | ✅ Complete | `c8b326f` | Database migration for existing installations |
| 14 | ✅ Complete | `b22e1d6` | Integration tests and docs |
| 15 | ✅ Complete | - | All tests pass, build succeeds |

**Current state:** All tasks complete. Implementation verified.

**Base commit (before implementation):** `2877a20fd70d9cc81df5fd58f288a8c0bd50d67f`

---

## Final Review (After All Tasks Complete)

> **IMPORTANT:** After Task 15 passes, run Fresh Eyes final review.

**Run TWO agents in PARALLEL:**

1. **Codex agent** (via june spawn):
```bash
june spawn codex "Review all changes since commit 2877a20. Use Fresh Eyes methodology: check for bugs, security issues, missing edge cases, test coverage gaps. Report: Files Examined, Issues Found (critical/major/minor/nit), Summary, REVIEW PASSED/FAILED." --reasoning-effort high --sandbox=read-only --name fresheyes-codex
```

2. **Claude subagent** (via Task tool):
```
Task tool with subagent_type="general-purpose":
  Use the Fresh Eyes review prompt from /Users/glowy/.claude/skills/fresheyes/fresheyes-prompt.md
  Review scope: All changes since commit 2877a20 (git diff 2877a20...HEAD)
  Report findings in same format as Codex
```

**Wait for BOTH to complete, then report combined findings.**

Do NOT use Gemini for the final review.

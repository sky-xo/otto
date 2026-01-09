package db

import (
	"database/sql"
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

// ErrTaskNotFound is returned when a task doesn't exist
var ErrTaskNotFound = fmt.Errorf("task not found")

// CreateTask inserts a new task into the database
func (d *DB) CreateTask(t Task) error {
	_, err := d.Exec(`
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
	row := d.QueryRow(`
		SELECT id, parent_id, title, status, notes, created_at, updated_at, deleted_at, repo_path, branch
		FROM tasks WHERE id = ?`, id)

	var t Task
	var parentID, notes, deletedAt *string
	var createdAt, updatedAt string

	err := row.Scan(&t.ID, &parentID, &t.Title, &t.Status, &notes,
		&createdAt, &updatedAt, &deletedAt, &t.RepoPath, &t.Branch)
	if err != nil {
		if err == sql.ErrNoRows {
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

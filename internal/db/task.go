package db

import (
	"database/sql"
	"fmt"
	"strings"
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
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("title cannot be empty")
	}

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
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	if deletedAt != nil {
		dt, err := time.Parse(time.RFC3339, *deletedAt)
		if err != nil {
			return nil, fmt.Errorf("parse deleted_at: %w", err)
		}
		t.DeletedAt = &dt
	}

	return &t, nil
}

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
		if strings.TrimSpace(*update.Title) == "" {
			return fmt.Errorf("title cannot be empty")
		}
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

	result, err := d.Exec(query, args...)
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

// ListRootTasks returns all non-deleted root tasks (no parent) for a scope
func (d *DB) ListRootTasks(repoPath, branch string) ([]Task, error) {
	rows, err := d.Query(`
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
	rows, err := d.Query(`
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
	err := d.QueryRow(`
		SELECT COUNT(*) FROM tasks
		WHERE parent_id = ? AND deleted_at IS NULL`,
		parentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count children: %w", err)
	}
	return count, nil
}

// DeleteTask soft-deletes a task and all its children
func (d *DB) DeleteTask(id string) error {
	now := time.Now().Format(time.RFC3339)

	// First check if task exists and is not already deleted
	task, err := d.GetTask(id)
	if err != nil {
		return err
	}
	if task.DeletedAt != nil {
		return ErrTaskNotFound
	}

	// Begin transaction
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // No-op if committed

	// Soft delete children first (recursive via CTE)
	_, err = tx.Exec(`
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
	result, err := tx.Exec(`UPDATE tasks SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, now, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
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
		t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		t.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}
		if deletedAt != nil {
			dt, err := time.Parse(time.RFC3339, *deletedAt)
			if err != nil {
				return nil, fmt.Errorf("parse deleted_at: %w", err)
			}
			t.DeletedAt = &dt
		}

		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

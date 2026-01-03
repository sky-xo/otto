package db

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrAgentNotFound is returned when an agent is not found
var ErrAgentNotFound = errors.New("agent not found")

// Agent represents a spawned Codex agent
type Agent struct {
	Name        string
	ULID        string
	SessionFile string
	Cursor      int
	PID         int
	SpawnedAt   time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name TEXT PRIMARY KEY,
	ulid TEXT NOT NULL,
	session_file TEXT NOT NULL,
	cursor INTEGER DEFAULT 0,
	pid INTEGER,
	spawned_at TEXT NOT NULL
);
`

// DB wraps a SQLite database connection
type DB struct {
	*sql.DB
}

// Open opens or creates the SQLite database at the given path
func Open(path string) (*DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}

// CreateAgent inserts a new agent record
func (db *DB) CreateAgent(a Agent) error {
	_, err := db.Exec(
		`INSERT INTO agents (name, ulid, session_file, cursor, pid, spawned_at)
		 VALUES (?, ?, ?, 0, ?, ?)`,
		a.Name, a.ULID, a.SessionFile, a.PID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetAgent retrieves an agent by name
func (db *DB) GetAgent(name string) (*Agent, error) {
	var a Agent
	var spawnedAt string
	err := db.QueryRow(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	var parseErr error
	a.SpawnedAt, parseErr = time.Parse(time.RFC3339, spawnedAt)
	if parseErr != nil {
		log.Printf("warning: failed to parse spawned_at for agent %s: %v", name, parseErr)
	}
	return &a, nil
}

// UpdateCursor updates the cursor position for an agent
func (db *DB) UpdateCursor(name string, cursor int) error {
	result, err := db.Exec(`UPDATE agents SET cursor = ? WHERE name = ?`, cursor, name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// UpdateSessionFile updates the session file path for an agent
func (db *DB) UpdateSessionFile(name string, sessionFile string) error {
	result, err := db.Exec(`UPDATE agents SET session_file = ? WHERE name = ?`, sessionFile, name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// ListAgents returns all agents
func (db *DB) ListAgents() ([]Agent, error) {
	rows, err := db.Query(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at
		 FROM agents ORDER BY spawned_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var spawnedAt string
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt); err != nil {
			return nil, err
		}
		var parseErr error
		a.SpawnedAt, parseErr = time.Parse(time.RFC3339, spawnedAt)
		if parseErr != nil {
			log.Printf("warning: failed to parse spawned_at for agent %s: %v", a.Name, parseErr)
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return agents, nil
}

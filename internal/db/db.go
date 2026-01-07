package db

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sky-xo/june/internal/agent"
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
	RepoPath    string // Git repo path for channel grouping
	Branch      string // Git branch for channel grouping
	Type        string // "codex" or "gemini"
}

// ToUnified converts a db.Agent to the unified agent.Agent type.
func (a Agent) ToUnified() agent.Agent {
	// Use file modification time for LastActivity, fall back to SpawnedAt
	lastActivity := a.SpawnedAt
	if a.SessionFile != "" {
		if info, err := os.Stat(a.SessionFile); err == nil {
			lastActivity = info.ModTime()
		}
	}

	source := agent.SourceCodex
	if a.Type == "gemini" {
		source = agent.SourceGemini
	}

	return agent.Agent{
		ID:             a.ULID,
		Name:           a.Name,
		Source:         source,
		RepoPath:       a.RepoPath,
		Branch:         a.Branch,
		TranscriptPath: a.SessionFile,
		LastActivity:   lastActivity,
		PID:            a.PID,
	}
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name TEXT PRIMARY KEY,
	ulid TEXT NOT NULL,
	session_file TEXT NOT NULL,
	cursor INTEGER DEFAULT 0,
	pid INTEGER,
	spawned_at TEXT NOT NULL,
	repo_path TEXT DEFAULT '',
	branch TEXT DEFAULT '',
	type TEXT DEFAULT 'codex'
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

	// Create schema (for new DBs)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	// Run migrations (for existing DBs)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db}, nil
}

// migrate runs schema migrations for existing databases
func migrate(db *sql.DB) error {
	// Check if repo_path column exists and add if missing
	var repoPathCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='repo_path'`).Scan(&repoPathCount)
	if err != nil {
		return err
	}
	if repoPathCount == 0 {
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN repo_path TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	// Check if branch column exists independently and add if missing
	var branchCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='branch'`).Scan(&branchCount)
	if err != nil {
		return err
	}
	if branchCount == 0 {
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN branch TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	// Check if type column exists and add if missing
	var typeCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('agents') WHERE name='type'`).Scan(&typeCount)
	if err != nil {
		return err
	}
	if typeCount == 0 {
		if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN type TEXT DEFAULT 'codex'`); err != nil {
			return err
		}
	}

	return nil
}

// CreateAgent inserts a new agent record
func (db *DB) CreateAgent(a Agent) error {
	agentType := a.Type
	if agentType == "" {
		agentType = "codex"
	}
	_, err := db.Exec(
		`INSERT INTO agents (name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type)
		 VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		a.Name, a.ULID, a.SessionFile, a.PID, time.Now().UTC().Format(time.RFC3339),
		a.RepoPath, a.Branch, agentType,
	)
	return err
}

// GetAgent retrieves an agent by name
func (db *DB) GetAgent(name string) (*Agent, error) {
	var a Agent
	var spawnedAt string
	err := db.QueryRow(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch, &a.Type)
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
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type
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
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch, &a.Type); err != nil {
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

// ListAgentsByRepo returns agents matching the given repo path.
func (db *DB) ListAgentsByRepo(repoPath string) ([]Agent, error) {
	rows, err := db.Query(
		`SELECT name, ulid, session_file, cursor, pid, spawned_at, repo_path, branch, type
		 FROM agents WHERE repo_path = ? ORDER BY spawned_at DESC`,
		repoPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var spawnedAt string
		if err := rows.Scan(&a.Name, &a.ULID, &a.SessionFile, &a.Cursor, &a.PID, &spawnedAt, &a.RepoPath, &a.Branch, &a.Type); err != nil {
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

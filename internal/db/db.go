package db

import (
	"database/sql"
	"errors"
	"log"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// schemaTablesSQL creates tables only - indexes created after migrations
const schemaTablesSQL = `-- agents table
CREATE TABLE IF NOT EXISTS agents (
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  task TEXT NOT NULL,
  status TEXT NOT NULL,
  session_id TEXT,
  pid INTEGER,
  compacted_at DATETIME,
  last_seen_message_id TEXT,
  peek_cursor TEXT,
  completed_at DATETIME,
  archived_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  id TEXT,  -- backwards compat alias for name
  PRIMARY KEY (project, branch, name)
);

-- messages table
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  from_agent TEXT NOT NULL,
  to_agent TEXT,
  type TEXT NOT NULL,
  content TEXT NOT NULL,
  mentions TEXT,
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  from_id TEXT  -- backwards compat alias for from_agent
);

-- logs table
CREATE TABLE IF NOT EXISTS logs (
  id TEXT PRIMARY KEY,
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  agent_name TEXT NOT NULL,
  agent_type TEXT NOT NULL,
  event_type TEXT NOT NULL,
  tool_name TEXT,
  content TEXT,
  raw_json TEXT,
  command TEXT,
  exit_code INTEGER,
  status TEXT,
  tool_use_id TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  agent_id TEXT,   -- backwards compat alias for agent_name
  direction TEXT   -- backwards compat (was 'in'/'out')
);

-- tasks table
CREATE TABLE IF NOT EXISTS tasks (
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  id TEXT NOT NULL,
  parent_id TEXT,
  name TEXT NOT NULL,
  sort_index INTEGER NOT NULL DEFAULT 0,
  assigned_agent TEXT,
  result TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (project, branch, id)
);
`

// schemaIndexesSQL creates indexes - must run AFTER migrations add columns
const schemaIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_logs_agent ON logs(agent_name, created_at);
CREATE INDEX IF NOT EXISTS idx_agents_cleanup ON agents(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agents_archived ON agents(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent, created_at);
`

func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	cleanupOldData(conn)
	return conn, nil
}

func ensureSchema(conn *sql.DB) error {
	// Step 1: Create tables (for new databases)
	if _, err := conn.Exec(schemaTablesSQL); err != nil {
		return err
	}

	// Step 2: Run migrations (for existing databases missing columns)
	// These use _, _ to ignore "duplicate column" errors
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN project TEXT`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN branch TEXT`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN name TEXT`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN compacted_at DATETIME`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN last_seen_message_id TEXT`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN peek_cursor TEXT`)
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN project TEXT`)
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN branch TEXT`)
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN from_agent TEXT`)
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN to_agent TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN project TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN branch TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN agent_name TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN agent_type TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN event_type TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN tool_name TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN raw_json TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN command TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN exit_code INTEGER`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN status TEXT`)
	_, _ = conn.Exec(`ALTER TABLE logs ADD COLUMN tool_use_id TEXT`)
	_, _ = conn.Exec(`ALTER TABLE transcript_entries RENAME TO logs`)

	// Step 2b: Backfill NULL values in migrated columns
	// Old data may have NULL project/branch - set defaults so scans work
	_, _ = conn.Exec(`UPDATE agents SET project = 'default' WHERE project IS NULL`)
	_, _ = conn.Exec(`UPDATE agents SET branch = 'main' WHERE branch IS NULL`)
	_, _ = conn.Exec(`UPDATE agents SET name = id WHERE name IS NULL AND id IS NOT NULL`)
	_, _ = conn.Exec(`UPDATE messages SET project = 'default' WHERE project IS NULL`)
	_, _ = conn.Exec(`UPDATE messages SET branch = 'main' WHERE branch IS NULL`)
	_, _ = conn.Exec(`UPDATE messages SET from_agent = from_id WHERE from_agent IS NULL AND from_id IS NOT NULL`)
	_, _ = conn.Exec(`UPDATE logs SET project = 'default' WHERE project IS NULL`)
	_, _ = conn.Exec(`UPDATE logs SET branch = 'main' WHERE branch IS NULL`)
	_, _ = conn.Exec(`UPDATE logs SET agent_name = agent_id WHERE agent_name IS NULL AND agent_id IS NOT NULL`)
	_, _ = conn.Exec(`UPDATE logs SET agent_type = 'unknown' WHERE agent_type IS NULL`)
	_, _ = conn.Exec(`UPDATE logs SET event_type = 'unknown' WHERE event_type IS NULL`)

	// Step 3: Create indexes (after migrations ensure columns exist)
	if _, err := conn.Exec(schemaIndexesSQL); err != nil {
		return err
	}
	return nil
}

func cleanupOldData(conn *sql.DB) {
	statements := []string{
		`DELETE FROM logs
		WHERE (project, branch, agent_name) IN (
			SELECT project, branch, name FROM agents
			WHERE archived_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM messages
		WHERE (project, branch, to_agent) IN (
			SELECT project, branch, name FROM agents
			WHERE archived_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM agents
		WHERE archived_at < datetime('now', '-7 days');`,
	}

	for _, stmt := range statements {
		if _, err := conn.Exec(stmt); err != nil {
			if isSQLiteBusy(err) {
				return
			}
			log.Printf("db cleanup: %v", err)
			return
		}
	}
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code() == sqlite3.SQLITE_BUSY
}

package repo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LogEntry struct {
	ID        string
	Project   string
	Branch    string
	AgentName string
	AgentType string
	EventType string
	ToolName  sql.NullString
	Content   sql.NullString
	RawJSON   sql.NullString
	Command   sql.NullString
	ExitCode  sql.NullInt64
	Status    sql.NullString
	ToolUseID sql.NullString
	CreatedAt string
}

func CreateLogEntry(db *sql.DB, entry LogEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	// Backwards compat: old schema has agent_id/direction/content as NOT NULL
	contentStr := ""
	if entry.Content.Valid {
		contentStr = entry.Content.String
	}
	_, err := db.Exec(
		`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, tool_name, content, raw_json, command, exit_code, status, tool_use_id, agent_id, direction) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.Project,
		entry.Branch,
		entry.AgentName,
		entry.AgentType,
		entry.EventType,
		entry.ToolName,
		contentStr, // content as string for old schema NOT NULL
		entry.RawJSON,
		entry.Command,
		entry.ExitCode,
		entry.Status,
		entry.ToolUseID,
		entry.AgentName, // backwards compat: agent_id = agent_name
		"out",           // backwards compat: direction = 'out'
	)
	return err
}

func ListLogs(db *sql.DB, project, branch, agentName, sinceID string) ([]LogEntry, error) {
	query := `SELECT id, project, branch, agent_name, agent_type, event_type, tool_name, content, raw_json, command, exit_code, status, tool_use_id, created_at FROM logs WHERE project = ? AND branch = ? AND agent_name = ?`
	args := []interface{}{project, branch, agentName}
	var sinceCreatedAt string
	var sinceRowID int64
	if sinceID != "" {
		if err := db.QueryRow(`SELECT strftime('%Y-%m-%d %H:%M:%S', created_at), rowid FROM logs WHERE id = ?`, sinceID).Scan(&sinceCreatedAt, &sinceRowID); err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
			sinceCreatedAt = ""
		}
	}
	if sinceCreatedAt != "" {
		query += " AND ((created_at = ? AND rowid > ?) OR created_at > ?)"
		args = append(args, sinceCreatedAt, sinceRowID, sinceCreatedAt)
	}
	query += " ORDER BY created_at ASC, rowid ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.Project, &entry.Branch, &entry.AgentName, &entry.AgentType, &entry.EventType, &entry.ToolName, &entry.Content, &entry.RawJSON, &entry.Command, &entry.ExitCode, &entry.Status, &entry.ToolUseID, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func ListLogsWithTail(db *sql.DB, project, branch, agentName string, n int) ([]LogEntry, error) {
	query := `SELECT id, project, branch, agent_name, agent_type, event_type, tool_name, content, raw_json, command, exit_code, status, tool_use_id, created_at
		FROM logs WHERE project = ? AND branch = ? AND agent_name = ?
		ORDER BY created_at DESC, rowid DESC LIMIT ?`

	rows, err := db.Query(query, project, branch, agentName, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.Project, &entry.Branch, &entry.AgentName, &entry.AgentType, &entry.EventType, &entry.ToolName, &entry.Content, &entry.RawJSON, &entry.Command, &entry.ExitCode, &entry.Status, &entry.ToolUseID, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func CountLogs(db *sql.DB, project, branch, agentName string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM logs WHERE project = ? AND branch = ? AND agent_name = ?`,
		project, branch, agentName).Scan(&count)
	return count, err
}

// ListAgentMessages returns agent_message log entries for a specific agent.
// These are the actual conversational responses from the agent.
// Used to surface june's responses to the main chat view.
func ListAgentMessages(db *sql.DB, project, branch, agentName, sinceID string) ([]LogEntry, error) {
	query := `SELECT id, project, branch, agent_name, agent_type, event_type, tool_name, content, raw_json, command, exit_code, status, tool_use_id, strftime('%Y-%m-%d %H:%M:%S', created_at)
		FROM logs
		WHERE project = ? AND branch = ? AND agent_name = ? AND event_type = 'agent_message'`
	args := []interface{}{project, branch, agentName}

	var sinceCreatedAt string
	var sinceRowID int64
	if sinceID != "" {
		if err := db.QueryRow(`SELECT strftime('%Y-%m-%d %H:%M:%S', created_at), rowid FROM logs WHERE id = ?`, sinceID).Scan(&sinceCreatedAt, &sinceRowID); err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
			sinceCreatedAt = ""
		}
	}
	if sinceCreatedAt != "" {
		query += " AND ((created_at = ? AND rowid > ?) OR created_at > ?)"
		args = append(args, sinceCreatedAt, sinceRowID, sinceCreatedAt)
	}
	query += " ORDER BY created_at ASC, rowid ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.Project, &entry.Branch, &entry.AgentName, &entry.AgentType, &entry.EventType, &entry.ToolName, &entry.Content, &entry.RawJSON, &entry.Command, &entry.ExitCode, &entry.Status, &entry.ToolUseID, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// GetAgentLastActivity returns the last activity time for each agent based on log entries.
// Returns a map of agent name -> last activity time.
func GetAgentLastActivity(db *sql.DB, project, branch string, agentNames []string) (map[string]time.Time, error) {
	result := make(map[string]time.Time)
	if len(agentNames) == 0 {
		return result, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(agentNames))
	args := make([]interface{}, 0, len(agentNames)+2)
	args = append(args, project, branch)
	for i, name := range agentNames {
		placeholders[i] = "?"
		args = append(args, name)
	}

	query := `SELECT agent_name, MAX(created_at) FROM logs WHERE project = ? AND branch = ? AND agent_name IN (` + strings.Join(placeholders, ",") + `) GROUP BY agent_name`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var agentName string
		var createdAt string
		if err := rows.Scan(&agentName, &createdAt); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			return nil, err
		}
		result[agentName] = t
	}
	return result, rows.Err()
}

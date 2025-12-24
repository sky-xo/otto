package repo

import (
	"database/sql"

	"github.com/google/uuid"
)

type LogEntry struct {
	ID        string
	AgentID   string
	Direction string
	Stream    sql.NullString
	Content   string
	CreatedAt string
}

func CreateLogEntry(db *sql.DB, agentID, direction, stream, content string) error {
	var streamValue sql.NullString
	if stream != "" {
		streamValue = sql.NullString{String: stream, Valid: true}
	}
	_, err := db.Exec(
		`INSERT INTO logs (id, agent_id, direction, stream, content) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(),
		agentID,
		direction,
		streamValue,
		content,
	)
	return err
}

func ListLogs(db *sql.DB, agentID, sinceID string) ([]LogEntry, error) {
	query := `SELECT id, agent_id, direction, stream, content, created_at FROM logs WHERE agent_id = ?`
	args := []interface{}{agentID}
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
		if err := rows.Scan(&entry.ID, &entry.AgentID, &entry.Direction, &entry.Stream, &entry.Content, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func ListLogsWithTail(db *sql.DB, agentID string, n int) ([]LogEntry, error) {
	query := `SELECT id, agent_id, direction, stream, content, created_at
		FROM logs WHERE agent_id = ?
		ORDER BY created_at DESC, rowid DESC LIMIT ?`

	rows, err := db.Query(query, agentID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(&entry.ID, &entry.AgentID, &entry.Direction, &entry.Stream, &entry.Content, &entry.CreatedAt); err != nil {
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

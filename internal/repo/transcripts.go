package repo

import (
	"database/sql"

	"github.com/google/uuid"
)

type TranscriptEntry struct {
	ID        string
	AgentID   string
	Direction string
	Stream    sql.NullString
	Content   string
	CreatedAt string
}

func CreateTranscriptEntry(db *sql.DB, agentID, direction, stream, content string) error {
	var streamValue sql.NullString
	if stream != "" {
		streamValue = sql.NullString{String: stream, Valid: true}
	}
	_, err := db.Exec(
		`INSERT INTO transcript_entries (id, agent_id, direction, stream, content) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(),
		agentID,
		direction,
		streamValue,
		content,
	)
	return err
}

func ListTranscriptEntries(db *sql.DB, agentID, sinceID string) ([]TranscriptEntry, error) {
	query := `SELECT id, agent_id, direction, stream, content, created_at FROM transcript_entries WHERE agent_id = ?`
	args := []interface{}{agentID}
	if sinceID != "" {
		query += " AND created_at > COALESCE((SELECT created_at FROM transcript_entries WHERE id = ?), '1970-01-01')"
		args = append(args, sinceID)
	}
	query += " ORDER BY created_at ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TranscriptEntry
	for rows.Next() {
		var entry TranscriptEntry
		if err := rows.Scan(&entry.ID, &entry.AgentID, &entry.Direction, &entry.Stream, &entry.Content, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

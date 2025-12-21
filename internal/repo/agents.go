package repo

import (
	"database/sql"
)

type Agent struct {
	ID        string
	Type      string
	Task      string
	Status    string
	SessionID sql.NullString
}

func CreateAgent(db *sql.DB, a Agent) error {
	_, err := db.Exec(`INSERT INTO agents (id, type, task, status, session_id) VALUES (?, ?, ?, ?, ?)`, a.ID, a.Type, a.Task, a.Status, a.SessionID)
	return err
}

func GetAgent(db *sql.DB, id string) (Agent, error) {
	var a Agent
	err := db.QueryRow(`SELECT id, type, task, status, session_id FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID)
	return a, err
}

func UpdateAgentStatus(db *sql.DB, id, status string) error {
	_, err := db.Exec(`UPDATE agents SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`SELECT id, type, task, status, session_id FROM agents ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

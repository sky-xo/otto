package repo

import (
	"database/sql"
)

type Agent struct {
	ID            string
	Type          string
	Task          string
	Status        string
	SessionID     sql.NullString
	Pid           sql.NullInt64
	CompletedAt   sql.NullTime
	ArchivedAt    sql.NullTime
	LastReadLogID sql.NullString
}

func CreateAgent(db *sql.DB, a Agent) error {
	_, err := db.Exec(`INSERT INTO agents (id, type, task, status, session_id, pid, completed_at, archived_at, last_read_log_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.ID, a.Type, a.Task, a.Status, a.SessionID, a.Pid, a.CompletedAt, a.ArchivedAt, a.LastReadLogID)
	return err
}

func GetAgent(db *sql.DB, id string) (Agent, error) {
	var a Agent
	err := db.QueryRow(`SELECT id, type, task, status, session_id, pid, completed_at, archived_at, last_read_log_id FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompletedAt, &a.ArchivedAt, &a.LastReadLogID)
	return a, err
}

func UpdateAgentStatus(db *sql.DB, id, status string) error {
	_, err := db.Exec(`UPDATE agents SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func UpdateAgentPid(db *sql.DB, id string, pid int) error {
	_, err := db.Exec(`UPDATE agents SET pid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, pid, id)
	return err
}

func UpdateAgentSessionID(db *sql.DB, id string, sessionID string) error {
	_, err := db.Exec(`UPDATE agents SET session_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID, id)
	return err
}

func UpdateAgentLastReadLogID(db *sql.DB, id, logID string) error {
	_, err := db.Exec(`UPDATE agents SET last_read_log_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, logID, id)
	return err
}

func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`SELECT id, type, task, status, session_id, pid, completed_at, archived_at, last_read_log_id FROM agents ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompletedAt, &a.ArchivedAt, &a.LastReadLogID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func ListAgentsFiltered(db *sql.DB, includeArchived bool) ([]Agent, error) {
	query := `SELECT id, type, task, status, session_id, pid, completed_at, archived_at, last_read_log_id FROM agents`
	if !includeArchived {
		query += " WHERE archived_at IS NULL"
	}
	query += " ORDER BY created_at ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompletedAt, &a.ArchivedAt, &a.LastReadLogID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func DeleteAgent(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}

func SetAgentComplete(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'complete', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func SetAgentFailed(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'failed', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func ResumeAgent(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'busy', completed_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func ArchiveAgent(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE agents SET archived_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func UnarchiveAgent(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE agents SET archived_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

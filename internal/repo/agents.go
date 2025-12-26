package repo

import (
	"database/sql"
)

type Agent struct {
	Project         string
	Branch          string
	Name            string
	Type            string
	Task            string
	Status          string
	SessionID       sql.NullString
	Pid             sql.NullInt64
	CompactedAt     sql.NullTime
	LastSeenMsgID   sql.NullString
	PeekCursor      sql.NullString
	CompletedAt     sql.NullTime
	ArchivedAt      sql.NullTime
}

type AgentFilter struct {
	Project         string
	Branch          string
	IncludeArchived bool
}

func CreateAgent(db *sql.DB, a Agent) error {
	// Also set id for backwards compatibility with old schema (id TEXT PRIMARY KEY)
	_, err := db.Exec(`INSERT INTO agents (project, branch, name, type, task, status, session_id, pid, compacted_at, last_seen_message_id, peek_cursor, completed_at, archived_at, id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Project, a.Branch, a.Name, a.Type, a.Task, a.Status, a.SessionID, a.Pid, a.CompactedAt, a.LastSeenMsgID, a.PeekCursor, a.CompletedAt, a.ArchivedAt, a.Name)
	return err
}

func GetAgent(db *sql.DB, project, branch, name string) (Agent, error) {
	var a Agent
	err := db.QueryRow(`SELECT project, branch, name, type, task, status, session_id, pid, compacted_at, last_seen_message_id, peek_cursor, completed_at, archived_at FROM agents WHERE project = ? AND branch = ? AND name = ?`, project, branch, name).
		Scan(&a.Project, &a.Branch, &a.Name, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompactedAt, &a.LastSeenMsgID, &a.PeekCursor, &a.CompletedAt, &a.ArchivedAt)
	return a, err
}

func UpdateAgentStatus(db *sql.DB, project, branch, name, status string) error {
	_, err := db.Exec(`UPDATE agents SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, status, project, branch, name)
	return err
}

func UpdateAgentPid(db *sql.DB, project, branch, name string, pid int) error {
	_, err := db.Exec(`UPDATE agents SET pid = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, pid, project, branch, name)
	return err
}

func UpdateAgentSessionID(db *sql.DB, project, branch, name string, sessionID string) error {
	_, err := db.Exec(`UPDATE agents SET session_id = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, sessionID, project, branch, name)
	return err
}

func UpdateAgentLastSeenMsgID(db *sql.DB, project, branch, name, msgID string) error {
	_, err := db.Exec(`UPDATE agents SET last_seen_message_id = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, msgID, project, branch, name)
	return err
}

func UpdateAgentPeekCursor(db *sql.DB, project, branch, name, logID string) error {
	_, err := db.Exec(`UPDATE agents SET peek_cursor = ?, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, logID, project, branch, name)
	return err
}

func ListAgents(db *sql.DB, f AgentFilter) ([]Agent, error) {
	query := `SELECT project, branch, name, type, task, status, session_id, pid, compacted_at, last_seen_message_id, peek_cursor, completed_at, archived_at FROM agents`
	var args []interface{}
	var conditions []string

	if f.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, f.Project)
	}
	if f.Branch != "" {
		conditions = append(conditions, "branch = ?")
		args = append(args, f.Branch)
	}
	if !f.IncludeArchived {
		conditions = append(conditions, "archived_at IS NULL")
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}
	query += " ORDER BY created_at ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.Project, &a.Branch, &a.Name, &a.Type, &a.Task, &a.Status, &a.SessionID, &a.Pid, &a.CompactedAt, &a.LastSeenMsgID, &a.PeekCursor, &a.CompletedAt, &a.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func DeleteAgent(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`DELETE FROM agents WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func SetAgentComplete(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'complete', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func SetAgentFailed(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'failed', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func ResumeAgent(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET status = 'busy', completed_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func ArchiveAgent(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET archived_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func UnarchiveAgent(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET archived_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

func MarkAgentCompacted(db *sql.DB, project, branch, name string) error {
	_, err := db.Exec(`UPDATE agents SET compacted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE project = ? AND branch = ? AND name = ?`, project, branch, name)
	return err
}

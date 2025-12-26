package repo

import "database/sql"

type Task struct {
	Project       string
	Branch        string
	ID            string
	ParentID      sql.NullString
	Name          string
	SortIndex     int
	AssignedAgent sql.NullString
	Result        sql.NullString
}

func CreateTask(db *sql.DB, task Task) error {
	_, err := db.Exec(`INSERT INTO tasks (project, branch, id, parent_id, name, sort_index, assigned_agent, result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Project, task.Branch, task.ID, task.ParentID, task.Name, task.SortIndex, task.AssignedAgent, task.Result)
	return err
}

func ListTasks(db *sql.DB, project, branch string) ([]Task, error) {
	rows, err := db.Query(`SELECT project, branch, id, parent_id, name, sort_index, assigned_agent, result
		FROM tasks WHERE project = ? AND branch = ? ORDER BY sort_index ASC`, project, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.Project, &task.Branch, &task.ID, &task.ParentID, &task.Name, &task.SortIndex, &task.AssignedAgent, &task.Result); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

package repo

import (
	"database/sql"
	"testing"
)

func TestCreateAndListTasks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	taskOne := Task{
		Project:   "proj",
		Branch:    "main",
		ID:        "t1",
		Name:      "first task",
		SortIndex: 2,
	}
	taskTwo := Task{
		Project:       "proj",
		Branch:        "main",
		ID:            "t2",
		Name:          "second task",
		SortIndex:     1,
		AssignedAgent: sql.NullString{String: "agent-1", Valid: true},
		Result:        sql.NullString{String: "done", Valid: true},
	}

	if err := CreateTask(db, taskOne); err != nil {
		t.Fatalf("create task one: %v", err)
	}
	if err := CreateTask(db, taskTwo); err != nil {
		t.Fatalf("create task two: %v", err)
	}

	tasks, err := ListTasks(db, "proj", "main")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks len = %d", len(tasks))
	}
	if tasks[0].ID != "t2" || tasks[1].ID != "t1" {
		t.Fatalf("unexpected task order: %#v", tasks)
	}
	if !tasks[0].AssignedAgent.Valid || tasks[0].AssignedAgent.String != "agent-1" {
		t.Fatalf("assigned_agent not persisted: %#v", tasks[0])
	}
	if !tasks[0].Result.Valid || tasks[0].Result.String != "done" {
		t.Fatalf("result not persisted: %#v", tasks[0])
	}
}

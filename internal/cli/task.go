package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/scope"
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage persistent tasks",
		Long:  "Create, list, update, and delete tasks that persist across context compaction.",
	}

	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskUpdateCmd())
	cmd.AddCommand(newTaskDeleteCmd())

	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	var parentID string
	var note string
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "create <title> [titles...]",
		Short: "Create one or more tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskCreate(cmd, args, parentID, note, outputJSON)
		},
	}

	cmd.Flags().StringVar(&parentID, "parent", "", "Parent task ID for creating child tasks")
	cmd.Flags().StringVar(&note, "note", "", "Set note on created task(s)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	return cmd
}

func newTaskListCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "list [task-id]",
		Short: "List tasks or show task details",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskList(cmd, args, outputJSON)
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	return cmd
}

func newTaskUpdateCmd() *cobra.Command {
	var status, note, title string

	cmd := &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskUpdate(cmd, args, status, note, title)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Set status (open, in_progress, closed)")
	cmd.Flags().StringVar(&note, "note", "", "Set note")
	cmd.Flags().StringVar(&title, "title", "", "Set title")

	return cmd
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task and its children",
		Args:  cobra.ExactArgs(1),
		RunE:  runTaskDelete,
	}
}

func runTaskCreate(cmd *cobra.Command, args []string, parentID, note string, outputJSON bool) error {
	// Get git scope
	repoPath := scope.RepoRoot()
	if repoPath == "" {
		return fmt.Errorf("not in a git repository")
	}
	branch := scope.BranchName()
	if branch == "" {
		branch = "main" // fallback
	}

	// Open database
	database, err := openTaskDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Validate parent exists if specified
	var parentPtr *string
	if parentID != "" {
		parentTask, err := database.GetTask(parentID)
		if err != nil {
			return fmt.Errorf("parent task %q not found", parentID)
		}
		// Check parent is not deleted
		if parentTask.DeletedAt != nil {
			return fmt.Errorf("parent task %q is deleted", parentID)
		}
		// Check parent is in same scope
		if parentTask.RepoPath != repoPath || parentTask.Branch != branch {
			return fmt.Errorf("parent task %q is in a different scope", parentID)
		}
		parentPtr = &parentID
	}

	// Create tasks
	now := time.Now()
	out := cmd.OutOrStdout()

	for _, title := range args {
		id, err := generateUniqueTaskID(database)
		if err != nil {
			return fmt.Errorf("generate task ID: %w", err)
		}

		var notePtr *string
		if note != "" {
			notePtr = &note
		}

		task := db.Task{
			ID:        id,
			ParentID:  parentPtr,
			Title:     title,
			Status:    "open",
			Notes:     notePtr,
			RepoPath:  repoPath,
			Branch:    branch,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := database.CreateTask(task); err != nil {
			return fmt.Errorf("create task: %w", err)
		}

		if outputJSON {
			fmt.Fprintf(out, `{"id":"%s"}`+"\n", id)
		} else {
			fmt.Fprintln(out, id)
		}
	}

	return nil
}

func runTaskList(cmd *cobra.Command, args []string, outputJSON bool) error {
	// Get git scope
	repoPath := scope.RepoRoot()
	if repoPath == "" {
		return fmt.Errorf("not in a git repository")
	}
	branch := scope.BranchName()
	if branch == "" {
		branch = "main"
	}

	// Open database
	database, err := openTaskDB()
	if err != nil {
		return err
	}
	defer database.Close()

	out := cmd.OutOrStdout()

	// If task ID provided, show that task + its children
	if len(args) == 1 {
		taskID := args[0]
		return listSpecificTask(database, taskID, out, outputJSON)
	}

	// Otherwise list root tasks
	return listRootTasks(database, repoPath, branch, out, outputJSON)
}

func listRootTasks(database *db.DB, repoPath, branch string, out io.Writer, asJSON bool) error {
	tasks, err := database.ListRootTasks(repoPath, branch)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if asJSON {
		type taskOutput struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Status   string `json:"status"`
			Children int    `json:"children"`
		}
		output := make([]taskOutput, len(tasks))
		for i, t := range tasks {
			childCount, err := database.CountChildren(t.ID)
			if err != nil {
				return fmt.Errorf("count children for %s: %w", t.ID, err)
			}
			output[i] = taskOutput{
				ID:       t.ID,
				Title:    t.Title,
				Status:   t.Status,
				Children: childCount,
			}
		}
		enc := json.NewEncoder(out)
		return enc.Encode(output)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, "No tasks.")
		return nil
	}

	for _, t := range tasks {
		childCount, err := database.CountChildren(t.ID)
		if err != nil {
			return fmt.Errorf("count children for %s: %w", t.ID, err)
		}
		childSuffix := ""
		if childCount > 0 {
			childSuffix = fmt.Sprintf("  (%d children)", childCount)
		}
		fmt.Fprintf(out, "%s  %s  [%s]%s\n", t.ID, t.Title, t.Status, childSuffix)
	}

	return nil
}

func listSpecificTask(database *db.DB, taskID string, out io.Writer, asJSON bool) error {
	task, err := database.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	// Don't show deleted tasks
	if task.DeletedAt != nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	children, err := database.ListChildTasks(taskID)
	if err != nil {
		return fmt.Errorf("list children: %w", err)
	}

	if asJSON {
		type childOutput struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}
		type taskOutput struct {
			ID       string        `json:"id"`
			ParentID *string       `json:"parent_id,omitempty"`
			Title    string        `json:"title"`
			Status   string        `json:"status"`
			Notes    *string       `json:"notes,omitempty"`
			Children []childOutput `json:"children"`
		}

		output := taskOutput{
			ID:       task.ID,
			ParentID: task.ParentID,
			Title:    task.Title,
			Status:   task.Status,
			Notes:    task.Notes,
			Children: make([]childOutput, len(children)),
		}
		for i, c := range children {
			output.Children[i] = childOutput{ID: c.ID, Title: c.Title, Status: c.Status}
		}

		enc := json.NewEncoder(out)
		return enc.Encode(output)
	}

	// Human-readable output
	fmt.Fprintf(out, "%s %q [%s]\n", task.ID, task.Title, task.Status)

	if task.ParentID != nil {
		fmt.Fprintf(out, "  Parent: %s\n", *task.ParentID)
	}

	if task.Notes != nil && *task.Notes != "" {
		fmt.Fprintf(out, "  Note: %s\n", *task.Notes)
	}

	fmt.Fprintln(out)
	if len(children) == 0 {
		fmt.Fprintln(out, "No children.")
	} else {
		fmt.Fprintln(out, "Children:")
		for _, c := range children {
			fmt.Fprintf(out, "  %s  %s  [%s]\n", c.ID, c.Title, c.Status)
		}
	}

	return nil
}

func runTaskUpdate(cmd *cobra.Command, args []string, status, note, title string) error {
	taskID := args[0]

	// Build update struct
	update := db.TaskUpdate{}
	if status != "" {
		// Validate status
		valid := map[string]bool{"open": true, "in_progress": true, "closed": true}
		if !valid[status] {
			return fmt.Errorf("invalid status %q (use: open, in_progress, closed)", status)
		}
		update.Status = &status
	}
	if note != "" {
		update.Notes = &note
	}
	if title != "" {
		update.Title = &title
	}

	// Check if any update provided
	if update.Status == nil && update.Notes == nil && update.Title == nil {
		return fmt.Errorf("no update provided (use --status, --note, or --title)")
	}

	// Open database
	database, err := openTaskDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Perform update
	err = database.UpdateTask(taskID, update)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("update task: %w", err)
	}

	return nil
}

func runTaskDelete(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Open database
	database, err := openTaskDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Delete task (soft delete with cascade)
	err = database.DeleteTask(taskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}

// openTaskDB opens the june database for task operations
func openTaskDB() (*db.DB, error) {
	home, err := juneHome()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return database, nil
}

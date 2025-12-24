package commands

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"strings"
	"testing"

	"otto/internal/repo"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	_ = r.Close()

	return buf.String()
}

func TestAttachCodexShowsDangerFullAccess(t *testing.T) {
	db := openTestDB(t)

	agent := repo.Agent{
		ID:        "codexer",
		Type:      "codex",
		Task:      "task",
		Status:    "complete",
		SessionID: sql.NullString{String: "thread-1", Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	output := captureStdout(t, func() {
		if err := runAttach(db, "codexer"); err != nil {
			t.Fatalf("runAttach: %v", err)
		}
	})

	expected := "codex exec --skip-git-repo-check -s danger-full-access resume thread-1"
	if !strings.Contains(output, expected) {
		t.Fatalf("expected %q in output, got %q", expected, output)
	}
}

func TestAttachUnarchivesAgent(t *testing.T) {
	db := openTestDB(t)

	agent := repo.Agent{
		ID:        "archived",
		Type:      "claude",
		Task:      "task",
		Status:    "complete",
		SessionID: sql.NullString{String: "session-2", Valid: true},
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := repo.ArchiveAgent(db, agent.ID); err != nil {
		t.Fatalf("archive agent: %v", err)
	}

	_ = captureStdout(t, func() {
		if err := runAttach(db, "archived"); err != nil {
			t.Fatalf("runAttach: %v", err)
		}
	})

	updated, err := repo.GetAgent(db, "archived")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if updated.ArchivedAt.Valid {
		t.Fatal("expected archived_at to be cleared")
	}
}

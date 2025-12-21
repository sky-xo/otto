package db

import (
	"path/filepath"
	"testing"
)

func TestEnsureSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otto.db")

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='agents'"); err != nil {
		t.Fatalf("query agents: %v", err)
	}

	if _, err := conn.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='messages'"); err != nil {
		t.Fatalf("query messages: %v", err)
	}
}

package commands

import (
	"testing"

	"june/internal/scope"
)

func TestParseMessagesFlags(t *testing.T) {
	ctx := scope.Context{Project: "test-project", Branch: "main"}
	f := parseMessagesFlags(ctx, "authbackend", "question", 10, "authbackend")
	if f.FromAgent != "authbackend" || f.Type != "question" || f.Limit != 10 || f.ReaderID != "authbackend" {
		t.Fatalf("unexpected filter: %#v", f)
	}
}

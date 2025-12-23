package repo

import (
	"database/sql"
	"encoding/json"
	"testing"
)

// Helper to create sql.NullString for ToID in tests
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func TestCreateMessage(t *testing.T) {
	db := openTestDB(t)

	msg := Message{
		ID:            "m1",
		FromID:        "agent-1",
		ToID:          nullStr("agent-2"),
		Type:          "say",
		Content:       "hello",
		MentionsJSON:  `["agent-2"]`,
		RequiresHuman: true,
		ReadByJSON:    `[]`,
	}

	if err := CreateMessage(db, msg); err != nil {
		t.Fatalf("create: %v", err)
	}

	msgs, err := ListMessages(db, MessageFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "m1" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}

	if err := CreateMessage(db, msg); err == nil {
		t.Fatal("expected duplicate insert error")
	}
}

func TestListMessagesFilters(t *testing.T) {
	db := openTestDB(t)

	messages := []Message{
		{ID: "m1", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "hello", MentionsJSON: `["agent-1","agent-2"]`, ReadByJSON: `[]`},
		{ID: "m2", FromID: "agent-1", ToID: nullStr("agent-3"), Type: "question", Content: "help", MentionsJSON: `["agent-2"]`, ReadByJSON: `["reader-1"]`},
		{ID: "m3", FromID: "agent-2", ToID: nullStr(""), Type: "say", Content: "later", MentionsJSON: `["agent-3"]`, ReadByJSON: `[]`},
		{ID: "m4", FromID: "agent-2", ToID: nullStr("agent-3"), Type: "say", Content: "bad json", MentionsJSON: `not-json`, ReadByJSON: `[]`},
	}

	for _, msg := range messages {
		if err := CreateMessage(db, msg); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	t.Run("filter by type", func(t *testing.T) {
		msgs, err := ListMessages(db, MessageFilter{Type: "question"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(msgs) != 1 || msgs[0].ID != "m2" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})

	t.Run("filter by from_id", func(t *testing.T) {
		msgs, err := ListMessages(db, MessageFilter{FromID: "agent-2"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(msgs) != 2 || msgs[0].FromID != "agent-2" || msgs[1].FromID != "agent-2" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})

	t.Run("filter by mention", func(t *testing.T) {
		msgs, err := ListMessages(db, MessageFilter{Mention: "agent-1"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(msgs) != 1 || msgs[0].ID != "m1" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})
	t.Run("filter by to_id", func(t *testing.T) {
		msgs, err := ListMessages(db, MessageFilter{ToID: "agent-3"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(msgs) != 2 || msgs[0].ToID.String != "agent-3" || msgs[1].ToID.String != "agent-3" {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
	})

	t.Run("filter unread by reader", func(t *testing.T) {
		msgs, err := ListMessages(db, MessageFilter{ReaderID: "reader-1"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(msgs) != 3 {
			t.Fatalf("unexpected messages: %#v", msgs)
		}
		for _, msg := range msgs {
			if msg.ID == "m2" {
				t.Fatalf("unexpected read message: %#v", msg)
			}
		}
	})
}

func TestListMessagesSinceAndLimit(t *testing.T) {
	db := openTestDB(t)

	messages := []Message{
		{ID: "m1", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "one", MentionsJSON: `[]`, ReadByJSON: `[]`},
		{ID: "m2", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "two", MentionsJSON: `[]`, ReadByJSON: `[]`},
		{ID: "m3", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "three", MentionsJSON: `[]`, ReadByJSON: `[]`},
	}

	for _, msg := range messages {
		if err := CreateMessage(db, msg); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	timestamps := map[string]string{
		"m1": "2024-01-01 00:00:00",
		"m2": "2024-01-02 00:00:00",
		"m3": "2024-01-03 00:00:00",
	}
	for id, ts := range timestamps {
		if _, err := db.Exec(`UPDATE messages SET created_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatalf("set created_at: %v", err)
		}
	}

	msgs, err := ListMessages(db, MessageFilter{SinceID: "m1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 2 || msgs[0].ID != "m2" || msgs[1].ID != "m3" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}

	msgs, err = ListMessages(db, MessageFilter{SinceID: "missing"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("unexpected messages: %#v", msgs)
	}

	msgs, err = ListMessages(db, MessageFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 2 || msgs[0].ID != "m1" || msgs[1].ID != "m2" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}
}

func TestListMessagesSinceSameSecond(t *testing.T) {
	db := openTestDB(t)

	messages := []Message{
		{ID: "m1", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "one", MentionsJSON: `[]`, ReadByJSON: `[]`},
		{ID: "m2", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "two", MentionsJSON: `[]`, ReadByJSON: `[]`},
		{ID: "m3", FromID: "agent-1", ToID: nullStr("agent-2"), Type: "say", Content: "three", MentionsJSON: `[]`, ReadByJSON: `[]`},
	}

	for _, msg := range messages {
		if err := CreateMessage(db, msg); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if _, err := db.Exec(`UPDATE messages SET created_at = ? WHERE id IN (?, ?)`, "2024-01-01 00:00:00", "m1", "m2"); err != nil {
		t.Fatalf("set created_at: %v", err)
	}
	if _, err := db.Exec(`UPDATE messages SET created_at = ? WHERE id = ?`, "2024-01-01 00:00:01", "m3"); err != nil {
		t.Fatalf("set created_at: %v", err)
	}

	msgs, err := ListMessages(db, MessageFilter{SinceID: "m1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 2 || msgs[0].ID != "m2" || msgs[1].ID != "m3" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}
}

func TestMarkMessagesRead(t *testing.T) {
	db := openTestDB(t)

	if err := CreateMessage(db, Message{ID: "m1", FromID: "agent-1", Type: "say", Content: "hello", MentionsJSON: `[]`, ReadByJSON: `[]`}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := CreateMessage(db, Message{ID: "m2", FromID: "agent-1", Type: "say", Content: "again", MentionsJSON: `[]`, ReadByJSON: `["reader-1"]`}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := MarkMessagesRead(db, []string{"m1", "m2"}, "reader-2"); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	msgs, err := ListMessages(db, MessageFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	readersByID := map[string][]string{}
	for _, msg := range msgs {
		var readers []string
		if err := json.Unmarshal([]byte(msg.ReadByJSON), &readers); err != nil {
			t.Fatalf("unmarshal read_by: %v", err)
		}
		readersByID[msg.ID] = readers
	}

	if !containsString(readersByID["m1"], "reader-2") {
		t.Fatalf("expected reader-2 in m1: %#v", readersByID["m1"])
	}
	if !containsString(readersByID["m2"], "reader-2") || !containsString(readersByID["m2"], "reader-1") {
		t.Fatalf("unexpected readers for m2: %#v", readersByID["m2"])
	}

	if err := MarkMessagesRead(db, []string{"m1", "m2"}, "reader-2"); err != nil {
		t.Fatalf("mark read again: %v", err)
	}

	var readByJSON string
	if err := db.QueryRow(`SELECT read_by FROM messages WHERE id = ?`, "m2").Scan(&readByJSON); err != nil {
		t.Fatalf("scan read_by: %v", err)
	}
	var readers []string
	if err := json.Unmarshal([]byte(readByJSON), &readers); err != nil {
		t.Fatalf("unmarshal read_by: %v", err)
	}
	if countString(readers, "reader-2") != 1 {
		t.Fatalf("unexpected reader-2 count: %#v", readers)
	}

	if err := db.QueryRow(`SELECT read_by FROM messages WHERE id = ?`, "m1").Scan(&readByJSON); err != nil {
		t.Fatalf("scan read_by: %v", err)
	}
	readers = nil
	if err := json.Unmarshal([]byte(readByJSON), &readers); err != nil {
		t.Fatalf("unmarshal read_by: %v", err)
	}
	if countString(readers, "reader-2") != 1 {
		t.Fatalf("unexpected reader-2 count for m1: %#v", readers)
	}
}

func TestMarkMessagesReadInvalidJSON(t *testing.T) {
	db := openTestDB(t)

	if err := CreateMessage(db, Message{ID: "m1", FromID: "agent-1", Type: "say", Content: "hello", MentionsJSON: `[]`, ReadByJSON: `not-json`}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := MarkMessagesRead(db, []string{"m1"}, "reader-1"); err == nil {
		t.Fatal("expected error for invalid read_by json")
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func countString(items []string, target string) int {
	count := 0
	for _, item := range items {
		if item == target {
			count++
		}
	}
	return count
}

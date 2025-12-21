package commands

import "testing"

func TestWatchSinceID(t *testing.T) {
	next := nextSince("m10")
	if next != "m10" {
		t.Fatalf("unexpected next: %s", next)
	}
}

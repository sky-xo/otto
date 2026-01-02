// internal/tui/cell_test.go
package tui

import "testing"

func TestStyledLineString(t *testing.T) {
	line := StyledLine{
		{Char: 'h', Style: CellStyle{}},
		{Char: 'i', Style: CellStyle{}},
	}
	if got := line.String(); got != "hi" {
		t.Errorf("String() = %q, want %q", got, "hi")
	}
}

func TestStyledLineLen(t *testing.T) {
	line := StyledLine{
		{Char: 'a', Style: CellStyle{}},
		{Char: 'b', Style: CellStyle{}},
		{Char: 'c', Style: CellStyle{}},
	}
	if got := len(line); got != 3 {
		t.Errorf("len() = %d, want 3", got)
	}
}

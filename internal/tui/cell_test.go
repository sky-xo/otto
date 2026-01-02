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

func TestParseStyledLinePlainText(t *testing.T) {
	line := ParseStyledLine("hello")
	if got := line.String(); got != "hello" {
		t.Errorf("String() = %q, want %q", got, "hello")
	}
	if len(line) != 5 {
		t.Errorf("len = %d, want 5", len(line))
	}
}

func TestParseStyledLineEmpty(t *testing.T) {
	line := ParseStyledLine("")
	if len(line) != 0 {
		t.Errorf("len = %d, want 0", len(line))
	}
}

func TestParseStyledLineUnicode(t *testing.T) {
	line := ParseStyledLine("hello 世界")
	if got := line.String(); got != "hello 世界" {
		t.Errorf("String() = %q, want %q", got, "hello 世界")
	}
}

func TestStyledLineWithSelection(t *testing.T) {
	line := StyledLine{
		{Char: 'h', Style: CellStyle{}},
		{Char: 'e', Style: CellStyle{}},
		{Char: 'l', Style: CellStyle{}},
		{Char: 'l', Style: CellStyle{}},
		{Char: 'o', Style: CellStyle{}},
		{Char: ' ', Style: CellStyle{}},
		{Char: 'w', Style: CellStyle{}},
		{Char: 'o', Style: CellStyle{}},
		{Char: 'r', Style: CellStyle{}},
		{Char: 'l', Style: CellStyle{}},
		{Char: 'd', Style: CellStyle{}},
	}
	highlight := Color{Type: Color256, Value: 238}

	selected := line.WithSelection(0, 5, highlight) // "hello"

	// First 5 chars should have highlight BG
	for i := 0; i < 5; i++ {
		if selected[i].Style.BG != highlight {
			t.Errorf("char %d should be highlighted", i)
		}
	}
	// Rest should not
	for i := 5; i < len(selected); i++ {
		if selected[i].Style.BG == highlight {
			t.Errorf("char %d should not be highlighted", i)
		}
	}
}

func TestStyledLineWithSelectionPreservesExistingStyle(t *testing.T) {
	redFG := Color{Type: ColorBasic, Value: 1}
	line := StyledLine{
		{Char: 'r', Style: CellStyle{FG: redFG}},
		{Char: 'e', Style: CellStyle{FG: redFG}},
		{Char: 'd', Style: CellStyle{FG: redFG}},
	}
	highlight := Color{Type: Color256, Value: 238}

	selected := line.WithSelection(0, 3, highlight)

	// Should have both red FG and highlight BG
	if selected[0].Style.FG != redFG {
		t.Error("should preserve red foreground")
	}
	if selected[0].Style.BG != highlight {
		t.Error("should have highlight background")
	}
}

func TestStyledLineWithSelectionPartial(t *testing.T) {
	line := StyledLine{
		{Char: 'h', Style: CellStyle{}},
		{Char: 'e', Style: CellStyle{}},
		{Char: 'l', Style: CellStyle{}},
		{Char: 'l', Style: CellStyle{}},
		{Char: 'o', Style: CellStyle{}},
	}
	highlight := Color{Type: Color256, Value: 238}

	// Select middle: "ell"
	selected := line.WithSelection(1, 4, highlight)

	if selected[0].Style.BG.Type != ColorNone {
		t.Error("'h' should not be highlighted")
	}
	if selected[1].Style.BG != highlight {
		t.Error("'e' should be highlighted")
	}
	if selected[4].Style.BG.Type != ColorNone {
		t.Error("'o' should not be highlighted")
	}
}

func TestStyledLineWithSelectionOutOfBounds(t *testing.T) {
	line := StyledLine{
		{Char: 'h', Style: CellStyle{}},
		{Char: 'i', Style: CellStyle{}},
	}
	highlight := Color{Type: Color256, Value: 238}

	// Should handle out of bounds gracefully
	selected := line.WithSelection(-1, 100, highlight)
	if len(selected) != 2 {
		t.Errorf("len = %d, want 2", len(selected))
	}
	// Both should be highlighted (clamped to 0-2)
	for i := range selected {
		if selected[i].Style.BG != highlight {
			t.Errorf("char %d should be highlighted", i)
		}
	}
}

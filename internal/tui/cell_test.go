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

func TestParseStyledLineBasicColors(t *testing.T) {
	// Red foreground: ESC[31m
	line := ParseStyledLine("\x1b[31mred\x1b[0m")
	if line[0].Style.FG.Type != ColorBasic || line[0].Style.FG.Value != 1 {
		t.Errorf("expected red FG, got %+v", line[0].Style.FG)
	}
	if got := line.String(); got != "red" {
		t.Errorf("String() = %q, want %q", got, "red")
	}
}

func TestParseStyledLine256Colors(t *testing.T) {
	// 256-color foreground: ESC[38;5;196m (bright red)
	line := ParseStyledLine("\x1b[38;5;196mtext\x1b[0m")
	if line[0].Style.FG.Type != Color256 || line[0].Style.FG.Value != 196 {
		t.Errorf("expected 256-color 196, got %+v", line[0].Style.FG)
	}
}

func TestParseStyledLineTrueColor(t *testing.T) {
	// Truecolor foreground: ESC[38;2;255;128;0m (orange)
	line := ParseStyledLine("\x1b[38;2;255;128;0mtext\x1b[0m")
	if line[0].Style.FG.Type != ColorTrueColor {
		t.Errorf("expected truecolor, got %v", line[0].Style.FG.Type)
	}
	expected := uint32(0xFF8000)
	if line[0].Style.FG.Value != expected {
		t.Errorf("expected 0x%06X, got 0x%06X", expected, line[0].Style.FG.Value)
	}
}

func TestParseStyledLineBold(t *testing.T) {
	line := ParseStyledLine("\x1b[1mbold\x1b[0m")
	if !line[0].Style.Bold {
		t.Error("expected bold")
	}
}

func TestParseStyledLineReset(t *testing.T) {
	line := ParseStyledLine("\x1b[31mred\x1b[0mnormal")
	// 'n' of 'normal' should have no color
	nIdx := 3 // r=0, e=1, d=2, n=3
	if line[nIdx].Style.FG.Type != ColorNone {
		t.Errorf("expected reset, got %+v", line[nIdx].Style.FG)
	}
}

func TestParseStyledLineBackground(t *testing.T) {
	// Background: ESC[48;5;238m
	line := ParseStyledLine("\x1b[48;5;238mtext\x1b[0m")
	if line[0].Style.BG.Type != Color256 || line[0].Style.BG.Value != 238 {
		t.Errorf("expected 256-color BG 238, got %+v", line[0].Style.BG)
	}
}

func TestParseStyledLineItalic(t *testing.T) {
	line := ParseStyledLine("\x1b[3mitalic\x1b[0m")
	if !line[0].Style.Italic {
		t.Error("expected italic")
	}
}

func TestParseStyledLineCombined(t *testing.T) {
	// Bold red: ESC[1;31m
	line := ParseStyledLine("\x1b[1;31mboldred\x1b[0m")
	if !line[0].Style.Bold {
		t.Error("expected bold")
	}
	if line[0].Style.FG.Type != ColorBasic || line[0].Style.FG.Value != 1 {
		t.Errorf("expected red FG, got %+v", line[0].Style.FG)
	}
}

func TestParseStyledLineBoldOff(t *testing.T) {
	// Bold on, then bold off (not full reset)
	line := ParseStyledLine("\x1b[1;31mbold\x1b[22mnotbold\x1b[0m")
	// First char should be bold + red
	if !line[0].Style.Bold {
		t.Error("expected bold at start")
	}
	// After ESC[22m, bold should be off but red should remain
	notBoldIdx := 4 // 'n' of "notbold"
	if line[notBoldIdx].Style.Bold {
		t.Error("bold should be off after ESC[22m")
	}
	if line[notBoldIdx].Style.FG.Type != ColorBasic || line[notBoldIdx].Style.FG.Value != 1 {
		t.Errorf("should still be red, got %+v", line[notBoldIdx].Style.FG)
	}
}

func TestStyledLineRender(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"plain", "hello"},
		{"basic color", "\x1b[31mred\x1b[0m"},
		{"256 color", "\x1b[38;5;196mtext\x1b[0m"},
		{"bold", "\x1b[1mbold\x1b[0m normal"},
		{"background", "\x1b[48;5;238mhighlighted\x1b[0m"},
		{"truecolor", "\x1b[38;2;255;128;0mtext\x1b[0m"},
		{"italic", "\x1b[3mitalic\x1b[0m"},
		{"combined", "\x1b[1;3;38;5;196mstyle\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := ParseStyledLine(tt.input)
			rendered := line.Render()
			// Parse the rendered output and compare strings
			reparsed := ParseStyledLine(rendered)
			if line.String() != reparsed.String() {
				t.Errorf("text mismatch: %q vs %q", line.String(), reparsed.String())
			}
			// Verify styles match
			for i := range line {
				if i < len(reparsed) && line[i].Style != reparsed[i].Style {
					t.Errorf("style mismatch at %d: %+v vs %+v", i, line[i].Style, reparsed[i].Style)
				}
			}
		})
	}
}

func TestStyledLineRenderEmpty(t *testing.T) {
	line := StyledLine{}
	if got := line.Render(); got != "" {
		t.Errorf("Render() = %q, want empty", got)
	}
}

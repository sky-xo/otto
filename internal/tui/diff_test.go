// internal/tui/diff_test.go
package tui

import (
	"strings"
	"testing"
)

func TestComputeDiff_IdenticalContent(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	diff := computeDiff(lines, lines)

	// All lines should be equal
	for i, d := range diff {
		if d.Op != DiffEqual {
			t.Errorf("Line %d: expected DiffEqual, got %v", i, d.Op)
		}
		if d.Content != lines[i] {
			t.Errorf("Line %d: expected %q, got %q", i, lines[i], d.Content)
		}
	}
}

func TestComputeDiff_AllDeleted(t *testing.T) {
	oldLines := []string{"line1", "line2", "line3"}
	newLines := []string{}
	diff := computeDiff(oldLines, newLines)

	if len(diff) != 3 {
		t.Fatalf("Expected 3 diff entries, got %d", len(diff))
	}

	for i, d := range diff {
		if d.Op != DiffDelete {
			t.Errorf("Line %d: expected DiffDelete, got %v", i, d.Op)
		}
	}
}

func TestComputeDiff_AllInserted(t *testing.T) {
	oldLines := []string{}
	newLines := []string{"line1", "line2", "line3"}
	diff := computeDiff(oldLines, newLines)

	if len(diff) != 3 {
		t.Fatalf("Expected 3 diff entries, got %d", len(diff))
	}

	for i, d := range diff {
		if d.Op != DiffInsert {
			t.Errorf("Line %d: expected DiffInsert, got %v", i, d.Op)
		}
	}
}

func TestComputeDiff_SimpleReplacement(t *testing.T) {
	oldLines := []string{"old line"}
	newLines := []string{"new line"}
	diff := computeDiff(oldLines, newLines)

	// Should be: delete "old line", insert "new line"
	if len(diff) != 2 {
		t.Fatalf("Expected 2 diff entries, got %d", len(diff))
	}

	if diff[0].Op != DiffDelete || diff[0].Content != "old line" {
		t.Errorf("Expected delete 'old line', got %v %q", diff[0].Op, diff[0].Content)
	}
	if diff[1].Op != DiffInsert || diff[1].Content != "new line" {
		t.Errorf("Expected insert 'new line', got %v %q", diff[1].Op, diff[1].Content)
	}
}

func TestComputeDiff_WithContext(t *testing.T) {
	oldLines := []string{"context1", "context2", "old", "context3", "context4"}
	newLines := []string{"context1", "context2", "new", "context3", "context4"}
	diff := computeDiff(oldLines, newLines)

	// Count operations
	var equals, deletes, inserts int
	for _, d := range diff {
		switch d.Op {
		case DiffEqual:
			equals++
		case DiffDelete:
			deletes++
		case DiffInsert:
			inserts++
		}
	}

	if equals != 4 {
		t.Errorf("Expected 4 equal lines, got %d", equals)
	}
	if deletes != 1 {
		t.Errorf("Expected 1 delete, got %d", deletes)
	}
	if inserts != 1 {
		t.Errorf("Expected 1 insert, got %d", inserts)
	}
}

func TestComputeDiff_LineNumbers(t *testing.T) {
	oldLines := []string{"a", "b", "c"}
	newLines := []string{"a", "x", "c"}
	diff := computeDiff(oldLines, newLines)

	// Check that line numbers are correct
	// Expected: a (equal, old=1, new=1), b (delete, old=2), x (insert, new=2), c (equal, old=3, new=3)
	for _, d := range diff {
		if d.Content == "a" {
			if d.OldLineNum != 1 || d.NewLineNum != 1 {
				t.Errorf("'a' should have old=1, new=1, got old=%d, new=%d", d.OldLineNum, d.NewLineNum)
			}
		}
		if d.Content == "b" && d.Op == DiffDelete {
			if d.OldLineNum != 2 {
				t.Errorf("'b' (delete) should have old=2, got %d", d.OldLineNum)
			}
		}
		if d.Content == "x" && d.Op == DiffInsert {
			if d.NewLineNum != 2 {
				t.Errorf("'x' (insert) should have new=2, got %d", d.NewLineNum)
			}
		}
		if d.Content == "c" {
			if d.OldLineNum != 3 || d.NewLineNum != 3 {
				t.Errorf("'c' should have old=3, new=3, got old=%d, new=%d", d.OldLineNum, d.NewLineNum)
			}
		}
	}
}

func TestExtractHunks_SingleChange(t *testing.T) {
	// 10 lines with one change in the middle
	oldLines := []string{"1", "2", "3", "4", "old", "6", "7", "8", "9", "10"}
	newLines := []string{"1", "2", "3", "4", "new", "6", "7", "8", "9", "10"}
	diff := computeDiff(oldLines, newLines)
	hunks := extractHunks(diff, 3, 3) // 3 lines context, 3 gap threshold

	if len(hunks) != 1 {
		t.Fatalf("Expected 1 hunk, got %d", len(hunks))
	}

	// Hunk should include context before and after
	hunk := hunks[0]
	var hasContext2, hasOld, hasNew, hasContext6 bool
	for _, d := range hunk.Lines {
		switch d.Content {
		case "2":
			hasContext2 = true
		case "old":
			hasOld = true
		case "new":
			hasNew = true
		case "6":
			hasContext6 = true
		}
	}

	if !hasContext2 {
		t.Error("Expected context line '2' in hunk")
	}
	if !hasOld {
		t.Error("Expected 'old' deletion in hunk")
	}
	if !hasNew {
		t.Error("Expected 'new' insertion in hunk")
	}
	if !hasContext6 {
		t.Error("Expected context line '6' in hunk")
	}
}

func TestExtractHunks_TwoDistantChanges(t *testing.T) {
	// Create content with changes at line 3 and line 12 (far apart)
	oldLines := []string{"1", "2", "old1", "4", "5", "6", "7", "8", "9", "10", "11", "old2", "13", "14", "15"}
	newLines := []string{"1", "2", "new1", "4", "5", "6", "7", "8", "9", "10", "11", "new2", "13", "14", "15"}
	diff := computeDiff(oldLines, newLines)
	hunks := extractHunks(diff, 2, 3) // 2 lines context, 3 gap threshold

	if len(hunks) != 2 {
		t.Fatalf("Expected 2 hunks for distant changes, got %d", len(hunks))
	}
}

func TestExtractHunks_NoChanges(t *testing.T) {
	lines := []string{"a", "b", "c"}
	diff := computeDiff(lines, lines)
	hunks := extractHunks(diff, 3, 3)

	if len(hunks) != 0 {
		t.Errorf("Expected 0 hunks for identical content, got %d", len(hunks))
	}
}

func TestExtractHunks_EmptyInput(t *testing.T) {
	hunks := extractHunks(nil, 3, 3)
	if hunks != nil {
		t.Error("Expected nil for empty input")
	}
}

func TestFormatDiff_UnifiedOutput(t *testing.T) {
	// Test that the unified diff shows context and changes together
	oldStr := "line1\nline2\nold\nline4\nline5"
	newStr := "line1\nline2\nnew\nline4\nline5"
	result := formatDiff(oldStr, newStr, 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Should have context lines (line1, line2, line4, line5)
	if !strings.Contains(stripped, "line1") {
		t.Errorf("Expected context 'line1', got: %s", stripped)
	}
	if !strings.Contains(stripped, "line2") {
		t.Errorf("Expected context 'line2', got: %s", stripped)
	}
	if !strings.Contains(stripped, "line4") {
		t.Errorf("Expected context 'line4', got: %s", stripped)
	}

	// Should have the change markers
	if !strings.Contains(stripped, "- old") || !strings.Contains(stripped, "+ new") {
		t.Errorf("Expected '- old' and '+ new' markers, got: %s", stripped)
	}
}

func TestFormatDiff_ContextLinesAreDim(t *testing.T) {
	// Context lines should not have +/- markers
	oldStr := "context1\nold\ncontext2"
	newStr := "context1\nnew\ncontext2"
	result := formatDiff(oldStr, newStr, 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Context lines should have number but no +/- marker
	// Look for the pattern "N   content" (number, spaces, content)
	if !strings.Contains(stripped, "1   context1") {
		t.Errorf("Expected context line format '1   context1', got: %s", stripped)
	}
}

func TestFormatDiff_HunkSeparator(t *testing.T) {
	// Create content with distant changes that should produce multiple hunks
	var oldLines, newLines []string
	for i := 1; i <= 20; i++ {
		if i == 3 {
			oldLines = append(oldLines, "old1")
			newLines = append(newLines, "new1")
		} else if i == 17 {
			oldLines = append(oldLines, "old2")
			newLines = append(newLines, "new2")
		} else {
			line := strings.Repeat("x", i)
			oldLines = append(oldLines, line)
			newLines = append(newLines, line)
		}
	}

	result := formatDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Should have "..." separator between hunks
	if !strings.Contains(stripped, "...") {
		t.Errorf("Expected '...' separator between hunks, got: %s", stripped)
	}
}

func TestFormatDiff_TruncatesLongOutput(t *testing.T) {
	// Create a very long diff
	var oldLines, newLines []string
	for i := 1; i <= 50; i++ {
		oldLines = append(oldLines, "old"+string(rune('A'+i%26)))
		newLines = append(newLines, "new"+string(rune('A'+i%26)))
	}

	result := formatDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), 80, "")

	// Should have reasonable number of lines (maxDiffLines + truncation message)
	if len(result) > 20 {
		t.Errorf("Expected output to be truncated, got %d lines", len(result))
	}

	// Should have truncation indicator
	output := strings.Join(result, "\n")
	if !strings.Contains(output, "more lines") {
		t.Errorf("Expected truncation indicator, got: %s", output)
	}
}

func TestFormatDiff_EmptyOld(t *testing.T) {
	result := formatDiff("", "new line", 80, "")

	if len(result) == 0 {
		t.Error("Expected output for pure insertion")
	}

	output := strings.Join(result, "\n")
	stripped := stripANSI(output)
	if !strings.Contains(stripped, "+") {
		t.Errorf("Expected '+' marker for insertion, got: %s", stripped)
	}
}

func TestFormatDiff_EmptyNew(t *testing.T) {
	result := formatDiff("old line", "", 80, "")

	if len(result) == 0 {
		t.Error("Expected output for pure deletion")
	}

	output := strings.Join(result, "\n")
	stripped := stripANSI(output)
	if !strings.Contains(stripped, "-") {
		t.Errorf("Expected '-' marker for deletion, got: %s", stripped)
	}
}

func TestFormatDiff_BothEmpty(t *testing.T) {
	result := formatDiff("", "", 80, "")

	// Empty to empty should produce no diff
	if len(result) != 0 {
		t.Errorf("Expected empty result for empty-to-empty diff, got %d lines", len(result))
	}
}

func TestFormatDiff_WithSyntaxHighlighting(t *testing.T) {
	// Test that syntax highlighting is applied for known file types
	oldStr := "func main() {"
	newStr := "func main() {\n\tfmt.Println(\"hello\")"
	result := formatDiff(oldStr, newStr, 80, "test.go")

	if len(result) == 0 {
		t.Error("Expected output for Go code diff")
	}

	// The output should contain the code content (exact ANSI codes will vary)
	output := strings.Join(result, "\n")
	if !strings.Contains(output, "func") {
		t.Errorf("Expected 'func' in output, got: %s", output)
	}
	if !strings.Contains(output, "Println") {
		t.Errorf("Expected 'Println' in output, got: %s", output)
	}
}

func TestFormatDiff_NoHighlightingForUnknownType(t *testing.T) {
	// Test that unknown file types don't cause errors
	oldStr := "some text"
	newStr := "different text"
	result := formatDiff(oldStr, newStr, 80, "file.unknownext")

	if len(result) == 0 {
		t.Error("Expected output for unknown file type diff")
	}

	output := strings.Join(result, "\n")
	stripped := stripANSI(output)
	if !strings.Contains(stripped, "some text") || !strings.Contains(stripped, "different text") {
		t.Errorf("Expected content in output, got: %s", stripped)
	}
}

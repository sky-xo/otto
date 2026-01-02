// internal/tui/cell.go
package tui

// ColorType indicates how to interpret a Color value
type ColorType uint8

const (
	ColorNone     ColorType = iota // default/no color set
	ColorBasic                     // 0-15 standard + bright colors
	Color256                       // 0-255 palette
	ColorTrueColor                 // 24-bit RGB
)

// Color represents a terminal color
type Color struct {
	Type  ColorType
	Value uint32 // Basic: 0-15, Color256: 0-255, TrueColor: 0xRRGGBB
}

// CellStyle holds styling attributes for a cell
type CellStyle struct {
	FG     Color
	BG     Color
	Bold   bool
	Italic bool
}

// Cell represents a single character with its style
type Cell struct {
	Char  rune
	Style CellStyle
}

// StyledLine is a sequence of styled cells
type StyledLine []Cell

// String returns the plain text content without styling
func (sl StyledLine) String() string {
	runes := make([]rune, len(sl))
	for i, cell := range sl {
		runes[i] = cell.Char
	}
	return string(runes)
}

// WithSelection returns a copy of the line with selection highlighting applied
// from startCol to endCol (exclusive). The highlight background is applied
// while preserving existing foreground colors and attributes.
func (sl StyledLine) WithSelection(startCol, endCol int, bg Color) StyledLine {
	if len(sl) == 0 {
		return sl
	}

	// Clamp to valid range
	if startCol < 0 {
		startCol = 0
	}
	if endCol > len(sl) {
		endCol = len(sl)
	}
	if startCol >= endCol {
		return sl
	}

	// Make a copy
	result := make(StyledLine, len(sl))
	copy(result, sl)

	// Apply highlight background to selected range
	for i := startCol; i < endCol; i++ {
		result[i].Style.BG = bg
	}

	return result
}

// ParseStyledLine parses a string with ANSI escape codes into a StyledLine
func ParseStyledLine(s string) StyledLine {
	var result StyledLine
	var currentStyle CellStyle

	runes := []rune(s)
	i := 0

	for i < len(runes) {
		r := runes[i]

		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Start of CSI sequence - skip for now, implement in next task
			// Find the end of the sequence (letter a-zA-Z)
			j := i + 2
			for j < len(runes) && !isCSITerminator(runes[j]) {
				j++
			}
			if j < len(runes) {
				j++ // include terminator
			}
			i = j
			continue
		}

		result = append(result, Cell{Char: r, Style: currentStyle})
		i++
	}

	return result
}

// isCSITerminator returns true if r terminates a CSI sequence
func isCSITerminator(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

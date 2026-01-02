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

// internal/tui/syntax.go
package tui

import (
	"bytes"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Custom style based on monokai but with white/default color for regular code text.
// The original monokai uses lime/green (#a6e22e) for names like functions, classes, etc.
// This custom style changes those to white (#f8f8f2) for better readability.
var (
	customStyleOnce sync.Once
	customStyle     *chroma.Style
)

// getCustomStyle returns a modified monokai style with white default text color.
// Uses sync.Once to ensure the style is only built once.
func getCustomStyle() *chroma.Style {
	customStyleOnce.Do(func() {
		baseStyle := styles.Get("monokai")
		if baseStyle == nil {
			baseStyle = styles.Fallback
		}

		// Create a builder from the base style
		builder := baseStyle.Builder()

		// The lime/green color (#a6e22e) is used for various "Name" tokens in monokai.
		// Change them to white (#f8f8f2) to match regular text, while keeping
		// special syntax like keywords, strings, and comments colored.
		// IMPORTANT: Only set foreground color, NO background - we need the diff
		// line backgrounds (green for insertions, red for deletions) to show through.
		whiteText := "#f8f8f2"

		// Override the lime-colored tokens to use white instead
		builder.Add(chroma.NameFunction, whiteText)
		builder.Add(chroma.NameClass, whiteText)
		builder.Add(chroma.NameOther, whiteText)
		builder.Add(chroma.NameDecorator, whiteText)
		builder.Add(chroma.NameException, whiteText)
		builder.Add(chroma.NameAttribute, whiteText)

		// Also override background on common token types to ensure transparency.
		// Monokai sets bg:#272822 on the base style which bleeds through to tokens.
		// We need to clear this so diff backgrounds show correctly.
		builder.Add(chroma.Background, "bg:")
		builder.Add(chroma.Text, "#f8f8f2")
		builder.Add(chroma.TextWhitespace, "#f8f8f2")

		// Clear backgrounds from Comment tokens - monokai has grayish backgrounds
		// on comments that override the diff line backgrounds.
		builder.Add(chroma.Comment, "italic #75715e")
		builder.Add(chroma.CommentSingle, "italic #75715e")
		builder.Add(chroma.CommentMultiline, "italic #75715e")
		builder.Add(chroma.CommentSpecial, "italic #75715e")
		builder.Add(chroma.CommentPreproc, "italic #75715e")

		var err error
		customStyle, err = builder.Build()
		if err != nil {
			// Fall back to the base style if building fails
			customStyle = baseStyle
		}
	})
	return customStyle
}

// syntaxHighlight applies syntax highlighting to code content based on file extension.
// Returns the original content if highlighting fails or language is not detected.
func syntaxHighlight(content, filePath string) string {
	if content == "" || filePath == "" {
		return content
	}

	// Get lexer from file extension
	lexer := lexers.Match(filePath)
	if lexer == nil {
		// Try to detect from filename
		lexer = lexers.Match(filepath.Base(filePath))
	}
	if lexer == nil {
		// No lexer found, return plain content
		return content
	}

	// Coalesce runs of identical token types for cleaner output
	lexer = chroma.Coalesce(lexer)

	// Use custom style based on monokai with white text for regular code
	// This changes lime/green function/class names to white for better readability
	style := getCustomStyle()

	// Use Terminal256 formatter for ANSI output
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Tokenize and format
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return content
	}

	// Remove trailing newline that chroma sometimes adds
	result := buf.String()
	result = strings.TrimSuffix(result, "\n")

	return result
}

// highlightLine applies syntax highlighting to a single line of code.
// This is a convenience wrapper that handles the common case of highlighting diff lines.
func highlightLine(line, filePath string) string {
	return syntaxHighlight(line, filePath)
}

// highlightWithBackground applies syntax highlighting while preserving a background color.
// The issue: chroma outputs ANSI reset codes (\x1b[0m) that clear all attributes including background.
// This function re-applies the background after each reset sequence.
// The targetWidth parameter specifies the total width to pad to (0 means no padding).
// Padding is added BEFORE the final reset to maintain background color across the full width.
func highlightWithBackground(line, filePath string, bgANSI string, targetWidth int) string {
	highlighted := syntaxHighlight(line, filePath)
	if highlighted == line {
		// No highlighting applied, return as-is (caller will handle fallback styling)
		return line
	}

	// Replace all reset sequences with reset + re-apply background
	// \x1b[0m becomes \x1b[0m + bgANSI
	result := strings.ReplaceAll(highlighted, "\x1b[0m", "\x1b[0m"+bgANSI)

	// Calculate padding needed based on original line length (not ANSI-escaped length)
	padding := ""
	if targetWidth > 0 && len(line) < targetWidth {
		padding = strings.Repeat(" ", targetWidth-len(line))
	}

	// Wrap the whole thing: start with background, add padding BEFORE reset, end with reset
	return bgANSI + result + padding + "\x1b[0m"
}

// ANSI background codes for diff lines (256-color mode approximations)
const (
	// Dark theme backgrounds - using 256-color palette
	ANSIBgDeleteDark = "\x1b[48;2;61;27;27m"  // #3D1B1B - dark red
	ANSIBgInsertDark = "\x1b[48;2;27;61;27m"  // #1B3D1B - dark green
	// Light theme backgrounds
	ANSIBgDeleteLight = "\x1b[48;2;255;235;238m" // #FFEBEE - light red
	ANSIBgInsertLight = "\x1b[48;2;232;245;233m" // #E8F5E9 - light green
)

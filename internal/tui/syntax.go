// internal/tui/syntax.go
package tui

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

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

	// Use monokai style - works well on dark terminals
	// For light terminals, we could use a different style, but monokai is readable on both
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

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

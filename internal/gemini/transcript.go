package gemini

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TranscriptEntry represents a parsed entry from a Gemini session file
type TranscriptEntry struct {
	Type      string
	Content   string
	ToolName  string                 // Tool name for tool_use entries
	ToolInput map[string]interface{} // Tool parameters for tool_use entries
}

// ReadTranscript reads a Gemini session file from the given line offset.
// Returns entries and the new line count.
// Note: Assistant messages with delta:true are accumulated into single entries.
func ReadTranscript(path string, fromLine int) ([]TranscriptEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fromLine, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024)

	var entries []TranscriptEntry
	var pendingMessage strings.Builder
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= fromLine {
			continue
		}

		entry := parseEntry(scanner.Bytes())

		// Accumulate assistant message deltas
		if entry.Type == "message" {
			pendingMessage.WriteString(entry.Content)
			continue
		}

		// Flush pending message when we hit a non-message entry
		if pendingMessage.Len() > 0 {
			entries = append(entries, TranscriptEntry{
				Type:    "message",
				Content: pendingMessage.String(),
			})
			pendingMessage.Reset()
		}

		if entry.Content != "" {
			entries = append(entries, entry)
		}
	}

	// Flush any remaining message
	if pendingMessage.Len() > 0 {
		entries = append(entries, TranscriptEntry{
			Type:    "message",
			Content: pendingMessage.String(),
		})
	}

	return entries, lineNum, scanner.Err()
}

func parseEntry(data []byte) TranscriptEntry {
	var raw struct {
		Type       string                 `json:"type"`
		Role       string                 `json:"role"`
		Content    string                 `json:"content"`
		ToolName   string                 `json:"tool_name"`
		Output     string                 `json:"output"`
		Status     string                 `json:"status"`
		Parameters map[string]interface{} `json:"parameters"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return TranscriptEntry{}
	}

	switch raw.Type {
	case "message":
		if raw.Role == "user" {
			return TranscriptEntry{Type: "user", Content: raw.Content}
		}
		// Assistant message (may be delta)
		return TranscriptEntry{Type: "message", Content: raw.Content}

	case "tool_use":
		return TranscriptEntry{
			Type:      "tool",
			Content:   fmt.Sprintf("[tool: %s]", raw.ToolName), // Keep for backwards compat
			ToolName:  raw.ToolName,
			ToolInput: raw.Parameters,
		}

	case "tool_result":
		output := raw.Output
		// Truncate long outputs
		runes := []rune(output)
		if len(runes) > 200 {
			output = string(runes[:200]) + "..."
		}
		return TranscriptEntry{Type: "tool_output", Content: output}

	case "init", "result":
		// Skip these - init is metadata, result is just stats
		return TranscriptEntry{}
	}

	return TranscriptEntry{}
}

// FormatEntries formats transcript entries for display
func FormatEntries(entries []TranscriptEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case "user":
			sb.WriteString("[user] ")
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "message":
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "tool":
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		case "tool_output":
			sb.WriteString("  -> ")
			sb.WriteString(e.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

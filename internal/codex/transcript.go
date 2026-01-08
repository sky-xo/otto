package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TranscriptEntry represents a parsed entry from a Codex session file
type TranscriptEntry struct {
	Type      string
	Content   string
	ToolName  string                 // Tool name for function_call entries
	ToolInput map[string]interface{} // Tool arguments for function_call entries
}

// ReadTranscript reads a Codex session file from the given line offset
// Returns entries and the new line count
func ReadTranscript(path string, fromLine int) ([]TranscriptEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fromLine, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Set larger buffer for long lines
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024)

	var entries []TranscriptEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= fromLine {
			continue
		}

		entry := parseEntry(scanner.Bytes())
		if entry.Content != "" {
			entries = append(entries, entry)
		}
	}

	return entries, lineNum, scanner.Err()
}

func parseEntry(data []byte) TranscriptEntry {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return TranscriptEntry{}
	}

	// Get payload for nested type info
	payload, _ := raw["payload"].(map[string]interface{})
	if payload == nil {
		return TranscriptEntry{}
	}

	// Actual Codex format uses payload.type for the event type
	payloadType, _ := payload["type"].(string)

	switch payloadType {
	// Skip agent_reasoning - it duplicates "reasoning" response_item
	case "reasoning":
		// response_item with payload.type = "reasoning", summary[0].text = content
		if summary, ok := payload["summary"].([]interface{}); ok && len(summary) > 0 {
			if first, ok := summary[0].(map[string]interface{}); ok {
				if text, ok := first["text"].(string); ok {
					return TranscriptEntry{Type: "reasoning", Content: text}
				}
			}
		}
	case "message":
		// response_item with payload.type = "message", content[0].text = content
		if content, ok := payload["content"].([]interface{}); ok && len(content) > 0 {
			if first, ok := content[0].(map[string]interface{}); ok {
				if text, ok := first["text"].(string); ok {
					return TranscriptEntry{Type: "message", Content: text}
				}
			}
		}
	// Skip agent_message - it duplicates "message" response_item
	case "function_call":
		// response_item with payload.type = "function_call", payload.name = tool name
		name, _ := payload["name"].(string)
		if name == "" {
			return TranscriptEntry{}
		}

		// Parse arguments JSON string into map
		var toolInput map[string]interface{}
		if argsStr, ok := payload["arguments"].(string); ok && argsStr != "" {
			if err := json.Unmarshal([]byte(argsStr), &toolInput); err != nil {
				fmt.Fprintf(os.Stderr, "codex: failed to unmarshal tool arguments for %s: %v\n", name, err)
			}
		}

		return TranscriptEntry{
			Type:      "tool",
			Content:   fmt.Sprintf("[tool: %s]", name), // Keep for backwards compat
			ToolName:  name,
			ToolInput: toolInput,
		}
	case "function_call_output":
		// response_item with payload.type = "function_call_output", payload.output = result
		if output, ok := payload["output"].(string); ok {
			// Truncate long outputs (using runes to avoid splitting multi-byte UTF-8 chars)
			runes := []rune(output)
			if len(runes) > 200 {
				output = string(runes[:200]) + "..."
			}
			return TranscriptEntry{Type: "tool_output", Content: output}
		}
	}

	return TranscriptEntry{}
}

// FormatEntries formats transcript entries for display
func FormatEntries(entries []TranscriptEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Type {
		case "message":
			sb.WriteString(e.Content)
			sb.WriteString("\n\n")
		case "reasoning":
			sb.WriteString("[thinking] ")
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

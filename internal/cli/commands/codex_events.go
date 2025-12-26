package commands

import "encoding/json"

// CodexItem represents the item field in Codex events
type CodexItem struct {
	Type             string `json:"type"`
	Text             string `json:"text,omitempty"`
	Command          string `json:"command,omitempty"`
	AggregatedOutput string `json:"aggregated_output,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	Status           string `json:"status,omitempty"`
}

// CodexEvent represents a JSON event emitted by Codex
type CodexEvent struct {
	Type     string     `json:"type"`
	ThreadID string     `json:"thread_id,omitempty"`
	Status   string     `json:"status,omitempty"`
	Item     *CodexItem `json:"item,omitempty"`
	Raw      string
}

// ParseCodexEvent parses a JSON line from Codex stdout into a CodexEvent.
// Returns an empty CodexEvent (Type == "") if the line is not valid JSON.
func ParseCodexEvent(line string) CodexEvent {
	var payload CodexEvent
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return CodexEvent{}
	}
	payload.Raw = line
	return payload
}

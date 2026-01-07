package cli

import (
	"fmt"
	"time"
)

// relativeTime returns a human-readable string like "2 hours ago"
func relativeTime(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// formatCollisionError creates a helpful error message when an agent name already exists
func formatCollisionError(name string, spawnedAt time.Time) string {
	return fmt.Sprintf("agent %q already exists (spawned %s)\nHint: use --name %s-2 or another unique name",
		name, relativeTime(spawnedAt), name)
}

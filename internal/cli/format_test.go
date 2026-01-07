package cli

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"30 seconds", 30 * time.Second, "just now"},
		{"59 seconds", 59 * time.Second, "just now"},
		{"61 seconds", 61 * time.Second, "1 minute ago"},
		{"5 minutes", 5*time.Minute + 30*time.Second, "5 minutes ago"},
		{"61 minutes", 61 * time.Minute, "1 hour ago"},
		{"2 hours", 2*time.Hour + 30*time.Minute, "2 hours ago"},
		{"23 hours", 23*time.Hour + 59*time.Minute, "23 hours ago"},
		{"25 hours", 25 * time.Hour, "1 day ago"},
		{"3 days", 72 * time.Hour, "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			past := time.Now().Add(-tt.duration)
			got := relativeTime(past)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

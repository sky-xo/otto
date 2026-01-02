// internal/claude/channels.go
package claude

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Channel represents a group of agents from a branch/worktree.
type Channel struct {
	Name   string  // Display name like "june:main" or "june:channels"
	Dir    string  // Full path to Claude project directory
	Agents []Agent // Agents in this channel
}

// FindRelatedProjectDirs finds all Claude project directories that share
// the same base project path (main repo + worktrees).
func FindRelatedProjectDirs(claudeProjectsDir, basePath string) []string {
	// Convert base path to Claude's dash format
	basePrefix := strings.ReplaceAll(basePath, "/", "-")

	entries, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		return nil
	}

	var related []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Match exact base or base with worktree suffix
		if name == basePrefix || strings.HasPrefix(name, basePrefix+"-") {
			related = append(related, filepath.Join(claudeProjectsDir, name))
		}
	}
	return related
}

// ExtractChannelName creates a display name like "june:main" or "june:feature".
// baseDir is the main repo's Claude dir name (e.g., "-Users-test-code-june")
// projectDir is the current dir name (e.g., "-Users-test-code-june--worktrees-channels")
// repoName is the repository name (e.g., "june")
func ExtractChannelName(baseDir, projectDir, repoName string) string {
	if projectDir == baseDir {
		return repoName + ":main"
	}

	// Extract worktree name from suffix
	suffix := strings.TrimPrefix(projectDir, baseDir)

	// Find the last "-worktrees-" marker and take everything after it.
	// This preserves hyphenated branch names like "select-mode".
	const worktreeMarker = "-worktrees-"
	lastIdx := strings.LastIndex(suffix, worktreeMarker)
	if lastIdx != -1 {
		branchName := suffix[lastIdx+len(worktreeMarker):]
		if branchName != "" {
			return repoName + ":" + branchName
		}
	}

	// Fallback: try removing leading dashes and "worktrees" prefix
	suffix = strings.TrimLeft(suffix, "-")
	suffix = strings.TrimPrefix(suffix, "worktrees-")
	suffix = strings.TrimPrefix(suffix, "worktrees")
	suffix = strings.TrimLeft(suffix, "-")

	if suffix == "" {
		return repoName + ":unknown"
	}
	return repoName + ":" + suffix
}

// ScanChannels scans all related project directories and returns channels
// sorted by most recent agent activity (channels with active/recent agents first).
func ScanChannels(claudeProjectsDir, basePath, repoName string) ([]Channel, error) {
	relatedDirs := FindRelatedProjectDirs(claudeProjectsDir, basePath)
	if len(relatedDirs) == 0 {
		return nil, nil
	}

	baseDir := strings.ReplaceAll(basePath, "/", "-")
	var channels []Channel

	for _, dir := range relatedDirs {
		dirName := filepath.Base(dir)
		channelName := ExtractChannelName(baseDir, dirName, repoName)

		agents, err := ScanAgents(dir)
		if err != nil {
			continue // Skip dirs we can't read
		}

		if len(agents) == 0 {
			continue // Skip empty channels
		}

		channels = append(channels, Channel{
			Name:   channelName,
			Dir:    dir,
			Agents: agents,
		})
	}

	// Sort channels by most recent agent (first agent in each channel is most recent due to ScanAgents sorting)
	sort.Slice(channels, func(i, j int) bool {
		// Channels with active agents come first
		iHasActive := len(channels[i].Agents) > 0 && channels[i].Agents[0].IsActive()
		jHasActive := len(channels[j].Agents) > 0 && channels[j].Agents[0].IsActive()
		if iHasActive != jHasActive {
			return iHasActive
		}

		// Then by most recent agent modification time
		var iTime, jTime time.Time
		if len(channels[i].Agents) > 0 {
			iTime = channels[i].Agents[0].LastMod
		}
		if len(channels[j].Agents) > 0 {
			jTime = channels[j].Agents[0].LastMod
		}
		return iTime.After(jTime)
	})

	return channels, nil
}

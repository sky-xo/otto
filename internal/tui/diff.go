// internal/tui/diff.go
package tui

// DiffOp represents a diff operation type.
type DiffOp int

const (
	DiffEqual  DiffOp = iota // Line is unchanged
	DiffDelete               // Line was deleted
	DiffInsert               // Line was inserted
)

// DiffLine represents a single line in a diff with its operation and content.
type DiffLine struct {
	Op         DiffOp
	Content    string
	OldLineNum int // Line number in old content (0 if insert)
	NewLineNum int // Line number in new content (0 if delete)
}

// Hunk represents a group of changes with surrounding context.
type Hunk struct {
	Lines []DiffLine
}

// computeDiff computes a line-by-line diff between old and new content.
// Returns a slice of DiffLine representing the unified diff.
func computeDiff(oldLines, newLines []string) []DiffLine {
	// Compute LCS (Longest Common Subsequence) matrix
	m, n := len(oldLines), len(newLines)

	// Build LCS length matrix
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Backtrack to build diff
	var result []DiffLine
	i, j := m, n

	// Collect in reverse, then flip
	var reversed []DiffLine

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			// Equal line
			reversed = append(reversed, DiffLine{
				Op:         DiffEqual,
				Content:    oldLines[i-1],
				OldLineNum: i,
				NewLineNum: j,
			})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			// Insert
			reversed = append(reversed, DiffLine{
				Op:         DiffInsert,
				Content:    newLines[j-1],
				OldLineNum: 0,
				NewLineNum: j,
			})
			j--
		} else if i > 0 {
			// Delete
			reversed = append(reversed, DiffLine{
				Op:         DiffDelete,
				Content:    oldLines[i-1],
				OldLineNum: i,
				NewLineNum: 0,
			})
			i--
		}
	}

	// Reverse to get correct order
	for k := len(reversed) - 1; k >= 0; k-- {
		result = append(result, reversed[k])
	}

	return result
}

// extractHunks groups diff lines into hunks with context.
// contextLines specifies how many unchanged lines to show around changes.
// gapThreshold specifies how many unchanged lines before starting a new hunk.
func extractHunks(diff []DiffLine, contextLines, gapThreshold int) []Hunk {
	if len(diff) == 0 {
		return nil
	}

	// Find indices of all changes (non-equal lines)
	var changeIndices []int
	for i, d := range diff {
		if d.Op != DiffEqual {
			changeIndices = append(changeIndices, i)
		}
	}

	if len(changeIndices) == 0 {
		// No changes
		return nil
	}

	var hunks []Hunk

	// Group changes into hunks based on proximity
	hunkStart := 0
	for i := 0; i < len(changeIndices); i++ {
		changeIdx := changeIndices[i]

		// Determine the start of context for this change
		contextStart := changeIdx - contextLines
		if contextStart < 0 {
			contextStart = 0
		}

		// If this is first change or close enough to previous, extend current hunk
		if i == 0 {
			hunkStart = contextStart
		} else {
			prevChangeIdx := changeIndices[i-1]
			// Check gap between previous change and this context start
			gap := contextStart - prevChangeIdx - 1
			if gap > gapThreshold {
				// End previous hunk with context after last change
				prevEnd := prevChangeIdx + contextLines + 1
				if prevEnd > len(diff) {
					prevEnd = len(diff)
				}
				// Also ensure we don't overlap with next hunk
				if prevEnd > contextStart {
					prevEnd = contextStart
				}
				hunks = append(hunks, Hunk{Lines: diff[hunkStart:prevEnd]})
				hunkStart = contextStart
			}
		}
	}

	// Close final hunk
	lastChangeIdx := changeIndices[len(changeIndices)-1]
	hunkEnd := lastChangeIdx + contextLines + 1
	if hunkEnd > len(diff) {
		hunkEnd = len(diff)
	}
	hunks = append(hunks, Hunk{Lines: diff[hunkStart:hunkEnd]})

	return hunks
}

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/sky-xo/june/internal/db"
)

var adjectives = []string{
	"swift", "quiet", "bold", "clever", "bright",
	"calm", "eager", "fair", "gentle", "happy",
	"keen", "lively", "merry", "noble", "proud",
	"quick", "ready", "sharp", "steady", "true",
	"warm", "wise", "young", "brave", "clear",
	"crisp", "deft", "dry", "fast", "firm",
	"fresh", "grand", "great", "kind", "light",
	"neat", "plain", "prime", "pure", "rare",
	"rich", "safe", "slim", "smooth", "soft",
	"sound", "spare", "strong", "sweet", "tidy",
}

var nouns = []string{
	"falcon", "river", "spark", "stone", "wave",
	"arrow", "blade", "brook", "cloud", "crane",
	"crown", "dawn", "flame", "frost", "grove",
	"hawk", "helm", "horn", "lake", "lance",
	"leaf", "light", "marsh", "mesa", "mist",
	"moon", "oak", "path", "peak", "pine",
	"pond", "rain", "reef", "ridge", "rose",
	"sage", "shade", "shell", "shore", "sky",
	"slope", "snow", "spire", "spring", "star",
	"storm", "stream", "sun", "tide", "wind",
}

// generateAdjectiveNoun creates a random name like "swift-falcon"
func generateAdjectiveNoun() string {
	adjIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		panic(fmt.Sprintf("failed to generate random index: %v", err))
	}
	nounIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		panic(fmt.Sprintf("failed to generate random index: %v", err))
	}
	return adjectives[adjIdx.Int64()] + "-" + nouns[nounIdx.Int64()]
}

// buildAgentName creates a name from prefix + ULID suffix.
// If prefix is empty, generates adjective-noun prefix.
// Suffix is last 4 chars of ULID, lowercased.
func buildAgentName(prefix, ulid string) string {
	if prefix == "" {
		prefix = generateAdjectiveNoun()
	}
	suffix := strings.ToLower(ulid[len(ulid)-4:])
	return prefix + "-" + suffix
}

// randomHexSuffix generates 4 random hex chars for collision fallback
func randomHexSuffix() string {
	bytes := make([]byte, 2)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	return hex.EncodeToString(bytes)
}

// resolveAgentNameWithULID builds a name from prefix + ULID suffix.
// If collision, falls back to random suffix with retries.
func resolveAgentNameWithULID(database *db.DB, prefix, ulid string) (string, error) {
	// Build initial name using ULID suffix
	name := buildAgentName(prefix, ulid)

	// Extract prefix for potential collision fallback (may have been auto-generated)
	lastDash := strings.LastIndex(name, "-")
	resolvedPrefix := name[:lastDash]

	_, err := database.GetAgent(name)
	if err == db.ErrAgentNotFound {
		return name, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to check for existing agent: %w", err)
	}

	// Collision (rare) - fall back to random suffix
	for attempts := 0; attempts < 10; attempts++ {
		name = resolvedPrefix + "-" + randomHexSuffix()
		_, err := database.GetAgent(name)
		if err == db.ErrAgentNotFound {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to check for existing agent: %w", err)
		}
	}

	return "", errors.New("failed to generate unique agent name after 10 attempts")
}

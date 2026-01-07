package cli

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const base62Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

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

// generateName creates a random name like "task-f3WlaB"
func generateName() string {
	suffix := make([]byte, 6)
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	for i := 0; i < 6; i++ {
		suffix[i] = base62Chars[randomBytes[i]%62]
	}
	return "task-" + string(suffix)
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

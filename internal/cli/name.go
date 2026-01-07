package cli

import (
	"crypto/rand"
	"fmt"
)

const base62Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

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

package cli

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/sky-xo/june/internal/db"
)

// generateTaskID creates a task ID with format "t-xxxxx"
// where xxxxx is 5 random hex characters (~1M possibilities)
func generateTaskID() (string, error) {
	b := make([]byte, 3) // 3 bytes = 6 hex chars, we use 5
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	// Format as hex and take first 5 chars after "t-"
	return fmt.Sprintf("t-%05x", b)[:7], nil // "t-" + 5 chars = 7 total
}

// generateUniqueTaskID generates a task ID that doesn't exist in the database.
// Retries up to 10 times on collision.
func generateUniqueTaskID(database *db.DB) (string, error) {
	for i := 0; i < 10; i++ {
		id, err := generateTaskID()
		if err != nil {
			return "", err
		}
		_, err = database.GetTask(id)
		if errors.Is(err, db.ErrTaskNotFound) {
			return id, nil // ID is available
		}
		if err != nil {
			return "", fmt.Errorf("check task existence: %w", err)
		}
		// ID exists, retry
	}
	return "", errors.New("failed to generate unique task ID after 10 attempts")
}

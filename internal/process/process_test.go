package process

import (
	"os"
	"testing"
)

func TestIsProcessAlive(t *testing.T) {
	// Current process should be alive
	if !IsProcessAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}

	// PID 0 or negative should return false
	if IsProcessAlive(0) {
		t.Fatal("PID 0 should not be alive")
	}

	if IsProcessAlive(-1) {
		t.Fatal("negative PID should not be alive")
	}
}

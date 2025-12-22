package process

import (
	"os"
	"syscall"
)

// IsProcessAlive checks if a process with the given PID is still running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send anything, just checks if process exists
	return proc.Signal(syscall.Signal(0)) == nil
}

package exec

import (
	"os"
	"os/exec"
)

// Runner wraps os/exec for easier test stubbing
type Runner interface {
	Run(name string, args ...string) error
}

// DefaultRunner uses os/exec
type DefaultRunner struct{}

func (r *DefaultRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

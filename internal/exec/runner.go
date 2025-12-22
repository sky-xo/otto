package exec

import (
	"os"
	"os/exec"
)

// Runner wraps os/exec for easier test stubbing
type Runner interface {
	Run(name string, args ...string) error
	Start(name string, args ...string) (pid int, wait func() error, err error)
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

func (r *DefaultRunner) Start(name string, args ...string) (int, func() error, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return 0, nil, err
	}
	return cmd.Process.Pid, cmd.Wait, nil
}

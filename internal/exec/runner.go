package exec

import (
	"bufio"
	"os"
	"os/exec"
)

// Runner wraps os/exec for easier test stubbing
type Runner interface {
	Run(name string, args ...string) error
	Start(name string, args ...string) (pid int, wait func() error, err error)
	StartWithCapture(name string, args ...string) (pid int, stdoutLines <-chan string, wait func() error, err error)
	StartWithCaptureEnv(name string, env []string, args ...string) (pid int, stdoutLines <-chan string, wait func() error, err error)
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

func (r *DefaultRunner) StartWithCapture(name string, args ...string) (int, <-chan string, func() error, error) {
	return r.StartWithCaptureEnv(name, nil, args...)
}

func (r *DefaultRunner) StartWithCaptureEnv(name string, env []string, args ...string) (int, <-chan string, func() error, error) {
	cmd := exec.Command(name, args...)

	// Set environment if provided
	if env != nil {
		cmd.Env = env
	}

	// Get stdout pipe for reading
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return 0, nil, nil, err
	}

	// Stderr goes directly to terminal
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Start the command
	if err := cmd.Start(); err != nil {
		return 0, nil, nil, err
	}

	// Create channel for streaming lines
	lines := make(chan string, 10)

	// Start goroutine to read and forward stdout
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// Send to channel for parsing
			lines <- line
			// Also write to stdout so user sees output
			os.Stdout.WriteString(line + "\n")
		}
	}()

	return cmd.Process.Pid, lines, cmd.Wait, nil
}

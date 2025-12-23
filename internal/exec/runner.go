package exec

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Runner wraps os/exec for easier test stubbing
type Runner interface {
	Run(name string, args ...string) error
	RunWithEnv(name string, env []string, args ...string) error
	Start(name string, args ...string) (pid int, wait func() error, err error)
	StartWithCapture(name string, args ...string) (pid int, stdoutLines <-chan string, wait func() error, err error)
	StartWithCaptureEnv(name string, env []string, args ...string) (pid int, stdoutLines <-chan string, wait func() error, err error)
	StartWithTranscriptCapture(name string, args ...string) (pid int, output <-chan TranscriptChunk, wait func() error, err error)
	StartWithTranscriptCaptureEnv(name string, env []string, args ...string) (pid int, output <-chan TranscriptChunk, wait func() error, err error)
}

// DefaultRunner uses os/exec
type DefaultRunner struct {
	TranscriptBufferSize int
}

// TranscriptChunk represents a batched output chunk from stdout or stderr.
type TranscriptChunk struct {
	Stream string
	Data   string
}

const defaultTranscriptBufferSize = 4 * 1024

func (r *DefaultRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *DefaultRunner) RunWithEnv(name string, env []string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if env != nil {
		cmd.Env = env
	}
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

// StartWithCaptureEnv starts a command with custom environment and captures stdout
// in a channel for line-by-line processing. If env is nil, the command inherits
// the parent process environment. If env is non-nil, it completely replaces the
// environment (use append(os.Environ(), "KEY=val") to extend parent environment).
// Returns the process PID, a channel for stdout lines, a wait function, and any error.
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

func (r *DefaultRunner) StartWithTranscriptCapture(name string, args ...string) (int, <-chan TranscriptChunk, func() error, error) {
	return r.StartWithTranscriptCaptureEnv(name, nil, args...)
}

// StartWithTranscriptCaptureEnv starts a command with custom environment and tees
// stdout/stderr to both the terminal and a buffered transcript channel.
func (r *DefaultRunner) StartWithTranscriptCaptureEnv(name string, env []string, args ...string) (int, <-chan TranscriptChunk, func() error, error) {
	cmd := exec.Command(name, args...)

	if env != nil {
		cmd.Env = env
	}

	bufferSize := r.TranscriptBufferSize
	if bufferSize <= 0 {
		bufferSize = defaultTranscriptBufferSize
	}

	capture := newTranscriptCapture(bufferSize)

	cmd.Stdout = io.MultiWriter(os.Stdout, capture.StreamWriter("stdout"))
	cmd.Stderr = io.MultiWriter(os.Stderr, capture.StreamWriter("stderr"))
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return 0, nil, nil, err
	}

	wait := func() error {
		err := cmd.Wait()
		capture.Close()
		return err
	}

	return cmd.Process.Pid, capture.Output(), wait, nil
}

type transcriptCapture struct {
	bufferSize int
	out        chan TranscriptChunk
	mu         sync.Mutex
	pending    []byte
	stream     string
	closed     bool
}

func newTranscriptCapture(bufferSize int) *transcriptCapture {
	return &transcriptCapture{
		bufferSize: bufferSize,
		out:        make(chan TranscriptChunk, 16),
	}
}

func (c *transcriptCapture) Output() <-chan TranscriptChunk {
	return c.out
}

func (c *transcriptCapture) StreamWriter(stream string) io.Writer {
	return &streamWriter{
		stream:  stream,
		capture: c,
	}
}

func (c *transcriptCapture) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.flushLocked()
	c.closed = true
	close(c.out)
}

func (c *transcriptCapture) write(stream string, data []byte) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if c.stream == "" {
		c.stream = stream
	}
	if c.stream != stream {
		c.flushLocked()
		c.stream = stream
	}
	c.pending = append(c.pending, data...)
	for len(c.pending) >= c.bufferSize {
		chunk := make([]byte, c.bufferSize)
		copy(chunk, c.pending[:c.bufferSize])
		c.pending = c.pending[c.bufferSize:]
		c.sendLocked(stream, chunk)
	}
}

func (c *transcriptCapture) flushLocked() {
	if len(c.pending) == 0 || c.stream == "" {
		return
	}
	chunk := make([]byte, len(c.pending))
	copy(chunk, c.pending)
	c.pending = c.pending[:0]
	c.sendLocked(c.stream, chunk)
	c.stream = ""
}

func (c *transcriptCapture) sendLocked(stream string, data []byte) {
	c.out <- TranscriptChunk{
		Stream: stream,
		Data:   string(data),
	}
}

type streamWriter struct {
	stream  string
	capture *transcriptCapture
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.capture.write(w.stream, p)
	return len(p), nil
}

package exec

import (
	"os"
	"strings"
	"testing"
)

func TestStartWithTranscriptCaptureBuffersOutput(t *testing.T) {
	restore := redirectStdoutStderr(t)
	defer restore()

	runner := &DefaultRunner{TranscriptBufferSize: 4}

	pid, chunksCh, wait, err := runner.StartWithTranscriptCapture("sh", "-c", "printf 'abcdefghij'")
	if err != nil {
		t.Fatalf("StartWithTranscriptCapture failed: %v", err)
	}
	if pid == 0 {
		t.Fatal("expected non-zero pid")
	}

	chunks, waitErr := collectChunks(chunksCh, wait)
	if waitErr != nil {
		t.Fatalf("wait failed: %v", waitErr)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	if chunks[0].Stream != "stdout" || chunks[0].Data != "abcd" {
		t.Fatalf("unexpected chunk[0]: %+v", chunks[0])
	}
	if chunks[1].Stream != "stdout" || chunks[1].Data != "efgh" {
		t.Fatalf("unexpected chunk[1]: %+v", chunks[1])
	}
	if chunks[2].Stream != "stdout" || chunks[2].Data != "ij" {
		t.Fatalf("unexpected chunk[2]: %+v", chunks[2])
	}
}

func TestStartWithTranscriptCaptureOrdersStreams(t *testing.T) {
	restore := redirectStdoutStderr(t)
	defer restore()

	runner := &DefaultRunner{TranscriptBufferSize: 1024}

	script := strings.Join([]string{
		"printf 'out1'",
		"sleep 0.01",
		"printf 'err1' 1>&2",
		"sleep 0.01",
		"printf 'out2'",
		"sleep 0.01",
		"printf 'err2' 1>&2",
	}, "; ")

	_, chunksCh, wait, err := runner.StartWithTranscriptCapture("sh", "-c", script)
	if err != nil {
		t.Fatalf("StartWithTranscriptCapture failed: %v", err)
	}

	chunks, waitErr := collectChunks(chunksCh, wait)
	if waitErr != nil {
		t.Fatalf("wait failed: %v", waitErr)
	}

	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}

	expect := []TranscriptChunk{
		{Stream: "stdout", Data: "out1"},
		{Stream: "stderr", Data: "err1"},
		{Stream: "stdout", Data: "out2"},
		{Stream: "stderr", Data: "err2"},
	}
	for i, chunk := range chunks {
		if chunk.Stream != expect[i].Stream || chunk.Data != expect[i].Data {
			t.Fatalf("unexpected chunk[%d]: %+v", i, chunk)
		}
	}
}

func collectChunks(ch <-chan TranscriptChunk, wait func() error) ([]TranscriptChunk, error) {
	var chunks []TranscriptChunk
	done := make(chan struct{})
	go func() {
		for chunk := range ch {
			chunks = append(chunks, chunk)
		}
		close(done)
	}()
	err := wait()
	<-done
	return chunks, err
}

func redirectStdoutStderr(t *testing.T) func() {
	t.Helper()

	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = null
	os.Stderr = null

	return func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		_ = null.Close()
	}
}

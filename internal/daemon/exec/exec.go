// Package exec runs a shell command and streams its stdout/stderr back as
// they are produced, for the daemon's Exec RPC.
package exec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

const readBufferSize = 8192

type StreamKind int

const (
	Stdout StreamKind = iota
	Stderr
)

type Chunk struct {
	Kind StreamKind
	Data []byte
}

type Result struct {
	ExitCode int32
	Err      string
}

// Session represents a single running "sh -c" command.
type Session struct {
	stdin     io.WriteCloser
	closeOnce sync.Once

	Chunks <-chan Chunk
	Done   <-chan Result
}

// Write sends data to the command's stdin.
func (s *Session) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

// CloseStdin closes the command's stdin, signalling EOF to it. Safe to call
// more than once (e.g. once explicitly and once implicitly on stream EOF).
func (s *Session) CloseStdin() error {
	var err error
	s.closeOnce.Do(func() { err = s.stdin.Close() })
	return err
}

// Start launches "sh -c command", with args (if any) exposed to the script
// as its positional parameters ($1, $2, ...), in workingDir (process cwd if
// empty). The command is killed if ctx is cancelled.
func Start(ctx context.Context, command string, args []string, workingDir string) (*Session, error) {
	shArgs := append([]string{"-c", command, "sh"}, args...)
	cmd := exec.CommandContext(ctx, "sh", shArgs...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	chunks := make(chan Chunk)
	done := make(chan Result, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go pump(stdout, Stdout, chunks, &wg)
	go pump(stderr, Stderr, chunks, &wg)

	go func() {
		wg.Wait()
		close(chunks)

		result := Result{}
		if err := cmd.Wait(); err != nil {
			var exitErr *exec.ExitError
			if ok := asExitError(err, &exitErr); ok {
				result.ExitCode = int32(exitErr.ExitCode())
			} else {
				result.ExitCode = -1
				result.Err = err.Error()
			}
		}
		done <- result
		close(done)
	}()

	return &Session{stdin: stdin, Chunks: chunks, Done: done}, nil
}

func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		*target = exitErr
	}
	return ok
}

func pump(r io.Reader, kind StreamKind, out chan<- Chunk, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, readBufferSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			out <- Chunk{Kind: kind, Data: data}
		}
		if err != nil {
			return
		}
	}
}

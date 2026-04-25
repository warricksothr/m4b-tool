package exec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	stdexec "os/exec"
	"strings"
	"time"
)

// Cmd describes a subprocess invocation. The zero value is not usable;
// at minimum Name must be set.
type Cmd struct {
	// Name is the binary to run — a bare name resolved against PATH, or an
	// absolute path. Never a shell command line.
	Name string
	// Args is the argv slice passed to the binary. Name is not included.
	Args []string
	// Dir is the working directory. Empty means the parent's cwd.
	Dir string
	// Env, if non-nil, replaces the parent environment. Nil inherits.
	Env []string
	// Stdin is piped to the child's stdin if non-nil.
	Stdin io.Reader
	// Timeout caps the wall-clock runtime. Zero means no deadline beyond
	// whatever the caller's context carries.
	Timeout time.Duration
	// OnStderr, if non-nil, is invoked once per line of stderr as the
	// process runs. Lines are also accumulated into Result.Stderr. Callers
	// that don't need incremental parsing leave this nil for buffered
	// capture, which is the common case.
	OnStderr func(line string)
	// OnTerminate, if non-nil, is invoked exactly once after Wait returns,
	// regardless of success or failure. Intended for temp-file cleanup.
	OnTerminate func()
}

// Result carries the captured output and exit status of a completed Run.
// Stdout and Stderr are always populated with whatever was captured, even
// when Run returns a non-nil error.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// stderrTailBytes caps the amount of stderr included in error messages.
const stderrTailBytes = 2048

// Run executes cmd and returns the captured output plus an error describing
// any non-zero exit, timeout, or spawn failure. A nil error means the
// process exited with status 0.
//
// On non-zero exit the returned error wraps the binary name, argv, exit
// code, and the tail of stderr; callers can still inspect Result for full
// output. Context cancellation (including the derived timeout) surfaces as
// [context.DeadlineExceeded] or [context.Canceled].
func Run(ctx context.Context, cmd Cmd) (Result, error) {
	if cmd.OnTerminate != nil {
		defer cmd.OnTerminate()
	}

	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	c := stdexec.CommandContext(ctx, cmd.Name, cmd.Args...)
	c.Dir = cmd.Dir
	c.Env = cmd.Env
	c.Stdin = cmd.Stdin

	var stdoutBuf, stderrBuf bytes.Buffer
	c.Stdout = &stdoutBuf

	var stderrPipe io.ReadCloser
	if cmd.OnStderr != nil {
		pipe, err := c.StderrPipe()
		if err != nil {
			return Result{}, fmt.Errorf("exec: stderr pipe: %w", err)
		}
		stderrPipe = pipe
	} else {
		c.Stderr = &stderrBuf
	}

	if err := c.Start(); err != nil {
		return Result{}, fmt.Errorf("exec: start %s: %w", cmd.Name, err)
	}

	if stderrPipe != nil {
		scanner := bufio.NewScanner(stderrPipe)
		// Allow long lines (ffmpeg can print verbose single-line records).
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			cmd.OnStderr(line)
		}
	}

	waitErr := c.Wait()
	exitCode := c.ProcessState.ExitCode()

	result := Result{
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		ExitCode: exitCode,
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	if waitErr != nil {
		var exitErr *stdexec.ExitError
		if errors.As(waitErr, &exitErr) {
			return result, fmt.Errorf("%s %s: exit code %d: %s",
				cmd.Name, strings.Join(cmd.Args, " "), exitCode, tailString(result.Stderr))
		}
		return result, fmt.Errorf("exec: wait %s: %w", cmd.Name, waitErr)
	}
	return result, nil
}

func tailString(b []byte) string {
	if len(b) <= stderrTailBytes {
		return strings.TrimSpace(string(b))
	}
	return "..." + strings.TrimSpace(string(b[len(b)-stderrTailBytes:]))
}

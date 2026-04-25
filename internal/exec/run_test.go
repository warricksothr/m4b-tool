package exec_test

import (
	"context"
	"errors"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

var helperBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "exec-helper-*")
	if err != nil {
		panic(err)
	}
	helperBin = filepath.Join(tmp, "echo-helper")
	build := stdexec.Command("go", "build", "-o", helperBin, "./testdata/echo-helper")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		_ = os.RemoveAll(tmp)
		panic("failed to build echo-helper: " + err.Error())
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func TestRun_CapturesStdoutAndStderr(t *testing.T) {
	res, err := exec.Run(context.Background(), exec.Cmd{
		Name: helperBin,
		Args: []string{"-stdout", "hello-out", "-stderr", "hello-err"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(res.Stdout); got != "hello-out" {
		t.Errorf("stdout = %q, want %q", got, "hello-out")
	}
	if got := string(res.Stderr); got != "hello-err" {
		t.Errorf("stderr = %q, want %q", got, "hello-err")
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d, want 0", res.ExitCode)
	}
}

func TestRun_NonZeroExitReturnsErrorWithCapturedOutput(t *testing.T) {
	res, err := exec.Run(context.Background(), exec.Cmd{
		Name: helperBin,
		Args: []string{"-stderr", "bad things happened", "-exit", "3"},
	})
	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "exit code 3") {
		t.Errorf("error %q does not mention exit code 3", err)
	}
	if !strings.Contains(err.Error(), "bad things happened") {
		t.Errorf("error %q does not include stderr tail", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit = %d, want 3", res.ExitCode)
	}
	if string(res.Stderr) != "bad things happened" {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestRun_TimeoutKillsProcess(t *testing.T) {
	start := time.Now()
	_, err := exec.Run(context.Background(), exec.Cmd{
		Name:    helperBin,
		Args:    []string{"-sleep", "5s"},
		Timeout: 150 * time.Millisecond,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("process not killed promptly: elapsed %v", elapsed)
	}
}

func TestRun_StderrStreamingLines(t *testing.T) {
	var mu sync.Mutex
	var received []string
	res, err := exec.Run(context.Background(), exec.Cmd{
		Name: helperBin,
		Args: []string{"-lines", "one|two|three"},
		OnStderr: func(line string) {
			mu.Lock()
			received = append(received, line)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"one", "two", "three"}
	if len(received) != len(want) {
		t.Fatalf("received %d lines, want %d (%v)", len(received), len(want), received)
	}
	for i, ln := range want {
		if received[i] != ln {
			t.Errorf("line[%d] = %q, want %q", i, received[i], ln)
		}
	}
	// Streaming callback must not prevent buffered accumulation.
	if !strings.Contains(string(res.Stderr), "two") {
		t.Errorf("stderr buffer lost streamed content: %q", res.Stderr)
	}
}

func TestRun_OnTerminateAlwaysCalled(t *testing.T) {
	var calls int
	_, err := exec.Run(context.Background(), exec.Cmd{
		Name:        helperBin,
		Args:        []string{"-exit", "2"},
		OnTerminate: func() { calls++ },
	})
	if err == nil {
		t.Fatal("expected non-zero exit error")
	}
	if calls != 1 {
		t.Errorf("OnTerminate called %d times, want 1", calls)
	}
}

func TestRun_SpawnFailureReportsError(t *testing.T) {
	_, err := exec.Run(context.Background(), exec.Cmd{
		Name: "/nonexistent/binary/definitely-not-here",
	})
	if err == nil {
		t.Fatal("expected spawn error")
	}
}

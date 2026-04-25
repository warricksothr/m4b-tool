// echo-helper is a test fixture used by the internal/exec unit tests.
// It writes configurable output to stdout/stderr, optionally streams
// stderr lines with a delay, sleeps, and exits with a configurable code.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	stdout := flag.String("stdout", "", "text to write to stdout (no newline)")
	stderr := flag.String("stderr", "", "text to write to stderr (no newline)")
	exitCode := flag.Int("exit", 0, "exit code")
	sleep := flag.Duration("sleep", 0, "sleep this long before exiting")
	lines := flag.String("lines", "", "'|'-separated lines to emit to stderr one at a time")
	lineDelay := flag.Duration("line-delay", 0, "delay between streamed stderr lines")
	flag.Parse()

	if *stdout != "" {
		fmt.Fprint(os.Stdout, *stdout)
	}
	if *stderr != "" {
		fmt.Fprint(os.Stderr, *stderr)
	}
	if *lines != "" {
		for _, ln := range strings.Split(*lines, "|") {
			fmt.Fprintln(os.Stderr, ln)
			if *lineDelay > 0 {
				time.Sleep(*lineDelay)
			}
		}
	}
	if *sleep > 0 {
		time.Sleep(*sleep)
	}
	os.Exit(*exitCode)
}

// Package doctor implements the `m4b-tool doctor` subcommand: a
// quick external-tool health check that reports whether each runtime
// dependency is on PATH and prints its first-line version string.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	stdexec "os/exec"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// Tool describes one external dependency we shell out to.
type Tool struct {
	// Bin is the executable name as we'd pass it to exec.LookPath.
	Bin string
	// VersionArgs is the argv passed to print the version. The first
	// non-empty line of stdout/stderr is reported as the version
	// string. Some tools print to stdout, some to stderr — we accept
	// either.
	VersionArgs []string
	// Required marks tools without which the v1 commands cannot work.
	// `doctor` exits non-zero only if a required tool is missing.
	Required bool
	// Purpose is shown in the report so users know why the tool matters.
	Purpose string
}

// Tools is the catalog of external dependencies probed by `doctor`.
// Order is the rendered report order: required tools first.
var Tools = []Tool{
	{Bin: "ffmpeg", VersionArgs: []string{"-version"}, Required: true, Purpose: "audio encode/decode, silence detection, FFMETADATA"},
	{Bin: "mp4chaps", VersionArgs: []string{"--version"}, Required: true, Purpose: "MP4 chapter read/write"},
	{Bin: "mp4tags", VersionArgs: []string{"--version"}, Required: true, Purpose: "MP4 tag write"},
	{Bin: "mp4art", VersionArgs: []string{"--version"}, Required: true, Purpose: "MP4 cover art read/write"},
	{Bin: "mp4info", VersionArgs: []string{"--version"}, Required: true, Purpose: "MP4 duration probe"},
	{Bin: "fdkaac", VersionArgs: []string{"--version"}, Required: false, Purpose: "high-quality AAC encoder (libfdk_aac alternative)"},
	{Bin: "tone", VersionArgs: []string{"--version"}, Required: false, Purpose: "freeform MP4 atom write (post-v1)"},
}

// Result is one tool's probe outcome.
type Result struct {
	Tool    Tool
	Path    string
	Version string
	Err     error
}

// Found reports whether the tool was located on PATH at all.
func (r Result) Found() bool { return r.Path != "" }

// Probe runs lookPath + version probe for a single tool. It never
// returns an error — failures land on Result.Err. ctx caps the version
// probe; missing-binary cases short-circuit before exec.
func Probe(ctx context.Context, t Tool) Result {
	r := Result{Tool: t}
	path, err := stdexec.LookPath(t.Bin)
	if err != nil {
		r.Err = err
		return r
	}
	r.Path = path

	if len(t.VersionArgs) == 0 {
		return r
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := exec.Run(probeCtx, exec.Cmd{Name: path, Args: t.VersionArgs})
	if err != nil && !isExpectedExitErr(err) {
		r.Err = err
		return r
	}
	r.Version = firstNonEmptyLine(string(res.Stdout), string(res.Stderr))
	return r
}

// ProbeAll probes every tool in Tools, in order.
func ProbeAll(ctx context.Context) []Result {
	out := make([]Result, len(Tools))
	for i, t := range Tools {
		out[i] = Probe(ctx, t)
	}
	return out
}

// Render writes a human-readable report of the probe results to w.
// Returns ok=true when every required tool was found.
func Render(w io.Writer, results []Result) (ok bool) {
	ok = true
	_, _ = fmt.Fprintln(w, "m4b-tool doctor")
	_, _ = fmt.Fprintln(w, "===============")
	for _, r := range results {
		req := "optional"
		if r.Tool.Required {
			req = "required"
		}
		status := "OK"
		switch {
		case !r.Found():
			status = "MISSING"
			if r.Tool.Required {
				ok = false
			}
		case r.Err != nil:
			status = "ERROR"
			if r.Tool.Required {
				ok = false
			}
		}
		_, _ = fmt.Fprintf(w, "  %-9s %-9s %s\n", r.Tool.Bin, "["+req+"]", status)
		_, _ = fmt.Fprintf(w, "    purpose: %s\n", r.Tool.Purpose)
		if r.Found() {
			_, _ = fmt.Fprintf(w, "    path:    %s\n", r.Path)
		}
		if r.Version != "" {
			_, _ = fmt.Fprintf(w, "    version: %s\n", r.Version)
		}
		if r.Err != nil && r.Found() {
			_, _ = fmt.Fprintf(w, "    error:   %v\n", r.Err)
		}
	}
	if ok {
		_, _ = fmt.Fprintln(w, "\nall required tools present.")
	} else {
		_, _ = fmt.Fprintln(w, "\none or more required tools are missing or broken — see above.")
	}
	return ok
}

// Run is the subcommand entry point. Renders to os.Stdout and returns
// 0 when every required tool checks out, 1 otherwise.
func Run(ctx context.Context) int {
	results := ProbeAll(ctx)
	if Render(os.Stdout, results) {
		return 0
	}
	return 1
}

// firstNonEmptyLine returns the first non-empty trimmed line across
// the given strings, scanning each in order. Used to pick whichever
// stream the tool wrote its version to.
func firstNonEmptyLine(streams ...string) string {
	for _, s := range streams {
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return line
			}
		}
	}
	return ""
}

// isExpectedExitErr lets us tolerate non-zero exit codes from version
// probes. Some tools (notably mp4info on older builds) print usage to
// stderr and exit 1 when given an unrecognized flag — but the first
// stderr line still identifies the tool well enough for our report.
func isExpectedExitErr(err error) bool {
	var ee *stdexec.ExitError
	return errors.As(err, &ee)
}

package ffmpeg

import (
	"fmt"
	stdexec "os/exec"
)

// DefaultBinary is the binary name used when a Client is constructed
// without an explicit path.
const DefaultBinary = "ffmpeg"

// Client wraps a resolved ffmpeg binary. The zero value is not usable;
// construct via [NewClient] so the binary-existence check runs once at
// startup rather than at first use.
type Client struct {
	// Bin is the absolute or PATH-resolved binary path.
	Bin string
	// Threads, if non-zero, is passed to ffmpeg as "-threads N".
	Threads int
	// ExtraArgs are user-supplied "--ffmpeg-param" flags appended to each
	// transcode invocation. They do not apply to probe-style calls.
	ExtraArgs []string
}

// NewClient resolves bin on PATH and returns a Client. An empty bin falls
// back to [DefaultBinary]. The returned error is actionable: it names the
// binary we looked for so the caller can surface an install hint.
func NewClient(bin string) (*Client, error) {
	if bin == "" {
		bin = DefaultBinary
	}
	resolved, err := stdexec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %q not found on PATH (install ffmpeg or set an explicit path)", bin)
	}
	return &Client{Bin: resolved}, nil
}

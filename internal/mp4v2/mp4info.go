package mp4v2

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// Two regex variants matching mp4info's duration output. The exact line
// format varies with container layout:
//
//	<NN>       audio   MPEG-4 AAC LC, 0.684 secs, 32 kbps, 44100 Hz
//	duration:    19012 ms
//
// See spec/external-tools.md §mp4info.
var (
	reMp4InfoSecs = regexp.MustCompile(`(\d+\.\d{3})\s+secs`)
	reMp4InfoMs   = regexp.MustCompile(`duration:\s+(\d+)\s+ms`)
)

// ProbeDuration runs `mp4info FILE` and returns the parsed duration.
// Per spec this is a cross-check against the ffmpeg probe; callers that
// already trust ffmpeg's probe can skip it.
func (c *Client) ProbeDuration(ctx context.Context, path string) (time.Duration, error) {
	res, err := exec.Run(ctx, exec.Cmd{Name: c.MP4Info, Args: []string{path}})
	if err != nil {
		return 0, fmt.Errorf("mp4info %s: %w", path, err)
	}
	return parseMp4InfoDuration(string(res.Stdout))
}

// parseMp4InfoDuration pulls a duration out of mp4info stdout, trying
// the milliseconds form first (authoritative when present), then the
// seconds form. Returns an error including the raw input on no match.
func parseMp4InfoDuration(out string) (time.Duration, error) {
	if m := reMp4InfoMs.FindStringSubmatch(out); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("mp4info: bad ms value %q: %w", m[1], err)
		}
		return time.Duration(n) * time.Millisecond, nil
	}
	if m := reMp4InfoSecs.FindStringSubmatch(out); m != nil {
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("mp4info: bad secs value %q: %w", m[1], err)
		}
		return time.Duration(f * float64(time.Second)).Round(time.Millisecond), nil
	}
	return 0, fmt.Errorf("mp4info: no duration found in output: %s", out)
}

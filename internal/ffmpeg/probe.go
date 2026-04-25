package ffmpeg

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/exec"
)

// reDuration matches the "Duration: HH:MM:SS.ms" header ffmpeg emits once
// per input when loglevel permits info-level output.
var reDuration = regexp.MustCompile(`Duration:\s+(\d+:\d+:\d+(?:\.\d+)?)`)

// reTimeStat matches the "time=HH:MM:SS.ms" progress stat ffmpeg emits
// when run with "-stats". Multiple matches per stream — the last one wins.
var reTimeStat = regexp.MustCompile(`time=(\d+:\d+:\d+(?:\.\d+)?)`)

// ProbeDuration returns the duration reported in ffmpeg's input header.
// This is the "fast" variant: ~10ms for typical files because ffmpeg only
// decodes headers, not the audio stream itself. The trade-off is that VBR
// files with a missing/incorrect Xing frame may report a wrong duration;
// use [Client.ProbeDurationExact] when the header is not trustworthy.
//
// The invocation is `ffmpeg -hide_banner -i path`, which exits non-zero
// because no output is specified. That is expected; the Duration line has
// already been written to stderr by the time ffmpeg fails.
func (c *Client) ProbeDuration(ctx context.Context, path string) (time.Duration, error) {
	res, _ := exec.Run(ctx, exec.Cmd{
		Name: c.Bin,
		Args: []string{"-hide_banner", "-i", path},
	})
	// We ignore the exit code: ffmpeg always exits 1 on "no output", but
	// the header is in stderr either way. Only the parse result matters.
	return parseDurationFromHeader(string(res.Stderr))
}

// ProbeDurationExact re-encodes the stream to the null muxer to force a
// full decode, then reads the final "time=" stat. Slower than
// [Client.ProbeDuration] — roughly real-time for very-low-bitrate audio,
// much faster with hardware — but authoritative for any input ffmpeg can
// read.
func (c *Client) ProbeDurationExact(ctx context.Context, path string) (time.Duration, error) {
	res, err := exec.Run(ctx, exec.Cmd{
		Name: c.Bin,
		Args: []string{
			"-hide_banner",
			"-i", path,
			"-loglevel", "panic",
			"-stats",
			"-f", "null", "-",
		},
	})
	if err != nil {
		return 0, fmt.Errorf("ffmpeg exact probe: %w", err)
	}
	return parseDurationFromTimeStats(string(res.Stderr))
}

// parseDurationFromHeader scans ffmpeg stderr for the "Duration:" line.
// Returns an error when no such line is present (e.g. ffmpeg could not
// open the input at all).
func parseDurationFromHeader(stderr string) (time.Duration, error) {
	m := reDuration.FindStringSubmatch(stderr)
	if m == nil {
		return 0, fmt.Errorf("ffmpeg: no Duration header found in output")
	}
	d, err := audio.ParseDuration(m[1])
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: parsing Duration %q: %w", m[1], err)
	}
	return d, nil
}

// parseDurationFromTimeStats returns the last "time=" value ffmpeg printed.
// ffmpeg prints stats every 0.5s with \r between updates; FindAllString
// does not care about the separator.
func parseDurationFromTimeStats(stderr string) (time.Duration, error) {
	matches := reTimeStat.FindAllStringSubmatch(stderr, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("ffmpeg: no time= stat found in output")
	}
	last := matches[len(matches)-1][1]
	d, err := audio.ParseDuration(last)
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: parsing time= %q: %w", last, err)
	}
	return d, nil
}

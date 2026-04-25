package ffmpeg

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

// reAudioBitrate matches the kb/s value on ffmpeg's per-stream audio
// info line. Examples it must handle:
//
//	Stream #0:0[0x1](und): Audio: aac (LC) ..., 44100 Hz, stereo, fltp, 125 kb/s (default)
//	Stream #0:0: Audio: mp3, 22050 Hz, mono, fltp, 32 kb/s
//
// Capture group 1 is the bitrate in kbps. Lossless audio (FLAC, PCM)
// usually omits the kb/s field — the regex then has no match and
// ProbeAudioBitrate returns 0 (i.e. "unknown").
//
// Anchoring on `Audio:` (not the Stream prefix) keeps the regex from
// being fooled by the "Duration: ..., bitrate: NN kb/s" header line,
// which is the *container* bitrate, not the stream's. `[^\n]*?` keeps
// the lazy match bounded to a single line.
var reAudioBitrate = regexp.MustCompile(`Audio:[^\n]*?(\d+)\s*kb/s`)

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

// ProbeAudioBitrate returns the input's audio stream bitrate in bits
// per second, parsed from ffmpeg's input header. Returns 0 (with a
// nil error) when ffmpeg doesn't print a bitrate for the stream, which
// is normal for lossless containers (FLAC, WAV) — callers should treat
// "0 means unknown" as a clean signal to fall back to a codec default,
// rather than handling a typed error.
//
// Same invocation pattern as ProbeDuration: ffmpeg without an output
// exits non-zero, but the per-stream info has already been written to
// stderr, so the parse runs on whatever was emitted.
func (c *Client) ProbeAudioBitrate(ctx context.Context, path string) (int, error) {
	res, _ := exec.Run(ctx, exec.Cmd{
		Name: c.Bin,
		Args: []string{"-hide_banner", "-i", path},
	})
	return parseAudioBitrateFromHeader(string(res.Stderr))
}

// parseAudioBitrateFromHeader returns the audio stream's bitrate in
// bits per second, or 0 when ffmpeg's header didn't include one.
// "Not present" is not an error — see ProbeAudioBitrate.
func parseAudioBitrateFromHeader(stderr string) (int, error) {
	m := reAudioBitrate.FindStringSubmatch(stderr)
	if m == nil {
		return 0, nil
	}
	kbps, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: parsing audio bitrate %q: %w", m[1], err)
	}
	return kbps * 1000, nil
}

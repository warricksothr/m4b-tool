package ffmpeg

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/exec"
)

// DefaultSilenceNoise is the amplitude threshold passed to ffmpeg's
// silencedetect filter. -30 dBFS matches the PHP tool's default and works
// well for typical audiobook content.
const DefaultSilenceNoise = "-30dB"

// reSilenceEnd matches silencedetect's per-region trailing line:
//
//	[silencedetect @ 0x...] silence_end: 125.789 | silence_duration: 2.333
//
// Group 1: end seconds, group 2: duration seconds. Start is end − duration.
// Integer forms (no decimal point) are accepted for robustness.
var reSilenceEnd = regexp.MustCompile(`silence_end:\s+(\d+(?:\.\d+)?)\s+\|\s+silence_duration:\s+(\d+(?:\.\d+)?)`)

// DetectSilence runs ffmpeg's silencedetect filter on path and returns
// the detected silent regions in time order.
//
// minLen is the minimum silence length to report and is passed to ffmpeg
// as the filter's `d=` parameter; regions shorter than minLen are not
// emitted at all.
//
// maxLen, if > 0, is a post-parse upper bound: regions longer than maxLen
// are dropped as suspect (silent intros, long ambient gaps). Pass 0 to
// keep every detected region.
//
// Empty result with a nil error is the valid outcome for very short
// streams or for files that never reach the threshold, per
// spec/chapter-algorithms.md §Silence parsing.
func (c *Client) DetectSilence(ctx context.Context, path string, minLen, maxLen time.Duration) ([]audio.Silence, error) {
	if minLen <= 0 {
		return nil, fmt.Errorf("ffmpeg: DetectSilence: minLen must be positive, got %v", minLen)
	}
	filter := fmt.Sprintf("silencedetect=noise=%s:d=%s", DefaultSilenceNoise, formatSilenceSeconds(minLen))

	var (
		silences []audio.Silence
	)

	_, err := exec.Run(ctx, exec.Cmd{
		Name: c.Bin,
		Args: []string{
			"-hide_banner",
			"-i", path,
			"-af", filter,
			"-f", "null", "-",
		},
		OnStderr: func(line string) {
			if s, ok := parseSilenceLine(line); ok {
				if maxLen > 0 && s.Length > maxLen {
					return
				}
				silences = append(silences, s)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: silencedetect on %s: %w", path, err)
	}
	return silences, nil
}

// parseSilenceLine matches a silence_end record and converts it to a
// Silence. Returns ok=false for any line that doesn't match.
func parseSilenceLine(line string) (audio.Silence, bool) {
	m := reSilenceEnd.FindStringSubmatch(line)
	if m == nil {
		return audio.Silence{}, false
	}
	endSec, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return audio.Silence{}, false
	}
	durSec, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return audio.Silence{}, false
	}
	length := time.Duration(durSec * float64(time.Second)).Round(time.Millisecond)
	end := time.Duration(endSec * float64(time.Second)).Round(time.Millisecond)
	start := end - length
	if start < 0 {
		start = 0
	}
	return audio.Silence{Start: start, Length: length}, true
}

// formatSilenceSeconds renders a Duration as a seconds literal suitable
// for ffmpeg's filter syntax, trimming trailing zeroes. Example:
// 1500ms → "1.5"; 500ms → "0.5"; 2s → "2".
func formatSilenceSeconds(d time.Duration) string {
	s := strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
	// Trim trailing zeroes and a dangling decimal point for aesthetics.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

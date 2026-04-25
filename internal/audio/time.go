package audio

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDuration accepts the human-readable time formats the Go port needs
// to ingest from external tools, per spec/data-model.md §Time:
//
//   - "HH:MM:SS"        — ffmpeg duration header
//   - "HH:MM:SS.mmm"    — mp4chaps chapter line, ffmpeg stats
//   - bare seconds      — "10", "10.5" (ffmpeg silencedetect)
//   - bare milliseconds — parsed only when requested via ParseMillis
//
// Cue-sheet "MM:SS:FF" frames-based format is handled separately by the cue
// parser; it is not accepted here because ":" appears in two different
// positional meanings.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) != 3 {
			return 0, fmt.Errorf("duration %q: expected HH:MM:SS[.mmm]", s)
		}
		hours, err := strconv.Atoi(parts[0])
		if err != nil || hours < 0 {
			return 0, fmt.Errorf("duration %q: bad hours", s)
		}
		minutes, err := strconv.Atoi(parts[1])
		if err != nil || minutes < 0 || minutes >= 60 {
			return 0, fmt.Errorf("duration %q: bad minutes", s)
		}
		seconds, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || seconds < 0 || seconds >= 60 {
			return 0, fmt.Errorf("duration %q: bad seconds", s)
		}
		d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
		d += time.Duration(seconds * float64(time.Second))
		return d.Round(time.Millisecond), nil
	}

	seconds, err := strconv.ParseFloat(s, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("duration %q: not a number", s)
	}
	d := time.Duration(seconds * float64(time.Second))
	return d.Round(time.Millisecond), nil
}

// FormatHMS renders d as "HH:MM:SS.mmm" with millisecond precision, the
// format used by mp4chaps sidecars and most human-facing output.
func FormatHMS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	h := ms / (3600 * 1000)
	ms -= h * 3600 * 1000
	m := ms / (60 * 1000)
	ms -= m * 60 * 1000
	s := ms / 1000
	ms -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

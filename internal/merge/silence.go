package merge

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// synthSilenceTrack writes a silent audio segment of length d into
// tmpDir, encoded with the same container/codec the parts use so the
// concat-demuxer can stream-copy the result. Returns the path to the
// generated file.
func synthSilenceTrack(ctx context.Context, ff *ffmpeg.Client, tmpDir string, d time.Duration, opts ffmpeg.SilenceOptions) (string, error) {
	if d <= 0 {
		return "", fmt.Errorf("silence duration must be positive, got %v", d)
	}
	out := filepath.Join(tmpDir, "silence.m4a")
	silOpts := opts
	silOpts.Overwrite = true
	if err := ff.SynthesizeSilence(ctx, out, d.Seconds(), silOpts); err != nil {
		return "", err
	}
	return out, nil
}

// interleaveWithSilence inserts the silence file between consecutive
// parts, returning the flat list to feed into the concat demuxer.
// No silence is appended after the last part.
func interleaveWithSilence(parts []string, silencePath string) []string {
	if silencePath == "" || len(parts) < 2 {
		return parts
	}
	out := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			out = append(out, silencePath)
		}
		out = append(out, p)
	}
	return out
}

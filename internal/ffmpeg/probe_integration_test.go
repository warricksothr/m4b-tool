//go:build integration

package ffmpeg_test

import (
	"context"
	"math"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// synthSilenceMP3 writes a short silent MP3 fixture to path using a local
// ffmpeg invocation. Keeps the committed testdata small (nothing committed
// at all) while still exercising the full subprocess path.
func synthSilenceMP3(t *testing.T, path string, seconds float64) {
	t.Helper()
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=mono:sample_rate=22050",
		"-t", formatSeconds(seconds),
		"-c:a", "libmp3lame", "-q:a", "9",
		path,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to synthesize fixture with ffmpeg: %v", err)
	}
}

func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
}

func TestProbeDuration_Integration(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "silence.mp3")
	synthSilenceMP3(t, fixture, 2.500)

	client, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fast, err := client.ProbeDuration(ctx, fixture)
	if err != nil {
		t.Fatalf("ProbeDuration: %v", err)
	}
	if diff := math.Abs(fast.Seconds() - 2.5); diff > 0.2 {
		t.Errorf("fast probe = %v, want ~2.5s (diff %.3fs)", fast, diff)
	}

	exact, err := client.ProbeDurationExact(ctx, fixture)
	if err != nil {
		t.Fatalf("ProbeDurationExact: %v", err)
	}
	if diff := math.Abs(exact.Seconds() - 2.5); diff > 0.2 {
		t.Errorf("exact probe = %v, want ~2.5s (diff %.3fs)", exact, diff)
	}
}

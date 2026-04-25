//go:build integration

package ffmpeg_test

import (
	"context"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// TestDetectSilence_Integration synthesizes a 2s tone-silence-tone clip
// and asserts the detector finds a single silence in the middle.
func TestDetectSilence_Integration(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "gap.mp3")

	// 0.5s tone + 1.0s silence + 0.5s tone, concatenated via filter_complex.
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-f", "lavfi", "-i", "anullsrc=channel_layout=mono:sample_rate=22050:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-filter_complex", "[0:a][1:a][2:a]concat=n=3:v=0:a=1[out]",
		"-map", "[out]",
		"-ar", "22050", "-ac", "1",
		"-c:a", "libmp3lame", "-q:a", "9",
		fixture,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to synthesize silence fixture: %v", err)
	}

	client, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	silences, err := client.DetectSilence(ctx, fixture, 500*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("DetectSilence: %v", err)
	}
	if len(silences) != 1 {
		t.Fatalf("expected 1 silence region, got %d: %+v", len(silences), silences)
	}
	s := silences[0]
	// The gap starts ~0.5s in and lasts ~1.0s, give or take MP3 framing.
	if diff := abs(s.Start - 500*time.Millisecond); diff > 200*time.Millisecond {
		t.Errorf("start = %v, want ~500ms (diff %v)", s.Start, diff)
	}
	if diff := abs(s.Length - time.Second); diff > 200*time.Millisecond {
		t.Errorf("length = %v, want ~1s (diff %v)", s.Length, diff)
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

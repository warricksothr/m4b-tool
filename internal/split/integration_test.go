//go:build integration

package split_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
	"github.com/warricksothr/m4b-tool/internal/split"
)

// synthM4BWithChapters synthesizes a silent m4b with three embedded
// chapters at 0s, 4s, 10s. Returns the path. The chapter list is
// written via mp4chaps after the audio is muxed.
func synthM4BWithChapters(t *testing.T, dir string, totalSec float64) string {
	t.Helper()
	out := filepath.Join(dir, "book.m4b")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
		"-t", fmt.Sprintf("%.3f", totalSec),
		"-c:a", "aac", "-b:a", "64k",
		"-metadata", "title=Test Book",
		"-metadata", "artist=Narrator",
		"-f", "mp4", out,
	}
	cmd := stdexec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg synth: %v", err)
	}

	mp, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	chs := []audio.Chapter{
		{Start: 0, Length: 4 * time.Second, Name: "Prologue"},
		{Start: 4 * time.Second, Length: 6 * time.Second, Name: "Opening"},
		{Start: 10 * time.Second, Length: time.Duration(totalSec*float64(time.Second)) - 10*time.Second, Name: "Finale"},
	}
	if err := mp.WriteChapters(context.Background(), out, chs, time.Duration(totalSec*float64(time.Second))); err != nil {
		t.Fatalf("write chapters: %v", err)
	}
	return out
}

func TestSplit_EmbeddedChapters(t *testing.T) {
	dir := t.TempDir()
	in := synthM4BWithChapters(t, dir, 15.0)

	outDir := filepath.Join(dir, "out")
	var logs bytes.Buffer
	err := split.Run(context.Background(), split.Config{
		Input:     in,
		OutputDir: outDir,
		Stdout:    &logs,
		Stderr:    &logs,
	})
	if err != nil {
		t.Fatalf("split.Run: %v\nlogs:\n%s", err, logs.String())
	}

	for _, want := range []string{"001-Prologue.m4b", "002-Opening.m4b", "003-Finale.m4b"} {
		p := filepath.Join(outDir, want)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %q (%v)\nlogs:\n%s", want, err, logs.String())
		}
	}

	// Probe the second chapter; expect ~6 seconds.
	ff, _ := ffmpeg.NewClient("")
	d, err := ff.ProbeDuration(context.Background(), filepath.Join(outDir, "002-Opening.m4b"))
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if math.Abs(d.Seconds()-6.0) > 0.5 {
		t.Errorf("chapter 2 duration = %v, want ~6s", d)
	}
}

func TestSplit_FixedLengthAndReindex(t *testing.T) {
	dir := t.TempDir()
	in := synthM4BWithChapters(t, dir, 12.0)

	outDir := filepath.Join(dir, "out")
	var logs bytes.Buffer
	err := split.Run(context.Background(), split.Config{
		Input:           in,
		OutputDir:       outDir,
		FixedLength:     5 * time.Second,
		ReindexChapters: true,
		Stdout:          &logs,
		Stderr:          &logs,
	})
	if err != nil {
		t.Fatalf("split.Run: %v\nlogs:\n%s", err, logs.String())
	}

	// 12s / 5s = 3 chapters (5+5+2). With --reindex-chapters titles
	// become 1, 2, 3, so files are 001-1, 002-2, 003-3.
	for _, want := range []string{"001-1.m4b", "002-2.m4b", "003-3.m4b"} {
		p := filepath.Join(outDir, want)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %q (%v)\nlogs:\n%s", want, err, logs.String())
		}
	}

	ff, _ := ffmpeg.NewClient("")
	d, err := ff.ProbeDuration(context.Background(), filepath.Join(outDir, "003-3.m4b"))
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if math.Abs(d.Seconds()-2.0) > 0.5 {
		t.Errorf("trailing chapter duration = %v, want ~2s", d)
	}
}

func TestSplit_ParallelExtractionMatchesSerial(t *testing.T) {
	dir := t.TempDir()
	in := synthM4BWithChapters(t, dir, 15.0)

	serialDir := filepath.Join(dir, "serial")
	parallelDir := filepath.Join(dir, "parallel")

	for label, cfg := range map[string]split.Config{
		"serial":   {Input: in, OutputDir: serialDir, Jobs: 1},
		"parallel": {Input: in, OutputDir: parallelDir, Jobs: 4},
	} {
		var logs bytes.Buffer
		cfg.Stderr = &logs
		cfg.Stdout = &logs
		if err := split.Run(context.Background(), cfg); err != nil {
			t.Fatalf("%s: split.Run: %v\nlogs:\n%s", label, err, logs.String())
		}
	}

	// Both runs should produce identical filenames; durations should
	// match within a frame (~0.1s tolerance).
	serialFiles, _ := os.ReadDir(serialDir)
	parallelFiles, _ := os.ReadDir(parallelDir)
	if len(serialFiles) != len(parallelFiles) {
		t.Fatalf("file count differs: serial=%d parallel=%d", len(serialFiles), len(parallelFiles))
	}
	ff, _ := ffmpeg.NewClient("")
	for _, f := range serialFiles {
		ds, err := ff.ProbeDuration(context.Background(), filepath.Join(serialDir, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		dp, err := ff.ProbeDuration(context.Background(), filepath.Join(parallelDir, f.Name()))
		if err != nil {
			t.Fatalf("missing parallel output %q: %v", f.Name(), err)
		}
		if math.Abs((ds - dp).Seconds()) > 0.1 {
			t.Errorf("%s: serial=%v parallel=%v", f.Name(), ds, dp)
		}
	}
}

func TestSplit_MP3OutputWritesID3Tags(t *testing.T) {
	dir := t.TempDir()
	in := synthM4BWithChapters(t, dir, 12.0)
	outDir := filepath.Join(dir, "mp3-out")

	var logs bytes.Buffer
	err := split.Run(context.Background(), split.Config{
		Input:        in,
		OutputDir:    outDir,
		AudioFormat:  "mp3",
		AudioCodec:   "libmp3lame",
		AudioBitrate: "128k",
		TagOverrides: audio.Tag{
			Album:  "Test Book",
			Artist: "Narrator",
			Genre:  "Audiobook",
			Year:   2024,
		},
		Stdout: &logs,
		Stderr: &logs,
	})
	if err != nil {
		t.Fatalf("split.Run: %v\nlogs:\n%s", err, logs.String())
	}

	// Read tags back via ffprobe — independent of ffmpeg's writer
	// path, so this catches "we wrote the args but they didn't make
	// it to the file" failures.
	first := filepath.Join(outDir, "001-Prologue.mp3")
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("expected output: %v", err)
	}
	cmd := stdexec.Command("ffprobe", "-v", "error",
		"-show_entries", "format_tags=title,album,artist,track,genre,date",
		"-of", "default=noprint_wrappers=1", first)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	probed := string(out)
	for _, want := range []string{
		"TAG:title=Prologue",
		"TAG:album=Test Book",
		"TAG:artist=Narrator",
		"TAG:track=1/3",
		"TAG:genre=Audiobook",
		"TAG:date=2024",
	} {
		if !bytes.Contains([]byte(probed), []byte(want)) {
			t.Errorf("missing %q in ffprobe output:\n%s", want, probed)
		}
	}
}

func TestSplit_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	in := synthM4BWithChapters(t, dir, 12.0)
	outDir := filepath.Join(dir, "out")

	var stdout bytes.Buffer
	err := split.Run(context.Background(), split.Config{
		Input:     in,
		OutputDir: outDir,
		DryRun:    true,
		Stdout:    &stdout,
		Stderr:    &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("split.Run: %v", err)
	}
	if _, err := os.Stat(outDir); err == nil {
		t.Errorf("dry-run should not create output dir")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("=== dry run ===")) {
		t.Errorf("expected dry-run header in stdout, got:\n%s", stdout.String())
	}
}

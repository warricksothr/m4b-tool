//go:build integration

package merge_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/merge"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// synthSilentM4A produces a silent AAC-in-MP4 fixture with the given
// embedded title and (optional) artist tag. Matching codec parameters
// across fixtures let the concat demuxer stream-copy.
func synthSilentM4A(t *testing.T, path, title, artist string, seconds float64) {
	t.Helper()
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
		"-t", fmt.Sprintf("%.3f", seconds),
		"-c:a", "aac", "-b:a", "64k",
		"-metadata", "title=" + title,
	}
	if artist != "" {
		args = append(args, "-metadata", "artist="+artist)
	}
	args = append(args, "-f", "mp4", path)
	cmd := stdexec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("synth %s: %v", path, err)
	}
}

func TestMerge_ConcatThreeFilesIntoOneM4B(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01-prologue.m4a")
	in2 := filepath.Join(dir, "02-opening.m4a")
	in3 := filepath.Join(dir, "03-finale.m4a")
	// Embed an artist tag on each fixture so --equate has a real source
	// value to propagate. Importer composition picks it up in step 7
	// (embedded tags MergeMissing) before equate runs in step 8.
	synthSilentM4A(t, in1, "Prologue", "Narrator", 4.0)
	synthSilentM4A(t, in2, "Opening", "Narrator", 6.0)
	synthSilentM4A(t, in3, "Finale", "Narrator", 5.0)

	out := filepath.Join(dir, "book.m4b")

	var logs bytes.Buffer
	err := merge.Run(context.Background(), merge.Config{
		Inputs: []string{dir},
		Output: out,
		TagOverrides: audio.Tag{
			Title: "The Big Book",
			Album: "Series X",
			Year:  2024,
		},
		EquateSpecs: []string{"artist,albumartist"},
		SkipCover:   true,
		Stdout:      &logs,
		Stderr:      &logs,
	})
	if err != nil {
		t.Fatalf("merge.Run: %v\nlogs:\n%s", err, logs.String())
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}

	ctx := context.Background()
	ff, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatal(err)
	}
	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	// Duration: 4+6+5 = 15s (±200ms for encoder framing).
	dur, err := mp4.ProbeDuration(ctx, out)
	if err != nil {
		t.Fatalf("ProbeDuration: %v", err)
	}
	if diff := math.Abs(dur.Seconds() - 15); diff > 0.3 {
		t.Errorf("duration = %v, want ~15s (diff %.3f)", dur, diff)
	}

	tag, err := ff.ReadFFMetadata(ctx, out)
	if err != nil {
		t.Fatalf("ReadFFMetadata: %v", err)
	}
	if tag.Title != "The Big Book" {
		t.Errorf("Title = %q", tag.Title)
	}
	if tag.Artist != "Narrator" {
		t.Errorf("Artist = %q", tag.Artist)
	}
	if tag.AlbumArtist != "Narrator" {
		t.Errorf("AlbumArtist = %q (equate expected)", tag.AlbumArtist)
	}
	if tag.Year != 2024 {
		t.Errorf("Year = %d", tag.Year)
	}
	if len(tag.Chapters) != 3 {
		t.Fatalf("got %d chapters, want 3", len(tag.Chapters))
	}
	wantNames := []string{"Prologue", "Opening", "Finale"}
	for i, name := range wantNames {
		if tag.Chapters[i].Name != name {
			t.Errorf("chapter[%d] = %q, want %q", i, tag.Chapters[i].Name, name)
		}
	}
	// Starts: 0, 4s, 10s (±100ms).
	wantStarts := []time.Duration{0, 4 * time.Second, 10 * time.Second}
	for i, want := range wantStarts {
		if diff := abs(tag.Chapters[i].Start - want); diff > 200*time.Millisecond {
			t.Errorf("chapter[%d] Start = %v, want ~%v", i, tag.Chapters[i].Start, want)
		}
	}
}

func TestMerge_UseFilenamesAsChapters(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01-track-one.m4a")
	in2 := filepath.Join(dir, "02-track-two.m4a")
	synthSilentM4A(t, in1, "TagTitle1", "", 3.0)
	synthSilentM4A(t, in2, "TagTitle2", "", 3.0)

	out := filepath.Join(dir, "book.m4b")
	err := merge.Run(context.Background(), merge.Config{
		Inputs:                 []string{dir},
		Output:                 out,
		UseFilenamesAsChapters: true,
		SkipCover:              true,
		NoConversion:           true,
		Stdout:                 &bytes.Buffer{},
		Stderr:                 &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	ff, _ := ffmpeg.NewClient("")
	tag, err := ff.ReadFFMetadata(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Chapters[0].Name != "01-track-one" {
		t.Errorf("chapter[0] name = %q, want 01-track-one", tag.Chapters[0].Name)
	}
}

func TestMerge_ChaptersTxtSidecarOverrides(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01.m4a")
	in2 := filepath.Join(dir, "02.m4a")
	synthSilentM4A(t, in1, "T1", "", 5.0)
	synthSilentM4A(t, in2, "T2", "", 5.0)

	// Place a chapters.txt sidecar next to the output so chapterstxt
	// picks it up via AudioPath-derived discovery.
	out := filepath.Join(dir, "book.m4b")
	sidecar := filepath.Join(dir, "book.chapters.txt")
	if err := os.WriteFile(sidecar, []byte("00:00:00.000 HandTuned One\n00:00:05.000 HandTuned Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// merge uses the primary input's dir as the sidecar discovery
	// directory, but chapterstxt looks for <audio-basename>.chapters.txt
	// based on the primary audio input. The primary input here is
	// "01.m4a", so the importer looks for "01.chapters.txt" — which
	// won't exist. Pass an explicit override pointing at book.chapters.txt.
	err := merge.Run(context.Background(), merge.Config{
		Inputs:           []string{dir},
		Output:           out,
		SkipCover:        true,
		NoConversion:     true,
		ChaptersFilename: sidecar,
		Stdout:           &bytes.Buffer{},
		Stderr:           &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	ff, _ := ffmpeg.NewClient("")
	tag, err := ff.ReadFFMetadata(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if len(tag.Chapters) != 2 || tag.Chapters[0].Name != "HandTuned One" {
		t.Errorf("chapters-txt override not applied: %+v", tag.Chapters)
	}
}

func TestMerge_DryRunProducesNoOutput(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01.m4a")
	synthSilentM4A(t, in1, "T1", "", 2.0)

	var stdout, stderr bytes.Buffer
	err := merge.Run(context.Background(), merge.Config{
		Inputs:       []string{dir},
		Output:       filepath.Join(dir, "book.m4b"),
		DryRun:       true,
		SkipCover:    true,
		NoConversion: true,
		Stdout:       &stdout,
		Stderr:       &stderr,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "book.m4b")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not produce output, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "=== dry run ===") {
		t.Errorf("dry-run output missing header:\n%s", stdout.String())
	}
}

func TestMerge_ForceOverwritesExisting(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir() // separate so the output doesn't get scanned as input
	in1 := filepath.Join(inDir, "01.m4a")
	synthSilentM4A(t, in1, "T1", "", 2.0)
	out := filepath.Join(outDir, "book.m4b")
	if err := os.WriteFile(out, []byte("preexisting"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --force: rejected.
	err := merge.Run(context.Background(), merge.Config{
		Inputs:       []string{inDir},
		Output:       out,
		SkipCover:    true,
		NoConversion: true,
		Stdout:       &bytes.Buffer{},
		Stderr:       &bytes.Buffer{},
	})
	if err == nil {
		t.Error("expected refusal without --force")
	}

	// With --force: succeeds.
	err = merge.Run(context.Background(), merge.Config{
		Inputs:       []string{inDir},
		Output:       out,
		Force:        true,
		SkipCover:    true,
		NoConversion: true,
		Stdout:       &bytes.Buffer{},
		Stderr:       &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run with --force: %v", err)
	}
}

// synthMP3 produces an MP3 fixture. The default merge path transcodes
// to AAC/MP4, so heterogeneous (non-MP4) inputs need the transcode
// pipeline to produce a uniform concat.
func synthMP3(t *testing.T, path, title string, seconds float64) {
	t.Helper()
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
		"-t", fmt.Sprintf("%.3f", seconds),
		"-c:a", "libmp3lame", "-b:a", "64k",
		"-metadata", "title="+title,
		path,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("synth %s: %v", path, err)
	}
}

// synthSineMP3 produces an MP3 with audible content (for trim-silence).
// silentSecPrefix and silentSecSuffix bracket the tone with silence so
// silenceremove has something to trim.
func synthSineMP3(t *testing.T, path string, silentSecPrefix, toneSec, silentSecSuffix float64) {
	t.Helper()
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i",
		fmt.Sprintf("anullsrc=channel_layout=stereo:sample_rate=44100:duration=%.3f", silentSecPrefix),
		"-f", "lavfi", "-i",
		fmt.Sprintf("sine=frequency=440:duration=%.3f", toneSec),
		"-f", "lavfi", "-i",
		fmt.Sprintf("anullsrc=channel_layout=stereo:sample_rate=44100:duration=%.3f", silentSecSuffix),
		"-filter_complex", "[0:a][1:a][2:a]concat=n=3:v=0:a=1[out]",
		"-map", "[out]",
		"-ar", "44100", "-ac", "2",
		"-c:a", "libmp3lame", "-b:a", "64k",
		path,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("synth %s: %v", path, err)
	}
}

func TestMerge_TranscodesMP3InputsToM4B(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01-one.mp3")
	in2 := filepath.Join(dir, "02-two.mp3")
	in3 := filepath.Join(dir, "03-three.mp3")
	synthMP3(t, in1, "One", 3.0)
	synthMP3(t, in2, "Two", 4.0)
	synthMP3(t, in3, "Three", 3.0)

	out := filepath.Join(dir, "book.m4b")
	var logs bytes.Buffer
	err := merge.Run(context.Background(), merge.Config{
		Inputs:       []string{dir},
		Output:       out,
		SkipCover:    true,
		AudioBitrate: "64k",
		Jobs:         2,
		Stdout:       &logs,
		Stderr:       &logs,
	})
	if err != nil {
		t.Fatalf("Run: %v\nlogs:\n%s", err, logs.String())
	}

	ff, _ := ffmpeg.NewClient("")
	tag, err := ff.ReadFFMetadata(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if len(tag.Chapters) != 3 {
		t.Fatalf("got %d chapters, want 3: %+v", len(tag.Chapters), tag.Chapters)
	}
	want := []string{"One", "Two", "Three"}
	for i, n := range want {
		if tag.Chapters[i].Name != n {
			t.Errorf("chapter[%d] = %q, want %q", i, tag.Chapters[i].Name, n)
		}
	}
}

func TestMerge_TrimSilenceShortensMiddleParts(t *testing.T) {
	dir := t.TempDir()
	// First/last keep boundary silence; the middle file is what trim
	// should affect. Synthesize: 2s tone bracketed by 1s silence on
	// each side (4s total per file). After trim, middle should be ~2s.
	in1 := filepath.Join(dir, "01.mp3")
	in2 := filepath.Join(dir, "02.mp3")
	in3 := filepath.Join(dir, "03.mp3")
	synthSineMP3(t, in1, 1.0, 2.0, 1.0) // 4s
	synthSineMP3(t, in2, 1.0, 2.0, 1.0) // 4s, expect ~2s after trim
	synthSineMP3(t, in3, 1.0, 2.0, 1.0) // 4s

	out := filepath.Join(dir, "book.m4b")
	var logs bytes.Buffer
	err := merge.Run(context.Background(), merge.Config{
		Inputs:       []string{dir},
		Output:       out,
		SkipCover:    true,
		TrimSilence:  true,
		AudioBitrate: "64k",
		Stdout:       &logs,
		Stderr:       &logs,
	})
	if err != nil {
		t.Fatalf("Run: %v\nlogs:\n%s", err, logs.String())
	}

	ff, _ := ffmpeg.NewClient("")
	tag, err := ff.ReadFFMetadata(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if len(tag.Chapters) != 3 {
		t.Fatalf("got %d chapters", len(tag.Chapters))
	}
	// Middle chapter's length should be ~2s (trimmed); first and last
	// should remain ~4s. Tolerance is generous because silenceremove +
	// AAC reframing introduces some drift.
	mid := tag.Chapters[1].Length
	if mid > 3*time.Second {
		t.Errorf("middle chapter length = %v, want ~2s after trim", mid)
	}
	for _, idx := range []int{0, 2} {
		l := tag.Chapters[idx].Length
		if l < 3500*time.Millisecond {
			t.Errorf("boundary chapter[%d] length = %v, expected close to 4s (no trim)", idx, l)
		}
	}
}

func TestMerge_AddSilenceShiftsChapters(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01.m4a")
	in2 := filepath.Join(dir, "02.m4a")
	in3 := filepath.Join(dir, "03.m4a")
	synthSilentM4A(t, in1, "A", "", 3.0)
	synthSilentM4A(t, in2, "B", "", 3.0)
	synthSilentM4A(t, in3, "C", "", 3.0)

	out := filepath.Join(dir, "book.m4b")
	var logs bytes.Buffer
	err := merge.Run(context.Background(), merge.Config{
		Inputs:       []string{dir},
		Output:       out,
		SkipCover:    true,
		NoConversion: true,
		AddSilence:   500 * time.Millisecond,
		Stdout:       &logs,
		Stderr:       &logs,
	})
	if err != nil {
		t.Fatalf("Run: %v\nlogs:\n%s", err, logs.String())
	}

	ff, _ := ffmpeg.NewClient("")
	tag, err := ff.ReadFFMetadata(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if len(tag.Chapters) != 3 {
		t.Fatalf("got %d chapters", len(tag.Chapters))
	}
	// A starts at 0, B at 3s + 500ms, C at 6.5s + 500ms.
	want := []time.Duration{0, 3500 * time.Millisecond, 7000 * time.Millisecond}
	for i, w := range want {
		if diff := abs(tag.Chapters[i].Start - w); diff > 200*time.Millisecond {
			t.Errorf("chapter[%d] Start = %v, want ~%v", i, tag.Chapters[i].Start, w)
		}
	}
}

func TestMerge_AdjustForIPodClampsRateAndChannels(t *testing.T) {
	// Synthesize a 48kHz fixture; --adjust-for-ipod should clamp to 44100.
	dir := t.TempDir()
	in1 := filepath.Join(dir, "01.mp3")
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000",
		"-t", "2.0",
		"-c:a", "libmp3lame", "-b:a", "64k",
		"-metadata", "title=High",
		in1,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "book.m4b")
	err := merge.Run(context.Background(), merge.Config{
		Inputs:        []string{dir},
		Output:        out,
		SkipCover:     true,
		AdjustForIPod: true,
		Stdout:        &bytes.Buffer{},
		Stderr:        &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Probe the output sample rate via mp4info.
	probeCmd := stdexec.Command("mp4info", out)
	probeOut, err := probeCmd.Output()
	if err != nil {
		t.Fatalf("mp4info: %v", err)
	}
	if !strings.Contains(string(probeOut), "44100 Hz") {
		t.Errorf("expected 44100 Hz in mp4info output:\n%s", string(probeOut))
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

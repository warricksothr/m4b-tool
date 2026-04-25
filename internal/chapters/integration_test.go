//go:build integration

package chapters_test

import (
	"bytes"
	"context"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/chapters"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// synthM4BWithGap produces a 30s stereo AAC m4b fixture that contains a
// silent gap in the middle — tone [0, 10), silence [10, 12), tone
// [12, 30). Used to exercise --adjust-by-silence.
func synthM4BWithGap(t *testing.T, path string) {
	t.Helper()
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10",
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=18",
		"-filter_complex", "[0:a][1:a][2:a]concat=n=3:v=0:a=1[out]",
		"-map", "[out]",
		"-ar", "44100", "-ac", "2", "-c:a", "aac", "-b:a", "64k",
		"-f", "mp4",
		path,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("synth fixture: %v", err)
	}
}

func setInitialChapters(t *testing.T, client *mp4v2.Client, path string, chs []audio.Chapter, total time.Duration) {
	t.Helper()
	ctx := context.Background()
	if err := client.WriteChapters(ctx, path, chs, total); err != nil {
		t.Fatalf("seed chapters: %v", err)
	}
}

func readChapters(t *testing.T, client *ffmpeg.Client, path string) []audio.Chapter {
	t.Helper()
	ctx := context.Background()
	tag, err := client.ReadFFMetadata(ctx, path)
	if err != nil {
		t.Fatalf("read chapters: %v", err)
	}
	return tag.Chapters
}

func TestRun_AdjustBySilenceSnapsChapterStart(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	synthM4BWithGap(t, fixture)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatalf("mp4v2.NewClient: %v", err)
	}
	ff, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatalf("ffmpeg.NewClient: %v", err)
	}

	// Seed a chapter boundary ~500ms off the actual silence at 10-12s.
	initial := []audio.Chapter{
		{Start: 0, Length: 11500 * time.Millisecond, Name: "One"},
		{Start: 11500 * time.Millisecond, Length: 18500 * time.Millisecond, Name: "Two"},
	}
	setInitialChapters(t, mp4, fixture, initial, 30*time.Second)

	var logs bytes.Buffer
	err = chapters.Run(context.Background(), chapters.Config{
		Input:           fixture,
		AdjustBySilence: true,
		Stdout:          &logs,
		Stderr:          &logs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readChapters(t, ff, fixture)
	if len(got) != 2 {
		t.Fatalf("got %d chapters, want 2", len(got))
	}
	// Boundary should snap to the silence midpoint (~11s).
	target := 11 * time.Second
	if diff := absDur(got[1].Start - target); diff > 500*time.Millisecond {
		t.Errorf("chapter 2 Start = %v, want ~%v (silence midpoint)", got[1].Start, target)
	}
}

func TestRun_MergeSimilarCollapsesDuplicates(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	synthM4BWithGap(t, fixture)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	ff, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatal(err)
	}

	initial := []audio.Chapter{
		{Start: 0, Length: 10 * time.Second, Name: "Same"},
		{Start: 10 * time.Second, Length: 10 * time.Second, Name: "Same"},
		{Start: 20 * time.Second, Length: 10 * time.Second, Name: "Other"},
	}
	setInitialChapters(t, mp4, fixture, initial, 30*time.Second)

	var logs bytes.Buffer
	err = chapters.Run(context.Background(), chapters.Config{
		Input:        fixture,
		MergeSimilar: true,
		Stdout:       &logs,
		Stderr:       &logs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readChapters(t, ff, fixture)
	if len(got) != 2 {
		t.Fatalf("got %d chapters, want 2 after merge: %+v", len(got), got)
	}
	if got[0].Name != "Same" || got[1].Name != "Other" {
		t.Errorf("names = %q, %q", got[0].Name, got[1].Name)
	}
}

func TestRun_ShiftMovesSelectedChapter(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	synthM4BWithGap(t, fixture)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	ff, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatal(err)
	}

	initial := []audio.Chapter{
		{Start: 0, Length: 10 * time.Second, Name: "A"},
		{Start: 10 * time.Second, Length: 10 * time.Second, Name: "B"},
		{Start: 20 * time.Second, Length: 10 * time.Second, Name: "C"},
	}
	setInitialChapters(t, mp4, fixture, initial, 30*time.Second)

	spec := chapters.ShiftSpec{
		Offset:  2 * time.Second,
		Indexes: []int{1},
	}
	err = chapters.Run(context.Background(), chapters.Config{
		Input:  fixture,
		Shift:  &spec,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readChapters(t, ff, fixture)
	if len(got) != 3 {
		t.Fatalf("got %d chapters", len(got))
	}
	if diff := absDur(got[1].Start - 12*time.Second); diff > 100*time.Millisecond {
		t.Errorf("B Start = %v, want ~12s", got[1].Start)
	}
}

func TestRun_OutputFileExportsSidecar(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	synthM4BWithGap(t, fixture)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	initial := []audio.Chapter{
		{Start: 0, Length: 15 * time.Second, Name: "First"},
		{Start: 15 * time.Second, Length: 15 * time.Second, Name: "Second"},
	}
	setInitialChapters(t, mp4, fixture, initial, 30*time.Second)

	out := filepath.Join(dir, "out.txt")
	err = chapters.Run(context.Background(), chapters.Config{
		Input:      fixture,
		OutputFile: out,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{"First", "Second", "00:00:00.000", "00:00:15.000"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("sidecar missing %q\ncontents:\n%s", want, body)
		}
	}
}

func TestRun_OutputFileRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	synthM4BWithGap(t, fixture)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	setInitialChapters(t, mp4, fixture, []audio.Chapter{{Start: 0, Length: 30 * time.Second, Name: "Only"}}, 30*time.Second)

	out := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(out, []byte("preexisting"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = chapters.Run(context.Background(), chapters.Config{
		Input:      fixture,
		OutputFile: out,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected overwrite refusal, got nil")
	}
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

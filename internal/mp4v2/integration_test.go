//go:build integration

package mp4v2_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	stdexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// synthM4B produces a short silent .m4b via ffmpeg for the integration
// fixture. Kept stereo + AAC since that's the shape the real tool emits.
func synthM4B(t *testing.T, path string, duration time.Duration) {
	t.Helper()
	cmd := stdexec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
		"-t", formatSeconds(duration),
		"-c:a", "aac", "-b:a", "64k",
		"-f", "mp4",
		path,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to synthesize m4b fixture: %v", err)
	}
}

func formatSeconds(d time.Duration) string {
	return time.Duration(d).String() // "30s" style also works with ffmpeg -t
}

func writePNGFixture(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 64), uint8(y * 64), 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
}

func TestPipeline_WriteChaptersTagsCoverAndReadBack(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "book.m4b")
	cover := filepath.Join(dir, "cover.png")
	synthM4B(t, fixture, 30*time.Second)
	writePNGFixture(t, cover)

	mp4, err := mp4v2.NewClient()
	if err != nil {
		t.Fatalf("mp4v2.NewClient: %v", err)
	}
	if !mp4.SupportsSortNames {
		t.Logf("note: mp4tags does not advertise sortname support; sort fields will be skipped by the writer")
	}

	ff, err := ffmpeg.NewClient("")
	if err != nil {
		t.Fatalf("ffmpeg.NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- Chapters ---
	chapters := []audio.Chapter{
		{Start: 0, Length: 10 * time.Second, Name: "Prologue"},
		{Start: 10 * time.Second, Length: 10 * time.Second, Name: "Chapter 1"},
		{Start: 20 * time.Second, Length: 10 * time.Second, Name: "Chapter 2: Finale"},
	}
	if err := mp4.WriteChapters(ctx, fixture, chapters, 30*time.Second); err != nil {
		t.Fatalf("WriteChapters: %v", err)
	}

	// Sidecar must have been cleaned up.
	if _, err := os.Stat(filepath.Join(dir, "book.chapters.txt")); !os.IsNotExist(err) {
		t.Errorf("sidecar should have been removed after WriteChapters, stat err: %v", err)
	}

	// --- Tags ---
	when := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	tag := &audio.Tag{
		Title:        "Pipeline Test",
		Album:        "Integration Suite",
		Artist:       "Narrator McTest",
		AlbumArtist:  "Author McWrite",
		Writer:       "Author McWrite",
		Genre:        "Audiobook",
		Year:         2024,
		Track:        1,
		Tracks:       1,
		Description:  "A fixture book",
		MediaType:    audio.MediaTypeAudioBook,
		PurchaseDate: when,
		SortTitle:    "Pipeline Test, The",
	}
	if err := mp4.WriteTags(ctx, fixture, tag); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}

	// --- Cover ---
	if err := mp4.AddCover(ctx, fixture, cover); err != nil {
		t.Fatalf("AddCover: %v", err)
	}
	n, err := mp4.ListCovers(ctx, fixture)
	if err != nil {
		t.Fatalf("ListCovers: %v", err)
	}
	if n != 1 {
		t.Errorf("ListCovers = %d, want 1", n)
	}

	// --- Probe back ---
	dur, err := mp4.ProbeDuration(ctx, fixture)
	if err != nil {
		t.Fatalf("mp4info ProbeDuration: %v", err)
	}
	if diff := math.Abs(dur.Seconds() - 30); diff > 0.5 {
		t.Errorf("mp4info duration = %v, want ~30s (diff %.3fs)", dur, diff)
	}

	readTag, err := ff.ReadFFMetadata(ctx, fixture)
	if err != nil {
		t.Fatalf("ReadFFMetadata: %v", err)
	}
	if readTag.Title != tag.Title {
		t.Errorf("Title = %q, want %q", readTag.Title, tag.Title)
	}
	if readTag.Album != tag.Album {
		t.Errorf("Album = %q, want %q", readTag.Album, tag.Album)
	}
	if readTag.Artist != tag.Artist {
		t.Errorf("Artist = %q, want %q", readTag.Artist, tag.Artist)
	}
	if readTag.Year != tag.Year {
		t.Errorf("Year = %d, want %d", readTag.Year, tag.Year)
	}
	if len(readTag.Chapters) != len(chapters) {
		t.Fatalf("got %d chapters, want %d", len(readTag.Chapters), len(chapters))
	}
	for i := range chapters {
		if diff := absDur(readTag.Chapters[i].Start - chapters[i].Start); diff > 100*time.Millisecond {
			t.Errorf("chapter[%d] Start = %v, want %v", i, readTag.Chapters[i].Start, chapters[i].Start)
		}
		if readTag.Chapters[i].Name != chapters[i].Name {
			t.Errorf("chapter[%d] Name = %q, want %q", i, readTag.Chapters[i].Name, chapters[i].Name)
		}
	}

	// --- Round-trip cover ---
	extracted, err := mp4.ExtractCover(ctx, fixture, 0)
	if err != nil {
		t.Fatalf("ExtractCover: %v", err)
	}
	if extracted == "" {
		t.Fatal("ExtractCover returned empty path")
	}
	info, err := os.Stat(extracted)
	if err != nil {
		t.Fatalf("stat extracted cover: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("extracted cover %s is empty", extracted)
	}

	// --- Remove flow ---
	if err := mp4.RemoveCover(ctx, fixture); err != nil {
		t.Fatalf("RemoveCover: %v", err)
	}
	if n, _ := mp4.ListCovers(ctx, fixture); n != 0 {
		t.Errorf("after RemoveCover, ListCovers = %d, want 0", n)
	}
	if err := mp4.RemoveChapters(ctx, fixture); err != nil {
		t.Fatalf("RemoveChapters: %v", err)
	}
	readBack2, err := ff.ReadFFMetadata(ctx, fixture)
	if err != nil {
		t.Fatalf("re-read after remove: %v", err)
	}
	if len(readBack2.Chapters) != 0 {
		t.Errorf("after RemoveChapters, got %d chapters", len(readBack2.Chapters))
	}
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

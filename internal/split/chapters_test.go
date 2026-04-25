package split

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestFixedLengthChapters_ExactMultiple(t *testing.T) {
	got := fixedLengthChapters(30*time.Second, 10*time.Second)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].Start != 0 || got[0].Length != 10*time.Second {
		t.Errorf("ch0 = %+v", got[0])
	}
	if got[2].Start != 20*time.Second || got[2].Length != 10*time.Second {
		t.Errorf("ch2 = %+v", got[2])
	}
	for i, ch := range got {
		if ch.Name == "" {
			t.Errorf("ch%d unnamed", i)
		}
	}
}

func TestFixedLengthChapters_RemainderTrimmed(t *testing.T) {
	got := fixedLengthChapters(25*time.Second, 10*time.Second)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	last := got[2]
	if last.Start != 20*time.Second || last.Length != 5*time.Second {
		t.Errorf("trailing chapter = %+v, want start=20s len=5s", last)
	}
}

func TestFixedLengthChapters_ZeroOrNegative(t *testing.T) {
	if got := fixedLengthChapters(0, time.Second); got != nil {
		t.Errorf("zero total should yield nil, got %v", got)
	}
	if got := fixedLengthChapters(time.Second, 0); got != nil {
		t.Errorf("zero length should yield nil, got %v", got)
	}
}

func TestReindexChapters(t *testing.T) {
	chs := []audio.Chapter{
		{Name: "Foreword"}, {Name: "Chapter One"}, {Name: "Epilogue"},
	}
	ReindexChapters(chs)
	for i, want := range []string{"1", "2", "3"} {
		if chs[i].Name != want {
			t.Errorf("ch[%d] name = %q, want %q", i, chs[i].Name, want)
		}
	}
}

func TestFillTrailingLength(t *testing.T) {
	chs := []audio.Chapter{
		{Start: 0, Length: 10 * time.Second},
		{Start: 10 * time.Second, Length: 5 * time.Second},
		{Start: 15 * time.Second}, // Length zero
	}
	FillTrailingLength(chs, 25*time.Second)
	if chs[2].Length != 10*time.Second {
		t.Errorf("trailing = %v, want 10s", chs[2].Length)
	}
}

func TestReadChaptersTxt_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.chapters.txt")
	body := "## total-duration: 00:00:25.000\n00:00:00.000 Intro\n00:00:10.000 Body\n00:00:15.000 Outro\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	chs, err := readChaptersTxt(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 3 {
		t.Fatalf("got %d chapters, want 3", len(chs))
	}
	if chs[0].Length != 10*time.Second || chs[1].Length != 5*time.Second {
		t.Errorf("derived lengths wrong: %+v", chs)
	}
	if chs[2].Length != 0 {
		t.Errorf("last length should be zero (orchestrator fills): %v", chs[2].Length)
	}
}

func TestReadSidecarChaptersTxt_FoundAndMissing(t *testing.T) {
	dir := t.TempDir()
	audio := filepath.Join(dir, "book.m4b")
	if err := os.WriteFile(audio, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSidecarChaptersTxt(audio); ok {
		t.Errorf("missing sidecar should yield !ok")
	}
	side := filepath.Join(dir, "book.chapters.txt")
	body := "00:00:00.000 One\n00:00:05.000 Two\n"
	if err := os.WriteFile(side, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	chs, ok := readSidecarChaptersTxt(audio)
	if !ok || len(chs) != 2 {
		t.Errorf("ok=%v chs=%v", ok, chs)
	}
}

func TestResolve_Priority_FixedLength_BeatsEverything(t *testing.T) {
	dir := t.TempDir()
	audio := filepath.Join(dir, "book.m4b")
	side := filepath.Join(dir, "book.chapters.txt")
	_ = os.WriteFile(audio, []byte("x"), 0o644)
	_ = os.WriteFile(side, []byte("00:00:00.000 Sidecar\n"), 0o644)

	got, err := ChapterSource{FixedLength: 5 * time.Second}.Resolve(
		context.Background(), audio, 15*time.Second, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expected fixed-length to override sidecar, got %d", len(got))
	}
}

func TestResolve_NoSourceReturnsErr(t *testing.T) {
	dir := t.TempDir()
	audio := filepath.Join(dir, "no-meta.m4b")
	_ = os.WriteFile(audio, []byte("x"), 0o644)

	_, err := ChapterSource{}.Resolve(context.Background(), audio, 0, nil, nil)
	if err == nil {
		t.Errorf("expected ErrNoChapters, got nil")
	}
}

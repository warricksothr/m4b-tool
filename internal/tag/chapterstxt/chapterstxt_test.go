package chapterstxt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_FromAudioPath(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "book.m4b")
	sidecar := filepath.Join(dir, "book.chapters.txt")
	body := "00:00:00.000 Prologue\n00:01:00.000 One\n"
	if err := os.WriteFile(sidecar, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(Options{AudioPath: audioPath}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 2 {
		t.Fatalf("got %d chapters", len(out.Chapters))
	}
	if out.Chapters[1].Start != time.Minute {
		t.Errorf("second chapter Start = %v", out.Chapters[1].Start)
	}
}

func TestImprove_OverrideWins(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "custom.txt")
	if err := os.WriteFile(override, []byte("00:00:00.000 Only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also stand up a default sidecar to prove the override is taken.
	audioPath := filepath.Join(dir, "book.m4b")
	other := filepath.Join(dir, "book.chapters.txt")
	if err := os.WriteFile(other, []byte("00:00:00.000 WrongOne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := New(Options{AudioPath: audioPath, Override: override}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 1 || out.Chapters[0].Name != "Only" {
		t.Errorf("override not honored: %+v", out.Chapters)
	}
}

func TestImprove_MissingSidecarNoOp(t *testing.T) {
	out, err := New(Options{AudioPath: filepath.Join(t.TempDir(), "book.m4b")}).
		Improve(context.Background(), audio.Tag{Title: "kept"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 0 {
		t.Errorf("got chapters: %+v", out.Chapters)
	}
	if out.Title != "kept" {
		t.Errorf("Title clobbered: %q", out.Title)
	}
}

func TestImprove_OverwritesExistingChapters(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "book.m4b")
	sidecar := filepath.Join(dir, "book.chapters.txt")
	if err := os.WriteFile(sidecar, []byte("00:00:00.000 New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := audio.Tag{Chapters: []audio.Chapter{{Name: "Old"}, {Name: "Older"}}}
	out, err := New(Options{AudioPath: audioPath}).Improve(context.Background(), existing, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 1 || out.Chapters[0].Name != "New" {
		t.Errorf("existing chapters not replaced: %+v", out.Chapters)
	}
}

func TestImprove_EmptyOptionsNoOp(t *testing.T) {
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 0 {
		t.Errorf("got %v", out.Chapters)
	}
}

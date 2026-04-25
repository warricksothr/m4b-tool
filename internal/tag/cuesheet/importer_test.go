package cuesheet

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

const sampleCue = `PERFORMER "Narrator"
TITLE "Book"
TRACK 01 AUDIO
  TITLE "One"
  INDEX 01 00:00:00
TRACK 02 AUDIO
  TITLE "Two"
  INDEX 01 01:30:00
`

func writeCue(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "book.cue")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImporter_NoPathNoOp(t *testing.T) {
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{Title: "kept"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "kept" || len(out.Chapters) != 0 {
		t.Errorf("got %+v", out)
	}
}

func TestImporter_MergesScalarFieldsMissing(t *testing.T) {
	path := writeCue(t, t.TempDir(), sampleCue)
	in := audio.Tag{Artist: "already set"}
	out, err := New(Options{Path: path}).Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Artist != "already set" {
		t.Errorf("Artist clobbered: %q", out.Artist)
	}
	if out.Album != "Book" {
		t.Errorf("Album = %q", out.Album)
	}
}

func TestImporter_ChaptersOverwrite(t *testing.T) {
	path := writeCue(t, t.TempDir(), sampleCue)
	in := audio.Tag{Chapters: []audio.Chapter{{Name: "existing"}}}
	out, err := New(Options{Path: path}).Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 2 || out.Chapters[0].Name != "One" {
		t.Errorf("chapters not replaced: %+v", out.Chapters)
	}
	if out.Chapters[1].Start != 90*time.Second {
		t.Errorf("ch1 start = %v", out.Chapters[1].Start)
	}
}

func TestImporter_MissingFileNoOp(t *testing.T) {
	out, err := New(Options{Path: "/no/such/file.cue"}).Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 0 {
		t.Errorf("unexpected chapters: %+v", out.Chapters)
	}
}

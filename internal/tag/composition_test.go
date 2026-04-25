package tag_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/tag"
	"github.com/warricksothr/m4b-tool/internal/tag/chapterstxt"
	"github.com/warricksothr/m4b-tool/internal/tag/cover"
	"github.com/warricksothr/m4b-tool/internal/tag/description"
	"github.com/warricksothr/m4b-tool/internal/tag/equate"
	"github.com/warricksothr/m4b-tool/internal/tag/ffmetadata"
	"github.com/warricksothr/m4b-tool/internal/tag/filetracks"
	"github.com/warricksothr/m4b-tool/internal/tag/silencealign"
)

// TestComposition_MergeOrder exercises a realistic merge-style ordering
// through the Composite runner, confirming that each importer's output
// is picked up by the next one in the chain.
func TestComposition_MergeOrder(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "book.m4b")

	// Sidecars
	mustWrite(t, filepath.Join(dir, "ffmetadata.txt"),
		";FFMETADATA1\nartist=Narrator\ngenre=Audiobook\n")
	mustWrite(t, filepath.Join(dir, "description.txt"),
		"Short desc.\n\nThe longer form of the description goes here.\n")
	mustWrite(t, filepath.Join(dir, "cover.jpg"), []byte{0xff, 0xd8})
	mustWrite(t, filepath.Join(dir, "book.chapters.txt"),
		"00:00:00.000 Hand-tuned One\n00:00:10.000 Hand-tuned Two\n")

	tracks := []filetracks.Track{
		{Path: filepath.Join(dir, "part01.mp3"), Duration: 10 * time.Second, Title: "auto 1"},
		{Path: filepath.Join(dir, "part02.mp3"), Duration: 20 * time.Second, Title: "auto 2"},
	}
	silences := []audio.Silence{{Start: 9 * time.Second, Length: 2 * time.Second}}

	c := &tag.Composite{
		Importers: []tag.Importer{
			filetracks.New(filetracks.Options{Tracks: tracks}),
			chapterstxt.New(chapterstxt.Options{AudioPath: audioPath}),
			ffmetadata.New(ffmetadata.Options{}),
			cover.New(cover.Options{}),
			description.New(description.Options{}),
			equate.New(equate.Options{
				Specs:  []string{"artist,albumartist"},
				Logger: func(string, ...any) {},
			}),
			silencealign.New(silencealign.Options{
				Silences: silences,
				Total:    30 * time.Second,
			}),
		},
	}

	out, err := c.Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatalf("Improve: %v", err)
	}

	// filetracks built two chapters; chapters-txt overwrote them.
	if len(out.Chapters) != 2 {
		t.Fatalf("got %d chapters", len(out.Chapters))
	}
	if out.Chapters[0].Name != "Hand-tuned One" {
		t.Errorf("chapters-txt did not overwrite: %+v", out.Chapters)
	}
	// Silence alignment should snap the 10s boundary to the 10s midpoint.
	if out.Chapters[1].Start != 10*time.Second {
		t.Errorf("ch1 Start = %v, want 10s", out.Chapters[1].Start)
	}

	// ffmetadata filled missing scalars.
	if out.Artist != "Narrator" {
		t.Errorf("Artist = %q", out.Artist)
	}
	// equate propagated Artist to AlbumArtist.
	if out.AlbumArtist != "Narrator" {
		t.Errorf("AlbumArtist = %q", out.AlbumArtist)
	}
	// cover discovered.
	if out.CoverPath == "" {
		t.Errorf("CoverPath not set")
	}
	// description filled both fields.
	if out.Description != "Short desc." {
		t.Errorf("Description = %q", out.Description)
	}
	if out.LongDescription == "" {
		t.Errorf("LongDescription empty")
	}
}

func TestComposition_DisableSkipsImporter(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ffmetadata.txt"),
		";FFMETADATA1\ntitle=FromSidecar\n")
	c := &tag.Composite{
		Importers: []tag.Importer{ffmetadata.New(ffmetadata.Options{})},
		Disable:   tag.NamesSet([]string{"ffmetadata"}),
	}
	out, err := c.Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "" {
		t.Errorf("ffmetadata ran despite being disabled: %q", out.Title)
	}
}

func mustWrite(t *testing.T, path string, data any) {
	t.Helper()
	var b []byte
	switch v := data.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		t.Fatalf("unsupported data type %T", data)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

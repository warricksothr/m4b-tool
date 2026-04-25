package equate

import (
	"context"
	"strings"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_SingleSpec(t *testing.T) {
	in := audio.Tag{Artist: "Terry Pratchett"}
	out, err := New(Options{
		Specs: []string{"artist,albumartist,sortartist"},
	}).Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlbumArtist != "Terry Pratchett" {
		t.Errorf("AlbumArtist = %q", out.AlbumArtist)
	}
	if out.SortArtist != "Terry Pratchett" {
		t.Errorf("SortArtist = %q", out.SortArtist)
	}
	if out.Artist != "Terry Pratchett" {
		t.Errorf("source mutated: %q", out.Artist)
	}
}

func TestImprove_OverwritesExistingTargets(t *testing.T) {
	in := audio.Tag{Artist: "Real", AlbumArtist: "Old"}
	out, err := New(Options{Specs: []string{"artist,albumartist"}}).
		Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlbumArtist != "Real" {
		t.Errorf("target not overwritten: %q", out.AlbumArtist)
	}
}

func TestImprove_EmptySourceSilentlySkipped(t *testing.T) {
	out, err := New(Options{Specs: []string{"artist,albumartist"}}).
		Improve(context.Background(), audio.Tag{AlbumArtist: "kept"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlbumArtist != "kept" {
		t.Errorf("AlbumArtist = %q", out.AlbumArtist)
	}
}

func TestImprove_SingleFieldIgnored(t *testing.T) {
	out, err := New(Options{Specs: []string{"artist"}}).
		Improve(context.Background(), audio.Tag{Artist: "X"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Artist != "X" {
		t.Errorf("Artist mutated: %q", out.Artist)
	}
}

func TestImprove_UnknownSourceWarns(t *testing.T) {
	var warnings []string
	out, err := New(Options{
		Specs: []string{"bogusfield,albumartist"},
		Logger: func(f string, args ...any) {
			warnings = append(warnings, f)
		},
	}).Improve(context.Background(), audio.Tag{AlbumArtist: "kept"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlbumArtist != "kept" {
		t.Errorf("AlbumArtist mutated on unknown source: %q", out.AlbumArtist)
	}
	if len(warnings) == 0 {
		t.Error("expected warning for unknown source")
	}
}

func TestImprove_UnknownTargetWarns(t *testing.T) {
	var warnings []string
	_, err := New(Options{
		Specs: []string{"artist,bogustarget"},
		Logger: func(f string, args ...any) {
			warnings = append(warnings, f)
		},
	}).Improve(context.Background(), audio.Tag{Artist: "X"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unknown target") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestImprove_FieldNameNormalization(t *testing.T) {
	// Album-Artist and album_artist and albumartist all map to the same field.
	in := audio.Tag{Artist: "A"}
	out, err := New(Options{Specs: []string{"Artist,album-artist,album_artist,Sort-Artist"}}).
		Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlbumArtist != "A" {
		t.Errorf("AlbumArtist = %q", out.AlbumArtist)
	}
	if out.SortArtist != "A" {
		t.Errorf("SortArtist = %q", out.SortArtist)
	}
}

func TestImprove_NumericFieldsStringify(t *testing.T) {
	in := audio.Tag{Year: 2024}
	out, err := New(Options{Specs: []string{"year,track"}}).
		Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Track != 2024 {
		t.Errorf("Track = %d, want 2024", out.Track)
	}
}

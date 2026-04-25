package ffmetadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_MissingSidecarNoOp(t *testing.T) {
	imp := New(Options{})
	out, err := imp.Improve(context.Background(), audio.Tag{Title: "keep"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "keep" {
		t.Errorf("got %+v", out)
	}
}

func TestImprove_ReadsSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFilename)
	body := ";FFMETADATA1\ntitle=From Sidecar\nartist=N\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := New(Options{})
	out, err := imp.Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "From Sidecar" {
		t.Errorf("Title = %q", out.Title)
	}
	if out.Artist != "N" {
		t.Errorf("Artist = %q", out.Artist)
	}
}

func TestImprove_MergeMissingPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFilename)
	body := ";FFMETADATA1\ntitle=Override\nalbum=Added\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := New(Options{})
	out, err := imp.Improve(context.Background(), audio.Tag{Title: "Kept"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Kept" {
		t.Errorf("existing title overwritten: %q", out.Title)
	}
	if out.Album != "Added" {
		t.Errorf("missing field not filled: %q", out.Album)
	}
}

func TestImprove_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "weird-name.txt")
	if err := os.WriteFile(custom, []byte(";FFMETADATA1\ntitle=Custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := New(Options{Path: custom})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "/does/not/matter")
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Custom" {
		t.Errorf("Title = %q", out.Title)
	}
}

func TestImprove_MalformedIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFilename)
	if err := os.WriteFile(path, []byte("not-a-key-value-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := New(Options{})
	if _, err := imp.Improve(context.Background(), audio.Tag{}, dir); err == nil {
		t.Error("expected parse error")
	}
}

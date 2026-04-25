package description

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func writeSidecar(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, DefaultFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImprove_SinglePara(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, "A one-paragraph description.\n")
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "A one-paragraph description." {
		t.Errorf("Description = %q", out.Description)
	}
	if out.LongDescription != "A one-paragraph description." {
		t.Errorf("LongDescription = %q", out.LongDescription)
	}
}

func TestImprove_MultipleParagraphs(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, "First paragraph.\n\nSecond paragraph with\nmultiple lines.\n\nThird.\n")
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "First paragraph." {
		t.Errorf("Description = %q", out.Description)
	}
	if out.LongDescription != "First paragraph.\n\nSecond paragraph with\nmultiple lines.\n\nThird." {
		t.Errorf("LongDescription = %q", out.LongDescription)
	}
}

func TestImprove_CRLF(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, "Short.\r\n\r\nLong body.\r\n")
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "Short." {
		t.Errorf("Description = %q", out.Description)
	}
	if out.LongDescription != "Short.\n\nLong body." {
		t.Errorf("LongDescription = %q", out.LongDescription)
	}
}

func TestImprove_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, "New\n")
	in := audio.Tag{Description: "kept", LongDescription: "kept long"}
	out, err := New(Options{}).Improve(context.Background(), in, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "kept" || out.LongDescription != "kept long" {
		t.Errorf("got %+v", out)
	}
}

func TestImprove_MissingSidecar(t *testing.T) {
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "" {
		t.Errorf("got %+v", out)
	}
}

func TestImprove_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, dir, "  \n\n\t\n")
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Description != "" || out.LongDescription != "" {
		t.Errorf("whitespace-only file leaked content: %+v", out)
	}
}

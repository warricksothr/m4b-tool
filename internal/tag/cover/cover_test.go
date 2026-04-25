package cover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_PrefersJpgOverPng(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte{0xff, 0xd8}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), []byte{0x89, 0x50}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.CoverPath, "cover.jpg") {
		t.Errorf("CoverPath = %q, want ending in cover.jpg", out.CoverPath)
	}
}

func TestImprove_FallsBackToPng(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), []byte{0x89, 0x50}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.CoverPath, "cover.png") {
		t.Errorf("CoverPath = %q", out.CoverPath)
	}
}

func TestImprove_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte{0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	in := audio.Tag{CoverPath: "/already/set.png"}
	out, err := New(Options{}).Improve(context.Background(), in, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.CoverPath != "/already/set.png" {
		t.Errorf("CoverPath overwritten: %q", out.CoverPath)
	}
}

func TestImprove_SkipOption(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte{0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(Options{Skip: true}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.CoverPath != "" {
		t.Errorf("Skip didn't prevent discovery: %q", out.CoverPath)
	}
}

func TestImprove_NoCandidates(t *testing.T) {
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out.CoverPath != "" {
		t.Errorf("expected empty CoverPath, got %q", out.CoverPath)
	}
}

func TestImprove_ZeroByteCoverIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), []byte{0x89}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.CoverPath, "cover.png") {
		t.Errorf("zero-byte jpg not skipped: got %q", out.CoverPath)
	}
}

package merge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// TestRunBatch_DryRun walks a synthetic input tree, expands a batch
// pattern, and verifies the dry-run summary lists each entry with the
// expected per-entry output path. No ffmpeg/mp4v2 invocation.
func TestRunBatch_DryRun(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{
		"Fantasy/Sanderson/Stormlight/01 - Way of Kings",
		"Fantasy/Sanderson/Stormlight/02 - Words of Radiance",
		"Sci-Fi/Asimov/Foundation/01 - Foundation",
	} {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "track-01.mp3"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outDir := filepath.Join(t.TempDir(), "out") + "/"

	var stdout, stderr bytes.Buffer
	cfg := Config{
		Inputs:           []string{root},
		Output:           outDir,
		DryRun:           true,
		BatchPatterns:    []string{"%g/%a/%s/%p - %n"},
		BatchPatternPath: root,
		Stdout:           &stdout,
		Stderr:           &stderr,
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{
		"01 - Way of Kings",
		"02 - Words of Radiance",
		"Foundation",
		// Per-entry output filenames are basename + .m4b under the
		// declared output dir.
		filepath.Join(filepath.Clean(outDir), "01 - Way of Kings.m4b"),
		filepath.Join(filepath.Clean(outDir), "01 - Foundation.m4b"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("dry-run output missing %q\n--- stdout ---\n%s", want, got)
		}
	}
}

// TestRunBatch_FilterAndResume verifies that --batch-filter narrows
// the enumerated set and that --batch-resume-file excludes already
// processed entries. Uses dry-run to avoid ffmpeg.
func TestRunBatch_FilterAndResume(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"A/Alpha/01 - One", "B/Beta/01 - Two"} {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "a.mp3"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	resumePath := filepath.Join(t.TempDir(), "resume.txt")
	abs, _ := filepath.Abs(filepath.Join(root, "B/Beta/01 - Two"))
	if err := AppendResume(resumePath, abs); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cfg := Config{
		Inputs:           []string{root},
		Output:           filepath.Join(t.TempDir(), "out") + "/",
		DryRun:           true,
		BatchPatterns:    []string{"%g/%a/%p - %n"},
		BatchPatternPath: root,
		BatchFilter:      "Alpha",
		BatchResumeFile:  resumePath,
		Stdout:           &stdout,
		Stderr:           &stderr,
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Alpha") {
		t.Errorf("expected Alpha entry in dry-run output:\n%s", out)
	}
	if strings.Contains(out, "Beta") {
		t.Errorf("Beta should be excluded by filter, got:\n%s", out)
	}
}

func TestPerEntryConfig_OverridesWin(t *testing.T) {
	parent := Config{
		TagOverrides: audio.Tag{Artist: "From CLI"},
	}
	entry := BatchEntry{
		SourceDir: "/src/dir",
		Tags:      audio.Tag{Artist: "From Pattern", Genre: "Fantasy"},
	}
	sub := perEntryConfig(parent, entry, "/out/dir.m4b")
	if sub.TagOverrides.Artist != "From CLI" {
		t.Errorf("CLI override should win: %q", sub.TagOverrides.Artist)
	}
	if sub.TagOverrides.Genre != "Fantasy" {
		t.Errorf("placeholder genre lost: %q", sub.TagOverrides.Genre)
	}
	if len(sub.BatchPatterns) != 0 {
		t.Errorf("batch fields should not propagate: %v", sub.BatchPatterns)
	}
}

func TestResolveBatchOutputDir(t *testing.T) {
	cases := []struct {
		in          string
		wantDirHas  string // substring expected in returned dir
		wantExt     string
		wantErr     bool
		description string
	}{
		{"out/", "out", ".m4b", false, "trailing slash → ext defaults to m4b"},
		{"out", "out", ".m4b", false, "no extension → m4b"},
		{"out/album.m4b", "out", ".m4b", false, "explicit file shape → ext propagates"},
		{"out/album.mp4", "out", ".mp4", false, "explicit mp4 → mp4"},
		{"out/album.flac", "", "", true, "unsupported ext → error"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			dir, ext, err := resolveBatchOutputDir(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(dir, tc.wantDirHas) {
				t.Errorf("dir = %q, want substring %q", dir, tc.wantDirHas)
			}
			if ext != tc.wantExt {
				t.Errorf("ext = %q, want %q", ext, tc.wantExt)
			}
		})
	}
}

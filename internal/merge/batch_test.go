package merge

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestParseBatchPattern_Errors(t *testing.T) {
	cases := map[string]string{
		"empty":     "",
		"trailing%": "%a/%",
		"unknown":   "%z/%n",
	}
	for name, pat := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseBatchPattern(pat); err == nil {
				t.Errorf("expected error for %q", pat)
			}
		})
	}
}

func TestBatchPattern_Match(t *testing.T) {
	cases := []struct {
		name string
		pat  string
		path string // forward-slash form; converted to OS sep before matching
		want audio.Tag
		ok   bool
	}{
		{
			name: "simple two-segment",
			pat:  "%a/%n",
			path: "Sanderson/Way of Kings",
			want: audio.Tag{Artist: "Sanderson", Title: "Way of Kings"},
			ok:   true,
		},
		{
			name: "literal separator inside segment",
			pat:  "%g/%a/%s/%p - %n",
			path: "Fantasy/Sanderson/Stormlight/01 - Way of Kings",
			want: audio.Tag{Genre: "Fantasy", Artist: "Sanderson", Series: "Stormlight", SeriesPart: "01", Title: "Way of Kings"},
			ok:   true,
		},
		{
			name: "trailing slash on pattern is tolerated",
			pat:  "%a/%n/",
			path: "Sanderson/Way of Kings",
			want: audio.Tag{Artist: "Sanderson", Title: "Way of Kings"},
			ok:   true,
		},
		{
			name: "wrong depth → no match",
			pat:  "%a/%n",
			path: "Sanderson/Stormlight/Way of Kings",
			ok:   false,
		},
		{
			name: "literal mismatch",
			pat:  "%p - %n",
			path: "01_Way of Kings",
			ok:   false,
		},
		{
			name: "literal percent",
			pat:  "%a/100%%/%n",
			path: "Sanderson/100%/Way of Kings",
			want: audio.Tag{Artist: "Sanderson", Title: "Way of Kings"},
			ok:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseBatchPattern(tc.pat)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.pat, err)
			}
			osPath := filepath.FromSlash(tc.path)
			got, ok := p.Match(osPath)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (path=%q)", ok, tc.ok, osPath)
			}
			if !ok {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestEnumerateBatch(t *testing.T) {
	root := t.TempDir()
	// Layout:
	//   <root>/Fantasy/Sanderson/Stormlight/01 - Way of Kings/a.mp3
	//   <root>/Fantasy/Sanderson/Stormlight/02 - Words of Radiance/a.mp3
	//   <root>/Sci-Fi/Asimov/Foundation/01 - Foundation/a.mp3
	//   <root>/notes.txt                              (no audio anywhere up its tree)
	//   <root>/Fantasy/Sanderson/cover.jpg            (no audio next to it)
	dirs := []string{
		"Fantasy/Sanderson/Stormlight/01 - Way of Kings",
		"Fantasy/Sanderson/Stormlight/02 - Words of Radiance",
		"Sci-Fi/Asimov/Foundation/01 - Foundation",
	}
	for _, d := range dirs {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "a.mp3"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pat, err := ParseBatchPattern("%g/%a/%s/%p - %n")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := EnumerateBatch([]string{root}, EnumerateOptions{
		Patterns: []*BatchPattern{pat},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.RelPath
	}
	want := []string{
		"Fantasy/Sanderson/Stormlight/01 - Way of Kings",
		"Fantasy/Sanderson/Stormlight/02 - Words of Radiance",
		"Sci-Fi/Asimov/Foundation/01 - Foundation",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}

	// Spot-check tag extraction on the first entry.
	first := entries[0]
	wantTag := audio.Tag{
		Genre: "Fantasy", Artist: "Sanderson", Series: "Stormlight",
		SeriesPart: "01", Title: "Way of Kings",
	}
	if !reflect.DeepEqual(first.Tags, wantTag) {
		t.Errorf("tags = %+v\nwant %+v", first.Tags, wantTag)
	}
}

func TestEnumerateBatch_FilterAndResume(t *testing.T) {
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
	pat, err := ParseBatchPattern("%g/%a/%p - %n")
	if err != nil {
		t.Fatal(err)
	}

	// Filter excludes the B branch.
	entries, err := EnumerateBatch([]string{root}, EnumerateOptions{
		Patterns: []*BatchPattern{pat},
		Filter:   "Alpha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Tags.Genre != "A" {
		t.Errorf("filter result wrong: %+v", entries)
	}

	// Resume excludes a specific absolute path.
	abs, _ := filepath.Abs(filepath.Join(root, "A/Alpha/01 - One"))
	entries, err = EnumerateBatch([]string{root}, EnumerateOptions{
		Patterns: []*BatchPattern{pat},
		Resume:   map[string]bool{abs: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Tags.Genre != "B" {
		t.Errorf("resume result wrong: %+v", entries)
	}
}

func TestLoadResumeFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resume.txt")

	if err := AppendResume(path, "/abs/one"); err != nil {
		t.Fatal(err)
	}
	if err := AppendResume(path, "/abs/two"); err != nil {
		t.Fatal(err)
	}

	set, err := LoadResumeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !set["/abs/one"] || !set["/abs/two"] || len(set) != 2 {
		t.Errorf("set = %v", set)
	}

	// Missing file → empty set, no error.
	set2, err := LoadResumeFile(filepath.Join(dir, "missing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(set2) != 0 {
		t.Errorf("missing file should give empty set, got %v", set2)
	}
}

func TestLoadResumeFile_IgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resume.txt")
	body := "# header comment\n\n/path/one\n\n   # indented comment after trim\n/path/two\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	set, err := LoadResumeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"/path/one": true, "/path/two": true}
	if !reflect.DeepEqual(set, want) {
		t.Errorf("got %v\nwant %v", set, want)
	}
}

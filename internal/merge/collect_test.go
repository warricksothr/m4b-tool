package merge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mustWrite(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectInputs_SortedDirectoryScan(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "02.mp3"), "x")
	mustWrite(t, filepath.Join(dir, "01.mp3"), "x")
	mustWrite(t, filepath.Join(dir, "03.mp3"), "x")
	mustWrite(t, filepath.Join(dir, "ignore.txt"), "x")

	got, err := collectInputs([]string{dir}, DefaultExtensions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "01.mp3"),
		filepath.Join(dir, "02.mp3"),
		filepath.Join(dir, "03.mp3"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectInputs_RecursiveWalk(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a/one.m4a"), "x")
	mustWrite(t, filepath.Join(dir, "b/two.m4a"), "x")
	mustWrite(t, filepath.Join(dir, "b/three.m4a"), "x")

	got, err := collectInputs([]string{dir}, DefaultExtensions)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	// Alphabetically sorted across the full walk.
	want := []string{
		filepath.Join(dir, "a/one.m4a"),
		filepath.Join(dir, "b/three.m4a"),
		filepath.Join(dir, "b/two.m4a"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectInputs_ExplicitFileBypassesExtensionFilter(t *testing.T) {
	// Explicit files should be included regardless of extension. Here
	// "audio.wvy" isn't in the default list but the user named it.
	dir := t.TempDir()
	oddball := filepath.Join(dir, "audio.wvy")
	mustWrite(t, oddball, "x")

	got, err := collectInputs([]string{oddball}, DefaultExtensions)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != oddball {
		t.Errorf("got %v", got)
	}
}

func TestCollectInputs_PreservesArgOrderAcrossSources(t *testing.T) {
	dir1 := t.TempDir()
	mustWrite(t, filepath.Join(dir1, "y.m4a"), "x")
	mustWrite(t, filepath.Join(dir1, "x.m4a"), "x")
	loose := filepath.Join(t.TempDir(), "loose.m4a")
	mustWrite(t, loose, "x")

	got, err := collectInputs([]string{dir1, loose}, DefaultExtensions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir1, "x.m4a"),
		filepath.Join(dir1, "y.m4a"),
		loose,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectInputs_MissingInputErrors(t *testing.T) {
	if _, err := collectInputs([]string{"/does/not/exist"}, DefaultExtensions); err == nil {
		t.Error("expected error on missing input")
	}
}

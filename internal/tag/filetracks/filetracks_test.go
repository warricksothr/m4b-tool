package filetracks

import (
	"context"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_UsesTitleWhenPresent(t *testing.T) {
	imp := New(Options{
		Tracks: []Track{
			{Path: "/a/part-01.mp3", Duration: 10 * time.Second, Title: "First"},
			{Path: "/a/part-02.mp3", Duration: 20 * time.Second, Title: "Second"},
		},
	})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 2 {
		t.Fatalf("got %d", len(out.Chapters))
	}
	if out.Chapters[0].Name != "First" || out.Chapters[1].Name != "Second" {
		t.Errorf("names = %+v", out.Chapters)
	}
	if out.Chapters[1].Start != 10*time.Second {
		t.Errorf("second Start = %v", out.Chapters[1].Start)
	}
	if out.Chapters[1].Length != 20*time.Second {
		t.Errorf("second Length = %v", out.Chapters[1].Length)
	}
}

func TestImprove_FallsBackToFilenameStem(t *testing.T) {
	imp := New(Options{
		Tracks: []Track{{Path: "/a/part-01.mp3", Duration: 10 * time.Second}},
	})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Chapters[0].Name != "part-01" {
		t.Errorf("name = %q, want part-01", out.Chapters[0].Name)
	}
}

func TestImprove_UseFilenamesOverridesTitle(t *testing.T) {
	imp := New(Options{
		UseFilenames: true,
		Tracks: []Track{
			{Path: "/a/one.mp3", Duration: time.Second, Title: "ignored"},
		},
	})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Chapters[0].Name != "one" {
		t.Errorf("name = %q, want one", out.Chapters[0].Name)
	}
}

func TestImprove_Empty(t *testing.T) {
	out, err := New(Options{}).Improve(context.Background(), audio.Tag{Title: "kept"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 0 {
		t.Errorf("unexpected chapters: %+v", out.Chapters)
	}
	if out.Title != "kept" {
		t.Errorf("Title clobbered: %q", out.Title)
	}
}

func TestImprove_GapShiftsLaterChapters(t *testing.T) {
	imp := New(Options{
		Tracks: []Track{
			{Path: "/a.m4a", Duration: 10 * time.Second, Title: "A"},
			{Path: "/b.m4a", Duration: 20 * time.Second, Title: "B"},
			{Path: "/c.m4a", Duration: 5 * time.Second, Title: "C"},
		},
		Gap: 500 * time.Millisecond,
	})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	// A starts at 0; B starts at 10s + 500ms; C starts at 30.5s + 500ms.
	if out.Chapters[0].Start != 0 {
		t.Errorf("A.Start = %v", out.Chapters[0].Start)
	}
	if out.Chapters[1].Start != 10500*time.Millisecond {
		t.Errorf("B.Start = %v, want 10.5s", out.Chapters[1].Start)
	}
	if out.Chapters[2].Start != 31*time.Second {
		t.Errorf("C.Start = %v, want 31s", out.Chapters[2].Start)
	}
}

func TestImprove_ZeroDurationAllowed(t *testing.T) {
	imp := New(Options{
		Tracks: []Track{
			{Path: "/a/zero.mp3", Duration: 0, Title: "Zero"},
			{Path: "/a/one.mp3", Duration: 10 * time.Second, Title: "One"},
		},
	})
	out, err := imp.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 2 {
		t.Fatalf("got %d", len(out.Chapters))
	}
	if out.Chapters[0].Length != 0 {
		t.Errorf("first Length = %v", out.Chapters[0].Length)
	}
}

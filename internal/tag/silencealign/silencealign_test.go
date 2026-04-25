package silencealign

import (
	"context"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestImprove_SnapsChaptersOntoSilences(t *testing.T) {
	in := audio.Tag{
		Chapters: []audio.Chapter{
			{Start: 0, Length: 0, Name: "A"},
			{Start: 30 * time.Second, Length: 0, Name: "B"},
		},
	}
	opts := Options{
		Silences: []audio.Silence{{Start: 29 * time.Second, Length: 2 * time.Second}},
		Total:    60 * time.Second,
	}
	out, err := New(opts).Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	// Silence midpoint at 30s coincides with B's start; snap leaves it
	// there but lengths are recomputed from Total.
	if out.Chapters[0].Length != 30*time.Second {
		t.Errorf("A length = %v", out.Chapters[0].Length)
	}
	if out.Chapters[1].Length != 30*time.Second {
		t.Errorf("B length = %v", out.Chapters[1].Length)
	}
}

func TestImprove_NoSilencesNoOp(t *testing.T) {
	in := audio.Tag{Chapters: []audio.Chapter{{Start: 0}, {Start: 30 * time.Second}}}
	out, err := New(Options{Total: 60 * time.Second}).Improve(context.Background(), in, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Chapters[1].Start != 30*time.Second {
		t.Errorf("chapter moved without silences: %v", out.Chapters[1].Start)
	}
}

func TestImprove_NoChaptersNoOp(t *testing.T) {
	out, err := New(Options{
		Silences: []audio.Silence{{Start: 10 * time.Second, Length: time.Second}},
		Total:    30 * time.Second,
	}).Improve(context.Background(), audio.Tag{Title: "kept"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "kept" {
		t.Errorf("title clobbered")
	}
}

package mp4v2

import (
	"reflect"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// sampleMp4ChapsList is a representative `mp4chaps -l` stdout. The
// header line and indentation match what enzo1982/mp4v2 produces in
// practice; the chapter rows include the small set of formatting
// quirks we need to handle:
//
//   - leading whitespace before "Chapter"
//   - 3-digit zero-padded index
//   - " - " separators around the timestamp
//   - quoted titles, including a title with punctuation
const sampleMp4ChapsList = `QuickTime Chapters of "/work/Some Book.m4b"
        Chapter #001 - 00:00:00.000 - "Opening Credits"
        Chapter #002 - 00:00:29.767 - "OMG. What a floor!"
        Chapter #003 - 00:09:39.197 - "1. Prologue"
        Chapter #004 - 00:31:55.923 - "I. Havana - Chapter 1"
`

func TestParseMp4ChapsList(t *testing.T) {
	got, err := parseMp4ChapsList(sampleMp4ChapsList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []audio.Chapter{
		{Start: 0, Length: 29*time.Second + 767*time.Millisecond, Name: "Opening Credits"},
		{Start: 29*time.Second + 767*time.Millisecond, Length: 9*time.Minute + 9*time.Second + 430*time.Millisecond, Name: "OMG. What a floor!"},
		{Start: 9*time.Minute + 39*time.Second + 197*time.Millisecond, Length: 22*time.Minute + 16*time.Second + 726*time.Millisecond, Name: "1. Prologue"},
		// Last chapter's Length stays zero — caller fills it via
		// FillTrailingLength once the input's total duration is known.
		{Start: 31*time.Minute + 55*time.Second + 923*time.Millisecond, Length: 0, Name: "I. Havana - Chapter 1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseMp4ChapsList_EmptyOutput(t *testing.T) {
	// File with no chapters: mp4chaps prints just the header (or
	// nothing). parseMp4ChapsList returns an empty slice, not an error
	// — callers treat zero chapters as a resolver miss.
	got, err := parseMp4ChapsList(`QuickTime Chapters of "/work/x.m4b"` + "\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d chapters, want 0: %#v", len(got), got)
	}
}

func TestParseMp4ChapsList_DedupesByStart(t *testing.T) {
	// Some files carry both QuickTime AND Nero atoms; mp4chaps then
	// prints both lists. Dedupe on Start so a chapter doesn't show up
	// twice in the resolver output.
	dual := `QuickTime Chapters of "/work/x.m4b"
        Chapter #001 - 00:00:00.000 - "Intro"
        Chapter #002 - 00:01:00.000 - "Body"
Nero Chapters of "/work/x.m4b"
        Chapter #001 - 00:00:00.000 - "Intro"
        Chapter #002 - 00:01:00.000 - "Body"
`
	got, err := parseMp4ChapsList(dual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d chapters, want 2 (post-dedupe): %#v", len(got), got)
	}
}

func TestParseMp4ChapsList_SkipsUnparseable(t *testing.T) {
	// Lines that don't match the chapter pattern (warnings, blank
	// lines, etc.) are skipped silently rather than failing the parse.
	noisy := `mp4chaps version 2.1.3
QuickTime Chapters of "/work/x.m4b"

        Chapter #001 - 00:00:00.000 - "Only Chapter"

End of chapters.
`
	got, err := parseMp4ChapsList(noisy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Only Chapter" {
		t.Errorf("got %#v, want one chapter named 'Only Chapter'", got)
	}
}

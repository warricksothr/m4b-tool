package cuesheet

import (
	"strings"
	"testing"
	"time"
)

func TestParseCueTime(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"00:00:00", 0},
		{"00:01:00", time.Second},
		{"00:00:75", 0}, // invalid, handled separately
		{"01:30:00", 90 * time.Second},
		{"00:01:37", time.Second + (37*1000/75)*time.Millisecond},
	}
	for _, tc := range tests {
		if tc.in == "00:00:75" {
			if _, err := parseCueTime(tc.in); err == nil {
				t.Errorf("parseCueTime(%q) should error on frames=75", tc.in)
			}
			continue
		}
		got, err := parseCueTime(tc.in)
		if err != nil {
			t.Errorf("parseCueTime(%q) err = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseCueTime(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParse_MinimalSheet(t *testing.T) {
	in := `PERFORMER "Narrator Name"
TITLE "The Book Title"
FILE "book.flac" FLAC
  TRACK 01 AUDIO
    TITLE "Chapter 1"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Chapter 2"
    INDEX 01 05:30:00
  TRACK 03 AUDIO
    TITLE "Chapter 3"
    INDEX 01 10:00:00
`
	sheet, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Tag.Artist != "Narrator Name" {
		t.Errorf("Artist = %q", sheet.Tag.Artist)
	}
	if sheet.Tag.Album != "The Book Title" {
		t.Errorf("Album = %q", sheet.Tag.Album)
	}
	if len(sheet.Chapters) != 3 {
		t.Fatalf("got %d chapters, want 3", len(sheet.Chapters))
	}
	if sheet.Chapters[0].Name != "Chapter 1" || sheet.Chapters[0].Start != 0 {
		t.Errorf("ch0 = %+v", sheet.Chapters[0])
	}
	if sheet.Chapters[1].Start != 330*time.Second {
		t.Errorf("ch1.Start = %v, want 5:30", sheet.Chapters[1].Start)
	}
	if sheet.Chapters[0].Length != 330*time.Second {
		t.Errorf("ch0.Length = %v, want gap to ch1", sheet.Chapters[0].Length)
	}
	if sheet.Chapters[2].Length != 0 {
		t.Errorf("last chapter should have zero Length (caller fills), got %v", sheet.Chapters[2].Length)
	}
}

func TestParse_DiscLevelFieldsIgnoreTrackScope(t *testing.T) {
	// PERFORMER inside a TRACK is the track performer, not disc Artist.
	in := `PERFORMER "Disc Artist"
TRACK 01 AUDIO
  PERFORMER "Track Performer"
  TITLE "Only"
  INDEX 01 00:00:00
`
	sheet, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Tag.Artist != "Disc Artist" {
		t.Errorf("Artist = %q", sheet.Tag.Artist)
	}
	if len(sheet.Chapters) != 1 || sheet.Chapters[0].Name != "Only" {
		t.Errorf("chapters = %+v", sheet.Chapters)
	}
}

func TestParse_DateYear(t *testing.T) {
	in := `TITLE "Book"
DATE "2024"
TRACK 01 AUDIO
  TITLE "c"
  INDEX 01 00:00:00
`
	sheet, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Tag.Year != 2024 {
		t.Errorf("Year = %d", sheet.Tag.Year)
	}
}

func TestParse_RemFileIgnored(t *testing.T) {
	// REM and FILE directives must not error or leak into Tag fields.
	in := `REM GENRE Audiobook
REM DATE 2024
FILE "book.flac" WAVE
TITLE "B"
TRACK 01 AUDIO
  TITLE "c"
  INDEX 01 00:00:00
`
	sheet, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Tag.Album != "B" {
		t.Errorf("Album = %q", sheet.Tag.Album)
	}
}

func TestParse_IgnoresIndex00(t *testing.T) {
	in := `TRACK 01 AUDIO
  TITLE "One"
  INDEX 01 00:00:00
TRACK 02 AUDIO
  TITLE "Two"
  INDEX 00 00:29:50
  INDEX 01 00:30:00
`
	sheet, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Chapters) != 2 {
		t.Fatalf("got %d chapters", len(sheet.Chapters))
	}
	if sheet.Chapters[1].Start != 30*time.Second {
		t.Errorf("ch1.Start = %v, want 30s", sheet.Chapters[1].Start)
	}
}

func TestParse_Empty(t *testing.T) {
	sheet, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Chapters) != 0 {
		t.Errorf("got %d chapters", len(sheet.Chapters))
	}
}

func TestParse_MalformedIndexErrors(t *testing.T) {
	in := `TRACK 01 AUDIO
  TITLE "c"
  INDEX 01 not-a-time
`
	if _, err := Parse(strings.NewReader(in)); err == nil {
		t.Error("expected error on bad timecode")
	}
}

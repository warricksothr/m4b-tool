package ffmpeg

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestEscapeUnescape_Roundtrip(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"has spaces",
		`equals = sign`,
		`semicolon ; hash #`,
		"line1\nline2",
		`backslash \ here`,
		"all of them: = ; # \\ \n",
	}
	for _, in := range cases {
		got := unescape(escape(in))
		if got != in {
			t.Errorf("roundtrip %q -> %q -> %q", in, escape(in), got)
		}
	}
}

func TestSplitEscapedKV(t *testing.T) {
	tests := []struct {
		line         string
		wantK, wantV string
		wantOK       bool
	}{
		{"a=b", "a", "b", true},
		{"title=Chapter 1", "title", "Chapter 1", true},
		{`weird\=key=value`, `weird\=key`, "value", true},
		{`empty=`, "empty", "", true},
		{"no-equals", "", "", false},
		{`=starts-with-eq`, "", "starts-with-eq", true},
	}
	for _, tc := range tests {
		k, v, ok := splitEscapedKV(tc.line)
		if ok != tc.wantOK || k != tc.wantK || v != tc.wantV {
			t.Errorf("splitEscapedKV(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.line, k, v, ok, tc.wantK, tc.wantV, tc.wantOK)
		}
	}
}

func TestParseFFMetadata_Empty(t *testing.T) {
	tag, err := ParseFFMetadata(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tag == nil {
		t.Fatal("nil tag")
	}
}

func TestParseFFMetadata_BasicFields(t *testing.T) {
	in := `;FFMETADATA1
title=Book Title
album=Series Name
artist=Narrator
composer=Author Name
date=2024
track=3
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Title != "Book Title" {
		t.Errorf("Title = %q", tag.Title)
	}
	if tag.Album != "Series Name" {
		t.Errorf("Album = %q", tag.Album)
	}
	if tag.Artist != "Narrator" {
		t.Errorf("Artist = %q", tag.Artist)
	}
	if tag.Writer != "Author Name" {
		t.Errorf("Writer = %q", tag.Writer)
	}
	if tag.Year != 2024 {
		t.Errorf("Year = %d", tag.Year)
	}
	if tag.Track != 3 {
		t.Errorf("Track = %d", tag.Track)
	}
}

func TestParseFFMetadata_Escapes(t *testing.T) {
	in := ";FFMETADATA1\n" +
		`title=Book \= Part \# 1` + "\n" +
		`description=has \; semi and \\ back` + "\n"
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Title != "Book = Part # 1" {
		t.Errorf("Title = %q", tag.Title)
	}
	if tag.Description != `has ; semi and \ back` {
		t.Errorf("Description = %q", tag.Description)
	}
}

func TestParseFFMetadata_MultilineValue_NewlineEscape(t *testing.T) {
	in := ";FFMETADATA1\n" +
		`description=line 1\nline 2\nline 3` + "\n"
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "line 1\nline 2\nline 3"
	if tag.Description != want {
		t.Errorf("Description = %q, want %q", tag.Description, want)
	}
}

func TestParseFFMetadata_MultilineValue_LineContinuation(t *testing.T) {
	in := ";FFMETADATA1\n" +
		"description=line 1\\\n" +
		"line 2\\\n" +
		"line 3\n"
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "line 1\nline 2\nline 3"
	if tag.Description != want {
		t.Errorf("Description = %q, want %q", tag.Description, want)
	}
}

func TestParseFFMetadata_Chapters(t *testing.T) {
	in := `;FFMETADATA1
title=Book
[CHAPTER]
TIMEBASE=1/1000
START=0
END=305000
title=Chapter 1
[CHAPTER]
TIMEBASE=1/1000
START=305000
END=610500
title=Chapter 2
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(tag.Chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(tag.Chapters))
	}
	if tag.Chapters[0].Start != 0 || tag.Chapters[0].Length != 305*time.Second {
		t.Errorf("chapter 0 = %+v", tag.Chapters[0])
	}
	if tag.Chapters[1].Start != 305*time.Second || tag.Chapters[1].Length != 305500*time.Millisecond {
		t.Errorf("chapter 1 = %+v", tag.Chapters[1])
	}
	if tag.Chapters[1].Name != "Chapter 2" {
		t.Errorf("chapter 1 name = %q", tag.Chapters[1].Name)
	}
}

func TestParseFFMetadata_ChapterWithDifferentTimebase(t *testing.T) {
	// TIMEBASE 1/1 means START/END are in seconds, not ms.
	in := `;FFMETADATA1
[CHAPTER]
TIMEBASE=1/1
START=10
END=20
title=Block
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Chapters[0].Start != 10*time.Second || tag.Chapters[0].Length != 10*time.Second {
		t.Errorf("chapter = %+v", tag.Chapters[0])
	}
}

// Regression: a real audiobook had a chapter with TIMEBASE=1/10000000
// and START≈4.1e11. The naive `count * 1e9 * num / den` order silently
// overflowed int64 and produced 7m10s instead of 11h23m33s.
func TestParseFFMetadata_FineTimebaseLargeOffset(t *testing.T) {
	in := `;FFMETADATA1
[CHAPTER]
TIMEBASE=1/10000000
START=410137300000
END=413000000000
title=Late
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := 41013730 * time.Millisecond // 11h23m33.730s
	if tag.Chapters[0].Start != want {
		t.Errorf("start = %v, want %v", tag.Chapters[0].Start, want)
	}
}

func TestParseFFMetadata_OutOfOrderChaptersPreserved(t *testing.T) {
	// Parser should not reorder; ordering is enforced by downstream passes.
	in := `;FFMETADATA1
[CHAPTER]
TIMEBASE=1/1000
START=10000
END=20000
title=Second
[CHAPTER]
TIMEBASE=1/1000
START=0
END=10000
title=First
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Chapters[0].Name != "Second" || tag.Chapters[1].Name != "First" {
		t.Errorf("order changed: %+v", tag.Chapters)
	}
}

func TestParseFFMetadata_CommentsSkipped(t *testing.T) {
	in := `;FFMETADATA1
; this is a comment
# so is this
title=Book
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Title != "Book" {
		t.Errorf("Title = %q", tag.Title)
	}
}

func TestParseFFMetadata_ExtraKeysPreserved(t *testing.T) {
	in := `;FFMETADATA1
title=Book
asin=B0ABCDEFGH
custom_key=custom value
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Extra["asin"] != "B0ABCDEFGH" {
		t.Errorf("asin = %q", tag.Extra["asin"])
	}
	if tag.Extra["custom_key"] != "custom value" {
		t.Errorf("custom_key = %q", tag.Extra["custom_key"])
	}
}

func TestParseFFMetadata_ID3FrameSortNames(t *testing.T) {
	in := `;FFMETADATA1
TSO2=Album Artist Sort
TSOC=Writer Sort
`
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.SortAlbumArtist != "Album Artist Sort" {
		t.Errorf("SortAlbumArtist = %q", tag.SortAlbumArtist)
	}
	if tag.SortWriter != "Writer Sort" {
		t.Errorf("SortWriter = %q", tag.SortWriter)
	}
}

func TestParseFFMetadata_TrackWithTotal(t *testing.T) {
	in := ";FFMETADATA1\ntrack=3/12\n"
	tag, err := ParseFFMetadata(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tag.Track != 3 {
		t.Errorf("Track = %d", tag.Track)
	}
}

func TestParseFFMetadata_MalformedLineIsError(t *testing.T) {
	in := ";FFMETADATA1\nno-equals-on-this-line\n"
	if _, err := ParseFFMetadata(strings.NewReader(in)); err == nil {
		t.Fatal("expected error on malformed line")
	}
}

func TestWriteFFMetadata_EmitsHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFFMetadata(&buf, &audio.Tag{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), ffmetadataHeader+"\n") {
		t.Errorf("output missing header, got %q", buf.String())
	}
}

func TestWriteFFMetadata_Roundtrip(t *testing.T) {
	orig := &audio.Tag{
		Title:           "The Fifth Elephant",
		Album:           "Discworld",
		Artist:          "Stephen Briggs",
		AlbumArtist:     "Terry Pratchett",
		Writer:          "Terry Pratchett",
		Genre:           "Fantasy",
		Year:            2000,
		Description:     "line 1\nline 2 with = and ; special chars",
		LongDescription: "much longer",
		Track:           5,
		MediaType:       audio.MediaTypeAudioBook, // not round-tripped via ffmetadata currently
		Grouping:        "Discworld Series",
		SortAlbumArtist: "Pratchett, Terry",
		SortWriter:      "Pratchett, Terry",
		PurchaseDate:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Chapters: []audio.Chapter{
			{Start: 0, Length: 305 * time.Second, Name: "Prologue"},
			{Start: 305 * time.Second, Length: 420500 * time.Millisecond, Name: "Chapter: One"},
			{Start: 725500 * time.Millisecond, Length: 600 * time.Second, Name: "Final"},
		},
		Extra: map[string]string{
			"asin":   "B000ABC",
			"isbn":   "978-0-061-35528-7",
			"custom": "keep me",
		},
	}

	var buf bytes.Buffer
	if err := WriteFFMetadata(&buf, orig); err != nil {
		t.Fatalf("write: %v", err)
	}
	parsed, err := ParseFFMetadata(&buf)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	// MediaType is not carried in FFMETADATA1 (no standard key). Drop it
	// from the comparison so the round-trip expectation is honest about
	// what the format preserves.
	expected := *orig
	expected.MediaType = audio.MediaTypeUnset

	if !reflect.DeepEqual(parsed, &expected) {
		t.Errorf("round-trip mismatch\nwant: %+v\n got: %+v", expected, parsed)
	}
}

func TestWriteFFMetadata_DeterministicExtraOrder(t *testing.T) {
	tag := &audio.Tag{
		Extra: map[string]string{"zzz": "last", "aaa": "first", "mmm": "mid"},
	}
	var a, b bytes.Buffer
	if err := WriteFFMetadata(&a, tag); err != nil {
		t.Fatal(err)
	}
	if err := WriteFFMetadata(&b, tag); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("non-deterministic output:\n%s\n---\n%s", a.String(), b.String())
	}
	// And keys should appear alphabetically.
	idxA := strings.Index(a.String(), "aaa=")
	idxM := strings.Index(a.String(), "mmm=")
	idxZ := strings.Index(a.String(), "zzz=")
	if idxA >= idxM || idxM >= idxZ {
		t.Errorf("extra keys not sorted: %s", a.String())
	}
}

func TestWriteFFMetadata_SkipsZeroFields(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFFMetadata(&buf, &audio.Tag{Title: "Only"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, k := range []string{"album=", "artist=", "date=", "track=", "composer="} {
		if strings.Contains(out, k) {
			t.Errorf("output contains zero field %q:\n%s", k, out)
		}
	}
	if !strings.Contains(out, "title=Only") {
		t.Errorf("output missing set title:\n%s", out)
	}
}

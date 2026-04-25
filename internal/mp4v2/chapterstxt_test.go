package mp4v2

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestParseChaptersTxt_Basic(t *testing.T) {
	in := `## total-duration: 00:15:00.000
00:00:00.000 Prologue
00:05:30.500 Chapter 1
00:10:00.000 Chapter 2
`
	got, err := ParseChaptersTxt(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []audio.Chapter{
		{Start: 0, Length: 330500 * time.Millisecond, Name: "Prologue"},
		{Start: 330500 * time.Millisecond, Length: 269500 * time.Millisecond, Name: "Chapter 1"},
		{Start: 10 * time.Minute, Length: 0, Name: "Chapter 2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:  %+v\nwant: %+v", got, want)
	}
}

func TestParseChaptersTxt_TitleWithSpaces(t *testing.T) {
	in := "00:00:00.000 Chapter 1: It Begins, with punctuation\n"
	got, err := ParseChaptersTxt(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Name != "Chapter 1: It Begins, with punctuation" {
		t.Errorf("name = %q", got[0].Name)
	}
}

func TestParseChaptersTxt_SkipsCommentsAndBlanks(t *testing.T) {
	in := `## total-duration: 00:10:00.000

## another comment
00:00:00.000 First
`
	got, err := ParseChaptersTxt(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "First" {
		t.Errorf("got %+v", got)
	}
}

func TestParseChaptersTxt_Empty(t *testing.T) {
	got, err := ParseChaptersTxt(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want empty", got)
	}
}

func TestParseChaptersTxt_MalformedLine(t *testing.T) {
	_, err := ParseChaptersTxt(strings.NewReader("00:00:00.000MissingSpace\n"))
	if err == nil {
		t.Fatal("expected error on missing space separator")
	}
}

func TestParseChaptersTxt_BadTimecode(t *testing.T) {
	_, err := ParseChaptersTxt(strings.NewReader("not-a-time Foo\n"))
	if err == nil {
		t.Fatal("expected error on bad timecode")
	}
}

func TestWriteChaptersTxt_WithTotal(t *testing.T) {
	chapters := []audio.Chapter{
		{Start: 0, Length: 330500 * time.Millisecond, Name: "Prologue"},
		{Start: 330500 * time.Millisecond, Length: 269500 * time.Millisecond, Name: "Chapter 1"},
	}
	var buf bytes.Buffer
	if err := WriteChaptersTxt(&buf, chapters, 10*time.Minute); err != nil {
		t.Fatalf("write: %v", err)
	}
	want := "## total-duration: 00:10:00.000\n" +
		"00:00:00.000 Prologue\n" +
		"00:05:30.500 Chapter 1\n"
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestWriteChaptersTxt_NoTotal(t *testing.T) {
	chapters := []audio.Chapter{{Start: 0, Name: "Only"}}
	var buf bytes.Buffer
	if err := WriteChaptersTxt(&buf, chapters, 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "total-duration") {
		t.Errorf("unexpected total-duration header: %q", buf.String())
	}
}

func TestChaptersTxt_Roundtrip(t *testing.T) {
	orig := []audio.Chapter{
		{Start: 0, Length: 5 * time.Second, Name: "A"},
		{Start: 5 * time.Second, Length: 15 * time.Second, Name: "B: complex, title"},
		{Start: 20 * time.Second, Length: 10 * time.Second, Name: "C"},
	}
	var buf bytes.Buffer
	if err := WriteChaptersTxt(&buf, orig, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseChaptersTxt(&buf)
	if err != nil {
		t.Fatal(err)
	}
	FillLastLength(parsed, 30*time.Second)

	if !reflect.DeepEqual(parsed, orig) {
		t.Errorf("round-trip mismatch\ngot:  %+v\nwant: %+v", parsed, orig)
	}
}

func TestFillLastLength(t *testing.T) {
	chapters := []audio.Chapter{
		{Start: 0, Length: 5 * time.Second},
		{Start: 5 * time.Second, Length: 0},
	}
	FillLastLength(chapters, 30*time.Second)
	if chapters[1].Length != 25*time.Second {
		t.Errorf("last length = %v, want 25s", chapters[1].Length)
	}
}

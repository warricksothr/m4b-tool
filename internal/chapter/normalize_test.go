package chapter

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestNormalize_RegexAndStripChars(t *testing.T) {
	pat := regexp.MustCompile(`^Chapter \d+:\s*(.*)$`)
	in := []audio.Chapter{
		ch(0, time.Minute, "Chapter 1: „Hello"),
		ch(time.Minute, time.Minute, "Chapter 2: World"),
	}
	out := Normalize(in, NormalizeOptions{
		Pattern:     pat,
		Replacement: "$1",
		RemoveChars: "„",
	})
	if out[0].Name != "Hello" {
		t.Errorf("got %q, want Hello", out[0].Name)
	}
	if out[1].Name != "World" {
		t.Errorf("got %q, want World", out[1].Name)
	}
}

func TestNormalize_MergeSimilar(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 10*time.Second, "A"),
		ch(10*time.Second, 10*time.Second, "A"),
		ch(20*time.Second, 10*time.Second, "A"),
		ch(30*time.Second, 10*time.Second, "B"),
	}
	out := Normalize(in, NormalizeOptions{MergeSimilar: true})
	if len(out) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(out), out)
	}
	if out[0].Length != 30*time.Second {
		t.Errorf("A merged length = %v, want 30s", out[0].Length)
	}
	if out[0].Name != "A" || out[1].Name != "B" {
		t.Errorf("names = %q, %q", out[0].Name, out[1].Name)
	}
}

func TestNormalize_DupSuffix(t *testing.T) {
	in := []audio.Chapter{
		ch(0, time.Second, "Part"),
		ch(time.Second, time.Second, "Part"),
		ch(2*time.Second, time.Second, "Part"),
		ch(3*time.Second, time.Second, "Other"),
	}
	out := Normalize(in, NormalizeOptions{})
	want := []string{"Part", "Part (2)", "Part (3)", "Other"}
	for i, w := range want {
		if out[i].Name != w {
			t.Errorf("out[%d] = %q, want %q", i, out[i].Name, w)
		}
	}
}

func TestNormalize_NoNumberingLeavesDupsAlone(t *testing.T) {
	in := []audio.Chapter{
		ch(0, time.Second, "Part"),
		ch(time.Second, time.Second, "Part"),
	}
	out := Normalize(in, NormalizeOptions{NoNumbering: true})
	for _, c := range out {
		if c.Name != "Part" {
			t.Errorf("got %q, want Part", c.Name)
		}
	}
}

func TestNormalize_DupCounterResetsOnDifferentName(t *testing.T) {
	in := []audio.Chapter{
		ch(0, time.Second, "A"),
		ch(time.Second, time.Second, "A"),
		ch(2*time.Second, time.Second, "B"),
		ch(3*time.Second, time.Second, "A"), // counter reset
	}
	out := Normalize(in, NormalizeOptions{})
	if out[3].Name != "A" {
		t.Errorf("out[3] = %q, want A (counter should have reset)", out[3].Name)
	}
}

func TestStripChars_UTF8Aware(t *testing.T) {
	got := stripChars(`„Quoted"`, `„"`)
	if got != "Quoted" {
		t.Errorf("got %q, want Quoted", got)
	}
}

func TestPrependIntro(t *testing.T) {
	in := []audio.Chapter{ch(0, 60*time.Second, "A")}
	out := PrependIntro(in, 5*time.Second)
	if len(out) != 2 {
		t.Fatalf("got %d", len(out))
	}
	if out[0].Name != "Intro" || out[0].Length != 5*time.Second {
		t.Errorf("intro = %+v", out[0])
	}
	if out[1].Start != 5*time.Second {
		t.Errorf("A shifted to %v, want 5s", out[1].Start)
	}
}

func TestPrependIntro_ZeroOffset(t *testing.T) {
	in := []audio.Chapter{ch(0, time.Second, "A")}
	out := PrependIntro(in, 0)
	if !reflect.DeepEqual(out, in) {
		t.Errorf("zero offset should return same contents, got %+v", out)
	}
}

func TestAppendOutro(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 60*time.Second, "B"),
	}
	out := AppendOutro(in, 90*time.Second, 10*time.Second)
	if len(out) != 3 {
		t.Fatalf("got %d", len(out))
	}
	// B should be truncated to end at 80s (90 - 10)
	if out[1].Length != 50*time.Second {
		t.Errorf("B truncated length = %v, want 50s", out[1].Length)
	}
	if out[2].Name != "Outro" || out[2].Start != 80*time.Second || out[2].Length != 10*time.Second {
		t.Errorf("outro = %+v", out[2])
	}
}

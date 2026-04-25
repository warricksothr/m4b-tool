package chapter

import (
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestMergeTooShort_MergesIntoPredecessor(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 60*time.Second, "A"),
		ch(60*time.Second, 2*time.Second, "B"), // too short
		ch(62*time.Second, 60*time.Second, "C"),
	}
	out := MergeTooShort(in, 5*time.Second, 0, nil)
	if len(out) != 2 {
		t.Fatalf("got %d chapters, want 2: %+v", len(out), out)
	}
	if out[0].Length != 62*time.Second {
		t.Errorf("A.Length = %v, want 62s", out[0].Length)
	}
	if out[1].Name != "C" {
		t.Errorf("got %+v, want C second", out[1])
	}
}

func TestMergeTooShort_MergesFirstIntoSuccessor(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 2*time.Second, "A"), // too short, no predecessor
		ch(2*time.Second, 60*time.Second, "B"),
	}
	out := MergeTooShort(in, 5*time.Second, 0, nil)
	if len(out) != 1 {
		t.Fatalf("got %d, want 1: %+v", len(out), out)
	}
	if out[0].Start != 0 {
		t.Errorf("Start = %v, want 0", out[0].Start)
	}
	if out[0].Length != 62*time.Second {
		t.Errorf("Length = %v, want 62s", out[0].Length)
	}
}

func TestMergeTooShort_RespectsExempt(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 60*time.Second, "A"),
		ch(60*time.Second, 2*time.Second, "Intro"),
		ch(62*time.Second, 60*time.Second, "C"),
	}
	out := MergeTooShort(in, 5*time.Second, 0, []int{1})
	if len(out) != 3 {
		t.Errorf("exempt chapter merged: %+v", out)
	}
}

func TestMergeTooShort_SkipsWhenMaxExceeded(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 50*time.Second, "A"),
		ch(50*time.Second, 2*time.Second, "B"), // would make A = 52s, over max=51
	}
	out := MergeTooShort(in, 5*time.Second, 51*time.Second, nil)
	if len(out) != 2 {
		t.Errorf("merge happened despite max exceeded: %+v", out)
	}
}

func TestMergeTooShort_EmptyInput(t *testing.T) {
	if out := MergeTooShort(nil, 5*time.Second, 0, nil); len(out) != 0 {
		t.Errorf("got %v", out)
	}
}

func TestMergeShortTail_Trims(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 300*time.Second, "Main"),
		ch(300*time.Second, 30*time.Second, "Short Tail"),
	}
	out := MergeShortTail(in, 400*time.Second)
	if len(out) != 1 {
		t.Fatalf("got %d, want 1", len(out))
	}
	if out[0].Length != 330*time.Second {
		t.Errorf("merged length = %v, want 330s", out[0].Length)
	}
}

func TestMergeShortTail_LeavesLongTail(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 300*time.Second, "Main"),
		ch(300*time.Second, 90*time.Second, "Long Tail"),
	}
	out := MergeShortTail(in, 400*time.Second)
	if len(out) != 2 {
		t.Errorf("long tail was merged: %+v", out)
	}
}

func TestMergeShortTail_SingleChapterNoop(t *testing.T) {
	in := []audio.Chapter{ch(0, 30*time.Second, "Only")}
	out := MergeShortTail(in, 0)
	if len(out) != 1 || out[0] != in[0] {
		t.Errorf("got %+v, want same as input", out)
	}
}

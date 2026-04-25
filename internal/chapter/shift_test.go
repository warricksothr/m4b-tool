package chapter

import (
	"errors"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func ch(start, length time.Duration, name string) audio.Chapter {
	return audio.Chapter{Start: start, Length: length, Name: name}
}

func TestShift_ShiftAllSkipsFirst(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 30*time.Second, "B"),
		ch(60*time.Second, 40*time.Second, "C"),
	}
	out, err := Shift(in, 10*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Start != 0 {
		t.Errorf("first chapter moved: %v", out[0])
	}
	if out[1].Start != 40*time.Second {
		t.Errorf("B.Start = %v, want 40s", out[1].Start)
	}
	if out[2].Start != 70*time.Second {
		t.Errorf("C.Start = %v, want 70s", out[2].Start)
	}
	// Lengths: A grows by 10s, B keeps 30s, C shrinks by 10s
	if out[0].Length != 40*time.Second {
		t.Errorf("A.Length = %v, want 40s", out[0].Length)
	}
	if out[1].Length != 30*time.Second {
		t.Errorf("B.Length = %v", out[1].Length)
	}
	if out[2].Length != 30*time.Second {
		t.Errorf("C.Length = %v, want 30s (preserved end)", out[2].Length)
	}
}

func TestShift_DoesNotMutateInput(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 70*time.Second, "B"),
	}
	_, err := Shift(in, 5*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if in[1].Start != 30*time.Second {
		t.Errorf("input mutated: %+v", in[1])
	}
}

func TestShift_SpecificIndex(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 30*time.Second, "B"),
		ch(60*time.Second, 40*time.Second, "C"),
	}
	out, err := Shift(in, 5*time.Second, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if out[1].Start != 35*time.Second {
		t.Errorf("B.Start = %v, want 35s", out[1].Start)
	}
	if out[2].Start != 60*time.Second {
		t.Errorf("C.Start = %v, want 60s (unchanged)", out[2].Start)
	}
}

func TestShift_NegativeIndex(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 30*time.Second, "B"),
		ch(60*time.Second, 40*time.Second, "C"),
	}
	// -1 = last chapter. Shift by -5s → C.Start = 55s, C.Length = 45s.
	out, err := Shift(in, -5*time.Second, []int{-1})
	if err != nil {
		t.Fatal(err)
	}
	if out[2].Start != 55*time.Second {
		t.Errorf("C.Start = %v, want 55s", out[2].Start)
	}
	if out[2].Length != 45*time.Second {
		t.Errorf("C.Length = %v, want 45s", out[2].Length)
	}
}

func TestShift_NegativeLengthRejected(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 30*time.Second, "B"),
		ch(60*time.Second, 40*time.Second, "C"),
	}
	// Shift B by -50s → B.Start = -20s → A.Length = -20s → invalid.
	_, err := Shift(in, -50*time.Second, []int{1})
	if !errors.Is(err, ErrShiftInvalid) {
		t.Errorf("err = %v, want ErrShiftInvalid", err)
	}
}

func TestShift_EmptyInput(t *testing.T) {
	out, err := Shift(nil, 5*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("got %d chapters, want 0", len(out))
	}
}

func TestShift_Index0AlwaysSkipped(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 70*time.Second, "B"),
	}
	out, err := Shift(in, 5*time.Second, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Start != 0 {
		t.Errorf("index 0 moved: %v", out[0])
	}
	if out[1].Start != 30*time.Second {
		t.Errorf("index 1 moved despite not being requested: %v", out[1])
	}
}

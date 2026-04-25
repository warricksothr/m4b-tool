package chapter

import (
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestRemoveDuplicateFollowUps_Merges(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 10*time.Second, "A"),
		ch(10*time.Second, 10*time.Second, "A"),
		ch(20*time.Second, 10*time.Second, "B"),
	}
	out := RemoveDuplicateFollowUps(in)
	if len(out) != 2 {
		t.Fatalf("got %d, want 2", len(out))
	}
	if out[0].Length != 20*time.Second {
		t.Errorf("A length = %v, want 20s", out[0].Length)
	}
}

func TestRemoveDuplicateFollowUps_PreservesIntroOutro(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 5*time.Second, "Intro"),
		ch(5*time.Second, 10*time.Second, "Intro"), // repeat — shouldn't merge
		ch(15*time.Second, 10*time.Second, "Outro"),
	}
	out := RemoveDuplicateFollowUps(in)
	if len(out) != 3 {
		t.Errorf("got %d, want 3 (Intro markers should not merge)", len(out))
	}
}

func TestRemoveDuplicateFollowUps_SingleChapter(t *testing.T) {
	in := []audio.Chapter{ch(0, time.Second, "Only")}
	out := RemoveDuplicateFollowUps(in)
	if len(out) != 1 {
		t.Errorf("got %d, want 1", len(out))
	}
}

func TestRemoveDuplicateFollowUps_ChainedDuplicates(t *testing.T) {
	in := []audio.Chapter{
		ch(0, 5*time.Second, "A"),
		ch(5*time.Second, 5*time.Second, "A"),
		ch(10*time.Second, 5*time.Second, "A"),
		ch(15*time.Second, 5*time.Second, "A"),
	}
	out := RemoveDuplicateFollowUps(in)
	if len(out) != 1 {
		t.Fatalf("got %d, want 1", len(out))
	}
	if out[0].Length != 20*time.Second {
		t.Errorf("merged length = %v, want 20s", out[0].Length)
	}
}

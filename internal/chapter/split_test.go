package chapter

import (
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func sil(start, length time.Duration) audio.Silence {
	return audio.Silence{Start: start, Length: length}
}

func TestSplitTooLong_Unchanged(t *testing.T) {
	c := ch(0, 30*time.Second, "A")
	out := SplitTooLong(c, nil, 20*time.Second, 30*time.Second)
	if len(out) != 1 || out[0] != c {
		t.Errorf("got %+v, want unchanged", out)
	}
}

func TestSplitTooLong_HardCutNoSilences(t *testing.T) {
	c := ch(0, 120*time.Second, "Long")
	out := SplitTooLong(c, nil, 30*time.Second, 40*time.Second)
	// With desired=30 max=40, no silences, hard cut at every 30s:
	// [0,30) [30,60) [60,90) [90,120)  — but tail 30 + prev 30 = 60 > max=40, no merge.
	if len(out) != 4 {
		t.Fatalf("got %d pieces, want 4: %+v", len(out), out)
	}
	for i, p := range out {
		if p.Name != "Long" {
			t.Errorf("piece %d: Name = %q, want Long", i, p.Name)
		}
		if p.Length != 30*time.Second {
			t.Errorf("piece %d: Length = %v, want 30s", i, p.Length)
		}
	}
}

func TestSplitTooLong_PrefersSilenceMidpointInWindow(t *testing.T) {
	c := ch(0, 120*time.Second, "Book")
	// Silence at 31s-33s (midpoint 32s) falls into [30s, 40s] window.
	silences := []audio.Silence{sil(31*time.Second, 2*time.Second)}
	out := SplitTooLong(c, silences, 30*time.Second, 40*time.Second)
	if out[0].Length != 32*time.Second {
		t.Errorf("first split at %v, want 32s (silence midpoint)", out[0].Length)
	}
}

func TestSplitTooLong_TailMergeWhenWithinMax(t *testing.T) {
	c := ch(0, 50*time.Second, "X")
	// Hard-cut pieces: [0,30s) [30,50s). Tail 20s + prev 30s = 50s > max 40 — no merge.
	// Change to: desired=20, max=40 → pieces [0,20s) [20,40s) [40,50s). Tail 10s + prev 20s = 30s <= max 40 → merge.
	out := SplitTooLong(c, nil, 20*time.Second, 40*time.Second)
	if len(out) != 2 {
		t.Fatalf("got %d pieces, want 2: %+v", len(out), out)
	}
	if out[1].Length != 30*time.Second {
		t.Errorf("merged tail length = %v, want 30s", out[1].Length)
	}
}

func TestSplitTooLong_NonZeroStartPreserved(t *testing.T) {
	c := ch(100*time.Second, 60*time.Second, "Middle")
	out := SplitTooLong(c, nil, 30*time.Second, 40*time.Second)
	if out[0].Start != 100*time.Second {
		t.Errorf("first piece Start = %v, want 100s", out[0].Start)
	}
}

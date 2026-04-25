package chapter

import (
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestAlignToSilence_SnapsToNearestMidpoint(t *testing.T) {
	chapters := []audio.Chapter{
		ch(0, 0, "A"),              // Length filled by alignment
		ch(30*time.Second, 0, "B"), // should snap to silence at 29-31s (mid 30s) — no change
		ch(60*time.Second, 0, "C"),
	}
	silences := []audio.Silence{
		sil(29*time.Second, 2*time.Second), // mid 30s
		sil(58*time.Second, 2*time.Second), // mid 59s
	}
	out := AlignToSilence(chapters, silences, AlignOptions{Total: 90 * time.Second})
	if out[1].Start != 30*time.Second {
		t.Errorf("B.Start = %v, want 30s (at midpoint)", out[1].Start)
	}
	if out[2].Start != 59*time.Second {
		t.Errorf("C.Start = %v, want 59s (midpoint of 58-60)", out[2].Start)
	}
	// Lengths recomputed
	if out[0].Length != 30*time.Second {
		t.Errorf("A.Length = %v, want 30s", out[0].Length)
	}
	if out[1].Length != 29*time.Second {
		t.Errorf("B.Length = %v, want 29s", out[1].Length)
	}
	if out[2].Length != 31*time.Second {
		t.Errorf("C.Length = %v, want 31s (90-59)", out[2].Length)
	}
}

func TestAlignToSilence_FirstChapterUntouched(t *testing.T) {
	chapters := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 30*time.Second, "B"),
	}
	silences := []audio.Silence{sil(1*time.Second, 2*time.Second)}
	out := AlignToSilence(chapters, silences, AlignOptions{Total: 60 * time.Second})
	if out[0].Start != 0 {
		t.Errorf("first chapter moved: %v", out[0])
	}
}

func TestAlignToSilence_MaxDiffLimit(t *testing.T) {
	chapters := []audio.Chapter{
		ch(0, 0, "A"),
		ch(30*time.Second, 0, "B"),
	}
	// Silence too far (>25s from expected position): no snap.
	silences := []audio.Silence{sil(time.Minute, time.Second)}
	out := AlignToSilence(chapters, silences, AlignOptions{Total: 90 * time.Second})
	if out[1].Start != 30*time.Second {
		t.Errorf("B snapped despite out-of-range silence: %v", out[1].Start)
	}
}

func TestAlignToSilence_CustomMaxDiff(t *testing.T) {
	chapters := []audio.Chapter{
		ch(0, 0, "A"),
		ch(30*time.Second, 0, "B"),
	}
	// Silence 40s away, but MaxDiff=60s lets it match.
	silences := []audio.Silence{sil(70*time.Second, 2*time.Second)}
	out := AlignToSilence(chapters, silences, AlignOptions{
		Total:   120 * time.Second,
		MaxDiff: 60 * time.Second,
	})
	if out[1].Start != 71*time.Second {
		t.Errorf("B.Start = %v, want 71s", out[1].Start)
	}
}

func TestAlignToSilence_AccumulatedOffsetCompensates(t *testing.T) {
	// B snaps late; C's expected position should be computed relative
	// to the drift, not B's adjusted position.
	chapters := []audio.Chapter{
		ch(0, 0, "A"),
		ch(30*time.Second, 0, "B"),
		ch(60*time.Second, 0, "C"),
	}
	silences := []audio.Silence{
		sil(35*time.Second, 2*time.Second), // mid 36s — B snaps forward by 6s
		sil(65*time.Second, 2*time.Second), // mid 66s — drift compensates: expected 60-6=54, distance to 65 is 11s (in range), snap to 66s
	}
	out := AlignToSilence(chapters, silences, AlignOptions{Total: 90 * time.Second})
	if out[1].Start != 36*time.Second {
		t.Errorf("B.Start = %v, want 36s", out[1].Start)
	}
	if out[2].Start != 66*time.Second {
		t.Errorf("C.Start = %v, want 66s", out[2].Start)
	}
}

func TestAlignToSilence_SalvageShortLastChapter(t *testing.T) {
	// Last chapter starts at 89s, total 90s → length 1s < 2.5s salvage threshold.
	// There is a silence at 70-72s (mid 71s). Walking backward we should
	// find it, move last chapter to 71s, reducing prev chapter to 71-30=41s.
	chapters := []audio.Chapter{
		ch(0, 30*time.Second, "A"),
		ch(30*time.Second, 59*time.Second, "B"),
		ch(89*time.Second, 0, "C"),
	}
	silences := []audio.Silence{sil(70*time.Second, 2*time.Second)}
	out := AlignToSilence(chapters, silences, AlignOptions{Total: 90 * time.Second, MaxDiff: 5 * time.Second})
	if out[2].Start != 71*time.Second {
		t.Errorf("salvage: C.Start = %v, want 71s", out[2].Start)
	}
	if out[2].Length != 19*time.Second {
		t.Errorf("salvage: C.Length = %v, want 19s", out[2].Length)
	}
	if out[1].Length != 41*time.Second {
		t.Errorf("salvage: B.Length = %v, want 41s", out[1].Length)
	}
}

func TestAlignToSilence_EmptyInputs(t *testing.T) {
	if out := AlignToSilence(nil, nil, AlignOptions{Total: 10 * time.Second}); len(out) != 0 {
		t.Errorf("got %d", len(out))
	}
}

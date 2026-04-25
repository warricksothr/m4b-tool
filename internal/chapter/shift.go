package chapter

import (
	"errors"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// ErrShiftInvalid is returned by [Shift] when applying the requested
// offset would produce a chapter with negative length.
var ErrShiftInvalid = errors.New("shift produces negative-length chapter")

// Shift moves the selected chapters by offset and repacks lengths.
//
// indexes names the chapters to move. An empty slice means "all
// chapters." Negative indexes count from the end of the slice
// (-1 = last). Index 0 is always skipped even when explicitly listed,
// because moving the first chapter's start would break the "first
// chapter starts at 0" invariant (see spec/data-model.md §Invariants).
//
// The last chapter's End is preserved. Middle chapters have their
// Length recomputed from the (possibly shifted) next chapter's Start.
//
// Returns a new slice on success, or [ErrShiftInvalid] if any resulting
// chapter would have negative length — the original slice is never
// mutated.
func Shift(chapters []audio.Chapter, offset time.Duration, indexes []int) ([]audio.Chapter, error) {
	if len(chapters) == 0 {
		return nil, nil
	}
	out := make([]audio.Chapter, len(chapters))
	copy(out, chapters)

	selected := resolveIndexes(indexes, len(out))
	lastIdx := len(out) - 1
	lastEnd := out[lastIdx].End()

	for _, i := range selected {
		if i <= 0 || i >= len(out) {
			// Skip out-of-range and index 0.
			continue
		}
		out[i].Start += offset
	}

	for i := 0; i < lastIdx; i++ {
		out[i].Length = out[i+1].Start - out[i].Start
	}
	out[lastIdx].Length = lastEnd - out[lastIdx].Start

	for _, c := range out {
		if c.Length < 0 {
			return nil, ErrShiftInvalid
		}
	}
	return out, nil
}

// resolveIndexes normalizes a possibly-empty index list into a slice of
// non-negative indexes. Empty input expands to [0, 1, ..., n-1].
// Negative indexes wrap around (-1 → n-1). Out-of-range indexes are
// dropped.
func resolveIndexes(indexes []int, n int) []int {
	if len(indexes) == 0 {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return all
	}
	out := make([]int, 0, len(indexes))
	for _, idx := range indexes {
		if idx < 0 {
			idx = n + idx
		}
		if idx >= 0 && idx < n {
			out = append(out, idx)
		}
	}
	return out
}

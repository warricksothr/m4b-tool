package chapter

import (
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// MergeTooShort walks the chapter list and merges any chapter whose
// Length is below minLength into a neighbor: into its predecessor by
// default, or into its successor when there is no predecessor. Chapters
// whose index is in `exempt` are never merged. A merge is skipped when
// the resulting chapter would exceed maxLength (0 = no cap).
//
// See spec/chapter-algorithms.md §Length-based merging.
func MergeTooShort(chapters []audio.Chapter, minLength, maxLength time.Duration, exempt []int) []audio.Chapter {
	if len(chapters) == 0 {
		return nil
	}

	exemptSet := make(map[int]struct{}, len(exempt))
	n := len(chapters)
	for _, idx := range exempt {
		if idx < 0 {
			idx = n + idx
		}
		if idx >= 0 && idx < n {
			exemptSet[idx] = struct{}{}
		}
	}

	out := make([]audio.Chapter, len(chapters))
	copy(out, chapters)

	i := 0
	origIdx := 0 // index in the original slice, for exempt lookups
	for i < len(out) {
		if _, isExempt := exemptSet[origIdx]; isExempt || out[i].Length >= minLength {
			i++
			origIdx++
			continue
		}
		switch {
		case i > 0 && (maxLength == 0 || out[i-1].Length+out[i].Length <= maxLength):
			out[i-1].Length += out[i].Length
			out = append(out[:i], out[i+1:]...)
		case i == 0 && len(out) > 1 && (maxLength == 0 || out[i].Length+out[i+1].Length <= maxLength):
			out[i+1].Start = out[i].Start
			out[i+1].Length += out[i].Length
			out = append(out[:i], out[i+1:]...)
		default:
			// No neighbor available within budget; leave in place.
			i++
		}
		origIdx++
	}
	return out
}

// MergeShortTail collapses the final chapter into its predecessor when
// it's shorter than 60 s AND the merged result fits within maxLength.
// This is the "tail trim" variant called out in spec/chapter-algorithms.md.
//
// Returns the input unchanged when there's only one chapter or when the
// merge would exceed maxLength.
func MergeShortTail(chapters []audio.Chapter, maxLength time.Duration) []audio.Chapter {
	const tailMin = 60 * time.Second
	n := len(chapters)
	if n < 2 {
		return append([]audio.Chapter(nil), chapters...)
	}
	out := make([]audio.Chapter, n)
	copy(out, chapters)

	if out[n-1].Length >= tailMin {
		return out
	}
	if out[n-2].Length+out[n-1].Length > maxLength {
		return out
	}
	out[n-2].Length += out[n-1].Length
	return out[:n-1]
}

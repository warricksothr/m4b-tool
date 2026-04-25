package chapter

import (
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// SplitTooLong divides an over-long chapter into pieces of approximately
// `desired` length, preferring silence boundaries within the search
// window [cursor+desired, cursor+max]. When no silence falls in the
// window, a hard cut is made at cursor+desired.
//
// All output pieces inherit the input chapter's Name; numeric suffixes
// are applied downstream by [Normalize]. A short tail piece is merged
// back into its predecessor when doing so stays within the max.
//
// Returns the input chapter unchanged (wrapped in a one-element slice)
// when chapter.Length <= max, so callers can apply this unconditionally.
func SplitTooLong(chapter audio.Chapter, silences []audio.Silence, desired, max time.Duration) []audio.Chapter {
	if chapter.Length <= max {
		return []audio.Chapter{chapter}
	}
	if desired <= 0 || desired > max {
		desired = max
	}

	var result []audio.Chapter
	cursor := chapter.Start
	end := chapter.End()

	for cursor < end {
		windowStart := cursor + desired
		windowEnd := cursor + max
		if windowEnd > end {
			windowEnd = end
		}

		split, found := firstSilenceIn(silences, windowStart, windowEnd)
		if !found {
			split = windowStart
			if split > end {
				split = end
			}
		}

		if split == cursor {
			// Degenerate: avoid infinite loop. Force one-unit advance.
			split = end
		}

		result = append(result, audio.Chapter{
			Start:  cursor,
			Length: split - cursor,
			Name:   chapter.Name,
		})
		cursor = split
	}

	if len(result) >= 2 {
		lastLen := result[len(result)-1].Length
		prevLen := result[len(result)-2].Length
		if lastLen < desired && lastLen+prevLen <= max {
			result[len(result)-2].Length += lastLen
			result = result[:len(result)-1]
		}
	}
	return result
}

// firstSilenceIn returns the midpoint of the first silence whose
// midpoint lies in [start, end], or (0, false) when no silence fits.
func firstSilenceIn(silences []audio.Silence, start, end time.Duration) (time.Duration, bool) {
	for _, s := range silences {
		mid := s.Midpoint()
		if mid >= start && mid <= end {
			return mid, true
		}
	}
	return 0, false
}

// SplitAllTooLong applies [SplitTooLong] to every chapter in the slice,
// returning a new flattened list. Convenience driver; see spec
// `ChapterLengthCalculator::splitTooLongChapters`.
func SplitAllTooLong(chapters []audio.Chapter, silences []audio.Silence, desired, max time.Duration) []audio.Chapter {
	var out []audio.Chapter
	for _, c := range chapters {
		out = append(out, SplitTooLong(c, silences, desired, max)...)
	}
	return out
}

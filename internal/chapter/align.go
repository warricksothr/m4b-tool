package chapter

import (
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// AlignOptions controls [AlignToSilence]. Zero values take spec defaults.
type AlignOptions struct {
	// Total is the total audio duration, used to compute the last
	// chapter's Length. Required; a zero value yields a zero-length
	// last chapter.
	Total time.Duration
	// MaxDiff caps how far a chapter boundary may be moved to snap onto
	// a silence. Defaults to 25s per spec.
	MaxDiff time.Duration
	// LastChapterMin is the threshold below which the last chapter is
	// considered "too short" and the salvage pass kicks in. Defaults to
	// 2500ms per spec.
	LastChapterMin time.Duration
}

// AlignToSilence snaps each chapter boundary onto the midpoint of the
// nearest detected silence, compensating for accumulated drift so later
// chapters use the already-adjusted positions. The first chapter (Start
// == 0) is never moved.
//
// After alignment the salvage pass checks whether the last chapter ended
// up too short; if so it walks silences in reverse to find a better
// breakpoint for the last chapter's Start.
//
// See spec/chapter-algorithms.md §Silence-based chapter alignment.
func AlignToSilence(chapters []audio.Chapter, silences []audio.Silence, opts AlignOptions) []audio.Chapter {
	if len(chapters) == 0 {
		return nil
	}
	maxDiff := opts.MaxDiff
	if maxDiff == 0 {
		maxDiff = 25 * time.Second
	}
	lastChapterMin := opts.LastChapterMin
	if lastChapterMin == 0 {
		lastChapterMin = 2500 * time.Millisecond
	}

	out := make([]audio.Chapter, len(chapters))
	copy(out, chapters)

	var accumulatedOffset time.Duration
	for i := range out {
		if out[i].Start == 0 {
			continue
		}
		adjustedStart := out[i].Start - accumulatedOffset
		bestMid := time.Duration(0)
		bestDist := maxDiff
		found := false
		for _, s := range silences {
			dist := absDur(adjustedStart - s.Start)
			if dist < bestDist {
				bestDist = dist
				bestMid = s.Midpoint()
				found = true
			}
		}
		if found {
			accumulatedOffset += adjustedStart - bestMid
			out[i].Start = bestMid
		} else {
			out[i].Start = adjustedStart
		}
	}

	// Recompute lengths from adjusted starts.
	last := len(out) - 1
	for i := 0; i < last; i++ {
		out[i].Length = out[i+1].Start - out[i].Start
	}
	if opts.Total > out[last].Start {
		out[last].Length = opts.Total - out[last].Start
	} else {
		out[last].Length = 0
	}

	// Salvage: last chapter too short? Walk silences backward for a
	// better split between the penultimate and last chapter.
	if last >= 1 && out[last].Length < lastChapterMin && opts.Total > 0 {
		prevEnd := out[last-1].End()
		for i := len(silences) - 1; i >= 0; i-- {
			mid := silences[i].Midpoint()
			if prevEnd-mid > lastChapterMin && mid > out[last-1].Start {
				out[last].Start = mid
				out[last].Length = opts.Total - mid
				out[last-1].Length = mid - out[last-1].Start
				break
			}
		}
	}
	return out
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

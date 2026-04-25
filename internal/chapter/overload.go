package chapter

import "github.com/warricksothr/m4b-tool/internal/audio"

// OverloadFromTracks assigns chapter names (and introductions) from
// namedChapters onto trackChapters by picking, for each track, the
// named chapter whose interval overlaps it the most. Tracks with no
// overlap retain their original name.
//
// Used to merge a file-derived chapter list (where names are filenames)
// with a metadata-derived list (MusicBrainz, Audible). See
// spec/chapter-algorithms.md §Overlap-based chapter matching.
func OverloadFromTracks(trackChapters, namedChapters []audio.Chapter) []audio.Chapter {
	if len(trackChapters) == 0 {
		return nil
	}
	out := make([]audio.Chapter, len(trackChapters))
	copy(out, trackChapters)

	for i, t := range out {
		var (
			bestOverlap = int64(0)
			bestIdx     = -1
		)
		for j, n := range namedChapters {
			ovr := overlapNanos(t, n)
			if ovr > bestOverlap {
				bestOverlap = ovr
				bestIdx = j
			}
		}
		if bestIdx >= 0 {
			out[i].Name = namedChapters[bestIdx].Name
			out[i].Introduction = namedChapters[bestIdx].Introduction
		}
	}
	return out
}

func overlapNanos(a, b audio.Chapter) int64 {
	start := a.Start
	if b.Start > start {
		start = b.Start
	}
	aEnd, bEnd := a.End(), b.End()
	end := aEnd
	if bEnd < end {
		end = bEnd
	}
	if end <= start {
		return 0
	}
	return int64(end - start)
}

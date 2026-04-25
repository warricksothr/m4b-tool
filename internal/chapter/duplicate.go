package chapter

import "github.com/warricksothr/m4b-tool/internal/audio"

// RemoveDuplicateFollowUps merges adjacent chapters that share the same
// name: chapters[i].Length grows by chapters[i+1].Length, and
// chapters[i+1] is dropped. The reserved names "Intro" and "Outro" are
// preserved as distinct markers even when a neighbor shares the name.
//
// See spec/chapter-algorithms.md §Duplicate adjacent chapter removal.
func RemoveDuplicateFollowUps(chapters []audio.Chapter) []audio.Chapter {
	if len(chapters) < 2 {
		return append([]audio.Chapter(nil), chapters...)
	}
	out := make([]audio.Chapter, 0, len(chapters))
	out = append(out, chapters[0])
	for i := 1; i < len(chapters); i++ {
		prev := &out[len(out)-1]
		curr := chapters[i]
		if curr.Name == prev.Name && !isReservedBoundary(prev.Name) {
			prev.Length += curr.Length
			continue
		}
		out = append(out, curr)
	}
	return out
}

func isReservedBoundary(name string) bool {
	return name == audio.ChapterIntro || name == audio.ChapterOutro
}

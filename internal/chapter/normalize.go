package chapter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// NormalizeOptions controls name rewriting in [Normalize].
type NormalizeOptions struct {
	// Pattern is a compiled regex matched against each chapter name.
	// When nil, no regex substitution is applied.
	Pattern *regexp.Regexp
	// Replacement is the substitution template used with Pattern. `$1`,
	// `$2`, etc. refer to capture groups.
	Replacement string
	// RemoveChars is a set of UTF-8 code points to strip from each name
	// after the regex pass. Each rune in the string is a character to
	// remove; rune handling is UTF-8 aware.
	RemoveChars string
	// MergeSimilar collapses consecutive chapters with identical names
	// into one. Lengths accumulate; the first chapter's Start wins.
	MergeSimilar bool
	// NoNumbering disables the "(2)", "(3)", ... suffix applied to
	// subsequent duplicate names.
	NoNumbering bool
}

// Normalize applies name rewriting and duplicate-handling to chapters.
// Two passes: (1) per-chapter regex + character stripping, (2) a merge
// or suffix pass depending on MergeSimilar. Returns a new slice.
//
// See spec/chapter-algorithms.md §Normalization. The consecutive-
// numbering heuristic ("if >75% of names match a numeric pattern, rewrite
// as 1..N") is not implemented here — it's called out as a follow-on
// refinement; the base dup-suffix behavior below is the fallback.
func Normalize(chapters []audio.Chapter, opts NormalizeOptions) []audio.Chapter {
	if len(chapters) == 0 {
		return nil
	}
	out := make([]audio.Chapter, 0, len(chapters))

	// Pass 1: rewrite each name in isolation.
	for _, c := range chapters {
		name := c.Name
		if opts.Pattern != nil {
			name = opts.Pattern.ReplaceAllString(name, opts.Replacement)
		}
		if opts.RemoveChars != "" {
			name = stripChars(name, opts.RemoveChars)
		}
		c.Name = name
		out = append(out, c)
	}

	// Pass 2: collapse or suffix consecutive duplicates.
	if opts.MergeSimilar {
		merged := make([]audio.Chapter, 0, len(out))
		for _, c := range out {
			if n := len(merged); n > 0 && merged[n-1].Name == c.Name {
				merged[n-1].Length += c.Length
				continue
			}
			merged = append(merged, c)
		}
		return merged
	}
	if !opts.NoNumbering {
		applyDupSuffix(out)
	}
	return out
}

// applyDupSuffix walks chapters in place and renames consecutive runs of
// the same name to "name (2)", "name (3)", ... Leaves the first occurrence
// untouched.
func applyDupSuffix(chapters []audio.Chapter) {
	var lastName string
	counter := 0
	for i := range chapters {
		if chapters[i].Name == lastName {
			counter++
			chapters[i].Name = fmt.Sprintf("%s (%d)", chapters[i].Name, counter+1)
			continue
		}
		lastName = chapters[i].Name
		counter = 0
	}
}

// stripChars returns s with every rune in remove deleted.
func stripChars(s, remove string) string {
	if remove == "" || s == "" {
		return s
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(remove, r) {
			return -1
		}
		return r
	}, s)
}

// PrependIntro returns a new slice with a synthetic "Intro" chapter at
// the front spanning [0, offset], and the existing chapters shifted so
// the old first chapter now starts at offset.
//
// When offset is <= 0 the slice is returned as-is.
func PrependIntro(chapters []audio.Chapter, offset time.Duration) []audio.Chapter {
	if offset <= 0 {
		return append([]audio.Chapter(nil), chapters...)
	}
	intro := audio.Chapter{Start: 0, Length: offset, Name: audio.ChapterIntro}
	out := make([]audio.Chapter, 0, len(chapters)+1)
	out = append(out, intro)
	for _, c := range chapters {
		c.Start += offset
		out = append(out, c)
	}
	return out
}

// AppendOutro returns a new slice with a synthetic "Outro" chapter at
// the end spanning [total-offset, total]. The previous last chapter's
// Length is truncated to meet the Outro's Start.
//
// When offset is <= 0 or total is <= 0 the slice is returned as-is.
func AppendOutro(chapters []audio.Chapter, total, offset time.Duration) []audio.Chapter {
	if offset <= 0 || total <= 0 || offset >= total {
		return append([]audio.Chapter(nil), chapters...)
	}
	outroStart := total - offset
	out := make([]audio.Chapter, 0, len(chapters)+1)
	for _, c := range chapters {
		if c.End() > outroStart {
			c.Length = outroStart - c.Start
			if c.Length < 0 {
				c.Length = 0
			}
		}
		out = append(out, c)
	}
	out = append(out, audio.Chapter{Start: outroStart, Length: offset, Name: audio.ChapterOutro})
	return out
}

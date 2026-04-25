package cuesheet

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Sheet is the parse result of a single cue file.
type Sheet struct {
	Tag      audio.Tag
	Chapters []audio.Chapter
}

// Parse reads a cue sheet from r. Directives before the first TRACK set
// disc-level fields on Sheet.Tag; directives inside a TRACK block set
// per-chapter fields. Chapter Start is the INDEX 01 time; Length is the
// gap to the next chapter's Start, with the last chapter left at zero
// for the caller to fill from the file's total duration.
//
// Note on INDEX 00 pregap: the spec discusses an optional tightening
// where INDEX 00 of the following track trims the previous chapter's
// Length. Real cue sheets rarely carry meaningful INDEX 00 values for
// audiobook content, so this parser ignores them; callers who need
// pregap-aware lengths can layer that on later.
func Parse(r io.Reader) (*Sheet, error) {
	s := &Sheet{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		track *partialTrack
		lineN int
	)
	tracks := []*partialTrack{}

	flushTrack := func() {
		if track != nil {
			tracks = append(tracks, track)
			track = nil
		}
	}

	for scanner.Scan() {
		lineN++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		directive, rest := splitDirective(line)
		switch directive {
		case "TRACK":
			flushTrack()
			track = &partialTrack{}
			// "01 AUDIO" — we don't use the number, just the start of a new block.
		case "INDEX":
			if track == nil {
				continue // stray INDEX before any TRACK
			}
			num, tc, ok := splitIndex(rest)
			if !ok {
				return nil, fmt.Errorf("cuesheet line %d: malformed INDEX %q", lineN, rest)
			}
			if num == 1 {
				d, err := parseCueTime(tc)
				if err != nil {
					return nil, fmt.Errorf("cuesheet line %d: %w", lineN, err)
				}
				track.start = d
				track.hasStart = true
			}
		case "TITLE":
			value := unquote(rest)
			if track == nil {
				s.Tag.Album = value
			} else {
				track.title = value
			}
		case "PERFORMER":
			value := unquote(rest)
			if track == nil {
				s.Tag.Artist = value
			} else {
				track.performer = value
			}
		case "SONGWRITER":
			value := unquote(rest)
			if track == nil {
				s.Tag.Writer = value
			}
		case "GENRE":
			if track == nil {
				s.Tag.Genre = unquote(rest)
			}
		case "DATE":
			if track == nil {
				if n, err := strconv.Atoi(strings.Trim(strings.TrimSpace(rest), `"`)); err == nil {
					s.Tag.Year = n
				}
			}
		case "REM", "FILE", "CATALOG", "CDTEXTFILE", "ISRC", "FLAGS", "PREGAP", "POSTGAP":
			// Ignored at present; could populate Tag.Extra for REM keys later.
		default:
			// Unknown directive — ignore for forward-compat.
		}
	}
	flushTrack()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	chapters := make([]audio.Chapter, 0, len(tracks))
	for _, t := range tracks {
		if !t.hasStart {
			continue
		}
		chapters = append(chapters, audio.Chapter{
			Start: t.start,
			Name:  t.title,
		})
	}
	for i := 0; i < len(chapters)-1; i++ {
		chapters[i].Length = chapters[i+1].Start - chapters[i].Start
	}
	s.Chapters = chapters
	return s, nil
}

type partialTrack struct {
	start     time.Duration
	hasStart  bool
	title     string
	performer string
}

// splitDirective splits a line into its uppercase directive keyword
// and the rest of the line. Cue directives are whitespace-separated.
func splitDirective(line string) (directive, rest string) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return strings.ToUpper(line), ""
	}
	return strings.ToUpper(line[:idx]), strings.TrimSpace(line[idx+1:])
}

// splitIndex parses the payload of an INDEX directive: "<NN> MM:SS:FF".
func splitIndex(s string) (num int, timecode string, ok bool) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0, "", false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return n, parts[1], true
}

// unquote strips surrounding double quotes from a value if present.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseCueTime parses the cue-sheet MM:SS:FF format where FF is CD
// frames (75 per second).
func parseCueTime(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("cuesheet: bad timecode %q (expected MM:SS:FF)", s)
	}
	m, err1 := strconv.Atoi(parts[0])
	sec, err2 := strconv.Atoi(parts[1])
	f, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("cuesheet: non-integer component in %q", s)
	}
	if m < 0 || sec < 0 || sec >= 60 || f < 0 || f >= 75 {
		return 0, fmt.Errorf("cuesheet: component out of range in %q", s)
	}
	totalMs := int64(m)*60_000 + int64(sec)*1_000 + int64(f)*1_000/75
	return time.Duration(totalMs) * time.Millisecond, nil
}

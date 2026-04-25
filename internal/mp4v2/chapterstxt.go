package mp4v2

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// chaptersTxtExt is the sidecar extension mp4chaps expects: it strips the
// audio file's extension and appends ".chapters.txt".
const chaptersTxtExt = ".chapters.txt"

// ParseChaptersTxt reads an mp4chaps-format chapter sidecar.
//
// Format (see spec/external-tools.md §mp4chaps):
//
//	## total-duration: 12:34:56.789
//	00:00:00.000 Chapter 1
//	00:05:30.123 Chapter 2
//
// Header comments (any line starting with "##") are skipped. Each data
// line is "HH:MM:SS.mmm <title>" — the first space separates the two
// fields and everything after it is the title verbatim (including any
// further whitespace, which is not trimmed on the right).
//
// Chapter lengths are derived from the next chapter's Start. The last
// chapter's Length is left at zero; callers who know the total duration
// should fill it via [FillLastLength].
func ParseChaptersTxt(r io.Reader) ([]audio.Chapter, error) {
	var chapters []audio.Chapter
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimLeft(scanner.Text(), " \t")
		if line == "" || strings.HasPrefix(line, "##") {
			continue
		}

		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			return nil, fmt.Errorf("chapters.txt line %d: no space separating timecode and title: %q", lineNum, line)
		}

		timeStr := line[:idx]
		name := strings.TrimLeft(line[idx+1:], " \t")

		start, err := audio.ParseDuration(timeStr)
		if err != nil {
			return nil, fmt.Errorf("chapters.txt line %d: %w", lineNum, err)
		}
		chapters = append(chapters, audio.Chapter{Start: start, Name: name})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for i := 0; i < len(chapters)-1; i++ {
		chapters[i].Length = chapters[i+1].Start - chapters[i].Start
	}
	return chapters, nil
}

// FillLastLength sets the last chapter's Length from a known total
// duration, mutating the slice in place. No-op if chapters is empty.
func FillLastLength(chapters []audio.Chapter, total time.Duration) {
	if len(chapters) == 0 {
		return
	}
	last := &chapters[len(chapters)-1]
	if total > last.Start {
		last.Length = total - last.Start
	}
}

// WriteChaptersTxt writes chapters in mp4chaps sidecar format. When
// total > 0 a `## total-duration:` comment is emitted first, matching
// the format mp4chaps itself produces on export.
func WriteChaptersTxt(w io.Writer, chapters []audio.Chapter, total time.Duration) error {
	bw := bufio.NewWriter(w)
	if total > 0 {
		if _, err := fmt.Fprintf(bw, "## total-duration: %s\n", audio.FormatHMS(total)); err != nil {
			return err
		}
	}
	for _, ch := range chapters {
		// mp4chaps expects the title verbatim after a single space.
		// Any newline in Name would break the line-oriented format;
		// replace with a space to avoid corrupting the sidecar.
		name := strings.ReplaceAll(ch.Name, "\n", " ")
		if _, err := fmt.Fprintf(bw, "%s %s\n", audio.FormatHMS(ch.Start), name); err != nil {
			return err
		}
	}
	return bw.Flush()
}

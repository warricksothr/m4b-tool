package split

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
	"github.com/warricksothr/m4b-tool/internal/tag/cuesheet"
)

// ChapterSource describes how the split command should obtain chapter
// boundaries for a given input. The fields are the priority-ordered
// inputs from spec/cli-surface.md §split:
//
//	--fixed-length → --by-silence → --chapters-filename
//	→ sidecar `<basename>.chapters.txt` → cue sheet → embedded
//
// A non-zero/non-empty field higher in this list wins.
type ChapterSource struct {
	// FixedLength, if > 0, generates equal-duration chapters across
	// the input. Total input duration must be supplied.
	FixedLength time.Duration

	// BySilence requests silence-detected chapters; SilenceMin /
	// SilenceMax tune the detector. Requires the ffmpeg client.
	BySilence  bool
	SilenceMin time.Duration
	SilenceMax time.Duration

	// ChaptersFilename overrides the sidecar path. When empty, the
	// resolver falls back to `<basename>.chapters.txt` next to Input.
	ChaptersFilename string
	// UseExistingChaptersFile prefers the sidecar over the embedded
	// chapter list when both are present.
	UseExistingChaptersFile bool

	// CueSheetPath is an explicit cue file path. When empty, the
	// resolver falls back to `<basename>.cue` next to Input.
	CueSheetPath string
}

// Resolve returns the ordered chapter list per the priority above.
// Returns ErrNoChapters when nothing yields any.
func (s ChapterSource) Resolve(ctx context.Context, input string, total time.Duration, ff *ffmpeg.Client, mp *mp4v2.Client) ([]audio.Chapter, error) {
	if s.FixedLength > 0 {
		return fixedLengthChapters(total, s.FixedLength), nil
	}
	if s.BySilence {
		return chaptersBySilence(ctx, ff, input, total, s.SilenceMin, s.SilenceMax)
	}
	if s.ChaptersFilename != "" {
		return readChaptersTxt(s.ChaptersFilename)
	}
	if s.UseExistingChaptersFile {
		if chs, ok := readSidecarChaptersTxt(input); ok {
			return chs, nil
		}
	}
	if s.CueSheetPath != "" {
		return readCueChapters(s.CueSheetPath, total)
	}
	if cuePath := defaultCuePath(input); cuePath != "" {
		if chs, err := readCueChapters(cuePath, total); err == nil {
			return chs, nil
		}
	}
	if chs, ok := readSidecarChaptersTxt(input); ok {
		return chs, nil
	}
	if ff != nil {
		if tag, err := ff.ReadFFMetadata(ctx, input); err == nil && len(tag.Chapters) > 0 {
			return tag.Chapters, nil
		}
	}
	// mp4chaps fallback for MP4-family inputs. ffmpeg's -f ffmetadata
	// silently drops chapters on files with a malformed timescale even
	// when the chapter atom itself is intact; mp4v2 reads the atom
	// directly and is unaffected.
	if mp != nil && isMP4FamilyExt(input) {
		if chs, err := mp.ListChapters(ctx, input); err == nil && len(chs) > 0 {
			return chs, nil
		}
	}
	return nil, ErrNoChapters
}

// ErrNoChapters indicates that no chapter source produced any chapters.
// The wrapping caller in split.Run formats this with a hint pointing at
// the fallback flags (`--by-silence`, `--fixed-length`, sidecar paths).
var ErrNoChapters = errors.New("no chapter source resolved any chapters")

// isMP4FamilyExt reports whether the input's extension belongs to the
// MP4 family that mp4v2's chapter reader understands. Mirrors
// split.isMP4Family but takes a path so it's callable from this file
// without an extension argument round-trip.
func isMP4FamilyExt(input string) bool {
	switch strings.ToLower(filepath.Ext(input)) {
	case ".m4a", ".m4b", ".mp4":
		return true
	}
	return false
}

// fixedLengthChapters yields ceil(total / length) chapters of length L,
// with the final chapter trimmed to the remainder. Names are "1", "2",
// ... — orchestrators that want themed names should override after.
func fixedLengthChapters(total, length time.Duration) []audio.Chapter {
	if length <= 0 || total <= 0 {
		return nil
	}
	var chs []audio.Chapter
	for start, i := time.Duration(0), 1; start < total; start, i = start+length, i+1 {
		end := start + length
		if end > total {
			end = total
		}
		chs = append(chs, audio.Chapter{
			Start:  start,
			Length: end - start,
			Name:   strconv.Itoa(i),
		})
	}
	return chs
}

func chaptersBySilence(ctx context.Context, ff *ffmpeg.Client, input string, total, minLen, maxLen time.Duration) ([]audio.Chapter, error) {
	if ff == nil {
		return nil, errors.New("split: --by-silence requires ffmpeg client")
	}
	if minLen <= 0 {
		minLen = 1750 * time.Millisecond
	}
	silences, err := ff.DetectSilence(ctx, input, minLen, maxLen)
	if err != nil {
		return nil, fmt.Errorf("detect silence: %w", err)
	}
	// Each silence boundary becomes a cut: chapter N ends at the
	// silence start. Nothing fancy here — silence-aligned splitting is
	// already wired up on the merge/chapters side; for split we just
	// want a clean boundary between segments.
	var chs []audio.Chapter
	cursor := time.Duration(0)
	for i, s := range silences {
		if s.Start <= cursor {
			continue
		}
		chs = append(chs, audio.Chapter{
			Start:  cursor,
			Length: s.Start - cursor,
			Name:   strconv.Itoa(i + 1),
		})
		cursor = s.End()
	}
	if cursor < total {
		chs = append(chs, audio.Chapter{
			Start:  cursor,
			Length: total - cursor,
			Name:   strconv.Itoa(len(chs) + 1),
		})
	}
	if len(chs) == 0 {
		return nil, ErrNoChapters
	}
	return chs, nil
}

// readChaptersTxt reads an mp4v2-style chapters.txt file. Length is
// derived from the next chapter's start (last chapter's length is left
// zero for the orchestrator to fill in from the input duration).
func readChaptersTxt(path string) ([]audio.Chapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open chapters file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	chs, err := mp4v2.ParseChaptersTxt(f)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	for i := 0; i+1 < len(chs); i++ {
		chs[i].Length = chs[i+1].Start - chs[i].Start
	}
	return chs, nil
}

// readSidecarChaptersTxt looks for `<base>.chapters.txt` next to the
// audio file (mp4chaps convention). Returns (nil, false) when missing
// or unreadable.
func readSidecarChaptersTxt(audioPath string) ([]audio.Chapter, bool) {
	side := chaptersTxtSidecarPath(audioPath)
	if _, err := os.Stat(side); err != nil {
		return nil, false
	}
	chs, err := readChaptersTxt(side)
	if err != nil {
		return nil, false
	}
	return chs, true
}

// chaptersTxtSidecarPath returns the conventional sidecar path: the
// audio file's path with its extension replaced by ".chapters.txt".
// Mirrors mp4chaps's behavior; kept here to avoid exporting the
// internal helper from package mp4v2.
func chaptersTxtSidecarPath(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return strings.TrimSuffix(audioPath, ext) + ".chapters.txt"
}

func readCueChapters(path string, total time.Duration) ([]audio.Chapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cue %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	sheet, err := cuesheet.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse cue %q: %w", path, err)
	}
	chs := sheet.Chapters
	if len(chs) == 0 {
		return nil, ErrNoChapters
	}
	for i := 0; i+1 < len(chs); i++ {
		chs[i].Length = chs[i+1].Start - chs[i].Start
	}
	if total > 0 {
		last := &chs[len(chs)-1]
		if last.Length == 0 {
			last.Length = total - last.Start
		}
	}
	return chs, nil
}

func defaultCuePath(audioPath string) string {
	ext := filepath.Ext(audioPath)
	candidate := strings.TrimSuffix(audioPath, ext) + ".cue"
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// FillTrailingLength sets the final chapter's Length to total-Start
// when it's zero. mp4v2 chapter sidecars don't encode a closing
// boundary, so the caller has to do this after reading.
func FillTrailingLength(chs []audio.Chapter, total time.Duration) {
	if len(chs) == 0 || total <= 0 {
		return
	}
	last := &chs[len(chs)-1]
	if last.Length == 0 && total > last.Start {
		last.Length = total - last.Start
	}
}

// RenameOptions captures the post-resolution chapter Name transforms
// applied by both `split` (via its rename flags) and `chapters export`
// (so users can preview the same transforms in the sidecar before
// committing to a split). Pipeline order is fixed: reindex first,
// then strip-title, then prefix.
type RenameOptions struct {
	Reindex    bool
	StripTitle bool
	Prefix     string
}

// ApplyRename mutates chs per opts. The fixed order matches the CLI
// flag descriptions and lets users predict the result without reading
// the source: --reindex-chapters wins over the source name, --strip-title
// then trims its zero padding, and --chapter-prefix labels what's left.
func ApplyRename(chs []audio.Chapter, opts RenameOptions) {
	if opts.Reindex {
		ReindexChapters(chs)
	}
	if opts.StripTitle {
		StripChapterTitles(chs)
	}
	if opts.Prefix != "" {
		for i := range chs {
			chs[i].Name = opts.Prefix + chs[i].Name
		}
	}
}

// ReindexChapters replaces every chapter's Name with its 1-based index
// rendered as a decimal string. Used for --reindex-chapters.
func ReindexChapters(chs []audio.Chapter) {
	for i := range chs {
		chs[i].Name = strconv.Itoa(i + 1)
	}
}

// StripChapterTitles removes leading whitespace and zero characters from
// every chapter's Name. Used for --strip-title; pairs with
// --chapter-prefix to turn "001" into "Chapter 1" without editing the
// sidecar. Names that are entirely whitespace or zeros collapse to "0"
// rather than the empty string so a downstream prefix doesn't render
// as a trailing space (e.g. "Chapter ").
func StripChapterTitles(chs []audio.Chapter) {
	for i := range chs {
		stripped := strings.TrimLeft(chs[i].Name, " \t0")
		if stripped == "" && chs[i].Name != "" {
			stripped = "0"
		}
		chs[i].Name = stripped
	}
}

// numericIndexFraction returns the fraction of chapters whose Name
// parses as a decimal integer matching their 1-based position. Leading
// zeros are accepted ("001" matches position 1) so the common case of
// zero-padded numeric titles is detected. Used by Run to emit a
// quality-of-life warning when a file's chapter atoms only carry
// numeric indices — at that point the user can't tell tracks apart by
// filename without a custom prefix or an edited sidecar.
func numericIndexFraction(chs []audio.Chapter) float64 {
	if len(chs) == 0 {
		return 0
	}
	matches := 0
	for i, ch := range chs {
		n, err := strconv.Atoi(strings.TrimLeft(ch.Name, "0"))
		// strings.TrimLeft("001", "0") → "1", but "0" → "" — guard
		// the empty case so a title of "0" still parses as zero.
		if err != nil && ch.Name != "" && strings.Trim(ch.Name, "0") == "" {
			n, err = 0, nil
		}
		if err != nil {
			continue
		}
		if n == i+1 {
			matches++
		}
	}
	return float64(matches) / float64(len(chs))
}

package mp4v2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/exec"
)

// WriteChapters imports chapters into path using the mp4chaps sidecar
// protocol: write a FILE.chapters.txt alongside the audio, run
// `mp4chaps -i FILE`, then remove the sidecar.
//
// mp4chaps names the sidecar by stripping path's extension and appending
// ".chapters.txt" (see mp4chaps source `parseChapterFile`). WriteChapters
// owns this sidecar path for the duration of the call: any existing
// file at that location is truncated. Callers that want to preserve a
// user-curated sidecar should read it (via the chapterstxt importer)
// before invoking WriteChapters; the chapters in the in-memory slice
// are the authoritative source here.
func (c *Client) WriteChapters(ctx context.Context, path string, chapters []audio.Chapter, total time.Duration) error {
	if len(chapters) == 0 {
		return nil
	}
	sidecar := sidecarPath(path)

	f, err := os.OpenFile(sidecar, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("mp4chaps sidecar create: %w", err)
	}
	sidecarCreated := true
	defer func() {
		if sidecarCreated {
			_ = os.Remove(sidecar)
		}
	}()

	if err := WriteChaptersTxt(f, chapters, total); err != nil {
		_ = f.Close()
		return fmt.Errorf("mp4chaps sidecar write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("mp4chaps sidecar close: %w", err)
	}

	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Chaps,
		Args: []string{"-i", path},
	}); err != nil {
		return fmt.Errorf("mp4chaps -i: %w", err)
	}
	return nil
}

// reMp4ChapsListLine matches one row of `mp4chaps -l` output:
//
//	Chapter #001 - 00:00:00.000 - "Opening Credits"
//
// The chapter index is captured for completeness but ignored — Resolve
// renumbers via FillTrailingLength + ReindexChapters anyway. The
// timestamp uses the same HH:MM:SS.ms grammar audio.ParseDuration
// understands. Title is captured greedily up to the closing quote on
// the same line; titles with embedded `"` would technically need
// escape handling, but mp4chaps's listing format does not escape, so
// the only way to break parsing is to put a literal `"` followed by
// end-of-line in the chapter title — vanishingly rare.
var reMp4ChapsListLine = regexp.MustCompile(`^\s*Chapter #(\d+)\s+-\s+(\d+:\d+:\d+\.\d+)\s+-\s+"(.*)"\s*$`)

// ListChapters runs `mp4chaps -l` and parses its stdout into the
// project's chapter representation. Returns an empty slice (not an
// error) when the file has no chapters — callers treat that as a
// resolver miss rather than an error.
//
// Chapter Length is filled in pair-wise from the next chapter's
// Start; the last chapter's Length stays zero so the caller can
// supply the input's total duration via FillTrailingLength.
//
// This wraps mp4v2's chapter-atom reader, which understands more
// MP4 chapter formats than `ffmpeg -f ffmetadata` does — the latter
// silently drops chapters when the file's timescale is malformed
// even if the chapter track itself is well-formed. mp4chaps is the
// reliable fallback for the split command's chapter resolver.
func (c *Client) ListChapters(ctx context.Context, path string) ([]audio.Chapter, error) {
	res, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Chaps,
		Args: []string{"-l", path},
	})
	if err != nil {
		return nil, fmt.Errorf("mp4chaps -l: %w", err)
	}
	return parseMp4ChapsList(string(res.Stdout))
}

// parseMp4ChapsList scans mp4chaps's `-l` output and returns the
// chapter list. Lines that don't match the chapter format (the
// "QuickTime Chapters of ..." header, blank lines, etc.) are skipped.
// Some files carry both QuickTime and Nero chapter atoms; mp4chaps
// then prints both lists. We dedupe on Start time, keeping the first
// occurrence — typically QuickTime, which is the format mp4chaps
// writes by default.
func parseMp4ChapsList(stdout string) ([]audio.Chapter, error) {
	var (
		chs  []audio.Chapter
		seen = map[time.Duration]bool{}
	)
	for _, line := range strings.Split(stdout, "\n") {
		m := reMp4ChapsListLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, err := audio.ParseDuration(m[2])
		if err != nil {
			return nil, fmt.Errorf("mp4chaps -l: parse start %q: %w", m[2], err)
		}
		if seen[start] {
			continue
		}
		seen[start] = true
		chs = append(chs, audio.Chapter{Start: start, Name: m[3]})
	}
	for i := 0; i+1 < len(chs); i++ {
		chs[i].Length = chs[i+1].Start - chs[i].Start
	}
	return chs, nil
}

// RemoveChapters strips chapter atoms from the MP4 file at path via
// `mp4chaps -r`.
func (c *Client) RemoveChapters(ctx context.Context, path string) error {
	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Chaps,
		Args: []string{"-r", path},
	}); err != nil {
		return fmt.Errorf("mp4chaps -r: %w", err)
	}
	return nil
}

// sidecarPath derives the chapters sidecar filename mp4chaps expects
// when invoked as `mp4chaps -i FILE`: FILE's extension is stripped and
// ".chapters.txt" is appended in the same directory.
func sidecarPath(audioPath string) string {
	dir, base := filepath.Split(audioPath)
	ext := filepath.Ext(base)
	return filepath.Join(dir, strings.TrimSuffix(base, ext)+chaptersTxtExt)
}

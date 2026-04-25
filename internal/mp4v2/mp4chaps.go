package mp4v2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

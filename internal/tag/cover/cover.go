// Package cover implements the `cover` tag importer: finds a cover
// image (cover.jpg / .jpeg / .png) in the working directory and sets
// Tag.CoverPath. See spec/tag-importers/cover.md.
package cover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// CandidateFilenames is the discovery order for cover files inside the
// working directory. First match wins.
var CandidateFilenames = []string{"cover.jpg", "cover.jpeg", "cover.png"}

// Options configures the importer.
type Options struct {
	// Skip forces the importer to no-op, used for --skip-cover.
	Skip bool
}

// Importer discovers a cover image in workDir.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "cover" }

// Improve sets CoverPath to the first existing candidate file in
// workDir. Does nothing when CoverPath is already set (MergeMissing),
// when Skip is true, or when no candidate exists. Zero-byte files are
// ignored: mp4art would fail on them anyway and a zero cover is never
// what the user wants.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, workDir string) (audio.Tag, error) {
	if i.opts.Skip || tag.CoverPath != "" {
		return tag, nil
	}
	for _, name := range CandidateFilenames {
		path := filepath.Join(workDir, name)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return tag, fmt.Errorf("stat %s: %w", path, err)
		}
		if info.Size() == 0 {
			continue
		}
		resolved, err := filepath.Abs(path)
		if err != nil {
			return tag, fmt.Errorf("abs %s: %w", path, err)
		}
		tag.CoverPath = resolved
		return tag, nil
	}
	return tag, nil
}

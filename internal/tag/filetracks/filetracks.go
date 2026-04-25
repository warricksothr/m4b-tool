// Package filetracks implements the `chapters-from-file-tracks`
// importer: derives base chapters from the list of input files being
// merged, one chapter per file. See spec/tag-importers/chapters-from-file-tracks.md.
package filetracks

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Track describes one input file for the merge operation: its path,
// its pre-probed duration, and any embedded title tag.
type Track struct {
	Path     string
	Duration time.Duration
	Title    string
}

// Options configures the importer.
type Options struct {
	// Tracks is the ordered list of input files.
	Tracks []Track
	// UseFilenames, when true, forces the chapter name to be the
	// filename stem even if the track has an embedded Title tag.
	// Matches --use-filenames-as-chapters.
	UseFilenames bool
	// Gap is the silence inserted between consecutive tracks
	// (--add-silence). Chapter starts include the cumulative gap; the
	// gap appears N-1 times for N tracks (no gap after the last).
	// Zero means contiguous tracks.
	Gap time.Duration
}

// Importer builds chapters from the input file list.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "chapters-from-file-tracks" }

// Improve writes a chapter per input file into Tag.Chapters. Starts
// accumulate from zero using each track's Duration. Because this
// importer is foundational — it runs first in the merge composition —
// it unconditionally sets Chapters. Subsequent importers
// (chapters-txt, cuesheet) may overwrite when they apply.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, _ string) (audio.Tag, error) {
	if len(i.opts.Tracks) == 0 {
		return tag, nil
	}
	out := make([]audio.Chapter, 0, len(i.opts.Tracks))
	var cursor time.Duration
	for idx, tr := range i.opts.Tracks {
		if idx > 0 && i.opts.Gap > 0 {
			cursor += i.opts.Gap
		}
		name := tr.Title
		if i.opts.UseFilenames || name == "" {
			name = filenameStem(tr.Path)
		}
		out = append(out, audio.Chapter{
			Start:  cursor,
			Length: tr.Duration,
			Name:   name,
		})
		cursor += tr.Duration
	}
	tag.Chapters = out
	return tag, nil
}

func filenameStem(p string) string {
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

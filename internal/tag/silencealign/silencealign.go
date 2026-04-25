// Package silencealign implements the `guess-chapters-by-silence`
// importer: a thin wrapper around chapter.AlignToSilence that fits the
// tag.Importer interface. See spec/tag-importers/guess-chapters-by-silence.md.
package silencealign

import (
	"context"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/chapter"
)

// Options configures the importer. Callers supply pre-computed silences
// and total duration so the importer itself does no I/O and no
// subprocess work — it just runs the pure algorithm.
type Options struct {
	Silences       []audio.Silence
	Total          time.Duration
	MaxDiff        time.Duration // 0 → spec default (25s)
	LastChapterMin time.Duration // 0 → spec default (2500ms)
}

// Importer snaps chapter boundaries onto silences.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "guess-chapters-by-silence" }

// Improve runs chapter.AlignToSilence on tag.Chapters. No silences and
// no chapters both yield a no-op. Returns the tag with an updated
// Chapters slice.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, _ string) (audio.Tag, error) {
	if len(tag.Chapters) == 0 || len(i.opts.Silences) == 0 {
		return tag, nil
	}
	tag.Chapters = chapter.AlignToSilence(tag.Chapters, i.opts.Silences, chapter.AlignOptions{
		Total:          i.opts.Total,
		MaxDiff:        i.opts.MaxDiff,
		LastChapterMin: i.opts.LastChapterMin,
	})
	return tag, nil
}

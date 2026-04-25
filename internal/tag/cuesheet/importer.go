package cuesheet

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Options configures the cuesheet importer.
type Options struct {
	// Path is the explicit cue file path. Empty = skip importer.
	Path string
}

// Importer wraps the cuesheet parser as a tag.Importer.
// See spec/tag-importers/cuesheet.md.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "cuesheet" }

// Improve parses the cue sheet and merges its contents into tag: scalar
// fields via MergeMissing (don't clobber more specific sources), and
// chapters OVERWRITE any existing list (cue sheets are authoritative
// when present).
func (i *Importer) Improve(_ context.Context, tag audio.Tag, _ string) (audio.Tag, error) {
	if i.opts.Path == "" {
		return tag, nil
	}
	f, err := os.Open(i.opts.Path)
	if errors.Is(err, os.ErrNotExist) {
		return tag, nil
	}
	if err != nil {
		return tag, fmt.Errorf("open %s: %w", i.opts.Path, err)
	}
	defer func() { _ = f.Close() }()

	sheet, err := Parse(f)
	if err != nil {
		return tag, fmt.Errorf("parse %s: %w", i.opts.Path, err)
	}
	// Scalar fields: MergeMissing.
	tag.MergeMissing(sheet.Tag)
	// Chapters: overwrite.
	if len(sheet.Chapters) > 0 {
		tag.Chapters = sheet.Chapters
	}
	return tag, nil
}

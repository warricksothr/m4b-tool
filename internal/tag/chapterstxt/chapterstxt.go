// Package chapterstxt implements the `chapters-txt` tag importer:
// loads chapters from an mp4chaps-format sidecar file
// (`<basename>.chapters.txt`) and overwrites Tag.Chapters.
// See spec/tag-importers/chapters-txt.md.
package chapterstxt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// Options configures the importer.
type Options struct {
	// AudioPath is the audio file whose sidecar to look for. The
	// sidecar name is AudioPath with its extension replaced by
	// ".chapters.txt".
	AudioPath string
	// Override takes precedence over AudioPath-derived discovery,
	// used for --chapters-filename.
	Override string
}

// Importer loads a chapters.txt sidecar.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "chapters-txt" }

// Improve reads the sidecar and overwrites Tag.Chapters with its
// contents. Absent sidecar is a no-op.
//
// Per spec, this importer overwrites: if the user dropped a
// chapters.txt next to the audio file, they want those chapters.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, _ string) (audio.Tag, error) {
	path := i.opts.Override
	if path == "" {
		if i.opts.AudioPath == "" {
			return tag, nil
		}
		path = sidecarPath(i.opts.AudioPath)
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return tag, nil
	}
	if err != nil {
		return tag, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	chapters, err := mp4v2.ParseChaptersTxt(f)
	if err != nil {
		return tag, fmt.Errorf("parse %s: %w", path, err)
	}
	tag.Chapters = chapters
	return tag, nil
}

// sidecarPath derives the `<basename>.chapters.txt` name from the
// audio file path by stripping the extension and appending.
func sidecarPath(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return strings.TrimSuffix(audioPath, ext) + ".chapters.txt"
}

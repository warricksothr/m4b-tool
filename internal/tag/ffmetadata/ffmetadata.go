// Package ffmetadata implements the `ffmetadata` tag importer.
// See spec/tag-importers/ffmetadata.md.
package ffmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// DefaultFilename is the sidecar name this importer looks for when no
// explicit path is supplied.
const DefaultFilename = "ffmetadata.txt"

// Options configures the importer.
type Options struct {
	// Path, when set, is the exact sidecar path. Overrides workDir-
	// based discovery. A non-empty Path that does not exist is NOT an
	// error — the importer returns the input Tag unchanged.
	Path string
}

// Importer loads an FFMETADATA1 sidecar and merges its contents into
// the Tag using MergeMissing policy.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "ffmetadata" }

// Improve reads the FFMETADATA sidecar (if present) and fills in zero
// fields of tag with its contents.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, workDir string) (audio.Tag, error) {
	path := i.opts.Path
	if path == "" {
		path = filepath.Join(workDir, DefaultFilename)
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return tag, nil
	}
	if err != nil {
		return tag, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	parsed, err := ffmpeg.ParseFFMetadata(f)
	if err != nil {
		return tag, fmt.Errorf("parse %s: %w", path, err)
	}
	tag.MergeMissing(*parsed)
	return tag, nil
}

// Package description implements the `description` tag importer: reads
// a plaintext description.txt sidecar and maps it to Description (short,
// first paragraph) and LongDescription (whole body).
// See spec/tag-importers/description.md.
package description

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// DefaultFilename is the sidecar name relative to workDir.
const DefaultFilename = "description.txt"

// Options configures the importer.
type Options struct {
	// Path, when set, overrides the default sidecar discovery.
	Path string
}

// Importer reads a plain-text description sidecar.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "description" }

// Improve reads description.txt (if present) and fills Description and
// LongDescription when they are empty. CRLF is normalized to LF.
// Description is the first paragraph (text before the first blank line),
// or the entire body when no blank line exists.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, workDir string) (audio.Tag, error) {
	path := i.opts.Path
	if path == "" {
		path = filepath.Join(workDir, DefaultFilename)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tag, nil
	}
	if err != nil {
		return tag, fmt.Errorf("read %s: %w", path, err)
	}
	body := strings.ReplaceAll(string(data), "\r\n", "\n")
	body = strings.TrimRight(body, " \n\t")
	if body == "" {
		return tag, nil
	}
	short, _, _ := strings.Cut(body, "\n\n")
	if tag.Description == "" {
		tag.Description = short
	}
	if tag.LongDescription == "" {
		tag.LongDescription = body
	}
	return tag, nil
}

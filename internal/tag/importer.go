package tag

import (
	"context"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Importer is a single metadata source. Given an in-progress Tag and
// the command's working directory (typically where sidecar files live),
// it returns an enriched Tag. Importers must not mutate their input:
// they take Tag by value and return a new Tag.
//
// Merge policy is an importer-level concern, not part of the interface:
// most importers use MergeMissing semantics internally, but a few
// (chapters-txt, cuesheet) overwrite specific fields when their source
// is authoritative. Each importer's doc-stub in spec/tag-importers/
// names the policy it uses.
type Importer interface {
	// Name returns the canonical, hyphen-lowercase name used for
	// --enable-improvers and --disable-improvers filtering.
	Name() string
	// Improve runs the importer. Returning an error aborts the composite.
	// Missing optional sidecars are NOT errors — importers should return
	// the input Tag unchanged and nil error when their source file is
	// absent.
	Improve(ctx context.Context, tag audio.Tag, workDir string) (audio.Tag, error)
}

// Package tag defines the Importer interface and the Composite runner
// that chains importers together.
//
// A tag importer takes a partially-populated Tag plus a working
// directory and returns an enriched Tag. Each importer focuses on a
// single metadata source (FFMETADATA sidecar, chapters.txt, cue sheet,
// cover image discovery, silence-based chapter alignment, ...).
// Concrete importers live in subpackages under internal/tag/<name>.
//
// See spec/tag-importers/README.md for the composition order used by
// the merge, split, and chapters commands.
package tag

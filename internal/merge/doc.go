// Package merge implements the `merge` command orchestrator: combine a
// set of audio files into a single m4b (or mp4/m4a) with chapters and
// tags.
//
// M7a scope (this slice): file collection, stream-copy concat via
// ffmpeg -f concat, tag importer composition, CLI tag overrides,
// cover/chapter/tag write-back via mp4v2, and --dry-run. Transcoding
// options, --trim-silence, --add-silence, --jobs parallelism, and
// batch mode land in later slices of this milestone.
package merge

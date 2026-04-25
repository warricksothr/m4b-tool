// Package chapters implements the `chapters` command orchestrator:
// read an audio file's existing chapter markers, optionally adjust
// them (silence-snap, normalize, shift, merge-similar, intro/outro
// offsets), and write them back — either to the audio file via
// mp4chaps -i or to a text sidecar via --output-file.
//
// All pure logic lives in internal/chapter; this package wires it to
// the ffmpeg and mp4v2 clients and the CLI flag surface defined in
// spec/cli-surface.md §chapters.
package chapters

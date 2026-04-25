// Package cuesheet parses CD-audio cue sheets into the m4b-tool data
// model: a Tag with disc-level metadata plus one Chapter per TRACK.
//
// The subset supported is described in spec/chapter-algorithms.md
// §Cue sheet parsing — just enough to drive the split command on
// .flac + .cue inputs.
package cuesheet

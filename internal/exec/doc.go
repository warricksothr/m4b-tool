// Package exec centralizes subprocess execution for the rest of the tool.
//
// Every external-tool wrapper (ffmpeg, mp4v2 suite, fdkaac, tone) builds a
// [Cmd] and calls [Run]; direct use of os/exec is discouraged. The runner
// handles timeouts, merged-output capture, optional line-oriented stderr
// streaming (for incremental parsers like ffmpeg's silencedetect), and a
// terminate callback for caller-side cleanup of temp files.
package exec

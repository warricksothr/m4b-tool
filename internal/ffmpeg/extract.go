package ffmpeg

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// ExtractOptions configures one chapter-range extraction.
//
// Encoding fields mirror EncodeOptions; when Codec is empty the extract
// uses stream-copy (`-c:a copy`) — the cheap, lossless fast path that
// preserves the source codec, which is correct for "split this m4b
// into chapters but don't re-encode".
type ExtractOptions struct {
	Format     string
	Codec      string
	Bitrate    string
	SampleRate int
	Channels   int
	Threads    int
	ExtraArgs  []string
	// Metadata is appended as -metadata key=value pairs to the ffmpeg
	// invocation. Used for non-MP4 outputs where post-extraction
	// tagging via mp4tags doesn't apply: ID3v2 for MP3, Vorbis comments
	// for FLAC, etc. — the muxer picks the on-disk frame format.
	Metadata  map[string]string
	Overwrite bool
}

// ExtractSegment writes the [start, start+duration) range of in to
// out, optionally re-encoding per opts. Used by the split command for
// per-chapter extraction.
//
// We use the demuxer-side "-ss before -i" form to seek with the input
// timestamps, which is faster than decoder-side seeking and accurate
// enough for chapter-grain splits.
func (c *Client) ExtractSegment(ctx context.Context, in, out string, start, duration time.Duration, opts ExtractOptions) error {
	args := buildExtractArgs(in, out, start, duration, opts)
	if _, err := exec.Run(ctx, exec.Cmd{Name: c.Bin, Args: args}); err != nil {
		return fmt.Errorf("ffmpeg extract %s [%s+%s]: %w", in, start, duration, err)
	}
	return nil
}

// buildExtractArgs is split out for unit testing.
func buildExtractArgs(in, out string, start, duration time.Duration, opts ExtractOptions) []string {
	args := []string{"-hide_banner"}
	if opts.Overwrite {
		args = append(args, "-y")
	}
	if start > 0 {
		args = append(args, "-ss", formatSeconds3(start))
	}
	args = append(args, "-i", in)
	if duration > 0 {
		args = append(args, "-t", formatSeconds3(duration))
	}
	if opts.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(opts.Threads))
	}
	args = append(args, "-vn")

	codec := opts.Codec
	if codec == "" {
		codec = "copy"
	}
	args = append(args, "-c:a", codec)
	if codec != "copy" {
		if opts.Bitrate != "" {
			args = append(args, "-b:a", opts.Bitrate)
		}
		if opts.SampleRate > 0 {
			args = append(args, "-ar", strconv.Itoa(opts.SampleRate))
		}
		if opts.Channels > 0 {
			args = append(args, "-ac", strconv.Itoa(opts.Channels))
		}
	}
	args = append(args, opts.ExtraArgs...)
	// -metadata pairs go after codec/extra args but before -f, so the
	// muxer sees them when it writes its tag frames. Sort keys for
	// argv determinism (matters for tests and logs).
	if len(opts.Metadata) > 0 {
		keys := make([]string, 0, len(opts.Metadata))
		for k := range opts.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-metadata", k+"="+opts.Metadata[k])
		}
	}
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	}
	args = append(args, out)
	return args
}

// formatSeconds3 prints a duration as decimal seconds with millisecond
// precision (e.g., "12.345"), the form ffmpeg accepts on -ss / -t.
func formatSeconds3(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}

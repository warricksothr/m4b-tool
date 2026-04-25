package ffmpeg

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// EncodeOptions configures a single-input transcode.
//
// The zero value is a valid "let ffmpeg pick everything" call;
// callers typically set Codec, Bitrate, and Format explicitly.
type EncodeOptions struct {
	// Format is the container ffmpeg muxer name. "mp4" for m4b/mp4/m4a
	// outputs (since ffmpeg has no "m4b" muxer); "mp3" for MP3.
	Format string
	// Codec is the audio codec name passed via -c:a, e.g. "aac",
	// "libfdk_aac", "libmp3lame". Empty leaves ffmpeg's default.
	Codec string
	// Bitrate is the -b:a value, e.g. "64k", "128k". Empty leaves the
	// codec default.
	Bitrate string
	// SampleRate is the -ar value (Hz). Zero leaves the source rate.
	SampleRate int
	// Channels is the -ac value. Zero leaves the source channel count.
	Channels int
	// Threads, if > 0, passes "-threads N" to this invocation.
	Threads int
	// TrimSilenceStart removes any silence at the start of the input.
	TrimSilenceStart bool
	// TrimSilenceEnd removes any silence at the end of the input.
	TrimSilenceEnd bool
	// SilenceThreshold is the dB threshold for trim-silence; zero
	// uses the default of -30 dBFS.
	SilenceThreshold string
	// ExtraArgs are appended just before the output filename (after
	// codec/format flags). Used for --ffmpeg-param passthrough.
	ExtraArgs []string
	// Overwrite passes "-y" so ffmpeg will clobber the output.
	Overwrite bool
}

// Transcode encodes in into out per opts. The arguments are built and
// passed verbatim to ffmpeg; opts is not validated beyond simple
// non-empty checks.
func (c *Client) Transcode(ctx context.Context, in, out string, opts EncodeOptions) error {
	args := buildTranscodeArgs(in, out, opts)
	if _, err := exec.Run(ctx, exec.Cmd{Name: c.Bin, Args: args}); err != nil {
		return fmt.Errorf("ffmpeg transcode %s: %w", in, err)
	}
	return nil
}

// SilenceOptions configures [Client.SynthesizeSilence]. The default
// (zero) value produces 44.1 kHz stereo silence in the AAC-in-MP4
// container, which is the parity case for typical audiobook output.
type SilenceOptions struct {
	Format     string // muxer; default "mp4"
	Codec      string // default "aac"
	Bitrate    string // -b:a; default "64k"
	SampleRate int    // default 44100
	Channels   int    // default 2
	Overwrite  bool
}

// SynthesizeSilence writes a silent audio segment of length seconds in
// the configured container/codec. Used to interleave between parts
// when --add-silence is set; the synthesized segment must share codec
// parameters with the encoded parts so concat-demuxer stream-copy
// still works.
func (c *Client) SynthesizeSilence(ctx context.Context, out string, seconds float64, opts SilenceOptions) error {
	args := buildSilenceArgs(out, seconds, opts)
	if _, err := exec.Run(ctx, exec.Cmd{Name: c.Bin, Args: args}); err != nil {
		return fmt.Errorf("ffmpeg synth silence: %w", err)
	}
	return nil
}

// buildTranscodeArgs is split out from Transcode so the argv can be
// asserted in unit tests without touching the binary.
func buildTranscodeArgs(in, out string, opts EncodeOptions) []string {
	args := []string{"-hide_banner"}
	if opts.Overwrite {
		args = append(args, "-y")
	}
	args = append(args, "-i", in)
	if opts.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(opts.Threads))
	}
	args = append(args, "-vn")

	if filter := buildSilenceRemoveFilter(opts); filter != "" {
		args = append(args, "-af", filter)
	}

	if opts.Codec != "" {
		args = append(args, "-c:a", opts.Codec)
	}
	if opts.Bitrate != "" {
		args = append(args, "-b:a", opts.Bitrate)
	}
	if opts.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(opts.SampleRate))
	}
	if opts.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(opts.Channels))
	}
	args = append(args, opts.ExtraArgs...)
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	}
	args = append(args, out)
	return args
}

// buildSilenceRemoveFilter assembles the -af expression for the
// requested combination of start/end trim. Returns "" when no trimming
// is requested.
//
// The filter uses ffmpeg's silenceremove with a peak detector at the
// configured threshold (default -30 dBFS). End-trim is implemented by
// reversing the stream, removing the new "start" silence, and
// reversing back.
func buildSilenceRemoveFilter(opts EncodeOptions) string {
	if !opts.TrimSilenceStart && !opts.TrimSilenceEnd {
		return ""
	}
	threshold := opts.SilenceThreshold
	if threshold == "" {
		threshold = "-30dB"
	}
	trim := fmt.Sprintf("silenceremove=start_periods=1:start_threshold=%s:start_silence=0:detection=peak", threshold)

	var parts []string
	if opts.TrimSilenceStart {
		parts = append(parts, trim)
	}
	if opts.TrimSilenceEnd {
		parts = append(parts, "areverse", trim, "areverse")
	}
	return strings.Join(parts, ",")
}

func buildSilenceArgs(out string, seconds float64, opts SilenceOptions) []string {
	format := opts.Format
	if format == "" {
		format = "mp4"
	}
	codec := opts.Codec
	if codec == "" {
		codec = "aac"
	}
	bitrate := opts.Bitrate
	if bitrate == "" {
		bitrate = "64k"
	}
	rate := opts.SampleRate
	if rate == 0 {
		rate = 44100
	}
	channels := opts.Channels
	if channels == 0 {
		channels = 2
	}
	channelLayout := "stereo"
	if channels == 1 {
		channelLayout = "mono"
	}

	args := []string{"-hide_banner"}
	if opts.Overwrite {
		args = append(args, "-y")
	}
	args = append(args,
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=channel_layout=%s:sample_rate=%d", channelLayout, rate),
		"-t", strconv.FormatFloat(seconds, 'f', 3, 64),
		"-c:a", codec,
		"-b:a", bitrate,
		"-ar", strconv.Itoa(rate),
		"-ac", strconv.Itoa(channels),
		"-f", format,
		out,
	)
	return args
}

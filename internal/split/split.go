package split

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/jobs"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// Config drives Run. Fields mirror the CLI flags in cli-surface.md
// §split; see cmd/m4b-tool/split.go for the user-facing names.
type Config struct {
	Input     string
	OutputDir string

	// FilenameTemplate is the user-provided text/template, or empty for
	// the default `{{printf "%03d" .Track}}-{{.Title}}`.
	FilenameTemplate string

	// Force overwrites existing per-chapter files without prompting.
	Force bool

	// DryRun prints the plan without invoking ffmpeg.
	DryRun bool

	// Chapter source (priority described in ChapterSource.Resolve).
	FixedLength             time.Duration
	BySilence               bool
	SilenceMin              time.Duration
	SilenceMax              time.Duration
	ChaptersFilename        string
	UseExistingChaptersFile bool
	CueSheetPath            string

	// ReindexChapters renames every chapter to its 1-based index.
	ReindexChapters bool

	// StripTitle drops leading whitespace and zero characters from each
	// chapter's Name. Runs after ReindexChapters and before ChapterPrefix,
	// so "001" + prefix "Chapter " becomes "Chapter 1" without touching
	// the sidecar. The all-zero/all-whitespace edge case collapses to
	// "0" rather than empty, so the prefix never trails into "Chapter ".
	StripTitle bool

	// ChapterPrefix, when non-empty, is prepended verbatim to each
	// chapter's Name after ReindexChapters and StripTitle run. Use this
	// to turn numeric-only source names (the M4B has chapters titled
	// "001", "002", ...) into something more readable like
	// "Chapter 001" — or "Chapter 1" with --strip-title — without
	// editing a sidecar.
	ChapterPrefix string

	// AudioFormat is the output container, e.g. "m4a", "mp3". When
	// empty, the input's extension is reused. Drives both the output
	// filename suffix and the ffmpeg muxer choice.
	AudioFormat string

	// Encoding (optional; empty Codec means stream-copy).
	AudioCodec      string
	AudioBitrate    string
	AudioSampleRate int
	AudioChannels   int

	// TagOverrides apply to every output file (after track-specific
	// values like Title and Track number). The CLI overrides win.
	TagOverrides audio.Tag

	// Jobs caps concurrent per-chapter extractions. When 0, Run picks
	// jobs.Default() — by default min(cpuCap, memoryCap) — to fan out
	// automatically without pegging the host. Each worker runs one
	// ffmpeg ExtractSegment plus (for MP4-family outputs) one
	// mp4tags write; both are independent per output file.
	Jobs int

	// IgnoreMemoryCap disables the memory-aware part of the auto-jobs
	// default (jobs.Default's `ignoreMemoryCap` arg). Has no effect
	// when Jobs is set explicitly. Useful when MemAvailable is
	// underreported (some WSL2 setups) and the user knows the host
	// can fit the full CPU-cap fan-out.
	IgnoreMemoryCap bool

	// Quiet suppresses all info/progress output on stderr — only
	// errors (which surface as the Run() return value) reach the
	// caller. Stdout output (the final summary line) is unaffected;
	// pipe `2>/dev/null` on top of --quiet for full silence.
	Quiet bool

	// Verbose adds a per-chapter "done in <elapsed>" line after each
	// successful extract so users can see wall-time progress on long
	// runs. When both Quiet and Verbose are set, Quiet wins (the
	// discarded output never reaches the user either way).
	Verbose bool

	Stdout io.Writer
	Stderr io.Writer
}

// Run executes the split command per spec/cli-surface.md §split.
//
// The pipeline:
//
//  1. Probe input duration.
//  2. Resolve chapter sources via ChapterSource.Resolve.
//  3. Optionally reindex chapter names.
//  4. For each chapter: render filename, ffmpeg ExtractSegment.
//  5. Tag each output via mp4tags when MP4 family; otherwise skipped
//     (MP3 tag-write lands with --audio-format mp3, post-v1).
//
// Returns an error on the first failure; partial outputs from earlier
// chapters are left in place for the caller to inspect or clean.
func Run(ctx context.Context, cfg Config) error {
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if cfg.Quiet {
		stderr = io.Discard
	}

	if cfg.Input == "" {
		return errors.New("split: --input required")
	}
	if cfg.Jobs == 0 {
		cfg.Jobs = jobs.Default(cfg.IgnoreMemoryCap)
	}
	if _, err := os.Stat(cfg.Input); err != nil {
		return fmt.Errorf("split: input %q: %w", cfg.Input, err)
	}

	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = defaultOutputDir(cfg.Input)
	}

	ff, err := ffmpeg.NewClient("")
	if err != nil {
		return err
	}

	total, err := ff.ProbeDuration(ctx, cfg.Input)
	if err != nil {
		return fmt.Errorf("probe %q: %w", cfg.Input, err)
	}

	ext := outputExt(cfg.Input, cfg.AudioFormat)
	muxFormat := muxerForExt(ext)

	// Construct the mp4v2 client once, up front, when either the input
	// or the output is MP4-family. The chapter resolver uses it as a
	// fallback when ffmpeg's `-f ffmetadata` drops chapters from a
	// malformed-timescale input; the post-extract tag write uses it
	// when the output is m4a/m4b/mp4. One client serves both.
	var mp *mp4v2.Client
	if isMP4FamilyExt(cfg.Input) || isMP4Family(ext) {
		mp, err = mp4v2.NewClient()
		if err != nil {
			return err
		}
	}

	chs, err := ChapterSource{
		FixedLength:             cfg.FixedLength,
		BySilence:               cfg.BySilence,
		SilenceMin:              cfg.SilenceMin,
		SilenceMax:              cfg.SilenceMax,
		ChaptersFilename:        cfg.ChaptersFilename,
		UseExistingChaptersFile: cfg.UseExistingChaptersFile,
		CueSheetPath:            cfg.CueSheetPath,
	}.Resolve(ctx, cfg.Input, total, ff, mp)
	if err != nil {
		if errors.Is(err, ErrNoChapters) {
			return fmt.Errorf("%w; this file has no chapters readable by either ffmpeg or mp4chaps. Pass --by-silence to derive chapter boundaries from silence detection, --fixed-length=<seconds> to split into equal-duration chunks, --chapters-filename=<path> to point at a sidecar, or place a `<basename>.chapters.txt` next to the input", err)
		}
		return err
	}
	FillTrailingLength(chs, total)
	ApplyRename(chs, RenameOptions{
		Reindex:    cfg.ReindexChapters,
		StripTitle: cfg.StripTitle,
		Prefix:     cfg.ChapterPrefix,
	})
	if cfg.ChapterPrefix == "" {
		if frac := numericIndexFraction(chs); frac >= 0.8 {
			logf(stderr, "split: %d%% of chapter names look like bare indices (e.g. %q). Run `m4b-tool chapters export %q` to dump them as a sidecar you can edit, or pass `--chapter-prefix \"Chapter \"` to prepend a label.\n", int(frac*100+0.5), chs[0].Name, cfg.Input)
		}
	}

	tmpl, err := CompileFilenameTemplate(cfg.FilenameTemplate)
	if err != nil {
		return err
	}

	logf(stderr, "split: %d chapter(s) -> %s\n", len(chs), outDir)

	if cfg.DryRun {
		printDryRun(stdout, cfg, chs, outDir, ext, total)
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	codec := cfg.AudioCodec
	if codec == "" {
		codec = defaultCodecForFormat(muxFormat)
	}
	bitrate := cfg.AudioBitrate
	if bitrate == "" && codec != "copy" {
		// Probe the source for a sensible default. When the source's
		// bitrate is unknown (e.g. lossless FLAC source with no kb/s
		// reported), pass empty and let ffmpeg pick the codec default.
		srcBps, _ := ff.ProbeAudioBitrate(ctx, cfg.Input)
		bitrate = defaultBitrate(srcBps)
	}
	extOpts := ffmpeg.ExtractOptions{
		Format:     muxFormat,
		Codec:      codec,
		Bitrate:    bitrate,
		SampleRate: cfg.AudioSampleRate,
		Channels:   cfg.AudioChannels,
		Overwrite:  cfg.Force,
	}

	// extractAll uses `mp` only when the OUTPUT is MP4-family for the
	// post-extract tag write. Suppress it when the output is anything
	// else, even though the same client served the chapter resolver
	// above for an MP4 input.
	if !isMP4Family(ext) {
		mp = nil
	}

	// Pre-render filenames and stat-check pre-existence serially. This
	// is cheap and lets us fail fast — and surface a single error
	// before we've spawned any ffmpeg processes.
	jobs := make([]splitJob, len(chs))
	for i, ch := range chs {
		track := i + 1
		fname, err := Render(tmpl, templateDataFromTag(track, len(chs), ch, cfg.TagOverrides))
		if err != nil {
			return err
		}
		out := filepath.Join(outDir, fname+ext)
		if !cfg.Force {
			if _, err := os.Stat(out); err == nil {
				return fmt.Errorf("split: %q already exists; use --force", out)
			}
		}
		jobs[i] = splitJob{track: track, total: len(chs), chapter: ch, out: out}
	}

	if err := extractAll(ctx, ff, mp, cfg.Input, jobs, extOpts, cfg.TagOverrides, cfg.Jobs, cfg.Verbose, stderr); err != nil {
		return err
	}

	logf(stdout, "split %d chapter(s) -> %s\n", len(chs), outDir)
	return nil
}

// templateDataFromTag projects a Chapter + the user's tag overrides
// into the smaller TemplateData struct. Track/Title come from the
// chapter; everything else from the overrides.
func templateDataFromTag(track, total int, ch audio.Chapter, t audio.Tag) TemplateData {
	return TemplateData{
		Track:       track,
		TrackTotal:  total,
		Title:       firstNonEmpty(ch.Name, t.Title),
		Album:       t.Album,
		Artist:      t.Artist,
		AlbumArtist: t.AlbumArtist,
		Genre:       t.Genre,
		Writer:      t.Writer,
		Year:        t.Year,
		Series:      t.Series,
		SeriesPart:  t.SeriesPart,
	}
}

// perTrackTag layers chapter-derived per-track values (Title and
// Track number) on top of the user's CLI overrides.
func perTrackTag(base audio.Tag, ch audio.Chapter, track, total int) audio.Tag {
	out := base
	out.Title = ch.Name
	out.Track = track
	out.Tracks = total
	return out
}

func defaultOutputDir(input string) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	return filepath.Join(filepath.Dir(input), base+"_splitted")
}

func outputExt(input, formatOverride string) string {
	if formatOverride != "" {
		if !strings.HasPrefix(formatOverride, ".") {
			return "." + formatOverride
		}
		return formatOverride
	}
	return filepath.Ext(input)
}

func muxerForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".m4a", ".m4b", ".mp4":
		return "mp4"
	case ".mp3":
		return "mp3"
	case ".flac":
		return "flac"
	}
	return ""
}

func isMP4Family(ext string) bool {
	switch strings.ToLower(ext) {
	case ".m4a", ".m4b", ".mp4":
		return true
	}
	return false
}

// bitrateCap is the highest default bitrate we will derive from a
// probed source. Without a cap, a lossless source (FLAC, PCM) that
// reports e.g. 1411 kb/s would produce an absurdly large MP3 — well
// past audible benefit for any content type. 192 kbps is enough for
// stereo music at "transparent for most listeners" quality and well
// above what spoken-word audiobooks need.
const bitrateCap = 192_000

// defaultBitrate maps a probed source bitrate (in bps) to the
// `-b:a <N>k` value to pass to ffmpeg when the user hasn't set
// --audio-bitrate. Returns "" (let ffmpeg pick the codec's own
// default) when the source bitrate is unknown.
//
// When the source bitrate is known we use it directly, capped at
// bitrateCap. Same bitrate value is applied regardless of output
// codec; ffmpeg silently ignores -b:a for lossless codecs (FLAC,
// PCM) that quantize via quality instead.
//
// Note: AAC and MP3 are not bitrate-equivalent — AAC@64k sounds
// closer to MP3@96-128k for similar perceived quality. Matching
// source bitrate on an AAC→MP3 transcode therefore produces *worse*
// perceived quality than the source. This is acceptable for the
// "user didn't say" path; users who care set --audio-bitrate.
func defaultBitrate(sourceBps int) string {
	if sourceBps <= 0 {
		return ""
	}
	if sourceBps > bitrateCap {
		sourceBps = bitrateCap
	}
	return strconv.Itoa(sourceBps/1000) + "k"
}

// defaultCodecForFormat returns the ffmpeg `-c:a` value to use when the
// user picks --audio-format but doesn't set --audio-codec (per
// spec/cli-surface.md: "Auto from --audio-format if omitted").
//
// MP4-family containers can hold AAC; since the typical input is an
// AAC-encoded m4b, "copy" stream-copies the source losslessly. Non-MP4
// containers don't accept AAC, so we transcode to a codec the muxer
// will write — the user can override with --audio-codec.
func defaultCodecForFormat(format string) string {
	switch strings.ToLower(format) {
	case "mp4", "m4a", "m4b":
		return "copy"
	case "mp3":
		return "libmp3lame"
	case "flac":
		return "flac"
	case "ogg":
		return "libvorbis"
	case "opus":
		return "libopus"
	case "wav":
		return "pcm_s16le"
	}
	return "copy"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func printDryRun(w io.Writer, cfg Config, chs []audio.Chapter, outDir, ext string, total time.Duration) {
	p := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	p("=== dry run ===\n")
	p("input    : %s\n", cfg.Input)
	p("duration : %s\n", audio.FormatHMS(total))
	p("output   : %s/\n", outDir)
	p("chapters (%d):\n", len(chs))
	tmpl, _ := CompileFilenameTemplate(cfg.FilenameTemplate)
	for i, ch := range chs {
		track := i + 1
		fname, _ := Render(tmpl, templateDataFromTag(track, len(chs), ch, cfg.TagOverrides))
		p("  %2d. %s -> %s\n", track, audio.FormatHMS(ch.Start), filepath.Join(outDir, fname+ext))
	}
}

func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

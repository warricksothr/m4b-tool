package merge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
	"github.com/warricksothr/m4b-tool/internal/tag"
	"github.com/warricksothr/m4b-tool/internal/tag/chapterstxt"
	"github.com/warricksothr/m4b-tool/internal/tag/cover"
	"github.com/warricksothr/m4b-tool/internal/tag/cuesheet"
	"github.com/warricksothr/m4b-tool/internal/tag/description"
	"github.com/warricksothr/m4b-tool/internal/tag/equate"
	"github.com/warricksothr/m4b-tool/internal/tag/ffmetadata"
	"github.com/warricksothr/m4b-tool/internal/tag/filetracks"
)

// Config is the input to [Run].
//
// M7b scope: stream-copy (--no-conversion) and the default transcode
// pipeline are both supported, with --jobs concurrency, --trim-silence
// on middle files, and --add-silence interleaving. Batch mode arrives
// in a later slice.
type Config struct {
	// Inputs is the list of files / directories to merge (positional args).
	Inputs []string
	// Output is the target file path (required; .m4b/.mp4/.m4a).
	Output string
	// Force overwrites Output without prompting.
	Force bool
	// IncludeExtensions filters files found via directory scan.
	// When empty, [DefaultExtensions] applies.
	IncludeExtensions []string
	// DryRun prints the plan without running ffmpeg or mp4v2.
	DryRun bool

	// UseFilenamesAsChapters forces chapter names to be the filename
	// stem even when the input has an embedded title tag.
	UseFilenamesAsChapters bool

	// Chapter-source sidecars (paths; empty = skip).
	CueSheetPath     string
	ChaptersFilename string // explicit chapters.txt override

	// TagOverrides are applied with MergeOverwrite after the importer
	// chain runs; they always win.
	TagOverrides audio.Tag
	// SkipCover disables cover discovery + embedding.
	SkipCover bool
	// SkipCoverIfExists leaves an already-embedded cover alone.
	SkipCoverIfExists bool
	// CoverOverride, when set, short-circuits the cover importer.
	CoverOverride string
	// EquateSpecs is the list of --equate CSV specs.
	EquateSpecs []string

	// Importer filters.
	EnableImprovers  []string
	DisableImprovers []string

	// --- Encoding pipeline (M7b) ---

	// NoConversion stream-copies inputs into a uniform-codec concat.
	// When true, the per-input transcode step is skipped and the
	// caller is responsible for ensuring all inputs share codec
	// parameters that ffmpeg's concat demuxer accepts.
	NoConversion bool
	// AudioCodec sets -c:a, e.g. "aac", "libfdk_aac". Empty means
	// "let the codec default for the output container apply".
	AudioCodec string
	// AudioBitrate sets -b:a, e.g. "64k".
	AudioBitrate string
	// AudioSampleRate sets -ar (Hz). Zero leaves the source rate.
	AudioSampleRate int
	// AudioChannels sets -ac. Zero leaves the source channel count.
	AudioChannels int
	// AdjustForIPod clamps SampleRate to 44100 max and Channels to 2
	// max for iPod-family-compatible output.
	AdjustForIPod bool
	// TrimSilence applies silenceremove at both ends of every MIDDLE
	// input file (the first and last keep their boundary silence).
	TrimSilence bool
	// AddSilence inserts a synthesized silent segment of this length
	// between consecutive parts. Zero means contiguous concat.
	AddSilence time.Duration
	// Jobs caps concurrent transcodes when NoConversion is false.
	// Defaults to 1 (sequential) when 0.
	Jobs int

	// --- Batch mode (M7c) ---

	// BatchPatterns is a list of --batch-pattern templates; when
	// non-empty, Run enumerates one merge job per matching directory
	// and Output is treated as the parent directory for per-entry
	// outputs.
	BatchPatterns []string
	// BatchPatternPath is the base path matched patterns are
	// considered relative to. Empty means each input root.
	BatchPatternPath string
	// BatchFilter is a substring; relative paths that don't contain
	// it are skipped during enumeration.
	BatchFilter string
	// BatchResumeFile is a file of previously-completed source dirs;
	// matching entries are skipped, and successful entries are
	// appended on completion.
	BatchResumeFile string

	// Logging sinks. Default os.Stdout / os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// supportedOutputExts are the MP4-family containers the chapter/tag
// write-back path supports. M7a does not yet cover mp3 output.
var supportedOutputExts = map[string]bool{".m4b": true, ".mp4": true, ".m4a": true}

// Run executes the merge command per spec/cli-surface.md §merge. When
// cfg.BatchPatterns is non-empty it dispatches to the batch enumerator
// (one merge per matched directory); otherwise it runs a single merge.
//
// In single mode the pipeline is stream-copy when cfg.NoConversion is
// set, otherwise each input is transcoded to a uniform AAC-in-MP4
// intermediate so the concat demuxer can stream-copy the result.
func Run(ctx context.Context, cfg Config) error {
	if len(cfg.BatchPatterns) > 0 {
		return runBatch(ctx, cfg)
	}
	return runSingle(ctx, cfg)
}

func runSingle(ctx context.Context, cfg Config) error {
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if err := validate(&cfg); err != nil {
		return err
	}

	inputs, err := collectInputs(cfg.Inputs, cfg.effectiveExtensions())
	if err != nil {
		return fmt.Errorf("collect inputs: %w", err)
	}
	if len(inputs) == 0 {
		return errors.New("no input files found")
	}
	logf(stderr, "collected %d input file(s)\n", len(inputs))

	ffClient, err := ffmpeg.NewClient("")
	if err != nil {
		return err
	}

	titles := readTitles(ctx, ffClient, inputs)

	// Encode each input (or pass-through), then probe each part for
	// the actual post-encode duration. This matters when --trim-silence
	// shortens the middle parts.
	var (
		parts  []string
		tmpDir string
	)
	if cfg.NoConversion {
		parts = inputs
	} else {
		tmpDir, err = os.MkdirTemp("", "m4btool-merge-*")
		if err != nil {
			return fmt.Errorf("temp dir: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		encOpts := buildEncodeOptions(&cfg)
		logf(stderr, "transcoding %d input(s) (codec=%s, jobs=%d, trim-silence=%t)\n",
			len(inputs), nonEmpty(encOpts.Codec, "default"), effectiveJobs(cfg.Jobs), cfg.TrimSilence)
		parts, err = encodeAll(ctx, ffClient, inputs, tmpDir, cfg.Jobs, encOpts)
		if err != nil {
			return fmt.Errorf("transcode: %w", err)
		}
	}

	partDurations, err := probeDurations(ctx, ffClient, parts)
	if err != nil {
		return err
	}

	tracks := buildTracks(inputs, partDurations, titles)
	total := totalDuration(tracks)
	if cfg.AddSilence > 0 && len(tracks) > 1 {
		total += time.Duration(len(tracks)-1) * cfg.AddSilence
	}
	logf(stderr, "total duration: %s\n", audio.FormatHMS(total))

	tagOut, err := runImporters(ctx, &cfg, ffClient, inputs, tracks)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		printDryRun(stdout, &cfg, inputs, tracks, tagOut)
		return nil
	}

	if err := ensureOutputDir(cfg.Output); err != nil {
		return err
	}

	// Optionally synthesize a silence segment and weave it between parts.
	concatList := parts
	if cfg.AddSilence > 0 && len(parts) > 1 {
		// Match the silence segment to whatever the parts use. Without
		// transcoding we don't know the exact codec parameters of the
		// inputs, so default to AAC stereo 44100Hz; if that diverges
		// from input parameters, the concat demuxer will reject it
		// and the caller can switch to the transcode path.
		silenceDir := tmpDir
		if silenceDir == "" {
			silenceDir, err = os.MkdirTemp("", "m4btool-silence-*")
			if err != nil {
				return fmt.Errorf("silence tmp: %w", err)
			}
			defer func() { _ = os.RemoveAll(silenceDir) }()
		}
		silOpts := ffmpeg.SilenceOptions{
			SampleRate: cfg.AudioSampleRate,
			Channels:   cfg.AudioChannels,
			Codec:      cfg.AudioCodec,
			Bitrate:    cfg.AudioBitrate,
		}
		silencePath, err := synthSilenceTrack(ctx, ffClient, silenceDir, cfg.AddSilence, silOpts)
		if err != nil {
			return fmt.Errorf("synth silence: %w", err)
		}
		concatList = interleaveWithSilence(parts, silencePath)
		logf(stderr, "interleaving %v silence between %d parts\n", cfg.AddSilence, len(parts))
	}

	logf(stderr, "concatenating %d segments -> %s\n", len(concatList), cfg.Output)
	if err := ffClient.Concat(ctx, concatList, cfg.Output, ffmpeg.ConcatOptions{
		Format:    outputFormatForExt(cfg.Output),
		Overwrite: cfg.Force,
	}); err != nil {
		return err
	}

	mp4Client, err := mp4v2.NewClient()
	if err != nil {
		return err
	}

	if len(tagOut.Chapters) > 0 {
		logf(stderr, "writing %d chapter(s)\n", len(tagOut.Chapters))
		if err := mp4Client.WriteChapters(ctx, cfg.Output, tagOut.Chapters, total); err != nil {
			return fmt.Errorf("write chapters: %w", err)
		}
	}
	if err := mp4Client.WriteTags(ctx, cfg.Output, &tagOut); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}
	if !cfg.SkipCover && tagOut.CoverPath != "" {
		shouldWrite := true
		if cfg.SkipCoverIfExists {
			n, err := mp4Client.ListCovers(ctx, cfg.Output)
			if err == nil && n > 0 {
				shouldWrite = false
			}
		}
		if shouldWrite {
			logf(stderr, "embedding cover from %s\n", tagOut.CoverPath)
			if err := mp4Client.AddCover(ctx, cfg.Output, tagOut.CoverPath); err != nil {
				return fmt.Errorf("embed cover: %w", err)
			}
		}
	}

	logf(stdout, "merged %d file(s) -> %s\n", len(inputs), cfg.Output)
	return nil
}

// readTitles best-effort reads each input's embedded title tag.
// Failures are silently treated as missing titles; filetracks falls
// back to filename stems when a track has no title.
func readTitles(ctx context.Context, ff *ffmpeg.Client, inputs []string) []string {
	out := make([]string, len(inputs))
	for i, p := range inputs {
		if tag, err := ff.ReadFFMetadata(ctx, p); err == nil {
			out[i] = tag.Title
		}
	}
	return out
}

// probeDurations probes each path serially. For M7b a parallel probe
// would shave seconds off large input sets, but we already parallelize
// the much-heavier transcode step.
func probeDurations(ctx context.Context, ff *ffmpeg.Client, paths []string) ([]time.Duration, error) {
	out := make([]time.Duration, len(paths))
	for i, p := range paths {
		d, err := ff.ProbeDuration(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", p, err)
		}
		out[i] = d
	}
	return out, nil
}

// buildTracks zips originals with post-encode durations and probed
// titles into the slice filetracks expects.
func buildTracks(originals []string, durations []time.Duration, titles []string) []filetracks.Track {
	out := make([]filetracks.Track, len(originals))
	for i, p := range originals {
		out[i] = filetracks.Track{
			Path:     p,
			Duration: durations[i],
			Title:    titles[i],
		}
	}
	return out
}

// buildEncodeOptions translates the user-facing Config into the
// ffmpeg-package EncodeOptions used per-input. Boundary-only behavior
// (no trim on first/last) is handled inside encodeAll, not here.
func buildEncodeOptions(cfg *Config) ffmpeg.EncodeOptions {
	codec := cfg.AudioCodec
	if codec == "" {
		codec = "aac"
	}
	bitrate := cfg.AudioBitrate
	if bitrate == "" {
		bitrate = "64k"
	}
	rate := cfg.AudioSampleRate
	channels := cfg.AudioChannels
	if cfg.AdjustForIPod {
		if rate == 0 || rate > 44100 {
			rate = 44100
		}
		if channels == 0 || channels > 2 {
			channels = 2
		}
	}

	return ffmpeg.EncodeOptions{
		Format:           "mp4",
		Codec:            codec,
		Bitrate:          bitrate,
		SampleRate:       rate,
		Channels:         channels,
		TrimSilenceStart: cfg.TrimSilence,
		TrimSilenceEnd:   cfg.TrimSilence,
		Overwrite:        true,
	}
}

func effectiveJobs(j int) int {
	if j < 1 {
		return 1
	}
	return j
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func validate(cfg *Config) error {
	if len(cfg.Inputs) == 0 {
		return errors.New("merge: at least one input required")
	}
	if cfg.Output == "" {
		return errors.New("merge: --output-file is required")
	}
	ext := strings.ToLower(filepath.Ext(cfg.Output))
	if !supportedOutputExts[ext] {
		return fmt.Errorf("merge: output %q: only .m4b/.mp4/.m4a supported in this build (ext=%q)", cfg.Output, ext)
	}
	if !cfg.Force {
		if _, err := os.Stat(cfg.Output); err == nil {
			return fmt.Errorf("merge: %q already exists; use --force to overwrite", cfg.Output)
		}
	}
	return nil
}

func (c Config) effectiveExtensions() []string {
	if len(c.IncludeExtensions) == 0 {
		return DefaultExtensions
	}
	return c.IncludeExtensions
}

func outputFormatForExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m4b":
		return "mp4" // ffmpeg has no "m4b" muxer; mp4 with .m4b suffix is the idiom
	case ".m4a":
		return "mp4"
	case ".mp4":
		return "mp4"
	}
	return ""
}

func ensureOutputDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("merge: output parent %q is not a directory", dir)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func totalDuration(tracks []filetracks.Track) time.Duration {
	var total time.Duration
	for _, t := range tracks {
		total += t.Duration
	}
	return total
}

// runImporters builds and runs the composite importer chain, then reads
// each input's embedded FFMETADATA (merge-missing) and applies CLI
// overrides (merge-overwrite), matching the spec composition order.
func runImporters(ctx context.Context, cfg *Config, ff *ffmpeg.Client, inputs []string, tracks []filetracks.Track) (audio.Tag, error) {
	workDir := workDirFor(inputs)
	primary := inputs[0]

	covOpts := cover.Options{Skip: cfg.SkipCover}
	coverImporter := cover.New(covOpts)

	// If user set --cover, it takes precedence over discovery — apply
	// it as a MergeMissing tag-override before the importer runs.
	var initial audio.Tag
	if cfg.CoverOverride != "" {
		initial.CoverPath = cfg.CoverOverride
	}

	composite := &tag.Composite{
		Importers: []tag.Importer{
			filetracks.New(filetracks.Options{
				Tracks:       tracks,
				UseFilenames: cfg.UseFilenamesAsChapters,
				Gap:          cfg.AddSilence,
			}),
			chapterstxt.New(chapterstxt.Options{AudioPath: primary, Override: cfg.ChaptersFilename}),
			ffmetadata.New(ffmetadata.Options{}),
			coverImporter,
			description.New(description.Options{}),
			cuesheet.New(cuesheet.Options{Path: cfg.CueSheetPath}),
		},
		Enable:  tag.NamesSet(cfg.EnableImprovers),
		Disable: tag.NamesSet(cfg.DisableImprovers),
	}

	out, err := composite.Improve(ctx, initial, workDir)
	if err != nil {
		return audio.Tag{}, err
	}

	// Step 7 of the composition order: merge embedded tags from input
	// files, MergeMissing. We take the primary input as the canonical
	// source; other inputs contribute only when the primary is silent
	// on a field.
	for _, in := range inputs {
		if embedded, err := ff.ReadFFMetadata(ctx, in); err == nil {
			// Embedded chapters from the input files are not flattened
			// here — filetracks has already emitted one chapter per
			// input. Zero them out before MergeMissing to avoid the
			// embedded chapter list overwriting when Chapters was set
			// to nil by another importer.
			embedded.Chapters = nil
			out.MergeMissing(*embedded)
		}
	}

	// Step 8: --equate propagation.
	if len(cfg.EquateSpecs) > 0 {
		eq := equate.New(equate.Options{Specs: cfg.EquateSpecs})
		out, err = eq.Improve(ctx, out, workDir)
		if err != nil {
			return audio.Tag{}, err
		}
	}

	// Step 9: CLI overrides win.
	out.MergeOverwrite(cfg.TagOverrides)

	return out, nil
}

// workDirFor returns the directory that sidecar-reading importers
// (ffmetadata.txt, description.txt, cover.*) should scan. The spec
// doesn't explicitly say which directory wins when inputs span
// multiple dirs; we use the directory of the first input, which is
// what the PHP tool does in practice.
func workDirFor(inputs []string) string {
	if len(inputs) == 0 {
		return "."
	}
	return filepath.Dir(inputs[0])
}

// printDryRun emits a human-readable plan summary to stdout.
func printDryRun(w io.Writer, cfg *Config, inputs []string, tracks []filetracks.Track, t audio.Tag) {
	p := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	p("=== dry run ===\n")
	p("output: %s\n", cfg.Output)
	p("inputs (%d):\n", len(inputs))
	for i, track := range tracks {
		p("  %2d. %s  (%s)\n", i+1, track.Path, audio.FormatHMS(track.Duration))
	}
	p("total duration: %s\n", audio.FormatHMS(totalDuration(tracks)))
	p("chapters (%d):\n", len(t.Chapters))
	for i, ch := range t.Chapters {
		p("  %2d. %s  %s\n", i+1, audio.FormatHMS(ch.Start), ch.Name)
	}
	p("tag summary:\n")
	for _, pair := range []struct{ k, v string }{
		{"Title", t.Title},
		{"Album", t.Album},
		{"Artist", t.Artist},
		{"AlbumArtist", t.AlbumArtist},
		{"Writer", t.Writer},
		{"Genre", t.Genre},
		{"Description", shorten(t.Description, 60)},
	} {
		if pair.v != "" {
			p("  %-12s %s\n", pair.k+":", pair.v)
		}
	}
	if t.CoverPath != "" {
		p("  %-12s %s\n", "Cover:", t.CoverPath)
	}
}

func shorten(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

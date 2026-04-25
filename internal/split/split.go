package split

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

	// Jobs caps concurrent per-chapter extractions. Defaults to 1
	// (serial) when 0. Each worker runs one ffmpeg ExtractSegment plus
	// (for MP4-family outputs) one mp4tags write; both are independent
	// per output file.
	Jobs int

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

	if cfg.Input == "" {
		return errors.New("split: --input required")
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

	chs, err := ChapterSource{
		FixedLength:             cfg.FixedLength,
		BySilence:               cfg.BySilence,
		SilenceMin:              cfg.SilenceMin,
		SilenceMax:              cfg.SilenceMax,
		ChaptersFilename:        cfg.ChaptersFilename,
		UseExistingChaptersFile: cfg.UseExistingChaptersFile,
		CueSheetPath:            cfg.CueSheetPath,
	}.Resolve(ctx, cfg.Input, total, ff, nil)
	if err != nil {
		return err
	}
	FillTrailingLength(chs, total)
	if cfg.ReindexChapters {
		ReindexChapters(chs)
	}

	tmpl, err := CompileFilenameTemplate(cfg.FilenameTemplate)
	if err != nil {
		return err
	}

	ext := outputExt(cfg.Input, cfg.AudioFormat)
	muxFormat := muxerForExt(ext)

	logf(stderr, "split: %d chapter(s) -> %s\n", len(chs), outDir)

	if cfg.DryRun {
		printDryRun(stdout, cfg, chs, outDir, ext, total)
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	extOpts := ffmpeg.ExtractOptions{
		Format:     muxFormat,
		Codec:      cfg.AudioCodec,
		Bitrate:    cfg.AudioBitrate,
		SampleRate: cfg.AudioSampleRate,
		Channels:   cfg.AudioChannels,
		Overwrite:  cfg.Force,
	}

	var mp *mp4v2.Client
	if isMP4Family(ext) {
		mp, err = mp4v2.NewClient()
		if err != nil {
			return err
		}
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

	if err := extractAll(ctx, ff, mp, cfg.Input, jobs, extOpts, cfg.TagOverrides, cfg.Jobs, stderr); err != nil {
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

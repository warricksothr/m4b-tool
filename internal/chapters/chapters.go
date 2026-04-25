package chapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/chapter"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// Defaults from spec/cli-surface.md.
const (
	DefaultSilenceMin         = 1750 * time.Millisecond
	DefaultSilenceMax         = 0 // 0 = unlimited
	DefaultChapterPattern     = `^[^:]+[1-9][0-9]*:\s*(.*),.*[1-9][0-9]*\s*$`
	DefaultChapterReplace     = "$1"
	DefaultChapterRemoveChars = `„"`
)

// Config is the input to [Run]. Zero values mean "not set" for every
// optional field; Run applies spec defaults where sensible.
type Config struct {
	// Input is the single audio file to operate on (required).
	Input string
	// OutputFile, when set, redirects chapter writing to this text
	// path instead of updating the audio file via mp4chaps -i.
	OutputFile string
	// Force overwrites OutputFile without prompting.
	Force bool
	// NoChapterImport writes the sidecar but does not run mp4chaps -i
	// to embed chapters in the audio file.
	NoChapterImport bool

	// Adjustments (applied in the order listed in Run).
	AdjustBySilence    bool
	Normalize          bool
	MergeSimilar       bool
	NoChapterNumbering bool
	Shift              *ShiftSpec // nil = no shift

	// Pattern settings. Only used when Normalize is true. Zero values
	// take the spec defaults.
	ChapterPattern     string
	ChapterReplacement string
	ChapterRemoveChars string

	// Offsets for synthetic Intro/Outro chapters.
	FirstChapterOffset time.Duration
	LastChapterOffset  time.Duration

	// Silence detection bounds.
	SilenceMinLength time.Duration
	SilenceMaxLength time.Duration

	// FFmpegBin / MP4v2Bins override auto-discovered binaries. Leave
	// empty for PATH resolution.
	FFmpegBin string

	// Stdout / Stderr divert informational / error output away from
	// the global stderr for testing. Default to os.Stdout / os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// MP4Extensions are the container file extensions that accept chapter
// atom writes via mp4chaps. Case-insensitive.
var MP4Extensions = map[string]bool{".m4b": true, ".mp4": true, ".m4a": true}

// Run executes the chapters command against cfg.Input per the spec
// (cli-surface.md §chapters). Returns an error covering validation,
// probe, adjustment, and write failures; callers typically surface the
// error text and exit non-zero.
func Run(ctx context.Context, cfg Config) error {
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if cfg.SilenceMinLength == 0 {
		cfg.SilenceMinLength = DefaultSilenceMin
	}
	if cfg.ChapterPattern == "" {
		cfg.ChapterPattern = DefaultChapterPattern
	}
	if cfg.ChapterReplacement == "" {
		cfg.ChapterReplacement = DefaultChapterReplace
	}
	if cfg.ChapterRemoveChars == "" {
		cfg.ChapterRemoveChars = DefaultChapterRemoveChars
	}

	if cfg.Input == "" {
		return errors.New("chapters: input file required")
	}
	info, err := os.Stat(cfg.Input)
	if err != nil {
		return fmt.Errorf("chapters: stat input: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("chapters: input %q is a directory; this command takes a single file", cfg.Input)
	}

	writeBackToAudio := cfg.OutputFile == "" && !cfg.NoChapterImport
	inputExt := strings.ToLower(filepath.Ext(cfg.Input))
	if writeBackToAudio && !MP4Extensions[inputExt] {
		return fmt.Errorf("chapters: cannot write chapters to %q (only .m4b/.mp4/.m4a supported); use --output-file to export to a text file", cfg.Input)
	}

	ffClient, err := ffmpeg.NewClient(cfg.FFmpegBin)
	if err != nil {
		return err
	}
	var mp4Client *mp4v2.Client
	if writeBackToAudio {
		mp4Client, err = mp4v2.NewClient()
		if err != nil {
			return err
		}
	}

	total, err := ffClient.ProbeDuration(ctx, cfg.Input)
	if err != nil {
		return fmt.Errorf("probe duration: %w", err)
	}
	_, _ = fmt.Fprintf(stderr, "input duration: %s\n", audio.FormatHMS(total))

	existing, err := ffClient.ReadFFMetadata(ctx, cfg.Input)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}
	chapters := existing.Chapters
	_, _ = fmt.Fprintf(stderr, "initial chapters: %d\n", len(chapters))

	if cfg.AdjustBySilence {
		_, _ = fmt.Fprintf(stderr, "detecting silences (min=%v, max=%v)...\n", cfg.SilenceMinLength, cfg.SilenceMaxLength)
		silences, err := ffClient.DetectSilence(ctx, cfg.Input, cfg.SilenceMinLength, cfg.SilenceMaxLength)
		if err != nil {
			return fmt.Errorf("detect silence: %w", err)
		}
		_, _ = fmt.Fprintf(stderr, "detected %d silences; snapping chapters\n", len(silences))
		chapters = chapter.AlignToSilence(chapters, silences, chapter.AlignOptions{Total: total})
	}

	if cfg.Normalize || cfg.MergeSimilar {
		opts := chapter.NormalizeOptions{
			MergeSimilar: cfg.MergeSimilar,
			NoNumbering:  cfg.NoChapterNumbering,
		}
		if cfg.Normalize {
			re, err := regexp.Compile(cfg.ChapterPattern)
			if err != nil {
				return fmt.Errorf("chapter-pattern regex: %w", err)
			}
			opts.Pattern = re
			opts.Replacement = cfg.ChapterReplacement
			opts.RemoveChars = cfg.ChapterRemoveChars
		}
		chapters = chapter.Normalize(chapters, opts)
	}

	if cfg.Shift != nil {
		shifted, err := chapter.Shift(chapters, cfg.Shift.Offset, cfg.Shift.Indexes)
		if err != nil {
			return fmt.Errorf("shift: %w", err)
		}
		chapters = shifted
	}

	if cfg.FirstChapterOffset > 0 {
		chapters = chapter.PrependIntro(chapters, cfg.FirstChapterOffset)
	}
	if cfg.LastChapterOffset > 0 {
		chapters = chapter.AppendOutro(chapters, total, cfg.LastChapterOffset)
	}

	_, _ = fmt.Fprintf(stderr, "final chapters: %d\n", len(chapters))

	return writeChapters(ctx, cfg, mp4Client, chapters, total, stdout)
}

// writeChapters dispatches to the right output sink based on cfg.
func writeChapters(ctx context.Context, cfg Config, mp4Client *mp4v2.Client, chapters []audio.Chapter, total time.Duration, stdout io.Writer) error {
	switch {
	case cfg.OutputFile != "":
		return writeSidecarToPath(cfg.OutputFile, chapters, total, cfg.Force)
	case cfg.NoChapterImport:
		sidecar := deriveSidecarPath(cfg.Input)
		_, _ = fmt.Fprintf(stdout, "wrote sidecar %s (not imported into audio, --no-chapter-import)\n", sidecar)
		return writeSidecarToPath(sidecar, chapters, total, true)
	default:
		if err := mp4Client.WriteChapters(ctx, cfg.Input, chapters, total); err != nil {
			return fmt.Errorf("write chapters: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "updated chapters in %s\n", cfg.Input)
		return nil
	}
}

// writeSidecarToPath writes a chapters.txt at path, refusing to
// overwrite unless force is true.
func writeSidecarToPath(path string, chapters []audio.Chapter, total time.Duration, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("output %q already exists; use --force to overwrite", path)
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	if err := mp4v2.WriteChaptersTxt(f, chapters, total); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}

// deriveSidecarPath returns the mp4chaps convention for audioPath.
func deriveSidecarPath(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return strings.TrimSuffix(audioPath, ext) + ".chapters.txt"
}

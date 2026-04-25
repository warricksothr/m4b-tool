package merge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// runBatch drives one merge per BatchPattern match. It is a thin
// dispatcher: per-entry merges go through runSingle so the actual
// pipeline (transcode, silence, concat, tag write) stays in one place.
//
// Output rules in batch mode:
//   - cfg.Output names the parent directory for per-entry outputs and
//     is created if missing.
//   - Per-entry filename is the matched directory's basename plus the
//     extension implied by the suffix of the user-provided Output. If
//     Output looks directory-shaped (no extension or trailing slash),
//     ".m4b" is the default.
//
// This convention matches the spec example
// `--output-file output/ --batch-pattern "input/%g/%a/%s/%p - %n/"`.
func runBatch(ctx context.Context, cfg Config) error {
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if len(cfg.Inputs) == 0 {
		return errors.New("merge: at least one input required")
	}
	if cfg.Output == "" {
		return errors.New("merge: --output-file is required (use a directory in batch mode)")
	}

	patterns, err := compilePatterns(cfg.BatchPatterns)
	if err != nil {
		return err
	}

	resume, err := LoadResumeFile(cfg.BatchResumeFile)
	if err != nil {
		return err
	}

	entries, err := EnumerateBatch(cfg.Inputs, EnumerateOptions{
		Patterns:   patterns,
		TrimBase:   cfg.BatchPatternPath,
		Extensions: cfg.effectiveExtensions(),
		Filter:     cfg.BatchFilter,
		Resume:     resume,
	})
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		logf(stderr, "batch: no matching directories found\n")
		return nil
	}

	outDir, ext, err := resolveBatchOutputDir(cfg.Output)
	if err != nil {
		return err
	}
	if !cfg.DryRun {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("batch: create output dir: %w", err)
		}
	}

	logf(stderr, "batch: %d entr(ies) to merge\n", len(entries))

	for i, e := range entries {
		entryOut := filepath.Join(outDir, filepath.Base(e.SourceDir)+ext)
		logf(stderr, "[%d/%d] %s -> %s\n", i+1, len(entries), e.RelPath, entryOut)

		if cfg.DryRun {
			printBatchEntry(stdout, e, entryOut)
			continue
		}

		sub := perEntryConfig(cfg, e, entryOut)
		if err := runSingle(ctx, sub); err != nil {
			return fmt.Errorf("batch entry %q: %w", e.RelPath, err)
		}
		if err := AppendResume(cfg.BatchResumeFile, e.SourceDir); err != nil {
			return err
		}
	}
	return nil
}

func compilePatterns(raws []string) ([]*BatchPattern, error) {
	out := make([]*BatchPattern, 0, len(raws))
	for _, r := range raws {
		p, err := ParseBatchPattern(r)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// resolveBatchOutputDir interprets cfg.Output as either an explicit
// directory (no extension, or trailing separator) or a file path whose
// extension propagates to per-entry filenames. Returns (dir, ext) with
// ext including the leading dot, or ".m4b" when no extension is given.
func resolveBatchOutputDir(out string) (string, string, error) {
	cleaned := filepath.Clean(out)
	hadSep := strings.HasSuffix(out, string(filepath.Separator)) || strings.HasSuffix(out, "/")

	ext := strings.ToLower(filepath.Ext(cleaned))
	if hadSep || ext == "" {
		return cleaned, ".m4b", nil
	}
	if !supportedOutputExts[ext] {
		return "", "", fmt.Errorf("batch: output extension %q unsupported (want .m4b/.mp4/.m4a)", ext)
	}
	// Treat <dir>/<base>.<ext> as: per-entry files use that ext, in dir.
	return filepath.Dir(cleaned), ext, nil
}

// perEntryConfig builds the runSingle-shaped Config for a single
// batch entry. Tag overrides are layered: extracted placeholder tags
// first (MergeMissing), then the user's CLI overrides (which still
// win), so a user-provided --artist outranks a `%a` capture.
func perEntryConfig(parent Config, e BatchEntry, output string) Config {
	overrides := mergeBatchTags(e.Tags, parent.TagOverrides)

	sub := parent
	sub.Inputs = []string{e.SourceDir}
	sub.Output = output
	sub.TagOverrides = overrides
	// Batch fields don't propagate to the per-entry call.
	sub.BatchPatterns = nil
	sub.BatchPatternPath = ""
	sub.BatchFilter = ""
	sub.BatchResumeFile = ""
	return sub
}

// mergeBatchTags layers placeholder-extracted tags under the user's
// explicit CLI overrides. The user's overrides win on every field
// they set; the placeholder values fill anything else.
func mergeBatchTags(extracted, cli audio.Tag) audio.Tag {
	out := extracted
	out.MergeOverwrite(cli)
	return out
}

func printBatchEntry(w io.Writer, e BatchEntry, outPath string) {
	p := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	p("  source : %s\n", e.SourceDir)
	p("  output : %s\n", outPath)
	if e.Tags.Title != "" {
		p("  title  : %s\n", e.Tags.Title)
	}
	if e.Tags.Artist != "" {
		p("  artist : %s\n", e.Tags.Artist)
	}
	if e.Tags.Album != "" {
		p("  album  : %s\n", e.Tags.Album)
	}
	if e.Tags.Series != "" || e.Tags.SeriesPart != "" {
		p("  series : %s #%s\n", e.Tags.Series, e.Tags.SeriesPart)
	}
	if e.Tags.Genre != "" {
		p("  genre  : %s\n", e.Tags.Genre)
	}
}

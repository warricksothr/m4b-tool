package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
	"github.com/warricksothr/m4b-tool/internal/split"
)

// runChaptersExportCmd implements `m4b-tool chapters export <input> [output]`.
// It resolves chapters via the same priority chain as `split` (including
// the mp4chaps fallback for files where ffmpeg's -f ffmetadata drops
// chapters) and writes them as an mp4chaps-format sidecar the user can
// edit before re-running split with --chapters-filename or
// --use-existing-chapters-file.
//
// When [output] is omitted the default is <input-base>.chapters.txt
// next to the input — the same path mp4chaps and split's resolver
// already look for, so no flags are needed to feed it back in.
//
// Pass "-" as [output] to write to stdout — useful for piping into
// `head` to preview the resolved chapters before committing to a
// split. Stdout writes skip both the overwrite check and the trailing
// "N chapter(s) -> path" status line.
//
// The --reindex-chapters / --strip-title / --chapter-prefix flags
// mirror split's so users can preview the transformed names in the
// sidecar (and tweak by hand from there) before committing.
func runChaptersExportCmd(args []string) int {
	fs := flag.NewFlagSet("chapters export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: m4b-tool chapters export [flags] <input> [output]")
		fs.PrintDefaults()
	}

	var (
		force         = fs.Bool("force", false, "overwrite an existing output file")
		forceShort    = fs.Bool("f", false, "alias for --force")
		reindex       = fs.Bool("reindex-chapters", false, "rename chapters to 1, 2, 3, ...")
		stripTitle    = fs.Bool("strip-title", false, `trim leading spaces and zeros from each chapter Name ("001" -> "1")`)
		chapterPrefix = fs.String("chapter-prefix", "", "prepend this string to every chapter Name (after --reindex-chapters and --strip-title)")
	)

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fs.Usage()
		return 2
	}
	input := fs.Arg(0)
	output := ""
	if fs.NArg() == 2 {
		output = fs.Arg(1)
	}
	toStdout := output == "-"
	if output == "" {
		output = defaultChaptersTxtPath(input)
	}

	if !toStdout && !(*force || *forceShort) {
		if _, err := os.Stat(output); err == nil {
			fmt.Fprintf(os.Stderr, "chapters export: %q already exists; use --force to overwrite\n", output)
			return 1
		}
	}

	ctx := context.Background()
	ff, err := ffmpeg.NewClient("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "chapters export:", err)
		return 1
	}
	total, err := ff.ProbeDuration(ctx, input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "chapters export: probe duration:", err)
		return 1
	}

	// mp4v2 client is only needed for the mp4chaps fallback; construct
	// it for MP4-family inputs and skip otherwise so non-MP4 inputs
	// don't fail the export when mp4chaps isn't installed.
	var mp *mp4v2.Client
	if isMP4FamilyPath(input) {
		mp, err = mp4v2.NewClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, "chapters export:", err)
			return 1
		}
	}

	chs, err := split.ChapterSource{}.Resolve(ctx, input, total, ff, mp)
	if err != nil {
		if errors.Is(err, split.ErrNoChapters) {
			fmt.Fprintf(os.Stderr, "chapters export: no chapters found in %q (try --by-silence or --fixed-length on `split` instead)\n", input)
			return 1
		}
		fmt.Fprintln(os.Stderr, "chapters export:", err)
		return 1
	}
	split.FillTrailingLength(chs, total)
	split.ApplyRename(chs, split.RenameOptions{
		Reindex:    *reindex,
		StripTitle: *stripTitle,
		Prefix:     *chapterPrefix,
	})

	if toStdout {
		if err := mp4v2.WriteChaptersTxt(os.Stdout, chs, total); err != nil {
			fmt.Fprintln(os.Stderr, "chapters export: write:", err)
			return 1
		}
		return 0
	}

	out, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "chapters export:", err)
		return 1
	}
	if err := mp4v2.WriteChaptersTxt(out, chs, total); err != nil {
		_ = out.Close()
		fmt.Fprintln(os.Stderr, "chapters export: write:", err)
		return 1
	}
	if err := out.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "chapters export:", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "chapters export: %d chapter(s) -> %s\n", len(chs), output)
	return 0
}

// defaultChaptersTxtPath returns "<input-base>.chapters.txt" next to
// input. Mirrors mp4chaps's sidecar convention so an exported file
// will be picked up automatically by `split` (and by mp4chaps -i)
// without any flags.
func defaultChaptersTxtPath(input string) string {
	ext := filepath.Ext(input)
	return strings.TrimSuffix(input, ext) + ".chapters.txt"
}

func isMP4FamilyPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m4a", ".m4b", ".mp4":
		return true
	}
	return false
}

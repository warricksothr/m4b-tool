package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/warricksothr/m4b-tool/internal/chapters"
)

// runChaptersCmd parses the `chapters` subcommand's flags and invokes
// the orchestrator. Returns a process exit code (0 success, 1 error,
// 2 usage).
func runChaptersCmd(args []string) int {
	fs := flag.NewFlagSet("chapters", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: m4b-tool chapters [flags] <input>")
		fs.PrintDefaults()
	}

	var (
		output             = fs.String("output-file", "", "write chapters to this text file instead of the audio file")
		outputShort        = fs.String("o", "", "alias for --output-file")
		force              = fs.Bool("force", false, "overwrite --output-file without prompting")
		forceShort         = fs.Bool("f", false, "alias for --force")
		noChapterImport    = fs.Bool("no-chapter-import", false, "write sidecar but do not run mp4chaps -i")
		adjustBySilence    = fs.Bool("adjust-by-silence", false, "snap chapter boundaries onto detected silences")
		normalize          = fs.Bool("normalize", false, "apply regex pattern, strip characters, and optimize titles")
		mergeSimilar       = fs.Bool("merge-similar", false, "collapse consecutive chapters with the same name")
		mergeSimilarShort  = fs.Bool("s", false, "alias for --merge-similar")
		noChapterNumbering = fs.Bool("no-chapter-numbering", false, "do not append (2), (3), ... suffixes to duplicate names")
		shiftArg           = fs.String("shift", "", "shift chapters by ms, optional indexes: ms[:idx,idx,...]")
		chapterPattern     = fs.String("chapter-pattern", chapters.DefaultChapterPattern, "regex applied to chapter names during --normalize")
		chapterReplacement = fs.String("chapter-replacement", chapters.DefaultChapterReplace, "replacement template for --chapter-pattern")
		chapterRemoveChars = fs.String("chapter-remove-chars", chapters.DefaultChapterRemoveChars, "characters to strip from chapter names during --normalize")
		firstOffsetMs      = fs.Int("first-chapter-offset", 0, "insert a synthetic Intro chapter spanning [0, <ms>]")
		lastOffsetMs       = fs.Int("last-chapter-offset", 0, "append a synthetic Outro chapter of <ms> length")
		silenceMinMs       = fs.Int("silence-min-length", 1750, "minimum silence duration for detection, in ms")
		silenceMinShort    = fs.Int("a", 1750, "alias for --silence-min-length")
		silenceMaxMs       = fs.Int("silence-max-length", 0, "maximum silence duration for detection, in ms (0 = unlimited)")
		silenceMaxShort    = fs.Int("b", 0, "alias for --silence-max-length")
	)

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	cfg := chapters.Config{
		Input:              fs.Arg(0),
		OutputFile:         firstNonEmpty(*output, *outputShort),
		Force:              *force || *forceShort,
		NoChapterImport:    *noChapterImport,
		AdjustBySilence:    *adjustBySilence,
		Normalize:          *normalize,
		MergeSimilar:       *mergeSimilar || *mergeSimilarShort,
		NoChapterNumbering: *noChapterNumbering,
		ChapterPattern:     *chapterPattern,
		ChapterReplacement: *chapterReplacement,
		ChapterRemoveChars: *chapterRemoveChars,
		FirstChapterOffset: time.Duration(*firstOffsetMs) * time.Millisecond,
		LastChapterOffset:  time.Duration(*lastOffsetMs) * time.Millisecond,
		SilenceMinLength:   time.Duration(firstNonZeroInt(*silenceMinMs, *silenceMinShort, 1750)) * time.Millisecond,
		SilenceMaxLength:   time.Duration(maxInt(*silenceMaxMs, *silenceMaxShort)) * time.Millisecond,
	}

	if spec, ok, err := chapters.ParseShiftSpec(*shiftArg); err != nil {
		fmt.Fprintln(os.Stderr, "invalid --shift:", err)
		return 2
	} else if ok {
		cfg.Shift = &spec
	}

	if err := chapters.Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "chapters:", err)
		return 1
	}
	return 0
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// firstNonZeroInt returns the first non-default value among the short/long
// flag pair, or fallback when both equal the default.
func firstNonZeroInt(long, short, fallback int) int {
	if long != fallback {
		return long
	}
	if short != fallback {
		return short
	}
	return fallback
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/split"
)

// runSplitCmd parses the `split` subcommand flags and invokes the
// orchestrator. See spec/cli-surface.md §split.
func runSplitCmd(args []string) int {
	fs := flag.NewFlagSet("split", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: m4b-tool split [flags] <input>")
		fs.PrintDefaults()
	}

	var (
		outputDir      = fs.String("output-dir", "", "directory for per-chapter outputs (default <input>_splitted/)")
		outputDirShort = fs.String("o", "", "alias for --output-dir")
		filenameTmpl   = fs.String("filename-template", "", `Go text/template for output filenames (default: {{printf "%03d" .Track}}-{{.Title}})`)
		filenameTmplP  = fs.String("p", "", "alias for --filename-template")
		force          = fs.Bool("force", false, "overwrite existing per-chapter outputs")
		forceShort     = fs.Bool("f", false, "alias for --force")
		dryRun         = fs.Bool("dry-run", false, "print the plan without extracting")

		fixedLengthSec = fs.Float64("fixed-length", 0, "fixed-duration chapters in seconds; overrides every other chapter source")
		bySilence      = fs.Bool("by-silence", false, "derive chapter boundaries from silence detection")
		silenceMinMs   = fs.Int("silence-min-length", 0, "minimum silence length for --by-silence (ms; 0 = default 1750)")
		silenceMaxMs   = fs.Int("silence-max-length", 0, "maximum silence length for --by-silence (ms; 0 = unlimited)")
		chaptersFile   = fs.String("chapters-filename", "", "explicit path to a chapters.txt file")
		useExisting    = fs.Bool("use-existing-chapters-file", false, "prefer the sidecar chapters.txt over embedded chapters")
		cuePath        = fs.String("cuesheet", "", "explicit path to a cue sheet")
		reindex        = fs.Bool("reindex-chapters", false, "rename chapters to 1, 2, 3, ...")

		audioFormat     = fs.String("audio-format", "", "output container (m4a, m4b, mp4, mp3, flac); default reuses the input extension")
		audioCodec      = fs.String("audio-codec", "", "output audio codec; empty stream-copies the input")
		audioBitrate    = fs.String("audio-bitrate", "", "output bitrate (only used when re-encoding)")
		audioSampleRate = fs.Int("audio-samplerate", 0, "output sample rate (only used when re-encoding)")
		audioChannels   = fs.Int("audio-channels", 0, "output channel count (only used when re-encoding)")
		jobs            = fs.Int("jobs", 0, "concurrent per-chapter extractions (default 1)")

		// Tag overrides (subset of the inherited set; applied to every output).
		name        = fs.String("name", "", "override album/title")
		album       = fs.String("album", "", "override album")
		artist      = fs.String("artist", "", "override artist")
		albumArtist = fs.String("albumartist", "", "override album artist")
		genre       = fs.String("genre", "", "override genre")
		writer      = fs.String("writer", "", "override writer")
		year        = fs.Int("year", 0, "override year")
		series      = fs.String("series", "", "override series")
		seriesPart  = fs.String("series-part", "", "override series part")
	)

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	cfg := split.Config{
		Input:                   fs.Arg(0),
		OutputDir:               firstNonEmpty(*outputDir, *outputDirShort),
		FilenameTemplate:        firstNonEmpty(*filenameTmpl, *filenameTmplP),
		Force:                   *force || *forceShort,
		DryRun:                  *dryRun,
		FixedLength:             time.Duration(*fixedLengthSec * float64(time.Second)),
		BySilence:               *bySilence,
		SilenceMin:              time.Duration(*silenceMinMs) * time.Millisecond,
		SilenceMax:              time.Duration(*silenceMaxMs) * time.Millisecond,
		ChaptersFilename:        *chaptersFile,
		UseExistingChaptersFile: *useExisting,
		CueSheetPath:            *cuePath,
		ReindexChapters:         *reindex,
		AudioFormat:             *audioFormat,
		AudioCodec:              *audioCodec,
		AudioBitrate:            *audioBitrate,
		AudioSampleRate:         *audioSampleRate,
		AudioChannels:           *audioChannels,
		Jobs:                    *jobs,
		TagOverrides: audio.Tag{
			Title:       *name,
			Album:       *album,
			Artist:      *artist,
			AlbumArtist: *albumArtist,
			Genre:       *genre,
			Writer:      *writer,
			Year:        *year,
			Series:      *series,
			SeriesPart:  *seriesPart,
		},
	}

	if err := split.Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "split:", err)
		return 1
	}
	return 0
}

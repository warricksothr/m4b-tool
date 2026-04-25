package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/merge"
	"github.com/warricksothr/m4b-tool/internal/tag"
)

// runMergeCmd parses the `merge` subcommand flags and invokes the
// orchestrator. M7a scope only: no transcoding, no batch, no jobs.
func runMergeCmd(args []string) int {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: m4b-tool merge [flags] <input> [input...]")
		fs.PrintDefaults()
	}

	var (
		output      = fs.String("output-file", "", "target .m4b/.mp4/.m4a path (required)")
		outputShort = fs.String("o", "", "alias for --output-file")
		force       = fs.Bool("force", false, "overwrite output without prompting")
		forceShort  = fs.Bool("f", false, "alias for --force")
		include     = fs.String("include-extensions", "", "CSV of extensions to include; default covers all common audio types")
		dryRun      = fs.Bool("dry-run", false, "print the plan without writing output")
		useFNChaps  = fs.Bool("use-filenames-as-chapters", false, "name chapters after input filenames even when the file has an embedded title")

		cueSheet         = fs.String("cuesheet", "", "path to a .cue file to import")
		chaptersFilename = fs.String("chapters-filename", "", "override path for the chapters.txt sidecar")

		coverPath         = fs.String("cover", "", "path to a cover image to embed (overrides auto-discovery)")
		skipCover         = fs.Bool("skip-cover", false, "do not embed or extract cover art")
		skipCoverIfExists = fs.Bool("skip-cover-if-exists", false, "leave embedded cover alone if one is already present")

		equateSpecs = stringSliceFlag{}

		enable  = fs.String("enable-improvers", "", "whitelist (CSV) of tag importers")
		disable = fs.String("disable-improvers", "", "blacklist (CSV) of tag importers")

		// Encoding pipeline (M7b).
		noConversion    = fs.Bool("no-conversion", false, "stream-copy concat instead of transcoding (inputs must share codec parameters)")
		audioCodec      = fs.String("audio-codec", "", "audio codec for transcoded output (default aac)")
		audioBitrate    = fs.String("audio-bitrate", "", "audio bitrate for transcoded output (default 64k)")
		audioSampleRate = fs.Int("audio-samplerate", 0, "audio sample rate (Hz)")
		audioChannels   = fs.Int("audio-channels", 0, "audio channel count")
		adjustForIPod   = fs.Bool("adjust-for-ipod", false, "clamp sample rate to 44100 and channels to 2")
		trimSilence     = fs.Bool("trim-silence", false, "trim silence from middle inputs (first and last keep their boundary silence)")
		addSilenceMs    = fs.Int("add-silence", 0, "milliseconds of silence to insert between consecutive parts")
		jobs            = fs.Int("jobs", 0, "concurrent transcoders (default 1)")

		// Batch mode (M7c).
		batchPatterns   = stringSliceFlag{}
		batchPath       = fs.String("batch-pattern-path", "", "base path matched batch patterns are taken relative to")
		batchFilter     = fs.String("batch-filter", "", "only consider directories whose relative path contains this substring")
		batchResumeFile = fs.String("batch-resume-file", "", "skip entries listed in this file; appended to as each entry completes")

		// Tag overrides (inherited set).
		name        = fs.String("name", "", "override title (--name)")
		sortName    = fs.String("sortname", "", "override sort title")
		album       = fs.String("album", "", "override album")
		sortAlbum   = fs.String("sortalbum", "", "override sort album")
		artist      = fs.String("artist", "", "override artist / narrator")
		sortArtist  = fs.String("sortartist", "", "override sort artist")
		albumArtist = fs.String("albumartist", "", "override album artist")
		genre       = fs.String("genre", "", "override genre")
		writer      = fs.String("writer", "", "override writer / composer")
		year        = fs.Int("year", 0, "override year")
		descFlag    = fs.String("description", "", "override short description")
		longdesc    = fs.String("longdesc", "", "override long description")
		comment     = fs.String("comment", "", "override comment")
		copyright   = fs.String("copyright", "", "override copyright")
		encodedBy   = fs.String("encoded-by", "", "override encoded-by")
		series      = fs.String("series", "", "set series (pseudo-tag)")
		seriesPart  = fs.String("series-part", "", "set series part (pseudo-tag)")
	)

	fs.Var(&equateSpecs, "equate", "CSV of tag fields to force-equal; first field is the source, rest are targets (repeatable)")
	fs.Var(&batchPatterns, "batch-pattern", "directory pattern with %a/%n/%g/%t/%s/%p placeholders; runs one merge per match (repeatable)")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return 2
	}

	outPath := firstNonEmpty(*output, *outputShort)

	cfg := merge.Config{
		Inputs:                 fs.Args(),
		Output:                 outPath,
		Force:                  *force || *forceShort,
		IncludeExtensions:      splitCSV(*include),
		DryRun:                 *dryRun,
		UseFilenamesAsChapters: *useFNChaps,
		NoConversion:           *noConversion,
		AudioCodec:             *audioCodec,
		AudioBitrate:           *audioBitrate,
		AudioSampleRate:        *audioSampleRate,
		AudioChannels:          *audioChannels,
		AdjustForIPod:          *adjustForIPod,
		TrimSilence:            *trimSilence,
		AddSilence:             time.Duration(*addSilenceMs) * time.Millisecond,
		Jobs:                   *jobs,
		BatchPatterns:          []string(batchPatterns),
		BatchPatternPath:       *batchPath,
		BatchFilter:            *batchFilter,
		BatchResumeFile:        *batchResumeFile,
		CueSheetPath:           *cueSheet,
		ChaptersFilename:       *chaptersFilename,
		CoverOverride:          *coverPath,
		SkipCover:              *skipCover,
		SkipCoverIfExists:      *skipCoverIfExists,
		EquateSpecs:            []string(equateSpecs),
		EnableImprovers:        splitCSV(*enable),
		DisableImprovers:       splitCSV(*disable),
		TagOverrides: audio.Tag{
			Title:           *name,
			SortTitle:       *sortName,
			Album:           *album,
			SortAlbum:       *sortAlbum,
			Artist:          *artist,
			SortArtist:      *sortArtist,
			AlbumArtist:     *albumArtist,
			Genre:           *genre,
			Writer:          *writer,
			Year:            *year,
			Description:     *descFlag,
			LongDescription: *longdesc,
			Comment:         *comment,
			Copyright:       *copyright,
			EncodedBy:       *encodedBy,
			Series:          *series,
			SeriesPart:      *seriesPart,
		},
	}

	if err := tag.ValidateNames(cfg.EnableImprovers); err != nil {
		fmt.Fprintln(os.Stderr, "merge:", err)
		return 2
	}
	if err := tag.ValidateNames(cfg.DisableImprovers); err != nil {
		fmt.Fprintln(os.Stderr, "merge:", err)
		return 2
	}

	if err := merge.Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "merge:", err)
		return 1
	}
	return 0
}

// splitCSV splits a comma-separated string into a trimmed slice,
// dropping empty entries. Returns nil for an empty input.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// stringSliceFlag accumulates values from a repeatable flag.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

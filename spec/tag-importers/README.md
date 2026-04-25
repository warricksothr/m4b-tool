# Tag Importers

A "tag importer" is a function that takes a partially-populated `Tag`
and a working directory (or explicit input path) and returns an
enriched `Tag`. It's the composition pattern m4b-tool uses to let
multiple metadata sources layer on top of each other — file tags
first, then a sidecar JSON, then a CLI override, etc.

Go signature:

```go
type Importer interface {
    Name() string              // e.g. "ffmetadata", "cuesheet"
    Improve(ctx context.Context, tag Tag, workDir string) (Tag, error)
}
```

A composite importer runs a list of them in order, each seeing the
output of the previous. Two merge policies to keep straight:

- **`MergeMissing`** — fill only zero-valued fields. Used by every
  importer by default.
- **`MergeOverwrite`** — overwrite unconditionally. Used for CLI
  overrides (`--name`, `--album`, etc.) applied after all importers
  run.

## v1 priority

**P0 — ship in v1.** Needed for the common local-files-only case.

| Importer | Stub | Role |
|----------|------|------|
| [ffmetadata](ffmetadata.md) | ✅ | Read ffmpeg's native metadata format. |
| [chapters-txt](chapters-txt.md) | ✅ | mp4chaps sidecar chapter file. |
| [description](description.md) | ✅ | Plain `description.txt` sidecar. |
| [cover](cover.md) | ✅ | Find `cover.jpg/png` in working directory. |
| [cuesheet](cuesheet.md) | ✅ | Cue sheets for flac split input. |
| [chapters-from-file-tracks](chapters-from-file-tracks.md) | ✅ | Derive chapters from input-file durations (merge). |
| [guess-chapters-by-silence](guess-chapters-by-silence.md) | ✅ | Align chapter boundaries to detected silences. |
| [equate](equate.md) | ✅ | `--equate` field-value propagation. |

**P1 — useful, can slip.**

| Importer | Input | Role | Target file |
|----------|-------|------|-------------|
| `ChaptersFromEpub` | EPUB 3 file | Extract chapter names from TOC. | src/library/Audio/Tag/ChaptersFromEpub.php |
| `OpenPackagingFormat` | `metadata.opf` | Calibre metadata sidecar. | src/library/Audio/Tag/OpenPackagingFormat.php |
| `MetadataJson` | `metadata.json` | Generic JSON sidecar. | src/library/Audio/Tag/MetadataJson.php |
| `M4bToolJson` | `m4b-tool.json` | m4b-tool's own JSON export. | src/library/Audio/Tag/M4bToolJson.php |
| `BuecherHtml` | `buecher.html` | Scrape buecher.de product page. | src/library/Audio/Tag/BuecherHtml.php |

**P2 — specialty, defer.**

| Importer | Input | Role | Target file |
|----------|-------|------|-------------|
| `AudibleJson` | `audible.json` | Audible product JSON. | src/library/Audio/Tag/AudibleJson.php |
| `AudibleTxt` | `audible.txt` | Audible metadata as text. | src/library/Audio/Tag/AudibleTxt.php |
| `AudibleChaptersJson` | `audible_chapters.json` | Audible chapter list. | src/library/Audio/Tag/AudibleChaptersJson.php |
| `ContentMetadataJson` | `content_metadata_*.json` | Audible content metadata. | src/library/Audio/Tag/ContentMetadataJson.php |
| `BookBeatJson` | `bookbeat.json` | BookBeat product JSON. | src/library/Audio/Tag/BookBeatJson.php |
| `BuchhandelJson` | `buchhandel.json` | Buchhandel.de product JSON. | src/library/Audio/Tag/BuchhandelJson.php |
| `ChaptersFromMusicBrainz` | MusicBrainz API | Fetch chapter list by recording ID. | src/library/Audio/Tag/ChaptersFromMusicBrainz.php |
| `ChaptersFromOverdrive` | Overdrive media markers | Extract markers embedded in source files. | src/library/Audio/Tag/ChaptersFromOverdrive.php |

## Chapter-adjustment helpers

Not importers, but live in the same PHP directory and operate on the
chapter list. Their logic is documented in
[chapter-algorithms.md](../chapter-algorithms.md):

- `AdjustChaptersByGroupLogic` — reindexing & grouping.
- `AdjustTooLongChapters` — length-based splitting.
- `AdjustTooShortChapters` — length-based merging.
- `MergeSubChapters` — flatten hierarchies.
- `RemoveDuplicateFollowUpChapters` — merge adjacent duplicates.
- `IntroOutroChapters` — synthesize intro/outro from offset flags.

In Go, these are functions in `internal/chapter/`, not importers.

## Composition order in v1

For `merge`:

```
1. ChaptersFromFileTracks           (compute base chapters from inputs)
2. ChaptersTxt (sidecar)            (override if present)
3. Ffmetadata (sidecar)             (tag metadata)
4. Cover (sidecar)                  (cover image)
5. Description (sidecar)            (description.txt)
6. CueSheet (if input is flac+cue)
7. Embedded tags from input files   (MergeMissing)
8. --equate propagation
9. CLI overrides                    (MergeOverwrite)
10. GuessChaptersBySilence          (final chapter alignment)
11. Length adjustments              (split too-long, merge too-short)
```

For `split`:

```
1. Embedded chapters from m4b
2. ChaptersTxt (if --use-existing-chapters-file)
3. CueSheet (if input is flac+cue)
4. Embedded tags
5. CLI overrides
```

For `chapters`:

```
1. Embedded chapters
2. ChaptersFromEpub (if --epub, P1)
3. GuessChaptersBySilence (if --adjust-by-silence)
4. Normalization / shift / merge-similar passes
5. CLI overrides
```

## Selection flags

`--enable-improvers <csv>` and `--disable-improvers <csv>` are
whitelist / blacklist filters applied to the importer list at runtime.
Names are case-insensitive. Implement as a simple set check inside the
composite runner.

# CLI Surface

v1 commands: `merge`, `split`, `chapters`. All flags below are the surface
the Go port should expose. Flag names match the PHP tool's long-name form
unless noted; short forms are kept where users are likely to have
muscle-memory on them.

"PHP default" means the reference implementation's current default. The
Go port should keep these unless a call-out says otherwise.

## Global flags (inherited by all commands)

| Flag | Default | Purpose |
|------|---------|---------|
| `--debug` | off | Verbose logging, keep temp files, write `m4b-tool.log`. |
| `--logfile <path>` | — | Log to a specific file. |
| `--force`, `-f` | off | Overwrite existing output without prompt. |
| `--no-cache` | off | Invalidate on-disk cache before run. |
| `--tmp-dir <path>` | `$M4BTOOL_TMP_DIR` or OS temp | Working directory for intermediates. |
| `--no-cleanup` | off | Keep `.chapters.txt` and other intermediates. |
| `--ffmpeg-threads <auto\|N>` | auto | `-threads` passed to ffmpeg. |
| `--ffmpeg-param <arg>` (repeatable) | — | Extra args appended to every ffmpeg invocation. |
| `--silence-min-length <ms>`, `-a` | 1750 | Minimum silence duration for detection. |
| `--silence-max-length <ms>`, `-b` | 0 | Maximum silence duration (0 = unlimited). |
| `--platform-charset <name>` | — | Charset for filesystem paths (e.g., `Windows-1252`). |
| `--enable-improvers <csv>` | — | Whitelist of tag importers to enable. |
| `--disable-improvers <csv>` | — | Blacklist of tag importers to disable. |

Also available on `merge` and `chapters` (chapter-length constraints):

| Flag | Default | Purpose |
|------|---------|---------|
| `--min-chapter-length <spec>` | `1` | Minimum chapter length in seconds. Syntax: `N[idx,idx,...]` to preserve specified indexes (e.g., `2.5[0,1,-1]`). |
| `--max-chapter-length <spec>` | `0` | Maximum chapter length. Syntax: `desired,max` to auto-split long chapters (e.g., `300,900`). |

## merge

Combine a set of audio files into a single m4b (or mp3/mp4) with chapter
markers.

**Arguments**
- `input` (required): file, directory, or glob. Directories are scanned
  recursively.
- Additional inputs (variadic): combined with the first.

**Output**
- `--output-file <path>`, `-o` (required): target file; in batch mode this
  becomes the target directory.

**File collection**
- `--include-extensions <csv>` — default: `aac,alac,flac,m4a,m4b,mp3,oga,ogg,wav,wma,mp4`.
- `--batch-pattern <pat>` (repeatable): placeholder-driven batch
  processing, e.g. `%a/%n` splits into (artist, title). Requires
  `--output-file` be a directory. Recognized placeholders:
  `%a`=Artist, `%n`=Title (Name), `%g`=Genre, `%t`=Album, `%s`=Series,
  `%p`=SeriesPart, plus `%%` for a literal `%`. Each placeholder
  consumes one or more non-separator characters, so adjacent
  placeholders separated by literals (e.g. `%p - %n`) split at the
  literal rather than greedily. Patterns are anchored — a path must
  match a pattern in full, not as a prefix. The first pattern that
  matches a candidate directory wins. Per-entry output filenames are
  the matched directory's basename plus the extension implied by
  `--output-file` (defaults to `.m4b` when the path is directory-shaped).
  Tag overrides from CLI flags still win against placeholder values.
- `--batch-pattern-path <path>`: base path matched patterns are taken
  relative to. Defaults to each input root.
- `--batch-filter <substr>`: skip paths that don't contain the substring.
- `--batch-resume-file <path>`: skip entries listed in this file, then
  append each successful absolute source-dir path on completion. Lines
  starting with `#` and blank lines are ignored.
- `--dry-run`: enumerate actions without executing.
- `--jobs <N>` — default: 1. Parallel ffmpeg encoders.

**Audio encoding**
- `--audio-format <fmt>` — default: `m4b`. One of `m4b`, `mp4`, `mp3`.
- `--audio-extension <ext>`: override file suffix.
- `--audio-codec <name>`: e.g., `aac`, `libfdk_aac`, `libmp3lame`. Auto
  from `--audio-format` if omitted.
- `--audio-bitrate <rate>`: e.g., `64k`, `128k`.
- `--audio-samplerate <hz>`: e.g., `22050`, `44100`.
- `--audio-channels <N>`
- `--audio-quality <0-100>`: VBR quality percentage.
- `--audio-profile <aac_he|aac_he_v2>`: low-bitrate AAC profiles.
- `--adjust-for-ipod`: clamp to iPod-compatible sample rates/bitrates.
- `--no-conversion`: skip transcoding; concatenate source-encoded.
- `--add-silence <ms>`: insert silence between concatenated files.
- `--trim-silence`: trim silence at start/end of each input (except first
  and last).
- `--fix-mime-type`: rewrite `video/mp4` → `audio/mp4`.

**Chapters**
- `--musicbrainz-id <id>`, `-m`: MusicBrainz recording ID. **P2**.
- `--use-filenames-as-chapters`: chapter name = filename of input part.
- `--no-chapter-reindexing`: don't auto-renumber numeric-only names.
- `--chapter-algo <none|legacy|grouping>` — default: `legacy`.
- `--prepend-series-to-longdesc`: prepend series/part to long description.
- `--equate <csv>` (repeatable): enforce equality across tag fields
  (e.g., `artist,albumartist,sortartist`).
- `--tag-debug-path <dir>`: dump tag-resolution debug info.

**Tag overrides** (inherited, also on `split`/`chapters`)
- `--name`, `--sortname`, `--album`, `--sortalbum`, `--artist`,
  `--sortartist`, `--albumartist`, `--genre`, `--writer`, `--year`,
  `--description`, `--longdesc`, `--comment`, `--copyright`,
  `--encoded-by`, `--series`, `--series-part`.
- `--cover <path>`: override cover image.
- `--skip-cover`: don't extract or embed cover.
- `--skip-cover-if-exists`: embed existing cover, don't re-extract.
- `--remove <tag>` (repeatable): drop a tag field.
- `--ignore-source-tags`: ignore embedded tags from inputs.
- `--prefer-metadata-tags`: metadata-file tags beat CLI-provided tags.

**Behavior**
1. Collect inputs recursively, filter by `--include-extensions`.
2. If `--batch-pattern`: match directories against pattern, extract tag
   values from placeholders, run one merge per batch group.
3. For each input: encode via ffmpeg (or fdkaac pipeline) in a worker pool
   of `--jobs` size. Apply `--trim-silence` except on boundary files.
4. If `--add-silence`: synthesize a silence segment in CAF, transcode to
   target format, interleave between parts.
5. Concatenate via `ffmpeg -f concat`.
6. Run tag importers (cue, ffmetadata, OverDrive, file tags, MusicBrainz,
   EPUB, JSON). Apply chapter adjustment via silence detection / length
   rules.
7. Embed cover via `mp4art --add` if present. Write tags via `mp4tags`
   (and/or `tone`).
8. Clean up temp files (unless `--debug` / `--no-cleanup`).

**Examples**
```
m4b-tool merge "data/my-audio-book/" --output-file="data/merged.m4b"
m4b-tool merge --jobs=2 --output-file="output/" \
    --max-chapter-length=300,900 --adjust-for-ipod \
    --batch-pattern="input/%g/%a/%s/%p - %n/" "input/"
```

## split

Split a single audio file into one file per chapter.

**Argument**
- `input` (required): single file — `.m4b`, `.mp3`, or `.flac` + cue
  sheet.

**Output**
- `--output-dir <path>`, `-o` — default: `<basename>_splitted/` next to
  input.
- `--filename-template <tmpl>`, `-p` — default:
  `{{"%03d"|format(track)}}-{{title|raw}}`. Twig template with access
  to tag fields. **Port note:** v1 can ship with Go `text/template`
  instead of Twig; the format differs but the variables (`track`,
  `title`, `album`, `artist`, etc.) are the same. Document this as a
  breaking change from the PHP tool.

**Chapter source**
- `--use-existing-chapters-file`: prefer `<basename>.chapters.txt`.
- `--chapters-filename <path>`: explicit external chapters file.
- `--by-silence`: derive chapters from silence detection.
- `--fixed-length <seconds>`: fixed-duration splits (float allowed).
- `--reindex-chapters`: replace names with `1, 2, 3, ...`.

**Silence**
- `--add-silence <pre[,post]>`: pre/post silence in ms; single value
  applies to both.

**Audio encoding** — same flag set as `merge`.

**Tag overrides** — same inherited set; applied to every output file.

**Behavior**
1. Probe input duration.
2. Resolve chapter source, in priority order:
   `--fixed-length` → `--by-silence` → `--chapters-filename` →
   sidecar `.chapters.txt` → cue sheet → embedded chapters.
3. If `--reindex-chapters`, replace titles with sequence numbers.
4. For each chapter: render filename template, call
   `ffmpeg -ss <start> -t <duration>` to extract, optionally prepend
   /append silence, tag with chapter title as track title + track
   number + cover.
5. Clean up temp files.

**Examples**
```
m4b-tool split --audio-format mp3 --audio-bitrate 96k --audio-channels 1 \
    --audio-samplerate 22050 "data/my-audio-book.m4b"
m4b-tool split --audio-format=mp3 --audio-bitrate=192k "data/my-album.flac"
```

## chapters

Add, adjust, or import chapter markers on a single file.

**Argument**
- `input` (required): single audio file.

**Output**
- `--output-file <path>`, `-o`: write chapters to text file instead of
  modifying the audio file.

**Chapter sources**
- `--musicbrainz-id <id>`, `-m`. **P2**.
- `--epub <path>`: pull chapter titles from EPUB TOC. **P1**.
- `--epub-restore`: restore chapters from `.chapters.bak.txt`.
- `--epub-dump`: dump EPUB chapters without writing.
- `--epub-ignore-chapters <csv>`: chapter indexes to skip (0-based;
  `-1` = last).
- `--epub-append-introduction`: append first-words snippet to numbered
  chapter names.
- `--adjust-by-silence`: snap existing chapter boundaries to nearest
  detected silence.

**Chapter manipulation**
- `--normalize`: strip chars, regex, optimize titles.
- `--merge-similar`, `-s`: collapse similar adjacent chapter prefixes.
- `--shift <ms[:idx,idx,...]>`: shift specified chapters (or all) by
  milliseconds.
- `--no-chapter-numbering`: don't append `(1)`, `(2)` suffixes.
- `--no-chapter-import`: write `.chapters.txt` but don't update the file.
- `--chapter-pattern <regex>` — default: parses `Chapter N: Title` style.
- `--chapter-replacement <repl>` — default: `$1`.
- `--chapter-remove-chars <chars>` — default: `„""`.
- `--find-misplaced-chapters <csv>`: debug — mark silence regions
  around specified chapters.
- `--find-misplaced-offset <sec>` — default: `120`. Search window.
- `--find-misplaced-tolerance <ms>` — default: `-4000`. Companion
  silence offset.
- `--first-chapter-offset <ms>` — default: `0`.
- `--last-chapter-offset <ms>` — default: `0`.

**Tag overrides** — same inherited set.

**Behavior**
Branches by flag:
- `--epub[-restore]`: load EPUB TOC, align to silences, back up existing
  chapters to `.chapters.bak.txt`, filter ignored chapters, apply
  length adjustments, write.
- `--musicbrainz-id`: fetch + cache MB recording, parse chapter list.
- `--adjust-by-silence`: read current chapters, snap each to nearest
  silence within tolerance.
- `--normalize`: regex/replace/strip characters; optionally merge
  similar; optionally renumber.
- `--shift`: apply offset to listed indexes.
- Otherwise: read chapters, optionally export to file.

Then (unless `--no-chapter-import`):
- Write chapters via `mp4chaps -i` (requires sidecar
  `<basename>.chapters.txt`).

**Examples**
```
m4b-tool chapters --adjust-by-silence -o "data/adjusted.m4b" "data/source.m4b"
m4b-tool chapters --merge-similar --first-chapter-offset 4000 \
    --last-chapter-offset 3500 "data/harry-potter-1.m4b"
m4b-tool chapters --normalize --shift 3000:0,1,4,5 "data/audiobook.m4b"
```

## Summary matrix

| Aspect            | merge                                              | split                           | chapters                     |
|-------------------|----------------------------------------------------|---------------------------------|------------------------------|
| Input             | file/dir/glob (recursive)                          | single file                     | single file                  |
| Output            | single file (or dir for batch)                     | directory of files              | file written-in-place or txt |
| Transcoding       | always (unless `--no-conversion`)                  | on-demand                       | never                        |
| Parallel          | `--jobs=N`                                         | no                              | no                           |
| Batch             | `--batch-pattern`                                  | no                              | no                           |
| Chapter sources   | tags, cue, OverDrive, MB, JSON, silence, EPUB      | embedded, `.chapters.txt`, cue, silence, fixed | embedded, EPUB, silence-snap, MB |
| v1 critical flags | everything except `--musicbrainz-id`               | everything                      | everything except `--musicbrainz-id` and `--epub` (P1 if time permits) |

## Exit codes

- `0` — success.
- `1` — any error (invalid input, output exists without `--force`,
  missing tool, ffmpeg failure, chapter parse failure, etc.). PHP tool
  doesn't distinguish further; Go port should keep that contract for v1
  but log specific errors at ERROR level with enough detail to diagnose.

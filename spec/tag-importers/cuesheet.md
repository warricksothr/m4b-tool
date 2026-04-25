# cuesheet

**Priority**: P0
**PHP**: `src/library/Audio/CueSheet.php`

## Role

Parse a cue sheet accompanying a single audio file (typically `.flac`)
and produce a tag bag with chapters. This is the only way `split`
works on flac inputs.

## Inputs

- Explicit file path, typically `<input-basename>.cue`.
- Also picked up when the merge command scans a directory: the first
  `.cue` file matching any input becomes its metadata source.

## Output fields

- `Tag.Album` (from top-level `TITLE`)
- `Tag.Artist` (from top-level `PERFORMER`)
- `Tag.Writer` (from top-level `SONGWRITER`)
- `Tag.Genre` (from top-level `REM GENRE` or `GENRE`)
- `Tag.Year` (from top-level `REM DATE` or `DATE`)
- `Tag.Chapters` (one per `TRACK` block, using `INDEX 01` as start
  time, `TITLE` as chapter name)

Merge policy: `MergeMissing` for scalar fields; cue chapters
overwrite existing chapters if present (they're authoritative).

## Grammar

See [chapter-algorithms.md](../chapter-algorithms.md#cue-sheet-parsing)
for the full grammar and chapter-building algorithm. Summary:

```
REM DATE 2024
PERFORMER "Author Name"
TITLE "Book Title"
FILE "book.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Chapter 1"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Chapter 2"
    INDEX 01 05:30:00
  TRACK 03 AUDIO
    TITLE "Chapter 3"
    INDEX 01 11:15:45
```

Time format is `MM:SS:FF` where `FF` is CD frames (0–74, 75/s). Convert:
```
totalMs = min*60000 + sec*1000 + frames*1000/75
```

## Edge cases

- REM lines can appear in any position and carry arbitrary key/value
  pairs. Recognized: `GENRE`, `DATE`, `COMPOSER`, `COMMENT`. Others
  are stored in `Tag.Extra["rem_<key>"]`.
- `INDEX 00` is a pre-gap marker; treat as "previous track ends here"
  only if present, otherwise infer from next track's `INDEX 01`.
- Gaps between tracks > 4 seconds: don't assume contiguous chapter
  coverage; leave chapter length unset for such chapters and let the
  file duration infer the last one.
- Multi-file cue sheets (multiple `FILE` directives): v1 supports
  single-file cues only. Log a warning and process only the tracks
  under the first `FILE` block.
- Quoted strings may contain escaped quotes via `\"`. Unescape on
  parse.
- Character encoding: cue files are often Windows-1252 or Latin-1.
  Apply `--platform-charset` if set; otherwise try UTF-8 first, fall
  back to Windows-1252 on invalid UTF-8.

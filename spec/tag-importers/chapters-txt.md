# chapters-txt

**Priority**: P0
**PHP**: `src/library/Audio/Tag/ChaptersTxt.php`

## Role

Load chapters from a `mp4chaps`-style sidecar file. This is the
user-editable format — when someone hand-tunes chapter positions,
they edit `foo.chapters.txt` and re-run.

## Inputs

- `<basename>.chapters.txt` next to the audio file.
- Path passed via `--chapters-filename`.

## Output fields

Only `Tag.Chapters`. Merge policy: if this importer is selected, it
**overwrites** any existing chapters (not MergeMissing) — the user
asked for these chapters specifically.

## Grammar

```
## total-duration: 12:34:56.789
00:00:00.000 Chapter 1
00:05:30.123 Chapter 2
01:23:45.678 Chapter 3 with, punctuation and "quotes"
```

Rules:
- Lines starting with `##` are comments. The specific comment
  `## total-duration: HH:MM:SS.mmm` is recognized but not required.
- Data lines: timestamp, single space, rest-of-line is the title.
- Timestamp: `HH:MM:SS.mmm`. Fewer digits tolerated on read
  (`H:MM:SS`, `HH:MM:SS`), always three decimal places on write.
- Chapter end is inferred from the next chapter's start, or the file
  duration for the last chapter.
- Blank lines allowed; ignored.

## Edge cases

- Title contains the word `## `: safe — only lines *starting* with
  `##` are comments.
- Timestamps out of order → sort on read, log a warning.
- Duplicate timestamps → keep the first, drop the rest with a warning.
- Total-duration comment disagrees with probed file duration: ignore
  the comment; it's informational only.

## Writer

The port also generates this file as a sidecar for `mp4chaps -i`. See
[external-tools.md](../external-tools.md#mp4chaps). Emit with three
decimal places always, one space separator, no trailing whitespace.

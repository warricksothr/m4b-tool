# chapters-from-file-tracks

**Priority**: P0
**PHP**: `src/library/Audio/Tag/ChaptersFromFileTracks.php`

## Role

Used by `merge` only. Derives chapter boundaries from the durations
of the input files being merged — each file becomes one chapter,
named after the file (or after the file's embedded title tag).

## Inputs

- The list of input files being merged (already duration-probed).
- Flag `--use-filenames-as-chapters`: if set, use filename stem as
  chapter name; otherwise use the file's embedded `title` tag if
  present, falling back to filename stem.

## Output fields

- `Tag.Chapters` — one chapter per input file, starts accumulate
  from 0.

Merge policy: foundational — this is called *before* any other
chapter importer. Subsequent importers may overwrite.

## Algorithm

```go
func BuildChaptersFromFiles(files []ProbeResult, useFilenames bool) []Chapter {
    out := make([]Chapter, 0, len(files))
    cursor := time.Duration(0)
    for _, f := range files {
        name := ""
        if !useFilenames && f.Tag.Title != "" {
            name = f.Tag.Title
        } else {
            name = filenameStem(f.Path)
        }
        out = append(out, Chapter{
            Start:  cursor,
            Length: f.Duration,
            Name:   name,
        })
        cursor += f.Duration
    }
    return out
}
```

## Edge cases

- File with zero duration → emit a zero-length chapter anyway; later
  algorithms will collapse it.
- Two files with identical titles: the `--no-chapter-reindexing` flag
  controls whether numeric suffixes get appended in normalization
  (see [chapter-algorithms.md](../chapter-algorithms.md#normalization)).
- File is itself an m4b with embedded chapters: **do not** flatten
  those here. This importer treats each file as one chapter; the
  `merge` orchestrator should extract and merge embedded chapter
  lists separately, then pick the more-informative one.

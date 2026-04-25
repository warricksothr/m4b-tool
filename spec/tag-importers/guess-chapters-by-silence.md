# guess-chapters-by-silence

**Priority**: P0
**PHP**: `src/library/Audio/Tag/GuessChaptersBySilence.php`,
main algorithm at `src/library/Chapter/ChapterMarker.php:25`

## Role

Align existing chapter boundaries onto detected silence points. Used
by `chapters --adjust-by-silence` and called implicitly by `merge`
when a metadata-provided chapter list needs to be snapped to the
actual audio.

Runs *after* other chapter sources have populated `Tag.Chapters`.

## Inputs

- `Tag.Chapters` — initial list, typically from metadata.
- Detected silences — retrieved via the ffmpeg client with
  `silence-min-length` and `silence-max-length` settings.
- Total audio duration.

## Output fields

- `Tag.Chapters` — same chapters, with `Start` adjusted to the
  nearest silence midpoint within tolerance.

Merge policy: in-place rewrite of the chapter list.

## Algorithm

Full pseudocode in
[chapter-algorithms.md](../chapter-algorithms.md#silence-based-chapter-alignment).
Summary:

1. Don't move chapter 0 (anchored at time 0).
2. For each subsequent chapter, find the silence whose start is
   closest to the chapter's (offset-adjusted) start; if within 25 s,
   snap to the silence's midpoint.
3. Track cumulative offset so later chapters drift with the adjustments
   made to earlier ones.
4. Recompute lengths as differences between successive starts.
5. Clamp last chapter to file duration; salvage if last chapter ended
   up shorter than 2.5 s by searching for an earlier silence to
   re-anchor.

## Parameters

- `maxDiff` — 25 s. Hard-coded in the PHP tool as
  `ChapterMarker::MAX_DIFF_MILLISECONDS`; expose as a config struct
  field in Go, default 25 s.
- `lastChapterMin` — 2.5 s. Likewise constant in PHP; expose as
  config.

## Edge cases

- No silences detected → no-op, return chapters unchanged.
- Only one chapter → no-op.
- Chapters already perfectly aligned → no-op (all distances ≥ maxDiff).
- Silences denser than chapters → only one silence picked per chapter;
  repeated silences between chapter boundaries are ignored.

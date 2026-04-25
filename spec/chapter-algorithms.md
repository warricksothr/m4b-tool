# Chapter Algorithms

The real domain logic of m4b-tool. These algorithms get called from
`merge` and `chapters` (and a little from `split`). Each section has
enough detail to reimplement in Go without reading the PHP source.

All times below are `time.Duration`. Silences and chapters are as
defined in [data-model.md](data-model.md).

## Silence parsing

**PHP**: `src/library/Parser/SilenceParser.php`.

ffmpeg's `silencedetect` filter emits stderr lines of the form:

```
[silencedetect @ 0x...] silence_start: 123.456
[silencedetect @ 0x...] silence_end: 125.789 | silence_duration: 2.333
```

Parse only `silence_end` lines with this regex:

```
^.*silence_end:\s+([0-9]+\.[0-9]+)\s+\|\s+silence_duration:\s+([0-9]+\.[0-9]+)$
```

Group 1 is end time (seconds, float), group 2 is duration. Start is
`end - duration`. Convert to `time.Duration` (ms precision).

Filter thresholds are set on the ffmpeg command line
(`silencedetect=noise=-30dB:d=<minSec>`), not post-hoc in the parser.
Defaults: `noise = -30dB`, `d = silenceMinLength / 1000`.

Also watch for the `Duration:` line in stderr early in the stream —
capture total stream duration for downstream use.

Edge cases:
- Very short streams may produce no `silence_end` events. Return an
  empty slice; don't error.
- Lines are emitted in time order. No need to sort the result.

`DetectSilence(ctx, path, minLen, maxLen)` wraps the above:
- `minLen` is passed to ffmpeg as `d=<minLen>`; regions shorter than
  this are never emitted by the filter in the first place.
- `maxLen`, if non-zero, is a post-parse upper bound. Regions longer
  than `maxLen` are dropped as suspect (extended ambient gaps, long
  silent intros that would swamp chapter alignment). A zero `maxLen`
  keeps everything the filter emits.

## Silence-based chapter alignment

**PHP**: `ChapterMarker::guessChaptersBySilences`
(`src/library/Chapter/ChapterMarker.php:25`).

Given a list of chapters (from metadata) and a list of silences
(from ffmpeg), snap chapter boundaries onto the nearest detected
silence.

**Inputs**
- `chapters []Chapter` — initial chapters, ordered by `Start`.
- `silences []Silence` — detected silences, ordered by `Start`.
- `total time.Duration` — total audio duration.

**Parameters**
- `maxDiff = 25 * time.Second` — farthest a chapter boundary may drift.
- `lastChapterMin = 2500 * time.Millisecond` — threshold for
  last-chapter salvage logic.

**Algorithm**

```
accumulatedOffset := 0

for i, ch := range chapters:
    if ch.Start == 0:
        continue // don't move the first chapter

    adjustedStart := ch.Start - accumulatedOffset
    best := nil
    bestDist := maxDiff

    for _, s := range silences:
        dist := abs(adjustedStart - s.Start)
        if dist < bestDist:
            best = s
            bestDist = dist

    if best != nil:
        chapters[i].Start = best.Midpoint()
        accumulatedOffset += (adjustedStart - best.Midpoint())
    // else: leave chapter at adjustedStart (don't move)

// Backward length pass
for i := 0; i < len(chapters)-1; i++:
    chapters[i].Length = chapters[i+1].Start - chapters[i].Start

// Last chapter
chapters[last].Length = total - chapters[last].Start

// Salvage: last chapter too short?
if chapters[last].Length < lastChapterMin:
    // Find the most recent silence that is > lastChapterMin before chapters[last-1].End
    for _, s := range reverse(silences):
        if chapters[last-1].End() - s.Midpoint() > lastChapterMin:
            chapters[last].Start = s.Midpoint()
            chapters[last].Length = total - s.Midpoint()
            chapters[last-1].Length = s.Midpoint() - chapters[last-1].Start
            break
```

Edge case: if no chapters matched *any* silence AND the gap between
consecutive chapter starts exceeds 60 s, synthesize chapters from
silences in that gap. This is a recovery mode and rarely fires in
practice.

## Shifting

**PHP**: `ChapterShifter::shiftChapters`
(`src/library/Chapter/ChapterShifter.php:15`).

Move specified chapters (or all) by a millisecond offset. Reject the
shift if it would produce a negative-length chapter.

**Inputs**
- `chapters []Chapter`
- `shift time.Duration` — positive = later, negative = earlier.
- `indexes []int` — specific chapters to shift; empty = all. Negative
  indexes count from the end (`-1` = last).

**Algorithm**

```
// Normalize indexes
if len(indexes) == 0:
    indexes = [0, 1, ..., len(chapters)-1]
for i, idx := range indexes:
    if idx < 0:
        indexes[i] = len(chapters) + idx

// Shift
for _, i := range indexes:
    if i != 0:
        chapters[i].Start += shift
        if i-1 valid: chapters[i-1].Length is now wrong — recompute
    if i != len(chapters)-1:
        chapters[i].Length stays (next chapter moved too) or shrinks

// Validate: any chapter with negative length → revert entire shift
for _, c := range chapters:
    if c.Length < 0:
        return ErrShiftInvalid
```

Practical implementation: operate on a copy, validate, swap in on
success. The PHP version mutates in place and reverts on failure;
copy-swap is cleaner in Go.

## Length-based splitting (long chapters)

**PHP**: `ChapterLengthCalculator::splitTooLongChapterBySilence`
(`src/library/Chapter/ChapterGroup/ChapterLengthCalculator.php:198`).

For any chapter whose length exceeds the user's max, carve it into
pieces near the `desired` length, preferring silence boundaries.

**Inputs**
- `chapter Chapter` — the over-long chapter.
- `silences []Silence`
- `desired time.Duration`
- `max time.Duration`

**Algorithm**

```
result := []
cursor := chapter.Start
end := chapter.End()

for cursor < end:
    windowStart := cursor + desired
    windowEnd := min(cursor + max, end)

    // Prefer the first silence whose midpoint lies in [windowStart, windowEnd]
    split := time.Duration(0)
    for _, s := range silences:
        mid := s.Midpoint()
        if mid >= windowStart && mid <= windowEnd:
            split = mid
            break

    if split == 0:
        // No silence found; hard cut at windowStart
        split = windowStart

    result = append(result, Chapter{Start: cursor, Length: split - cursor, Name: chapter.Name})
    cursor = split

// Tail handling: last piece too small? Merge into previous if within max.
if len(result) >= 2:
    lastLen := result[last].Length
    prevLen := result[last-1].Length
    if lastLen < desired && (lastLen + prevLen) < max:
        result[last-1].Length += lastLen
        result = result[:last]
```

Inherit the parent chapter's name on each split piece; numbering is
applied in a later pass (see [Normalization](#normalization)).

Driver: `ChapterLengthCalculator::splitTooLongChapters` iterates over
all chapters, calling the above on any whose length exceeds `max`.

## Length-based merging (short chapters)

**PHP**: `ChapterHandler::mergeNeedlessSplits`
(`src/library/Chapter/ChapterHandler.php:278`) and
`AdjustTooShortChapters`.

Consolidate very short chapters into their neighbors.

**Parameters**
- `minChapterLength` — user-supplied `--min-chapter-length`, default
  `60 * time.Second` in the tail-trimming variant, `1 * time.Second`
  in general.
- `maxLength` — upper bound on the merged chapter.

**Tail-trim variant** (merge last chapter if short):
```
if chapters[last].Length < 60s && chapters[last].Length + chapters[last-1].Length <= maxLength:
    chapters[last-1].Length += chapters[last].Length
    chapters = chapters[:last]
```

**General variant**: walk the list; for each chapter < minLength,
append its length to its predecessor (or, if there is no predecessor,
prepend to its successor). Exempt indexes passed via the bracket
syntax `2.5[0,1,-1]`.

## Normalization (renaming & numbering)

**PHP**: `ChapterMarker::normalizeChapters`
(`src/library/Chapter/ChapterMarker.php:284`),
`ChapterHandler::adjustChapterNames`.

Apply regex extraction, character stripping, and duplicate-index
suffixes.

**Parameters** (from flags on `chapters` command)
- `pattern` — default `^[^:]+[1-9][0-9]*:\s*(.*),.*[1-9][0-9]*\s*$` —
  matches "Chapter 1: Title, 00:00:30" → captures "Title".
- `replacement` — default `$1`.
- `removeChars` — default `„""` (smart quotes). UTF-8 aware.
- `mergeSimilar` — collapse consecutive identical names.
- `noNumbering` — don't append `(1)`, `(2)` suffixes.

**Algorithm**

```
for i, c := range chapters:
    name := regexp.ReplaceAll(pattern, replacement, c.Name)
    name = stripChars(name, removeChars)
    chapters[i].Name = name

// Duplicate-index pass
counters := map[string]int{}
lastName := ""
for i, c := range chapters:
    if mergeSimilar && c.Name == lastName:
        // delete this chapter, extend previous
        continue
    if c.Name == lastName:
        counters[c.Name]++
        if !noNumbering:
            chapters[i].Name = fmt.Sprintf("%s (%d)", c.Name, counters[c.Name])
    else:
        counters = map[string]int{}
    lastName = c.Name
```

Offset-insertion: if `--first-chapter-offset` is set, prepend a
synthetic `Chapter{Start: 0, Length: offset, Name: "Intro"}`. If
`--last-chapter-offset` is set, append a synthetic
`Chapter{Start: end - offset, Length: offset, Name: "Outro"}`.

**Consecutive-numbering heuristic** (`adjustChapterNames`): if > 75%
of chapters have names that are identical except for their numeric
components, treat them as a numbered series and rewrite as
`1, 2, 3, ...` or `1.1, 1.2, 2.1` for hierarchical patterns. Below
that threshold, fall back to dup-suffix behavior.

## Duplicate adjacent chapter removal

**PHP**: `ChapterHandler::removeDuplicateFollowUps`
(`src/library/Chapter/ChapterHandler.php:672`).

If two adjacent chapters have the exact same name, merge the second
into the first.

```
for i := 0; i < len(chapters)-1; i++:
    if chapters[i].Name == chapters[i+1].Name:
        chapters[i].Length += chapters[i+1].Length
        chapters = append(chapters[:i+1], chapters[i+2:]...)
        i--  // re-check
```

Preserve `Intro` and `Outro` reserved names: if either is involved in
a merge, adjust the bookkeeping so those markers remain distinct at
file boundaries.

## Overlap-based chapter matching

**PHP**: `ChapterHandler::overloadTrackChapters`
(`src/library/Chapter/ChapterHandler.php:612`).

When two chapter lists exist (one from physical file boundaries,
another from metadata like MusicBrainz or Audible), match each track
chapter to the named chapter that overlaps it most.

```
for i, t := range trackChapters:
    bestOverlap := 0
    bestIdx := -1
    for j, n := range namedChapters:
        overlap := min(t.End(), n.End()) - max(t.Start, n.Start)
        if overlap > bestOverlap:
            bestOverlap = overlap
            bestIdx = j
    if bestIdx >= 0:
        trackChapters[i].Name = namedChapters[bestIdx].Name
        trackChapters[i].Introduction = namedChapters[bestIdx].Introduction
```

No overlap → leave the track chapter's original name (usually a
filename or blank). Only used by MusicBrainz (P2) and
ChaptersFromOverdrive (P2) in v1 scope.

## Cue sheet parsing

**PHP**: `src/library/Audio/CueSheet.php`.

A cue sheet is a plaintext metadata file that accompanies a single
audio file (commonly `.flac`) and divides it into tracks.

**Grammar** (the subset m4b-tool supports):

```
REM <KEY> <value>               ; optional, free-form metadata
PERFORMER "<name>"              ; disc-level or track-level
SONGWRITER "<name>"
TITLE "<title>"
GENRE "<genre>"
DATE "<year>"
FILE "<filename>" <format>      ; informational
TRACK <NN> AUDIO
  TITLE "<chapter name>"
  PERFORMER "<name>"
  INDEX 00 MM:SS:FF             ; optional — previous track end
  INDEX 01 MM:SS:FF             ; required — current track start
```

Time format for `INDEX` is `MM:SS:FF`, where `FF` is CD frames
(0–74, 75 per second). Parse as:

```go
totalMs := minutes*60_000 + seconds*1000 + frames*1000/75
```

**Algorithm**

```
tag := Tag{}
chapters := []Chapter{}
currentTrack := nil

for line := range scanner:
    switch directive(line):
    case "PERFORMER":  if no track yet: tag.Artist = value else currentTrack.performer = value
    case "SONGWRITER": if no track yet: tag.Writer = value
    case "TITLE":      if no track yet: tag.Album = value else currentTrack.title = value
    case "GENRE":      tag.Genre = value
    case "DATE":       tag.Year = parseInt(value)
    case "TRACK":      currentTrack = new Track{Num: parseInt}
    case "INDEX":
        switch idx:
        case 01: currentTrack.start = parseTime(value)
        case 00: // previous track's end, if close enough
    ... etc

// Build chapters
for i, t := range tracks:
    ch := Chapter{Start: t.start, Name: t.title}
    if i < len(tracks)-1:
        ch.Length = tracks[i+1].start - t.start
    // Last chapter: leave Length = 0; caller fills from file duration.
    chapters = append(chapters, ch)

tag.Chapters = chapters
```

Pregap handling note: the cue format allows an `INDEX 00` timecode on
a track that marks the start of its pregap (the end of the *previous*
track's audio). The PHP tool uses this when the gap between `INDEX 00`
of track N+1 and `INDEX 01` of track N is small (≤ 4 s), tightening
the previous chapter's Length to exclude the pregap. Audiobook cue
sheets rarely carry meaningful `INDEX 00` values, so the Go port's v1
parser ignores them; a later milestone can layer pregap-aware
tightening on top of the simple gap-to-next-start rule above.

Cue sheets are P0 because they're the only way `split` works on
`.flac` inputs.

## Misplaced-chapter diagnostic (P2)

`--find-misplaced-chapters` is a debug aid that doesn't modify output.
It prints silence regions near each listed chapter index within
`--find-misplaced-offset` seconds. The Go port can ship this in v1 or
defer to a later milestone; it's low-effort either way.

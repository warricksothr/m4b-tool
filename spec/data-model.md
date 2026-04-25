# Data Model

Core types the Go port needs. Field names are suggestions; the shape is
what matters. Where the PHP tool keeps a type for historical reasons
that Go doesn't need, the Go equivalent is called out.

## Time

All durations and offsets are `time.Duration`. Working precision is
milliseconds — parsers round to ms on ingest, formatters render with ms
precision (`HH:MM:SS.mmm`).

The PHP tool wraps time in a `TimeUnit` struct because PHP lacks a
duration primitive. Go doesn't need this. Where the PHP code says
`TimeUnit::MILLISECOND` or `TimeUnit::SECOND`, the Go port uses
`time.Duration` directly.

**Parsing** — accept these human-readable formats:
- `HH:MM:SS` (ffmpeg duration header)
- `HH:MM:SS.mmm` (mp4chaps chapter line)
- `MM:SS:FF` where FF is CD frames 0–74 (cue sheet `INDEX` line)
- Bare integer milliseconds (ffmetadata chapter timestamps)
- Bare integer seconds with optional decimal (ffmpeg silencedetect output)

**Formatting** — render as `HH:MM:SS.mmm` except for cue sheets, which
need `MM:SS:FF`.

## Tag

The metadata bag. One struct, all optional fields, zero value means
"unset."

```go
type Tag struct {
    // Identity
    Title           string
    Album           string
    Artist          string
    AlbumArtist     string
    Writer          string // composer / narrator

    // Descriptive
    Genre           string
    Year            int
    Description     string  // short, iTunes "desc" atom
    LongDescription string  // iTunes "ldes" atom
    Comment         string
    Copyright       string
    Publisher       string
    Encoder         string
    EncodedBy       string

    // Ordinals
    Track  int
    Tracks int
    Disk   int
    Disks  int

    // Series (pseudo-tags, stored in "grouping" on MP4)
    Series     string
    SeriesPart string

    // Sort keys
    SortTitle       string
    SortAlbum       string
    SortArtist      string
    SortAlbumArtist string
    SortWriter      string

    // MP3-specific (unused when writing to MP4)
    Performer string
    Language  string
    Lyrics    string

    // iTunes
    MediaType    ItunesMediaType
    Grouping     string
    PurchaseDate time.Time

    // Cover
    CoverPath string // path to image file; empty = no cover

    // Chapters (transient; not a tag atom)
    Chapters []Chapter

    // Arbitrary extras — ASIN, ISBN, Audible ID, Overdrive markers
    Extra map[string]string

    // Properties to strip on write
    Remove []string
}
```

Methods the Go port needs:

- `MergeMissing(other Tag)` — fill in zero-valued fields from `other`.
- `MergeOverwrite(other Tag)` — overwrite fields in-place when `other`
  has a non-zero value.

Both are used during tag-importer composition (see
[tag-importers](tag-importers/)).

### ItunesMediaType

```go
type ItunesMediaType int

const (
    MediaTypeMusic     ItunesMediaType = 1
    MediaTypeAudioBook ItunesMediaType = 2  // default for m4b
    MediaTypeMovie     ItunesMediaType = 9
    MediaTypeTVShow    ItunesMediaType = 10
    MediaTypeBooklet   ItunesMediaType = 11
    MediaTypeRingtone  ItunesMediaType = 14
    MediaTypePodcast   ItunesMediaType = 21
    MediaTypeITunesU   ItunesMediaType = 23

    // m4b-tool extensions, not standard iTunes values
    MediaTypeBook      ItunesMediaType = 256
    MediaTypeEbook     ItunesMediaType = 257
)
```

Accept case-insensitive constant names *and* integer values when
parsing user input for `--itunes-media-type` and equivalent flags.

## Chapter

A time-bounded section of an audio stream.

```go
type Chapter struct {
    Start        time.Duration
    Length       time.Duration
    Name         string
    Introduction string // optional snippet, used by --epub-append-introduction
}

func (c Chapter) End() time.Duration { return c.Start + c.Length }
```

Reserved names:
- `"Intro"` — synthetic first chapter from `--first-chapter-offset`.
- `"Outro"` — synthetic last chapter from `--last-chapter-offset`.

Treat these as string constants, not an enum — they're matched by
string equality in `RemoveDuplicateFollowUpChapters` and similar
passes.

## Silence

A detected silent region in an audio stream.

```go
type Silence struct {
    Start          time.Duration
    Length         time.Duration
    IsChapterStart bool // marked during chapter-silence alignment
}

func (s Silence) End() time.Duration      { return s.Start + s.Length }
func (s Silence) Midpoint() time.Duration { return s.Start + s.Length/2 }
```

Silence lists from ffmpeg are returned in start-time order and are
non-overlapping by construction.

## Chapter collections

The PHP tool wraps `[]Chapter` in a `ChapterCollection` that also
carries ISBN/ASIN/Audible-ID metadata. In Go those identifiers belong
on `Tag.Extra`, not bolted onto the chapter list. The Go port should
use a plain `[]Chapter` everywhere and keep identifiers on `Tag`.

`ChapterGroup` in the PHP tool is an implementation detail of the
length calculator; the Go port can inline it as a local slice-of-slice
in that algorithm rather than exporting a type.

## Invariants

Enforced by construction or checked at algorithm boundaries — not by
the types themselves:

1. **Chapters are sorted by `Start` ascending.** Any algorithm that
   rearranges them re-sorts before returning.
2. **Chapters do not overlap.** `chapters[i].End() <= chapters[i+1].Start`
   for all valid `i`.
3. **First chapter starts at `0`** unless an intro offset is configured.
4. **Last chapter ends at the file duration.** Probed from the audio
   file, not computed from chapter data.
5. **All time values are non-negative** after any shift/adjust
   operation. Shifters that would produce negative values must reject
   the operation (see [chapter-algorithms.md](chapter-algorithms.md#shifting)).
6. **Silences are non-overlapping and start-sorted.**

## Audio file probe result

What the ffmpeg/mp4info wrappers return after probing an input:

```go
type ProbeResult struct {
    Path       string
    Duration   time.Duration
    Codec      string   // "aac", "mp3", "alac", "flac", ...
    Format     string   // "mp4", "mp3", ...
    Channels   int
    SampleRate int
    Bitrate    int      // bits/sec; 0 if unknown
    Tag        Tag      // tags embedded in file
    HasCover   bool
    CoverType  string   // "jpeg", "png"; empty if none
}
```

The parser in `internal/ffmpeg` produces this from either
`-f ffmetadata` output combined with stream-info lines, or from
`mp4info` output for MP4 files as a cross-check.

## Config objects

One per command. Not part of the core model; shown here to anchor the
shape.

```go
type MergeConfig struct {
    Inputs      []string
    Output      string
    IncludeExt  []string
    BatchPattern []string
    Jobs        int
    Audio       AudioEncodingOptions
    Chapters    ChapterConfig
    TagOverrides Tag
    DryRun      bool
    // ... etc, one field per flag
}

type AudioEncodingOptions struct {
    Format     string // "m4b" | "mp4" | "mp3"
    Codec      string
    Bitrate    string // "128k"
    SampleRate int    // 44100
    Channels   int
    Quality    int    // VBR 0-100
    Profile    string // "aac_he", etc.
    NoConvert  bool
    TrimSilence bool
    AddSilence time.Duration
}

type ChapterConfig struct {
    MinLength    time.Duration
    MaxLengthDesired time.Duration
    MaxLengthHard    time.Duration
    Algorithm    string // "none" | "legacy" | "grouping"
    Source       string // "silence" | "epub" | "cue" | "embedded" | "fixed-length" | ...
    // etc.
}
```

Keep configs as plain structs with no methods. Validation runs once at
the top of each orchestrator; after that, fields are assumed sensible.

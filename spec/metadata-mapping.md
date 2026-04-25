# Metadata Mapping

How the Go port translates between three tag representations:

1. **`Tag` struct** — the in-memory model (see
   [data-model.md](data-model.md#tag)).
2. **FFMETADATA1** — key=value text format used by ffmpeg for I/O.
3. **MP4 atoms** — iTunes-style tags in the MP4 container, written via
   `mp4tags` and `mp4art`.

The PHP tool also has a fourth: the `tone` JSON format. That's a P2
concern and covered in [external-tools.md](external-tools.md#tone) —
ignore for v1.

## Core mapping table

| `Tag` field        | FFMETADATA key         | mp4tags flag | MP4 atom    | Notes                                    |
|--------------------|------------------------|--------------|-------------|------------------------------------------|
| `Title`            | `title`                | `-s`         | `©nam`      |                                          |
| `Album`            | `album`                | `-A`         | `©alb`      |                                          |
| `Artist`           | `artist`               | `-a`         | `©ART`      |                                          |
| `AlbumArtist`      | `album_artist`         | `-R`         | `aART`      |                                          |
| `Writer`           | `composer`             | `-w`         | `©wrt`      | Also used for audiobook narrator.        |
| `Genre`            | `genre`                | `-g`         | `©gen`      | String form; no genre-ID enum.           |
| `Year`             | `date`                 | `-y`         | `©day`      | Int → string on write.                   |
| `Track`            | `track`                | `-t`         | `trkn`      |                                          |
| `Tracks`           | —                      | `-T`         | `trkn`      | Packed with `Track` into one atom.       |
| `Disk`             | `disc`                 | `-d`*        | `disk`      |                                          |
| `Disks`            | —                      | `-D`*        | `disk`      | Same atom as `Disk`.                     |
| `Comment`          | `comment`              | `-c`         | `©cmt`      |                                          |
| `Description`      | `description`          | `-m`         | `desc`      | Short description, iTunes.               |
| `LongDescription`  | `longdesc`             | `-l`         | `ldes`      | Extended synopsis.                       |
| `Copyright`        | `copyright`            | `-C`         | `cprt`      |                                          |
| `Publisher`        | `publisher`            | —            | `©pub`      | Via ffmetadata only (`mp4tags` has none).|
| `Encoder`          | `encoder`              | `-E`         | `©too`      |                                          |
| `EncodedBy`        | `encoded_by`           | `-e`         | —           | Not round-tripped via MP4.               |
| `Grouping`         | `grouping`             | `-G`         | `©grp`      |                                          |
| `SortTitle`        | `title-sort`           | `-f`         | `sonm`      | mp4tags extra feature — see below.       |
| `SortAlbum`        | `album-sort`           | `-k`         | `soal`      | Extra feature.                           |
| `SortArtist`       | `artist-sort`          | `-F`         | `soar`      | Extra feature.                           |
| `SortAlbumArtist`  | `TSO2`                 | —            | `soaa`      | ffmetadata via ID3 frame name.           |
| `SortWriter`       | `TSOC`                 | —            | `soco`      | ffmetadata via ID3 frame name.           |
| `MediaType`        | —                      | `-i`         | `stik`      | iTunes media-type enum, int.             |
| `PurchaseDate`     | —                      | `-U`         | `purd`      | Extra feature; time.Time formatted ISO.  |
| `Lyrics`           | `lyrics`               | `-L`         | `©lyr`      |                                          |
| `Performer`        | `performer`            | —            | —           | MP3 only (TPE3).                         |
| `Language`         | `language`             | —            | —           | MP3 only (TLAN).                         |
| `Series`           | `grouping`             | `-G`         | `©grp`      | Pseudo-tag; stored IN grouping.          |
| `SeriesPart`       | `grouping` (appended)  | `-G`         | `©grp`      | Pseudo-tag; formatted `Series, Part N`.  |
| `CoverPath`        | *separate input*       | *`mp4art`*   | `covr`      | See [Cover art](#cover-art).             |
| `Chapters`         | `[CHAPTER]` blocks     | *`mp4chaps`* | chapter trk | See [Chapters](#chapters).               |
| `Extra[k]`         | *custom freeform keys* | —            | `----:…`    | Reverse-DNS atoms via tone / ffmetadata. |

\* `mp4tags` flag names for disk vary between mp4v2 versions; use the
long form if the short is missing.

## mp4tags "extra features"

Older mp4v2 builds lack the sort-name and purchase-date flags. Detect
at startup by running `mp4tags -help` and looking for:

- `-U, -purchasedate` → sandreas fork pattern
- `-purchasedate` → new upstream

If absent, silently drop those fields from write operations. Don't
error — the tool is still usable for the common case.

## iTunes media type

```
1    MediaTypeMusic
2    MediaTypeAudioBook   ← default for m4b outputs
9    MediaTypeMovie
10   MediaTypeTVShow
11   MediaTypeBooklet
14   MediaTypeRingtone
21   MediaTypePodcast
23   MediaTypeITunesU
256  MediaTypeBook        (m4b-tool extension, non-standard)
257  MediaTypeEbook       (m4b-tool extension, non-standard)
```

Accept case-insensitive constant names or integer values from user
flags. Write as an integer to the `stik` atom.

## Series pseudo-tag

iTunes doesn't have a first-class "series" atom. m4b-tool packs series
into `grouping` by convention, with a comma-separator format:

```
<Series>, Part <SeriesPart>
```

On read: parse `grouping`; if it matches `^(.*?), Part (\d+)$`, split
into `Series` and `SeriesPart`. Otherwise treat it as a plain
`Grouping` value.

On write: if `Series` is set, synthesize the grouping string before
handing to `mp4tags`.

## Cover art

**Read**: cover is not in FFMETADATA. Extract via:

```
ffmpeg -hide_banner -i FILE -an -vcodec copy -f image2 cover.jpg
```

or via `mp4art --extract` (see [external-tools.md](external-tools.md#mp4art)).

**Write**: `mp4tags` cannot set cover. Use `mp4art --add <image> FILE`
after all other tags are written. The Go port should:

1. Write tags with `mp4tags`.
2. Write chapters with `mp4chaps -i`.
3. Write cover with `mp4art --add` (removing existing first if
   replacing: `mp4art --remove --art-any`).

**Format**: JPEG or PNG. The atom (`covr`) stores the raw image bytes;
no conversion needed.

## Chapters

FFMETADATA uses `[CHAPTER]` blocks:

```
[CHAPTER]
TIMEBASE=1/1000
START=0
END=305000
title=Chapter 1
```

`TIMEBASE=1/1000` means START/END are in milliseconds. Other
timebases are possible but the Go port should always write `1/1000`
and tolerate other values on read by scaling.

MP4 chapters aren't written via ffmetadata in practice. They go
through `mp4chaps -i` with a sidecar `.chapters.txt` file — see
[external-tools.md](external-tools.md#mp4chaps).

## Extra fields

Free-form metadata that doesn't fit the schema (Audible ASIN, ISBN,
Google ID, Overdrive media markers). Stored in `Tag.Extra` as a flat
string map.

Three sinks:
- **FFMETADATA**: written as plain `key=value` lines. Some ffmpeg
  versions will promote them to ID3 `TXXX` frames on MP3 output.
- **MP4 `----:` atoms**: reverse-DNS freeform atoms. The PHP tool
  delegates this to `tone` (see
  [external-tools.md](external-tools.md#tone)). mp4tags/mp4art can't
  write these. For v1 the Go port should read them (they appear as
  keys in ffmetadata output) but only write them when tone is
  available. Document the limitation; don't silently lose data.
- **mp4tags `Extra` map**: mp4tags itself has no freeform support.

## Charset handling

Input tags from file-based sources may be non-UTF-8. The PHP tool
supports `--platform-charset=Windows-1252` to force a codepage. The Go
port should:

- Assume UTF-8 by default.
- If `--platform-charset` is set, decode all file-system paths and
  sidecar file contents (chapters.txt, FFMETADATA sidecars) using the
  specified encoding via `golang.org/x/text/encoding`.
- Normalize to UTF-8 internally; emit UTF-8 for all outputs.

## Multiline values

FFMETADATA supports multi-line values using trailing backslash:

```
description=line 1\
line 2\
line 3
```

Other escapes: `\=`, `\;`, `\#`, `\\`, `\n`. The parser must
unescape on read and the writer must escape on write. Most audiobook
`description` fields contain newlines — this is not optional.

## Write order (MP4 outputs)

The tagging pipeline for an `.m4b` output:

1. ffmpeg writes the stream with embedded FFMETADATA (titles, dates,
   series/grouping, description). Cover and chapters are NOT passed
   to ffmpeg; it handles them inconsistently across codecs.
2. `mp4chaps -i` writes the chapter atoms from a sidecar.
3. `mp4art --remove --art-any && mp4art --add` writes the cover.
4. `mp4tags` writes any atoms ffmpeg didn't handle (sort names,
   purchase date, iTunes media type). Many of these overlap with what
   ffmpeg wrote; mp4tags is the authoritative source — last writer
   wins.

Keep this order; ffmpeg-first, then mp4v2 tools in sequence.

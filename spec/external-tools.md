# External Tool Contracts

The Go port shells out to the same seven tools the PHP version does.
Each tool gets its own Go package that builds argv, invokes the binary,
and parses output. This document specifies every invocation the port
must be able to produce, the output contracts to parse, and fallback
behavior.

| Tool      | Role                                      | v1 required? |
|-----------|-------------------------------------------|--------------|
| ffmpeg    | probing, transcoding, silence detect, tag/chapter I/O via ffmetadata | yes |
| mp4chaps  | write chapter atoms to MP4                | yes          |
| mp4art    | embed/extract/remove cover art            | yes          |
| mp4tags   | write iTunes-style tags                   | yes          |
| mp4info   | duration probe                            | yes (or derive via ffmpeg) |
| fdkaac    | higher-quality AAC encoder                | optional (fallback to ffmpeg `aac`/`libfdk_aac`) |
| tone      | alternate tagger + duration probe         | optional     |

## Reference versions

Pinned by the PHP tool's Dockerfile; the Go port should support these
or newer, and refuse older only if output format differs.

| Tool   | Pinned version | Notes                               |
|--------|---------------|--------------------------------------|
| ffmpeg | 5.0.1         | Needs `silencedetect`, `-f ffmetadata`, `-f concat`, `-movflags +faststart`. |
| mp4v2  | 2.1.0         | Supplies `mp4chaps`, `mp4art`, `mp4tags`, `mp4info`. Upstream is unmaintained and has been dropped from Debian/Ubuntu repos; the maintained continuation is [`enzo1982/mp4v2`](https://github.com/enzo1982/mp4v2), with `v2.1.0` as its latest tagged release. Build from source with `cmake`; the Docker image in Milestone 9 should clone and build it rather than relying on a distro package. |
| fdkaac | 2.0.1         | Optional.                            |
| tone   | 0.2.5         | Optional; min ≥ 0.0.9 checked by PHP. |

## Process runner

The Go port should centralize subprocess handling in one helper
(`internal/exec` or similar) that:

- Constructs `exec.Cmd` from argv slices (no shell unless piping).
- Streams stderr into a line-oriented channel when parsing is
  incremental (silence detection), buffers otherwise.
- Times out by default at some user-configurable ceiling; `0` means no
  timeout (needed for long transcodes).
- Returns a non-nil error including merged stdout+stderr on non-zero
  exit.
- Supports a terminate callback for temp-file cleanup (the PHP tool
  uses this pattern to clean up ffmetadata sidecars).

For the single piped case (fdkaac reads ffmpeg CAF from stdin), use
`cmd.StdoutPipe()` / `cmd.StdinPipe()` and `exec.Cmd.Start()`.

---

## ffmpeg (`internal/ffmpeg`)

The workhorse. Used for everything except MP4-specific tag/chapter/cover
writing.

### Operations

1. **Probe duration** (fast): parse `Duration:` line from stderr of a
   no-op invocation.
2. **Probe duration** (exact): re-encode to `/dev/null` and read final
   `time=...` stat.
3. **Silence detection**: `silencedetect` audio filter; parse stderr.
4. **Audio checksum**: `-f crc` for verifying byte-identical streams.
5. **FFMETADATA read**: `-f ffmetadata -` to stdout.
6. **FFMETADATA write**: pass metadata as a second input mapped via
   `-map_metadata`.
7. **Cover extract**: `-an -vcodec copy`.
8. **Concat merge**: `concat` demuxer reading a list file.
9. **Silence synthesis**: `anullsrc` filter with target format.
10. **Partial extraction**: `-ss <start> -t <duration>` for split.

### Canonical invocations

**Exact duration probe**
```
ffmpeg -hide_banner -i INPUT -loglevel panic -stats -f null -
```
Parse last stderr line matching `time=HH:MM:SS.ms`.

**Silence detection**
```
ffmpeg -hide_banner -i INPUT -af silencedetect=noise=-30dB:d=1.0 -f null -
```
Parse stderr lines:
```
^.*silence_end:\s+([0-9]+\.[0-9]+)\s+\|\s+silence_duration:\s+([0-9]+\.[0-9]+)$
```
Two capture groups: end time (s), duration (s). Start = end − duration.
Also capture header `Duration: HH:MM:SS.ms` for total duration context.

**FFMETADATA read**
```
ffmpeg -hide_banner -i INPUT -f ffmetadata -
```
Stdout is valid FFMETADATA1 text (see grammar below).

**Transcode with embedded metadata + chapters**
```
ffmpeg -hide_banner \
    -i INPUT -i METADATA.txt \
    -map_metadata 1 -max_muxing_queue_size 9999 \
    -af silenceremove=start_periods=1:start_threshold=-30dB:stop_periods=2 \
    -strict experimental -movflags +faststart -vn \
    -q:a 1.5 -ar 22050 -ac 2 -acodec libfdk_aac -f m4b \
    OUTPUT.m4b
```
Threading: inject `-threads <N>` when set. Extra user flags
(`--ffmpeg-param`) append just before the output filename.

**Concat merge**
```
ffmpeg -hide_banner -f concat -safe 0 -vn -i CONCAT.txt \
    -max_muxing_queue_size 9999 -c copy -f m4b OUTPUT.m4b
```
`CONCAT.txt` format, one line per file, single-quoted with `'` escaped as `'\''`:
```
file '/abs/path/to/part-01.m4a'
file '/abs/path/to/part-02.m4a'
```

**Silence generation** (for `--add-silence`)
```
ffmpeg -hide_banner -f lavfi -i anullsrc=channel_layout=stereo:sample_rate=44100 \
    -t 0.5 -acodec aac -f caf silence.caf
```

**Partial extraction** (split)
```
ffmpeg -hide_banner -i INPUT -ss <start.s> -t <duration.s> \
    -acodec copy -vn OUTPUT.m4a
```

### FFMETADATA1 grammar (produced and consumed)

```
;FFMETADATA1
title=Book Title
artist=Narrator
album=Series Name
; multi-line values may end with backslash to continue
description=line 1\
line 2
[CHAPTER]
TIMEBASE=1/1000
START=0
END=305000
title=Chapter 1
[CHAPTER]
TIMEBASE=1/1000
START=305000
END=610000
title=Chapter 2
```

Escape sequences: `\=` `\;` `\#` `\\` `\n`. Chapters use
`TIMEBASE=1/N` (almost always `1/1000`) with integer START/END.

### Fallbacks

- If `libfdk_aac` is absent from `ffmpeg -codecs` output, fall back to
  `aac`.
- If silence detection produces no output (detector saturated, bad
  input), fall back to treating the whole file as one chapter.
- If duration probe regex doesn't match, try the other probe variant.

### Go package shape (sketch)

```go
type Client struct {
    Bin      string   // path to ffmpeg, default "ffmpeg"
    Threads  int      // 0 = unset (auto)
    ExtraArgs []string // user --ffmpeg-param
}

func (c *Client) ProbeDuration(ctx, path string) (time.Duration, error)
func (c *Client) DetectSilence(ctx, path string, minLen, maxLen time.Duration) ([]Silence, error)
func (c *Client) ReadFFMetadata(ctx, path string) (*Tag, error)
func (c *Client) Transcode(ctx, in, out string, opts TranscodeOptions) error
func (c *Client) Concat(ctx, list []string, out string, opts ConcatOptions) error
func (c *Client) ExtractSpan(ctx, in, out string, start, dur time.Duration, opts ExtractOptions) error
func (c *Client) CreateSilence(ctx, out string, dur time.Duration, opts SilenceOptions) error
```

---

## mp4chaps (`internal/mp4v2`)

Writes chapter atoms to an MP4 file using a sidecar text file.

### Operations

- Import chapters: `mp4chaps -i [-N] FILE.m4b` — reads `FILE.chapters.txt`.
- Remove chapters: `mp4chaps -r FILE.m4b`.

`-N` requests Nero-format chapters (the default is QuickTime). The Go
port should always write QuickTime unless the user asks for Nero.

### Chapters text format (produced and consumed)

```
## total-duration: 12:34:56.789
00:00:00.000 Chapter 1
00:05:30.123 Chapter 2
01:23:45.678 Chapter 3
```

- Optional header comment starting `## total-duration:`.
- Each chapter: `HH:MM:SS.mmm <title>` — space-separated, title is the
  rest of the line verbatim.

The sidecar must be named `<basename>.chapters.txt` alongside the audio
file. The Go port should create it in the temp dir if possible and only
rename into place at the last moment, then delete after success (unless
`--no-cleanup`).

### Fallbacks

None. Chapter writing is mp4v2-specific. If `mp4chaps` is absent:
- For `merge` and `chapters`: hard fail at startup with
  "mp4chaps not found; install mp4v2".
- For `split`: only needed when outputs are m4b with embedded chapters;
  warn and skip chapter embedding on splits.

---

## mp4art (`internal/mp4v2`)

Cover art management. Three modes:

```
mp4art --add COVER.jpg FILE.m4b
mp4art --remove --art-any FILE.m4b
mp4art --art-index 0 --extract FILE.m4b
mp4art --list FILE.m4b
```

### Output parsing

- `--list`: stdout contains a table with columns `IDX BYTES CRC32 TYPE
  FILE`. Count rows after the header separator to get cover count.
- `--extract`: writes sidecar `FILE.art[<index>].<ext>` where `<ext>` is
  `jpg` or `png` depending on embedded type. The wrapper must discover
  the output path rather than assume an extension.

---

## mp4tags (`internal/mp4v2`)

Writes tag atoms. Short-flag oriented.

### Property → flag mapping

| Property         | Short flag | Long flag (newer builds) |
|------------------|------------|--------------------------|
| artist           | `-a`       |                          |
| title            | `-s`       |                          |
| album            | `-A`       |                          |
| track            | `-t`       |                          |
| tracks (total)   | `-T`       |                          |
| genre            | `-g`       |                          |
| writer / composer| `-w`       |                          |
| description      | `-m`       |                          |
| long description | `-l`       |                          |
| album artist     | `-R`       |                          |
| year             | `-y`       |                          |
| comment          | `-c`       |                          |
| copyright        | `-C`       |                          |
| encoded-by       | `-e`       |                          |
| encoder          | `-E`       |                          |
| lyrics           | `-L`       |                          |
| itunes media type| `-i`       |                          |
| grouping         | `-G`       |                          |
| sort title       | `-f`       | `--sortname`             |
| sort album       | `-k`       | `--sortalbum`            |
| sort artist      | `-F`       | `--sortartist`           |
| purchase date    | `-U`       | `--purchasedate`         |

Example call:
```
mp4tags -a "Author" -s "Title" -A "Book" -t 1 -T 10 -g "Audiobook" FILE.m4b
```

Remove tags:
```
mp4tags -r "a,s,A,t" FILE.m4b
```

### Extra-feature detection

The lower four properties (sort names, purchase date) exist only in
patched builds. Detect at startup by running `mp4tags -help` and
grepping for `-U, -purchasedate` (sandreas fork) or `-purchasedate`
(new upstream). If absent, silently skip those properties in writes
and removes.

---

## mp4info (`internal/mp4v2`)

Single operation: duration probe.

```
mp4info FILE.m4b
```

Parse stdout for one of:
```
<NN>       audio   MPEG-4 AAC LC, 0.684 secs, 32 kbps, 44100 Hz
duration:    19012 ms
```

Regex 1: `([0-9]+\.[0-9]{3})\s+secs` — seconds with three decimals.
Regex 2: `duration:\s+([0-9]+)\s+ms` — integer ms.

If neither matches, fail with the raw stdout in the error message. The
Go port can skip mp4info entirely if ffmpeg probing is already wired up
and produces trustworthy results; keep it as a cross-check for tests.

---

## fdkaac (`internal/fdkaac`)

Higher-quality AAC encoder. Piped from ffmpeg's CAF output:

```
ffmpeg -i INPUT -vn -af silenceremove=... -ac 2 -ar 44100 -f caf - | \
    fdkaac --raw-channels 2 --raw-rate 44100 -p 2 -b 256k -m 3 -o OUT.m4b -
```

Profile table (PHP `Fdkaac::SUPPORTED_PROFILE_MAPPING`):

| Flag value | Profile       |
|------------|---------------|
| 2          | AAC LC (default) |
| 5          | HE-AAC        |
| 29         | HE-AAC v2     |
| 23         | AAC LD        |
| 39         | AAC ELD       |

VBR mode `-m`: 0 = CBR, 1–5 = VBR quality (higher = better).

### Detection

Running `fdkaac` with no args prints `Usage: fdkaac`. Check for that at
startup; if absent, set `supportsConversion=false` and the transcode
pipeline uses ffmpeg's `libfdk_aac` or `aac` instead.

---

## tone (`internal/tone`)

Optional. Two operations v1 might use:

**Duration probe** (often more accurate than mp4info for weird files):
```
tone dump FILE.m4b --format json --query $.audio.duration
```
Stdout is an integer (milliseconds). Empty or non-positive → treat as
unsupported, fall back to ffmpeg/mp4info.

**Tag write** with JSON sidecar:
```
tone tag --meta-tone-json-file META.json --prepend-movement-to-description FILE.m4b
```

`META.json` shape:
```json
{
  "meta": {
    "title": "...",
    "album": "...",
    "chapters": [
      {"start": 0, "length": 305000, "title": "Chapter 1"}
    ],
    "additionalFields": {
      "----:com.pilabor.tone:AUDIBLE_ASIN": "B00..."
    },
    "lyrics": { "language": "eng", "unsynchronized": "..." }
  }
}
```

Tone's field names differ from the PHP Tag model — see
`PROPERTY_PARAMETER_MAPPING` in `src/library/Executables/Tone.php:82` for
the 22-entry translation table. v1 can ignore tone entirely and do all
tagging via `mp4tags` + `mp4chaps` + ffmetadata; defer tone integration
to a later milestone.

### Detection

`tone -v` prints a version string. Require ≥ 0.0.9 (PHP's check). Mark
globally disabled if absent.

---

## Parsers the wrappers need

These sit in `internal/ffmpeg/parse.go` (or equivalent) and are pure
functions — no subprocess, no I/O.

### SilenceParser

Input: ffmpeg stderr text.
Output: `[]Silence{Start, Duration}`.
Grammar: one regex per line as shown under ffmpeg above. Tolerate
arbitrary leading text (ffmpeg prefixes with `[silencedetect @ ...]`).

### FFMETADATA parser

Input: FFMETADATA1 text.
Output: `*Tag` with scalar fields, chapter list, stream info (codec,
channels, duration, cover type) when present.

Line-oriented; state machine with two states: top-level metadata and
`[CHAPTER]` block. Reset to top-level at a new `[CHAPTER]` marker or
EOF. Stream info lines are optional — some callers pipe raw ffmetadata
without them.

### mp4chaps parser

Input: `.chapters.txt` text.
Output: `[]Chapter{Start, Title}` — end time is derived from the next
chapter's start or the file duration.
Grammar: skip `##` comment lines; each data line is
`HH:MM:SS.mmm SP title-rest-of-line`.

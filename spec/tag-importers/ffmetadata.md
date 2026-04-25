# ffmetadata

**Priority**: P0
**PHP**: `src/library/Audio/Tag/Ffmetadata.php`, parser at
`src/library/Parser/FfmetaDataParser.php`

## Role

Load tag metadata and chapter list from a sidecar FFMETADATA1 text
file. This is the primary interchange format between m4b-tool and
ffmpeg, and the one most likely to be hand-edited by users.

## Inputs

- `<workdir>/ffmetadata.txt` — default filename.
- Any file whose contents begin with `;FFMETADATA1` — caller may pass
  an explicit path.
- Stream from `ffmpeg -f ffmetadata -` — same grammar, no sidecar.

## Output fields

Most scalar `Tag` fields and the full chapter list. See
[metadata-mapping.md](../metadata-mapping.md#core-mapping-table) for
the key→field table. Merge policy: `MergeMissing`.

## Grammar (consumer + producer)

```
;FFMETADATA1
key=value
key=multi-line value line 1\
line 2\
line 3

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

Rules:
- First non-empty line must be `;FFMETADATA1` (the leading `;` is a
  comment marker; this specific comment is the magic header).
- Lines starting with `;` or `#` are comments, ignored.
- `key=value` outside any `[CHAPTER]` block is global metadata.
- A backslash at end of line continues the value on the next line.
- Escape sequences in values: `\=` `\;` `\#` `\\` `\n`.
- `[CHAPTER]` section must contain `TIMEBASE`, `START`, `END`, and
  optionally `title`. Unknown keys inside a chapter block are ignored.
- `TIMEBASE=1/N` — START and END are in `1/N`-second units. Virtually
  always `1/1000` (ms). Scale to `time.Duration` on parse.
- Trailing whitespace on lines is preserved; leading whitespace is
  not trimmed.

## Go sketch

```go
func (i *Ffmetadata) Improve(ctx context.Context, t Tag, workDir string) (Tag, error) {
    path := filepath.Join(workDir, "ffmetadata.txt")
    if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
        return t, nil // no-op if absent
    }
    data, err := os.ReadFile(path)
    if err != nil { return t, err }

    parsed, err := ffmetadata.Parse(data)
    if err != nil { return t, err }

    out := t
    out.MergeMissing(parsed)
    return out, nil
}
```

## Edge cases

- Missing `;FFMETADATA1` header → refuse to parse, return
  `ErrUnrecognizedFormat`. Don't guess.
- `TIMEBASE` missing → assume `1/1000`.
- Chapter blocks out of time order → accept, sort before returning.
- Values that look like binary → the cover is *not* in ffmetadata;
  some malformed tools put base64-encoded images in the `cover` key.
  Reject values longer than 64 KB with a warning; they're not useful
  anyway.

## Writer

The port also emits FFMETADATA when piping metadata into `ffmpeg -i
metadata.txt ... -map_metadata 1`. Output format is the same; produce
`TIMEBASE=1/1000` always, escape values symmetrically with the parser.

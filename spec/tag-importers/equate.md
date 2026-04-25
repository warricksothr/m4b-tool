# equate

**Priority**: P0
**PHP**: `src/library/Audio/Tag/Equate.php`

## Role

Propagate a value across multiple tag fields. A workaround for the
common case where the user wants, e.g., the author's name to appear
as `artist`, `albumartist`, and `sortartist` all at once.

Driven by one or more `--equate <csv>` flags.

## Inputs

One or more equate specifications. Each is a comma-separated list of
`Tag` field names (case-insensitive, matched against struct-field
names). The first field named is the **source**; the remaining are
**targets**.

Examples:
- `--equate=artist,albumartist,sortartist` → copy `Artist` value
  into `AlbumArtist` and `SortArtist`.
- `--equate=album,sortalbum` → copy `Album` into `SortAlbum`.

## Output fields

Whatever fields are named as targets. Merge policy: equate
unconditionally overwrites the target fields, even if they were
already populated. This is what the flag means — "force these equal."

## Algorithm

```go
func (e *Equate) Improve(ctx context.Context, t Tag, _ string) (Tag, error) {
    for _, spec := range e.Specs {
        parts := strings.Split(spec, ",")
        if len(parts) < 2 { continue }
        srcName := strings.TrimSpace(parts[0])
        srcVal, ok := tagFieldValue(t, srcName)
        if !ok || srcVal == "" { continue }
        for _, tgt := range parts[1:] {
            setTagField(&t, strings.TrimSpace(tgt), srcVal)
        }
    }
    return t, nil
}
```

Needs a helper (reflection or a switch) to map string field names to
`Tag` struct fields. The set of valid names mirrors
[metadata-mapping.md](../metadata-mapping.md#core-mapping-table) —
rejected names should produce a WARN, not an error, so a typo doesn't
kill the run.

## Edge cases

- Source field empty → skip silently (no value to propagate).
- Target field name unknown → log WARN, skip target.
- Only one field in the CSV → ignore (nothing to propagate).
- Numeric fields like `year`, `track`: equate still works; the
  helper stringifies on read and parses on write.

## Ordering

Runs *after* all other importers and *after* file tags are merged
in, but *before* other CLI overrides (`--name`, `--album`, etc.)
apply. This way the user's explicit `--album="Foo"` still beats
`--equate=title,album`.

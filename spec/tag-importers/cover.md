# cover

**Priority**: P0
**PHP**: `src/library/Audio/Tag/Cover.php`

## Role

Find a cover image in the working directory and set `Tag.CoverPath`.
Does not embed the image — that's done later by `mp4art` during
tagging.

## Inputs

Search order in `<workdir>`:
1. `cover.jpg`
2. `cover.jpeg`
3. `cover.png`

First match wins. If user passed `--cover <path>`, skip this importer
entirely (CLI override takes precedence).

If `--skip-cover` is set, skip this importer.

If `--skip-cover-if-exists` is set: skip this importer if the source
audio file already has an embedded cover; otherwise run normally.

## Output fields

- `Tag.CoverPath` — absolute path to the discovered image.

Merge policy: `MergeMissing` (don't overwrite a path already set by
CLI or prior importer).

## Edge cases

- Cover file is zero bytes → ignore it, don't set CoverPath. Log at
  WARN.
- Cover file is neither JPEG nor PNG (despite extension) → let mp4art
  fail loudly during tagging rather than pre-validating here.
- Symlinks → follow; pass the resolved path.

## Write side

At tag-writing time:
1. If target file has a cover and `Tag.CoverPath` differs, run
   `mp4art --remove --art-any <file>` first.
2. Run `mp4art --add <cover> <file>`.

See [external-tools.md](../external-tools.md#mp4art).

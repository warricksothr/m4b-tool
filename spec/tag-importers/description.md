# description

**Priority**: P0
**PHP**: `src/library/Audio/Tag/Description.php`

## Role

Load long-form book description from a plaintext sidecar file. Useful
because FFMETADATA's line-continuation syntax is painful to hand-edit
for multi-paragraph descriptions.

## Inputs

- `<workdir>/description.txt`

## Output fields

- `Tag.Description` — first line of the file (short description).
- `Tag.LongDescription` — entire file contents.

Merge policy: `MergeMissing`.

## Grammar

No grammar. Raw UTF-8 text. The file is read verbatim except:

- Trailing whitespace on the file as a whole is trimmed.
- `\r\n` → `\n` on read.
- The "short description" is the text before the first blank line
  (two consecutive `\n`), or the entire file if no blank line exists.

## Go sketch

```go
func (i *Description) Improve(ctx context.Context, t Tag, workDir string) (Tag, error) {
    data, err := os.ReadFile(filepath.Join(workDir, "description.txt"))
    if errors.Is(err, os.ErrNotExist) { return t, nil }
    if err != nil { return t, err }

    body := strings.ReplaceAll(string(data), "\r\n", "\n")
    body = strings.TrimRight(body, " \n\t")

    short, _, _ := strings.Cut(body, "\n\n")
    if t.Description == "" { t.Description = short }
    if t.LongDescription == "" { t.LongDescription = body }
    return t, nil
}
```

## Edge cases

- Empty file → no-op.
- File is only whitespace → no-op.
- Single-paragraph file → short and long are identical; that's fine.

---
name: Bug report
about: Report a bug or unexpected behavior
title: ''
labels: bug
assignees: ''
---

## What did you run?

The exact command, including every flag, in a code fence:

```
m4b-tool ...
```

## What did you expect to happen?

<!-- Replace with what you wanted. -->

## What actually happened?

<!-- Full stderr / stdout output. Wrap long output in a code fence.
     If the error mentions ffmpeg or one of the mp4v2 utilities, the
     lines from those tools matter — keep them in. -->

## Environment

- **m4b-tool version**: (output of `m4b-tool version`)
- **Install method**: container (`ghcr.io/warricksothr/m4b-tool`) / release archive / `go install` / built from source
- **Host OS** and architecture:
- **Output of `m4b-tool doctor`**:

```
(paste output here)
```

## Sample input

If the bug is input-specific, describe the file's chapter structure
and provenance — e.g. "Audible AAX → libfdk_aac re-encoded m4b, 37
chapters, 16h54m, mono 125 kb/s." Please don't attach copyrighted
audio; a small synthetic reproduction or a metadata-only `ffprobe`
dump is more useful anyway.

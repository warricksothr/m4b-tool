# m4b-tool

A Go CLI for managing audiobook `.m4b` files — chapters, merging,
splitting, and tag handling.

An independent Go reimplementation of the
[PHP m4b-tool](https://github.com/sandreas/m4b-tool), focused on the
chapter, merge, and split workflows audiobook listeners reach for.
The PHP project is the inspiration and remains a useful reference;
this exists as an alternative for users who prefer a single static
Go binary and a smaller runtime footprint.

> **Status:** under active development. Functional equivalence with
> the PHP tool's `merge`, `split`, and `chapters` commands is the bar;
> verification is by observable properties (same chapters at the same
> offsets, same required tags, indistinguishable audio), not byte
> identity. Online metadata lookup, the `meta`/`extra` subcommands,
> and any GUI/daemon are out of scope.

## Install

### Container (recommended — bundles ffmpeg + mp4v2)

```sh
podman run --rm -v "$PWD:/work" ghcr.io/warricksothr/m4b-tool:latest doctor
```

### From a release archive

Download the archive matching your OS/arch from the
[releases page](https://github.com/warricksothr/m4b-tool/releases),
extract, and place `m4b-tool` on your `PATH`. You'll also need:

- [`ffmpeg`](https://ffmpeg.org/) ≥ 5.0
- the [`enzo1982/mp4v2`](https://github.com/enzo1982/mp4v2)
  utilities ≥ 2.1 — `mp4chaps`, `mp4tags`, `mp4art`, `mp4info`

Run `m4b-tool doctor` after install to verify the toolchain.

### From source

```sh
go install github.com/warricksothr/m4b-tool/cmd/m4b-tool@latest
```

(Same external-tool requirements as above.)

## Commands

```sh
m4b-tool merge    <inputs...>  -o output.m4b   # combine into a tagged .m4b
m4b-tool split    <input.m4b>  -o out-dir/     # extract chapters to files
m4b-tool chapters <input.m4b>                  # read/write/shift markers
m4b-tool doctor                                # check external tool versions
```

Each subcommand has a `--help`. Design docs live under
[`spec/`](spec/) — start with [`spec/README.md`](spec/README.md).

## Recipes

The examples below assume `m4b-tool` is on your `PATH`. If you run
the container instead, set up a shell alias once and every example
below works as-written:

```sh
alias m4b-tool='podman run --rm --userns=keep-id -v "$PWD:/work" ghcr.io/warricksothr/m4b-tool:latest'
```

`--userns=keep-id` keeps your host UID inside the container so files
written through the bind mount are owned by you (rootless podman
otherwise maps the container's user to a subuid that can't write to
your home directory). Rootful Docker doesn't need it.

With the alias, paths must live inside `$PWD` (or any other directory
you bind-mount); the container sees them under `/work`.

### Split an audiobook into per-chapter MP3s

```sh
m4b-tool split --audio-format mp3 /work/book.m4b
```

`split` picks the codec automatically from `--audio-format`
(`libmp3lame` for `mp3`, `flac` for `flac`, etc.) and probes the
source bitrate to pick a sensible default (capped at 192 kbps so a
lossless source doesn't produce an absurd MP3). Override either with
`--audio-codec` and `--audio-bitrate` if you want something specific —
e.g. add `--audio-bitrate 96k --audio-channels 1` for spoken-word
narration where mono at a low bitrate is plenty.

Per-chapter extraction runs in parallel by default — `--jobs` defaults
to `min(8, NumCPU-1)`, further capped by available memory. Pass
`--jobs 1` to force serial, or `--no-memory-cap` if your platform
under-reports `MemAvailable` (some WSL2 setups).

### Preview chapter names before splitting

Some `.m4b` files store chapters with bare numeric titles ("001",
"002", ...) instead of real chapter names — `split` will still cut
the audio correctly, but the output filenames look like `001-001.mp3`.
Pipe `chapters export` to stdout to skim the resolved chapter list
first:

```sh
m4b-tool chapters export /work/book.m4b - | head
```

If the titles look right (e.g. `00:00:00.000 Prologue`), just run
`split`. If they're bare indices, you have three options:

1. **Add a label, no editing.** `--chapter-prefix` prepends a literal
   string to every chapter title; `--strip-title` first trims the
   leading zeros:

   ```sh
   m4b-tool split --audio-format mp3 \
     --strip-title --chapter-prefix "Chapter " \
     /work/book.m4b
   # -> 001-Chapter 1.mp3, 002-Chapter 2.mp3, ...
   ```

2. **Custom names via sidecar.** Dump the chapters as a `.chapters.txt`
   sidecar, edit in your editor of choice, re-run `split`:

   ```sh
   m4b-tool chapters export /work/book.m4b   # -> book.chapters.txt
   $EDITOR /work/book.chapters.txt           # replace "001" with real titles
   m4b-tool split --audio-format mp3 /work/book.m4b   # sidecar auto-detected
   ```

   Sidecar format is `HH:MM:SS.mmm <title>` per line; lines starting
   with `##` are comments. Pass `--use-existing-chapters-file` if you
   want the sidecar to win over a file that also has embedded
   chapters.

3. **Bake-and-tweak.** Combine the above: `chapters export` accepts
   the same `--chapter-prefix`/`--strip-title`/`--reindex-chapters`
   flags as `split`, so you can bake the transformed names into the
   sidecar, optionally tweak by hand, then run a flag-free `split`.

   > Don't double-apply: if you bake `--chapter-prefix "Chapter "`
   > into the sidecar **and** pass it to `split`, you'll get
   > "Chapter Chapter 1". Pick one side.

`split` warns automatically when ≥80% of resolved chapter names look
like bare indices. Suppress with `--quiet`.

### Split by silence (when the file has no chapters)

```sh
m4b-tool split --by-silence --silence-min-length 1500 \
  --audio-format mp3 /work/long-recording.mp3
```

`--silence-min-length` (ms) is the minimum gap that counts as a
chapter break. 1500 ms is a sane default for audiobook content;
shorten it for music with deliberate pauses.

### Merge a directory of audio files into a single `.m4b`

```sh
m4b-tool merge -o /work/book.m4b /work/source-chapters/
```

Inputs are read in filename order; each file becomes one chapter.
Embedded tags carry through unless overridden via flags
(`--name`, `--artist`, `--cover`, etc. — see `m4b-tool merge --help`).

### Split with a custom filename template

```sh
m4b-tool split --audio-format mp3 \
  --filename-template '{{printf "%02d" .Track}} - {{.Title}}' \
  /work/book.m4b
```

The template is Go's `text/template`. Variables: `Track`,
`TrackTotal`, `Title`, `Album`, `Artist`, `AlbumArtist`, `Genre`,
`Writer`, `Year`, `Series`, `SeriesPart`.

### Verify the toolchain

```sh
m4b-tool doctor
```

Reports which external utilities (ffmpeg, mp4v2) are installed and
runnable. Run after install or whenever a command starts
misbehaving — cheaper than reading stderr.

## Issues

If you hit a bug or compatibility problem with a real-world `.m4b`,
please file an issue at
<https://github.com/warricksothr/m4b-tool/issues>.

## License

[MIT](LICENSE) © Drew Short.

The original PHP m4b-tool is also MIT licensed and remains the
canonical inspiration for this project. This is an independent
reimplementation, not a fork — no PHP code is reused.

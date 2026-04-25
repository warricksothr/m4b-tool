# Overview

## What m4b-tool does

`m4b-tool` is a CLI that manages audiobook files in the m4b container
format. It doesn't do audio processing itself — it orchestrates a set of
external tools (ffmpeg, the mp4v2 suite, optionally fdkaac and tone) and
adds the domain logic they lack: chapter management, metadata mapping
across formats, file organization, and workflow glue.

The Go port keeps the same shape: **thin orchestrator, external tools do
the heavy lifting.** No CGO, no native audio decoding, no native MP4 atom
writing.

## v1 commands

| Command    | Verb                                       | Input                                  | Output                                 |
|------------|--------------------------------------------|----------------------------------------|----------------------------------------|
| `merge`    | combine N audio files → 1 m4b              | dir of mp3/m4a/m4b/flac + metadata     | single `.m4b` with chapter markers     |
| `split`    | split 1 m4b → N files by chapter           | single `.m4b` (or `.flac` + `.cue`)    | one file per chapter in a dir          |
| `chapters` | detect / edit / import chapter markers     | single audio file + optional source    | modified audio file (chapters updated) |

## Data flow

```
                ┌─────────────────────────────────────────┐
                │               CLI (cobra)               │
                └─────────────────────────────────────────┘
                                   │
                                   ▼
         ┌─────────────────────────────────────────────────┐
         │              Command orchestrators              │
         │    merge.Run   split.Run   chapters.Run         │
         └─────────────────────────────────────────────────┘
            │                  │                   │
            ▼                  ▼                   ▼
     ┌──────────┐       ┌──────────┐        ┌────────────┐
     │ Probe /  │       │ Tag map  │        │ Chapter    │
     │ metadata │       │ FF↔MP4   │        │ algorithms │
     └──────────┘       └──────────┘        └────────────┘
                                   │
                                   ▼
          ┌───────────────────────────────────────────────┐
          │               Executable wrappers             │
          │   ffmpeg   mp4chaps   mp4tags   mp4art        │
          │   mp4info  fdkaac     tone                    │
          └───────────────────────────────────────────────┘
```

The CLI layer is dumb: parse flags, build a config struct, hand to an
orchestrator. Orchestrators hold no state beyond the current run.

## Design principles for the port

1. **Prefer `os/exec` with `exec.Cmd`**. Each external tool gets a typed
   wrapper (`internal/ffmpeg`, `internal/mp4v2`, etc.) that builds argv
   and parses output. The wrapper's surface is Go-native; the fact that
   a subprocess runs is an implementation detail.

2. **Time is one type**. Chapter boundaries, silence ranges, and probe
   durations all use the same time type. See
   [data-model.md](data-model.md#time). `time.Duration` is fine;
   milliseconds are the working precision.

3. **No global state**. Config is passed explicitly. The PHP tool uses
   Symfony Console's input/output objects everywhere; the Go port does
   not need an equivalent.

4. **Tool availability is checked at startup**, not at first use. If a
   command needs `ffmpeg` and it's missing, fail immediately with a
   clear message and the expected binary name.

5. **Fallbacks are explicit, not automatic where it matters.** The PHP
   tool silently falls back from `tone` to `ffmpeg` silence detection
   and from `fdkaac` to `ffmpeg` encoding. For v1 make the fallback
   automatic but log it at INFO so users can tell which path ran.

6. **Functional equivalence, not binary equivalence.** See
   [README.md](README.md#fidelity-target).

## Non-goals for v1

- **No custom MP4 atom writer.** mp4v2 and ffmpeg write atoms; we don't.
- **No audio DSP.** Silence detection goes through ffmpeg `silencedetect`.
  We parse the stderr, that's it.
- **No online metadata lookups.** MusicBrainz, Audible, BookBeat,
  Buchhandel, Overdrive importers are P2 (see
  [tag-importers/](tag-importers/)). They can ship later.
- **No GUI, no daemon, no server.** Single-shot CLI.
- **No automatic binary downloads.** We assume the user installs
  ffmpeg/mp4v2 themselves. A `doctor` subcommand that reports which
  tools are found and their versions is fine; fetching them is not.

## Distribution

- Single static Go binary. One `go build` per platform.
- GoReleaser for cross-compile matrix + GitHub Releases.
- Docker image that bundles the binary with ffmpeg and mp4v2, as a
  convenience — not the primary distribution.
- No PHAR, no composer, no PHP extension checks.

## Repository layout (suggested, not binding)

```
.
├── cmd/m4b-tool/          # main package, flag parsing
├── internal/
│   ├── merge/             # merge orchestrator
│   ├── split/             # split orchestrator
│   ├── chapters/          # chapters orchestrator
│   ├── audio/             # Tag, Chapter, time types
│   ├── chapter/           # algorithms: silence, shift, group
│   ├── tag/               # importers; one subpkg per format
│   ├── ffmpeg/            # wrapper
│   ├── mp4v2/             # wrapper (mp4chaps/mp4tags/mp4art/mp4info)
│   ├── fdkaac/            # wrapper
│   └── tone/              # wrapper
├── spec/                  # these docs
└── testdata/              # golden fixtures
```

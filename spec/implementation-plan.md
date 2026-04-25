# Implementation Plan

A suggested milestone order for the Go port. Each milestone is sized
for one focused work session (half a day to a couple of days) and
ends with something runnable and tested.

The order is chosen so every milestone builds on tested foundations;
hardest domain work (chapter algorithms) lands after the supporting
types and I/O exist, not before.

> **Progress tracking**: check the boxes below as milestones complete.
> Keep this file current — it's the map for the next agent in.

## Milestone 0 — Repo scaffolding

- [x] `go mod init <module-path>`
- [x] `cmd/m4b-tool/main.go` — parses no flags yet, prints "ok".
- [x] Repo layout matches [`overview.md`](overview.md#repository-layout-suggested-not-binding).
- [x] CI: GitHub Actions running `go test ./...` and `golangci-lint`.
- [x] GoReleaser config for cross-compile matrix. *(landed in M9.)*

**Outcome:** `go build ./cmd/m4b-tool && ./m4b-tool` prints "ok".

## Milestone 1 — Exec wrapper + ffmpeg probe

Draws from: [external-tools.md §ffmpeg](external-tools.md#ffmpeg),
[external-tools.md §Process runner](external-tools.md#process-runner).

- [x] `internal/exec` — centralized subprocess helper with timeout,
      stdout/stderr capture, streaming-line mode, terminate callbacks.
- [x] `internal/ffmpeg/client.go` — `Client` type with `Bin` field and
      startup binary-existence check.
- [x] `internal/ffmpeg.ProbeDuration(ctx, path)` — fast and exact
      variants. *(Exposed as two methods: `ProbeDuration` and
      `ProbeDurationExact`.)*
- [x] Unit tests for the exec wrapper using a tiny helper binary
      (`testdata/echo-helper/main.go` built at test time).
- [x] Integration test (`-tags=integration`): probe a fixture.
      *(Fixture is synthesized at test time via `ffmpeg anullsrc`
      rather than committed, per the AGENTS.md "no committed audio"
      rule.)*

**Outcome:** `m4b-tool probe <file>` prints duration. (Command is
throwaway; lets you wire the main pipeline early.)

## Milestone 2 — FFMETADATA + silence parsers

Draws from: [external-tools.md §FFMETADATA1 grammar](external-tools.md#ffmetadata1-grammar-produced-and-consumed),
[chapter-algorithms.md §Silence parsing](chapter-algorithms.md#silence-parsing).

- [x] `internal/audio` package with `Tag`, `Chapter`, `Silence` types
      from [data-model.md](data-model.md).
- [x] `internal/ffmpeg/ffmetadata.go` — parser and writer. Round-trip
      test (parse → write → parse produces identical Tag).
- [x] `internal/ffmpeg/silence.go` — parser for `silencedetect` stderr.
- [x] `internal/ffmpeg.DetectSilence(ctx, path, minLen, maxLen)`.
      *(maxLen is a post-parse drop threshold; see
      [chapter-algorithms.md](chapter-algorithms.md#silence-parsing).)*
- [x] Unit tests: table-driven, covering escape sequences, multi-line
      values, `[CHAPTER]` blocks, out-of-order chapters, empty input.

**Outcome:** can probe a file and list silences + embedded chapters.

## Milestone 3 — mp4v2 wrappers

Draws from: [external-tools.md §mp4chaps/mp4tags/mp4art/mp4info](external-tools.md#mp4chaps).

- [x] `internal/mp4v2/mp4info.go` — duration probe with both regex
      variants.
- [x] `internal/mp4v2/mp4chaps.go` — read/write chapters.txt, run
      `mp4chaps -i` / `-r`.
- [x] `internal/mp4v2/mp4tags.go` — write/remove tags with short-flag
      mapping and `-help` feature detection for sort names + purchase
      date.
- [x] `internal/mp4v2/mp4art.go` — add/remove/extract/list cover.
- [x] Shared chapters.txt parser/writer in `internal/mp4v2/chapterstxt.go`.
- [x] Integration smoke test: write chapters to a fixture m4b, probe,
      assert. *(Covers the full pipeline: chapters + tags + cover +
      probe via mp4info + readback via ffmetadata + extract/remove.)*

**Outcome:** full read/write roundtrip on MP4 metadata available.

## Milestone 4 — Pure chapter algorithms

Draws from: [chapter-algorithms.md](chapter-algorithms.md).

- [x] `internal/chapter.Shift` — shifter with negative-length check.
- [x] `internal/chapter.SplitTooLong` — length-based splitter.
      Also `SplitAllTooLong` driver.
- [x] `internal/chapter.MergeTooShort` — length-based merger.
      Also `MergeShortTail` for the tail-trim variant.
- [x] `internal/chapter.Normalize` — regex / strip-chars / numbering.
      Also `PrependIntro` / `AppendOutro` helpers.
      *(Consecutive-numbering heuristic — the >75%-similar rewrite — is
      deferred; base dup-suffix behavior is in place.)*
- [x] `internal/chapter.RemoveDuplicateFollowUps`.
- [x] `internal/chapter.AlignToSilence` — the core algorithm; matches
      PHP's `guessChaptersBySilences`.
      *(Recovery mode for no-match + 60s+ gap is noted in the doc and
      deferred — spec calls it rare.)*
- [x] `internal/chapter.OverloadFromTracks` — overlap-based name
      assignment.
- [x] `internal/tag/cuesheet` — parser.
- [x] Table-driven unit tests for each algorithm. No subprocess, no
      I/O. Cover the edge cases listed in the spec.

**Outcome:** all chapter math is working and tested in isolation.

## Milestone 5 — Importer framework + P0 importers

Draws from: [tag-importers/](tag-importers/).

- [x] `internal/tag.Importer` interface and `Composite` runner.
- [x] `internal/tag/ffmetadata` — P0.
- [x] `internal/tag/chapterstxt` — P0. *(Go package name is `chapterstxt`;
      the canonical importer name is `chapters-txt`.)*
- [x] `internal/tag/description` — P0.
- [x] `internal/tag/cover` — P0.
- [x] `internal/tag/cuesheet` — P0 (wraps M4 parser).
- [x] `internal/tag/filetracks` — P0. *(Go package name; canonical
      importer name is `chapters-from-file-tracks`.)*
- [x] `internal/tag/silencealign` — P0 (wraps M4 algorithm; canonical
      name `guess-chapters-by-silence`).
- [x] `internal/tag/equate` — P0. Reflection-free field lookup with
      normalized alias handling (`album-artist` == `albumartist`).
- [x] `internal/tag/registry.go` — canonical-name catalog with
      `ValidateNames` for flag validation. Actual importer lists are
      built by orchestrators (M6/M7), not the registry.
- [x] `--enable-improvers` / `--disable-improvers` filtering.
      Implemented on `Composite` via Enable/Disable sets.

**Outcome:** can compose a `Tag` from any combination of sidecars +
CLI overrides.

## Milestone 6 — `chapters` command

Draws from: [cli-surface.md §chapters](cli-surface.md#chapters).

- [x] `cmd/m4b-tool` subcommand wiring. *(Stdlib `flag` with a switch
      dispatch; zero external deps so far. Revisit cobra if M7/M8
      make stdlib painful.)*
- [x] `internal/chapters.Run(ctx, config)` orchestrator.
- [x] Flag parsing: all of `--adjust-by-silence`, `--normalize`,
      `--shift`, `--merge-similar`, `--chapter-pattern`, offsets.
      Plus `--output-file`, `--force`, `--no-chapter-import`,
      `--no-chapter-numbering`, `--chapter-replacement`,
      `--chapter-remove-chars`, `--silence-min-length` /
      `--silence-max-length`, and short aliases (`-o`, `-f`, `-s`,
      `-a`, `-b`).
- [x] Defer: `--epub` (P1), `--musicbrainz-id` (P2),
      `--find-misplaced-chapters` (P2 debug aid).
- [x] Parity test: integration test synthesizes an m4b with seeded
      chapters and exercises adjust-by-silence, merge-similar,
      shift, and --output-file export, reading back via ffmetadata.

**Outcome:** `chapters` command end-to-end. First real feature parity
moment with PHP.

## Milestone 7 — `merge` command

Draws from: [cli-surface.md §merge](cli-surface.md#merge),
[tag-importers/README.md §composition-order](tag-importers/README.md#composition-order-in-v1).

- [x] `internal/merge.Run(ctx, config)` orchestrator. *(M7a: concat
      path; M7b: transcoding + silence ops.)*
- [x] File collection, extension filter, recursive scan.
- [x] Worker pool for parallel encoding (`--jobs`). *(M7b — bounded
      goroutine pool with first-error cancel.)*
- [x] `--trim-silence` integration. *(M7b — silenceremove on middle
      files only; first/last keep boundary silence.)*
- [x] `--add-silence` synthesis and interleaving. *(M7b — anullsrc
      synthesized once, interleaved between parts; chapter starts
      shift via filetracks Gap.)*
- [x] Concat via `ffmpeg -f concat`.
- [x] Tag importer composition per spec. *(M7a wired up
      filetracks → chapters-txt → ffmetadata → cover → description →
      cuesheet → embedded-tags merge → equate → CLI overrides.)*
- [x] `--batch-pattern` with placeholder expansion. *(M7c — also
      `--batch-pattern-path` and `--batch-filter`.)*
- [x] `--batch-resume-file` support. *(M7c.)*
- [x] `--dry-run` support. *(M7a.)*
- [ ] Defer: `--musicbrainz-id` (P2). Also deferred from M7b:
      `--audio-quality` (VBR percentage scaling), `--audio-profile`
      (HE-AAC variants), `--audio-extension`, `--fix-mime-type`.

**Outcome:** `merge` command end-to-end. The big one.

## Milestone 8 — `split` command

Draws from: [cli-surface.md §split](cli-surface.md#split).

- [x] `internal/split.Run(ctx, config)` orchestrator.
- [x] Chapter-source resolution (fixed-length, silence, sidecar, cue,
      embedded).
- [x] Per-chapter extraction via `ffmpeg -ss -t`. *(Demuxer-side
      seeking; default is stream-copy, --audio-codec re-encodes.)*
- [x] Filename templating with Go `text/template` (see
      [cli-surface.md note](cli-surface.md#split) — this is a
      breaking change from PHP's Twig).
- [x] Per-output tagging. *(MP4-family via mp4tags; MP3 tag-write is
      deferred until the next pass.)*
- [x] `--reindex-chapters` rewrite.

**Outcome:** v1 complete. All three commands working on the fixture
suite.

## Milestone 9 — Distribution

- [x] GoReleaser config finalized: linux/darwin/windows × amd64/arm64.
- [x] Docker image bundling ffmpeg + mp4v2 + the Go binary. *(Named
      `Containerfile`; OCI-compliant; works with podman/buildah and
      with `docker build -f Containerfile .`. Not yet validated by an
      actual build run.)*
- [x] Release notes template in `.github/release-template.md`.
- [x] `doctor` subcommand reporting found tools + versions.

**Outcome:** shippable v1.

## Post-v1 milestones

Loose sketch — treat as backlog, not commitment:

- **P1 importers**: `ChaptersFromEpub`, `OpenPackagingFormat`,
  `MetadataJson`, `M4bToolJson`, `BuecherHtml`.
- **`meta` command**: standalone metadata I/O command (PHP milestone
  deferred to here).
- **P2 importers**: Audible family, BookBeat, Buchhandel, Overdrive,
  MusicBrainz API client.
- **`tone` integration**: write-path support for freeform `----:` MP4
  atoms.
- **`extra` command**: Audible-specific plugin commands.

## Milestone dependency graph

```
M0 ──► M1 ──► M2 ──► M3 ──► M4 ──► M5 ──► M6 ──► M7 ──► M8 ──► M9
                      │              │
                      └──────────────┴──► (M6, M7, M8 all depend on
                                           M4+M5; otherwise linear)
```

## Exit criteria per milestone

Each milestone is "done" when:
1. Code is merged.
2. Relevant spec sections are up to date (no drift).
3. Tests pass on CI including `-tags=integration` on a container with
   ffmpeg and mp4v2.
4. The checkboxes above are updated in a commit.
5. If the milestone introduced a new package, it has a one-paragraph
   `doc.go` explaining its purpose.

# m4b-tool Go Port — Specification

This directory captures what the Go reimplementation of `m4b-tool` needs to do.
It is written to be self-contained: the original PHP source tree is the
reference implementation, but nothing here should require reading PHP to
understand. File:line references point into the PHP tree for disambiguation
only.

The full `spec/` directory is intended to be copied into the new Go project
repository as its starting design document.

## v1 scope

Three commands:

- `merge`   — combine a set of audio files into a single m4b with chapters.
- `split`   — split an m4b (or flac+cue) into one file per chapter.
- `chapters` — detect, adjust, and import chapter markers.

Out of scope for v1, tracked as later milestones:

- `meta`    — standalone metadata read/write command.
- `extra`   — Audible-specific plugin.

## Fidelity target

**Functionally equivalent**, not byte-exact. Produced audio should be
indistinguishable to the listener; chapters should land at the same
times; required tags should be present. Atom ordering, padding,
encoder-signature tags, and other artifacts of the underlying tools
(ffmpeg, mp4v2) may differ.

Tests should assert on observable properties (duration, chapter count and
offsets, presence and value of required tags) rather than file hashes.

## Documents

**For agents:** start at [`AGENTS.md`](../AGENTS.md) at the repo root.

| File | Contents |
|------|----------|
| [AGENTS.md](../AGENTS.md) | Per-session orientation. Lives at the repo root. |
| [implementation-plan.md](implementation-plan.md) | Milestone order from scaffolding to v1 ship. |
| [overview.md](overview.md) | Architecture, data flow, principles, non-goals. |
| [cli-surface.md](cli-surface.md) | All flags, arguments, and behavior for v1 commands. |
| [data-model.md](data-model.md) | Core types: Tag, Chapter, Silence, time representation. |
| [external-tools.md](external-tools.md) | ffmpeg, mp4v2 suite, fdkaac, tone — invocation contracts. |
| [chapter-algorithms.md](chapter-algorithms.md) | Silence-based detection, shifting, grouping, cue parsing. |
| [metadata-mapping.md](metadata-mapping.md) | Tag property ↔ FFMETADATA ↔ MP4 atom table. |
| [tag-importers/](tag-importers/) | One stub per importer format. P0/P1/P2 priority. |
| [test-fixtures.md](test-fixtures.md) | Golden files and equivalence test plan. |

## Conventions used in these docs

- **P0/P1/P2** — v1 priority. P0 must ship in v1; P1 is desirable; P2 is post-v1.
- **PHP: `path:line`** — pointer into the reference implementation for
  disambiguation. Not a dependency; the Go port stands alone.
- Code fences show shell commands with `$` prompts; everything else is
  language-tagged where relevant.

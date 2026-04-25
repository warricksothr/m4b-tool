# m4b-tool task recipes.
#
# Run `just` (no args) to list available recipes.
# https://just.systems/

# Default: list available recipes.
default:
    @just --list

# Install/refresh the m4b-tool binary in $GOBIN.
install:
    go install ./cmd/m4b-tool

# Run unit + integration tests (needs ffmpeg + mp4v2 on PATH).
test:
    go test ./...
    go test -tags=integration ./...

# Run golangci-lint.
lint:
    golangci-lint run

# Source-embedded ID3-equivalent tags (artist, album, composer, genre,
# date, etc.) pass through automatically via ffmpeg's metadata copy.
# Per-track values (title, track N/M) come from the chapter list.
#
# The split writes to a local tempdir first and bulk-copies to the
# destination at the end. This matters when the destination is on a
# slow filesystem (e.g. /mnt/c on WSL): per-chapter ffmpeg writes
# stay on fast local storage, and the only crossing is one big copy.
#
# Args:
#   source   path to the .m4b file
#   dest     directory to copy the split mp3s into (created if missing)
#   bitrate  optional, default 128k
#   jobs     optional, default 4
#
# Examples:
#   just split-mp3 ./book.m4b /mnt/c/Users/me/Books/Book/
#   just split-mp3 ./book.m4b ./out 192k
#   just split-mp3 ./book.m4b ./out 128k 8

# Split an m4b into per-chapter MP3s and copy them to dest.
split-mp3 source dest bitrate="128k" jobs="4": install
    #!/usr/bin/env bash
    set -euo pipefail
    src=$(realpath '{{source}}')
    dst='{{dest}}'
    if [ ! -f "$src" ]; then
        echo "split-mp3: source not found: $src" >&2
        exit 1
    fi
    mkdir -p "$dst"
    tmp=$(mktemp -d -t m4b-split-XXXXXX)
    trap 'rm -rf "$tmp"' EXIT

    echo ">>> splitting $src"
    echo ">>> bitrate={{bitrate}} jobs={{jobs}} tmp=$tmp"
    m4b-tool split \
        --jobs={{jobs}} \
        --audio-format=mp3 \
        --audio-codec=libmp3lame \
        --audio-bitrate={{bitrate}} \
        --output-dir "$tmp" \
        "$src"

    n=$(find "$tmp" -maxdepth 1 -name '*.mp3' | wc -l)
    echo ">>> copying $n files to $dst"
    cp "$tmp"/*.mp3 "$dst/"
    size=$(du -sh "$dst" | cut -f1)
    echo ">>> done: $n mp3 files, $size total in $dst"

# Print the embedded tags of an m4b/mp3 (sanity check before/after split).
tags file:
    @ffprobe -v error -show_entries format_tags -of default=noprint_wrappers=1 '{{file}}'

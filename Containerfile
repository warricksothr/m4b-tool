# OCI-compliant build file (named `Containerfile` for Podman/buildah;
# Docker also accepts it via `docker build -f Containerfile .`).
#
# Multi-stage build:
#   1. go-build  — compiles the static m4b-tool binary
#   2. mp4v2     — builds the enzo1982 mp4v2 fork, since it's no longer
#                  available in apt and the v1 commands need its CLI
#                  utilities (mp4chaps/mp4tags/mp4art/mp4info)
#   3. runtime   — Debian slim with ffmpeg from apt + mp4v2 from stage 2
#                  + the Go binary

# ----- stage 1: Go build -----
FROM docker.io/library/golang:1.22-bookworm AS go-build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/m4b-tool ./cmd/m4b-tool

# ----- stage 2: mp4v2 utilities -----
FROM docker.io/library/debian:bookworm-slim AS mp4v2
RUN apt-get update && apt-get install -y --no-install-recommends \
        git ca-certificates cmake build-essential \
    && rm -rf /var/lib/apt/lists/*
# Pinned to a specific commit on enzo1982/mp4v2's `main` branch.
# The v2.1.0 tag has a broken cmake install rule (project.h is
# generated into the build directory but install rules look in the
# source directory). `main` carries the fix; no newer tag exists yet.
#
# To bump (do this periodically to pick up upstream bug fixes):
#   git ls-remote https://github.com/enzo1982/mp4v2.git refs/heads/main
# Update both the SHA here and the note in spec/external-tools.md.
# Bumping invalidates the GHA layer cache; the next CI build will
# rebuild mp4v2 from scratch (~3-5 min).
ARG MP4V2_REF=86b5aaf13d03291873608af559fe0399efe898f4

# `git clone --branch` doesn't accept commit SHAs, so use init +
# shallow-fetch instead. This pattern works for branches, tags, AND
# commit SHAs, keeping MP4V2_REF flexible.
RUN mkdir -p /src/mp4v2 \
    && cd /src/mp4v2 \
    && git init -q \
    && git fetch --depth 1 https://github.com/enzo1982/mp4v2.git "${MP4V2_REF}" \
    && git checkout -q FETCH_HEAD
RUN cmake -S /src/mp4v2 -B /build -DCMAKE_BUILD_TYPE=Release \
    && cmake --build /build -j"$(nproc)" \
    && cmake --install /build --prefix /opt/mp4v2

# ----- stage 3: runtime image -----
FROM docker.io/library/debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
        ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# mp4v2 utilities + shared library
COPY --from=mp4v2 /opt/mp4v2/bin/ /usr/local/bin/
COPY --from=mp4v2 /opt/mp4v2/lib/ /usr/local/lib/
RUN ldconfig

COPY --from=go-build /out/m4b-tool /usr/local/bin/m4b-tool

# Run as non-root by default; users mounting volumes can override with
# `--user`. UID 1000 is the Debian-typical user UID.
RUN useradd --create-home --uid 1000 m4b
USER m4b
WORKDIR /work

ENTRYPOINT ["m4b-tool"]

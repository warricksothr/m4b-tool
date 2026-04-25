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

> **Note:** the container image may not yet be published. If
> `docker.io/library/...` or `ghcr.io/warricksothr/m4b-tool` returns
> "image not found", build from the [Containerfile](Containerfile) or
> use one of the other install paths below. This notice will go away
> once an image-publishing workflow is in place.

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

## Issues

If you hit a bug or compatibility problem with a real-world `.m4b`,
please file an issue at
<https://github.com/warricksothr/m4b-tool/issues>.

## License

[MIT](LICENSE) © Drew Short.

The original PHP m4b-tool is also MIT licensed and remains the
canonical inspiration for this project. This is an independent
reimplementation, not a fork — no PHP code is reused.

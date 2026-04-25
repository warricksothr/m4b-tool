# m4b-tool {VERSION}

<!--
  Used as the editorial template when drafting a release. GoReleaser
  produces draft releases automatically with the `## Changelog` section
  filled in from commit messages; copy this template above that block
  before publishing.
-->

## Highlights

- _Headline change 1_
- _Headline change 2_

## Compatibility

- Go binary: linux/darwin/windows × amd64/arm64.
- Runtime requires `ffmpeg` and the `mp4v2` CLI utilities (`mp4chaps`,
  `mp4tags`, `mp4art`, `mp4info`) on `PATH`. Run `m4b-tool doctor` to
  verify.
- The container image bundles both. Pull
  `ghcr.io/warricksothr/m4b-tool:{VERSION}`.

## Install

**Binary**: download the archive matching your OS/arch from the
assets list, extract, and place `m4b-tool` on your `PATH`.

**Container**:
```sh
podman run --rm -v "$PWD:/work" ghcr.io/warricksothr/m4b-tool:{VERSION} doctor
```

## Breaking changes

- _List, with migration guidance. Empty when there are none._

## Known issues

- _Items the user should know going in. Empty when there are none._

## Changelog

_The auto-generated changelog from GoReleaser appears below this line
when the release is finalized._

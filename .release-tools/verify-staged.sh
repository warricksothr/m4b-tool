#!/usr/bin/env bash
# Sanity-check the staged release commit before `git commit`.
#
# Usage:
#   .release-tools/verify-staged.sh
#
# Exits 0 with a "✓ clean" line on success, non-zero with one ✗ line per
# finding on failure. Catches the failure modes seen during real cuts:
#
#   - .release-exclude paths still present in the worktree or index
#     (manifest didn't apply, or got skipped).
#   - .release-only paths missing from the worktree (squash dropped
#     them and the manifest restore didn't fire).
#   - `go build ./...` broken on the staged tree (squash brought in a
#     half-finished change, or release-side hand-edits broke compilation).
#
# Project-specific in that it runs `go build` — adapt for non-Go repos.
# See ~/.claude/skills/release-cut/SKILL.md for the surrounding flow.

set -uo pipefail

cd "$(git rev-parse --show-toplevel)"

fail=0

[ -f .release-exclude ] || { echo "✗ missing .release-exclude" >&2; exit 1; }
[ -f .release-only ]    || { echo "✗ missing .release-only" >&2; exit 1; }

# Pre-load .release-only into a lookup set. Paths listed in both
# manifests are the divergent-content case (e.g. README.md): the
# exclude step strips main's version, the only step restores release's
# version, and the path *should* be present at the end. Skip those
# during the exclude-must-be-absent check.
declare -A in_only
while IFS= read -r p; do
  [[ -z "$p" || "$p" =~ ^# ]] && continue
  in_only["$p"]=1
done < .release-only

while IFS= read -r p; do
  [[ -z "$p" || "$p" =~ ^# ]] && continue
  [[ -n "${in_only[$p]+set}" ]] && continue
  if [ -e "$p" ]; then
    echo "✗ exclude path still in worktree: $p" >&2
    fail=1
  fi
  if git ls-files --error-unmatch -- "$p" >/dev/null 2>&1; then
    echo "✗ exclude path still tracked: $p" >&2
    fail=1
  fi
done < .release-exclude

while IFS= read -r p; do
  [[ -z "$p" || "$p" =~ ^# ]] && continue
  if [ ! -e "$p" ]; then
    echo "✗ release-only path missing: $p" >&2
    fail=1
  fi
done < .release-only

if [ -f go.mod ]; then
  if ! go build ./... 2>&1; then
    echo "✗ go build failed on staged tree" >&2
    fail=1
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "✓ staged release commit looks clean"
fi
exit "$fail"

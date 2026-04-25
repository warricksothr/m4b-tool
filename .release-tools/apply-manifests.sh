#!/usr/bin/env bash
# Apply .release-exclude (strip) and .release-only (restore) manifests.
#
# Usage:
#   .release-tools/apply-manifests.sh
#
# Run from the release branch *after* a `git merge --squash main` (and
# any add/add conflict resolution) and *before* `git commit`. Mutates
# the index and worktree:
#
#   - Every path in .release-exclude is removed from both.
#   - Every path in .release-only is restored from HEAD (the previous
#     release tip) and re-staged.
#
# Safe to re-run: paths already absent stay absent, paths already at
# HEAD stay at HEAD.
#
# See ~/.claude/skills/release-cut/SKILL.md for the surrounding flow.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

[ -f .release-exclude ] || { echo "✗ missing .release-exclude" >&2; exit 1; }
[ -f .release-only ]    || { echo "✗ missing .release-only" >&2; exit 1; }

stripped=0
while IFS= read -r p; do
  [[ -z "$p" || "$p" =~ ^# ]] && continue
  if git ls-files --error-unmatch -- "$p" >/dev/null 2>&1 || [ -e "$p" ]; then
    git rm -rf --cached --ignore-unmatch -- "$p" >/dev/null 2>&1 || true
    rm -rf -- "$p"
    stripped=$((stripped + 1))
  fi
done < .release-exclude

restored=0
while IFS= read -r p; do
  [[ -z "$p" || "$p" =~ ^# ]] && continue
  if git checkout HEAD -- "$p" 2>/dev/null; then
    git add -- "$p"
    restored=$((restored + 1))
  fi
done < .release-only

echo "manifests applied: ${stripped} stripped, ${restored} restored"

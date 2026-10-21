#!/bin/sh
# Prepare a DSH checkout with the CoNest overlay applied.
# Usage: dsh/scripts/sync.sh [target-dir]   (default: .local/dsh at the repo root)
# Re-running resets the target to the pinned commit, so edit overlay sources
# under dsh/, not inside the target.
set -eu

here=$(cd "$(dirname "$0")/.." && pwd)
root=$(cd "$here/.." && pwd)
target=${1:-"$root/.local/dsh"}
repo=$(node -p "require('$here/upstream.pin.json').repo")
commit=$(node -p "require('$here/upstream.pin.json').commit")

# The target must be its own repository: an existing empty directory inside
# this repo would otherwise resolve to CoNest itself and get force-checked-out.
top=$(git -C "$target" rev-parse --show-toplevel 2>/dev/null || true)
if [ -d "$target" ] && [ -n "$top" ] && [ "$top" != "$(cd "$target" && pwd -P)" ]; then
  echo "refusing: $target is inside another repository ($top)" >&2
  exit 1
fi
if [ -z "$top" ]; then
  git clone --filter=blob:none "$repo" "$target"
fi
if ! git -C "$target" cat-file -e "$commit^{commit}" 2>/dev/null; then
  git -C "$target" fetch --quiet "$repo" "$commit"
fi
git -C "$target" checkout --quiet --force "$commit"
git -C "$target" clean --quiet -fd packages/experimental

for pkg in "$here"/packages/*/; do
  name=$(basename "$pkg")
  rm -rf "$target/packages/experimental/$name"
  cp -R "$pkg" "$target/packages/experimental/$name"
done

for patch in "$here"/patches/*.patch; do
  [ -e "$patch" ] || continue
  git -C "$target" apply "$patch"
done

echo "DSH $commit with CoNest overlay ready at $target"
echo "next: cd $target && corepack pnpm install && pnpm run build:artifacts"

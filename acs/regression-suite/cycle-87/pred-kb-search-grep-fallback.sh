#!/usr/bin/env bash
# AC-ID: cycle-87-kb-search-grep-fallback
# Verifies kb-search.sh falls back to grep -rE when ripgrep (rg) is unavailable.
# We stub PATH to a directory that contains every standard tool EXCEPT rg, then
# confirm the script still returns hits and signals fallback (stderr note or
# absence of "rg:" prefixes in output).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
KB="$REPO_ROOT/scripts/research/kb-search.sh"

if [ ! -f "$KB" ]; then
  echo "RED cycle-87-kb-search-grep-fallback: kb-search.sh not found at $KB"
  exit 1
fi

TEST_DIR=$(mktemp -d -t kb-search-fallback.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT

KB_DIR="$TEST_DIR/knowledge-base/research"
mkdir -p "$KB_DIR"
PHRASE="zzqq-fallback-87-marker"
printf '# Header\n\nLine 1\nContains %s here.\nLine 3\n' "$PHRASE" > "$KB_DIR/seed.md"

# Build a PATH stub that masks ripgrep. We do this by creating a shim dir with
# symlinks to common tools but NO `rg` entry, then prepending it to PATH and
# removing the rest. We allow only /bin and /usr/bin tools the script may need.
STUB="$TEST_DIR/stubbin"
mkdir -p "$STUB"
# Symlink the canonical resolutions of common utilities (excluding rg).
for tool in bash sh sed awk grep cat head wc env tr date dirname basename find sort uniq mkdir mv cp rm ls stat realpath jq python3; do
  src=$(command -v "$tool" 2>/dev/null || true)
  if [ -n "$src" ] && [ "$(basename "$src")" != "rg" ]; then
    ln -sf "$src" "$STUB/$tool"
  fi
done

# Sanity: ensure `rg` is NOT in the stub PATH.
if [ -e "$STUB/rg" ]; then
  echo "RED cycle-87-kb-search-grep-fallback: test setup leaked rg into stub"
  exit 1
fi

export EVOLVE_KB_SEARCH_PATHS="$KB_DIR"

OUT_FILE="$TEST_DIR/out.txt"
set +e
PATH="$STUB" bash "$KB" "$PHRASE" > "$OUT_FILE" 2>&1
rc=$?
set -e

if [ "$rc" != "0" ]; then
  echo "RED cycle-87-kb-search-grep-fallback: kb-search.sh exited rc=$rc under rg-less PATH"
  head -20 "$OUT_FILE" >&2 2>/dev/null || true
  exit 1
fi

# The marker phrase must appear — proves grep -rE fallback found it.
if ! grep -F "$PHRASE" "$OUT_FILE" >/dev/null 2>&1; then
  echo "RED cycle-87-kb-search-grep-fallback: fallback did not return the marker"
  head -20 "$OUT_FILE" >&2
  exit 1
fi

# Defense-in-depth: the output must reference seed.md (file:line shape) so we
# know it actually walked the KB tree and didn't just print the query back.
if ! grep -E 'seed\.md(:[0-9]+|.*L[0-9]+|$)' "$OUT_FILE" >/dev/null 2>&1; then
  echo "RED cycle-87-kb-search-grep-fallback: output missing file path / line marker"
  head -20 "$OUT_FILE" >&2
  exit 1
fi

echo "GREEN cycle-87-kb-search-grep-fallback: grep -rE fallback returned marker hit"
exit 0

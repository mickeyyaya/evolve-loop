#!/usr/bin/env bash
# AC-ID: cycle-87-kb-search-fixture
# Verifies kb-search.sh against a seeded fixture:
#   - returns markdown hits with file:line + context
#   - output ≤ 2KB
#   - completes in < 5s
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
KB="$REPO_ROOT/scripts/research/kb-search.sh"

if [ ! -f "$KB" ]; then
  echo "RED cycle-87-kb-search-fixture: kb-search.sh not found at $KB"
  exit 1
fi

# Place fixture in a private temp KB; point kb-search at it via env override.
TEST_DIR=$(mktemp -d -t kb-search-fixture.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
KB_DIR="$TEST_DIR/knowledge-base/research"
mkdir -p "$KB_DIR"

# Distinctive phrase that should NOT collide with any real KB content.
PHRASE="zzqq-tdd87-fixture-marker"
cat > "$KB_DIR/seed.md" <<EOF
# Seed fixture

Line two of irrelevant content.
Line three contains the marker: $PHRASE — needle to be found.
Trailing content for context window.
EOF

# kb-search.sh must accept a search root override. We try two common shapes:
#   1. EVOLVE_KB_SEARCH_PATHS env var (per intent.md non-goals note)
#   2. Positional second arg
export EVOLVE_KB_SEARCH_PATHS="$KB_DIR"

# Time-bounded invocation (5s ceiling). We use a portable backgrounded child
# + kill rather than `timeout` (not always installed on macOS).
OUT_FILE="$TEST_DIR/kb-out.txt"
(
  bash "$KB" "$PHRASE" > "$OUT_FILE" 2>&1
  echo $? > "$TEST_DIR/rc.txt"
) &
child=$!
elapsed=0
while kill -0 "$child" 2>/dev/null; do
  if [ "$elapsed" -ge 5 ]; then
    kill "$child" 2>/dev/null || true
    echo "RED cycle-87-kb-search-fixture: kb-search.sh exceeded 5s deadline"
    exit 1
  fi
  sleep 1
  elapsed=$((elapsed + 1))
done
wait "$child" 2>/dev/null || true

rc=$(cat "$TEST_DIR/rc.txt" 2>/dev/null || echo 99)
if [ "$rc" != "0" ]; then
  echo "RED cycle-87-kb-search-fixture: kb-search.sh exited rc=$rc"
  head -20 "$OUT_FILE" >&2 2>/dev/null || true
  exit 1
fi

# Size check: ≤ 2048 bytes.
size=$(wc -c < "$OUT_FILE" | tr -d ' ')
if [ "$size" -gt 2048 ] 2>/dev/null; then
  echo "RED cycle-87-kb-search-fixture: output size $size bytes > 2048 limit"
  exit 1
fi

# Format check: hit line must contain the seed.md basename + a line-number
# marker (":<digits>:" or "L<digits>" style — ripgrep default vs custom).
if ! grep -E 'seed\.md(:[0-9]+|.*L[0-9]+)' "$OUT_FILE" >/dev/null 2>&1; then
  echo "RED cycle-87-kb-search-fixture: output missing file:line marker"
  head -20 "$OUT_FILE" >&2
  exit 1
fi

# The marker phrase must appear in output (proves the hit, not just a header).
if ! grep -F "$PHRASE" "$OUT_FILE" >/dev/null 2>&1; then
  echo "RED cycle-87-kb-search-fixture: marker phrase not in output"
  head -20 "$OUT_FILE" >&2
  exit 1
fi

echo "GREEN cycle-87-kb-search-fixture: ${size}B output, file:line present, marker found, <5s"
exit 0

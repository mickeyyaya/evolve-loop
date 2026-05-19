#!/usr/bin/env bash
# Predicate: mutate-eval.sh grep_only_check returns rc=2 for grep-only input
# Behavioral: creates a fixture grep-only file and invokes mutate-eval.sh on it
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MUTATE="$REPO_ROOT/scripts/verification/mutate-eval.sh"
# Create a minimal grep-only eval fixture in a temp file
TMP_EVAL=$(mktemp /tmp/test-mutate-eval-XXXX.md)
cleanup() { rm -f "$TMP_EVAL"; }
trap cleanup EXIT
cat > "$TMP_EVAL" <<'EOF'
# Test eval
grep -q "PASS" result.txt
EOF
# mutate-eval.sh should return rc=2 (DECLINE) for this grep-only file
output=$(bash "$MUTATE" "$TMP_EVAL" 2>&1)
rc=$?
# Verify rc=2 and DECLINE message
[ "$rc" -eq 2 ]
echo "$output" | grep -q "DECLINE"

#!/usr/bin/env bash
# fake-claude.sh — stand-in for `claude` binary in mock tests.
# Parses -p <prompt> and emits a synthetic Write tool effect.

set -uo pipefail

if [[ "${1:-}" == "--version" ]]; then
  echo "fake-claude 0.0.1 (Mock)"
  exit 0
fi

prompt=""
artifact_path=""
model=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p) prompt="$2"; shift 2 ;;
    --model) model="$2"; shift 2 ;;
    --allowedTools) shift; while [[ $# -gt 0 && "$1" != --* ]]; do shift; done ;;
    *) shift ;;
  esac
done

# Extract token from prompt and artifact path
token=$(echo "$prompt" | grep -oE 'challenge-token: [a-f0-9]+' | awk '{print $2}' | head -1)
artifact_path=$(echo "$prompt" | grep -oE '/[^ "]*\.md' | head -1)

if [[ -n "$artifact_path" ]]; then
  mkdir -p "$(dirname "$artifact_path")"
  cat > "$artifact_path" <<EOF
<!-- challenge-token: ${token:-unknown} -->
PROTOTYPE OK (FAKE-CLAUDE)
EOF
fi

echo "[fake-claude] Wrote artifact: $artifact_path"
exit 0

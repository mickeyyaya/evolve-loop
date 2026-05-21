#!/usr/bin/env bash
# fake-agy.sh — stand-in for `agy` binary

set -uo pipefail

if [[ "${1:-}" == "--version" ]]; then
  echo "fake-agy 0.0.1 (Mock)"
  exit 0
fi

prompt=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--print|--prompt) prompt="$2"; shift 2 ;;
    --dangerously-skip-permissions) shift ;;
    *) shift ;;
  esac
done

token=$(echo "$prompt" | grep -oE 'challenge-token: [a-f0-9]+' | awk '{print $2}' | head -1)
artifact_path=$(echo "$prompt" | grep -oE '/[^ "]*\.md' | head -1)

if [[ -n "$artifact_path" ]]; then
  mkdir -p "$(dirname "$artifact_path")"
  cat > "$artifact_path" <<EOF
<!-- challenge-token: ${token:-unknown} -->
PROTOTYPE OK (FAKE-AGY)
EOF
fi

echo "(fake-agy) wrote $artifact_path"
exit 0

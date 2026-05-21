#!/usr/bin/env bash
# fake-codex.sh — stand-in for `codex` binary

set -uo pipefail

if [[ "${1:-}" == "--version" ]]; then
  echo "fake-codex-cli 0.0.1 (Mock)"
  exit 0
fi

# Skip the 'exec' subcommand
[[ "${1:-}" == "exec" ]] && shift

output_path=""
model=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-last-message) output_path="$2"; shift 2 ;;
    -m) model="$2"; shift 2 ;;
    *) shift ;;
  esac
done

# Read prompt from stdin
prompt=$(cat)
token=$(echo "$prompt" | grep -oE 'challenge-token: [a-f0-9]+' | awk '{print $2}' | head -1)

if [[ -n "$output_path" ]]; then
  mkdir -p "$(dirname "$output_path")"
  cat > "$output_path" <<EOF
<!-- challenge-token: ${token:-unknown} -->
PROTOTYPE OK (FAKE-CODEX, model=${model:-default})
EOF
fi

echo "(fake-codex) responded; wrote to $output_path"
exit 0

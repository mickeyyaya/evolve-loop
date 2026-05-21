#!/usr/bin/env bash
# fake-claude-argcapture.sh — like fake-claude.sh but records its full argv.
#
# Used by permission-mode-drivers.bats to verify that bridge drivers pass
# --permission-mode through to the claude binary.
#
# Output: writes one arg per line to ${BRIDGE_FAKE_ARGS_FILE:-/tmp/fake-claude-args.txt}.
# Behavior otherwise mirrors fake-claude.sh (handles --version, -p prompt,
# emits a synthetic artifact at the path embedded in the prompt).

set -uo pipefail

out_file="${BRIDGE_FAKE_ARGS_FILE:-/tmp/fake-claude-args.txt}"
# Write one arg per line so tests can grep -F -x "--permission-mode" and
# the next line for "plan", AND check that "--dangerously-skip-permissions"
# is absent / present as expected.
: > "$out_file"
for a in "$@"; do
  printf '%s\n' "$a" >> "$out_file"
done

if [[ "${1:-}" == "--version" ]]; then
  echo "fake-claude-argcapture 0.0.1 (Mock)"
  exit 0
fi

# Mirror fake-claude.sh artifact behavior so the bridge's artifact-verification
# step doesn't fail and the test can focus on arg assertions.
prompt=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p) prompt="$2"; shift 2 ;;
    --model|--permission-mode) shift 2 ;;
    --allowedTools) shift; while [[ $# -gt 0 && "$1" != --* ]]; do shift; done ;;
    *) shift ;;
  esac
done

token=$(echo "$prompt" | grep -oE 'challenge-token: [a-f0-9]+' | awk '{print $2}' | head -1)
artifact_path=$(echo "$prompt" | grep -oE '/[^ "]*\.md' | head -1)

if [[ -n "$artifact_path" ]]; then
  mkdir -p "$(dirname "$artifact_path")"
  cat > "$artifact_path" <<EOF
<!-- challenge-token: ${token:-unknown} -->
PROTOTYPE OK (FAKE-CLAUDE-ARGCAPTURE)
EOF
fi

echo "[fake-claude-argcapture] wrote artifact: ${artifact_path:-(none)}; args captured to: $out_file"
exit 0

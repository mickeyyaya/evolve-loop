#!/usr/bin/env bash
# fake-cli-prompt-emitter.sh — synthesize realistic CLI pane content per pattern
#
# Usage:
#   fake-cli-prompt-emitter.sh --emit-pattern=<name>
#
# Emits to stdout a pane snapshot designed to match the named auto-respond
# regex pattern. Used by tests/simulation/auto-respond-scenarios.bats to drive
# auto_respond_decide deterministically without launching any real CLI.
#
# Pattern catalog mirrors lib/manifests/<cli>.json:
#   auth_recheck, rate_limit, quota_exhausted, permission_prompt,
#   trust_prompt, model_deprecation_continue, terminal_resize_redraw,
#   fact_forcing_gate

set -uo pipefail

PATTERN=""
for arg in "$@"; do
  case "$arg" in
    --emit-pattern=*) PATTERN="${arg#--emit-pattern=}" ;;
    -h|--help)
      sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
  esac
done

if [[ -z "$PATTERN" ]]; then
  echo "error: --emit-pattern=<name> required" >&2
  exit 64
fi

case "$PATTERN" in
  auth_recheck)
    cat <<'EOF'
❯ Working on your request...
ERROR: Authentication required. Please log in again to continue.
EOF
    ;;
  rate_limit)
    cat <<'EOF'
❯ Working on your request...
HTTP 429: rate limit exceeded — please retry after 60s
EOF
    ;;
  quota_exhausted)
    cat <<'EOF'
❯ Generating response...
ERROR: monthly quota exceeded. Upgrade your plan to continue.
EOF
    ;;
  permission_prompt)
    cat <<'EOF'
Allow tool execution: Write /tmp/output.md?
EOF
    ;;
  trust_prompt_codex)
    cat <<'EOF'
Do you trust the contents of this directory?
1. Yes, I trust this folder
2. No, exit
EOF
    ;;
  trust_prompt_agy)
    cat <<'EOF'
Antigravity CLI requires permission to access this directory.
EOF
    ;;
  model_deprecation_continue)
    cat <<'EOF'
Note: this model is deprecated. Continue with the deprecated model? [y/n]
EOF
    ;;
  terminal_resize_redraw)
    cat <<'EOF'
Terminal too small. Please resize and press enter.
EOF
    ;;
  fact_forcing_gate)
    cat <<'EOF'
◇ Fact-Forcing Gate: please present these facts before writing
EOF
    ;;
  *)
    echo "error: unknown pattern '$PATTERN'" >&2
    exit 65
    ;;
esac

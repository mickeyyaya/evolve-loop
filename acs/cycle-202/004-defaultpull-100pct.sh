#!/usr/bin/env bash
# ACS cycle-202 / AC4 — DefaultPull reaches 100% statement coverage.
#
# Behavioral: same shape as 003 but for DefaultPull. Fails at baseline
# (DefaultPull = 50.0% — only the non-git no-op leg is covered by the existing
# TestDefaultPull_NonGitDir). Goes GREEN once the Builder adds a test that
# creates a `.git` directory so the two swallowed `exec.Command` git calls and
# the trailing `return nil` are executed.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"
DIR=$(acs_go_module_dir)

PROF=$(mktemp) || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -f "$PROF"' EXIT

if ! (cd "$DIR" && go test -count=1 -coverprofile="$PROF" ./internal/marketplacepoll/... >/dev/null 2>&1); then
  echo "RED: marketplacepoll suite failed — cannot measure DefaultPull coverage" >&2
  exit 1
fi

PCT=$(cd "$DIR" && go tool cover -func="$PROF" \
  | awk '$2=="DefaultPull"{gsub(/%/,"",$NF); print $NF; exit}')

if [ -z "$PCT" ]; then
  echo "RED: DefaultPull not present in coverage profile" >&2
  exit 1
fi
if acs_pct_ge "$PCT" "100.0"; then
  echo "GREEN: DefaultPull coverage ${PCT}% >= 100%" >&2
  exit 0
fi
echo "RED: DefaultPull coverage ${PCT}% < 100%" >&2
exit 1

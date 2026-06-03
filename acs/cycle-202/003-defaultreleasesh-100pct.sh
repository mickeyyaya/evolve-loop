#!/usr/bin/env bash
# ACS cycle-202 / AC3 — DefaultReleaseSh reaches 100% statement coverage.
#
# Behavioral: runs the marketplacepoll suite WITH a coverage profile, then
# reads the PER-FUNCTION coverage that `go tool cover -func` reports and
# asserts the exact function hits 100.0%. This necessarily fails at baseline
# (DefaultReleaseSh = 0.0% — never exercised) and only goes GREEN once the
# Builder adds the three branch tests:
#   - script absent           -> returns nil
#   - script present, exit !=0 -> wrapped error
#   - script present, exit 0   -> returns nil
# Adding a magic string to the source cannot satisfy this: only tests that
# actually drive the function's branches move the coverage number.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"
DIR=$(acs_go_module_dir)

PROF=$(mktemp) || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -f "$PROF"' EXIT

if ! (cd "$DIR" && go test -count=1 -coverprofile="$PROF" ./internal/marketplacepoll/... >/dev/null 2>&1); then
  echo "RED: marketplacepoll suite failed — cannot measure DefaultReleaseSh coverage" >&2
  exit 1
fi

# $2 is the function name column; $NF is the trailing "NN.N%". Exact-match the
# function name (no ^ anchor, no PASS-line scraping) so this is immune to the
# indent/scrape footguns documented in acs/lib/assert.sh.
PCT=$(cd "$DIR" && go tool cover -func="$PROF" \
  | awk '$2=="DefaultReleaseSh"{gsub(/%/,"",$NF); print $NF; exit}')

if [ -z "$PCT" ]; then
  echo "RED: DefaultReleaseSh not present in coverage profile" >&2
  exit 1
fi
if acs_pct_ge "$PCT" "100.0"; then
  echo "GREEN: DefaultReleaseSh coverage ${PCT}% >= 100%" >&2
  exit 0
fi
echo "RED: DefaultReleaseSh coverage ${PCT}% < 100%" >&2
exit 1

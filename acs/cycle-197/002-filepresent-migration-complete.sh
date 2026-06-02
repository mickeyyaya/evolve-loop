#!/usr/bin/env bash
# AC-ID: cycle-197-002-filepresent-migration-complete
# AC-source: intent.md acceptance_criteria[3] ("acs skip-guards use
#            FilePresent"); scout-report.md Task 1 gate.
# Behavioral + migration-completeness predicate (3 checks):
#   (a) ZERO `if !acsassert.FileExists(t` skip-guards remain anywhere under
#       go/acs/. This is the FileExists-as-skip anti-pattern: acsassert.FileExists
#       calls t.Errorf when the file is missing, so the test is marked FAILED
#       before it Skips. RED baseline = 33 files / 111 sites.
#   (b) The migrated guards now use fixtures.FilePresent (>=1 file) — proving the
#       guards were CONVERTED to the pure-boolean predicate, not merely deleted.
#   (c) `go test ./acs/...` EXITS 0 — the migration compiles and the migrated
#       tests still skip/run correctly with no regression. Authoritative
#       behavioral signal via assert_go_test_pass (exit code, not output scrape).
#
# Scope note: cycle42 / cycle75 use acsassert.FileExists as ASSERTION guards
#   (NOT the `if !...{Skip}` form). They do not match the (a) pattern and are
#   intentionally left unmigrated — the migration targets only skip-guards.
#
# Mutation spec:
#   Mutant: leave any one skip-guard unmigrated  -> (a) FAIL (RED).
#   Mutant: delete the guards instead of convert -> (b) FAIL (no FilePresent).
#   Mutant: migration breaks an acs test         -> (c) FAIL.
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
top="$(git rev-parse --show-toplevel)"
. "$top/acs/lib/assert.sh"

# (a) no FileExists-as-skip guards remain
remaining=$(grep -rl "if !acsassert.FileExists(t" "$top/go/acs/" 2>/dev/null | wc -l | tr -d ' ')
if [ "${remaining:-0}" -ne 0 ]; then
  echo "RED: $remaining file(s) still use the FileExists-as-skip anti-pattern:" >&2
  grep -rl "if !acsassert.FileExists(t" "$top/go/acs/" >&2
  exit 1
fi
echo "GREEN: zero FileExists-as-skip guards remain under go/acs/" >&2

# (b) guards were converted to fixtures.FilePresent, not deleted
fp=$(grep -rl "fixtures.FilePresent(" "$top/go/acs/" 2>/dev/null | wc -l | tr -d ' ')
if [ "${fp:-0}" -lt 1 ]; then
  echo "RED: no go/acs file uses fixtures.FilePresent — guards deleted, not migrated" >&2
  exit 1
fi
echo "GREEN: $fp file(s) now use fixtures.FilePresent" >&2

# (c) no regression across the acs suite
assert_go_test_pass ./acs/... || exit 1
echo "PASS"; exit 0

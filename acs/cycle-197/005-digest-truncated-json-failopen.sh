#!/usr/bin/env bash
# AC-ID: cycle-197-005-digest-truncated-json-failopen
# AC-source: intent.md acceptance_criteria[1] ("a new negative/chaos test feeds
#            a malformed/truncated artifact and asserts WARN-completion, not
#            abort"); scout-report.md Task 3 (truncated-JSON gap).
# Behavioral + presence predicate (2 checks):
#   (a) A truncated-JSON test now exists in internal/router/digest_test.go
#       (grep -i "truncat" >=1). RED baseline = 0. The existing
#       TestDigest_FailOpenOnMissingAndCorrupt uses syntactically-invalid
#       "{ this is not json"; the new case must use a STRUCTURALLY-VALID but
#       TRUNCATED payload (e.g. `{"phase":"build"` — the partial-write /
#       SIGKILL-mid-flush failure mode), a distinct branch.
#   (b) `go test -run TestDigest ./internal/router/` EXITS 0 — the new test
#       asserts Digest FAILS OPEN on truncated input (Build.Present==false AND
#       err==nil): a graceful WARN-completion under the errors-as-observations
#       contract, never an abort. Authoritative via assert_go_test_pass.
#
# Mutation spec:
#   Mutant: add no truncated-JSON test               -> (a) FAIL (RED).
#   Mutant: assert Digest returns err!=nil on truncation (abort, not fail-open)
#                                                     -> (b) FAIL (run fails).
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
top="$(git rev-parse --show-toplevel)"
. "$top/acs/lib/assert.sh"

dt="$top/go/internal/router/digest_test.go"
n=$(grep -ci "truncat" "$dt" 2>/dev/null) || n=0
if [ "${n:-0}" -lt 1 ]; then
  echo "RED: no truncated-JSON test in digest_test.go" >&2
  exit 1
fi
echo "GREEN: truncated-JSON test present ($n match line(s))" >&2

assert_go_test_pass ./internal/router/ 'TestDigest' || exit 1
echo "PASS"; exit 0

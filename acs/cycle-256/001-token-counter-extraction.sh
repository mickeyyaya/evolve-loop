#!/usr/bin/env bash
# ACS — cycle-256 task `token-counter-extraction`
# Behavioral: invokes the system under test via `go test` EXIT CODE
# (assert.sh; cycle-137 lesson — never scrape PASS). Covers all three ACs:
#   - extractTokenCount parser (peak + zero on no-match/malformed)
#   - token-usage.json sidecar written by the real REPL engine (peak_tokens >= 0)
#   - bridge.Report.TokenUsage populated, malformed-sidecar-tolerant
# Contract source: go/internal/bridge/tokencount_test.go.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/bridge/ 'TestExtractTokenCount' || exit 1
assert_go_test_pass ./internal/bridge/ 'TestTmuxPhase_WritesTokenUsage' || exit 1
assert_go_test_pass ./internal/bridge/ 'TestBuildReport_TokenUsage' || exit 1

echo "GREEN: token-counter extraction + sidecar + report field behavioral suite passes"
exit 0

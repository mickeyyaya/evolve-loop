#!/usr/bin/env bash
# AC-ID:         cycle-230-004
# Description:   LedgerEntry.Source ("source,omitempty") added; recordRoutingDecision stamps Source:"router" on phase_skipped entries; round-trip + omitempty hash-chain stability
# Evidence:      go/internal/core/ports.go + go/internal/core/orchestrator.go + go/internal/core/ledger_source_test.go
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 4 (ledger-skip-source) — attribution fidelity, PRIORITY 0 carryover sub-mode

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# Behavioral: field presence, router stamp via recordRoutingDecision, and
# JSON round-trip/omitempty — exit-code assertion per cycle-137 rule.
assert_go_test_pass ./internal/core/... 'TestLedgerEntrySource' || exit 1

# No collateral damage: the whole core package must stay green (ledger
# hash-chain code is load-bearing for every cycle).
assert_go_test_pass ./internal/core/... || exit 1

echo "GREEN: LedgerEntry.Source wired; router stamp + round-trip + omitempty verified" >&2
exit 0

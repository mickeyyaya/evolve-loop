# Comment history: `acs/cycle739`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle739/predicates_test.go:3` — above `package cycle739`

```text
// Package cycle739 materializes the cycle-739 acceptance criteria for the sole
// committed top_n task fleet-config-hot-reload-wave-boundary (triage-report.md
// ## top_n; the scout's other selections were deferred by triage, so per R9.3
// no predicates bind to them and no deferred-floor predicates exist).
//
// AC map (1:1):
//
//	AC1 min_lanes committed mid-batch takes effect next wave      → C739_001
//	AC2 count committed mid-batch takes effect next wave          → C739_002
//	AC3 unchanged policy ⇒ identical config + zero reload noise   → C739_003
//	AC4 malformed policy at boundary holds width + WARNs (neg.)   → C739_004
//	AC5 repo-wide -race / vet / apicover on touched pkgs          → manual+checklist (audit CI-parity gate, ADR-0069)
//	AC6 batch loop invokes the reload seam at every wave boundary → manual+checklist (auditor)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract in cmd/evolve, which EXERCISES the SUT (the wave-boundary
// reload seam against real temp policy.json documents) — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```

### `go/acs/cycle739/predicates_test.go:49` — above `func TestC739_001_ReloadsMinLanesAtWaveBoundary(t *testing.T) {`

```text
// AC1 — the incident twin: min_lanes 3->10 committed between waves is
// resolved at the next wave boundary, logged, and the new floor holds width
// under a quota bench.
```

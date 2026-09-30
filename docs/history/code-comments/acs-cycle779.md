# Comment history: `acs/cycle779`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle779/predicates_test.go:3` — above `package cycle779`

```text
// Package cycle779 materializes the cycle-779 acceptance criteria for the sole
// committed top_n task token-telemetry-input-cache-fidelity (triage-report.md
// ## top_n; lane-scope.json assigns exactly this one id, so per R9.3 no
// predicates bind to the scout report's other-lane items — ship-window-lease,
// mechanical-scans-to-native, disjoint-composition-fastpath belong to other
// concurrent lanes).
//
// Task source: inbox 2026-07-13T13-06-00Z-token-input-cache-fidelity.json
// (weight 0.96, operator-boosted 2026-07-13). Incident: the first live token
// baseline (cycles 767-774) reported input=0 / cache_read=0 / cache_write=0
// for EVERY phase — output-only telemetry hides the dominant cost dimension
// (input outweighs output 2:1-100:1 per
// knowledge-base/research/token-optimization-2026) and blocks re-ranking the
// gated tokenopt-* items.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 "RED: TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks"
//	    → C779_001 (+ C779_002 no-fabrication edge). Authored this cycle in
//	      internal/tokenusage/inputcache_fidelity_test.go; observed
//	      PRE-EXISTING GREEN — the claude transcript scanner already extracts
//	      input/cache. Bound as regression so the fix cannot regress the
//	      already-working half; the LIVE defect is AC2/AC3.
//	AC2 "RED: TestScanner_PerDriverCoverageWarnsNotZeros"
//	    → C779_003 (coverage surfaced per driver), C779_004 (negative:
//	      unknown/uncovered driver fails OPEN — no error, explicit uncovered
//	      signal, never a silent zero attributed as covered), C779_005
//	      (bridge engine forwards the launch's CLI/driver into the resolver
//	      Window so per-driver dispatch is possible at all). These name unit
//	      contracts the Builder authors with the new seam (Window carries no
//	      driver today); each predicate stays RED (no PASS marker) until the
//	      named test exists AND passes.
//	AC3 "evolve tokens report shows non-zero input and a real cache-hit ratio
//	    for a soaked batch; coverage line present"
//	    → C779_006 (Coverage: line with phases-with-data/phases-run ratio),
//	      C779_007 (negative: an all-zero window reports 0/N, never claims
//	      coverage). Authored RED this cycle in
//	      cmd/evolve/cmd_tokens_coverage_test.go. The soaked-batch non-zero
//	      half needs a live batch → manual+checklist in test-report.md.
//	AC4 "go test -race PASS; apicover clean"
//	    → every delegated predicate runs under -race; C779_008 runs the whole
//	      touched tokenusage package under -race. apicover runs in the
//	      repo-wide CI-parity gate the audit executes (ADR-0069).
//
// Adversarial axes: negative (C779_004 uncovered-driver must not silently
// zero; C779_007 all-zero window must not claim coverage), edge (C779_002
// absent cache fields stay zero, not fabricated), semantic (extraction vs
// coverage-accounting vs driver-plumbing vs report-rendering are distinct
// behaviors). No source-grep predicates (cycle-85 rule) — every predicate
// exercises the system under test via `go test -race -run '^<name>$'` with a
// verbose "--- PASS:" guard rejecting rename/no-tests-matched silent greens.
```

### `go/acs/cycle779/predicates_test.go:124` — above `func TestC779_007_tokens_report_zero_window_not_covered(t *testing.T) {`

```text
// AC3 negative: an all-zero window (the 2026-07-13 baseline shape) reports
// coverage 0/N — zero-token phases are uncovered, never covered-and-free.
```

### `go/acs/cycle779/predicates_test.go:130` — above `func TestC779_008_tokenusage_package_race_clean(t *testing.T) {`

```text
// AC4: the whole touched tokenusage package passes under -race (the repo-wide
// apicover/CI-parity gate runs in audit per ADR-0069).
```

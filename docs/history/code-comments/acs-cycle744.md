# Comment history: `acs/cycle744`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle744/predicates_test.go:3` — above `package cycle744`

```text
// Package cycle744 materializes the cycle-744 acceptance criteria for the sole
// committed top_n task fleet-config-hot-reload-wave-boundary (triage-report.md
// ## top_n; scout's two selections were deferred by triage, so per R9.3 no
// predicates bind to them and no deferred-floor predicates exist).
//
// DUPLICATE-TASK NOTICE (recorded in test-report.md): the committed carryover
// item duplicates work ALREADY SHIPPED in cycle 739 — the seam
// reloadFleetConfigAtWaveBoundary (go/cmd/evolve/cmd_loop_wave.go), its
// batch-loop wiring (cmd_loop.go: reload before budgetAwareWaveConfig every
// iteration), the unit-test contract (cmd_loop_wave_reload_test.go), and the
// cycle-739 ACS suite all exist in this worktree's base. These predicates are
// therefore expected pre-existing GREEN: they re-pin the committed AC set for
// THIS cycle's audit gate (ACS suites are cycle-scoped) rather than encode new
// RED work for Builder.
//
// AC map (1:1), derived from the top_n task text ("reload fleet policy config
// at each wave boundary … so a mid-batch policy.json commit takes effect at
// the next wave without killing in-flight lanes"):
//
//	AC1 min_lanes committed mid-batch takes effect next wave      → C744_001
//	AC2 count committed mid-batch takes effect next wave
//	    (widen AND narrow-to-sequential edge)                     → C744_002
//	AC3 unchanged policy ⇒ identical config + zero reload noise   → C744_003
//	AC4 malformed policy at boundary holds width + WARNs (neg.)   → C744_004
//	AC5 batch loop invokes the reload seam before quota/budget
//	    sizing at every iteration                                 → manual+checklist (auditor)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract in cmd/evolve, which EXERCISES the SUT (the wave-boundary
// reload seam against real temp policy.json documents) — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```

### `go/acs/cycle744/predicates_test.go:62` — above `func TestC744_001_ReloadsMinLanesAtWaveBoundary(t *testing.T) {`

```text
// AC1 — the incident twin: min_lanes committed between waves is resolved at
// the next wave boundary, logged, and the new floor holds width under a
// quota bench.
```

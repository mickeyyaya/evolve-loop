# Comment history: `acs/cycle986`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle986/predicates_test.go:3` — above `package cycle986`

```text
// Package cycle986 materializes the cycle-986 acceptance criteria for the sole
// fleet lane this cycle is pinned to: recover-false-fail-features-876-897-898
// (goal 1f6d5bf8…). Per R9.3 no predicate here binds to any other lane's items.
//
// Task nature: CONVERGENCE-CONFIRMATION + CLOSEOUT. Scout found the three
// features the clean-exit false-FAIL bug discarded across cycles 862–899 —
// tier-fallback (876), skill-overlay/`/evo:fable` injection (897/884),
// scoped-review (898) — are ALREADY landed on main and green (tier-fallback via
// PR #331 6b4e4096; skill-overlay via PR #333 daf993e8; scoped-review in
// internal/core). The lane has converged. The only residual value is retiring
// the stale inbox item (weight 0.93) that keeps re-selecting completed work and
// stamping the recovery ledger CLOSED with the two real landing SHAs — the exact
// livelock class ADR-0072 exists to end.
//
// The Builder's deliverable (the RED contract — do NOT modify this file):
//
//	B1  remove the stale inbox JSON
//	    `.evolve/inbox/2026-07-17T14-35-00Z-recover-false-fail-features-876-897-898.json`
//	    from the STATE root (main's `.evolve/inbox/`; `.evolve/inbox` is
//	    untracked shared state — writable outside the worktree per the
//	    boundary-only-main-tree-writes rule). Suite reads it via
//	    EVOLVE_PROJECT_ROOT (the STATE root); see stateRoot below.
//	B2  stamp `docs/operations/false-fail-recovery-862-899.md` (a SOURCE doc, so
//	    edited in the WORKTREE) with CLOSED + both landing SHAs 6b4e4096 /
//	    daf993e8, WITHOUT dropping the genuine-FAIL (889/894/895/896)
//	    do-not-land classification.
//
// AC map (1:1 with scout-report.md ## Acceptance Criteria Summary):
//
//	AC1 "stale inbox JSON gone from active root"
//	    → C986_001 (negative/absence): the specific item is absent from the live
//	      inbox root AND no live inbox *.json still carries the lane id. RED now
//	      (the item is present in both the worktree and main inbox roots).
//	AC2 "ledger records CLOSED + both landing SHAs (bare CLOSED fails)"
//	    → C986_002: ledger contains CLOSED and BOTH SHAs, and each cited SHA is
//	      VERIFIED against git as a real landing commit whose subject names the
//	      recovered feature (tier-fallback / skill-overlay). This is what lifts
//	      the check above a degenerate magic-string grep (cycle-85 rule): a bare
//	      "CLOSED", a missing SHA, or a fabricated/wrong SHA all fail. RED now
//	      (ledger status is "QUEUED"; neither SHA appears).
//	AC3 "all 3 recovered features remain green"
//	    → C986_003 (behavioral): runs the recovered features' own tests as
//	      subprocesses across runner/bridge/guards/core and requires each named
//	      top-level PASS marker (a -run that matches zero tests exits 0 — that
//	      gaming vector is rejected by counting the markers). PRE-EXISTING GREEN
//	      by design: this IS the confirmation half of a convergence task, bound
//	      so audit re-proves the landing instead of trusting the scout.
//	AC4 "negative: the genuine FAILs (889/894/895/896) are not resurrected"
//	    → C986_004 (negative/semantic): closing the ledger must NOT flip the
//	      genuine FAILs to recovered — the ledger must STILL classify
//	      889/894/895/896 as GENUINE FAIL / do-not-land, and no live inbox item
//	      may propose LANDING them. Guards the real over-closure regression this
//	      edit risks. Currently GREEN; stays green iff the Builder preserves the
//	      do-not-land record.
//	AC5 "`go vet ./internal/...` clean"
//	    → C986_005 (behavioral): runs `go vet ./internal/...` and requires exit
//	      0. PRE-EXISTING GREEN by design.
//
// Adversarial axes: negative (C986_001 absence; C986_004 no-resurrection),
// edge (C986_003 rejects exit-0-with-zero-matched-tests), semantic (retire vs
// stamp vs preserve-genuine-FAIL are distinct behaviors). No degenerate
// source-grep predicates: C986_002 verifies the cited SHAs against real git
// history, C986_003/005 execute the system under test, C986_001/004 assert on
// real runtime/emitted artifacts (the live inbox, the ledger record).
```

### `go/acs/cycle986/predicates_test.go:82` — above `tierSHA    = "6b4e4096"`

```text
// PR #331 — tier-fallback dispatch swap
```

### `go/acs/cycle986/predicates_test.go:83` — above `overlaySHA = "daf993e8"`

```text
// PR #333 — skill-overlay injection
```

### `go/acs/cycle986/predicates_test.go:96` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the MAIN project root (the STATE root): the ACS suite
// exports EVOLVE_PROJECT_ROOT (issue #12) so `.evolve/` runtime data resolves to
// main, not the worktree; else fall back to the repo root (the redteam idiom).
```

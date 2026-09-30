# Comment history: `acs/cycle1181`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1181/predicates_test.go:3` — above `package cycle1181`

```text
// Package cycle1181 materialises the cycle-1181 acceptance criteria for this
// lane's single triage-COMMITTED (## top_n) task:
//
//	todo-quarantine-dead-lane-code → resolve-todo-quarantine-dead-lane-code
//
// The carryover entry (first seen cycle 1159, cycles_unpicked 3) asks: mark
// whichever of `carryforward-filter-wire-fleet-rebase` / `menu-pass-preserve-
// committed-ids` did NOT land at cycle 1159 with a QUARANTINED-DEAD marker, or
// confirm a no-op if both landed. Scout's forensics say BOTH landed, so the
// verdict is no-op and the deliverable is a durable resolution doc plus the
// sanctioned removal of the entry from the live .evolve/state.json.
//
// Predicate strategy — the failure mode this cycle must avoid is precisely a
// doc that ASSERTS a verdict nothing checked (the D1 HIGH audit note that sank
// cycle 1164: a shipped doc claiming a carryoverTodos removal the live state
// never received). So the predicates split into three independent proofs:
//
//	001 — the doc artifact exists at the canonical path and carries the verdict
//	      and its cited evidence, plus the NEGATIVE half: a no-op verdict means
//	      neither lane's source may actually gain a QUARANTINED-DEAD marker.
//	002 — the doc's no-op verdict is SUBSTANTIATED by driving both cited lanes'
//	      real code (triagecap seed selection; core fleet-rebase classifier) over
//	      isolated temp trees. A doc claiming "both landed" over dead code fails
//	      here even though 001 would pass — this is the anti-assertion predicate.
//	003 — the LIVE state.json actually lost the entry (read independently of the
//	      doc), with anti-gaming guards: the rest of the backlog must survive and
//	      the locked read-modify-write must have bumped stateRevision.
//
// Diversity: 001 positive artifact + a negative source-marker guard, 002 two
// behavioral drivers each with a negative branch (an id that must be dropped /
// a candidate that must classify as Conflict, never as AlreadyLanded), 003 the
// live-state mutation with a wipe-the-list negative.
```

### `go/acs/cycle1181/predicates_test.go:59` — above `const stateRevisionPinned = 1753`

```text
// stateRevisionPinned is .evolve/state.json:stateRevision as observed when these
// predicates were authored (TDD phase, cycle 1181).
//
// CORRECTED POST-AUDIT (cycle 1233). Cycle 1181 failed audit because it asserted
// stateRevision MUST BE UNCHANGED. While `evolve carryover apply-decisions --apply`
// leaves stateRevision untouched, the cycle boundary itself unconditionally bumps
// stateRevision via `persistCycleEndState` (storage/updatestate.go). An equality
// assertion is therefore unsatisfiable after the cycle ends, causing permanent
// test failures on all subsequent cycles. The guard is now `>=` the baseline.
```

### `go/acs/cycle1181/predicates_test.go:77` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the MAIN project root (the STATE root): .evolve/ runtime
// data lives on main, not in the cycle worktree (issue #12 dual-root pattern).
// The ACS suite exports EVOLVE_PROJECT_ROOT; fall back to the repo root.
```

### `go/acs/cycle1181/predicates_test.go:90` — above `func TestC1181_001_ResolutionDocRecordsNoOpVerdictWithEvidence(t *testing.T) {`

```text
// TestC1181_001_ResolutionDocRecordsNoOpVerdictWithEvidence pins AC-1.
//
// The carryover entry is retired by a DURABLE artifact, not by a commit message:
// the next cycle that meets this question must be able to read why it is closed.
// So the doc must exist at the canonical path, name both scoped ids, state the
// no-op verdict, and cite the two landed-code evidence points scout verified
// (`lane_menu.go` for menu-pass-preserve-committed-ids; `carryforward_filter.go`
// / commit 9eacd83f for carryforward-filter-wire-fleet-rebase) plus the note
// that the entry's either/or framing is itself stale.
//
// Negative half: "no-op" is a claim with a physical consequence — if it is true,
// no QUARANTINED-DEAD marker is warranted, so neither cited source file may
// actually carry one. A cycle that hedged by writing the doc AND stamping a
// marker fails here.
```

### `go/acs/cycle1181/predicates_test.go:154` — above `func TestC1181_002_BothScopedLanesAreLiveNotDead(t *testing.T) {`

```text
// TestC1181_002_BothScopedLanesAreLiveNotDead is the cycle-1181 CRUX: it makes
// the doc's central claim falsifiable instead of merely asserted.
//
// A resolution doc is only worth landing if "both landed" is TRUE. Predicate 001
// would pass on a doc that says so over dead code; this one calls the real
// surfaces and asserts their landed behavior:
//
//	(a) menu-pass-preserve-committed-ids — triagecap.SelectWaveSeedMenus must
//	    route committed candidates through the committed-AWARE widen path, so a
//	    committed id survives into the seed even when its weight is below the
//	    backlog's top entries. The committed-blind SelectFleetWidthTopN path
//	    (the pre-fix behavior) drops it.
//	    Negative: an id the inbox lifecycle has already CONSUMED must still be
//	    pruned — "preserve committed" must not have degraded into "preserve
//	    everything", which would re-pin consumed work (the cycle-1116 re-pin).
//
//	(b) carryforward-filter-wire-fleet-rebase — core.ClassifyFleetRebaseCandidate
//	    must exist and classify over a real git tree: an already-landed candidate
//	    reads AlreadyLanded (short-circuit, the 948 duplicate-waste fix).
//	    Negative: a genuinely conflicting candidate must read Conflict, NEVER
//	    AlreadyLanded — mislabelling a conflict as landed silently drops real
//	    overlapping work, so the negative is the load-bearing half.
```

### `go/acs/cycle1181/predicates_test.go:244` — above `func TestC1181_003_CarryoverEntryRetiredFromLiveState(t *testing.T) {`

```text
// TestC1181_003_CarryoverEntryRetiredFromLiveState pins AC-2, read independently
// of the doc.
//
// This is the exact gap that sank cycle 1164 (D1 HIGH): the resolution existed
// as prose while the live .evolve/state.json still carried the entry, so the
// next triage re-picked it. The predicate therefore parses the LIVE state file
// under the STATE root and asserts the id is gone.
//
// Two anti-gaming guards, because "make the id absent" has a trivially wrong
// implementation:
//
//	(a) the rest of the backlog must survive — truncating or wiping
//	    carryoverTodos would green a naive absence check while destroying 60
//	    other tracked items;
//	(b) statemapRevision must have strictly advanced past the pre-cycle baseline,
//	    which the sanctioned locked read-modify-write
//	    (`evolve carryover apply-decisions --apply`) bumps.
//	    stateRevision is no longer pinned to equality because it bumps unconditionally
//	    at cycle end, so it must be >= the baseline.
```

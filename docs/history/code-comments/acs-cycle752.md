# Comment history: `acs/cycle752`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle752/predicates_test.go:3` — above `package cycle752`

```text
// Package cycle752 materializes the cycle-752 acceptance criteria for the sole
// committed top_n task inbox-promotion-requires-landed-ship (triage-report.md
// ## top_n; scout's three selections were out of this fleet lane's assigned
// set and dropped, so per R9.3 no predicates bind to them and no
// deferred-floor predicates exist).
//
// Task source: .evolve/inbox/2026-07-07T19-32-00Z-inbox-promotion-requires-
// landed-ship.json (weight 0.90). Incident: cycle-598 (batch b63fyf1ai) — ship
// push rejected, recovery ended needs-reaudit, cycle still reported PASS, and
// the inbox item was promoted to processed/ although the work never landed on
// any ref. Verdict is not delivery.
//
// AC map (1:1), derived from the inbox item's acceptance list. The landing
// gate itself landed in a prior cycle (postship.go isLanded + inboxmover
// IsLandedFn), so AC1 predicates pin PRE-EXISTING GREEN unit contracts; the
// residual RED gap this cycle is AC2's "returns with a retry note":
//
//	AC1 promotion refused when the ship commit is absent from main
//	    ancestry; twin: landed SHA promotes          → C752_001 + C752_002 (twin)
//	                                                   + C752_003 (unit-gate reroute)
//	AC2 reaudit-recovery terminal without landing releases the item
//	    back with a note                             → C752_004 (never promotes,
//	                                                   pre-existing GREEN)
//	                                                   + C752_005 (retry note, RED)
//	                                                   + C752_006 (negative anti-stamp)
//	AC3 go vet, -race, apicover -enforce green       → manual+checklist (auditor
//	    runs the repo-wide CI-parity gates on touched pkgs per ADR-0069)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract in the target package, which EXERCISES the SUT (scripted
// git runner seams, real temp inbox dirs, real ledger.jsonl writes) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```

### `go/acs/cycle752/predicates_test.go:88` — above `func TestC752_004_NeedsReauditTerminalNeverPromotes(t *testing.T) {`

```text
// AC2 (refusal half) — the cycle-598 regression shape itself:
// RepairOutcome=="needs-reaudit" with an unlanded commit never promotes,
// regardless of any upstream PASS verdict. Pre-existing GREEN pin.
```

### `go/acs/cycle752/predicates_test.go:95` — above `func TestC752_005_UnlandedReleaseCarriesRetryNote(t *testing.T) {`

```text
// AC2 (note half — the cycle-752 RED anchor) — an unlanded ship releases the
// item back to the inbox root AND leaves a durable per-item unlanded retry
// note in the lifecycle ledger, so triage/operators can tell a delivery
// failure from an ordinary residual drain.
```

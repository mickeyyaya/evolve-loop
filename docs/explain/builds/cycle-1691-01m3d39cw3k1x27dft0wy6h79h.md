# Build Explanation — Cycle 1691

## Build Binding
- Cycle: 1691
- Base SHA: f08ae603833fca38dcf70e1053db59681c21ab5c

## Summary
Ship-time inbox consumption stays gated on the verdict string `PASS`, and the
cycle now pins that contract with tests. The 2026-08-16 inbox item described
a WARN verdict with `red_count:0` shipping without retiring its inbox item.
That report predates #535 (`09d32a19`, 2026-09-09). Since #535 the cycle ship
path refuses that artifact before consumption runs. No production code
behavior changes: the only production edits correct stale comments in
`consume.go` and `postship.go`.

## Rationale
The first attempt in this cycle re-keyed the gate on `red_count==0`. The audit
rejected that (cycle-1691 defect-ledger.json, H1). `red_count` is a weaker key
than the verdict string. `acsrunner` writes `verdict:"FAIL"`,
`ship_eligible:false`, `red_count:0` for a suite that never finished
(`go/internal/acsrunner/runner.go:158-162`). The manual ship class consumes
with a `WorkspacePath` and has no `ReadVerdict` reader, so a red_count gate
would retire unfinished work. On the cycle path, `acssuite.ReadVerdict`
(`go/internal/acssuite/evidence.go:79-84`) requires `verdict=="PASS"` exactly
when `red_count==0`, so the PASS-string gate and a red_count gate already agree
there. No writer emits WARN. Restoring the PASS-string gate is therefore both
the smallest change and the only one that is correct on both ship classes. It
also keeps the in-commit gate in lockstep with postship's `landedPASS` gate
(`postship.go:237`).

## Changed Areas
- `go/internal/phases/ship/consume.go` — doc comment only: it now states that verdict PASS is the sole consumption authority, why red_count alone is weaker, and that the manual path has no other guard; the gate code is byte-identical to the base.
- `go/internal/phases/ship/postship.go` — comment only: the non-PASS branch no longer claims WARN ships; it describes non-PASS verdicts as FAIL, unfinished, or unknown evidence that leaves work pickable.
- `go/internal/phases/ship/consume_gate_authority_test.go` — adds a unit test proving the cycle path's EGPS reader refuses WARN/red_count:0 and FAIL/ship_eligible:false/red_count:0, with a PASS positive control, so the cycle-path guarantee is pinned.
- `go/internal/phases/ship/consume_integration_test.go` — corrects the WARN test's comment and adds three integration tests: a WARN-with-reds cycle ship is not consumed, a manual ship with a real acsrunner unfinished verdict keeps its item pickable, and a parallel 10-shape table on both classes pins consumption to the postship PASS expression.
- `.evolve/evals/warn-ship-consumption-gap.md` — the task eval, which records the #535 closure and the verdict-string contract its graders check.
- `.evolve/inbox/2026-08-16T19-30-00Z-warn-ship-consumption-gap.json` (deleted) — retires the inbox filing: the #535 fix already closed the WARN-ships-but-stays-pickable gap this record describes, so the item is resolved rather than actioned as a code change.
- `.evolve/inbox/consumed/2026-08-16T19-30-00Z-warn-ship-consumption-gap.json` (added) — the consumed-record copy of the same item, carrying a `consumed.via: "ship"` annotation so triage history shows this filing was retired by this cycle, not silently dropped.

## Design Decisions
Two alternatives were rejected:
- Gating on `ship_eligible==true`. It adds a second authority field that
  legacy PASS records (`{"verdict":"PASS"}`) do not carry.
- Unifying `postship.go` behind a new shared predicate. Both gates already key
  on the same expression, `workspaceACSVerdict(...) == "PASS"`, and
  `TestConsumeGate_OnlyVerdictPASSConsumes` asserts they agree on every row.

The consumed-record `verdict` annotation from the first attempt was dropped.
Under a PASS-only gate it would always read `PASS`.

## Verification
The eval graders in `.evolve/evals/warn-ship-consumption-gap.md` all pass:
- `TestManualShip_NonShippableVerdictKeepsItemPickable`
- `TestConsumeGate_OnlyVerdictPASSConsumes` (20 subtests)
- the two `TestShipFromWorktree_Warn*` tests
- `TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption`
- the PASS-path cycle and manual tests

`test-red-output.txt` shows the new tests failing against the first attempt's
red_count gate. `gofmt`, `go vet`, and `go test -count=1 ./...` were run from
`go/`.

## Compatibility
Ship behavior is identical to the base commit on both classes: only
`verdict=="PASS"` consumes. The first attempt's widening (non-PASS with
`red_count:0` consuming) never shipped and is fully reverted. No schema
changes.

## Limitations
`TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption` pins the
cycle-path guarantee to `acssuite.ReadVerdict`. If a future writer starts
emitting WARN as a shippable verdict, that test and this gate must be
revisited together.

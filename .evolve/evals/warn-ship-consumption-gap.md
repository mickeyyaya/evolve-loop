---
score_cap:
  - criterion: "A manual ship whose workspace verdict is non-shippable (acsrunner: verdict FAIL, ship_eligible false, red_count 0) lands the code but never retires its inbox item"
    max_if_missing: 9
    evidence: "cd go && go test -tags integration -race -count=1 -run TestManualShip_NonShippableVerdictKeepsItemPickable ./internal/phases/ship/"
  - criterion: "In-commit consumption fires exactly when acs-verdict.json says verdict PASS, for both cycle and manual classes, in lockstep with postship's landedPASS gate"
    max_if_missing: 8
    evidence: "cd go && go test -tags integration -race -count=1 -run TestConsumeGate_OnlyVerdictPASSConsumes ./internal/phases/ship/"
  - criterion: "A WARN verdict never consumes, whatever its red_count"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -race -count=1 -run 'TestShipFromWorktree_WarnVerdictDoesNotConsume|TestShipFromWorktree_WarnWithRedsDoesNotConsume' ./internal/phases/ship/"
  - criterion: "The cycle ship's EGPS reader refuses a WARN or FAIL verdict with red_count 0, so neither reaches cycle-class consumption"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption ./internal/phases/ship/"
  - criterion: "A PASS ship still retires its inbox item in the landing commit (cycle and manual paths unchanged)"
    max_if_missing: 7
    evidence: "cd go && go test -tags integration -race -count=1 -run 'TestShipFromWorktree_ConsumesCommittedItemInTheShipCommit|TestManualShip_ClosesInboxItemInShipCommit' ./internal/phases/ship/"
---

# Eval: warn-ship-consumption-gap — the verdict string PASS is the only consumption authority

> The 2026-08-16 inbox item reported a WARN acs verdict with `red_count:0`
> shipping at cycle-1494 (commit 4f133efe) without retiring its inbox item.
> That report predates #535 (`09d32a19`, 2026-09-09). Since #535,
> `acssuite.ReadVerdict` (go/internal/acssuite/evidence.go:79-84) requires
> `verdict=="PASS"` exactly when `red_count==0`. The cycle ship runs it in
> `verifyClass` → `verifyPredicateReceipt` → `checkEGPSGate`
> (go/internal/phases/ship/audit.go) before `atomicShip`, so a WARN +
> `red_count:0` artifact cannot reach `consumeCommittedItems` on the cycle
> path. No verdict writer emits WARN at all: acssuite and acsrunner both write
> PASS or FAIL, derived from their own counts.
>
> The first repair attempt (cycle 1691, audit round 1) re-keyed the gate on
> `red_count==0`. The audit rejected it (records H1/H2/M1 in the cycle-1691
> defect-ledger.json). red_count is a weaker key than the verdict string:
> acsrunner writes `red_count:0` beside `verdict:"FAIL"`,
> `ship_eligible:false` for a suite that never finished, and the manual ship
> path has no `ReadVerdict` gate. A red_count-keyed gate therefore retires
> unfinished work. This eval pins the verdict-string contract:
> - only PASS consumes;
> - the in-commit gate and postship's landedPASS gate agree;
> - the WARN quadrant stays unreachable on the cycle path.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| non-shippable-manual | Real acsrunner FAIL/ship_eligible:false/red_count:0 verdict: manual ship lands, item stays tracked and pickable | 9/10 | `go test -tags integration -run TestManualShip_NonShippableVerdictKeepsItemPickable ./internal/phases/ship/` |
| pass-only-lockstep | 10 verdict shapes × {cycle, manual}: consume iff verdict PASS, equal to `workspaceACSVerdict(ws)=="PASS"` (postship.go landedPASS) | 8/10 | `go test -tags integration -run TestConsumeGate_OnlyVerdictPASSConsumes ./internal/phases/ship/` |
| warn-never-consumes | WARN with red_count 0 or 2 leaves the item pickable | 6/10 | `go test -tags integration -run 'TestShipFromWorktree_Warn…' ./internal/phases/ship/` |
| cycle-path-unreachable | `checkEGPSGate` refuses WARN/red_count:0 and FAIL/ship_eligible:false/red_count:0; PASS twin accepted (positive control) | 6/10 | `go test -run TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption ./internal/phases/ship/` |
| pass-path-unchanged | PASS ships consume in the landing commit, cycle and manual | 7/10 | `go test -tags integration -run 'TestShipFromWorktree_ConsumesCommittedItemInTheShipCommit\|TestManualShip_ClosesInboxItemInShipCommit' ./internal/phases/ship/` |

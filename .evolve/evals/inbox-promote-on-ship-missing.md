---
score_cap:
  - criterion: "On a successful cycle ship, claimed items in processing/<cycle>/ that appear in triage-decision.json top_n[] are moved to processed/cycle-N/"
    max_if_missing: 9
    evidence: "cd go && go test -run '^TestInboxPromote_ShippedCycleMovesToProcessed$' -v ./internal/phases/ship/... 2>&1 | grep -q 'PASS'"
  - criterion: "On a successful cycle ship, items claimed into processing/<cycle>/ that do NOT appear in triage-decision.json top_n[] are released back to inbox root"
    max_if_missing: 9
    evidence: "cd go && go test -run '^TestInboxPromote_UnfinishedClaimReleasedOnShip$' -v ./internal/phases/ship/... 2>&1 | grep -q 'PASS'"
  - criterion: "On cycle FAIL/abort (no ship), all items in processing/<cycle>/ are released back to inbox root"
    max_if_missing: 10
    evidence: "cd go && go test -run '^TestInboxRelease_FailedCycleReleasesAllClaimed$' -v ./internal/inboxmover/... 2>&1 | grep -q 'PASS'"
  - criterion: "Double-claim race (file already moved before release) is treated as WARN not an error"
    max_if_missing: 7
    evidence: "cd go && go test -run '^TestInboxRelease_DoubleClaimRaceIsWarn$' -v ./internal/inboxmover/... 2>&1 | grep -q 'PASS'"
---

# Eval: Inbox promote on ship missing (go-native postship + fail-release)

> SYSTEMIC: claimed inbox items strand in .evolve/inbox/processing/cycle-N/
> forever because the promote-to-processed step died with legacy ship.sh (v12
> flag day) and the go-native ship never inherited the fail-release path.
> agents/evolve-triage.md Step 0 documents the contract: claim half works
> (inbox-mover claim), promote half is incomplete (no fail-release wiring).
>
> Fix: after a successful cycle commit (postship.go), for each claimed item in
> processing/<this-cycle>/ — if id appears in triage-decision.json top_n[] AND
> cycle shipped → move to processed/cycle-N/; else → release back to inbox root.
> On cycle FAIL/abort terminal: release ALL of processing/<cycle>/ back to inbox
> root so the next triage scan picks them up again (Step 0a reads maxdepth-1).
> Double-claim race (file already gone) → WARN not error.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| positive/success | shipped+top_n → processed/ | 9/10 | `TestInboxPromote_ShippedCycleMovesToProcessed` PASS |
| unfinished-release | shipped but not in top_n → back to inbox | 9/10 | `TestInboxPromote_UnfinishedClaimReleasedOnShip` PASS |
| fail-release | cycle FAIL/abort → all claimed → back to inbox | 10/10 | `TestInboxRelease_FailedCycleReleasesAllClaimed` PASS |
| race-safe | double-claim → WARN not error | 7/10 | `TestInboxRelease_DoubleClaimRaceIsWarn` PASS |

## Acceptance Criteria

### C1: Shipped+top_n item → processed/cycle-N/ [code]
```bash
cd go && go test -run '^TestInboxPromote_ShippedCycleMovesToProcessed$' -v ./internal/phases/ship/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Positive case:** An item in `processing/cycle-307/item.json` whose id appears in `triage-decision.json.top_n[]` after a successful ship must end up in `processed/cycle-307/item.json` (or with SHA prefix).

### C2: Unfinished claim (not in top_n) released back to inbox root [code]
```bash
cd go && go test -run '^TestInboxPromote_UnfinishedClaimReleasedOnShip$' -v ./internal/phases/ship/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Edge case:** A cycle may claim an item (Step 0a) but then drop it from the final triage. That item should be visible to the NEXT cycle's triage scan, not stranded in processing/.

### C3: Failed/aborted cycle releases all claimed items back to inbox [code]
```bash
cd go && go test -run '^TestInboxRelease_FailedCycleReleasesAllClaimed$' -v ./internal/inboxmover/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Negative case / fail path:** When a cycle ends without a ship (FAIL, abort, loop-fatal), all items still in `processing/<cycle>/` must be moved back to inbox root so future triage scans find them. A release that silently ignores the fail terminal → stranded items (the original bug).

**Gaming fake named:** A no-op release that leaves files in processing/ cannot pass the test (test verifies item is no longer in processing/ and is back in inbox/).

### C4: Double-claim race is WARN not error [code]
```bash
cd go && go test -run '^TestInboxRelease_DoubleClaimRaceIsWarn$' -v ./internal/inboxmover/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**OOD/race case:** If a file has already been moved (concurrent release, manual operator rescue) the release function must not fail — log a WARN and continue.

## Grader type summary
- C1–C4: `[code]` — all criteria are executable Go test assertions
- No `[model]` or `[human]` graders needed; behavior is deterministic

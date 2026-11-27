---
score_cap:
  - criterion: "A second delivery with the same patch ids is a no-op at the write-once writer"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestWriteOnce_RowRa_ASecondDeliveryWithTheSamePatchIDsIsANoOp' ./internal/integration"
  - criterion: "A second delivery with new patch ids is refused with a typed error naming the member and leaves the record untouched"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestWriteOnce_RowRb_|TestWriteOnce_PatchIDsAreComparedAsAWholeSetNotByPrefix' ./internal/integration"
  - criterion: "At enforce the driver never replaces a member of any status with a new patch set, and a merged member lands once"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestDriverAdmits_RowRb_|TestDriverAdmits_RowRc_' ./internal/integration"
  - criterion: "The guard table holds for every ship path and both race rows"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestWriteOnce_GuardTable_' ./internal/integration"
  - criterion: "A ship retry after a delivery reads as delivered and a new patch set is a permanent, non-transient refusal"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags integration -run 'TestDeliverWriteOnce_' ./internal/phases/ship"
  - criterion: "A ship retry that crosses a second boundary keeps the patch ids of the delivered build, so the consume stamp never decides the write-once compare"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags integration -run 'TestDeliverWriteOnce_A(ShipRetryUnderANewSHA|ResumedShipOfTheSameBuild|RetryAfterTheLaneRefFailed)' ./internal/phases/ship"
  - criterion: "Row 10d: a member delivered again under a new SHA with the same patch ids lands once and nothing is ejected"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestDriverAdmits_Row10d_' ./internal/integration"
---

# Eval: Write-once delivery at enforce, admits defence and ship ErrRecordExists handling

> Pins rows R-a to R-c of card section 10 of docs/architecture/integration-driver-enforce.md and journey rows 10, 10b, 10d and 10e of section 11.
> Source incident: pools 2373 and 2378, where each ship retry after a delivery made a wave not clean (cycle 2653 task train-write-once-defence).
> Row 10c (the lane Heads ejection) is outside this eval: the card table assigns lane Heads to part 2 and part 7 (this item) holds the write-once defence.
> Cycle 2657 root cause of the two failing ship-retry tests of cycle 2653: `itemConsumer.stage` stamps the consumed inbox item with `time.Now()`, the item rides the ship commit, so the patch ids of two ships of one cycle differ when they cross a second boundary.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-op | same patch ids write nothing and fail nothing | 6/10 | `TestWriteOnce_RowRa_*` |
| typed refusal | new patch ids refuse with a member-naming error | 7/10 | `TestWriteOnce_RowRb_*` |
| admits defence | no member of any status takes new patches at enforce | 7/10 | `TestDriverAdmits_RowRb_*`, `TestDriverAdmits_RowRc_*` |
| guard table | every ship path and race row keeps the first record | 7/10 | `TestWriteOnce_GuardTable_*` |
| ship retry | a retry after a delivery is delivered, not a failure | 8/10 | `TestDeliverWriteOnce_*` |
| stable patch ids | a retry across a second boundary keeps the delivered patch ids | 8/10 | `TestDeliverWriteOnce_A(ShipRetryUnderANewSHA|ResumedShipOfTheSameBuild|RetryAfterTheLaneRefFailed)` |
| row 10d | a re-delivery under a new SHA lands once, ejects nothing | 6/10 | `TestDriverAdmits_Row10d_*` |

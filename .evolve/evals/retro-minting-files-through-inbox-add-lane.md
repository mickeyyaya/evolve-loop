---
score_cap:
  - criterion: "faillearn, ciwatch ci-red and base-red items carry the full filing shape and the filing writer names them"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_00[1-5]'"
  - criterion: "a minted item that repeats a filed id is an idempotent skip"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_00[6-9]'"
  - criterion: "concurrent mints of one id file exactly one item and wait for the inbox bump lock"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_01[0-5]'"
  - criterion: "each minter degraded path is unchanged when the writer refuses or faults"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_01[6-9]|TestC2035_02[01]'"
  - criterion: "the inbox filing writer package is enrolled in apicover and the inbox writers contract"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_02[23]' && go test -count=1 ./internal/inboxfile"
  - criterion: "the core retrospective remediation tests count inbox items, not the writer's lock file"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_024'"
  - criterion: "the lane leaves the out-of-scope wave process start time test untouched"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2035 -run 'TestC2035_025'"
---

# Eval: faillearn and ciwatch file through the inbox filing writer

> Pins the lane part of the retro-minting item. Source incident: cycle 2030 audit FAIL (H1: the inbox writers contract did not enroll the new writer; M2: the base-red minter still wrote inbox JSON itself).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| filing-shape | every minter writes the full filing shape under the writer's file name | 4/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_00[1-5]` |
| idempotent-skip | a repeated id is a skip | 6/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_00[6-9]` |
| lock-and-race | one item under concurrency, lock honored | 5/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_01[0-5]` |
| degraded-path | refused or faulted writer keeps each minter's old behavior | 6/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_01[6-9]` |
| enrollment | apicover and writers contract enroll internal/inboxfile | 6/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_02[23]` |
| core-retro-helper | core remediation tests hold with the writer's inbox layout | 5/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_024` |
| scope-fence | cmd_wave_pid_integration_test.go unchanged from the base | 6/10 | `go test -tags acs ./acs/cycle2035 -run TestC2035_025` |

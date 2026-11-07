---
score_cap:
  - criterion: "A minor release with no content review of its own is refused by preflight, and one with its own review passes"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2210_001_|TestC2210_002_|TestC2210_004_' ./acs/cycle2210"
  - criterion: "A replayed run of an earlier release, a run of another seat and a run with a verdict other than PASS are refused"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2210_005_|TestC2210_006_|TestC2210_007_' ./acs/cycle2210"
  - criterion: "The bypass is a parameter of evolve release and writes its reason to the release journal"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2210_008_|TestC2210_009_' ./acs/cycle2210"
  - criterion: "A patch release needs no review and the review record is outside the version bump set"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2210_003_|TestC2210_010_' ./acs/cycle2210"
---

# Eval: release content review gate

> Pins the preflight gate of plan section 4.3 (docs/plans/release-keeps-site-current-2026-11.md). Source incident: on 2026-11-06 the public site stated v22.27.0 while the release was v23.0.0, and no code checked that a content review ran. Two architecture reviews refused the first gate design, so the gate binds the record to the release, the seat, the verdict and the previous tag, and reads the committed file.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | Release A passes, minor release B with no new review is refused | 4/10 | `TestC2210_001_`, `TestC2210_002_` |
| replay and seat | Replay, other seat, non-PASS verdict, unknown run and uncommitted record are refused | 5/10 | `TestC2210_005_` to `TestC2210_007_` |
| bypass | `--skip-content-review <reason>` reaches preflight and journals the reason | 5/10 | `TestC2210_008_`, `TestC2210_009_` |
| preservation | Patch release is not gated; record is not in the bump set | 7/10 | `TestC2210_003_`, `TestC2210_010_` |

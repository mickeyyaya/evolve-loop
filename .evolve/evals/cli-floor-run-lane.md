---
score_cap:
  - criterion: "evolve floor run --tier fmt flags a file gofmt -s would rewrite and exits 1"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1962_001|TestC1962_002|TestC1962_003' ./acs/cycle1962"
  - criterion: "A red tier exits 1 naming it and the other tiers still run and report"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1962_004|TestC1962_005|TestC1962_006' ./acs/cycle1962"
  - criterion: "A tier that cannot run or an unknown tier exits 2"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1962_007|TestC1962_008' ./acs/cycle1962"
  - criterion: "Tier commands match the steps of the workflows required.yml calls"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestFloorRun_' ./cmd/evolve/"
---

# Eval: evolve floor run, the local CI floor

> Pins the `evolve floor run [--tier ...] [--root DIR] [--dry-run]` contract: one
> verdict line per tier (PASS, RED or ERROR), exit 1 on any red, exit 2 when a tier
> cannot run, every selected tier runs even after a red one, and each tier's command
> equals the step CI runs. Source incident: skipped local tiers shipped CI reds
> (#711 missed e2e; a comment-reduction round missed the size ratchet and
> acs/regression). Cycle 1962.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fmt-flags-rewrite | fmt tier is red for a file gofmt -s rewrites | 4/10 | `go test -tags acs -run TestC1962_001 ./acs/cycle1962` |
| red-does-not-stop-run | a red tier is named and later tiers report | 5/10 | `go test -tags acs -run TestC1962_004 ./acs/cycle1962` |
| exit-2-contract | unrunnable or unknown tier exits 2 | 6/10 | `go test -tags acs -run TestC1962_007 ./acs/cycle1962` |
| ci-parity | tier commands equal required.yml's steps | 5/10 | `go test -run TestFloorRun_TierCommandsMatchRequiredCI ./cmd/evolve/` |

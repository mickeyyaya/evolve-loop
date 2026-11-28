---
score_cap:
  - criterion: "The cmd/evolve baseline holds none of the 13 slice rows (no-assertion, asserts-own-setup, unverified-fake, mock-call-order)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2680_001' ./acs/cycle2680"
  - criterion: "The testaudit analyzer reports none of the 13 slice findings for cmd/evolve"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2680_002' ./acs/cycle2680"
  - criterion: "The cmd/evolve baseline and the analyzer agree, and the baseline holds at most 169 rows"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2680_003' ./acs/cycle2680"
  - criterion: "A rewritten test fails on a production mutant, so it asserts behavior (absorbed-fail soft path, resupply depth, subcommand refusal, drained family, Argon window, sync-main replay, quarantine record, stamp landing)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2680_00[4-9]|TestC2680_01[01]' ./acs/cycle2680"
  - criterion: "The rewritten tests and the consumers of the lane and detach helpers still pass"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2680_01[23]' ./acs/cycle2680"
---

# Eval: Tautology findings of cmd/evolve, slice 1 (T6)

> Pins the first slice of the test-audit T6 work for go/cmd/evolve. The slice removes 13 baseline rows of the
> rules no-assertion, asserts-own-setup, unverified-fake and mock-call-order by rewriting tests only. Each
> rewritten test must fail on a production mutant that the predicates apply with go test -overlay. Source
> incident: cycle 2680, inbox item test-audit-t6c-tautology-cmd.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| baseline-rows | the 13 rows leave the baseline | 5/10 | `TestC2680_001` |
| analyzer-clean | the analyzer reports none of the 13 | 5/10 | `TestC2680_002` |
| ratchet-agree | baseline and analyzer agree, no growth | 6/10 | `TestC2680_003` |
| mutant-kill | each rewritten test fails on a mutant | 6/10 | `TestC2680_004` to `TestC2680_011` |
| preservation | rewritten tests and helper consumers pass | 6/10 | `TestC2680_012`, `TestC2680_013` |

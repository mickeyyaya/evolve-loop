---
score_cap:
  - criterion: "A replay of cycle 1637's sequence ships on the first recovery, and a byte-divergent rebase does not carry"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestCycle1637Replay_ShipsOnFirstRecovery|TestCycle1637Replay_ByteDivergentRebaseDeclinesCarry)$' ./internal/core"
  - criterion: "The replay is red on a pre-carry base (identityCarryForward declines every carry)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1931_002 ./acs/cycle1931"
  - criterion: "docs/incidents/ holds a record for cycles 1637 and 1691 that names the replay test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1931_003 ./acs/cycle1931"
  - criterion: "REGRESSION-COVERAGE-INDEX.md has a covered row that names the replay test and links the record"
    max_if_missing: 6
    evidence: "grep -q 'carry_chase_replay_test.go::TestCycle1637Replay_ShipsOnFirstRecovery' docs/incidents/REGRESSION-COVERAGE-INDEX.md"
---

# Eval: Cycle 1637's rebase chase has a replay test, an incident record and an index row

> Pins the end-to-end replay of cycle 1637: audit PASS, then peers move main
> again and again, and the lane must ship on the first recovery through the
> ADR-0105 identity carry instead of a Build and Audit chase. Source incident:
> cycle 1637 (21 phases, five rebuild/re-audit rounds, retro FAIL on correct
> work) and cycle 1691 (the same path, survived only because no peer was in
> flight). Inbox item carry-chase-replay-and-incident-record, cycle 1931.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| replay-ships | identical rebase ships on the first recovery; divergent does not | 8/10 | `go test -run TestCycle1637Replay ./internal/core` |
| red-on-pre-carry | the replay fails when the carry is disabled | 8/10 | `TestC1931_002` |
| incident-record | record for 1637 and 1691, STE-clean, links resolve | 6/10 | `TestC1931_003` |
| index-row | covered row names the replay and links the record | 6/10 | `grep` on the index |

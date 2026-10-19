---
score_cap:
  - criterion: "evolve handoff check on the cycle 1950 fixture prints blocked and exits 1"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestHandoffCheck_Cycle1950PrintsBlockedAndExits1' ./cmd/evolve"
  - criterion: "evolve handoff check on the cycle 1946 fixture prints blocked with the protected path"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestHandoffCheck_Cycle1946PrintsBlockedWithTheProtectedPath' ./cmd/evolve"
  - criterion: "a blocked claim without proof, a stale ledger and an unresolved evidence pointer never print blocked"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestHandoffCheck_UnmetAsksNameTheIdsToRetry|TestHandoffCheck_AnInvalidLedgerIsNamedAndExits1' ./cmd/evolve"
  - criterion: "--json prints one JSON object on every exit: 0, 1, 2 and 10"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestHandoffCheck_JSONExitFieldMatchesTheExitCodeOnEveryExit|TestHandoff_JSONPrintsOneObjectOnAUsageError' ./cmd/evolve"
  - criterion: "the exit codes are 0 pass, 1 blocked or unmet, 2 IO error and 10 usage"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestHandoff_UsageErrorsExit10|TestHandoffCheck_AnUnreadableInputExits2|TestHandoffCheck_PassExitsZero' ./cmd/evolve"
  - criterion: "handoff show lists the result of each supported pair and writes nothing"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestHandoffShow_|TestHandoff_VerbsWriteNothingAndRepeatTheSameAnswer' ./cmd/evolve"
  - criterion: "the verbs are in the CLI registry, the usage text and docs/operations/runtime-reference.md"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference' ./cmd/evolve"
  - criterion: "the built binary meets the same acceptance end to end"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle1976"
---

# Eval: Phase handoff H7, evolve handoff check and evolve handoff show

> Pins the CLI of component H7 of ADR-0141. `evolve handoff check --cycle N --pair P` runs the deterministic
> layer and the exit decision on the ledger of a run directory. It replays the blocked ends of cycles 1946
> (a protected path) and 1950 (two predicates that conflict) in round 1. Source incident: cycles 1946 and
> 1950 spent three build rounds on a request that no output could meet.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| replay-1950 | the conflict of cycle 1950 prints blocked, exit 1 | 3/10 | `go test -run TestHandoffCheck_Cycle1950PrintsBlockedAndExits1 ./cmd/evolve` |
| replay-1946 | the protected path of cycle 1946 prints blocked | 3/10 | `go test -run TestHandoffCheck_Cycle1946PrintsBlockedWithTheProtectedPath ./cmd/evolve` |
| no-false-block | a claim without proof stays unmet | 5/10 | `go test -run TestHandoffCheck_UnmetAsksNameTheIdsToRetry ./cmd/evolve` |
| json-every-exit | one JSON object on exits 0, 1, 2, 10 | 5/10 | `go test -run TestHandoff_JSONPrintsOneObjectOnAUsageError ./cmd/evolve` |
| exit-codes | 0 pass, 1 blocked, 2 IO, 10 usage | 5/10 | `go test -run TestHandoff_UsageErrorsExit10 ./cmd/evolve` |
| registered | registry, usage, runtime-reference | 6/10 | `go test -run TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference ./cmd/evolve` |
| end-to-end | the built binary meets the acceptance | 6/10 | `go test -tags acs ./acs/cycle1976` |

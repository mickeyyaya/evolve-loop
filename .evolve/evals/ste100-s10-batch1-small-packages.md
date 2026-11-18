---
score_cap:
  - criterion: "ste-lint --strict --go reports no finding in the six batch-1 packages"
    max_if_missing: 4
    evidence: "cd go && ! go run ./cmd/evolve docs ste-lint --strict --go 2>&1 | grep -E 'go/internal/(campaign|codequality|fleet|gc|inboxbatch|inboxcurator)/'"
  - criterion: "No test in the six batch-1 packages pins the old wording of the rewritten messages"
    max_if_missing: 6
    evidence: "cd go && ! grep -lE 'could not be pinned|could not read the tree|commit them via|failed after %d attempt|would rank below every class|explain the console route' internal/campaign/*_test.go internal/codequality/*_test.go internal/fleet/*_test.go internal/gc/*_test.go internal/inboxbatch/*_test.go internal/inboxcurator/*_test.go"
  - criterion: "The tests of the gc package pass after the message rewrite"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestApplyDevWorktrees_ACheckpointThatCannotBeWrittenKeepsTheTree ./internal/gc"
  - criterion: "The parser sentinels ErrNoPriorityClass and ErrUnexplainedRoute keep their identity"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRequirePriorityClass|Admit' ./internal/inboxbatch ./internal/inboxcurator"
---

# Eval: STE rewrite of Go log and error text, batch 1 (S10)

> Pins the batch-1 rewrite of six Go message strings to ASD-STE100 (cycle 2507, inbox item
> `ste100-docs-rewrite-s10-go-log-text-rewrite`). The lint is the evidence for criterion 1. The
> pinned gc test and the sentinel tests show that criteria 2 and 3 hold.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lint-clean | six packages have 0 ste-lint --strict --go findings | 4/10 | `docs ste-lint --strict --go` filtered to the six packages |
| pins-follow | no test pins the old wording | 6/10 | grep of the old fragments in the six test sets |
| gc-pin | the gc pin test passes | 6/10 | `go test -run TestApplyDevWorktrees_ACheckpointThatCannotBeWrittenKeepsTheTree ./internal/gc` |
| sentinels | the two error sentinels keep their identity | 6/10 | `go test` of inboxbatch and inboxcurator |

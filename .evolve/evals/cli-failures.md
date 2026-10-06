---
score_cap:
  - criterion: "evolve failures reset --fingerprint records the operator-reset ack and the loop --reset prune without launching or preparing a loop"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_001' ./acs/cycle1803"
  - criterion: "failures prune --dry-run names exactly the expired failedApproaches and carryoverTodos and leaves state.json byte- and mtime-identical; the real prune removes exactly that set, never touches cycles_unpicked or expiresAt, and a usage error never falls through to the mutating prune"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_00[234]' ./acs/cycle1803"
  - criterion: "failures list --json includes every classification; the text form shows each entry's expiry and the sorted acknowledged fingerprints"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_00[56]' ./acs/cycle1803"
  - criterion: "failures list/reset/prune exit 2 on state or ledger I/O and never rewrite an unparseable file; a mutating run without --project-root is refused with exit 1"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_007' ./acs/cycle1803"
  - criterion: "loop --reset and failures reset share one prune+ack function, and the launch-time prune and failures prune share one expiry function, proven by identical survivors on one fixture and by the cmd/evolve call graph"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_008' ./acs/cycle1803"
  - criterion: "the launch path keeps its own per-batch bookkeeping: the launch-time prune still increments cycles_unpicked while failures prune leaves it alone"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestFailuresResetMatchesTheLoopReset|TestFailuresPruneMatchesTheLaunchPruneWithoutCarryoverBookkeeping)$' ./cmd/evolve"
  - criterion: "loop --reset and failures reset still acknowledge --fingerprint when state.json is unparseable, and still report a committed prune under the base prefixes when the ack fails"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestRunLoop_ResetErrorPathsKeepBasePrefixes|TestResetBatchState_KeepsBasePrefixesOnEveryErrorPath|TestRunFailures_ResetErrorPaths)$' ./cmd/evolve"
---

# Eval: evolve failures command group

> Pins the `evolve failures` list/reset/prune verbs (inbox item `cli-failures`,
> cycles 1798, 1800, 1801 and 1803). Acking a blocker fingerprint or pruning
> failedApproaches used to happen only as a side effect of launching a loop.
> Cycle 1801 shipped the group. Cycle 1803 adds the rest of the record:
> `prune --dry-run`, carryoverTodos expiry in the standalone prune, expiry and
> the acknowledged-fingerprint ledger in `list`, and the 0/1/2/10 exit
> vocabulary. It also proves the verbs and `loop` run the same prune and ack
> functions. The previous version of this eval pointed at `./acs/cycle1800`,
> which never landed. Its evidence resolved to no package, so this version
> repoints it at cycle 1803's predicates and the durable cmd/evolve unit tests.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reset-without-loop | reset acks and prunes without launching a loop | 6/10 | `go test -tags acs -run 'TestC1803_001' ./acs/cycle1803` |
| dry-run-purity | dry run names the expired set and changes nothing; prune removes exactly it | 4/10 | `go test -tags acs -run 'TestC1803_00[234]' ./acs/cycle1803` |
| list-visibility | every class in JSON; expiry and fingerprints in text | 6/10 | `go test -tags acs -run 'TestC1803_00[56]' ./acs/cycle1803` |
| exit-vocabulary | 2 on state/ledger I/O, 1 on mutating refusal, files never rewritten | 7/10 | `go test -tags acs -run 'TestC1803_007' ./acs/cycle1803` |
| shared-functions | one prune+ack and one expiry function behind both paths | 5/10 | `go test -tags acs -run 'TestC1803_008' ./acs/cycle1803` |
| launch-bookkeeping | loop increments cycles_unpicked, the verb does not | 5/10 | `go test -run 'TestFailures(Reset|Prune)…' ./cmd/evolve` |
| reset-error-paths | ack survives a prune error; prune report and base prefix survive an ack error | 5/10 | `go test -run 'TestRunLoop_ResetErrorPathsKeepBasePrefixes|…' ./cmd/evolve` |

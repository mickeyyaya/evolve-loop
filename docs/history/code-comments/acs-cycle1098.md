# Comment history: `acs/cycle1098`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1098/predicates_test.go:3` — above `package cycle1098`

```text
// Package cycle1098 materialises the cycle-1098 acceptance criteria for the two
// triage-committed top_n tasks of the `chain-policy-flag` lane (scout-report.md /
// triage-report.md). The lane's headline feature (batch chaining) LANDED at
// cycle 1075; this cycle fixes its two residual defects:
//
//   - chain-min-one-batch — `--until-inbox-empty` against an already-drained
//     inbox returns rc=0 having run ZERO cycles, silently weaker than the
//     pre-chain contract where `evolve loop` always ran one batch.
//   - chain-inbox-pending-validity — inboxPendingCount counts every root-level
//     `*.json` with no shape validation, so one malformed file pins pending>0
//     permanently and the chain burns batches to max_batches on a FALSE signal,
//     silently skipping nothing and hiding a real item lost to a typo.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1064 precedent).
// The subjects live in `package main` (go/cmd/evolve), which cannot be imported,
// so each predicate shells `go test -run` over the RED contract tests authored
// this cycle in go/cmd/evolve/cmd_loop_chain_minbatch_test.go and
// cmd_loop_chain_inboxvalidity_test.go. Every one of
// those exercises the system under test — calling chainStartDecision /
// inboxPendingCount and driving runLoopChain end-to-end over a real temp-dir
// inbox — and asserts on the returned decision, count, skip list, exit code and
// emitted chain summary. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban). RED now: `inboxPendingCount` still has its 2-value
// signature and `chainStartDecision` still stops at n==0, so go/cmd/evolve does
// not compile / the assertions fail.
//
// The single doc predicate (004) asserts on a DOC artifact, which is inherently
// textual; it is a documentation-content criterion, not a behavioural one, and
// the behaviour it describes is independently pinned by 001–003.
```

### `go/acs/cycle1098/predicates_test.go:110` — above `func TestC1098_004_ChainRegressionSuiteStillGreen(t *testing.T) {`

```text
// TestC1098_004_ChainRegressionSuiteStillGreen — anti-regression across BOTH
// tasks: the cycle-1075 chain contract (drain→next batch, quota defer, exact
// cap, brake, fleet-width preservation, rc mapping, CLI opt-in) must still hold
// after the two fixes. This is the predicate that fails if Builder "fixes" the
// defects by weakening the landed behaviour.
```

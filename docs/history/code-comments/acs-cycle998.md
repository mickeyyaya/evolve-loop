# Comment history: `acs/cycle998`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle998/predicates_test.go:3` — above `package cycle998`

```text
// Package cycle998 materialises the cycle-998 acceptance criteria for the two
// fleet-scoped carryover-consolidation tasks pinned to this lane:
//
//   - carryover-decisions-authoring   → author the judgment artifact AND apply it
//   - carryover-sweep-group-filer     → the surviving-items sweep-group re-filing
//
// What changed vs cycle-997. Cycle-997 built the machinery (`evolve carryover
// apply-decisions`, landed at go/cmd/evolve/cmd_carryover.go in this tree) and
// filed the six sweep-group inbox JSONs — but the decisions artifact it consumes
// never landed here, so nothing has run the apply and state.json:carryoverTodos
// is STILL 135 entries. The cycle-998 acceptance bar is therefore not "author a
// file" but "converge the live array": the decisions file must exist AND
// `--apply` must have run, so the drop/cluster ids are physically ABSENT from
// state.json. That applied effect is the crux predicate (003) — a re-author that
// skips `--apply` leaves every drop/cluster id resident and fails it.
//
// Predicate strategy — each predicate exercises a REAL emitted artifact or the
// APPLIED runtime state, never a source-grep of production code (the cycle-85
// degenerate-predicate ban):
//
//   - 001 parses the emitted decisions JSON and asserts it is internally valid
//     and does real pruning work (valid enum, non-empty reason, unique ids,
//     >= 60 drops, every cluster row names a cluster_group).
//   - 002 cross-references the decisions file against the LIVE state.json id set:
//     every surviving carryover id must be classified (survivors are `keep`).
//   - 003 is the cycle-998 crux: it reads the APPLIED state.json and asserts
//     every `drop`/`cluster` id is GONE and the live count fell well below the
//     135 baseline — proof the `--apply` step actually ran, not just authored.
//   - 004 cross-references the emitted sweep-group inbox JSONs against the
//     decisions file's `cluster` rows: exact 1:1 coverage, size 4–6, weight
//     0.7–0.8.
//
// Root resolution mirrors the cycle-997 predicates: artifacts and runtime state
// are read under acsassert.RepoRoot (the worktree, where Builder writes/applies
// per worktree isolation). The decisions file and applied state are Builder
// deliverables this cycle, so their absence / un-applied state is a FAILURE, not
// a skip.
```

### `go/acs/cycle998/predicates_test.go:82` — above `func loadDecisions(t *testing.T) decisionsFile {`

```text
// loadDecisions reads + parses the emitted decisions artifact under the worktree
// root. It t.Fatalf's (RED) when the file is absent or unparseable — this
// artifact is a mandatory Builder deliverable this cycle (it never landed in
// cycle-997), so its absence is a failure, not a skip.
```

### `go/acs/cycle998/predicates_test.go:208` — above `func TestC998_003_CarryoverDecisionsAppliedToState(t *testing.T) {`

```text
// TestC998_003_CarryoverDecisionsAppliedToState — the cycle-998 CRUX AC
// (carryover-decisions-authoring): authoring alone is NOT the deliverable; the
// `evolve carryover apply-decisions --apply` step must have RUN against this
// worktree's state.json. Proof of that is behavioural, read from the applied
// runtime state (not the authored file): every `drop` and every `cluster` id
// must be ABSENT from state.json:carryoverTodos, and the live count must have
// fallen well below the 135 baseline. A re-author that skips `--apply` leaves all
// drop/cluster ids resident and the count at 135 → this predicate stays RED.
```

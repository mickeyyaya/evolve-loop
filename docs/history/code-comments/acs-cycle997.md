# Comment history: `acs/cycle997`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle997/predicates_test.go:3` — above `package cycle997`

```text
// Package cycle997 materialises the cycle-997 acceptance criteria for the
// fleet-scoped inbox item `carryover-consolidation-sweep`, which triage split
// into three dependency-chained committed tasks:
//
//   - carryover-decisions-authoring     → the judgment artifact
//   - carryover-apply-consolidation-cli → the sanctioned apply path
//   - carryover-sweep-group-filer       → the surviving-items re-filing
//
// The inbox item asks for a ONE-TIME judgment pass over
// state.json:carryoverTodos (135 live entries this cycle): drop stale failure
// echoes + landed duplicate shadows, and cluster genuine small items into
// 4–6-item sweep-group inbox filings, shrinking the array toward ~25. The TTL
// prune machinery already exists (failurelog.PruneExpiredCarryoverTodos, wired
// in cmd_loop.go) but only removes entries whose expiresAt is already past — it
// cannot perform the semantic keep/drop/cluster judgment. THAT judgment, plus a
// sanctioned locked-RMW path to apply it and the re-filing of survivors, is the
// gap these three tasks close.
//
// Predicate strategy — each predicate exercises a REAL emitted artifact or the
// system-under-test, never a source-grep of production code (the cycle-85
// degenerate-predicate ban):
//
//   - 001 / 002 parse the emitted decisions JSON artifact and cross-reference it
//     against the LIVE .evolve/state.json id set — a source-independent data
//     binding: the file must actually classify the real carryover population,
//     not merely contain a magic string.
//   - 003 shells `go test -race` over the DEFAULT build suite for the four
//     binding tests Builder must author against the new `evolve carryover`
//     subcommand, requiring a `--- PASS: <name>` line for each (the cycle-987
//     behavioural-via-subprocess precedent). A still-missing command, an
//     unregistered subcommand, an un-rejected missing-reason entry, or an apply
//     that bypasses the flock RMW path each leaves one PASS line absent → RED.
//   - 004 parses the emitted sweep-group inbox JSON files and cross-references
//     them against the decisions file's `cluster` entries: exact 1:1 coverage,
//     group sizing 4–6, weight band 0.7–0.8.
//
// Root resolution mirrors the established cycle-84/86 regression predicates:
// artifacts are read under acsassert.RepoRoot (the worktree, where Builder
// writes them per worktree-isolation), and a genuinely-absent runtime file is a
// SKIP, never a false PASS — but the decisions file / sweep files / CLI tests
// are Builder deliverables, so they must be PRESENT and correct at audit time.
```

### `go/acs/cycle997/predicates_test.go:101` — above `func liveCarryoverIDs(t *testing.T) (map[string]bool, bool) {`

```text
// liveCarryoverIDs reads the id set of state.json:carryoverTodos under the
// worktree root. Returns (ids, true) on success; (nil, false) when state.json is
// genuinely absent (SKIP-worthy — cannot verify coverage without the source
// population), following the cycle-84/86 regression-predicate convention.
```

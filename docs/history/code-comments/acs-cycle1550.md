# Comment history: `acs/cycle1550`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1550/predicates_test.go:3` — above `package cycle1550`

```text
// Package cycle1550 materialises the cycle-1550 acceptance criterion for the
// fleet-scoped task pipeline-defect-pipeline-blocker.
//
// This lane's triage top_n committed a single, generic placeholder — id
// "pipeline-defect-pipeline-blocker", action "fix the pipeline defect/blocker
// identified for this lane" (triage-decision.json) — with no matching entry
// in scout-report.md's selected tasks. That mismatch IS the live pipeline
// defect: state.json's carryover queue (todo-halt-autofiler-mints-unique-ids,
// first_seen_cycle 1548, HIGH) and its recorded audit history (cycle 1548
// defect H1) both document that the ADR-0072 halt auto-filer
// (writePipelineEscalation, go/cmd/evolve/cmd_loop_escalation.go) mints the
// auto-filed inbox item's id as `pipeline-defect-<category>` — a bare
// category label, not a unique record identity. Because the filename is
// deterministic on category alone, every later halt sharing a category
// silently overwrites the earlier one's on-disk record (state.json: "17
// on-disk records share the id pipeline-defect-pipeline-blocker — 1 live +
// 16 consumed"). A fleet lane whose scope snapshot captured that bare id at
// triage time can then resolve, at scout time, to whatever content a LATER
// unrelated halt happened to overwrite it with — or to nothing scout
// recognizes at all — which is exactly what happened to THIS lane
// (cycle 1550) and to cycle 1548 before it (the inst-L1543c empty-scope
// class, now a second-plus recurrence).
//
// The fix is to mint the id as `pipeline-defect-<category>-cycle<N>`
// unconditionally, so every halt's record identity is unique and a scope
// snapshot can never drift onto a different halt's content.
//
// Predicate strategy — writePipelineEscalation is unexported inside
// `package main` (go/cmd/evolve), unreachable from a leaf acs package, so
// this predicate drives it through the sanctioned behavioural-via-subprocess
// shape (cycle-987/997/1532/1544 precedent): a `-run`-narrowed, single-named-
// package `go test -v` that must print `--- PASS: <name>` for each binding
// test the Builder makes pass. Asserting on the PASS line (never exit 0)
// is load-bearing: `go test -run` against a pattern matching no test exits 0
// with "no tests to run", so a still-missing/still-failing binding test would
// otherwise false-GREEN.
```

### `go/acs/cycle1550/predicates_test.go:75` — above `func TestC1550_001_HaltAutofilerMintsCycleScopedIdentity(t *testing.T) {`

```text
// TestC1550_001_HaltAutofilerMintsCycleScopedIdentity — AC1: the ADR-0072
// halt auto-filer must mint the auto-filed inbox item's id as
// pipeline-defect-<category>-cycle<N>, so a category alone can never be
// mistaken for a unique record identity by a downstream scope snapshot.
```

### `go/acs/cycle1550/predicates_test.go:85` — above `func TestC1550_002_DistinctHaltsNeverCollideOnDisk(t *testing.T) {`

```text
// TestC1550_002_DistinctHaltsNeverCollideOnDisk — AC2, the negative/edge
// case: two ADR-0072 halts sharing a category (the common case today — 17
// on-disk records share "pipeline-defect-pipeline-blocker" alone, per
// state.json) must both survive on disk instead of the later halt silently
// destroying the earlier one's evidence via a collision on the deterministic
// category-only filename.
```

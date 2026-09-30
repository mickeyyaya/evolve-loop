# Comment history: `acs/cycle1546`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1546/predicates_test.go:3` — above `package cycle1546`

```text
// Package cycle1546 materialises the cycle-1546 acceptance criteria for the one
// task triage committed to this lane:
//
//   - continuation-create-reuse-snapshot-base-guard  (ADR-0076 slice C, architect finding #3)
//
// The other two ids in the lane scope (lost-ship-closeout-universal-landing-witness,
// lost-ship-dossier-evidence) are `## deferred` in triage-report.md and therefore
// carry ZERO predicates here — R9.3: predicates bind only to triage-committed work,
// and a predicate gating deferred work starves the committed task (cycle-280).
//
// The defect. Slice C changed preserved-worktree state from DIRTY to COMMITTED: a
// FAILed cycle's work is now snapshot-committed onto the cycle branch with the
// subject `salvage snapshot (ADR-0076 continuation-on-fail)`. The adoption path
// (CreateFrom) handles that shape. The REUSE path does not: when gitWorktree.Create
// reuses an existing worktree for the same cycle number (resume/reset edge), the
// tree is now CLEAN — so ensureCleanWorktree no-ops — and cyclerun's base capture
// (`rev-parse HEAD`, cyclerun.go:497) records the SALVAGE SNAPSHOT as
// CycleState.WorktreeBaseSHA. normalizeWorktreeToBase then soft-resets to that very
// snapshot, so the salvaged work sits AT the base, produces an empty review diff,
// and is never re-exposed for audit. Silent, and the failure looks like "the builder
// did nothing".
//
// Predicate strategy — every predicate exercises the SYSTEM, never a source grep of
// production code (the cycle-85 degenerate-predicate ban). The seams under test
// (gitWorktree.Create, ensureCleanWorktree, the runCycle base capture,
// normalizeWorktreeToBase) are all UNEXPORTED, so they are unreachable from a leaf
// acs package. Each predicate therefore drives them through the sanctioned
// behavioural-via-subprocess shape (the cycle-987/997/1532 precedent): a
// `-run`-narrowed, single-named-package `go test -v` that must print
// `--- PASS: <name>` for every binding test Builder authors. Asserting on the PASS
// LINE — never on exit 0 — is load-bearing: `go test -run` on a pattern that matches
// NO test exits 0 with "no tests to run", so a still-missing binding test would
// false-GREEN.
//
// Every invocation is `-run`-narrowed against ONE named package, never a `/...`
// sweep and never the bare 40s+ ./internal/core suite, so a concurrent lane's
// contamination in an untouched package can never red this cycle
// (flaky-predicate-shape / scope-lint contract).
```

### `go/acs/cycle1546/predicates_test.go:147` — above `func TestC1546_005_GuardIsReachedFromTheProductionProvisioningPath(t *testing.T) {`

```text
// TestC1546_005_GuardIsReachedFromTheProductionProvisioningPath — AC5, the WIRING
// PROOF. A seam whose only caller is a test is dead code, and the four predicates
// above would all pass on a guard helper that production never invokes — the failure
// mode #373 is named for. The binding test must drive the REAL provisioning path
// (the orchestrator's worktree.Create + base capture in cyclerun.go, not the helper
// directly) against a worktree whose HEAD is a salvage snapshot, and assert the
// resulting CycleState.WorktreeBaseSHA is the guarded value. Builder must name that
// production caller as `file:line` in build-report.md.
```

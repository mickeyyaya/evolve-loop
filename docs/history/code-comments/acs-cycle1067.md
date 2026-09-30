# Comment history: `acs/cycle1067`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1067/predicates_test.go:3` — above `package cycle1067`

```text
// Package cycle1067 materialises the cycle-1067 acceptance criteria for this
// lane's single fleet-scoped task:
//
//   - ship-stage-explicit-paths → shipDirect (gitops.go:228) and
//     shipFromWorktree (gitops.go:374) must stop staging with `git add -A` for
//     the non-release classes and stage an explicit `git add -- <paths>`
//     instead, sourced from the DECLARED manifest (build-report.md +
//     test-report.md, the set declaredManifest already computes for the
//     manifest gate), with a porcelain-changed-set fallback when no manifest is
//     readable. The release class already proves the pattern (stageReleaseSet,
//     gitops.go:707).
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban). The staging call sites are unexported, so
// each predicate shells the ship package's OWN behavioural tests, which drive
// shipDirect / shipFromWorktree through their production code paths:
//
//   - 001 asserts the declared-manifest staging effect (git args captured from a
//     real shipDirect run: explicit pathspec, undeclared stray excluded).
//   - 002 asserts the manifest-empty FALLBACK — the H2 blast-radius risk: an
//     empty manifest must fall back to the porcelain changed set, never to
//     `add -A` and never to a silent skip (which would ship nothing while
//     reporting success).
//   - 003 is the negative/anti-no-op axis: NO non-release class may emit
//     `git add -A`, and the stale discriminator test name asserting the old
//     behaviour must no longer exist.
//   - 004 is the real-git effect check (integration tag): the ship commit
//     produced against a genuine repository contains the declared path and NOT
//     the undeclared untracked stray `add -A` would sweep in.
//   - 005 is the no-regression floor: the whole ship package, N/N PASS.
//
// No predicate asserts on production source text.
```

### `go/acs/cycle1067/predicates_test.go:80` — above `func TestC1067_002_AReportlessWorkspaceAdoptsNoPathAndNoWorkspaceStagesTheChangedSet(t *testing.T) {`

```text
// TestC1067_002_AReportlessWorkspaceAdoptsNoPathAndNoWorkspaceStagesTheChangedSet
// — AC2 (H2), moved to the F43 contract (2026-09-28, design §5.12 component 4):
// with no WorkspacePath (an operator's manual ship) staging is still the
// porcelain changed set. A workspace whose reports declare nothing adopts no
// path and never runs an empty `git add -A --` (which stages the whole tree):
// its tracked edits ship through the audit binding's `add -u`, and adopting
// every changed path would carry untracked residue into the binding (cycle
// 1594). An audited cycle cannot reach Ship with only undeclared untracked
// work: the audit refuses those inputs by name, so the empty-ship worry this
// predicate was written for is answered upstream.
```

### `go/acs/cycle1067/predicates_test.go:117` — above `func TestC1067_004_WorktreeShipCommitExcludesUndeclaredStray(t *testing.T) {`

```text
// TestC1067_004_WorktreeShipCommitExcludesUndeclaredStray — AC1 for the
// worktree path, verified as an OBSERVABLE EFFECT against a genuine repository
// (integration tag): the ship commit contains the declared path and not the
// undeclared untracked stray that `git add -A` sweeps in (cycle-645 leak).
```

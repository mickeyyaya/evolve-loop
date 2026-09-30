# Comment history: `acs/cycle1547`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1547/predicates_test.go:3` — above `package cycle1547`

```text
// Package cycle1547 materialises the cycle-1547 acceptance criteria for the one
// task triage committed to this lane:
//
//   - continuation-create-reuse-snapshot-base-guard  (ADR-0076 slice C, architect finding #3)
//
// The other lane-scope id, worktree-provisioning-retry, is `## deferred` in
// scout-report.md (all three named production call sites — core, swarm, and the
// operator CLI — already consume gitexec.AddWorktreeWithRetry) and therefore
// carries ZERO predicates here — R9.3: predicates bind only to triage-committed
// work, and a predicate gating deferred work starves the committed task
// (cycle-280).
//
// PRE-EXISTING GREEN. This exact defect and fix already landed at
// go/internal/core/worktree_base.go + worktree_base_test.go, shipped in
// commit e3c99d77 "fix(core): salvage the snapshot-base guard from the
// 1539-1546 absorbing-FAIL chain (#486)", which is already an ancestor of this
// cycle's worktree (`git log --oneline -1 -- go/internal/core/worktree_base.go`
// resolves to e3c99d77; see also the go/acs/cycle1546/predicates_test.go
// original for the same claim). Scout re-selected the slug from lane scope
// before that landing was reconciled against the inbox. Per this agent's Step 4
// RED-verification rule ("Unexpected pass: log as pre-existing GREEN, mark in
// handoff") these predicates are authored, run, and confirmed GREEN rather than
// invented as false RED — there is no production code left for Builder to
// write, and Builder's job this cycle is a no-op confirmation, not a fix.
//
// Predicate strategy — every predicate exercises the SYSTEM, never a source
// grep of production code (the cycle-85 degenerate-predicate ban). The seams
// under test (gitWorktree.Create, ensureCleanWorktree, the runCycle base
// capture, normalizeWorktreeToBase) are all UNEXPORTED, so they are
// unreachable from a leaf acs package. Each predicate therefore drives them
// through the sanctioned behavioural-via-subprocess shape (the
// cycle-987/997/1532/1546 precedent): a `-run`-narrowed, single-named-package
// `go test -v` that must print `--- PASS: <name>` for every binding test. Since
// the binding tests already exist and already pass, this predicate package
// re-confirms the live wiring rather than gating a pending fix — a regression
// here means the guard was silently removed or broken, not merely unbuilt.
//
// Every invocation is `-run`-narrowed against ONE named package, never a `/...`
// sweep and never the bare 40s+ ./internal/core suite, so a concurrent lane's
// contamination in an untouched package can never red this cycle
// (flaky-predicate-shape / scope-lint contract).
```

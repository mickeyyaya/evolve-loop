# Comment history: `acs/cycle1268`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1268/predicates_test.go:3` — above `package cycle1268`

```text
// Package cycle1268 materialises the cycle-1268 acceptance criteria for the two
// tasks triage committed to this lane:
//
//   - worktree-provisioning-retry-consolidate (inbox retro-fleet-worktree-dispatch, w=0.9)
//   - test-amplification-context-scope        (inbox test-amplification-context-scope, w=0.89)
//
// Scope note (read before judging these predicates). This lane is an ADR-0076
// continuation of cycle-1267 (snapshot 79d130d4), whose audit FAILed on an
// unrelated integration-tier bridge flake — so task 2 arrived here already
// substantially landed. The TDD phase verified the live tree rather than
// trusting the report, and pinned only what is genuinely open:
//
//	Task 1 — fully open. PR #401's bounded retry exists at ONE of four
//	         `git worktree add` call sites; CreateFrom, swarm's addWorktree and
//	         the operator CLI still issue the bare unretried add. 001-004.
//	Task 2 — the derivation half (CoveringTests, DirectImporters, the artifact,
//	         the fail-open guard, the truncation log) is PRE-EXISTING GREEN and
//	         is pinned as a regression guard, not as new work. The open half is
//	         the D3/F1 MEDIUM its own auditor raised: renderCoveringTests
//	         interpolates attacker-influenced filenames unescaped into a
//	         document agent.md declares authoritative. 005-006.
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255/1267
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - No source-grep predicate is used as a load-bearing assertion — it would
//     pass the moment the magic string appeared, fix or no fix (cycle-85 ban).
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the two naming ./internal/core and ./cmd/evolve (known
//     40s+ suites) are narrowed with -run, which the rule explicitly permits.
//     No wall-clock bounds, no literal PIDs, no bare `git`, no load generators.
//
// Consolidation is pinned BEHAVIOURALLY rather than by grepping for a helper
// name: 002/003/004 each assert the site honours gitexec.DefaultWorktreeAddAttempts,
// so three private copies of the constant cannot satisfy the suite. A structural
// pin on a package-qualified call shape is also what burned cycle-644.
```

### `go/acs/cycle1268/predicates_test.go:82` — above `func TestC1268_001_SharedWorktreeAddRetryHelperExists(t *testing.T) {`

```text
// TestC1268_001_SharedWorktreeAddRetryHelperExists — AC1-AC4 of task 1.
//
// The extraction target itself: one bounded retry-with-backoff loop, in the
// only package core, swarm and cmd/evolve all already depend on. It must absorb
// a transient rc=255, stay bounded at DefaultWorktreeAddAttempts, surface git's
// own diagnosis on persistent failure (the CB.2 alarm chain stays armed — the
// refuted PR #400 is the record of what silencing it costs), and charge a clean
// provision exactly one attempt and zero backoff.
```

### `go/acs/cycle1268/predicates_test.go:100` — above `func TestC1268_002_CreateFromRetriesTransientCollision(t *testing.T) {`

```text
// TestC1268_002_CreateFromRetriesTransientCollision — AC5-AC7 of task 1,
// adoption site #1 (ADR-0076 continuation seeding, worktree.go:208).
//
// The two PR #401 tests are re-run alongside the new ones: "existing tests
// staying green" is an explicit acceptance criterion, so a consolidation that
// regressed Create while fixing CreateFrom must not be able to green this.
```

### `go/acs/cycle1268/predicates_test.go:116` — above `func TestC1268_003_SwarmWorkerProvisioningRetries(t *testing.T) {`

```text
// TestC1268_003_SwarmWorkerProvisioningRetries — AC8-AC11 of task 1, adoption
// site #2: the highest-contention seam in the tree (N workers, one shared .git).
// BOTH production entry points are required — wiring one path only is the same
// defect, just narrower (#373).
```

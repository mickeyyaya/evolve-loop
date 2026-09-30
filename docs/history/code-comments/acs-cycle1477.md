# Comment history: `acs/cycle1477`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1477/predicates_test.go:3` — above `package cycle1477`

```text
// Package cycle1477 materialises the cycle-1477 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - worktree-retry-diagnostic-integrity → the shared `git worktree add` retry
//     must keep BOTH the initiating and the terminal failure recoverable from
//     what it returns, must ANNOUNCE contention before it pays the backoff, must
//     pay NO backoff on a permanent failure, and must leave a RECOVERED success
//     free of retry noise.
//
// State of the work when these predicates were authored. The behaviour above
// landed in cycle-1474 (commit 07514fe8, reachable from this lane's base
// 18aa6f05): go/internal/gitexec/worktree.go now captures the first retryable
// failure and appends it only on terminal failure, and calls OnRetry before
// Sleep. Predicates 001-004 are therefore expected PRE-EXISTING GREEN — they are
// the durable contract for the criteria, not a RED bar, and the test-report
// records that honestly rather than manufacturing a false RED.
//
// 005 is this cycle's NEW coverage and the reason the set is not a copy of
// cycle-1474's. Cycle-1474 drove ONLY the gitexec seam directly; nothing pinned
// that a PRODUCTION provisioning caller surfaces git's own diagnostic, or that
// the newly-added retry-history decoration does not leak into a caller's error
// on a single-attempt permanent failure. 005 drives
// swarm.NewGitWorkerProvisioner — the real constructor the composition root
// uses, with the real git binary — and asserts on the error it returns.
//
// Predicate strategy — every predicate invokes the system under test in-process
// and asserts on returned values (the cycle-85 degenerate-predicate ban): no
// source greps, no `go test` subprocess, no whole-package sweep, no wall-clock
// bound, no literal PID, no bare `git` against process cwd.
```

### `go/acs/cycle1477/predicates_test.go:84` — above `func TestC1477_001_RetryPreservesInitiatingFailure(t *testing.T) {`

```text
// TestC1477_001_RetryPreservesInitiatingFailure is the crux criterion: a
// transient rc=255 followed by a DIFFERENT terminal failure must leave both
// recoverable. Losing the initiating rc=255 reports a plain path collision that
// never happened; losing the terminal code/stderr disarms the downstream
// fail-fast (the refuted PR #400 is the record of that cost).
```

### `go/acs/cycle1477/predicates_test.go:202` — above `func TestC1477_005_ProductionProvisionerSurfacesGitStderrWithoutFabricatedHistory(t *testing.T) {`

```text
// TestC1477_005_ProductionProvisionerSurfacesGitStderrWithoutFabricatedHistory
// is this cycle's WIRING PROOF and its new coverage over cycle-1474.
//
// Cycle-1474 pinned the helper in isolation; nothing pinned that a real
// provisioning caller renders what the helper returns. This drives the
// production constructor swarm.NewGitWorkerProvisioner (the one the composition
// root calls) with the REAL git binary against a directory that is not a
// repository, so the shared classifier fires on a real rc=128 and the real error
// path runs. Two things must hold at the caller:
//
//   - git's OWN stderr reaches the operator through the shared helper (the
//     reason the operator path was routed through gitexec at all: a raw
//     exec.Command could only ever report "exit status 255"), and
//   - the retry-history decoration added for 001 does NOT leak onto a
//     single-attempt permanent failure — the caller-visible negative axis that
//     004 only covers on the success path.
//
// Deterministic by construction: a non-repository parent yields the same rc=128
// "not a git repository" on every platform, one attempt, no backoff, no clock.
```

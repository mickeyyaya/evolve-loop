# Comment history: `acs/cycle1356`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1356/predicates_test.go:3` — above `package cycle1356`

```text
// Package cycle1356 materializes the acceptance criteria of this lane's fleet
// -scoped inbox item `auto-refresh-binary-at-boundary` (scout-report.md Tasks
// 1/2: `chain-boundary-binary-refresh`, `pin-boundary-repin-branch-residual`).
//
// FINDING (read-first, rule 8 — this cycle changes NO production code): Task
// 1's implementation already exists in this worktree, in full, GREEN — the
// SAME finding cycles 1343 and 1352 already made and pinned for this same
// recurring inbox item (go/acs/cycle1343, go/acs/cycle1352). Re-verified
// fresh for cycle 1356:
//
//	grep -rn 'maybeRefreshChainBoundary|chainBoundaryRefreshLogFile' go/cmd/evolve/cmd_loop_chain.go
//	  -> present, fully wired at runLoopChain's per-batch boundary
//	go test -run BoundaryRefresh ./cmd/evolve/...  -> all PASS
//
// Task 2's residual, however, is NOT covered by any prior cycle
// (`grep -rln 'attemptBootRepin' go/cmd/evolve/*_test.go` -> zero hits before
// this cycle). It also cannot be tested AS LITERALLY SCOPED: the scout
// report's target file `cmd_loop_boot_refresh.go` and seam
// `bootRefreshRepinFn` do not exist anywhere in this worktree's checked-out
// source —
//
//	grep -rn 'bootRefreshRepinFn|BootBinaryRefresh' go/   -> zero hits
//	find go -name 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main (4dadf62a923640c) is 71 commits
// behind current main; the boot-time rebuild+re-exec self-heal the scout
// report describes is a main-line feature this worktree's snapshot predates.
// Writing a predicate against `cmd_loop_boot_refresh.go` here would mean
// inventing an API — rule 8 forbids it.
//
// The residual's INTENT — pin which of two repin call sites either side of a
// re-exec heals, and prove the other is a documented no-op, not an
// accidental race — is preserved and applied to the mechanism this worktree
// actually has: maybeRefreshChainBoundary's pre-exec repin (the "parent"
// branch) versus the re-exec'd child's boot-recovery repin
// (attemptBootRepin, gated by detectShipSHAMismatch — the "child" branch).
// See test-report.md AC-Materialization for the full disposition and the
// one-line reason for this reframing (rule 3, no silent guessing).
//
// Adversarial diversity:
//
//	C1356_001 positive — Task 1's functional core: the chain-boundary refresh
//	                      fires strictly before every batch, never mid-batch,
//	                      and a trip stops the chain before that boundary's
//	                      own batch (AC1/AC2).
//	C1356_002 positive — Task 1's fail-open contract: an ahead-check error (or
//	                      any staleness-check failure) degrades to
//	                      refreshed=false and the chain keeps running on the
//	                      current binary rather than halting (AC3).
//	C1356_003 positive — Task 1's auditable authorization class: a successful
//	                      boundary refresh is ledgered under the
//	                      distinguishable "boundary-refresh" class, separate
//	                      from the boot-time class (AC4).
//	C1356_004 positive — Task 2 (reframed): the parent branch
//	                      (maybeRefreshChainBoundary) performs the heal
//	                      before re-exec; the child branch's own repin
//	                      (attemptBootRepin) is a documented no-op both via
//	                      the upstream mismatch gate and directly.
//	C1356_005 negative — no second, duplicate stop-only staleness code path
//	                      (`chain_binary_stale`/`StaleAtBoundary`) has been
//	                      (re-)introduced alongside the shipped
//	                      rebuild+re-pin+re-exec design.
```

### `go/acs/cycle1356/predicates_test.go:86` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

### `go/acs/cycle1356/predicates_test.go:170` — above `func TestC1356_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {`

```text
// TestC1356_005_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal wording names a WEAKER stop-only design that was
// superseded before this cycle by the shipped rebuild+re-pin+re-exec
// boundary hook (cycles 1343/1352 already pinned this). Fails loudly if a
// future change reintroduces that duplicate, narrower code path.
```

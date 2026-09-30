# Comment history: `acs/cycle1373`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1373/predicates_test.go:3` — above `package cycle1373`

```text
// Package cycle1373 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `auto-refresh-binary-at-boundary`
// (scout-report.md Task 1 — "chain-boundary-binary-refresh"; triage
// top_n — "call bootBinaryRefreshFn from runLoopChain at batch boundaries
// (n>0), reusing existing staleness/fleet-lane-guard machinery").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout's premise (and triage's literal top_n text, carried over verbatim
// from scout) names `go/cmd/evolve/cmd_loop_boot_refresh.go` /
// `bootBinaryRefreshFn` as an existing, callable helper this lane merely
// needs to wire into `runLoopChain`. Neither the file nor the symbol
// exists anywhere in this worktree's checked-out source:
//
//	grep -rn 'func bootBinaryRefreshFn|BootBinaryRefresh' go/   -> zero hits
//	find go -iname 'cmd_loop_boot_refresh*.go'                 -> no file
//
// `git merge-base --is-ancestor 2329f350 HEAD` (the commit that landed
// `cmd_loop_boot_refresh.go` on `main`) reports NOT an ancestor of this
// worktree's HEAD — this lane's branch diverged from `main` at 4dadf62a,
// before that boot-time-only self-heal landed upstream. Inventing a call
// to a function that does not exist on this branch would be rule-8
// API-invention, not a real fix.
//
// What DOES exist, already fully built, wired, and GREEN on THIS
// worktree's HEAD (commit 31790b6d, "salvage snapshot"), is the exact
// chain-boundary self-heal the inbox item's fix text asks for —
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go:475),
// called at:
//   - the chain boundary, inside runLoopChain's per-batch loop
//     (cmd_loop_chain.go:660), strictly after the operator-brake check and
//     strictly before runLoopBatchFn — never mid-batch; AND
//   - the plain wave/fleet boundary inside runLoop's per-iteration loop
//     (cmd_loop.go:552), before any lane/wave dispatch starts that
//     iteration — never mid-lane.
//
// This is the SAME finding cycles 1314/1323/1340/1343/1352/1356/1368/1370
// already made and pinned for this same recurring inbox item (their
// go/acs/cycle<N> predicate files are all present on this worktree's
// history) — the item keeps re-entering fleet_scope (queue-hygiene
// residual, operator memory: "queued at priority 0.55") faster than the
// consuming commit that would remove it from the backlog lands. Re-verified
// fresh for cycle 1373:
//
//	go build ./...                                          -> clean
//	go vet ./cmd/evolve/...                                  -> clean
//	go test ./cmd/evolve/...                                 -> ok (full package, no regression)
//	go test -run BoundaryRefresh ./cmd/evolve/... -v         -> all PASS
//
// No new production code is warranted (Operating Principle 1 — TDD must
// NOT implement production code, and there is nothing missing to
// implement). This cycle's predicates REGRESSION-LOCK the already-shipped
// contract for THIS cycle's audit gate (ACS predicates are cycle-scoped,
// never replayed by a later gate — test-report.md Step 6b) via named
// `go test -run` subprocess assertions against the real production callers
// (House Rule 2 — caller proof), the established idiom for `package main`
// coverage from `go/acs` (cycle-1352/1356/1368/1370 precedent).
//
// Adversarial diversity:
//
//	C1373_001 positive — the wave-boundary call site (cmd_loop.go:552)
//	                      fires before dispatch, never mid-lane (AC1).
//	C1373_002 positive — the chain-boundary call site
//	                      (cmd_loop_chain.go:660) fires strictly before
//	                      every batch, never mid-batch (AC1, AC5's
//	                      chain-mode half).
//	C1373_003 positive — the fail-open + fleet-lane-guard contract: any
//	                      staleness/rebuild check failure, AND a live
//	                      sibling fleet lane, both degrade to
//	                      refreshed=false with no rebuild invoked (AC3,
//	                      AC4 — "refuse mid-batch (lanes in flight)").
//	C1373_004 positive — the re-exec loop breaker: two consecutive
//	                      boundary hits at the same running commit refuse
//	                      the second rebuild attempt (AC5), and a
//	                      successful refresh is ledgered under a
//	                      distinguishable "boundary-refresh" authorization
//	                      class (AC2 — "an auditable ... authorization
//	                      class (NOT silent)").
//	C1373_005 negative — no second, superseded stop-only staleness code
//	                      path has been (re-)introduced alongside the
//	                      shipped rebuild+re-pin+re-exec boundary design.
```

### `go/acs/cycle1373/predicates_test.go:104` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

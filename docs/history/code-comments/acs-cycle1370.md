# Comment history: `acs/cycle1370`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1370/predicates_test.go:3` — above `package cycle1370`

```text
// Package cycle1370 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `auto-refresh-binary-at-boundary`
// (scout-report.md Task 1 — "land-chain-boundary-binary-refresh").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout's premise names `go/cmd/evolve/cmd_loop_boot_refresh.go` /
// `bootBinaryRefresh` as an existing "load-bearing precedent" on this
// worktree. Neither the file nor the symbol exists anywhere in this
// worktree's checked-out source:
//
//	grep -rn 'func bootBinaryRefresh|BootRefreshClass' go/   -> zero hits
//	find go -iname 'cmd_loop_boot_refresh*.go'               -> no file
//
// Scout's own decision trace notes this worktree is a fresh cutout of
// `main` — but `main` and this worktree's checked-out HEAD are 71 commits
// apart (`git rev-list --count HEAD..origin/main` = 71); the boot-time
// self-heal scout describes is a main-line feature this worktree's
// snapshot predates, so Task 2 (refactor-boot-refresh-shared-heal-helper)
// has no source to refactor here — inventing one would mean inventing an
// API (rule 8 forbids it), and fleet_scope pins this lane to
// `auto-refresh-binary-at-boundary` only, so Task 2 is out of scope for
// this lane regardless.
//
// What DOES exist, already fully built, wired, and GREEN on THIS
// worktree's HEAD, is the exact chain-boundary self-heal Task 1 asks to
// land: `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go:441),
// called at
//   - the chain boundary, inside runLoopChain's per-batch loop
//     (cmd_loop_chain.go:626), AND
//   - the plain wave/fleet boundary inside runLoopBatch's per-iteration
//     loop (cmd_loop.go:552).
//
// This is the SAME finding cycles 1314/1323/1343/1352/1356/1368 already
// made and pinned for this same recurring inbox item (their
// go/acs/cycle<N> predicate files are all present on this worktree's
// history) — the item keeps re-entering fleet_scope (queue-hygiene
// residual, operator memory: "queued at priority 0.55") faster than the
// consuming commit lands. Re-verified fresh for cycle 1370:
//
//	go build ./...                                          -> clean
//	go vet ./...                                             -> clean
//	go test ./cmd/evolve/...                                 -> all PASS (full package, no regression)
//	go test -run BoundaryRefresh ./cmd/evolve/... -v         -> all PASS
//
// No new production code is warranted (Operating Principle 1 — TDD must
// NOT implement production code, and there is nothing missing to
// implement). This cycle's predicates REGRESSION-LOCK the already-shipped
// contract for THIS cycle's audit gate (ACS predicates are cycle-scoped,
// never replayed by a later gate — test-report.md Step 6b) via named
// `go test -run` subprocess assertions against the real production callers
// (House Rule 2 — caller proof), the established idiom for `package main`
// coverage from `go/acs` (cycle-1352/1356/1368 precedent).
//
// Adversarial diversity:
//
//	C1370_001 positive — the wave-boundary call site (cmd_loop.go:552),
//	                      exactly where scout's reference designs describe
//	                      it, fires before dispatch, never mid-lane (AC1).
//	C1370_002 positive — the chain-boundary call site (cmd_loop_chain.go:626)
//	                      fires strictly before every batch, never
//	                      mid-batch (AC1, AC5's chain-mode half).
//	C1370_003 positive — the fail-open + fleet-lane-guard contract: any
//	                      staleness/rebuild check failure, AND a live
//	                      sibling fleet lane, both degrade to
//	                      refreshed=false with no rebuild invoked (AC3, AC4).
//	C1370_004 positive — the re-exec loop breaker: two consecutive boundary
//	                      hits at the same running commit refuse the second
//	                      rebuild attempt (AC5), and the refresh is ledgered
//	                      under a distinguishable "boundary-refresh"
//	                      authorization class (AC2).
//	C1370_005 negative — no second, superseded stop-only staleness code
//	                      path has been (re-)introduced alongside the
//	                      shipped rebuild+re-pin+re-exec boundary design.
```

### `go/acs/cycle1370/predicates_test.go:97` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

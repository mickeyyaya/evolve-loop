# Comment history: `acs/cycle1368`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1368/predicates_test.go:3` — above `package cycle1368`

```text
// Package cycle1368 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `auto-refresh-binary-at-boundary`
// (scout-report.md Task 1 — "Wire bootBinaryRefreshFn into the
// wave-boundary block of runLoopBatch").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout's premise names `bootBinaryRefreshFn` /
// `go/cmd/evolve/cmd_loop_boot_refresh.go` as the seam to wire. Neither
// exists anywhere in this worktree's checked-out source:
//
//	grep -rn 'bootBinaryRefreshFn|BootBinaryRefresh' go/   -> zero hits
//	find go -iname 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main is 71 commits behind current
// main (`git rev-list --count HEAD..origin/main` = 71); the boot-time
// rebuild+re-exec self-heal the scout report describes is a main-line
// feature this worktree's snapshot predates — inventing a predicate against
// it would mean inventing an API (rule 8 forbids it).
//
// What DOES exist, already fully built, wired, and GREEN, is the
// FUNCTIONAL EQUIVALENT the AC set actually asks for:
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go), called at
//   - the chain boundary, inside runLoopChain's per-batch loop
//     (cmd_loop_chain.go:626), AND
//   - the plain wave/fleet boundary inside runLoopBatch's per-iteration
//     loop (cmd_loop.go:552) — the EXACT call site scout's Task 1 asks to
//     add, already present, right where scout said to put it (next to
//     `reloadFleetConfigAtWaveBoundary` / `syncMainFromOriginAtWaveBoundary`,
//     cmd_loop.go:535-559).
//
// This is the SAME finding cycles 1343, 1352, and 1356 already made and
// pinned for this same recurring inbox item (go/acs/cycle1343,
// go/acs/cycle1352, go/acs/cycle1356 — all present on this worktree's HEAD).
// The item keeps re-entering fleet_scope (queue-hygiene residual, tracked in
// operator memory as "queued at priority 0.55") faster than the consuming
// commit lands; re-verified fresh for cycle 1368:
//
//	go build ./...                                          -> clean
//	go test -run BoundaryRefresh ./cmd/evolve/...            -> all PASS
//
// No new production code is warranted. This cycle's predicates
// REGRESSION-LOCK the already-shipped contract for THIS cycle's audit gate
// (ACS predicates are cycle-scoped, never replayed by a later gate —
// test-report.md Step 6b) via named `go test -run` subprocess assertions,
// the established idiom for `package main` coverage from `go/acs`
// (cycle-1352/1356 precedent).
//
// Adversarial diversity:
//
//	C1368_001 positive — the wave-boundary call site scout's Task 1 literally
//	                      asks for exists and fires before dispatch, never
//	                      mid-lane (AC1/AC2).
//	C1368_002 positive — the chain-boundary call site (the other half of the
//	                      same seam) fires strictly before every batch, never
//	                      mid-batch (AC1, chain-mode coverage).
//	C1368_003 positive — the fail-open contract: any staleness/rebuild check
//	                      failure degrades to refreshed=false, batch/chain
//	                      keeps running on the old binary (AC3).
//	C1368_004 positive — the refresh is ledgered under a distinguishable
//	                      "boundary-refresh" authorization class (AC4).
//	C1368_005 negative — no second, duplicate stop-only staleness code path
//	                      has been (re-)introduced alongside the shipped
//	                      rebuild+re-pin+re-exec design.
```

### `go/acs/cycle1368/predicates_test.go:87` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

### `go/acs/cycle1368/predicates_test.go:177` — above `func TestC1368_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {`

```text
// TestC1368_005_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal wording (and its predecessors', cycles 1343/1352/
// 1356) each pinned that a WEAKER stop-only design must never resurface
// alongside the shipped rebuild+re-pin+re-exec boundary hook. Fails loudly
// if a future change reintroduces that duplicate, narrower code path.
```

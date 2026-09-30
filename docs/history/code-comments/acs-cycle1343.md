# Comment history: `acs/cycle1343`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1343/predicates_test.go:3` — above `package cycle1343`

```text
// Package cycle1343 materializes the acceptance criteria of this lane's sole
// top_n task, `chain-boundary-binary-refresh` (triage-report.md "## top_n";
// scout-report.md Task 1; fleet-scoped inbox item
// `auto-refresh-binary-at-boundary`).
//
// FINDING (read-first, rule 8 — this cycle changes NO production code): the
// scout report's premise does not match the repo on disk. It cites
// `bootBinaryRefreshFn` and `cmd_loop_boot_refresh.go:77,145-151` as the
// existing boot-time seam to reuse at the chain boundary, and
// `docs/chronicle/2026-08-binary-lag.md` as the motivating chronicle. None of
// the three exist anywhere in this worktree (`grep -rn bootBinaryRefreshFn
// go/` and `find go -iname '*boot_refresh*'` both return nothing; there is no
// `docs/chronicle/` directory at all).
//
// What DOES exist, already fully built and wired, is the FUNCTIONAL
// equivalent the AC set actually asks for: `maybeRefreshChainBoundary`
// (go/cmd/evolve/cmd_loop_chain.go) — shipped across cycles 1314/1320/1323/
// 1325/1330 (see docs/operations/runtime-reference.md:67, "Boundary binary
// auto-refresh"). It is called at the chain boundary
// (cmd_loop_chain.go:578, inside runLoopChain, before every batch) AND at the
// wave/fleet boundary (cmd_loop.go:552, inside runLoopBatch's per-iteration
// loop) — i.e. it already covers the chain case this task names AND the
// "single-batch/non-chain unchanged" case the AC set separately worries about
// (that second call site is cycle-1325's addition, pre-dating this cycle).
// Exact per-boundary call-count and ordering are already pinned by
// TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch and
// TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch
// (cmd_loop_chain_boundaryrefresh_test.go), and the wave-boundary wiring is
// pinned by TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary /
// TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers
// (cmd_loop_wave_boundaryrefresh_wiring_test.go).
//
// The scout AC also asks for an "off dial" reusing policy.BootBinaryRefresh()
// — that function does not exist either, and the shipped design is
// documented as deliberately dial-less ("n/a (no opt-out flag)",
// runtime-reference.md:67): every failure mode degrades fail-open by
// construction (ahead-check error, rebuild error, provenance-refused repin,
// re-exec error all skip the refresh and keep the chain running), so a kill
// switch was a considered omission, not a gap. Adding one now would fork a
// SECOND disable path alongside an already-hardened, already-shipped
// fail-open design — the standing no_feature_flag_sprawl /
// never_duplicate_centralize_via_design_patterns rules cut against inventing
// one to satisfy a premise that was never real. See test-report.md
// AC-Materialization for the full disposition (predicate x2,
// unverifiable-remove x1, manual+checklist x1).
//
// EXECUTION SHAPE: the subject (maybeRefreshChainBoundary and its call sites)
// lives in `package main` (go/cmd/evolve) — an external acs package cannot
// import it. Following the cycle1340 precedent (defect_ledger_worktree_
// evidence_test.go), these predicates shell the REAL production test binary
// with `-run` narrowed to the exact named tests that already drive the real
// seams, and demand the `--- PASS: <name>` receipt so a deleted/skipped test
// reads as a miss, never a silent pass.
//
// Both predicates below are PRE-EXISTING GREEN — see test-report.md RED Run
// Output. They are authored anyway as this cycle's regression pin
// (AC-Materialization requires a `predicate` artifact for every
// predicate-dispositioned AC; a GREEN result on unmodified shipped code is an
// explicitly-sanctioned outcome, not a violation — Step 4's "pre-existing
// GREEN" carve-out) so a future regression in either boundary's wiring is
// caught by THIS cycle's card, not silently absorbed into an unrelated one.
//
// Adversarial diversity:
//
//	C1343_001 positive — chain-boundary refresh fires exactly once per
//	                      boundary, strictly BEFORE that boundary's batch, and
//	                      a trip stops the chain before running that
//	                      boundary's batch (no mid-batch refresh).
//	C1343_002 positive — the SAME wave/fleet-boundary call site fires for a
//	                      chain-less `evolve loop` run too, and never fires
//	                      from inside a dispatch helper (never mid-lane).
```

### `go/acs/cycle1343/predicates_test.go:95` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

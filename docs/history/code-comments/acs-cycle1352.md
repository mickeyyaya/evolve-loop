# Comment history: `acs/cycle1352`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1352/predicates_test.go:3` — above `package cycle1352`

```text
// Package cycle1352 materializes the acceptance criteria of this lane's two
// top_n tasks — `chain-boundary-binary-refresh-stop` and
// `chain-summary-refresh-event-field` (triage-report.md "## top_n";
// scout-report.md Tasks 1/2; fleet-scoped inbox item
// `auto-refresh-binary-at-boundary`).
//
// FINDING (read-first, rule 8 — this cycle changes NO production code): the
// scout report's premise does not match the repo on disk, for the SAME
// reason cycle-1343 already found and pinned (go/acs/cycle1343/predicates_
// test.go). Re-verified fresh for this cycle:
//
//	grep -rn 'bootRefreshHeadFn\|bootRefreshBinaryCommitFn\|bootRefreshSource
//	DeltaFn\|BootBinaryRefresh\|bootBinaryRefresh\|chain_binary_stale\|
//	StaleAtBoundary\|StaleBinaryCommit' go/   -> zero hits outside this file
//	find docs/chronicle -iname '*binary-lag*'  -> no chronicle directory exists
//
// None of the seams the scout report names as reuse targets
// (bootRefreshHeadFn, bootRefreshBinaryCommitFn, bootRefreshSourceDeltaFn,
// policy.Load(...).BootBinaryRefresh()) or the chronicle doc it cites exist
// anywhere in this worktree.
//
// What DOES exist, already fully built, tested, and wired at BOTH call
// sites the AC set cares about, is the functional superset:
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go, shipped
// across cycles 1314/1320/1323/1325/1330, documented at
// docs/operations/runtime-reference.md "Boundary binary auto-refresh").
//
// Task 1's AC set, read literally, asks for a *stop-only* handoff (rc=0,
// reason `chain_binary_stale`, "does not rebuild or re-exec itself — stays
// boot-owned") because the scout reasoned re-exec-from-inside-the-chain was
// unsafe. The shipped design proves that reasoning's premise wrong: it DOES
// rebuild + provenance-gate + re-pin + re-exec from inside the boundary
// (maybeRefreshChainBoundary, cmd_loop_chain.go:400-484), and does so
// SAFELY — `res.Batches`/`chainResult` accumulated so far is marshaled and
// printed to stdout BEFORE the re-exec (runLoopChain, cmd_loop_chain.go:
// 578-586), and a loop-breaker marker (chainBoundaryRefreshAttemptFile)
// refuses a second re-exec attempt on the same build commit so the process
// can never livelock at zero batches run. This is a strictly stronger
// contract than the scout's stop-and-wait-for-the-next-launch design: no
// operator/wrapper relaunch is needed at all. Re-litigating it down to a
// weaker stop-only path, or duplicating a second `chain_binary_stale`
// code path alongside the shipped `chain_boundary_refresh_reexec` one,
// would violate the standing no_workaround_root_cause_redesign and
// never_duplicate_centralize_via_design_patterns rules — see
// test-report.md AC-Materialization for the full disposition.
//
// Task 2's AC (surface the refresh event in the summary) IS already
// satisfied: `chainResult.BoundaryRefresh *chainBoundaryRefreshLogEntry`
// (json:"boundary_refresh,omitempty") carries Batch/OldSHA/NewSHA/Timestamp
// when a boundary refresh fires, and is nil/omitted on every ordinary stop —
// the identical shape the AC's `stale_at_boundary`/`stale_binary_commit`
// pair asks for (richer, since it also carries the commit pair and
// timestamp a dossier consumer needs, not just a boolean+string).
//
// Both predicates below are PRE-EXISTING GREEN — see test-report.md RED Run
// Output. They are authored anyway as THIS cycle's regression pin
// (AC-Materialization requires a `predicate` artifact for every
// predicate-dispositioned AC; a GREEN result on unmodified shipped code is
// the explicitly-sanctioned "pre-existing GREEN" carve-out, Step 4) so a
// future regression in either boundary's wiring or the summary field is
// caught by THIS cycle's card, not silently absorbed into cycle-1343's.
//
// Adversarial diversity:
//
//	C1352_001 positive — the chain-boundary refresh (Task 1's functional
//	                      equivalent) fires strictly before every batch,
//	                      never mid-batch, and a trip stops the chain
//	                      before running that boundary's own batch.
//	C1352_002 positive — the refresh event (Task 2) surfaces into
//	                      chainResult's JSON when a boundary refresh fires,
//	                      and is omitted on every ordinary (non-refresh)
//	                      stop.
//	C1352_003 negative — no second, duplicate `chain_binary_stale` reason
//	                      string or `StaleAtBoundary`/`StaleBinaryCommit`
//	                      field exists anywhere in the chain source — i.e.
//	                      nobody has (re-)introduced the weaker, superseded
//	                      stop-only design the scout report's literal AC
//	                      wording describes.
```

### `go/acs/cycle1352/predicates_test.go:102` — above `func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
```

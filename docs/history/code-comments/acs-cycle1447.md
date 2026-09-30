# Comment history: `acs/cycle1447`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1447/predicates_test.go:3` — above `package cycle1447`

```text
// Package cycle1447 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane:
//
//	fill-verdict-correlation-report-land — the CONTINUATION of cycle-1402.
//
// Cycle-1402 authored the whole feature and then died at ship on the
// repo-contract scanner pack ("[REPO_CONTRACT_GATE/precondition @atomic-ship]
// … RED in the lane worktree"), off a base that is now four releases stale.
// Predicates 001–005 are cycle-1402's contract carried forward verbatim (the
// package was renamed 1402→1447 so `evolve acs suite --cycle 1447` binds it —
// a predicate package the current cycle does not name is a package the gate
// never runs). Predicate 006 is NEW and pins the thing that actually killed
// the prior ship: the scanner pack must be GREEN in THIS lane.
//
//	Feature context — part (3) of inbox item
//	`context-fill-telemetry-and-cap` (P1, weight 0.89). Parts (1) and (2) —
//	the per-phase fill-ratio derivation (internal/contextfill) and its durable
//	wiring onto phasetiming.Entry.ContextFillRatio — already landed in
//	cycle-1271. What does NOT exist is the join: nothing anywhere correlates
//	fill% at dispatch against the cycle's final verdict, which is the evidence
//	that promotes or demotes the whole tokenopt band.
//
// The contract Builder must satisfy, pinned by the predicates below:
//
//   - a new leaf package go/internal/contextfillcorrelate exposing a PURE
//     join/bucket function Correlate([]CycleFill) Report plus a Load() that
//     reads the real corpus (knowledge-base/cycles/cycle-*.json dossiers for
//     final_verdict, .evolve/runs/*/phase-timing.json for ContextFillRatio);
//   - a CLI subcommand `evolve context-fill correlate` reachable from the
//     top-level dispatch table (registry.go), emitting --json to stdout and a
//     markdown artifact via --out;
//   - explicit "no data" handling: a cycle with no usable fill ratio or no
//     verdict is reported in NoData, NEVER bucketed as a fabricated 0.0;
//   - ADR-0069 new-package graduation for the new package (enrolment in
//     go/.apicover-enforce plus an apicover_named_test.go that names and
//     exercises every exported symbol).
//
// Predicate strategy — every load-bearing assertion EXERCISES the system under
// test (the cycle-85 degenerate-predicate ban):
//
//   - 001 calls Correlate directly on a crafted fixture and asserts the
//     bucketing arithmetic and the high-fill/low-fill FAIL-rate ordering.
//   - 002 is the negative/edge predicate: missing fill data and a missing
//     verdict must land in NoData and be absent from every bucket, and an
//     all-PASS corpus must yield finite 0 rates (never NaN from a 0/0).
//   - 003 is the CLI WIRING PROOF: it builds the real `evolve` binary from this
//     worktree and invokes `evolve context-fill correlate` through top-level
//     dispatch against a synthetic project root, asserting the emitted JSON and
//     the emitted markdown artifact. A seam whose only caller is a unit test
//     would leave this predicate RED.
//   - 004 runs the same binary against the REAL repo corpus and asserts no
//     silent drops: joined + no-data must account for every dossier on disk.
//   - 005 is the ADR-0069 graduation predicate: it runs the new package's
//     apicover named test, so an unenrolled or unnamed export stays RED.
//   - 006 is this cycle's own criterion: it RUNS each of the four repo-contract
//     scanner suites the ship gate runs, one named package per invocation, in
//     this lane's module dir — the gate's own precondition, proved in-lane
//     before ship rather than discovered at ship for a second time.
//
// Bucket boundary note: the hot bucket's lower bound is asserted against
// contextfill.HotThreshold rather than a re-declared 0.85 literal, so the
// inclusive hot boundary keeps exactly one definition in the tree.
```

### `go/acs/cycle1447/predicates_test.go:447` — above `func TestC1447_005_new_package_apicover_graduation(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 005 — ADR-0069 new-package graduation for internal/contextfillcorrelate.
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1447/predicates_test.go:480` — above `var repoContractSuites = []string{`

```text
// ---------------------------------------------------------------------------
// 006 — the repo-contract scanner pack is GREEN in THIS lane (cycle-1447).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1447/predicates_test.go:496` — above `func TestC1447_006_repo_contract_scanner_pack_green_in_lane(t *testing.T) {`

```text
// TestC1447_006_repo_contract_scanner_pack_green_in_lane is this continuation's
// own acceptance criterion. Cycle-1402's ship died here:
//
//	[REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack
//	RED in the lane worktree (exit status 1) — pushing would red main
//
// and the reason was never diagnosed, so the whole feature has sat stranded.
// The predicate RUNS each suite in this lane's module dir and requires exit 0,
// which is exactly the gate's precondition — a landing that reintroduces the
// prior RED (a stray/invalid `.evolve/phases/<name>/phase.json` overlay, an
// unpaired phase↔agent pair, a routing-table drift) fails here rather than at
// ship. Each suite is invoked on ONE named package with an explicit -C module
// dir, never a `./...` sweep and never cwd-relative `go` — the lane worktree is
// not the process cwd.
//
// The auxiliary check afterwards pins the suite list against the gate's own
// declaration so this predicate cannot silently drift into testing a subset of
// what the gate enforces.
```

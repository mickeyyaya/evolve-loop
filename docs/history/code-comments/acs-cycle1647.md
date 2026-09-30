# Comment history: `acs/cycle1647`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1647/predicates_test.go:3` — above `package cycle1647`

```text
// Package cycle1647 materializes the acceptance criteria of this fleet lane's
// single committed task:
//
//   - overlay-family-name-transport-ambiguity (triage `## top_n`; the lane
//     pin in lane-scope.json names the same id) — a routing overlay naming a
//     bare FAMILY must not cross transport when the phase's chain holds a
//     non-default driver of that family, on the ADVISOR projection path.
//
// The acceptance is the claimed inbox record's `acceptance` array, verbatim
// (.evolve/inbox/processing/cycle-1647/2026-07-30T23-05-00Z-overlay-family-
// name-transport-ambiguity.json). The task-contract block rendered the record
// as unreadable because triage had already moved it from pending to
// processing/; the array is unchanged by the move.
//
// # What is already built, and what this package adds
//
// Commit 797b8518 landed the RESOLVER half: llmroute.ApplySoftOverlay resolves
// ov.CLI in three rungs (exact chain entry > bare-family same-family entry >
// defaultDriverForFamily), and overlay_family_transport_test.go pins it at the
// helper. Verified here by driving the callers, not by reading code:
//
//  1. THE PACKAGE DOC DOES NOT CARRY THE DECISION. `go doc ./internal/llmroute`
//     prints the package comment from llmroute.go, which still describes the
//     pre-overlay precedence table and says nothing about family-vs-driver
//     selector semantics; the decision lives only on the ApplySoftOverlay
//     function comment. AC1 asks for one sentence in the PACKAGE doc — the
//     ambiguity was the defect, so the decision must be discoverable at the
//     package boundary. 001 is RED.
//  2. THE PRODUCTION ADVISOR CALLER IS UNPROVEN. runner.resolveDispatchPlan
//     (internal/phases/runner/routing.go:71) is where the advisor projection
//     reaches ApplySoftOverlay. No runner test drives a bare-family overlay
//     over a headless chain, and none drives an explicit driver over a
//     same-family headless entry: `go test -run
//     '^TestResolveRouting_AdvisorOverlayPreservesFamilyTransport$'
//     ./internal/phases/runner` reports "no tests to run". 004 is RED.
//  3. THE BEHAVIOR ITSELF, THROUGH THE RUNNER, IS ALREADY CORRECT. 002 and
//     003 drive runner.New(...).Run — the real BaseRunner over an on-disk
//     profile and a recording bridge — so resolveDispatchPlan is reached from
//     its production entry point, and they observe the dispatched CLI. Both
//     are GREEN on this tree (the resolver fix is merged and the runner passes
//     the advisor's string through un-normalized). They stay as the
//     anti-gaming floor: a Builder who satisfies 004 with a trivially-passing
//     named test still cannot regress the transport contract without 002/003
//     going red, and a future caller that normalizes ModelRoutingCLI before
//     the seam (the scout's beyond-the-ask hypothesis) trips 002 directly.
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002 forbids
// claude-tmux ever being dispatched for the bare overlay (the historical
// crossing); 003 forbids the naive family-match answer (claude-p first) for an
// explicit claude-tmux. EDGE — 003 exercises the fallback walk (exit 80 on the
// explicit driver) to prove the headless entry is RETAINED, not replaced.
// SEMANTIC — family identity preserves transport (002), explicit driver
// changes it (003), the decision is documented at the package boundary (001),
// the projection path is covered and its caller named (004), the package
// stays race-clean with every export exercised (005), the predicate package
// and eval are git-tracked (006): six distinct behaviors, not one restated.
//
// # Audit round 1 (same cycle) — continuation hygiene, 007-008
//
// Round 1 passed 001-006 and FAILED the shipping tree: go/acs/cycle1638 (added
// by this diff, named as evidence by the tracked unified-synthesis eval)
// resolved its explanation by a cycle-1638-*.md glob the host archives on
// every continuation, so TestC1638_010/011 were RED and nobody ran them (H1);
// the 1647 explanation repeated the two defects they pin (M1) and cited the
// two root inbox records at `:1` although the diff deletes them (correction
// 3). Reconciled at the TDD seam: cycle1638's helper now resolves the record
// the tree ships; 007 runs every inherited go/acs/cycle* package the diff
// adds (derived from git, one named package each) so the harness lane reaches
// them; 008 forbids a `path:line` citation into a deleted path. All three go
// GREEN with edits to the DOCUMENT only (probed at RED).
//
// Flaky-shape hygiene: every `go` subprocess names ONE package (./internal/
// phases/runner with -run narrowed; ./internal/llmroute; ./internal/router —
// measured 1.3s / 1.9s -race / 1.3s), no `/...` sweep, no ./internal/core or
// ./cmd/evolve (the AC5 `-race ./internal/core` half is a whole-suite shape
// the predicate lint bans; it is delegated to CI + the Builder's pasted run,
// see test-report.md), no wall-clock bounds, no literal PIDs, every git call
// is -C rooted, every go call sets cmd.Dir, no load generators. 007's nested
// runs each name ONE inherited ./acs/cycle<N> package (measured 1.0s / 1.8s /
// 3.1s), never ./acs/... .
```

### `go/acs/cycle1647/predicates_test.go:451` — above `func TestC1647_006_PredicatePackageAndEvalAreGitTracked(t *testing.T) {`

```text
// TestC1647_006_PredicatePackageAndEvalAreGitTracked — cycle-93 lesson: the
// audit's predicate tree and the ship tree must agree. Disk presence alone
// passes for a gitignored file that is dropped at ship; pair it with an
// index check (`git ls-files --error-unmatch`), -C rooted at the worktree.
```

### `go/acs/cycle1647/predicates_test.go:471` — above `const thisPackage = "cycle1647"`

```text
// ---------------------------------------------------------------------------
// 007-008: continuation hygiene — cycle-1647 audit round 1 (H1, M1 corr. 3)
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1647/predicates_test.go:592` — above `func TestC1647_007_InheritedPredicatePackagesAddedByThisDiffStayGreen(t *testing.T) {`

```text
// TestC1647_007 pins cycle-1647 audit H1: a continuation ships every inherited
// go/acs/cycle* package its base-bound diff ADDS, and the tracked eval
// (.evolve/evals/triage-unified-solution-synthesis.md) names their tests as
// evidence — so a RED inherited package is a RED shipping tree, whether or not
// the harness's own lane (`./acs/cycle1647` only, acssuite.goLanePatterns)
// happens to run it. The set is derived from the diff, never hard-coded; each
// package is run as ONE named `go test` (no `/...` sweep). RED on this tree:
// go/acs/cycle1638 TestC1638_010/011 fail against the shipping explanation
// (phase-registry.json unnamed; this diff's own package attributed to
// history — the audit's M1). The Builder's fix is the DOCUMENT, never the
// tests.
```

### `go/acs/cycle1647/predicates_test.go:631` — above `func TestC1647_008_ExplanationCitesNoLineIntoAPathThisDiffDeletes(t *testing.T) {`

```text
// TestC1647_008 pins cycle-1647 audit correction (3): a `path:line` citation
// is a promise the reader can open that file at that line on the shipped
// tree. The explanation cites `.evolve/inbox/…-overlay-family-name-transport-
// ambiguity.json:1` and `…-triage-unified-solution-synthesis.json:1` as its
// source records, yet this diff DELETES both root copies (they live on under
// .evolve/inbox/consumed/), so the citations resolve only through `git show
// <base>:…`. Deleted paths may still be NAMED (`## Changed Areas` explains
// the removal); they may not be cited at a line. The deleted set is derived
// from git (`--diff-filter=D`), so any future dead line-citation fails the
// same way; a diff that deletes nothing has nothing to assert and says so.
```

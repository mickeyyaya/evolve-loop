# Comment history: `acs/cycle1439`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1439/predicates_test.go:3` — above `package cycle1439`

```text
// Package cycle1439 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `salvage-worktree-relanding` (triage-report.md ## top_n).
//
// What this cycle is. Not new design: a LANDING. The stranded worktree
// .evolve/worktrees/cycle-42824668-1407 (branch cycle-42824668-1407, snapshot
// 04d3dee1, continuation records `task-a-salvage-extraction-stage` /
// `task-b-decoy-sentinel-fixture`) holds complete, never-landed work — the
// quote-aware + tail-anchored `ownSentinelPayload` selector, the
// `SummarizeBadVerdictBaseline` reader, and the `evolve salvage report` CLI —
// blocked on one isolated correctness defect: `isQuotedEcho` treats a single
// adjacent backtick as proof of a CLOSED inline-code span, so one stray
// unmatched backtick suppresses a report's own genuine verdict sentinel
// (cycle-1407 adversarial finding F1).
//
// Predicate strategy. Every predicate exercises the system: predicates 001-005
// call `deliverable.ClassifyBadVerdict` directly and assert on its returned
// classification; 006-008 build and drive the REAL CLI entry point
// (go/cmd/evolve, via the registry.go dispatch table) as a subprocess and assert
// on its emitted JSON / exit codes; 009 runs the named unit + apicover tests in
// internal/deliverable. No predicate here is load-bearing on a source grep —
// the cycle-85 degenerate-predicate ban.
//
// Wiring proof, not unit proof. 006-008 deliberately reach the salvage reader
// through `evolve salvage report`, never by calling SummarizeBadVerdictBaseline
// directly: a reader whose only caller is a test is dead code, and the whole
// point of this landing is that the sidecar written since cycle-1389 finally has
// a production reader an operator can run.
//
// RED baseline (this worktree, main-based). 002/003 fail because today's
// ClassifyBadVerdict takes the FIRST sentinel-shaped span with no quote
// awareness at all; 006/007/008 fail because go/cmd/evolve/cmd_salvage.go and
// go/internal/deliverable/salvage_report.go do not exist on main; 009 fails
// because the named guard/apicover tests are worktree-only. 001/004/005 are
// pre-existing GREEN and are pinned as regression guards: they are exactly the
// cases a naive fix ("drop quote-awareness" / "last-match-wins only") would
// break, so they must stay green THROUGH the landing.
```

### `go/acs/cycle1439/predicates_test.go:376` — above `func TestC1439_009_NamedGuardAndApicoverTestsPass(t *testing.T) {`

```text
// TestC1439_009_NamedGuardAndApicoverTestsPass runs the unit tests the landing
// owes, by name, in ONE package with a narrowed -run (never a ./... sweep):
//
//   - the three isQuotedEcho guard cases the build plan enumerates, and
//   - the apicover named test that exercises the newly exported
//     SummarizeBadVerdictBaseline / BaselineSummary. internal/deliverable is
//     already enrolled in go/.apicover-enforce:237, so every exported symbol the
//     landing adds must be named in a real assertion or the repo-wide apicover
//     gate (ADR-0069's second gate) fails the build.
//
// Asserting on `--- PASS: <name>` lines, and rejecting "no tests to run", is
// what makes this a coverage proof rather than a vacuous exit-0.
```

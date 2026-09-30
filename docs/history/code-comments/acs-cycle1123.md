# Comment history: `acs/cycle1123`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1123/predicates_test.go:3` — above `package cycle1123`

```text
// Package cycle1123 materializes the cycle-1123 acceptance criteria for this
// fleet lane's sole committed item, `fatalpane-strip-agent-content` (lane-scope
// pins this lane to that one todo-id; per R9.3 no predicate here binds to a
// deferred or other-lane item).
//
// TASK NATURE — the SECOND call site. Cycle-1117 closed the bridge half: the
// stop-review fast-fail (bridge.fatalPaneVerdict) now reads an agent-STRIPPED
// pane, so an agent editing the fatal registry is no longer killed on its own
// edit buffer. The registry's OTHER consumer stayed raw:
// core.adviseOnUnclassifiedFailure (failure_hook.go:87) calls
// det.Detect(report.FinalPane) unstripped. (scout-report.md named this site
// "fatalpane_persistence.go:73" — no such file exists; the real second raw
// Detect consumer is failure_hook.go:87. Same class, corrected file.)
//
// There the defect is INVERTED: the raw match drives the deterministic-first
// SHORT-CIRCUIT ("pane already classified; skipping advisor"), so agent-authored
// diff content quoting a seed makes a genuinely novel wedge look known — no C3
// consultation, no promotion, and every recurrence burns the maxExtends
// backstop again. ADR-0044's learning loop, disabled by the agent's own text.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 "a single exported stripper recovery.StripAgentContent(pane,
//	     injectedPrompt, protected) implements the cycle-1117 semantics
//	     (blank-in-place D1, protect-list D2, fail-open empty prompt)"
//	    -> C1123_001 (named PASS for the recovery contract tests)
//	     + C1123_005 (exercised IN-PROCESS against the live registry).
//	AC2 "bridge.strippedForFatalPaneScan DELEGATES to it — one copy of the
//	     rules, not two"
//	    -> C1123_003 (mutation: neuter recovery.StripAgentContent => the
//	       cycle-1117 bridge diff test MUST die. A bridge that kept its own
//	       copy survives this and is rejected).
//	AC3 "adviseOnUnclassifiedFailure strips before Detect: an agent-diff line
//	     quoting a seed no longer suppresses the advisor"
//	    -> C1123_001 + C1123_002 (mutation: pass-through strip => the core
//	       diff test MUST die — the only predicate that tells a WIRED strip
//	       from an inert helper that merely exists).
//	AC4 "deterministic-first is NOT weakened: real CLI chrome, and a genuine
//	     newline-anchored dead shell sitting under agent diff content, both
//	     still short-circuit"
//	    -> C1123_001 (both are negative tests) + C1123_004 (mutation: the
//	       delete-based strip => the anchored test MUST die, replaying D1 at
//	       this call site).
//	AC5 "go test ./internal/core/... ./internal/bridge/... ./internal/recovery/...
//	     green, no regression"
//	    -> C1123_006, which additionally requires a NAMED pass for every
//	       pre-existing hook and fatal-pane test (a bare exit 0 cannot see a
//	       deleted inconvenient test).
//
// Adversarial axes. NEGATIVE: C1123_004 and the two negative core tests assert
// the system must NOT stop classifying real fatal panes — the lazy over-fix
// (strip everything) is killed there; 002/003/004 assert the new tests
// themselves DIE under mutation, so a tautological test is killed here. EDGE:
// 001 rejects `go test -run <nonexistent>`'s vacuous exit 0; 005 covers the
// empty-pane, empty-prompt, blank-protect-entry and nil-registry boundaries;
// 006 rejects exit-0-with-a-test-deleted. SEMANTIC: wiring (002), single-source
// delegation (003), line-preservation (004) and suite health (006) are four
// distinct behaviours, not one restated.
//
// No source-grep predicates (cycle-85 rule): every predicate below either
// exercises the system in-process (005) or runs it as a subprocess and asserts
// on real emitted output (001-004, 006). Mutants are applied via
// `go test -overlay` — the real tree is never written.
```

### `go/acs/cycle1123/predicates_test.go:97` — above `coreDiffTest     = "TestC1123_AgentDiffQuotedSignatureStillReachesAdvisor"`

```text
// The cycle-1123 contracted tests. Naming them individually is what lets
// the mutation predicates say WHICH test a mutant must kill.
```

### `go/acs/cycle1123/predicates_test.go:109` — above `bridgeDiffTest = "TestC1117_AgentDiffSeedTextDoesNotFastFail"`

```text
// bridgeDiffTest is cycle-1117's agent-diff test. It is the delegation
// witness: it can only die under a mutation of recovery.StripAgentContent
// if the bridge seam actually routes through it (AC2).
```

### `go/acs/cycle1123/predicates_test.go:129` — above `var preExistingFatalPaneTests = []string{`

```text
// preExistingFatalPaneTests guard the cycle-1117 half against a regression
// introduced while lifting its stripper into recovery (AC2/AC5).
```

### `go/acs/cycle1123/predicates_test.go:171` — above `func TestC1123_003_bridge_diff_test_dies_under_the_same_mutation(t *testing.T) {`

```text
// TestC1123_003_bridge_diff_test_dies_under_the_same_mutation is AC2, the
// single-source proof. The cycle-1117 bridge test can only notice a mutation of
// recovery.StripAgentContent if bridge.strippedForFatalPaneScan DELEGATES to
// it. A bridge that keeps its own copy of the rules passes its own suite and is
// rejected here — which is exactly the drift this cycle exists to prevent.
```

### `go/acs/cycle1123/predicates_test.go:266` — above `const deleteBasedMutant = '_ = injectedPrompt`

```text
// deleteBasedMutant is cycle-1115's rejected shape: line-DELETING instead of
// blanking, which collapses the newline anchors (D1).
```

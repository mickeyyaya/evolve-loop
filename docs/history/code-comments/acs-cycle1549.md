# Comment history: `acs/cycle1549`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1549/predicates_test.go:3` — above `package cycle1549`

```text
// Package cycle1549 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `premise-challenge-learning-real-path`
// (triage-report.md ## top_n).
//
// What this cycle is. Not new production behavior: a COMPOSED-PATH PROOF. #479
// made a judgment phase's FAIL verdict leave a carryover lesson without a halt
// vector; #481 made a well-formed verdict sentinel able to produce that FAIL
// instead of being structurally forced to PASS. Both are unit-proven in
// isolation — judgment_lesson_test.go injects a hand-built PhaseResponse, and
// the specrunner sentinel tests stop at classification. Nothing drives ONE real
// sentinel through parse → verdict → recordAndBranch → persisted state →
// next-cycle planner context. That seam can be cut while every existing test
// stays green, which is precisely the regression this cycle must make loud.
//
// Predicate strategy. Every predicate here is BEHAVIORAL: it builds and runs the
// real `internal/core` test binary as a subprocess against a named subtest and
// asserts on the emitted `--- PASS:` line. Exit code alone is deliberately NOT
// load-bearing — `go test -run` over a pattern that matches nothing exits 0 with
// "no tests to run", so an exit-code-only predicate would go GREEN on an absent
// test (the cycle-131/137 vacuity trap). The PASS-line assertion is what keeps
// these RED today and what makes a rename fail loudly instead of silently.
//
// SCOPE NARROWING, compiler-proven (the cycle-644 reachability rule). The task's
// AC says the FAIL must come from "real sentinel parsing", and the obvious
// reading is "call specrunner.EvaluateClassify". That is IMPOSSIBLE and no
// predicate here may pin it: internal/phases/specrunner imports internal/core,
// so a `package core` test importing specrunner is an import cycle. Probed:
//
//	imports .../internal/phases/specrunner from zz_probe_import_test.go
//	imports .../internal/core from specrunner.go: import cycle not allowed in test
//
// So the composed path is pinned one layer down, at the parser BOTH layers
// already share: phasecontract.ParseVerdictSentinelFull — the exact function
// specrunner's applySentinelStage calls, and a package internal/core already
// imports in production (build_removal_check.go, cyclerun_remediate.go). That is
// the single-source reading, not a second grammar: a drift in the sentinel
// vocabulary breaks this test and the classifier together, which is the property
// the AC is actually after.
//
// RED baseline (this worktree, main-based). 001-004 fail because
// TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting
// and TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL do not
// exist in go/internal/core/judgment_lesson_test.go — `go test` exits 0 and
// prints no PASS line, so each predicate reports the vacuity explicitly.
//
// Adversarial diversity. NEGATIVE — 004 is the anti-no-op predicate: a stated
// PASS, a malformed sentinel, and an absent sentinel must ALL leave zero
// lessons, so an implementation that files a lesson unconditionally (the
// cheapest way to pass 001-003) fails here. EDGE — 004's malformed and absent
// rows are the fail-open boundary; 003 pins the FailedAt array UNCHANGED, not
// merely "small". SEMANTIC — parse-produces-FAIL, persistence-reaches-planner,
// non-halt, and fail-open-negatives are four distinct behaviors, not one
// restated.
```

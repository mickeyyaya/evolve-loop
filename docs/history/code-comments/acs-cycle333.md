# Comment history: `acs/cycle333`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle333/predicates_test.go:3` — above `package cycle333`

```text
// Package cycle333 materializes the cycle-333 acceptance criteria for the one
// committed top_n task (scout-report.md "## Selected Tasks"):
//
//	hoist-subagent-json-extractors — replace the three per-call
//	    `regexp.MustCompile(fmt.Sprintf(...))` JSON field extractors in
//	    internal/subagent (extractProfileString in modeltier.go,
//	    extractInt in ctxadvisory.go, extractBoolField in dispatchparallel.go)
//	    with package-level static regexp vars + a thin matchField helper, so no
//	    dynamic regexp is compiled per call. Behaviour MUST be byte-identical;
//	    only the compilation timing (per-call → package init) changes.
//
// Predicate design (cycle-85 lesson — every gate EXERCISES the system under
// test, no load-bearing source grep):
//
//   - C333_001..003 are BEHAVIORAL + structural (the "Mixed" category): each
//     drives the real exported entry point that consumes one of the three
//     extractors (ResolveModelTier → extractProfileString, CheckCtxAdvisory →
//     extractInt, DispatchParallel → extractBoolField), asserting the extracted
//     value still steers the observable outcome (positive AND negative/missing-
//     field cases — the anti-no-op axis). The behavioral half passes today
//     (the refactor preserves behaviour); the auxiliary structural half
//     ("this file no longer compiles a regexp from fmt.Sprintf") is RED today
//     and is what fails until Builder hoists the pattern. So each predicate is
//     RED now for the RIGHT reason (refactor not done) yet pins behaviour so a
//     no-op or a behaviour-breaking edit cannot pass.
//
//   - C333_004 is the structural goal-completion gate (waived config/structure
//     check, see the inline waiver): across the three target files, ZERO
//     dynamic `regexp.MustCompile(fmt.Sprintf` calls remain AND the package
//     gained >= 5 static package-level regexp vars (the hoisted field
//     patterns). RED today (3 dynamic compiles present, only 3 static vars in
//     these files).
//
// Floor binding (R9.3): internal/subagent is the SOLE package the one committed
// top_n task targets — every predicate here binds committed work. No triage-
// DEFERRED item (guards/docdelete, quotareset) gets a predicate (cycle-280
// lesson: a deferred-task floor starves the committed task).
//
// AC map (1:1 with the task's Acceptance Criteria Summary):
//
//	AC-1 no dynamic regexp.MustCompile(fmt.Sprintf in prod subagent  → C333_004 (+ per-file in 001/002/003)
//	AC-2 >= 5 static package-level regexp vars added to subagent      → C333_004
//	AC-3 all subagent tests pass (behaviour preserved)               → C333_001..003 exercise the real APIs
//	AC-4 no regexp.MustCompile inside any surviving extractXxx body   → C333_004 (no dynamic compile anywhere)
//	AC-5 extractProfileString behaviour preserved                    → C333_001
//	AC-6 extractInt behaviour preserved                              → C333_002
//	AC-7 extractBoolField behaviour preserved                        → C333_003
```

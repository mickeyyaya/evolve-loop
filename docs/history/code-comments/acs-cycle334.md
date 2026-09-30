# Comment history: `acs/cycle334`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle334/predicates_test.go:3` — above `package cycle334`

```text
// Package cycle334 materializes the cycle-334 acceptance criteria for the one
// behavior-preserving, build-ready task committed to triage `## top_n`:
//
//	hoist-quotareset-hint-re — replace the per-call
//	    `re := regexp.MustCompile("(?i)(\d{1,2}):(\d{2})(am|pm)")` inside
//	    parseHint (go/internal/quotareset/quotareset.go) with a package-level
//	    static var (var hintTimeRE = regexp.MustCompile(...)). Behaviour MUST be
//	    byte-identical; only the compilation timing (per-call → package init)
//	    changes.
//
// Floor binding (R9.3 / cycle-280 lesson). Triage `## top_n` for cycle 334 holds
// TWO entries:
//   - hoist-quotareset-hint-re  (scout, behaviour-preserving, goal-aligned) — gated here.
//   - graduated-enforcement     (operator-HIGH inbox feature)              — NOT gated.
//
// `graduated-enforcement` gets ZERO predicates on purpose, and the reason is a
// disposition decision recorded in test-report.md, not an omission: it is a
// behaviour-CHANGING feature (audit FAIL → correction-retry; tree-diff abort →
// quarantine+warn; SELF_SHA block → warn+repin) dropped into a behaviour-
// PRESERVING refactor cycle whose phase plan runs behavior-baseline /
// behavior-compare — phases that BLOCK exactly the behaviour drift the feature
// introduces. It also arrived with no scout build plan or target files. It is
// dispositioned manual+checklist (carry to a dedicated feature cycle), so
// binding a predicate to it would gate work this cycle cannot ship. The triage-
// DEFERRED `dry-truncate-inline-middle` likewise gets no predicate (deferred-
// floor starvation, cycle-280).
//
// Predicate design (cycle-85 lesson — every gate EXERCISES the system under
// test, no load-bearing source grep):
//
//   - C334_001 is BEHAVIORAL + structural (the "Mixed" category): it drives the
//     real exported entry point quotareset.Compute through the hint-file source
//     path (the ONLY caller of the unexported parseHint), pinning every
//     observable consequence of the regexp match — positive parse, rollover-to-
//     tomorrow, and two negative/rejection cases (malformed text and out-of-
//     range minutes) that fall through to the default source. That behavioural
//     half passes TODAY (the refactor preserves behaviour); the auxiliary
//     structural half (parseHint no longer compiles a regexp in its body) is RED
//     today and is what fails until Builder hoists the pattern. So the predicate
//     is RED now for the RIGHT reason yet locks behaviour so a no-op rename or a
//     behaviour-breaking edit cannot pass.
//
//   - C334_002 is the structural goal-completion gate (waived config/structure
//     check, see the inline waiver): a package-level `var hintTimeRE =
//     regexp.MustCompile(...)` exists AND parseHint's body compiles zero
//     regexps. RED today (the compile lives inline in parseHint; no package
//     var). This proves the pattern MOVED to package init rather than being
//     deleted.
//
// AC map (1:1 with scout-report "Acceptance Criteria Summary" + the eval
// .evolve/evals/hoist-quotareset-hint-re.md AC1..AC5):
//
//	AC1 package-level hintTimeRE var declared                 → C334_002
//	AC2 zero regexp.MustCompile inside parseHint body         → C334_002 (+ auxiliary in C334_001)
//	AC3 all quotareset behaviour preserved / tests pass       → C334_001 (drives Compute end-to-end)
//	AC4 old inline `re := ...MustCompile` removed             → C334_002 (no compile in body)
//	AC5 package still builds                                  → implied (this _test.go compiles against the pkg; C334_001 runs it)
```

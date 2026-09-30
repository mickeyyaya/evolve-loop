# Comment history: `acs/cycle347`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle347/predicates_test.go:3` — above `package cycle347`

```text
// Package cycle347 materializes the cycle-347 acceptance criteria for the three
// committed top_n tasks (triage-report.md ## top_n):
//
//	T1  skillcheck-run-coverage — raise go/internal/skillcheck statement coverage
//	    from the 69.2% baseline to >= 70% by adding four TestRun_* tests that
//	    exercise Run(projectRoot, write, stdout, stderr) across: (a) write=false
//	    no-drift → exit 0 + "check OK" on stdout, (b) write=false drift → exit 2
//	    + "DRIFT:" on stderr, (c) write=true drift → file rewritten in-place
//	    exit 0, (d) invalid projectRoot (catalog missing) → exit 1.
//
//	T2  codequality-edge-coverage — raise go/internal/codequality coverage from
//	    86.4% to >= 90% by adding firstLine(s) no-newline + missing-gofmt-binary
//	    tests (t.Setenv("PATH", "")).
//
//	T3  cycle-audit-cycle-scoped-ci-gap (operator inbox HIGH) — add
//	    TestNewDefault_WiresSkillsDriftCheck to audit_skillsdrift_test.go and
//	    cover the Worktree="" fallback path of skillsDriftCheckDefault, so audit
//	    correctly fails cycles with SKILL.md drift from any call site.
//
// Predicates are BEHAVIORAL (cycle-85 lesson). Coverage gates run the real
// suites under -coverprofile and assert on `go tool cover -func` output; the
// behavioral gate (C347_005) runs a specific test function via subprocess and
// asserts on the PASS line presence. No load-bearing source-grep.
//
// AC map (1:1 with triage top_n items):
//
//	T1.coverage     skillcheck package coverage >= 70%           → C347_001
//	T1.run-nonzero  Run function not at 0.0% coverage           → C347_002
//	T2.coverage     codequality package coverage >= 90%         → C347_003
//	T2.firstline    firstLine function at 100% coverage         → C347_004
//	T3.skills-wire  TestNewDefault_WiresSkillsDriftCheck passes → C347_005
//	T3.audit-cover  audit package coverage >= 91%               → C347_006
//
// Floor binding (R9.3): T1/T2/T3 are the three committed top_n tasks this cycle;
// their coverage floors bind committed packages. soakreport (triage-deferred P3)
// gets ZERO predicates here — a floor on a deferred task starves the committed
// ones (cycle-280 lesson).
```

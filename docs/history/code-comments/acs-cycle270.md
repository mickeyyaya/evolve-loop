# Comment history: `acs/cycle270`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle270/predicates_test.go:3` — above `package cycle270`

```text
// Package cycle270 materializes the cycle-270 acceptance criteria for the
// `looppreflight-coverage-gaps` task: raise the lowest-coverage package in the
// tree (`internal/looppreflight`, 69.3% at the cycle-270 baseline) to >= 82% by
// unit-testing the six zero-coverage host-side `default*` seams and the
// `resolve()` nil-default branches. This is a TEST-ONLY change — no production
// logic moves — so the contract is "the suite gains these named tests, they
// pass, and coverage clears the bar."
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each RUNS the
// system-under-test — the `internal/looppreflight` Go suite — as a subprocess
// and asserts on its real `go test -cover -v` output (top-level + subtest PASS
// lines, the coverage %, absence of FAIL) and on a real `-race` run. There is
// NO source-grep gaming: a magic string in a .go file cannot produce a `--- PASS`
// line for a named test, nor move the coverage number. The builder's job is the
// new test code in `internal/looppreflight`; these predicates gate it.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	C1 → TestC270_001 (coverage >= 82%)
//	C2 → TestC270_002 (suite green under -race)
//	C3 → TestC270_003 (TestDefaultDirWritable: positive + negative)
//	C4 → TestC270_004 (TestDefaultTmuxSessions)
//	C5 → TestC270_005 (TestBootRCName: known + unknown codes)
//	C6 → TestC270_006 (TestResolve_NilDefaults)
//	C7 → manual+checklist (Auditor diff-scope review; see test-report.md)
```

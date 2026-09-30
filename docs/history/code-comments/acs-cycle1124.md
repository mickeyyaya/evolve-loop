# Comment history: `acs/cycle1124`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1124/predicates_test.go:3` — above `package cycle1124`

```text
// Package cycle1124 materialises the cycle-1124 acceptance criteria for the
// single fleet-scoped task `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, kind pipeline-integrity).
//
// Defect. `hooks.Classify` (go/internal/phases/audit/audit.go:131-179) extracts
// the auditor's narrative verdict and then unconditionally overwrites it with
// core.VerdictFAIL at three EGPS gate branches (acs-verdict.json unreadable,
// red_count>0, ship_eligible=false) without recording what the narrative said.
// Every downstream consumer (cyclestate.ErrorMessages → AuditFailReasons →
// <phase>-fail-reason.json → dossier SubstantiveError) therefore sees only the
// gate's own message, so an operator cannot tell a genuine defect from a
// POISONED predicate the auditor itself flagged clean (cycles 1116/1107/1117,
// the connected `audit-probe-tree-isolation` case).
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against the
// worktree and asserts on its exit code. A no-op implementation cannot green
// these: 002 is the anti-noise negative axis (a blanket "always append a
// conflict diagnostic" fix fails it), and 003 pins the untouched-gate
// regression suites.
//
//   - 001 the producer half: the conflict record exists, is error-severity,
//     names the narrative verdict, and fires on all three override branches.
//   - 002 the negative/anti-no-op half: coherent narrative-FAIL, unparseable
//     narrative, and green-gate cases emit NO conflict record.
//   - 003 the consumer half: the record reaches AuditFailReasons / the forensic
//     file / the dossier's SubstantiveError with no new plumbing.
//   - 004 no regression: the pre-existing audit + core verdict suites stay green
//     (the gate's FAIL override must not have been softened).
```

### `go/acs/cycle1124/predicates_test.go:55` — above `func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {`

```text
// runGoTest runs `go test -C <worktree>/go -run ^(pattern)$ <pkg>` and reports
// whether it exited 0. A compile failure in the target package (the expected
// RED signal before Builder implements) surfaces as a non-zero exit, NOT as a
// launch failure: SubprocessOutput returns a non-nil err for any non-zero exit,
// so only code < 0 is a genuine "could not launch" (cycle-574 lesson).
```

### `go/acs/cycle1124/predicates_test.go:80` — above `func TestC1124_001_ConflictRecordEmittedOnEveryOverrideBranch(t *testing.T) {`

```text
// TestC1124_001_ConflictRecordEmittedOnEveryOverrideBranch — AC-1/AC-2. The
// producer half: Classify emits an error-severity `verdict-conflict:` record
// naming the auditor's narrative verdict at each of the three EGPS override
// branches, and the branches/narratives are distinguishable (the failure
// fingerprint must not collapse — batch-12 breaker lesson).
```

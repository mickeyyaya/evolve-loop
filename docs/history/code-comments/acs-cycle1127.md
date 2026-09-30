# Comment history: `acs/cycle1127`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1127/predicates_test.go:3` — above `package cycle1127`

```text
// Package cycle1127 materialises the cycle-1127 acceptance criteria for the
// single fleet-scoped task `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, kind pipeline-integrity,
// 4th recurrence of the cycle-87 / cycle-352 / cycle-456 family).
//
// State at HEAD (33596bb0, the cycle-1124 salvage snapshot). `hooks.Classify`
// already captures the auditor's narrative verdict and records an
// error-severity `verdict-conflict:` diagnostic — but ONLY inside the EGPS
// override block (acs-verdict.json unreadable / red_count>0 /
// ship_eligible=false). Five further gates in the same function still clobber
// `verdict` to core.VerdictFAIL with no record at all:
//
//	gofmt · skills-drift · go vet · acs-durable · integration-tier ·
//	apicover-enforce · apicover new-package graduation
//
// AC-1 names those gates explicitly, so the task is not done: on 7 of 10
// override paths the operator still cannot tell a genuine defect from a
// poisoned/non-hermetic gate the auditor itself read as clean — which is the
// exact forensic cost the inbox item was filed for (cycles 1107/1116/1117,
// the connected `audit-probe-tree-isolation` case).
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against THIS
// worktree and asserts on its exit code. No predicate here asserts that a
// source file contains a string.
//
//   - 001 producer: every non-EGPS gate that overrides a found, non-FAIL
//     narrative leaves exactly one error-severity conflict record naming it.
//   - 002 anti-no-op: the coherent, unparseable, fail-OPEN and all-green cases
//     emit ZERO records. An unconditional append greens 001 and fails here.
//   - 003 additive-only (AC-4): the returned verdict is byte-identical to
//     today across the narrative x gate-state matrix, and the pre-existing
//     audit suites (EGPS override, red-identity fingerprint, gofmt,
//     skills-drift, CI-parity) stay green.
//   - 004 consumer wiring: an error-severity conflict record reaches
//     CycleState.AuditFailReasons → <phase>-fail-reason.json → the dossier's
//     SubstantiveError, and a warning-severity one would be silently dropped.
```

### `go/acs/cycle1127/predicates_test.go:63` — above `func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {`

```text
// runGoTest runs `go test -C <worktree>/go -run ^(pattern)$ <pkg>` and reports
// whether it exited 0. A compile failure in the target package (a legitimate
// RED signal) surfaces as a non-zero exit, NOT as a launch failure:
// SubprocessOutput returns a non-nil err for any non-zero exit, so only
// code < 0 is a genuine "could not launch" (cycle-574 lesson).
```

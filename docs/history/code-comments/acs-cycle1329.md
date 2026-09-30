# Comment history: `acs/cycle1329`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1329/predicates_test.go:3` — above `package cycle1329`

```text
// Package cycle1329 materialises the cycle-1329 acceptance criteria for the
// fleet-scoped todo `audit-warn-prescription-gate`: give the audit-side
// new-package graduation gate (`apicoverNewPackageGraduationDefault`,
// go/internal/phases/audit/ciparity.go:647) the same copy-pasteable
// prescription the build-entry seam already emits
// (`graduationPrescription`, go/internal/core/phase_bindings_graduation.go:81)
// instead of the terse "add it + an apicover_named_test.go" sentence — by
// relocating that helper into the shared `ciparity` package (exported as
// `GraduationPrescription`) both seams already import.
//
// Predicate strategy — each predicate shells `go test -run <pattern> -count=1
// -v <pkg>` over the DEFAULT (non-acs) suite and requires a `--- PASS: <name>`
// line per named test (the cycle-997 SubprocessOutput precedent). Asserting on
// the PASS line, not merely exit 0, is essential: a pattern matching zero
// tests exits 0 with "no tests to run", so a still-missing/still-unwired test
// would otherwise false-GREEN. This mirrors the reachability-probe /
// caller-proof house rule: these predicates require the RELOCATED seams to be
// actually reached, not merely present as dead code.
//
//   - 001 binds go/internal/ciparity/graduation_prescription_test.go (this
//     TDD phase's own new white-box tests) — the relocated, exported
//     GraduationPrescription helper, covering AC3 (the "..." pattern branch
//     preserved) plus positive/negative/semantic diversity.
//   - 002 binds go/internal/phases/audit/ciparity_newpkg_test.go's new
//     TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix —
//     AC1: the audit offender line must carry the literal .apicover-enforce
//     append line and apicover_named_test.go path.
//   - 003 binds the PRE-EXISTING build-entry regression suite
//     (go/internal/core/phase_bindings_graduation_test.go) — AC2: the
//     relocation must not require editing that file, and its abort_reason
//     content must stay byte-identical (already green today; this predicate
//     keeps it green through the refactor without letting it silently break).
//   - 004 is AC4: `go vet` must stay clean on every package this task
//     touches. Scoped to the three touched packages individually (not a
//     whole-repo `/...` sweep) per the flaky-predicate-shape house rule —
//     `go vet` is a static check (no test execution, no timing/PID/process
//     hazard), so per-package invocation is both correct-scoped and cheap.
//     Currently RED: `go vet ./internal/ciparity/...` fails to compile
//     graduation_prescription_test.go (undefined: GraduationPrescription).
```

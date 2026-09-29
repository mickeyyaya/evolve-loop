# Build Explanation — Cycle 1764

## Build Binding
- Cycle: 1764
- Base SHA: 78eef9e8fd9fc182c0963cb2aa9b57394673d234

## Summary
Three size-ratchet offenders in the eval-validation packages now fit the
50-line ratchet limit: `posteditvalidate.Run` (was 108 lines),
`evalqualitycheck.CheckDiversity` (was 59) and `verifyeval.Verify` (was 53).
Each one delegates named steps to unexported helpers, and behavior is unchanged.
`go/internal/sizeratchet/offenders.json` is left untouched, so its three
entries become slack for a later boundary tighten. This cycle continues the
extraction that cycle 1761 carried in through the ADR-0076 continuation
salvage. It re-verifies that extraction against this cycle's base and makes no
further source changes.

## Rationale
Each function mixed steps at different levels of abstraction. `Run` combined
logger wiring, early-exit target resolution, validator defaulting and
per-extension dispatch. `CheckDiversity` did per-file open, scan and
fingerprint work inside its directory loop. `Verify` executed and graded each
script inside its loop. Moving each step into one unexported function is the
smallest change that fits the ratchet without altering control flow, log text,
error wording or return values. Raising the allowances in `offenders.json` was
rejected, because the ratchet exists to shrink them. No exported identifier,
interface or package was added.

## Changed Areas
- `go/internal/posteditvalidate/posteditvalidate.go` — `Run` now delegates to `guardsLogger` (guards.log path and clock resolution), `resolveTarget` (the no-payload, bypass, missing-file_path and missing-file skips, in their original order), `resolveValidators` (validator seam defaulting) and `validateFile` (the per-extension switch, returning kind and ok). `Run` keeps the `warnLLM` closure, so it still sets `WarnEmitted` and still owns every write to `Result`.
- `go/internal/evalqualitycheck/diversity.go` — the per-file open, scan and negative/edge fingerprinting moved from the `CheckDiversity` loop into `fingerprintEval`. It returns the fingerprint, whether the file had commands, and the same wrapped open/scan errors.
- `go/internal/verifyeval/verifyeval.go` — per-script execution and grading moved from the `Verify` loop into `runScript`. `Verify` sets the verdict to FAIL whenever a command result is not `Passed`, which covers exactly the three original failure branches: runner error, expectation mismatch and missing execution evidence.

## Design Decisions
The helpers stay unexported and sit beside the function they came from. Each
existing inline comment moved with its code, and no comment was added.
`validateFile` returns `(kind, ok)` instead of mutating a `*Result`, so the
exported function remains the single writer of its result.

## Verification
The unmodified baseline test suites of all three packages pass. All nine
cycle-1764 ACS predicates pass: ratchet fit, `offenders.json` unchanged,
module-wide ratchet check, frozen baseline tests, package tests, vet/gofmt, no
comment lines added, none lost, and scope fence. The native ACS suite for
cycle 1764 reports red=0.

## Compatibility
No exported signature, result field, log line or error message changed.
Callers of `Run`, `CheckDiversity` and `Verify` are unaffected.

## Limitations
`offenders.json` still lists the three functions at their old allowances until
a boundary tighten removes the slack. `verifyeval.shellWords` is out of scope
and remains a listed offender.

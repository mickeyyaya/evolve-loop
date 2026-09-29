# Build Explanation — Cycle 1761

## Build Binding
- Cycle: 1761
- Base SHA: e8452207e36b48f2441d711df4529ac66d2d583b

## Summary
Shrank the three size-ratchet offenders in the eval-validation packages —
`posteditvalidate.Run` (108 lines), `evalqualitycheck.CheckDiversity`
(59 lines) and `verifyeval.Verify` (53 lines) — to at or below the 50-line
ratchet limit by extracting named steps. Behavior is unchanged and
`go/internal/sizeratchet/offenders.json` is left untouched; its three entries
are now slack for a later boundary tighten.

## Rationale
Each function mixed several steps at different levels of abstraction: logger
wiring, early-exit target resolution, validator defaulting and per-extension
dispatch in `Run`; per-file open/scan/fingerprint work inside the directory
loop of `CheckDiversity`; and per-script execution and grading inside the loop
of `Verify`. Extracting each step into one unexported function is the smallest
change that fits the ratchet without touching control flow, log text, error
wording or return values. No exported identifier, interface or new package was
added, so there is no new API surface.

## Changed Areas
- `go/internal/posteditvalidate/posteditvalidate.go` — `Run` now delegates to `guardsLogger` (guards.log path and clock resolution), `resolveTarget` (no-payload, bypass, missing file_path and missing-file skips, in the original order), `resolveValidators` (validator seam defaulting) and `validateFile` (the per-extension switch returning kind and ok); `Run` keeps the `warnLLM` closure so `WarnEmitted` is still set on the result.
- `go/internal/evalqualitycheck/diversity.go` — the per-file open, scan and negative/edge fingerprinting moved from the `CheckDiversity` loop into `fingerprintEval`, which returns the fingerprint, whether the file had commands, and the same wrapped open/scan errors.
- `go/internal/verifyeval/verifyeval.go` — per-script execution and grading moved from the `Verify` loop into `runScript`; `Verify` marks the verdict FAIL whenever a script result is not `Passed`, which matches the three original failure branches (runner error, expectation mismatch, missing execution evidence).
- `.evolve/evals/sizeratchet-shrink-eval-validators.md` — the lane's eval, authored by the earlier TDD phase in this worktree; it ships with the cycle and the Build did not edit it.

## Design Decisions
Helpers stay unexported and live beside the function they came from. Each
existing inline comment moved with its code (`// Resolve guards.log path.`,
`// Resolve validator defaults.`, `// Clean up __pycache__ ...`), and no new
comment was added. `validateFile` returns `(kind, ok)` rather than mutating a
`*Result`, so `Run` still owns every write to its result.

## Verification
The unchanged baseline suites of all three packages pass
(`go test -count=1 ./internal/posteditvalidate/ ./internal/evalqualitycheck/ ./internal/verifyeval/`).
The cycle's ACS predicates 001–008 pass: ratchet fit, `offenders.json`
unchanged, module-wide ratchet check, frozen baseline tests, vet/gofmt, no
comment lines added and none lost.

## Compatibility
No exported signature, result field, log line or error message changed.
Callers of `Run`, `CheckDiversity` and `Verify` are unaffected.

## Limitations
`offenders.json` still lists the three functions at their old allowances until
a boundary tighten removes the slack. `verifyeval.shellWords` (51 lines) is out
of scope and remains a listed offender.

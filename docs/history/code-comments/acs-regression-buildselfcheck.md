# Comment history: `acs/regression/buildselfcheck`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/buildselfcheck/selfcheck_test.go:3` — above `package buildselfcheck`

```text
// Package buildselfcheck is the ACS toolchain-green hard gate. It fails the
// per-cycle audit when the deterministic post-build self-check
// (core.buildSelfCheck) recorded any changed-package unit-test failure in
// .evolve/build-selfcheck.json.
//
// Why this guard exists: buildSelfCheck already runs `go test` (which runs
// `go vet`) on every changed Go package after the build phase and writes the
// failing packages to .evolve/build-selfcheck.json — but it is best-effort and
// NEVER aborts ("audit is the backstop"). Nothing enforced the artifact, so the
// LLM auditor was the only thing standing between a broken build and a ship —
// and it missed relaunch cycle 12, which shipped a PASS with a `go vet` failure
// (`string(Stage)` → a 1-rune garbage string) and a failing unit test. This
// guard makes the existing detection a DETERMINISTIC red_count failure: a cycle
// whose changed packages don't build/vet/test green cannot ship, regardless of
// what the LLM auditor concludes.
//
// Naturally inert: when no cycle ran (main/CI), the artifact is absent → PASS.
// buildSelfCheck clears the artifact at the start of every build, so a passing
// retry never inherits a stale failure (the gate would otherwise loop).
```

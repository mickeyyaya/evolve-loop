# Comment history: `acs/regression/cycle99`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle99/predicates_test.go:3` — above `package cycle99`

```text
// Package cycle99 ports the cycle-99 ACS predicates (3 bash files).
//
// Bash predicates 001+003 are presence/structure checks of doc files
// (PSMAS A/B verification + incident analysis). Predicate 002 invokes
// scripts/guards/gitignore-reachability-check.sh against synthetic
// fixtures; Go port reduces to source-presence + behavioral smoke.
```

### `go/acs/regression/cycle99/predicates_test.go:20` — above `func TestC99_001_PsmasABVerificationDocumented(t *testing.T) {`

```text
// TestC99_001_PsmasABVerificationDocumented ports cycle-99/001.
// The PSMAS doc must reference ≥5 cycles, a percentage, the 20% threshold,
// a FLIP/DEFER/REJECT verdict, and the EVOLVE_PSMAS_SKIP flag.
```

### `go/acs/regression/cycle99/predicates_test.go:53` — above `func TestC99_002_GitignoreReachabilityGuardFunctional(t *testing.T) {`

```text
// TestC99_002_GitignoreReachabilityGuardFunctional ports cycle-99/002.
// Source-presence: the guard script exists, executable, git-tracked.
// Behavioral smoke via SubprocessOutput: invoking guard with CLAUDE.md
// returns rc=0; with a known-ignored path returns non-zero.
```

### `go/acs/regression/cycle99/predicates_test.go:74` — above `func TestC99_003_TurnOverrunIncidentAnalysisComplete(t *testing.T) {`

```text
// TestC99_003_TurnOverrunIncidentAnalysisComplete ports cycle-99/003.
// Verifies the cycle-95 turn-overrun incident report exists at one of
// the accepted persistent paths with the 6-part structure.
```

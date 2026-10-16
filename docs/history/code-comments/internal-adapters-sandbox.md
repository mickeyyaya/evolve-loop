# Comment history: `internal/adapters/sandbox`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/adapters/sandbox/confinement_satisfied_test.go:8` — above `func TestConfinementSatisfied(t *testing.T) {`

```text
// TestConfinementSatisfied names the three-cell Specification both the bridge
// gate (sandboxRequiredButUnavailable) and preflight's host-capabilities check
// project from — extracted 2026-09-01 after the two sites diverged in the
// nested cell (preflight HALTed a launch the gate would have run) and the
// opt-out cell (the reverse).
```

### `go/internal/adapters/sandbox/sandbox_test.go:70` — above `func TestGenerateSBPL_TmpReadWriteBoth(t *testing.T) {`

```text
// TestGenerateSBPL_TmpReadWriteBoth — both /tmp and /var/folders need
// read+write. claude.sh:477 explains this is load-bearing for bash
// tool output files. Missing the read rule was the root cause of
// cycles 8121-8128 build failures.
```

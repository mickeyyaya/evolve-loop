# Comment history: `internal/cyclesimulator`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/cyclesimulator/cyclesimulator.go:1` — above `package cyclesimulator`

```text
// Package cyclesimulator ports legacy/scripts/dispatch/cycle-simulator.sh.
//
// No-LLM cycle plumbing simulator (v8.50.0+). Walks every phase of an
// /evo:loop cycle writing deterministic artifacts and appending
// tamper-evident ledger entries WITHOUT making any LLM API calls.
//
// What this validates:
//   - cycle-state.json advances correctly through every phase
//   - phase-gate.sh accepts every transition
//   - prev_hash chain remains intact post-run
//   - ship.sh --dry-run executes inside a real cycle context
//
// What this does NOT validate:
//   - LLM output quality (no LLM is invoked)
//   - Real Builder file edits (no source code changes)
//   - Real Auditor judgment (verdict is hardcoded PASS)
```

### `go/internal/cyclesimulator/cyclesimulator.go:368` — above `if runID := core.RunIDFromWorkspace(filepath.Dir(artifactPath)); runID != "" {`

```text
// Cycle-1571 H1: a run-scoped binding lookup refuses an entry with no run
// identity. The simulator writes into the SAME ledger as the real loop, so
// its entries carry it too — resolved from the artifact's own run workspace.
// Set only when resolvable, mirroring core's `json:"run_id,omitempty"`.
```

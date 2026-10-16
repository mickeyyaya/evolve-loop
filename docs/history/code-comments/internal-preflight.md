# Comment history: `internal/preflight`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d5

### `go/internal/preflight/preflight.go:1` — above `package preflight`

```text
// Package preflight ports legacy/scripts/dispatch/preflight-environment.sh.
//
// Single capability-detection probe (v8.25.0). Probes the host environment
// ONCE at dispatcher start and emits a JSON capability profile. The
// dispatcher reads the profile and auto-configures the auto-relaxable flags.
//
// Design principle: Discover, Decide, Log, Verify.
// This package implements Discover + Decide + Log. Verify happens elsewhere
// (phase-gate, ledger-SHA, role-gate, ship-gate).
```

### `go/internal/preflight/preflight.go:113` — above `WorktreeBase string`

```text
// WorktreeBase is the operator override for the per-cycle worktree base,
// resolved from policy.json (worktree.base) at the composition root. Empty ⇒
// preflight selects a default writable base. Replaces the former
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
```

### `go/internal/preflight/preflight_test.go:94` — above `const standaloneDefault = "standalone shell with working sandbox: defense-in-depth enabled"`

```text
// Guard for the audit MEDIUM (cycle-990613): whenever InnerSandbox is false,
// InnerSandboxReason must NOT be the "standalone … defense-in-depth" default —
// it must explain WHY the inner sandbox was disabled. Pins the reason switch's
// exhaustiveness (incl. the catch-all) so the host profile can't misreport
// across every reachable disabled path.
```

### `go/internal/preflight/worktreebase_config_test.go:8` — above `func TestSelectWorktreeBase_WritableOverrideWins(t *testing.T) {`

```text
// TestSelectWorktreeBase_WritableOverrideWins locks the flag-reduction change
// (ADR-0064): the operator worktree-base override now flows from Options.WorktreeBase
// (resolved from policy.json worktree.base at the composition root) — NOT the
// EVOLVE_WORKTREE_BASE env var, which is removed. A writable override wins and is
// reported as such.
```

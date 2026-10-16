# Comment history: `internal/failureadapter`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D4

### `go/internal/failureadapter/failureadapter.go:1` — above `package failureadapter`

```text
// Package failureadapter is the Go port of scripts/failure/failure-adapter.sh
// — the deterministic failure-adaptation kernel introduced in v8.22.0.
//
// It is a PURE FUNCTION over a slice of failure entries. No I/O; no
// process spawning; safe to call from any phase. The orchestrator (or
// CLI subcommand wrapper) reads state.json:failedApproaches, hands the
// entries to Decide(), and follows the resulting Decision verbatim.
//
// The bash source remains canonical until the v11.0 cutover; both
// implementations must agree on every input. See failureadapter_test.go
// for the bash-source line references on each rule.
```

### `go/internal/failureadapter/failureadapter.go:20` — above `type Classification string`

```text
// Classification mirrors the v8.22.0+ taxonomy at
// scripts/failure/failure-classifications.sh:66-86.
```

### `go/internal/failureadapter/failureadapter.go:59` — above `type Entry struct {`

```text
// Entry mirrors one element of state.json:failedApproaches. JSON tags
// match the on-disk schema observed at .evolve/state.json (canonical
// since v8.22.0).
```

### `go/internal/failureadapter/failureadapter.go:131` — above `nonExpired := make([]Entry, 0, len(entries))`

```text
// Filter to non-expired entries using the v8.23.1 fallback rules
// from failure-adapter.sh:120-130.
```

### `go/internal/failureadapter/failureadapter.go:253` — above `func isNonExpired(e Entry, now time.Time) bool {`

```text
// isNonExpired implements the bash v8.23.1 expiration logic
// (failure-adapter.sh:120-130). Three cases:
//
//  1. explicit non-null expiresAt in the future  → keep
//  2. both expiresAt and recordedAt missing      → keep (truly legacy)
//  3. expiresAt missing but recordedAt present   → effective 1d TTL
```

### `go/internal/failureadapter/failureadapter_test.go:1` — above `package failureadapter`

```text
// Package failureadapter ports scripts/failure/failure-adapter.sh —
// the deterministic failure-adaptation kernel introduced in v8.22.0.
//
// The bash source is at scripts/failure/failure-adapter.sh and its
// taxonomy at scripts/failure/failure-classifications.sh. This Go port
// must produce the SAME Decision for the SAME inputs so the trust
// kernel stays host-CLI-agnostic.
```

### `go/internal/failureadapter/failureadapter_test.go:55` — above `func TestDecide_LegacyMissingExpiresAt_OneDayTTL(t *testing.T) {`

```text
// TestDecide_LegacyMissingExpiresAt_OneDayTTL — bash v8.23.1 fix:
// when expiresAt is missing but recordedAt is present, effective TTL
// is recordedAt + 1d. Replicates failure-adapter.sh:124-130.
```

### `go/internal/failureadapter/failureadapter_test.go:239` — above `func TestDecide_OneInfraTransient_RetryWithFallback_StrictMode(t *testing.T) {`

```text
// TestDecide_OneInfraTransient_RetryWithFallback_StrictMode — rule 6:
// any non-expired infrastructure-transient → RETRY-WITH-FALLBACK in strict mode.
// EVOLVE_SANDBOX_FALLBACK_ON_EPERM was retired in cycle-10; sandbox fallback is
// now auto-managed by the binary from nest detection, not injected via set_env.
```

### `go/internal/failureadapter/failureadapter_test.go:255` — above `func TestDecide_OneInfraTransient_FluentMergesSetEnv(t *testing.T) {`

```text
// TestDecide_OneInfraTransient_FluentMergesSetEnv — fluent mode keeps PROCEED.
// EVOLVE_SANDBOX_FALLBACK_ON_EPERM was retired in cycle-10; no env injection needed.
```

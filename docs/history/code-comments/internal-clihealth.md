# Comment history: `internal/clihealth`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/clihealth/resetparse.go:42` — above `func evidenceLine(pane string) string {`

```text
// evidenceLine returns the most representative line for a bench record: the
// wall BANNER line that carries the reset hint (the line ParseResetHint keys
// on), which is what actually walled the CLI — not the pane's first line. On a
// scrolled pane firstLine catches a later frame or, as in cycle-314, the
// agent's own edit content ("53 +\tFamily: codex,"), obscuring the real cause
// in the bench evidence. Falls back to firstLine when no reset-hint line is
// present (no regression for walls without a parseable hint).
```

## comment-zero h1

### `go/internal/clihealth/clihealth.go:160` — above `func truncateRunes(s string, n int) string {`

```text
// truncateRunes truncates on RUNE boundaries — wall text carries multi-byte
// glyphs (the cycle-283 codex wall opens with '■') and a byte slice could
// split one, corrupting the stored evidence.
```

### `go/internal/clihealth/clihealth_amplify_cycle426_test.go:3` — above `import (`

```text
// clihealth_amplify_cycle426_test.go — adversarial amplification for cycle-426
// ClearBootStrike lifecycle (T2).  Tests target lifecycle corners the Build
// tests do not probe:
//
//  1. Double-clear idempotency — clearing an already-cleared entry.
//  2. Full re-accumulation after clear — bench→clear→re-bench lifecycle.
//  3. Multi-driver isolation — ClearBootStrike(A) must not affect driver B.
```

### `go/internal/clihealth/clihealth_amplify_cycle426_test.go:64` — above `for i := 0; i < DefaultBootBenchThreshold; i++ {`

```text
// Round 1: accumulate to threshold.
```

### `go/internal/clihealth/clihealth_amplify_cycle426_test.go:81` — above `for i := 0; i < DefaultBootBenchThreshold-1; i++ {`

```text
// Round 2: strikes below threshold must NOT bench.
```

### `go/internal/clihealth/clihealth_boottimeout_reset_test.go:3` — above `import (`

```text
// clihealth_boottimeout_reset_test.go — RED contract for the reset-boot-strike-
// on-success feature (cycle-426 T2): ClearBootStrike removes the
// BootTimeoutPattern bench entry for a given driver so consecutive-strike
// counting restarts after a successful boot (any exit code != ExitREPLBootTimeout).
//
// All tests in this file are RED: ClearBootStrike does not yet exist on *Store.
// They become GREEN when the Builder adds:
//   func (s *Store) ClearBootStrike(driver string) error — removes the entry at
//   key=driver only when the stored Reason == BootTimeoutPattern; no-op on absent
//   driver or on entries with a different Reason.
```

### `go/internal/clihealth/clihealth_test.go:3` — above `import (`

```text
// RED contract for the CLI health bench store (cycle-283 forensics): codex hit
// its quota wall, the bridge classified rate_limit on every dispatch, and
// NOTHING remembered it — every codex-routed phase re-burned a 5-15min boot
// before falling back, all night. The store is the memory: a tiny standalone
// .evolve/cli-health.json keyed by CLI FAMILY with time-based expiry.
```

### `go/internal/clihealth/clihealth_test.go:242` — above `func TestParseResetHintBasics(t *testing.T) {`

```text
// TestParseResetHintBasics — slice-1 baseline (slice 2 hardens). The verbatim
// cycle-283 wall text must parse to the printed clock time (+margin).
```

### `go/internal/clihealth/credential_wall_test.go:9` — above `func TestBenchable_ACredentialWallBenchesTheFamily(t *testing.T) {`

```text
// A credential wall is worth benching for the same reason a quota wall is: re-dispatching into it is
// guaranteed waste. Wave 14 re-dispatched claude fourteen times into "Please log in".
```

### `go/internal/clihealth/exhausted_bench_test.go:8` — above `func TestBenchable_AdmitsTheExhaustedWall(t *testing.T) {`

```text
// exhausted_bench_test.go — agy's quota wall never reached the bench ledger.
//
// Live, 4/4 lanes across two waves: agy's escalation reports carry
// pattern_name "exhausted" ("Individual quota reached. Please upgrade your
// subscription… Resets in 6h12m56s"), while codex's carry "rate_limit". Same
// wall, two vocabularies — and only one was admitted here, so cli-health never
// learned agy was walled and the bench-aware chain reorder (which demonstrably
// works: cycle-1525's build phase logged "cli-health bench reordered chain")
// had no row to act on. Every agy-primary phase paid a dead-CLI round-trip.
```

### `go/internal/clihealth/resetparse_fuzz_test.go:3` — above `import (`

```text
// Fuzz harness for ParseResetHint (inbox fuzz-parser-surfaces, slice 1). Seeded
// from the TestParseResetHintTable goldens (cycle-283/cycle-314 wall shapes) so
// the corpus starts from real provider wording, then explores mutations for
// crashers the curated table can't anticipate. `now` is held fixed (fuzz
// corpora can't carry a time.Time) so every invariant check is deterministic.
//
// Invariants asserted on every input (never just "no panic"):
//   - ok=false must return the zero time.Time (no half-parsed leakage).
//   - ok=true must return a time strictly after now (no negative "duration
//     until reset") and no more than 24h after now (the capHint ceiling).
```

### `go/internal/clihealth/resetparse_test.go:3` — above `import (`

```text
// Slice-2 hardening table for ParseResetHint. The clock-time grammar is the
// cycle-283 codex wall shape; the relative grammar covers the other wording
// providers use. Everything time-sensitive injects now — no wall-clock reads.
```

### `go/internal/clihealth/resetparse_test.go:74` — above `func TestEvidenceLine_PrefersBannerOverFirstLine(t *testing.T) {`

```text
// TestEvidenceLine_PrefersBannerOverFirstLine pins the bench-evidence accuracy
// fix (bridge-ratelimit, cycle-314 forensics): the bench Evidence must be the
// actual wall BANNER line (the one carrying the reset hint), not the pane's
// first line — which on a scrolled pane caught a later frame ("53 +\tFamily:
// codex,", an agent edit-diff line) and obscured what really walled the CLI.
```

### `go/internal/clihealth/clihealth.go:1` — above `package clihealth`

```text
// Package clihealth is the durable memory for transient CLI-family outages
// (cycle-283 forensics): the bridge classifies a quota wall on every dispatch
// (pattern_name=rate_limit → exit 85) but nothing remembered it, so every
// phase re-burned the walled CLI's 5-15min boot before falling back. This
// package stores expiring per-FAMILY bench records in .evolve/cli-health.json;
// llmroute demotes benched families, the loop canaries expired ones, and
// preflight reports active ones. Stdlib-only leaf — importable by llmroute,
// runner, cmd, and looppreflight without cycles.
//
// The "walled until T" fact is idempotent, but the per-family Strikes counter is
// ACCUMULATIVE — concurrent fleet cycles walling the same family must each see the
// prior strike. An unlocked read-modify-write loses increments and under-escalates
// the cooldown, so the full RMW (BenchWall/Bench/Clear) runs under the "<file>.lock"
// sidecar flock (the project-wide convention; EVOLVE_FLEET=1 skips the global cycle
// lock that previously serialized these writers). Writes stay temp+rename atomic; a
// corrupt or missing file degrades to empty, never fatal.
```

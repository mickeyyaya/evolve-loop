# Comment history: `acs/cycle1677`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1677/predicates_test.go:3` — above `package cycle1677`

```text
// Package cycle1677 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	ledger-verify-seal-anchor  (priority M, weight 0.70, code)
//
// The lane's two other scoped ids, kb-graph-projector and
// inbox-console-worklist-view, were triage-DEFERRED and get ZERO predicates
// here (a predicate gating deferred work starves the committed task —
// cycle-280).
//
// The gap. The anchor RESOLVER is already correct: effectiveAnchorSHA
// (anchor.go) picks the last self-chaining operator `reset-seal-*` at or after
// the sidecar ledger-anchor.json, and walkChain (ledger.go) resumes STRICT
// validation from that exact line SHA. What is missing is OBSERVABILITY at the
// only surface an operator sees: runLedgerVerify (cmd_ledger.go:52) prints
// `[ledger] OK: chain intact (<dir>/ledger.jsonl)` whether it verified every
// byte from genesis or deliberately trusted a 136k-line adjudicated prefix.
// Those two outcomes are NOT the same claim, and today they are the same
// string. This lane makes a successful verification state the scope it
// actually verified.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1  break → eligible seal → valid tail verifies successfully AND
//	     states the sealed prefix informationally                      → 001, 002
//	AC2  a break AFTER the last eligible seal still returns a
//	     chain-broken error                                            → 003
//	AC3  the real repository ledger verifies and reports its scope
//	     without modifying ledger history                              → 004
//	AC-H2 (house rule 2) every path the seam claims is wired: --deep
//	     reports the same provenance as the default path               → 005
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 003 is the
// anti-no-op killer — an implementation that unconditionally prints a
// sealed-prefix line and exits 0 greens 001/005 and fails 003. 002 is the
// anti-hardcode killer — two fixtures whose ONLY difference is the seal's
// identity must produce two different, each-correct outputs, and a
// no-anchor chain must claim neither. 004's no-mutation half is a negative
// over the real 141k-line ledger: verify is a reader.
// EDGE/OOD: a chain whose damage precedes the seal (001), a chain with no
// anchor at all (002's strict arm), a tail forged one line past the anchor
// (003). SEMANTIC: provenance content (001), provenance derivation (002),
// refusal (003), live-corpus behaviour + read-only-ness (004), path parity
// (005) — five distinct behaviours, not one restated.
//
// Flaky-shape contract: no `go test` sweep (the CLI is built ONCE in TestMain
// and every assertion runs that binary), no wall-clock bounds, no literal
// PIDs, no bare `git` (every call is -C anchored), no un-reaped load. The one
// contended read — the LIVE ledger in 004 — is taken as a stat-stable
// snapshot into t.TempDir() and retried, so a concurrent fleet append can
// never make it a false RED.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf, pinning no import edge.
// Nothing here names an internal symbol, so the Builder is free to choose the
// seam's shape (a returned report value, an out-param, a second method); the
// frozen contract is the CLI's observable output, which is the surface the
// acceptance criteria are written against.
```

### `go/acs/cycle1677/predicates_test.go:93` — above `var (`

```text
// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659/1666 TestMain shape).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1677/predicates_test.go:278` — above `func TestC1677_001_SealedPrefixIsStatedOnSuccessfulVerify(t *testing.T) {`

```text
// TestC1677_001_SealedPrefixIsStatedOnSuccessfulVerify drives the production
// CLI over a ledger whose damage is covered by an eligible operator seal. The
// exit code is already correct today (the resolver landed in cycle-1191); what
// must change is that success no longer hides WHICH scope it verified.
```

### `go/acs/cycle1677/predicates_test.go:566` — above `func TestC1677_005_DeepPathReportsTheSameProvenance(t *testing.T) {`

```text
// TestC1677_005_DeepPathReportsTheSameProvenance pins the OTHER production path
// through the same command. `--deep` runs VerifyDeep, which resolves the epoch
// anchor with the same helper; if only the default path gained provenance, an
// operator's two verification commands would disagree about what was verified —
// the wired-into-one-path-only defect (#373).
```

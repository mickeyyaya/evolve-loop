# Comment history: `acs/cycle1333`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1333/predicates_test.go:3` — above `package cycle1333`

```text
// Package cycle1333 materializes the cycle-1333 acceptance criteria for this
// fleet lane's sole assigned todo-id, blocker-breaker-fingerprint-ack (per
// R9.3, no predicates bind to any other lane's items).
//
// Scout's finding: the fingerprint-ack mechanism itself (AckedFingerprints,
// ResolvedFingerprint, LoadResolvedFingerprints, AppendResolvedFingerprint,
// the `evolve loop --reset --fingerprint <fp>` CLI flag) was already
// implemented and merged in cycle-1332 — go build and the fingerprint test
// subset are green. The remaining defect this cycle closes is a
// documentation gap: docs/operations/runtime-reference.md's "Operator
// commands" section documents every other operator-facing mutating command
// but has zero mention of `--fingerprint`, and CHANGELOG.md has no entry
// naming it (cycle-1332's commit was a bare `evolve-cycle:` commit, not a
// `/commit`-attested manual ship, so it never flowed through the changelog
// generator). Per doc_stewardship_policy ("everything learned → docs/ or
// kb/research/"), an undocumented operator-facing CLI flag is itself the
// open defect.
//
// AC map (1:1, from scout-report.md "Acceptance Criteria Summary"):
//
//	AC1 docs/operations/runtime-reference.md names the flag, its --reset
//	    gating, the ledger file .evolve/resolved-fingerprints.json, its
//	    record shape, and which blocker-breaker rule it excludes from
//	    (Rule B, identical-fingerprint).
//	    → C1333_001 (flag + --reset gating + ledger filename)
//	    → C1333_002 (record shape: fingerprint/resolved_at/resolved_by)
//	    → C1333_003 (Rule B / identical-fingerprint exclusion)
//	AC2 CHANGELOG.md has a new entry mentioning `--reset --fingerprint`.
//	    → C1333_004
//	AC3 Zero Go source changes; existing fingerprint/blocker-breaker tests
//	    still pass unchanged (regression — proves the doc-only fix didn't
//	    touch the shipped mechanism).
//	    → C1333_005 (core package, subprocess go test)
//	    → C1333_006 (cmd/evolve package, subprocess go test,
//	      TestRunLoop_FingerprintAck_AppendsLedgerRecord — the real
//	      production-entrypoint caller proof cycle-1332 authored)
//
// Predicate-quality note: AC1/AC2 assert against PROSE (a documentation
// accuracy criterion — the task IS the doc edit, there is no runtime
// behaviour to invoke for "does this doc name this flag"), so they carry
// the `config-check` waiver per go/acs/README.md. This is not the cycle-85
// degenerate shape: the strings asserted (exact flag spelling, exact ledger
// filename, exact JSON field names, "Rule B") are drawn verbatim from the
// real source (go/internal/core/blocker_breaker.go,
// go/cmd/evolve/cmd_loop_args.go) rather than invented, so a Builder cannot
// green these by writing an unrelated sentence containing generic words —
// each substring pins a specific, checkable fact. AC3 (C1333_005/006) is
// fully behavioral: it shells the real `go test` binary against the exact
// package + test names cycle-1332 shipped and requires their PASS marker.
```

### `go/acs/cycle1333/predicates_test.go:120` — above `func TestC1333_004_ChangelogMentionsResetFingerprintFlag(t *testing.T) {`

```text
// TestC1333_004_ChangelogMentionsResetFingerprintFlag verifies CHANGELOG.md
// gained a dated entry naming the `--reset --fingerprint` flag pair, so
// cycle-1332's undocumented ship leaves a trace in the canonical release
// history (scout's AC2, verifiableBy: `grep -q -- "--reset --fingerprint"
// CHANGELOG.md`).
//
// acs-predicate: config-check — documentation accuracy criterion (AC2); see
// package-level waiver rationale above.
```

### `go/acs/cycle1333/predicates_test.go:136` — above `func TestC1333_005_CoreBlockerBreakerFingerprintTestsStillPass(t *testing.T) {`

```text
// TestC1333_005_CoreBlockerBreakerFingerprintTestsStillPass is the AC3
// regression predicate for the core package: the doc-only fix must not
// touch (and must not break) the fingerprint-ack mechanism cycle-1332
// shipped. Runs the real `go test` binary — a subprocess, not a direct
// function call — against the exact test names naming the mechanism.
//
// Behavioral (not config-check): invokes the system-under-test's own test
// binary and requires its PASS marker.
```

### `go/acs/cycle1333/predicates_test.go:163` — above `func TestC1333_006_RunLoopFingerprintAckCallerProofStillPasses(t *testing.T) {`

```text
// TestC1333_006_RunLoopFingerprintAckCallerProofStillPasses is the AC3
// regression predicate for the cmd/evolve package: the real production
// caller proof cycle-1332 authored (TestRunLoop_FingerprintAck_AppendsLedgerRecord,
// which drives runLoop — the actual CLI entrypoint, not a direct
// AppendResolvedFingerprint call) must remain green after this cycle's
// docs-only diff.
//
// Behavioral (not config-check): invokes the system-under-test's own test
// binary and requires its PASS marker.
```

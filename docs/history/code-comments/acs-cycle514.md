# Comment history: `acs/cycle514`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle514/predicates_test.go:3` — above `package cycle514`

```text
// Package cycle514 materialises the cycle-514 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	boot-recovery-auto-repin-shipsha (bug, CRITICAL — 9-cycle SELF_SHA_TAMPERED
//	ship cascade, carryover cycles 498/500/502/508-513). Cycle 507 wired
//	*detection* of a ship-binary SHA mismatch into runLoop's boot path but only
//	WARNs — it never invokes the existing provenance-gated repin primitive
//	phaseintegrity.RepinShipSHA, so the cascade kept recurring. This task closes
//	the wiring gap: a provenance-VERIFIED mismatch (legit rebuild) auto-repins at
//	boot; an UNVERIFIED mismatch (possible tampering) is still refused.
//
// (task triage-worktree-leak-root-cause is DEFERRED — no predicates authored for
// it, per R9.3: predicates bind ONLY to triage-committed work.)
//
// Predicate strategy (mirrors cycle499/cycle503/cycle504/cycle507): BEHAVIORAL
// predicates drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan closes
// the cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer:
//
//	cmd/evolve/cmd_loop_boot_recovery_repin_test.go  (auto-repin wiring: positive/negative/edge)
//
// The Builder implements production code ONLY (the seam named in that file —
// bootRecoveryResult.Healed + shipRepinProvenanceFn + the RepinShipSHA call in
// defaultBootRecovery); it must not modify the tests.
```

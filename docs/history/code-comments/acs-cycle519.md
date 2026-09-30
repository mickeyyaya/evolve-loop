# Comment history: `acs/cycle519`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle519/predicates_test.go:3` — above `package cycle519`

```text
// Package cycle519 materialises the cycle-519 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	loop-cannot-selfheal-dirty-main-tree — implement ONLY the boot pre-flight
//	slice: detect uncommitted tracked-source changes in the main tree at loop
//	boot (git status --porcelain, EXCLUDING .evolve/ and knowledge-base/) and
//	auto-quarantine via a TIMESTAMPED `git stash` (or fail-fast with a precise
//	remediation message if quarantine is unsafe).
//
// (The three deferred sub-scopes of this L-sized item — tree-diff-guard
// attribution, stale-marker self-heal, pre-wave worktree isolation — are in
// triage ## deferred, NOT top_n, so NO predicates are authored for them, per
// R9.3: predicates bind ONLY to triage-committed work. The scout-report's
// wave-seed / disjoint-topn fleet tasks are handled by a sibling lane and are
// likewise out of scope here.)
//
// ── DELTA vs. PRE-EXISTING GREEN (transparently reported) ──────────────────
// Detection, the .evolve/ + knowledge-base/ exclusion, and the non-destructive
// stash all shipped in cycles 507/514 (boot_preflight.go / cmd_loop_boot_recovery.go).
// The ONE behaviour the committed slice ADDS is the TIMESTAMP on the quarantine
// stash label: cmd_loop_boot_recovery.go currently stashes under the FIXED
// constant "boot-quarantine", collapsing every boot quarantine across every batch
// under one ambiguous, unrecoverable name. TestC519_001 is therefore RED at TDD
// time; the three regression predicates (002-004) PIN the already-shipped
// detect/exclude/non-destructive contract so the timestamp change cannot silently
// regress them (they are GREEN now — non-degenerate: each drives a real in-package
// test that CALLS the system and asserts on a stash/side-effect, never a grep).
//
// Predicate strategy (mirrors cycle507/514/518): BEHAVIOURAL predicates drive the
// system under test through its in-package tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85 "no tests
// to run" trap) — never a source grep. Driven tests:
//
//	cmd/evolve/cmd_loop_boot_recovery_timestamp_test.go  (RED: timestamped label)
//	cmd/evolve/cmd_loop_boot_recovery_test.go            (GREEN: detect+quarantine wiring)
//	internal/core/boot_preflight_test.go                 (GREEN: exclusion + non-destructive)
```

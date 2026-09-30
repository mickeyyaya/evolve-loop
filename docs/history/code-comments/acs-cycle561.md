# Comment history: `acs/cycle561`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle561/predicates_test.go:3` — above `package cycle561`

```text
// Package cycle561 materialises the cycle-561 acceptance criteria for the THREE
// `## top_n` tasks triage-report.md committed THIS cycle (NOT scout-report's
// original three — scout picked auditor-egps/memo/sandbox; triage re-ranked the
// live inbox and committed workspace-hygiene-s1, auditor-egps, workspace-hygiene-s3,
// deferring memo + sandbox). Per the AC-Materialization Contract (R9.3 —
// "predicates bind ONLY to triage-committed work") this package predicates ONLY
// those three; the deferred memo/sandbox items get NO predicate here.
//
// READ-FIRST FINDING (AGENTS.md rule 8, surfaced loudly in test-report.md):
// two of the three committed tasks are ALREADY IMPLEMENTED in the worktree base
// (they landed in prior same-goal cycles, commits 5ee210dc / 9e72ac4e):
//
//   - workspace-hygiene-s1: runlease.OwnerLive(l, now, ttl, alive) exists and is
//     consumed at reset.go:149 (SealCycle fence), cmd_loop.go:320 (unfinished
//     guard) and cmd_cycle.go:123 — with a full passing unit suite.
//   - workspace-hygiene-s3: core.deleteCycleBranch (worktree.go:163) and the swarm
//     provisioner mirror already run `git branch -d <leaf>` post-remove, WARN-only,
//     never `-D`, gated on the `cycle-` prefix — with a full passing unit suite.
//
// Their predicates below are therefore behavioural REGRESSION LOCKS (pre-existing
// GREEN, documented as such in test-report.md), driving the REAL unit tests the
// prior cycles authored — not a source grep (the cycle-85 degenerate-predicate
// ban). The only GENUINE RED this cycle is auditor-egps: audit's normal-completion
// path reads red_count but never the authoritative acs-verdict ship_eligible flag,
// so a narrative PASS with ship_eligible:false slips through. TestC561_003 drives
// the RED unit test authored this cycle (audit_normalcompletion_reconcile_test.go).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557
// precedent) — each predicate shells `go test -run '^Name$' <fullModulePkg>` over
// the compiled SUT and asserts a clean (exit-0) run. Full module import paths so
// the subprocess resolves from the acs/cycle561 test cwd.
```

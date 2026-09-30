# Comment history: `acs/cycle786`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle786/predicates_test.go:3` — above `package cycle786`

```text
// Package cycle786 materializes the cycle-786 acceptance criteria for the
// single fleet-scoped todo merge-rung0-trivial-rebase-carryforward, decomposed
// by scout into three dependency-chained tasks (all triage ## top_n, R9.3):
// composition-verdict ledger entry → trivial-rebase fastpath in
// verifyAuditBinding → composed-tree native gates.
//
// AC map (1:1 with the inbox acceptance[] list):
//
//	AC1 "TestTrivialRebase_CarriesAuditForward (clean rebase, same patch-id:
//	    ship proceeds after native gates, no auditor dispatch)"
//	    → C786_001 runs that ship-package test as a subprocess. RED at
//	      authoring (fast path absent: CodeAuditBindingHeadMoved).
//	AC2 "TestTrivialRebase_PatchIdDriftFallsBackToReaudit"
//	    → C786_002 runs the drift test plus the failed-composed-gates
//	      rejection test (adversarial negatives — pre-existing GREEN today
//	      because moved-HEAD rejects everything; load-bearing the moment the
//	      fast path lands, pinning it never over-accepts).
//	AC3 "TestCompositionVerdict_KernelRecomputesPatchId (tampered entry
//	    rejected); ledger verify covers new entry kind"
//	    → C786_003 runs the cmd/evolve in-process `ledger verify` tests:
//	      tampered entries (drifted diff, forged patch_id) must exit 2, an
//	      honest entry must stay exit 0. RED at authoring (unknown kinds are
//	      ignored by Verify today).
//	AC4 "batch soak: audit dispatches ≈ cycle count; go test -race PASS;
//	    apicover clean"
//	    → manual+checklist in test-report.md (requires a live soaked batch);
//	      the -race half is enforced here by running C786_001/002 under
//	      -race.
//	Step 6b: C786_004 gates the three tasks' .evolve/evals/ files through the
//	    SSOT quality checker (non-vacuous, all commands PASS).
//
// Adversarial axes: negative (C786_002 drift + failed gates; C786_003
// tampered entries), edge (forged-patch-id vs drifted-diff are distinct
// forgeries), semantic (carry-forward, drift fallback, kernel recompute, and
// eval rigor are four distinct behaviors). Every predicate executes the
// system under test (go test subprocess / in-process checker) — no
// source-grep predicates (cycle-85 rule).
```

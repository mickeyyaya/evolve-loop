# Comment history: `acs/cycle296`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle296/predicates_test.go:3` — above `package cycle296`

```text
// Package cycle296 materializes the cycle-296 acceptance criteria for the two
// committed top_n tasks (scout-report.md — soak batch #6 reliability fixes):
//
//	T1  swarm-worktreebase-guard  — worktreeBase() returns EVOLVE_WORKTREE_BASE
//	    verbatim, including RELATIVE values; the IsAbs guard lives one call deeper
//	    in addWorktree. The inbox defect (swarm-tests-relative-worktree-base) wants
//	    the refusal in worktreeBase ITSELF. Fix: change worktreeBase to
//	    (string, error), add the IsAbs check there, and remove the duplicate from
//	    addWorktree (which now propagates the error).
//	T2  resume-inserted-phase     — RunCycleFromPhase rejects every phase that is
//	    not spine-valid via Phase.IsValid(), so a checkpoint whose resumeFromPhase
//	    is an advisor-inserted phase (e.g. "mutation-gate", registered in o.runners
//	    at runtime) cannot be resumed. Fix: also accept a startPhase present in
//	    o.runners, keeping the PhaseEnd/PhaseStart rejection.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real (unexported) worktreeBase and drive the real Orchestrator
// resume guard, then assert on the real `--- PASS:` / `--- FAIL:` lines. The
// functions under test are unexported, so an in-package white-box test driven by
// subprocess is the only way to exercise them — a magic string in a .go file can
// neither make worktreeBase return an error nor make the resume guard dispatch an
// inserted phase, so none of these is gameable by source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.guard  worktreeBase() itself refuses a relative base   → C296_001 (named PASS line)
//	T1.green  full internal/swarm suite stays green           → C296_002 (no FAIL line)
//	T2.accept inserted-in-runners phase accepted + negatives  → C296_003 (named PASS lines)
```

### `go/acs/cycle296/predicates_test.go:135` — above `prov := filepath.Join(goDir(t), "internal", "swarm", "provision.go")`

```text
// Auxiliary anti-duplication check (not RED-discriminating), FUNCTION-SCOPED
// so it cannot rot when later cycles legitimately add IsAbs elsewhere in the
// file (the original file-wide ==2 count rotted within a day — 0c210b52):
// the IsAbs guard belongs in worktreeBase and must NOT be duplicated in
// addWorktree.
```

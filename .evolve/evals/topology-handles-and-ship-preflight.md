---
score_cap:
  - criterion: "Ship pre-flight refuses untracked main-side colliders before the worktree commit, naming each path"
    max_if_missing: 7
    evidence: "cd go && grep -q 'func TestShipFromWorktree_ColliderPreflight(' internal/phases/ship/gitops_collider_test.go && go test -count=1 -run 'TestShipFromWorktree_ColliderPreflight|TestShipFromWorktree_ColliderError_IsActionable' ./internal/phases/ship/"
  - criterion: "Pre-flight does not block unrelated untracked main-side files (false-positive guard)"
    max_if_missing: 6
    evidence: "cd go && grep -q 'func TestShipFromWorktree_NoCollider(' internal/phases/ship/gitops_collider_test.go && go test -count=1 -run 'TestShipFromWorktree_NoCollider' ./internal/phases/ship/"
  - criterion: "Auditor C0 block is a kernel-owned documentation note, not an LLM instruction"
    max_if_missing: 4
    evidence: "grep -qE 'kernel.owned|cycle-state\\.json' agents/evolve-auditor.md && ! grep -qF 'Run EXACTLY (no improvised roots)' agents/evolve-auditor.md"
---

# Eval: Topology handles + ship collider pre-flight

> Pins the ship collider pre-flight introduced in cycle 232:
> `shipFromWorktree` must detect staged worktree paths that exist as
> untracked files in the main working tree and refuse with a
> precondition-class ShipError naming every collider — BEFORE creating the
> worktree commit. Without it, the ff-merge aborts post-commit ("would be
> overwritten by merge") and the orchestrator loops audit↔ship. Also pins
> the auditor C0 demotion: `--root` resolution is kernel-owned
> (cycle-state.json), no longer an auditor-LLM instruction. Source incident:
> campaign retrospective cycles 215-231 §4 Invariant 2 / I-10 (cycle-230
> audit↔ship recovery loop ×3); cycles 226-227 improvised `-root` false-FAILs.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| collider-refusal | pre-commit refusal, class=precondition, paths named | 7/10 | `go test -run 'TestShipFromWorktree_Collider…' ./internal/phases/ship/` (existence-guarded) |
| false-positive-guard | unrelated untracked files don't block ship | 6/10 | `go test -run TestShipFromWorktree_NoCollider ./internal/phases/ship/` (existence-guarded) |
| c0-demotion | auditor doc = kernel-owned note, imperative gone | 4/10 | positive + negative grep on `agents/evolve-auditor.md` |

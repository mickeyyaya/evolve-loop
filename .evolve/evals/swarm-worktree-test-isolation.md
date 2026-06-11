---
score_cap:
  - criterion: "addWorktree refuses a non-absolute (relative) worktree base before touching git"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestAddWorktree_RelativeBaseRefused ./internal/swarm/"
  - criterion: "running the swarmrunner suite registers zero worktrees in the live repo"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/phases/swarmrunner/ >/dev/null 2>&1; test \"$(git worktree list | grep -c swarmrunner)\" -eq 0"
---

# Eval: Swarm worker-provisioner worktree-base isolation

> Pins the worktree-base isolation contract introduced in cycle 294. The swarm
> WorkerProvisioner (`go/internal/swarm/provision.go`) builds its worktree base
> from `EVOLVE_WORKTREE_BASE` or `<projectRoot>/.evolve/worktrees`. Before this
> cycle the swarmrunner writer-failure test ran the real provisioner with
> `ProjectRoot:"."` and no base pin, so `worktreeBase(".")` returned the RELATIVE
> `.evolve/worktrees` and `git -C . worktree add` leaked
> cycle-1-{integration,w0,w1} into the LIVE repo on every test run (3 leaked
> worktrees confirmed by scout). The fix is two-fold: a guard in `addWorktree`
> refuses a non-absolute base before any git/filesystem mutation, and the
> swarmrunner test runs against an isolated temp git repo with an absolute base
> pin. This is the same relative/empty worktree-base defect class behind the
> cycle-280 and cycle-282 inserted-phase `worktree=""` carryover todos.
> Source incident: cycle 294 (scout-report.md T1 swarm-worktree-test-isolation).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard-refuses-relative | addWorktree rejects a non-absolute base before git | 8/10 | `go test -run TestAddWorktree_RelativeBaseRefused ./internal/swarm/` |
| suite-leaves-no-leak | swarmrunner suite registers 0 repo worktrees | 9/10 | `go test ./internal/phases/swarmrunner/ && git worktree list \| grep -c swarmrunner == 0` |

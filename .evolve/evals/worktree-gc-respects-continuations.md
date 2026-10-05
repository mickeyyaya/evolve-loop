---
score_cap:
  - criterion: "`evolve worktree cleanup --stale` is a dry-run by default: it lists the sealed, unbound cycle worktrees and removes no worktree, registration or branch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_001_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --stale --apply` removes exactly the sealed, unbound worktrees (a stale lease does not protect) with their superseded branches, and keeps bound, freshly leased and unsealed lanes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_002_' ./acs/cycle1799"
  - criterion: "A worktree named by a continuation binding, or a lane with a fresh run lease, is reported kept with its reason in both dry-run and --apply"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_003_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --stale` with no cycle worktrees exits 0 in both modes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_004_' ./acs/cycle1799"
  - criterion: "`evolve branches audit` reports a live lane's branch as live, never superseded, while a sealed dead branch contained in main stays superseded=true"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_005_' ./acs/cycle1799"
  - criterion: "The existing worktree and branches CLI behavior does not regress"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run 'Worktree|Branches'"
---

# Eval: Stale cycle worktrees get a safe CLI cleanup that respects continuations and live lanes

> Pins the `worktree-gc-respects-continuations` inbox item (console, 2026-09-28; built in cycle 1797).
> At the operator's request to remove legacy worktrees and branches, the console found no safe CLI
> path: `evolve worktree cleanup` without `--cycle` only ran `git worktree prune`, `--cycle N` ran
> `git worktree remove --force` with no check for a live run lease, a continuation binding or a
> salvage pointer, and `evolve branches audit` marked the live lanes' branches (no commits yet, so
> contained in main) as superseded. All 95 worktrees (7.2 GB) and 89 branches were left in place.
> The predicates drive the real `evolve` binary against a fixture with five lanes: sealed and
> unbound, sealed and bound by branch through a continuation binding whose worktree path is stale,
> sealed with a fresh lease, unsealed, and sealed with a stale lease.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dry-run-default | lists removable lanes, removes nothing | 7/10 | `go test -tags acs -run TestC1799_001_ ./acs/cycle1799` |
| apply-exactly | removes only sealed, unbound lanes and their branches | 8/10 | `go test -tags acs -run TestC1799_002_ ./acs/cycle1799` |
| kept-with-reason (negative) | bound and leased lanes kept with reason | 7/10 | `go test -tags acs -run TestC1799_003_ ./acs/cycle1799` |
| empty-set (edge) | no cycle worktrees, exit 0 | 5/10 | `go test -tags acs -run TestC1799_004_ ./acs/cycle1799` |
| audit-live (negative) | live lane never superseded | 7/10 | `go test -tags acs -run TestC1799_005_ ./acs/cycle1799` |
| regression | existing worktree/branches tests | 6/10 | `go test ./cmd/evolve -run 'Worktree\|Branches'` |

## Adversarial Cases
- Negative: a sealed lane named by a continuation binding by branch only (the binding's worktree path points at a retired hub) must be kept.
- Negative: a fresh lease keeps a sealed lane; a lease whose heartbeat is three hours old does not, even though its owner pid is alive.
- Edge: an unsealed lane is reported kept and never removed; an empty worktree set is a clean no-op.
- Cheapest gaming fake: treating every lane as kept passes the dry-run's no-removal check but fails `TestC1799_002_`; printing `live` for every audited branch fails `TestC1799_005_`'s superseded=true control.

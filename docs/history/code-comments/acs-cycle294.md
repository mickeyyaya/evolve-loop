# Comment history: `acs/cycle294`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle294/predicates_test.go:3` — above `package cycle294`

```text
// Package cycle294 materializes the cycle-294 acceptance criteria for the two
// committed top_n tasks (scout-report.md — swarm worktree-base isolation +
// dispatch semaphore-cancel coverage):
//
//	T1  swarm-worktree-test-isolation       — the swarmrunner writer-failure test
//	    ran the real WorkerProvisioner with ProjectRoot:"." and no
//	    EVOLVE_WORKTREE_BASE pin, so worktreeBase(".") returned the RELATIVE
//	    ".evolve/worktrees" and `git -C . worktree add` leaked
//	    cycle-1-{integration,w0,w1} into the LIVE repo every run. Fix: (a) a guard
//	    in addWorktree refuses a non-absolute base before touching git; (b) the
//	    test runs against an isolated temp git repo with an absolute base pin.
//	T2  swarm-dispatch-semaphore-cancel      — Dispatch's `case <-rootCtx.Done()`
//	    arm (a worker still queued on the bounded semaphore when a sibling's fatal
//	    failure cancels the root context) was uncovered (Dispatch func = 96.0%).
//	    Fix: a new test drives 3 workers at Concurrency:1 with a failing+slow w0 so
//	    a queued worker observes context.Canceled.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test — they call the real provisioner, run the real swarmrunner
// suite and inspect `git worktree list`, run `go test -v` and assert on the real
// `--- PASS:` lines, and read the real `go tool cover -func` Dispatch number. A
// magic string in a .go file can neither refuse a relative base, remove a
// registered worktree, produce a named PASS line, nor move a coverage number, so
// none of these is gameable by source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.guard  addWorktree refuses a relative base                 → C294_001 (direct call)
//	T1.noleak swarmrunner suite leaves 0 repo worktrees           → C294_002 (suite + git)
//	T1.suite  full swarm/swarmrunner suite stays green            → manual+checklist (auditor)
//	T2.test   TestDispatch_CancelWhileQueuedOnSemaphore PASSes    → C294_003 (PASS line)
//	T2.cover  Dispatch function coverage >= 97%                   → C294_004 (cover -func)
```

### `go/acs/cycle294/predicates_test.go:167` — above `func TestC294_002_SwarmrunnerSuiteLeavesNoRepoWorktrees(t *testing.T) {`

```text
// --- C294_002 (T1.noleak): running the swarmrunner suite leaks 0 repo worktrees -
//
// The strongest anti-no-op signal and the scout verifiableBy
// (`git worktree list | grep swarmrunner | wc -l == 0`). Runs the real swarmrunner
// suite (the writer-failure test drives the real provisioner in writer+enforce
// mode) and then inspects the LIVE `git worktree list`. A registered worktree is a
// real side effect no source string can fake or erase.
//
// RED baseline: cycle-1-{integration,w0,w1} are already registered (scout
// confirmed 3 leaked) and the unfixed test re-leaks; GREEN requires BOTH the
// isolated-temp-repo test rewrite AND removal of the 3 stale worktrees.
```

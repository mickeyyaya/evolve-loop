# Plan: each cycle ends with a mandatory recycle step, and gc checks each host resource it owns

- **Status:** partly built, 2026-11-21 (1 of 12 component rows on main). Earlier status: proposed, 2026-11-20. Amended 2026-11-22 by section 13. Round 2 with the in-session fixes of its review (sections 11 and 12 list each finding and its fix). The design review comes before any build.
- **Design:** [internal-gc](../architecture/packages/internal-gc.md), [internal-gcpolicy](../architecture/packages/internal-gcpolicy.md), [the shadow stage contract](../architecture/shadow-stage-contract.md). **Earlier plan:** [gc closes the legacy gap](gc-closes-the-legacy-gap-2026-11.md).
- **Evidence:** the measurements and the manual recovery of 2026-11-20 (section 2), and the incident record "salvage bundles filled the disk and hung a loop test" in [PR #1584](https://github.com/mickeyyaya/evolve-loop/pull/1584).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)
- [11. Review of round 1](#11-review-of-round-1)
- [12. Review of round 2](#12-review-of-round-2)
- [13. Amendment 2026-11-22: the static gc flow](#13-amendment-2026-11-22-the-static-gc-flow)

## Terms used throughout

- **Recycle step.** The last step of each cycle. It writes the archive leaf and the recycle record of the cycle, and then it releases the resources of the cycle.
- **Archive leaf.** One directory per cycle in the existing salvage home (`OperatorSalvageDir`). It holds the work of the cycle that `origin/main` does not hold.
- **Thin bundle.** A git bundle that holds only the commits that `origin/main` does not hold (`git bundle create <file> <tip> ^origin/main`).
- **Landed.** After a fetch, the cycle tip is reachable from `origin/main`, or the ship receipt names a commit that `merge-base --is-ancestor` finds in `origin/main`. The hub `HEAD` never counts. A cycle that is only in an integration branch has not landed, and a failed fetch means "not landed".
- **Held.** `sealHold(cycle)` returns a reason: the cycle is paused with a resumable checkpoint, a continuation binds it, or another process holds its run lease (`go/internal/gc/cycleseal.go:61-74`).
- **Failed to deliver.** The cycle sealed with a final verdict other than PASS, and it is not held.
- **Check.** One rule of the gc check process. It scans one kind of resource, gives the rule and the evidence for each item, and applies one action.
- **Holder.** A live process, lease or open pull request that still needs an item.

## 1. Request

On 2026-11-20 the operator sent four messages:

1. "Build the mandatory requirement that when a cycle is completed, it must invoke gc to clean up all it's worktrees, branches if it successfully delivered, ultrathink to learn the best way to recycle the resources for the cycle when it's done."
2. "Make the recycle as the mandatory static step that when the cycle changes has been merged (to the integration branches or main) or failed to deliver, it should compress the history into archive and clean up the mess."
3. "Also identify what takes all the storage space outside of branch and worktrees, clean up the temp and cache file if it's no longer required"
4. "Put all your scan and justification into static gc check process"

The same day the host disk fell to 2.6% free, and four cycles of wave 182 failed for it. Section 2 gives the scan, and section 5 turns each manual step of the console into a check.

## 2. Facts

All numbers are from the host of the hub on 2026-11-20 (disk of 460 GB), unless a line says otherwise.

### 2.1 The cycle resources

**F1.1** At 23:30 UTC on 2026-11-19 the disk had 12 GB free (2.6%). The disk guard pauses a new dispatch under 5% (`go/internal/looppreflight/disk_percent.go:10`). Cycles 2547, 2548, 2550 and 2551 failed their ACS for it.
*Implication:* a resource leak stops the loop. Recycle is a correctness need, not hygiene.

**F1.2** `.evolve/operator-salvage` held 254 leaves and 187 GB, a mean of 0.74 GB for each leaf. The newest `refs.bundle` files were about 1.1 GB. `writeSalvageLeaf` bundles the branch ref with no negative base (`go/internal/gc/salvage.go:75`, `bundleVerified` at `:85`), and `bundleBranch` does the same (`go/internal/gc/branchsweep.go:115-125`). So each bundle holds the full history up to its tip. The leaf retention `salvage_ttl_days` is 30 days (`go/internal/gc/gc.go:79-81`, `go/internal/gcpolicy/gcpolicy.go:53-54`), and all 254 leaves were younger.
*Implication:* the retention exists, but a full bundle for each leaf makes 30 days of leaves too large for the disk.

**F1.3** A failed cycle keeps its worktree and its branch. `teardownCycleWorktree` preserves the tree when the cycle did not complete normally (`go/internal/core/cycle_worktree_teardown.go:12-18`). The continuation binds the snapshot commit (`SnapshotSHA`) of the cycle, and the sealed sweep holds a bound cycle (`go/internal/gc/cycleseal.go:66-71`). After 24 hours the worktree sweep salvages the tree into a leaf and keeps the branch.
*Implication:* the work of a failed cycle is kept three times. It is in the branch, in the worktree for 24 hours, and in a full bundle.

**F1.4** The hub held 31 `cycle-*` branches. 21 of them had no worktree, and the oldest was from 2026-11-09. Four cycle worktrees were from before wave 182 (cycles 2232, 2508, 2520 and 2532).
*Implication:* no step ends the life of a failed cycle's branch.

**F1.5** The sealed sweep runs only when `gc.mode` is `enforce` (`go/cmd/evolve/cmd_loop_sealed_gc.go:14-16`). Three command entry points call it (`go/cmd/evolve/cmd_cycle.go:280`, `cmd_loop_resume.go:76`, `cmd_loop_sequential_dispatch.go:53`). The cycle state machine does not call it.
*Implication:* the end of a cycle is a policy hook of some callers, not a step of each cycle.

**F1.6** The run dir of a cycle has readers after the seal. The closeout reads it (`go/cmd/evolve/cmd_cycle.go:277`, `:306-333`). The continuation writes `continuation-manifest.json` into it and points `FindingsPath` at `audit-fail-reason.json` in it (`go/internal/core/continuation_stamp.go:70-78`). Adoption reads the findings from there (`:174-177`).

The run ladder owns the run dirs (`go/internal/gc/gc.go:90-103`). It archives them under `.evolve/archive/runs` (`:178-194`), which has no retention, and it deletes them after `runs.delete_after_days` (`:96-97`).
*Implication:* the recycle step must not delete a run dir. It already has one owner, and its readers run after the seal.

### 2.2 The host resources outside the branches and the worktrees

**F2.1** The Go build cache held 56.1 GB in 169,746 files. 5,038 directories of the shape `<hash>-d` held 35.9 GB, and 35.8 GB of that was cached `evolve` executables. 30.2 GB of them were unused for more than 24 hours. `TrimGoCache` skips each directory entry (`go/internal/gc/gocache.go:88`). After its trim, `evolve gc` reported "20.00 GB remains", and `du` measured 53 GB.
*Implication:* the cap of 20 GB does not bound the cache, because the largest entries are not visible to the trim.

**F2.2** The cache trim, the temp reap and the process reap run only in `evolve gc` (`go/cmd/evolve/cmd_gc.go:297`, `:412`, `:434`). The loop hook does not run them ([internal-gc](../architecture/packages/internal-gc.md), "Two callers drive it").
*Implication:* each of these leaks grows until a person runs `evolve gc`.

**F2.3** The temp directory held 11 GB. 131,402 top-level entries were older than 24 hours (1.3 GB). The largest groups were `evcodexhome*` (42,990), `evghconfig*` (42,976), `evcodeusage*` (42,976) and `buildfloor-cover-*` (1,979). `pipelineTempRe` matches none of these shapes (`go/internal/gc/tempdir.go:12`).
*Implication:* the temp reap knows a fixed list of shapes, and new code adds shapes that the list does not get.

**F2.4** `TestMain` of `cmd/evolve` makes `evcodexhome` and `evcodeusage` (`go/cmd/evolve/main_test.go:69`, `:78`) and removes them after `m.Run` (`:91-100`). The dirs stay only when a test binary is killed, for example by a test timeout.
*Implication:* a killed process cannot clean up. Only a reaper outside the process can.

**F2.5** 282 tmux servers on sockets `evolve-bridge-t<N>` had the parent pid 1, and 276 were older than one day. Test helpers start them with `exit-empty off` (`go/internal/bridge/tmux_own_server_integration_test.go:30`). A reaper exists: `swarm.ReapOrphanSockets` reads the owner pid from the socket name and spares a live owner (`go/internal/swarm/reap_orphans.go:111`). Its injected lister (`:123-125`) reads only the socket dir of its own process (`go/internal/bridge/tmux.go:90-95`). The test binaries put their sockets under `TMUX_TMPDIR=/tmp/evtmux*` (`go/cmd/evolve/main_test.go:56-61`).
*Implication:* the reaper has the right liveness rule but does not see the test socket dirs.

**F2.6** 49 `go-build*` work dirs were older than three hours (2.1 GB), and six `go` commands were live.
*Implication:* the temp reap horizon of 24 hours keeps the dirs of killed builds for a day.

**F2.7** The gh HTTP cache (`~/.cache/gh`) held 4.4 GB, and 4.3 GB of it was older than 24 hours.
*Implication:* a cache with no trim grows with each `gh` call.

**F2.8** Outside the pipeline: the Homebrew cache held 3.9 GB, the pip cache 0.9 GB, Claude Code transcripts 12 GB and other projects 14 GB.
*Implication:* the pipeline does not own these, so gc must not change them.

### 2.3 The manual recovery of 2026-11-20

The console released space by hand, with the justification in the right column. Each row becomes a check in section 5.

| Action | Freed | Justification |
|---|---|---|
| `evolve gc --only cache` | 12.5 GB | the existing trim to the 20 GB cap |
| Delete 48 salvage leaves | about 50 GB | the tip is an ancestor of `origin/main`, and the leaf has no patch and no untracked files. The index of all 254 leaves is a host file, `backups/operator-salvage-index-2026-11-20.json` of the hub |
| Delete 4,111 cached executable dirs | 29.4 GB | no file in the dir changed in 24 hours, and no live process had a file of the dir open or mapped (`lsof` snapshot) |
| Delete gh cache files older than 24 hours | 4.3 GB | expired HTTP responses. The gh command fetches again on a miss |
| `brew cleanup --prune=7`, `pip cache purge` | 5.0 GB | the operator asked for it. These caches are outside the pipeline |

Free disk went from 12 GB to 107 GB. Two scan errors happened in the console's own shell commands, and each one is a reason for a tested check:

- The first age test used `find -newermt "-24 hours"`. The `find` of this host is `bfs`, which refuses that time format. The error made each dir look stale. The console saw the error before the delete.
- The console matched each of 131,402 temp entries against a list, one entry at a time, and the run took more than ten minutes. gc does not have this defect: `LiveHolderProbe` takes one `lsof` snapshot for each run (`go/internal/gc/openpaths.go:18`).

### 2.4 Outside research

**R1** A bundle holds the objects that its prerequisites do not reach. A range `old..new` needs `old` in the receiver, and `git bundle verify` checks that ([git-bundle](https://git-scm.com/docs/git-bundle/2.48.0)). Git refuses to create an empty bundle.
*Implication:* a thin bundle with `^origin/main` holds only the work of the cycle. A tip that is in main has no bundle, so the leaf records the tip instead.

**R2** The Go build cache keeps the package directory out of the key only under `-trimpath` ([build_cache_trimpath.txt](https://go.dev/src/cmd/go/testdata/script/build_cache_trimpath.txt)).
*Implication:* each new worktree path makes new cache entries for each package. This explains the growth of F2.1.

**R3** The Kubernetes TTL-after-finished controller deletes a finished Job together with its dependents, and it waits for the finalizers first ([TTL after finished](https://v1-34.docs.kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)).
*Implication:* the model is an owner, a terminal state, a cascade to the dependents, and a finalizer that must complete before the delete. The recycle step is the cascade, and the archive leaf is the finalizer.

**R4** For CI runners, current practice is a single-use runner and a deterministic cleanup, not a best-effort one. Logs, caches and workspaces outlive a job unless a step destroys them ([ephemeral CI runners](https://vexxhost.com/blog/ephemeral-ci-runners/)).
*Implication:* the end of a cycle needs a step that always runs, not a hook that some callers run.

## 3. Goals and non-goals

| Id | Goal | Measure |
|---|---|---|
| G1 | Each cycle ends with the recycle step on each terminal path: ship, FAIL, abort and resume. A boundary check covers a crash | cycles with no recycle record and no hold: 0 |
| G2 | The archive is small and can restore the work | a failed cycle leaf is under 50 MB. The proposed verb `evolve archive restore` rebuilds the branch |
| G3 | Each host resource that the pipeline makes has a check with a rule, a justification and an action | the proposed verb `evolve gc check` lists each item with its rule and its evidence |
| G4 | The loop runs the checks itself: the cycle checks in the recycle step, the host checks at each boundary | no manual `evolve gc` is needed for 7 days |
| G5 | No check releases a live item | each apply re-checks the holder, and the tests prove it |

Non-goals:

- Caches and files that the pipeline does not make (F2.8). The check reports their size and does not change them.
- Remote branches. The earlier plan owns them.
- A change of the ship, the audit or the integration train.

## 4. Candidate designs

- **A. Wider sweeps.** Keep the sweeps of `evolve gc`. Add the missing shapes, count the executable dirs, and call all sweeps from the batch hook.
- **B. A recycle step and a check registry.** The cycle lifecycle gets one static recycle step. gc gets one registry of checks, and each check adapts an existing `Plan`/`Apply` pair or adds a pair. The recycle step runs the cycle checks, and the boundary and `evolve gc` run the host checks. A boundary check recycles each cycle that has no recycle record and no hold.
- **C. A fixed pool of lane slots.** Eight worktrees with fixed paths (`lane-1` to `lane-8`). Each cycle resets a slot instead of a new worktree.

| Criterion | A | B | C |
|---|---|---|---|
| Runs on each terminal path of a cycle (G1) | 1: a hook of some callers | 3: a step of the lifecycle | 2: a reset at the next claim |
| Each release has a justification (G3) | 1: counts only | 3: an item, a rule and evidence | 1 |
| Bounds the Go cache (F2.1) | 2 | 2 | 3: one path for each lane |
| Keeps cycle isolation | 3 | 3 | 1: state of one cycle reaches the next |
| Reuses the existing code | 3 | 2 | 1 |
| Size of the change | 3 | 2 | 1 |
| **Total** | 13 | **15** | 9 |

B wins. Only B makes the end of a cycle a step, as the operator asked. Only B records why each item was released. C bounds the cache better. But a reset slot carries state from one cycle to the next, and the liveness rules need one path for each cycle. Open question O1 keeps the cache part of C for a measured decision.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The recycle step is a static step of the cycle lifecycle. It runs after each continuation stamp: the FAIL stamp in `finalizeCycle` (`go/internal/core/cyclerun.go:198-201`), and the abnormal and deadline stamps (`go/internal/core/cyclerun_epilogue.go:54`, `:71`). It runs after the final state write, last in the defer stack (`go/internal/core/orchestrator.go:554-557`). A parked exit writes no stamp, and its paused checkpoint holds it through `sealHold`. No mode skips the step or its record, and the step always writes and verifies the archive leaf and the recycle record. These actions do not delete | F1.5, R3, R4. The continuation stamp writes the binding that `sealHold` reads | A hook at `emitCycleClose`: it runs before the stamp, so a timed-out cycle loses its continuation. A policy hook (today): a caller can skip it |
| D2 | Each new or wider destructive rule has a stage in `policy.json` (`gc.checks.<id>.stage`). It starts in shadow and goes to enforce on a verified record of [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md). An adapter of a sweep that deletes today keeps its current stage. The stage is configuration data, not a feature flag | the shadow stage contract | Each check in shadow: the sweeps that enforce today stop, and the leak of F1.1 comes back |
| D3 | The recycle step reads the hold through `sealHold(cycle)`. It does not restate the rule. A held cycle gets its leaf and its record, and its release waits for the boundary check | F1.3, `cycleseal.go:61-74` | A second rule in the step: it is weaker, and it releases a cycle under a live lease |
| D4 | A landed cycle loses its worktree and its branch, and its leaf records the landed tip. The landing check of the sealed sweep changes its target from the hub `HEAD` (`go/internal/gc/worktrees.go:267`, `:285`) to the fetched `origin/main`. Each branch delete writes the leaf first | F1.4, R1 | The hub `HEAD`: it can hold an integration merge that is not pushed, and then a delete with no bundle loses the cycle |
| D5 | A cycle that failed to deliver gets a leaf with the uncommitted patch and the untracked files. Its branch moves to `refs/archive/cycles/<leaf>` with a compare-and-delete (`update-ref -d <ref> <expected>`). The ref is the one copy of the commits. The leaf writes a thin bundle only when a check drops the ref | F1.3, R1 | A ref and a bundle together: two copies of one piece of work |
| D6 | A cycle that delivered and has not landed loses its worktree. Its branch stays. The landing proof is the ship receipt, and an end rule (`EndedAfter`) moves a branch that never lands to D5 | F1.5, `go/internal/gc/squashlanded.go:40-60` | Squash path equality alone: a later main commit makes it false, so the branch stays for ever |
| D7 | The recycle step does not delete the run dir. The run ladder stays its one owner. It gets a retention for `.evolve/archive/runs`, and it keeps the run dir of a cycle while a continuation binds the cycle | F1.6 | Delete the run dir in the step: the closeout and the continuation read it after the seal |
| D8 | The leaf lives in the salvage home (`OperatorSalvageDir`), with the existing file names. `salvage_ttl_days` stays its retention, with a size cap added. A leaf and its `refs/archive` ref go together, never while a continuation binds the cycle | F1.2 | A new `.evolve/archive/cycles` root: `protected()` refuses each path under `archive` (`go/internal/gc/gc.go:196-208`), and a second owner of the layout |
| D9 | Each bundle writer makes a thin bundle. When `rev-list --count origin/main..<tip>` is 0, the writer records the tip and writes no bundle | F1.2, R1 | A full bundle: up to 1.1 GB for each leaf |
| D10 | gc gets one registry of checks. A check is an adapter: `Scan` calls the existing `Plan*`, the justification reads its `Rule` or `Reason`, and `Apply` calls the existing `Apply*`. A new resource gets a new pair | F2.2, the request | A second set of sweeps: each belief then has two owners |
| D11 | The proposed `evolve gc check` prints the report, `evolve gc` applies it, and the loop runs the host checks at each boundary | F2.2, G4 | Manual runs only (today) |
| D12 | Each check that deletes a file uses the `LiveHolderProbe` snapshot and spares each item with an open or mapped file. Each apply re-checks the mtime of the item | the manual recovery, `openpaths.go:18` | mtime alone: a running binary keeps its old mtime |
| D13 | gc releases only what the pipeline makes: names that the creator registers, paths under the evolve dir, and sockets with the `evolve-` prefix. Other caches are report-only | F2.8 | Clean each cache on the host: gc does not own them |
| D14 | The recycle step and the apply re-checks get rows on the integrity surface (`go/internal/guards/integrity_surface.go`) | a delete rule is a guarantee | Lane-editable rules: a later cycle can weaken a delete rule after a normal audit |
| D15 | The recycle step is the one releaser of a cycle. X4 removes the release from `gitWorktree.Cleanup` and `deleteCycleBranch` (`go/internal/core/worktree.go:205-231`). Then `teardownCycleWorktree` removes no tree and runs no `branch -d`. X4 makes the three sealed-sweep calls (`go/cmd/evolve/cmd_cycle.go:280`, `cmd_loop_resume.go:76`, `cmd_loop_sequential_dispatch.go:53`) call the step. The step is idempotent on its record, so a second call does nothing | the docs review of round 2, the patch check of round 2. `branch -d` judges "merged" against the hub `HEAD` and writes no leaf | Keep the in-cycle teardown next to the step: it deletes a branch with no leaf before the step runs. It also removes ignored files before K1 can archive them |
| D16 | K4 calls a cycle crashed only when the owner pid of its lease is dead. A lease that cannot be read and has a fresh file time counts as held, as in `go/internal/core/lane_resume_lease.go:25-31`. An absent lease counts as held while the run dir is younger than the lease TTL. The first lease write only logs a failure (`go/internal/core/runlease_hook.go:22-24`). K4 does not exclude its own pid. A candidate also needs no holder in its tree and no cycle state in a running phase | the lease rules of `go/internal/runlease/runlease.go:104-107` and `go/internal/gc/worktrees.go:198-201`. The holder and phase rules are new | A stale heartbeat as a crash: a lane under disk pressure is alive, and K4 then removes its tree |

### The check catalog

Each row is one check. The justification column is the evidence that the report prints for each item.

| Id | Scope | Resource | Rule | Justification | Action | Owner today |
|---|---|---|---|---|---|---|
| K1 | cycle | cycle worktree | the cycle sealed, is not held, and is not the project root. `gateLeftovers` finds no ignored file outside the rebuildable list | seal record, `sealHold`, holder probe, the ignored files | archive the dirty files, then `git worktree remove` | `teardownCycleWorktree`, `releaseLaneTree` |
| K2 | cycle | cycle branch | landed (D4), failed to deliver (D5), or delivered and not landed past `EndedAfter` (D6) | the landing proof or the final verdict | landed: delete. Otherwise: move to `refs/archive/cycles/` | `releaseSealedLane` |
| K3 | cycle | processes and tmux sessions of the cycle | the existing rules | `ps` and the session tag | the existing reap (injected sender) | `SweepSealedCycle`, `swarm.ReapRunSessions` |
| K4 | host | cycles with no recycle record | the lease owner pid is dead (D16), no holder in the tree, and no running phase | `sealHold`, the lease pid, the holder probe and the cycle state | run the recycle step | new |
| K5 | host | salvage leaves and their `refs/archive` refs | older than `salvage_ttl_days` and no continuation binding. Over the size cap, only landed leaves go, the oldest first | the leaf age, the size, the landing and the binding | write a thin bundle when the ref holds unlanded work, then delete the ref. Delete the leaf at its age limit | `Plan` (`salvage_ttl_days`) |
| K6 | host | legacy full-bundle leaves | a full bundle | the bundle size and its tip | fetch the full bundle into a temporary ref, write a thin bundle, verify `list-heads`, replace, and drop the temporary ref. Keep the full leaf on each failure. Delete a leaf only when its tip is in `origin/main`, it has no patch and it has no untracked files | new |
| K7 | host | Go build cache, with the `<hash>-d` dirs | unused past the horizon, or over the cap, and no open or mapped file | the newest mtime and the holder probe | delete | `TrimGoCache` |
| K8 | host | pipeline temp entries | a registered shape, older than the horizon, and no holder, a live socket included | the shape id and its creator file | delete | `ReapPipelineTemp` |
| K9 | host | orphan `go-build*` dirs | older than three hours, and no holder | the age and the holder probe | delete | `ReapPipelineTemp` |
| K10 | host | orphan tmux test servers | the owner pid in the socket name is dead. The lister reads the registered test socket dirs too | the owner pid and `ps` | `kill-server` on that socket, before K8 | `swarm.ReapOrphanSockets` |
| K11 | host | gh HTTP cache | older than 24 hours | the file age | report only until O3 is decided | new |
| K12 | host | the disk, `.evolve/archive/runs` and the hub `.git` included | free space under the warn level | `statfs`, and the top consumers with their owners | report only | new |
| K13 | host | the workspaces of `evolve agent run` under `.evolve/ephemeral/agent-runs/` | the run ended, no live lease or pane, older than the horizon | the run record and the holder probe | delete | new |

## 6. Components

Each component is one inbox item and one pull request.

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| X0 | Thin bundles in `bundleVerified` and `bundleBranch` (D9) | a leaf of a branch one commit past main is under 1 MB. A tip in main gives a leaf with the tip and no bundle | lane |
| X2 | The trim counts the `<hash>-d` executable dirs and spares a held one (K7, D12) | a fixture cache with an old executable dir is trimmed. A dir with a mapped file in the injected snapshot stays | lane |
| X1a-X1e | The check registry and the proposed `evolve gc check`, one sweep for each pull request: worktrees, run dirs, cache, temp, processes (D10). X1a moves `gateLeftovers` out of the dev sweep (`go/internal/gc/devleftovers.go:40`) into a function that K1 can call. X1a also changes the landing target to `origin/main` (D4) in the plan and in the apply re-check `deleteSquashed` (`go/internal/gc/worktrees.go:614-632`). It uses `update-ref -d` in place of `branch -d` | the report of a fixture tree lists each item with its rule and its evidence. A hub whose `HEAD` holds an unpushed merge of the cycle keeps the branch | lane |
| X3 | The leaf writer in the salvage home and the proposed `evolve archive restore` (D5, D8) | a round trip restores the branch, the patch and the untracked files of a fixture cycle | lane |
| X5 | The continuation and the resume read the leaf when the branch or the run dir is gone. The leaf restores the manifest and the findings | an adoption after the release gets the findings of the failed audit, not "none" | console: `go/internal/core/resume_execution.go` is protected (`integrity_surface.go:128`) |
| X4 | The recycle step after the epilogue, in the seal path and the resume path (D1, D3 to D6, D15). Its new destructive actions start in shadow (D2). It calls `RunCycleChecks` of X4b (section 13.7) | a PASS cycle, a FAIL cycle, a resumed cycle, an abnormal exit and a deadline exit each end with a verified leaf and a record. The deadline case keeps its continuation. A PASS cycle whose branch is merged only into the hub `HEAD` keeps its branch. A second call of the step does nothing. The shadow stage deletes nothing | console: `go/internal/core/cyclerun.go` is protected (`integrity_surface.go:103`) |
| X6 | K5 size cap, K6 legacy migration, the retention of `.evolve/archive/runs` (D7, D8), and the retention of `refs/archive/branches/` and `branch-gc/` (D21b). It replaces the runtime inbox item `gc-salvage-bundles-hold-the-whole-history-with-no-retention`, filed on 2026-11-20 and published at the next boundary | a full bundle becomes a thin bundle that verifies. A leaf with untracked files stays. A branch-gc stamp dir past its age goes with its ref | lane |
| X8 | `ReapOrphanSockets` lists the registered test socket dirs, and the test helpers tie the server to the life of the test binary (K10) | an orphan test server is stopped, and a server whose owner pid is alive stays | lane |
| X7 | The temp shape registry replaces `pipelineTempRe`, and a guard test checks each `os.MkdirTemp` pattern of the repository (K8). K8 runs after K10 | the guard fails on an unregistered pattern. A temp dir with a live socket stays | lane |
| X9 | The loop runs the host checks at each boundary, and K4 recycles a crashed cycle (D11, D16) | a boundary run of a fixture loop runs each host check once. A cycle with a stale heartbeat and a live pid stays, and an unreadable fresh lease stays | lane |
| X10 | K11, K12 and K13, and the integrity rows of D14 | the report names the top consumers and their owners. An ended agent-run workspace goes, and a live one stays | console: the integrity surface is protected |

## 7. Order

1. X0 and X2 first. They are small, and they bound the two largest leaks (F1.2, F2.1).
2. X1a to X1e next, one sweep at a time, because each later check plugs into the registry.
3. X3 before X5, because X5 reads the leaf.
4. X5 before X4. The recycle step releases a failed cycle's branch only after the continuation and the resume can read the leaf.
5. X4 in shadow for one wave, then enforce on a verified record (D2). X1a, with the `origin/main` landing target, lands before X4 goes to enforce.
6. X6, then X8 before X7, because K8 must run after K10.
7. X9 after the checks that it runs.
8. X10 last.

## 8. Verification

| Measure | Before (2026-11-20) | Target |
|---|---|---|
| Free disk | 12 GB (2.6%) before the recovery | over 20% with no manual run for 7 days |
| `operator-salvage` | 254 leaves, 187 GB | thin leaves only, under 5 GB |
| `cycle-*` branches with no live or held cycle | 21, the oldest 11 days | 0 older than one boundary |
| Go build cache | 56.1 GB | under the cap plus the in-use window |
| Temp entries older than 24 hours | 131,402 | 0 of the registered shapes |
| Orphan tmux test servers | 282 | 0 |
| Cycles with no recycle record and no hold | not measured | 0 |

Each check also has its unit tests, with 100% coverage of the changed functions, and one end-to-end test through the real entry point.

## 9. Risks

| Risk | Control |
|---|---|
| The step deletes work that a continuation needs | D3 reads `sealHold`. X5 lands before X4. The leaf and its ref go only when no continuation binds the cycle (D8) |
| The step deletes a run dir that the closeout reads | the step does not delete run dirs (D7) |
| A batch branch is rebuilt without a landed cycle | "landed" means the fetched `origin/main` only, never the hub `HEAD` (D4) |
| A boundary run recycles a live lane | K4 needs a dead lease owner pid, no holder and no running phase (D16) |
| The step runs before the continuation stamp | the step runs after the epilogue and the final state write (D1) |
| A check deletes a live temp dir, cache entry or binary | one holder snapshot for each run, with the open and the mapped files, and a re-check of the mtime before each delete (D12) |
| A check kills a live tmux test server | K10 uses the owner pid of the socket name, not the parent pid (F2.5) |
| A wrong rule deletes many items at once | one wave in shadow with the report (D2) |
| A later cycle weakens a delete rule | the integrity rows of D14 |
| The step makes the seal of a lane slower | measure the time of the step in each seal signal |
| A test kills real processes | each kill takes an injected sender. No test runs a live kill guard |

## 10. Open questions

- **O1.** Build the pipeline binaries with `-trimpath` (R2), or use a fixed pool of lane paths (design C)? Measure the cache growth for each cycle first.
- **O2.** The size cap of the salvage home: a fixed number of GB, or a share of the free disk?
- **O3.** Does the pipeline own the gh HTTP cache, or does the console? Until the answer, K11 is a report row.
- **O4.** When the integration train goes to enforce, does a landed batch commit count as a ship receipt for D6?

## 11. Review of round 1

The adversarial review `agent-adversarial-review-20261120T002736Z-20ef` gave FAIL: 0 CRITICAL, 6 HIGH, 10 MEDIUM and 7 LOW. It verified all 22 code citations. The docs review `agent-docs-review-20261120T002739Z-7339` gave PASS: 1 MEDIUM and 4 LOW.

| Finding | Fix in round 2 |
|---|---|
| H1: the step deleted the run dir before its readers | F1.6, D7: the run ladder stays the owner, and X5 restores the manifest and the findings |
| H2: "failed to deliver" restated a weaker hold rule | Terms, D3: the step calls `sealHold` |
| H3: an integration branch counted as landed | Terms, D4: `origin/main` or the ship receipt, and the leaf keeps the bundle |
| H4: K6 deleted a leaf with untracked files | K6 needs no patch and no untracked files |
| H5: K7 used mtime alone | D12, K7: the holder snapshot with the mapped files |
| H6: K10 used the parent pid | F2.5, K10, X8: the owner pid rule of `ReapOrphanSockets` and the test socket dirs |
| M1: D1 and the shadow stage conflicted | D1, D2: the step and its record always run, and each destructive action has a stage |
| M2: F1.2 said no sweep removes a leaf, and K5 duplicated the retention | F1.2, D8, K5 |
| M3: `archive` is a protected root | D8: the leaf lives in the salvage home |
| M4: no component changed the bundle writers | D9, X0 |
| M5: `refs/archive` had no end of life | D8, K5 |
| M6: D4 can hold a branch for ever | D6: the ship receipt and `EndedAfter` |
| M7: duplicate owners in the catalog | D10, the "Owner today" column, section 2.3 |
| M8: the crash and the resume paths | D1, K4, X4, X9 |
| M9: K8 removed a socket before K10 | K8, K10, X7, the order |
| M10: at-once removal lost the ignored files | K1 runs `gateLeftovers` |
| L1-L7 | K11 report-only, `origin/main` in the terms, D14, compare-and-delete in D5, X1 split, the project root in K1, `SnapshotSHA` in F1.3 |
| Docs MEDIUM: the item name was not in the tree | X6 names it as a runtime item, published at the next boundary |
| Docs LOW: F1.2 arithmetic, the shadow rule link, the host file, the proposed verbs | F1.2 mean size, D2 link, section 2.3 "host file", G2 and G3 "proposed verb" |

## 12. Review of round 2

The adversarial review `agent-adversarial-review-20261120T003622Z-d915` of round 2 verified five of the six round-1 HIGH fixes. It found three new HIGH paths, 5 MEDIUM and 4 LOW. The docs review `agent-docs-review-20261120T003625Z-478b` gave PASS with 1 MEDIUM and 3 LOW. Each fix below is small and local, so the console made it in session, and a fresh seat checks the patch.

| Finding | Fix |
|---|---|
| N1: the sealed sweep still judges "landed" against the hub `HEAD` and deletes with no bundle | Terms, D4, X1a: the target is the fetched `origin/main`, and each branch delete writes the leaf first |
| N2: K4 called a stale or unreadable lease a crash | D16, K4, X9: a dead owner pid, no holder and no running phase |
| N3: "at the seal" ran before the continuation stamp | D1, X4: the step runs after the epilogue and the final state write |
| M1: the size cap evicted unlanded leaves | K5: over the cap, only landed leaves go |
| M2: shadow for each check stopped the sweeps that enforce today | D2: an adapter keeps its current stage |
| M3: a ref and a bundle kept two copies | D5: the ref is the one copy, and K5 writes the bundle when it drops the ref. K12 shows the hub `.git` |
| M4: the legacy rewrite needs objects that can be pruned | K6: fetch the full bundle into a temporary ref first |
| M5: the agent-run workspaces had no owner | K13, X10 |
| Docs MEDIUM: two releasers of one cycle | D15 |
| L1-L4 and docs LOW | F1.6 names the delete of the run ladder. X1a names `gateLeftovers`. The terms name the ancestor check. F2.5 cites the lister |

The patch check `agent-adversarial-review-20261120T004321Z-0bc6` verified the fixes of N2 and N3. It found one more HIGH: the in-cycle teardown is a second site of the N1 path. The docs patch check `agent-docs-review-20261120T004324Z-61d5` gave PASS.

| Finding | Fix |
|---|---|
| HIGH-1: `gitWorktree.Cleanup` runs `branch -d` against the hub `HEAD` with no leaf, before the step | D15, X4: the in-cycle teardown releases nothing, and the step is the one releaser |
| MEDIUM-1: an absent lease | D16: held while the run dir is younger than the lease TTL |
| MEDIUM-2: the apply re-check still reads the hub `HEAD` | X1a retargets `deleteSquashed` and uses `update-ref -d` |
| MEDIUM-3: the step runs twice | D15: the step is idempotent on its record |
| LOW-1, LOW-2 | D1 cites the FAIL stamp and states the parked exit |

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| X0 thin bundles | done | cycle 2558, [`dad6bdf`](https://github.com/mickeyyaya/evolve-loop/commit/dad6bdf3482ca3b084aafe568be4da3a764578e8); `go/internal/gc/salvage.go` | none (consumed) |
| X1a check registry: worktrees | not started | none | `recycle-x1a-check-registry-worktrees` |
| X1b check registry: run dirs | not started | none | `recycle-x1b-check-registry-run-dirs` |
| X1c check registry: cache | not started | none | `recycle-x1c-check-registry-cache` |
| X1d check registry: temp | not started | none | `recycle-x1d-check-registry-temp` |
| X1e check registry: processes | not started | none | `recycle-x1e-check-registry-processes` |
| X2 trim counts executable dirs | not started | none | `recycle-x2-gocache-counts-executable-dirs` |
| X3 leaf writer and `evolve archive restore` | not started | none; no `archive restore` verb in `go/cmd/evolve` | `recycle-x3-archive-leaf-writer` |
| X4 mandatory recycle step | not started | none | `recycle-x4-mandatory-recycle-step` |
| X5 continuation reads the leaf | not started | none | `recycle-x5-continuation-reads-the-leaf` |
| X6 size cap, migration, retention | not started | none | `recycle-x6-salvage-retention-and-migration` |
| X7 temp shape registry | not started | none; `pipelineTempRe` is still in `go/internal/gc` | `recycle-x7-temp-shape-registry` |
| X8 test tmux socket reap | not started | none; `ReapOrphanSockets` predates this plan (`go/internal/swarm/reap_orphans.go`) | `recycle-x8-test-tmux-socket-reap` |
| X9 loop runs host checks | not started | none | `recycle-x9-loop-runs-host-checks` |
| X10 reports and integrity rows | not started | none | `recycle-x10-reports-and-integrity-rows` |

Only X0 is on main. The plan is still under design review, and the other components wait for it. Every unfinished component has an open inbox item, so no follow-up draft is needed.

## 13. Amendment 2026-11-22: the static gc flow

The operator asked for the missing half of this plan. Sections 1 to 12 free the resources of one cycle and bound the host caches. They do not name branches that landed, dev worktrees of the console, or a review at the end of a wave. This section adds X11, X12 and X13. It states which checks run at a cycle end and which at a wave end, and it gives the build order.

This section does not change the table "Delivery status (2026-11-21)". [Section 13.9](#139-rows-for-the-delivery-table) lists the new rows for it.

### 13.1 Request

On 2026-11-22 the operator wrote: "I want you to build a static gc flow that clean up cache, temp, merged branches and worktrees. It should be applied at cycle level and reviewed and invoked again when the wave is completed".

Three terms are new:

- **Static.** Each check is code with a reading, a threshold from policy and one outcome. No LLM judges a check.
- **Allow-list.** The list of items that a verb names as deletable, each with its rule and its evidence. A delete outside the list is a defect. The list is never "all items minus the kept ones".
- **Wave-end review.** One verb run at wave completion. It scans each check again and prints one verdict for the wave.

### 13.2 Facts

Each fact has a date. A fact marked "operator" is a report that this session cannot re-measure.

**F13.1** In the hub, the disk guard paused the start of a cycle 20 times in wave 186. Count: `grep -c HOST_DISK_LOW .evolve/signals.ndjson` gives 20. The loop log `.evolve/logs/20261121T192937Z/loop.log` has 1191 `[disk-guard] WARN` lines. Operator: free disk fell to 5 GiB of 460 GiB.

*Implication:* a leak between boundaries stops cycle starts. No check runs in that window.

**F13.2** The Go build cache is 44 GB on disk (`du -sh` of the `go-build` dir, 2026-11-22). Operator: `evolve gc --only cache` counted 20.9 GB against its 20 GB cap and trimmed 0.91 GB. The cause is the open item `recycle-x2-gocache-counts-executable-dirs` (filed 2026-11-20, not consumed), and `TrimGoCache` skips each directory entry (`go/internal/gc/gocache.go:88`).

*Implication:* the counter of the check and the measure of `du` disagree by more than 2 times. Nothing compares the two.

**F13.3** The `gc` step is the last step before the launch in the boundary chain (`go/cmd/evolve/cmd_boundary.go:239`). It runs the project sweep of `go/cmd/evolve/cmd_gc.go:225-262`. The sweep covers cycle worktrees and branches, dev worktrees, run dirs, Go cache, temp, processes and remote branches.

Between two boundaries only the batch start and end sweeps run, for run dirs and cycle worktrees. A wave can last days, so the cache and temp leaks grow for that time.

*Implication:* the cache trim does run at a boundary, so "only on an explicit run" is not exact. The defect is the gap between boundaries, and no step after the wave verifies the result.

**F13.4** The remote branch sweep exists and never deletes. `PlanRemoteSweep` keeps a branch unless a merged pull request has the same tip (`go/internal/gc/remotesweep.go:116-128`).

`RemoteStage` is `shadow` when `gc.remote_sweep.stage` is absent (`go/internal/gcpolicy/gcpolicy.go:171-176`), and the runtime policy has no such key. Only `main` and `master` count as the default branch (`go/internal/gc/branchsweep.go:23`). The lookup asks for 1000 pull requests (`go/internal/gc/remotegh.go:17`). On 2026-11-22 `gh pr list --state all --limit 1000` returned exactly 1000 (636 MERGED, 326 CLOSED, 38 OPEN).

*Implication:* the lookup is full, so older pull requests are invisible. A closed pull request is not a reason to delete today. The sweep never ran at the enforce stage.

**F13.5** The local branch sweep exists. `evolve gc` sets `BranchScopeAll` (`go/cmd/evolve/cmd_gc.go:237`). It deletes a non-cycle branch that landed, or that has no commit for 14 days (`dev_worktrees.archive_after_days`), after a verified bundle (`go/internal/gc/branchsweep.go:53-76`).

The repository has 130 local branches outside `origin/main` and 24 inside. Remote branches by prefix: console 66, integration 20, docs 13, inbox 6, policy 3, fix 2, chore 2, protect-proof 1.

*Implication:* the sweeps are right in kind. They lack the closed state, the head-in-main rule for a remote branch, a data-owned list of protected branches and one verb that prints the allow-list.

**F13.6** The runtime inbox holds 13 open items `gc-flag-unmerged-cycle-*` that flag cycle branches (2026-11-20 to 2026-11-21).

On 2026-11-22 each of the 13 branches is local with one commit that `origin/main` lacks. None has a worktree or a remote branch. The last 1000 pull requests have no head named `cycle-*`. Three flags say "unmerged orphan branch (no worktree)" and ten say "branch not merged into HEAD".

*Implication:* these are failed or undelivered cycles, not merged branches. The hub `HEAD` judges ten of them (decision D4 of this plan). Deleting them as "merged" is wrong. Their end belongs to K2 and decision D5.

**F13.7** The dev sweep removes a console worktree in two cases. One: the tree is idle for 14 days. Two: it "landed early". That means a clean tree with own work and no open pull request, and a tip that `origin/main` holds (`go/internal/gc/devlanded.go:14-36`, `go/internal/gc/devworktrees.go:280-300`). A closed pull request that did not merge is not a case.

A dirty tree is kept with no list of its changed paths. Code protects only the project root and the plane worktrees (`go/internal/gc/devworktrees.go:219-227`). The hub has 94 worktrees on 2026-11-22.

Operator: the dev dirs held 22 GB in 119 directories. The console removed 35 clean ones by hand (6 GB). 39 dirty ones hold uncommitted source.

*Implication:* the closed state, the dirty report and a protected list in data are missing. This session did not find which gate kept the 35 trees. Step 1 of the build order (section 13.8) prints the keep reason of each tree.

**F13.8** The 2026-11-19 branch cleanup left a restore path. The hub file `backups/branch-cleanup-2026-11-19.tsv` has 344 rows with the columns `side`, `branch`, `tip`, `pr` and `class`.

The hub has 26 refs under `refs/archive/salvage/`. GitHub keeps `refs/pull/<n>/head` for each pull request.

*Implication:* a deleted branch with a pull request is recoverable from its tip. A branch with no pull request needs a ref or a bundle that holds the objects. A list of tips alone does not.

**F13.9** The check registry of X1a has one check, `worktrees` (`go/internal/gc/check.go:36-38`). `evolve gc check` prints items with `subject`, `action`, `rule` and `evidence`. It has no reading, no threshold and no verdict.

`evolve wave` has no `checkin` verb. The open item `wave-checkin-verb-runs-the-static-sop` (2026-11-21, route `console-manual`) adds it. The SOP file `docs/operations/console-checkin-sop.md` is not in `origin/main` at `7fe823e5a`.

*Implication:* X13 supplies the gc readings. `wave checkin` reads them and does not recompute them.

### 13.3 Goals and non-goals

| Id | Goal | Measure |
|---|---|---|
| G6 | Each cycle end releases the temp, process and worktree resources of that cycle, and trims the shared cache when the disk is low | cycle ends with a leftover of its own resources: 0 |
| G7 | Each wave end scans each check again and prints one verdict | waves with no recorded review: 0 |
| G8 | A branch of the pipeline or the console that landed or closed is deleted from an allow-list, local and remote | landed or closed owned branches older than the grace: 0 |
| G9 | A console dev worktree that is clean and landed or closed is removed. A dirty one is reported | clean landed dev trees: 0. Dirty trees removed by X12: 0 |
| G10 | The cache counter and `du` agree | difference under the policy tolerance |

Non-goals:

- A change of the rules of K1 to K13. X11 to X13 add checks and one verdict. They do not weaken a rule.
- A branch that the pipeline and the console did not create. Its prefix is not in the owned list.
- Any `gh` write by a seat. Only `evolve gc`, run by the console or the boundary, deletes a remote branch.
- A brake on the wave. A FAIL verdict never blocks `wave next`, except the disk floor that the preflight already enforces (`preflight.min_free_gib`).

### 13.4 Candidate designs

- **A. Widen the existing sweeps.** Add the closed state to the remote and dev sweeps, add a loop step between boundaries, and print nothing new.
- **B. One flow on the registry.** The registry (X1a) holds the new checks. X11 and X12 are adapters over the existing sweeps, and the release of X4 is `RunCycleChecks`. A cycle-end set and a wave-end set are policy data. X13 scans again and gives a verdict.
- **C. A scheduler outside the loop.** A launchd or cron job runs `evolve gc` each hour and the console reads the log.

| Criterion | A | B | C |
|---|---|---|---|
| Runs at the cycle end and the wave end, as the operator asked | 1: boundary only | 3 | 1: a clock, not an event |
| Each decision has a reading, a threshold and a verdict (static) | 1 | 3 | 1 |
| Reuses the sweeps that exist | 3 | 2 | 3 |
| Catches the counter and `du` gap of F13.2 | 1 | 3: the review compares both | 1 |
| Consistent with the rules "no scratch scripts" and "no env control values" | 3 | 3 | 1: a job outside the verbs |
| Size of the change | 3 | 2 | 2 |
| **Total** | 12 | **16** | 9 |

B wins. A leaves F13.3 open, because a boundary sweep has no later check. C runs on time and not on events, and it is a script the operator rules out. B costs three components and one new set of policy keys.

Three designs for the allow-list of X11:

| Criterion | L1. Pure judge over the facts | L2. A stored list that the apply reads | L3. A plan and an apply that re-plan |
|---|---|---|---|
| A branch that moved after the plan is safe | 3 | 1: the stored tip is stale | 3: apply re-checks the tip |
| The operator can read the list before the apply | 1 | 3 | 3: the plan verb prints it |
| Follows "a stored manifest is never re-applied" ([internal-gc](../architecture/packages/internal-gc.md), "Apply is not a replay") | 3 | 1 | 3 |
| **Total** | 7 | 5 | **9** |

L3 wins and matches `PlanRemoteSweep` and `ApplyRemoteSweep`.

### 13.5 Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D17 | A branch is owned when its name starts with a prefix in `gc.branches.owned_prefixes` (data in `policy.json`). The start value lists `console/`, `policy/`, `docs/`, `inbox/`, `integration/`, `fix/` and `chore/`. The prefix `cycle-` is not in the list: K2 alone owns a cycle branch. The default branch is read from `refs/remotes/origin/HEAD` and is never owned. | F13.4, F13.5. The code holds `main` and `master` in a map (`branchsweep.go:23`). K2 and decision D5 hold the cycle branches. | A hard-coded prefix list: it changes with a release. "All branches except main": that is "all minus keeps". `cycle-` in the list: two owners write two archive layouts for one cycle. |
| D18 | X11 is one registry check, `branches`. It replaces `otherBranchItems` (`go/internal/gc/branchsweep.go:25-50`), limits its idle rule (`:66-75`) to owned prefixes, and extends `PlanRemoteSweep` and `ApplyRemoteSweep`. `evolve gc check --only branches` prints the allow-list. X11 adds `--only <check>` to `gc check`, with the names from `DefaultChecks()`, and `evolve gc --only` keeps its sweep scopes. An item has side, branch, tip, pull request, class and evidence. `evolve gc` deletes an item only when a fresh plan has the same item and tip. | F13.4, F13.5. L3 of section 13.4. D10 (one adapter for each resource). | A second verb `evolve gc branches plan`: two report verbs list branch items. A stored list: its tips go stale. |
| D19 | Two classes delete on landing. B1: a merged pull request has this tip. B2: the tip is in `origin/main` (ancestor, or the squash proof of `branchLandedBySquash`), and the branch has own work. The proof for a branch is the oldest entry of `git reflog show --format=%H refs/heads/<b>`: it differs from the tip. B2 does not apply to a remote-only branch. | The request, F13.4. A new branch at the main tip has no own work, so it is not landed. A branch that someone fast-forwarded to main also passes the proof, and that delete is harmless. An expired reflog fails closed. | B2 on ancestry alone: it deletes a branch that no one used yet. |
| D19b | B3 deletes a closed branch. A closed pull request did not merge, and its `headRefOid` equals the tip. No open pull request has the branch. Its `closedAt` is older than `gc.branches.closed_grace_days` (start value 3, D34). B3 also needs the record of D21. Each other case keeps the branch and names the reason. | A branch that moved past its closed pull request holds unreviewed work. A person can reopen a pull request in the grace window. | Delete at once on CLOSED: it removes a branch that a person can reopen. |
| D20 | Keep a branch that has one of seven marks: default branch, open pull request, checked out, cycle held by `sealHold`, ruleset, unowned prefix, moved tip. The `sealHold` mark is defence in depth, because D17 keeps `cycle-*`. A ruleset is a GitHub repository ruleset that protects the branch (`RulesetProtects` in `remotegh.go`). The open lookup is its own call with `--state open`. A lookup that returns exactly its limit fails closed, and X11 then asks for each candidate branch (`gh pr list --head <branch> --state all`). | F13.4 (1000 of 1000). A live cycle or worktree is never a target. | One lookup for all states: a full list can hide an open pull request. |
| D21 | Before any branch delete, local or remote, the apply writes one record by class. B1 and B2: one tsv row, because `origin/main` or `refs/pull/<n>/head` holds the objects. Add a thin bundle when `rev-list --count origin/main..<tip>` is above 0 (D9). B3: the ref `refs/archive/branches/<stamp>/<branch>`, with no bundle (D5). The delete runs after the record verifies. | F13.8. D5 rejects a ref and a bundle together, which are two copies. A stamp stops a name collision. | A ref, a bundle and a tsv row for each delete: three copies. |
| D21b | The tsv file is `<salvage home>/branch-gc/<stamp>.tsv`, with the columns of 2026-11-19: `side`, `branch`, `tip`, `pr`, `class`. Restore with `git branch <branch> <tip>` or `git push origin <tip>:refs/heads/<branch>`. K5 gets a retention row for `refs/archive/branches/` and `branch-gc/`. The age source is the stamp. The bundle home is `branch-gc/<stamp>/`, so that K5 can judge a ref that has no leaf. The hand-made refs of `refs/archive/salvage/` stay apart. | F13.8. K5 must tell the two kinds of ref apart. Round 1 finding M5 asked for an end of life for each archive ref. | A tsv alone: for B3 it holds no objects. |
| D22 | Only `evolve gc` deletes a branch, and only the console (by hand) or the boundary `gc` step starts it. A seat never does. The remote delete uses the existing `GhRemoteAPI.DeleteBranch`. A failed remote delete is a WARN of the `gc` step, and the exit code stays. | the operator rule of 2026-11-17 (the console runs `gh`, a seat never does) | Let a lane delete a remote branch: a lane has no `gh` write. |
| D22b | The stage of the remote delete is `gc.remote_sweep.stage`. The stage of the local delete of X11 and of X12 is `gc.checks.branches.stage` and `gc.checks.dev_worktrees.stage` (decision D2). Each starts at shadow. It goes to enforce on a verified shadow record of [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md). The review of D27b reads the stage key of its check. | the shadow stage contract. The enforce path must be built and wired before the flip. | An early flip: the enforce path has to exist first. |
| D23 | The 13 `gc-flag-unmerged` items fold in by a per-branch rule, not by a bulk delete, and K2 decides each one, because all 13 are `cycle-*`. A held cycle is kept, and its flag stays. K2 deletes a cycle that D4 calls landed, after its leaf, and the flag clears. An unlanded, unheld cycle: K2 moves it to `refs/archive/cycles/` by D5 after X3, X5 and X4, and the flag clears. X11 deletes none of them, and X4 tests the fold-in. | F13.6. D4, D5 and D6 own the cycle branches. | Delete the 13 as merged: none has a pull request, and each has a commit that main lacks. Let X11 delete them: a second owner. |
| D24 | X12 adds the closed state to the dev sweep. Remove a dev tree that is clean, has no process inside, and is not protected. Its branch is merged or closed, or its head is in `origin/main`. The branch stays and X11 decides its end. The removal runs the existing checkpoint save first (decision D3 of [internal-gc](../architecture/packages/internal-gc.md)). The dev archive file gets the record. | F13.7 | Remove a tree on a merged pull request alone: it can hold new uncommitted files. |
| D25 | A dirty dev tree is reported with its changed paths and is never removed by X12. The existing idle archive (14 days, after a verified checkpoint) is not changed by X12. The operator decided on 2026-11-22 to keep that archive (D32) | F13.7 | Remove a dirty tree after a checkpoint on a merged pull request: it removes the working copy of 39 trees that hold source |
| D26 | The protected dev trees are data: `gc.dev_worktrees.protected` is a list of path globs relative to the dev dir, for example `code-index`. The project root and plane worktrees stay protected by code, because they are structural. A protected tree is kept with the reason "protected by policy: <glob>" | F13.7. The code index lives in `dev/code-index` | A longer code list: the next name needs a release |
| D27 | X13 adds the verb `evolve gc review [--wave N] [--json]`. It scans each registered check. It reads the threshold of the check itself: the cap, the TTL or the disk level. `gc.review.checks.<check>` holds only what the review owns: the action, the retry permission and a tolerance. It prints one row for each check: reading, threshold, outcome, one action. | The request, F13.3, F13.9. A second copy of a cap diverges when an operator changes the cap. | A review threshold block that copies the caps: two homes for one belief. An LLM summary: a summary is not a threshold. |
| D27b | The outcome is PASS, FAIL or SHADOW. A check in the shadow stage reports its planned item count and the outcome SHADOW, which does not fail the wave. The action is `none` for PASS and SHADOW. For FAIL it is `retry-apply`, `file-item` or `console-route`. The wave verdict is FAIL when one row fails. The verb records `.evolve/gc-reviews/wave-<N>.json` and appends `GC_REVIEW_PASS` or `GC_REVIEW_FAIL`. | A check in shadow leaves items by design, so a FAIL from it is noise. A static outcome needs a recorded verdict. | FAIL for a shadow check: each wave fails until the enforce flip. |
| D28 | `retry-apply` is the second invocation of gc. It is allowed only for a check with `retry: true` in `gc.review.checks.<check>` (the start value covers cache, temp and processes). The review calls the same stage-aware apply as `evolve gc`, once, reads the row again, and records both readings. A check without the permission gets `console-route`, so D22 holds. | The request: "invoked again when the wave is completed". D22: the destructive paths of branches and worktrees keep one start path. | An action that no code runs: it is a label. An unbounded retry: it can loop on a full disk. A retry for each check: a third start path for a branch delete. |
| D29 | The disk row reads free bytes of the volume of `.evolve` and of the temp dir. It uses the classifier that X13 extracts from `diskSignal` (`go/cmd/evolve/cmd_gc_disk.go:23`), so the report level and the review agree. The floor that stops a launch stays `preflight.min_free_gib`. | F13.1. `gc.disk_floor_gb` and `preflight.min_free_gib` are two floors (decision D35). | A second classifier in the review: two readers of one belief. |
| D29b | The cache row uses one sizer function, `allocatedBytes`. It sums `st_blocks`, counts a hard link once, and includes each `<hash>-d` dir. The trim counter and the review both call it, so the two have one unit. A second row compares two readings: the sum over the entries that the trim walks, and `allocatedBytes` of the cache root. It fails when they differ by more than the tolerance in `gc.review.checks.cache`. It catches a skipped entry kind, such as the `-d` dirs of F13.2. | F13.2. The case of 44 GB against 20.9 GB fails this row. Apparent size and allocated size differ for many small files. | `du` against apparent sizes: the row fails each wave. `gc` output against itself: it repeats the defect. |
| D30 | The review is a boundary step after the `gc` step and before the launch. It is `advisory: true`, the form of `cmd_boundary.go:237-238`. It exits 0 on any verdict and on a read error. It exits non-zero only for a usage error. | F13.3. The rule "the boundary never blocks the wave". The boundary stops at a non-advisory step with a non-zero exit. | A failing step: it blocks the launch. A step before `gc`: it reads the state that gc is about to change. |
| D30b | The wave number is the last wave of the wave history (`cmd_wave_compose.go:105`), and `--wave N` overrides it. `evolve wave checkin` and the console check-in SOP read the last record. They do not recompute it. A FAIL verdict files at most one inbox item for each check and wave, with the evidence. | The boundary has no wave number today (`cmd_boundary.go:224-240`). | A new flag on the boundary: the history already holds the number. |
| D31 | A check that has no review row fails a registry guard test. X13 writes the rows of each check on main when it builds (X1a to X1e, X2, X11 and X12). Each later check adds its row in its own pull request. | G7. | A review that lists only the checks of today: a new check then hides from it. |
| D32 | The idle archive of dirty dev trees stays as it is: 14 days idle, after a verified checkpoint. A fresher dirty tree is only reported. | The operator decision of 2026-11-22 (O5). The checkpoint keeps the work (ADR-0122). | Stop the archive for a dirty tree: 39 dirty trees then stay for ever. |
| D33 | The run dir is wave-scoped. The cycle end records its size and deletes nothing. | The operator decision of 2026-11-22 (O6). D7. | A delete at the cycle end: the closeout reads the dir after the seal. |
| D34 | `gc.branches.closed_grace_days` starts at 3 days. | The operator decision of 2026-11-22 (O7). | 7 days: a closed branch then waits longer than a person needs to reopen it. |
| D35 | The disk floors merge into one home. This is a separate item, `disk-floor-one-home`, and it is not part of the gc flow build. X13 reads the one classifier until the item lands. | The operator decision of 2026-11-22 (O8). Three homes hold a floor: `gc.disk_floor_gb` (`gcpolicy.go:18`), `preflight.min_free_gib` (`go/internal/policy/preflight_config.go:16`) and the 5% guard of F1.1. | Keep three floors: they disagree in units and in levels. |

### 13.6 The flow

Two scopes, one registry. The recycle step of X4 (decision D1) runs the cycle-scoped checks for the cycle's own targets. The boundary `gc` step runs the wave-scoped checks. The review then scans all checks again.

| Check | Scope | Runs at the cycle end (X4) | Runs at the wave end (boundary `gc`, then the review) |
|---|---|---|---|
| K1 cycle worktree | cycle | yes, for this cycle | the review scans all cycles. K4 recycles a cycle that has no record |
| K2 cycle branch | cycle | yes, for this cycle | the review scans all cycles |
| K3 processes and sessions | cycle | yes, for this cycle | the review scans the host |
| K8 and K9 temp | cycle for a shape with an owner tag, host for the others | only entries with the cycle id in their name or owner file (X7) | all registered shapes, with the horizon |
| K7 Go cache | host | only when free disk is under `gc.cache_trim_free_gb`. The trim keeps its holder probe | the full trim, then the du reading of D29 |
| Run dirs (K1b) | host | no delete. The recycle record stores the size of the run dir | the run ladder (decision D7) |
| K5, K6 salvage | host | no | yes |
| K10 test servers, K13 workspaces | host | no | yes |
| X11 branches (not `cycle-*`, which K2 owns) | host | no | yes (plan in shadow, apply at enforce) |
| X12 dev worktrees | host | no | yes |

The Go cache is not cycle-scoped, because the cache key follows the worktree path (R2). No entry belongs to one cycle. A trim by one lane must not remove an entry that a sibling lane has open, so the holder probe bounds it. The run dir is not cycle-scoped either. The closeout and the continuation read it after the seal (F1.6).

**Worked example (illustrative values, not a measurement).** The boundary `gc` step of wave 186 ends. Then `evolve gc review --wave 186` reads three rows.

- Free disk is 31 GiB against a warn level of 50 GiB: FAIL, action `console-route`. No gc check owns the disk.
- The cache root is 22 GB against a trim cap of 20 GB: FAIL, action `retry-apply`. The trim walk sums 14 GB and the root sums 22 GB, a gap over the tolerance of 10%: FAIL, action `file-item`.
- Temp entries older than the horizon number 0: PASS.

The verb records `wave-186.json` and appends `GC_REVIEW_FAIL`. `wave next` still starts wave 187, because the free disk is above the floor.

### 13.7 Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| X11 | The registry check `branches`. It replaces `otherBranchItems`, extends the remote sweep, and applies D17 to D21b. | see the red tests below | lane |
| X12 | The dev sweep gets the closed state, the changed paths of a dirty tree, and `gc.dev_worktrees.protected` | see the red tests below | lane |
| X13 | `evolve gc review`, the `gc.review.checks` policy block, the `allocatedBytes` sizer, the disk classifier, the boundary step and the record | see the red tests below | lane (verb and policy). The SOP text is console |
| X4b | A scope on each registry check (`cycle` or `host`), the cycle-end mode of K7 and K8, and one call `RunCycleChecks` | see the red tests below | lane for the registry. The call in the X4 step is console, because `go/internal/core/cyclerun.go` is protected |

Red tests, first:

- **X11.** A fixture hub has a merged, a closed, an open, a checked-out, an unowned, a `cycle-*` and an empty new branch. `evolve gc check --only branches` plans a delete for the merged branch, and for the closed branch past grace with the same tip. It keeps each other branch.
- **X11.** A branch that a ruleset protects stays. A closed pull request whose branch moved is kept. A branch that moves after the plan stays. The default branch that `origin/HEAD` names stays, even if it is not `main`.
- **X11.** The printed command restores a deleted branch. A full lookup of 1000 pull requests falls back to one lookup for each candidate. A failed remote delete is a WARN, and the `gc` exit code stays.
- **X12.** The dry run prints the keep reason of each tree. A clean tree with a merged or closed branch is removed, and its branch stays. A dirty tree is kept, and the report lists its changed paths.
- **X12.** A tree that matches a protected glob stays, and the reason names the glob. A process inside keeps the tree.
- **X13.** A fixture with a cache of 44 GB allocated and 20.9 GB counted fails the cache row. A fixture of many files under 4 KiB passes it. A free disk under the warn level fails the disk row.
- **X13.** A FAIL row runs its apply once more and records both readings. The record and the signal exist after the run. A FAIL verdict and an unreadable disk both reach the launch step.
- **X6.** A branch-gc stamp dir and its `refs/archive/branches/` ref past their age go together. A younger one stays.
- **X13.** A check in the shadow stage with planned items gets SHADOW and does not fail the wave. A check without `retry: true` gets `console-route`. The wave number at boundary time is the wave that ends.
- **X13.** A check with no review row fails the guard test. The boundary runs the step once, after `gc`.
- **X4b.** A cycle end under `cache_trim_free_gb` trims the cache, and an entry that a sibling lane holds stays. An entry with the id of another cycle stays. A host-scoped check does not run at a cycle end.

Each component gets one end-to-end test through the real verb, and a changed-function coverage of 100%. The X4 step calls `RunCycleChecks` of X4b, so X4 depends on X4b. The scope table of X4b joins the integrity rows of X10 (D14). A lane must not edit the set that a cycle end deletes. A policy key lands in the same pull request as its reader. The boundary stops on a key that no reader knows ([wave-operations](../operations/wave-operations.md), the build step).

### 13.8 Order, routes and wave sets

Dependencies. X11 and X12 need X1a. X13 needs X1a to X1e, X2, X11 and X12. Its cache reading needs a correct counter, and it writes the review rows of the checks that exist (D31).

X4b needs X1a, X1c, X1d, X1e, X2 and X7, and X4 needs X4b. Build step 0 files the items of X11, X12, X13 and X4b from the drafts, and adds X4b to the deps of the item X4. Build step 1 runs `evolve gc --dry-run` on the hub and prints the keep reason of each dev tree (F13.7). Each later step is a wave of the table below. The 13 flagged cycle branches (D23) need X3, X5 and X4 for the unlanded ones. Until then those branches stay flagged.

| Wave | Items | Route | Why this set |
|---|---|---|---|
| A (unblocked now) | X2, X1b, X1d, X1e, X8 | lane | their dependencies (X1a, X0) are on main |
| B | X1c, X3, X11, X12 | lane | X1c needs X2. X3 needs X0. X11 and X12 need X1a. They wait only to keep the lanes of wave A to five items |
| C | X13, X6, X7 | lane | X13 needs X1a to X1e, X2, X11 and X12. X6 needs X3 and keeps the refs that X11 writes. X7 needs X8 and X1d |
| D | X5, X4b | console for X5 (protected files), lane for X4b | X5 needs X3. X4b needs X1a, X1c, X1d, X1e, X2 and X7 |
| E | X4 in shadow | console | X4 needs X5, X4b and X1a |
| F | X9, X10, then X4 to enforce | lane for X9, console for X10 and the flip | X9 starts when X4 is on main in shadow. The enforce flip needs a verified shadow record. X10 touches the integrity surface |
| G | `gc.remote_sweep.stage` to enforce | console | needs X11 and one verified shadow record of the allow-list |

A wave can hold more than one set when the lanes allow it. Items that edit the same file, such as `cmd_gc.go` and `gocache.go`, rebase on each other, and the console merges them in a line. X11 and X12 matter most for the disk, so the console can move them into wave A when a lane is free.

The order of section 7 holds. The wave sets above place X11, X12, X13 and X4b in it. Section 6 lists the components of sections 1 to 12, and section 13.7 lists X11, X12, X13 and X4b.

### 13.9 Rows for the delivery table

The table "Delivery status (2026-11-21)" is on main. The console adds these rows to it after it files the open items of X11, X12, X13 and X4b.

| Id | Component | Depends on | Route | Wave set |
|---|---|---|---|---|
| X11 | Merged and closed branches deleted from an allow-list | X1a | lane | B |
| X12 | Console dev worktrees: closed state, dirty report, protected by data | X1a | lane | B |
| X13 | Wave-end review: one verdict from every check | X1a, X1b, X1c, X1d, X1e, X2, X11, X12 | lane | C |
| X4b | Scope of each check and the cycle-end subset (cache, temp, processes) | X1a, X1c, X1d, X1e, X2, X7 | lane, with a console call in X4 | D |
| X4 (change) | X4 calls `RunCycleChecks`, so X4 also depends on X4b | X4b | console | E |

### 13.10 Verification

| Measure | Before (2026-11-22) | Target |
|---|---|---|
| Cache counter against `du` | 20.9 GB against 44 GB | within the tolerance, or the review fails |
| Landed or closed owned branches older than the grace | not measured. The plan of `evolve gc check --only branches` in shadow gives the count. Upper bound on 2026-11-22: 130 local branches outside `origin/main` | 0 after the first enforce wave |
| Clean landed dev trees | 35 removed by hand | 0 left by the next boundary |
| Dirty dev trees removed by X12 | 0 | 0, and each listed with its paths |
| Waves with a recorded review | 0 | each wave |
| Disk guard pauses in a wave | 20 (wave 186) | 0 for 7 days |

### 13.11 Risks

| Risk | Control |
|---|---|
| A delete removes a branch that a person still needs | the allow-list rules of D19 and D20, the record of D21, the grace of B3, the stage of D22 |
| A full pull request lookup hides an open pull request | D20: a separate open lookup, and a full list falls back to one lookup for each candidate |
| X11 and K2 delete the same branch | D17 and D23: `cycle-*` is owned by K2 alone |
| A new empty branch counts as landed | D19: B2 needs own work |
| The review stops the wave | D30: the step is advisory and exits 0 |
| A list of owned prefixes owns a personal branch | the list is data, the shadow stage prints it first, and an unowned prefix is kept |
| A cache trim at a cycle end removes a file of a sibling lane | the holder probe of D12 and the free-disk threshold |
| The review hides a failure in a "none" action | each FAIL row has an action other than `none`, and a test proves it. `retry-apply` runs once (D28) |
| A review step slows the boundary | measure its time in the signal. The scan reads, and one bounded retry runs only for a check that allows it |

### 13.12 Open questions

O1 to O4 are in section 10. The operator answered O5 to O8 on 2026-11-22. They are decisions D32 to D35, so section 13 has no open question.

The item `disk-floor-one-home` (draft) carries D35. The delivery table does not hold it, because it is not part of the gc flow.

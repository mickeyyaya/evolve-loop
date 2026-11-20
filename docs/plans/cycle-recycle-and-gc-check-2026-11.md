# Plan: each cycle ends with a mandatory recycle step, and gc checks each host resource it owns

- **Status:** proposed, 2026-11-20. Round 2 with the in-session fixes of its review (sections 11 and 12 list each finding and its fix). The design review comes before any build.
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
| X4 | The recycle step after the epilogue, in the seal path and the resume path (D1, D3 to D6, D15). Its new destructive actions start in shadow (D2) | a PASS cycle, a FAIL cycle, a resumed cycle, an abnormal exit and a deadline exit each end with a verified leaf and a record. The deadline case keeps its continuation. A PASS cycle whose branch is merged only into the hub `HEAD` keeps its branch. A second call of the step does nothing. The shadow stage deletes nothing | console: `go/internal/core/cyclerun.go` is protected (`integrity_surface.go:103`) |
| X6 | K5 size cap, K6 legacy migration, and the retention of `.evolve/archive/runs` (D7, D8). It replaces the runtime inbox item `gc-salvage-bundles-hold-the-whole-history-with-no-retention`, filed on 2026-11-20 and published at the next boundary | a full bundle becomes a thin bundle that verifies. A leaf with untracked files stays | lane |
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

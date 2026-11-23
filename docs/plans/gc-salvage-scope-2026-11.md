# Plan: `evolve gc --only salvage` re-packs full-history salvage bundles with no loss

- **Status:** proposed, 2026-11-23.
- **Design:** [internal-gc](../architecture/packages/internal-gc.md). **Parent plan:** [cycle recycle and gc check](cycle-recycle-and-gc-check-2026-11.md), checks K5 and K6 and component X6.
- **Evidence:** the survey of section 2, run read-only on the runtime hub on 2026-11-23.

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

## Terms used throughout

- **Leaf.** One directory under `.evolve/operator-salvage`. It holds `HEAD` and, for most leaves, `refs.bundle`.
- **Full bundle.** A bundle that records a complete history. It holds every commit of the repository.
- **Thin bundle.** A bundle that holds only the commits that `origin/main` does not hold. It has `^origin/main` as its basis.
- **Basis.** The commit of `refs/remotes/origin/main` that a thin bundle excludes.
- **Held.** A leaf whose cycle is live, paused, or bound to a continuation.

## 1. Request

The operator decided on 2026-11-22: "build this verb (lossless) now at the wave boundary". The inbox item states the need. `evolve gc --only salvage` re-packs each full-history bundle against `origin/main`. It removes a bundle that `origin/main` already holds. It never touches a running cycle. It writes one audit row per action.

The salvage writer must also make incremental bundles.

In wave 187 the disk guard paused dispatch at 3% free, and 3 of 5 lanes never ran a cycle.

## 2. Facts

**F2.1** The salvage home holds 269 leaves, from 2026-10-26 to 2026-11-22. The prefixes are `cycle-` (119), `branch-` (85) and `cl-` (65). The inbox item names only `cycle-<run>-<N>`. Source: `ls .evolve/operator-salvage`, 2026-11-23.
*Implication:* the scope must treat each leaf by its files, not by its name.

**F2.2** 182 leaves hold a `refs.bundle`. They hold 161.8 GB (150.7 GiB). 87 leaves hold no bundle. Source: a read-only loop of `stat` over the 269 leaves, 2026-11-23.
*Implication:* a leaf with no bundle is out of scope. The scope lists only leaves with a bundle.

**F2.3** `git bundle verify` reports "records a complete history" for 144 bundles, and they hold 161.77 GB of the 161.8 GB. The other 38 bundles are already thin. No bundle has more than one head. Source: the same loop.
*Implication:* the thin rewrite frees nearly the whole 150 GiB, and the multi-head case is rare.

**F2.4** Every head of the 182 bundles is a commit that the runtime repository holds (`git cat-file -e <sha>^{commit}`). 4 bundles (4.6 GB) have all heads reachable from `origin/main`. Source: the same loop.
*Implication:* the repository can build each thin bundle, and 4 bundles need no replacement.

**F2.5** The example `cycle-cd3ae73e-2532/refs.bundle` is 1,092 MiB, has one head `refs/heads/cycle-cd3ae73e-2532` at `32078de1b`, and records a complete history. Source: `git bundle list-heads` and `git bundle verify`.

**F2.6** The leaf writer is thin already. `bundleVerified` adds `^refs/remotes/origin/main` when that ref exists, and writes no bundle when the tip is in `origin/main` (`go/internal/gc/salvage.go:85-104`). Commit `dad6bdf34` of 2026-11-20 added it, and `go/internal/gc/salvage_thin_bundle_repro_test.go` pins it. `writeSalvageLeaf` (`go/internal/gc/salvage.go:48`) and `bundleBranch` (`go/internal/gc/branchsweep.go:124`) both call `bundleVerified`.
*Implication:* the 144 full bundles are old output. The writer fix is not needed there.

**F2.7** One writer is still full. `bundleRef` runs `bundle create <file> <ref>` with no basis (`go/internal/gc/remotesweep.go:195`). It saves the tip of a remote branch before the branch is deleted.
*Implication:* the writer fix is this one call. It gives a leaf that is thin when `origin/main` exists.

**F2.8** `evolve gc` has one scope today, `cache` (`go/cmd/evolve/cmd_gc.go:35`, `:41`, `go/cmd/evolve/cmd_gc.go:117-119`). A non-dry run takes the gc lease (`go/cmd/evolve/cmd_gc.go:50-57`). `--dry-run` swaps each mutating function for a no-op (`go/cmd/evolve/cmd_gc.go:161-164`).
*Implication:* the new scope follows the same switch. It needs its own dry-run proof, because it writes files.

**F2.9** gc detects a running cycle in `isLive` (`go/internal/gc/worktrees.go:219`). It reads the active worktree of `cycle-state.json` and of each `run.json`, and the run lease (`runLive`, `:238`). `sealHold` adds the paused checkpoint and the continuation binding (`go/internal/gc/cycleseal.go:61-74`).

Only a `cycle-<run>-<N>` leaf has a cycle that can run. A `branch-` leaf comes from a branch sweep, and a `cl-` leaf comes from a dev worktree archive. Neither has a running cycle.
*Implication:* the scope applies `isLive` and `sealHold` to the leaf name, with the worktree path `<WorktreeBase>/<leaf>`. For each other leaf, the settle window and the recheck of D5 protect it.

**F2.13** The sealed-cycle sweep writes `cycle-` leaves under the ship lock only (`go/internal/gc/cycleseal.go:47`, `go/internal/gc/salvage.go:48`). The loop runs it at each cycle seal, outside `evolve gc`. The review of round 1 found this.
*Implication:* the gc lease does not exclude that writer. The scope takes the ship lock for each final step.

**F2.12** The default gc deletes each leaf older than `salvage_ttl_days`, 30 days by default, judged by the mtime of the leaf directory (`go/internal/gc/gc.go:79`, `go/internal/gcpolicy/gcpolicy.go:53`). The oldest leaves (2026-10-26) reach 30 days on 2026-11-25. A rename into a leaf sets its directory mtime to now.

*Implication:* a repack must not change the age that the TTL sees. The scope restores the mtime of the leaf directory after each change. The TTL can delete unique work before this scope runs. That risk is not new, and K5 of the parent plan owns it.

**F2.10** The audit home of gc is `.evolve/archive/`. `AppendDevArchive` appends one JSON line per action to `.evolve/archive/dev-worktrees.jsonl` (`go/internal/gc/devarchive.go:32`). The discover step protects `archive` (`go/internal/gc/gc.go:204`). gc writes no other per-action log.
*Implication:* the new rows go to `.evolve/archive/salvage-repack.jsonl`, in the same shape.

**F2.11** The parent plan already specifies the legacy migration. Check K6 reads: "fetch the full bundle into a temporary ref, write a thin bundle, verify `list-heads`, replace, and drop the temporary ref. Keep the full leaf on each failure" ([plan](cycle-recycle-and-gc-check-2026-11.md), section 5). Component X6 names the red test: "a full bundle becomes a thin bundle that verifies".
*Implication:* this plan builds the K6 migration as a manual scope, ahead of the registry.

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1 Free the disk | bytes of `.evolve/operator-salvage` | under 20 GiB after one run, from 150.7 GiB |
| G2 Lose nothing | the heads of each bundle, one for one | the new bundle lists the same `sha ref` lines as the old bundle |
| G3 Spare a live cycle | leaves that a live or held cycle owns | 0 changed |
| G4 Prove before apply | `--dry-run` | 0 bytes changed in the salvage home |
| G5 New bundles stay thin | size of a leaf one commit past `origin/main` | under 1 MiB |

Non-goals:

- This plan does not delete a leaf, a patch or an untracked archive.
- This plan does not set a retention age. The parent plan owns K5.
- This plan does not move work into `refs/archive`.

## 4. Candidate designs

- **A. Re-pack against `origin/main`, and drop a bundle that `origin/main` holds.** The scope builds a thin bundle with the same heads, verifies it, and renames it over the old bundle.
- **B. Replace the bundles with refs under `refs/archive/salvage/`, and add a retention rule.** Each head becomes a ref in the hub. The scope deletes the bundle.
- **C. Retention by age only.** The scope deletes each leaf older than a limit.

| Criterion (weight) | A | B | C |
|---|---|---|---|
| Loses no commit (0.30) | 5: heads stay, in a thin bundle plus `origin/main` | 4: the hub repository holds the only copy | 1: unique commits are deleted |
| Frees the disk (0.25) | 4: 150 GiB to a few GiB | 5: 150 GiB to 0 | 5: frees all that it deletes |
| The leaf restores alone (0.20) | 4: needs `origin/main`, as the writer already does | 2: needs the hub ref to survive | 1: the leaf is gone |
| Build risk (0.15) | 3: a scratch repository, one verify step | 3: a ref namespace, a retention rule and a restore verb | 5: a delete loop |
| Fit with the parent plan (0.10) | 5: it is K6 | 3: it is D5, for new work | 2: K5 without its landing proof |
| **Weighted total** | **4.30** | **3.55** | **2.65** |

A wins on the first and third criteria. The operator chose lossless, so C is out at once. B frees more, but a hub-only copy has one failure point, and the leaf stops being a backup. B stays valid for K5 later, with a ref for each leaf that a retention rule drops.

## 5. Decisions

| Id | Decision | Because | Rejected and why |
|---|---|---|---|
| D1 | Build design A as `evolve gc --only salvage` | F2.3, F2.4, F2.11 | C: it deletes unique commits. B: the leaf stops being a backup |
| D2 | A leaf action is one of `repack`, `drop-redundant`, `keep`. Each `keep` has a reason | F2.1, F2.3 | A silent skip: the operator cannot see why a leaf stays |
| D3 | `drop-redundant` removes only `refs.bundle`. The leaf keeps `HEAD`, the patch and the untracked archive | F2.6. The writer makes this same leaf shape when the tip is in `origin/main` | Remove the leaf: the patch is unique work |
| D4 | The scope builds the thin bundle in a scratch bare repository. The scratch repository reads objects of the hub through `objects/info/alternates`. It sets each head ref under its original name with `update-ref --no-deref`, and sets `refs/remotes/origin/main`. The apply step runs under the gc lease and a few seconds after the plan. A missing object makes `update-ref` fail, and the leaf stays. This plan replaces the fetch step of K6, because a fetch copies about 1 GiB for each leaf | F2.5. A bundle records ref names, and `bundle create` takes a ref name, not a sha. A temporary ref in the hub has another name, or it revives the branch | A temporary ref in the hub: the head name changes. A recreated `refs/heads/<cycle>`: it revives a branch that the sweep deleted |
| D5 | The apply step checks four rules before the rename. The heads are equal one for one. `git bundle verify` passes in the hub. The old bundle has the size and mtime of the plan, and `HEAD` holds the text of the plan. The leaf is not held. A `drop-redundant` apply also counts again the commits of each head that `origin/main` lacks, and the count must be 0 | G2 | Trust the plan: the writer of a live cycle can replace the bundle between the plan and the apply |
| D6 | The replace writes `refs.bundle.repack` in the leaf and renames it over `refs.bundle` | G2. A rename on one filesystem is atomic | A copy: a crash leaves a half file |
| D7 | A leaf is held when `isLive` or `sealHold` returns a reason, or when its bundle changed in the last hour | F2.9. A writer replaces `HEAD` in place, so a fresh leaf can be half written | Only the lease: the writer of a leaf runs after the lease ends |
| D8 | Each applied action appends one row to `.evolve/archive/salvage-repack.jsonl` before the destructive step. The row has the leaf, the action, the heads, the basis, and the bytes before and after | F2.10 | A new log home: gc has one |
| D9 | A bundle that fails any rule is kept and reported with its reason. The scope exits 1 when one apply step failed | the request | Delete on doubt |
| D10 | `bundleRef` of the remote sweep reuses `bundleVerified`, with the same basis rule | F2.6, F2.7 | A copy of the basis logic: two owners of one belief |
| D14 | The final recheck, the rename and the remove run under the ship lock (`flock.ShipLockPath`), one leaf at a time | F2.13. The sealed-cycle sweep writes `cycle-` leaves under the ship lock, not under the gc lease | The gc lease alone: a seal can rewrite the leaf between the recheck and the rename |
| D12 | The scope restores the mtime of the leaf directory after each rename or remove | F2.12 | Leave the mtime: a repack then resets the TTL of its leaf, and the other leaves age alone |
| D13 | The plan reads only `refs.bundle`. A stray `refs.bundle.repack` from a crash is overwritten by the next repack of that leaf | F2.6 | Read the stray file as a bundle |
| D11 | The dry-run prints the same lines as the apply, with the prefix `evolve gc --dry-run:`. Its `repack` size is the size that the bundle holds today, so the figure is an upper bound | F2.8 | Build the thin bundle in the dry-run: it writes files |

Consequences:

- Good: one run frees about 150 GiB, and the leaf still restores from `origin/main` and the thin bundle.
- Bad: a thin bundle needs its basis commits. A rewrite of `origin/main` history can remove them. The audit row names the basis.
- Change the decision when a leaf must restore with no network and no `origin/main`. Then use design B with a full mirror.

## 6. Components

| Id | Component | Red test first |
|---|---|---|
| C1 | `PlanSalvageRepack` and `ApplySalvageRepack` in `go/internal/gc/salvage_repack.go` | a full bundle in a fixture repository becomes a thin bundle with the same heads, and `git bundle verify` passes |
| C2 | The audit row writer, `AppendSalvageRepack` | an applied action appends one JSON line with the heads and the basis |
| C3 | The scope `salvage` in `go/cmd/evolve/cmd_gc_scope.go`, and the flag text | `evolve gc --only salvage --dry-run` lists each action and changes no byte. A non-dry run applies them |
| C4 | The writer fix in `bundleRef` | the remote sweep leaf for a branch one commit past `origin/main` is under 1 MiB |

Tests drive the real verbs: the `gc` package functions with real git repositories from `go/internal/gittest`, and `runGC` for the scope. Each test covers one rule:

| Case | Expectation |
|---|---|
| a full bundle of one head | `repack`. The new bundle lists the same `sha ref` and verifies |
| a bundle whose heads are all in `origin/main` | `drop-redundant`. `refs.bundle` is gone and the patch stays |
| a seal writer holds the ship lock during the apply, and changes the bundle | the apply waits, checks again and refuses. The new bundle of the writer stays |
| the `HEAD` text of a leaf changes after the plan | the apply refuses |
| a bundle that is corrupt | `keep`. The file is unchanged and the report names the reason |
| a bundle whose head the hub does not hold | `keep` |
| a bundle that is already thin | `keep` |
| the leaf of a live run | `keep`. The file is unchanged |
| a leaf written in the last hour | `keep` |
| `--dry-run` | the same lines, and a hash of the salvage home is equal before and after |
| the bundle changes between plan and apply | `keep`. The new file stays |

## 7. Order

1. C1 and C2 with their red tests, because the scope calls them.
2. C3, because it is the verb that the console runs.
3. C4, because it is independent and small.

## 8. Verification

| Measure | Before | After |
|---|---|---|
| bytes in `.evolve/operator-salvage` | 150.7 GiB, 144 full bundles | under 20 GiB, 0 full bundles |
| heads that differ between old and new bundle | not applicable | 0 |
| bundles that fail `git bundle verify` | 0 | 0 |

The console runs the dry-run first and compares its counts with F2.3. After the apply it runs `evolve gc --only salvage --dry-run` again. The second dry-run must list 0 `repack` leaves.

## 9. Risks

| Risk | Control |
|---|---|
| A live cycle writes `refs.bundle` while the scope reads it | D7 settle window, and the size and mtime check of D5 |
| The thin bundle misses a commit | D5 compares the heads, and the verify step needs each basis commit in the hub |
| The writer of a leaf rewrites the leaf between the recheck and the rename | D14: the recheck and the rename run under the ship lock that the sealed-cycle writer holds. A bundle that the writer makes after the rename replaces ours, and it is complete |
| Two gc runs race | the gc lease of a non-dry run (`go/cmd/evolve/cmd_gc.go:50`) |
| `origin/main` is stale in the hub | the report prints the basis. A stale basis gives a thicker, still correct bundle |
| The scratch repository leaks | the scope removes it in a deferred call. It holds no object copy, so a leak is a few kilobytes |
| A multi-head bundle has a head in `origin/main` | `bundle create` drops that head, the heads differ, and D5 keeps the leaf |

## 10. Open questions

- **Q1.** Does the operator want design B for K5 later? This plan does not decide it.
- **Q2.** A head that the hub no longer holds is kept. A later scope can fetch it from the full bundle. The count today is 0.

## 11. Review history

- **Round 1** (architecture-reviewer, design only): FAIL with 1 CRITICAL, 3 HIGH and 4 MEDIUM findings.
  - The CRITICAL finding asked for a scratch repository that fetches the full bundle. The plan keeps the alternates design (D4). It states that each failure keeps the leaf.
  - The HIGH findings gave three fixes. F2.9 covers liveness for each leaf prefix. D5 and section 9 cover the rewind of `origin/main` and the writer race. F2.12 with D12 covers the TTL age of a leaf.
  - The MEDIUM findings gave D13 and `--no-deref` in D4.
- **Round 2**: WARN with 1 HIGH finding. It gave F2.13 and D14, the ship lock. All other round 1 fixes were confirmed.

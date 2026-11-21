# Plan: concurrent edits — one entry, one file, so that parallel lanes do not meet in the same lines

- **Status:** partly built, 2026-11-21 (C2 done, C6 and C7 partial, C3 to C5 open, C1 and C8 dropped). Earlier status: proposed, 2026-11-17. Amended the same day: section 11 changes the components and the order. Amended 2026-11-18: section 12 starts C2 again.
- **Research:** [Concurrent edits: where lane changes collide in the code](../research/concurrent-edit-layout-2026-11.md). **Decision:** no ADR yet. D1 and D2 add a layout rule to the code conventions. No decision here changes admission.
- **Related:** [the console builds only what a lane cannot build](console-build-or-route-2026-11.md). Its component K5 changes admission so that two items in different files of `cmd/evolve` both admit. This plan does not replace K5. C6 moves code into smaller packages, so each admission cell gets smaller with no admission change. K5 stays in its plan and runs after C6 only when the waiting items stay above the target of §8. The landing side stays as [concurrent cycle landing](concurrent-cycle-landing-2026-10.md) designed it.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)
11. [Amendment of 2026-11-17](#11-amendment-of-2026-11-17)
12. [Amendment of 2026-11-18: C2 starts again](#12-amendment-of-2026-11-18-c2-starts-again)

## 1. Request

The operator, 2026-11-17:

> "Could we breakdown the code in to small independent pieces and restructuring them to be easier to be concurrently modifiable without resolving the merge conflict? Research online to learn how other harness reduce the time for merge conflicts, I remembered we did the similar research before, check it out and do the research against the latest online results"

A worked example from the same day. The console rebuilt old PRs on the current main and resolved 9 conflicts in 8 PRs. Seven were two independent additions to one list, one struct or one import block. One was a stub that had to follow a changed signature. Only one (#1542) was a real design conflict.

## 2. Facts

| Fact | Research id |
|---|---|
| The earlier research made conflicts cheap to verify. It did not change the layout that makes them. | E1.1 |
| A hot file raises the conflict rate, also for a small change. | E1.2 |
| 7 central lists take 330 touches in 30 days. The biggest is a 1,014-line serial-test map. | F2.2 |
| 2 generated aggregates take 164 touches. Their sources never conflict. | F2.3 |
| `cmd/evolve` and `internal/core` hold 2,654 touches. A package is one admission cell. | F2.4, F2.5 |
| 7 of the 9 conflicts of 2026-11-17 were independent additions. One was a design conflict. | F2.7 |
| A clean merge can still be wrong. | F2.8, P3.8 |
| The signals do not name the conflicted files. | F2.9 |
| Practice: one fragment file for each entry, self-registration, disjoint modules, a syntax-aware merge driver as a help. | P3.3 to P3.6 |

## 3. Goals and non-goals

Goals:

1. A lane that adds a member to a set edits no file that another lane edits for its own member.
2. No person and no model resolves a conflict in a generated file.
3. Two lanes that change two unrelated parts of a big package can run at the same time.
4. The loop measures which files conflict.

Non-goals:

- This plan does not remove the gates on the composed tree. A clean merge still runs every gate (F2.8).
- It does not change the version control system (research §4, P3.10).
- It does not move a whole package in one change.

## 4. Candidate designs

| Design | What it does |
|---|---|
| A. Merge tooling only | Add a syntax-aware merge driver and a regenerate driver. No file moves. |
| B. One entry, one file | Replace each central list with one file or one registration call for each member. A generator writes the aggregate, and a merge regenerates it. |
| C. Split the big packages | Move verb groups out of `cmd/evolve` and seams out of `internal/core`, along their change coupling. Admission cells get smaller as a result. |
| D. Serialize more | Lower the lane window, or dispatch only lanes with disjoint predicted files. |

Scores: 0 is bad, 2 is good.

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Removes the conflicts of central lists (F2.2) | 1 | 2 | 0 | 1 |
| Removes the conflicts of generated files (F2.3) | 2 | 2 | 0 | 1 |
| More lanes run at the same time (F2.5) | 0 | 1 | 2 | 0 |
| Keeps every gate on the composed tree (F2.8) | 1 | 2 | 2 | 2 |
| Cost and risk to adopt | 2 | 1 | 0 | 2 |
| **Total** | 6 | 8 | 4 | 6 |

B wins as the first step. A merge driver alone (A) still lets two additions meet. It resolves them only when the syntax is simple, and a bad resolution passes as clean. C gives the most lane width. But each move conflicts with each lane in the package, so C runs after B, one group at a time.

D trades lane width for fewer conflicts, which is the opposite of the goal. A single-writer lock on each hot list is a form of D: it serializes the lanes that add to one list.

The plan adopts B now, A as a shadow help, and C in steps.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Each member of a set that changes add to has one entry in one place. The entry is a registration call in its own file, or one fragment file. | F2.2, F2.7, P3.4, P3.5 | `merge=union` on the list: our lists also delete entries, and a deleted entry comes back with no conflict (P3.7). |
| D2 | A tracked aggregate has one generator. When a merge conflicts in generated files only, the merge runs the generator and takes its output. This depends on Q1. If `git merge-tree` reads no merge driver, the train and the fleet rebase run the generator as their own step. | F2.3, P3.4 | Stop tracking the aggregates: people and CI read them on GitHub. |
| D3 | A test declares its own serial reason in its own file, and the parallel-test guard reads the declaration from the test. The central serial maps go. | F2.2 (155 touches) | Keep the maps sorted: sorting moves the conflict, it does not remove it. |
| D4 | Each verb registers itself from its own file. `registry.go` keeps only the dispatch. A golden test lists the verbs, so a lost registration fails. | F2.2, F2.4, P3.5 | One table sorted by name: two new verbs still meet near each other. |
| D5 | Big packages split along change coupling, one group at a time, at a boundary, with no lane in the package. Verb groups move to `internal/cli/<group>cmd`, the pattern that 6 groups already use. | F2.4, F2.5, P3.3, P3.9 | One big move: it conflicts with each lane in the package, and its review cannot hold the whole change. |
| D6 | A syntax-aware merge (mergiraf, Go support) runs in shadow on each fleet rebase and train composition. It records if it resolved the conflict. Its result runs every gate. It decides nothing until a shadow record proves it. | F2.7, P3.6, P3.8 | Enforce at once: a wrong clean merge looks the same as a right one (F2.8). |
| D7 | The rebase and train signals carry the list of conflicted files. | F2.9, R5 | Count rebases only: the count does not say which file to fix. |
| D8 | The protected surface follows the files. A gate list that moves into fragment files keeps its protection. The manifest entry covers the fragment directory, which depends on Q3. If a directory entry does not deny a new file in it, the gate keeps one protected file that lists its fragments. | `guards.ProtectedSurfaceManifest` | Leave the manifest as is: a lane can add an allowance to a gate fragment. |

Consequences:

- Good: a new verb, test, gate package or index entry is a new file. Two lanes that add two members do not meet.
- Good: a conflict in generated text costs no console time and no model call.
- Bad: a reader finds a list through a generator or a registry, not in one file. The generated aggregate stays as the place to read.
- Bad: self-registration hides the order of registration. The golden test and a sorted output fix the order.

What changes our mind: the hot-file measure (D7) shows that most conflicts after B are not in lists. Then C moves up, before the rest of B.

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| C1 | The rebase and train signals name each conflicted file (D7). The fleet merge is in `go/internal/phases/ship/landing/`, a protected directory (`guards/integrity_surface.go:157`). | a fleet rebase with a conflict in `a.go` emits a signal whose fields list `a.go` | console (the landing is protected) |
| C2 | A generated-only conflict regenerates (D2) for `docs/research-index.md` and `docs/architecture/signal-codes.md`. | two branches that each add an index shard compose with no conflict, and the aggregate holds both rows | console (the train is protected) |
| C3 | Serial tests declare their reason in their own file (D3). The guard in `internal/paralleltest` reads the declaration. The maps of all 10 packages go, one package for each change. | a test with no `t.Parallel()` and no declaration fails the guard; a declared test passes | lane, one package at a time |
| C4 | Verbs self-register (D4). | a verb file that registers itself is found by `evolve help`; the golden verb list fails when a registration is lost | lane |
| C5 | `.apicover-enforce` and `.cover-strict` become one fragment file for each package under a gate directory, and the manifest protects the directory (D1, D8). | a package with a fragment is enforced; a package without one is not; the manifest covers the directory | console (gate lists are protected) |
| C6 | Split `cmd/evolve` by verb group, along change coupling (D5). Each group becomes its own package, so its own admission cell, with no change to admission. Design first, with the coupling census. | two items that edit two different verb groups both admit | lane, after a design review |
| C7 | Split the hot package notes (`internal-core.md`, `cmd-evolve.md`, `internal-bridge.md`) along the same groups as C6. | the docs index check finds each new note | lane, with C6 |
| C8 | mergiraf in shadow on rebase and train composition (D6). | a composition with two import additions records "resolved by structural merge" in shadow, and the gates still run | console (the train is protected) |

## 7. Order

1. C1 first, so that each later step has a measure.
2. C2 and C3: the largest touch counts and the smallest risk.
3. C4 and C5.
4. C8 in shadow, in parallel with the rest.
5. C6 and C7 after their design review, one group at a time, at boundaries.

## 8. Verification

| Measure | Before (2026-11-17) | Target | Command |
|---|---|---|---|
| Touches on the 7 central lists in 30 days | 330 | under 50 | `git log --since --name-only` |
| Conflicts in generated files that a person resolves | 2 on 2026-11-17 | 0 | the C1 signal |
| Items that wait on a cell | 20 (wave 178) | under 8 | `evolve wave supply` |
| Fleet rebase recoveries for each 100 cycles | 61 in cycles 2300 to 2452 | a falling trend | `evolve signals` |
| Conflicts by file kind | not measured | the C1 measure exists for 2 waves | the C1 signal |

## 9. Risks

| Risk | Control |
|---|---|
| A self-registration is lost and a verb or a test disappears with no error. | The golden list (C4) and the guard (C3) fail on a lost entry. |
| A gate list in fragments lets a lane add its own allowance. | The manifest protects the fragment directory (D8). The fragment is a protected path, so a lane cannot write it. |
| A package move conflicts with each lane in the package. | One group at a time, at a boundary, with no lane in the package (D5). |
| A structural merge gives a clean but wrong result. | Shadow first, and every gate on the composed tree (D6, F2.8). |

## 10. Open questions

- **Q1.** Does `git merge-tree --write-tree`, which the train uses, read a merge driver from `.gitattributes`? C2 and C8 depend on the answer. A test answers it before their design.
- **Q2.** Does C6 split `cmd/evolve` into many verb packages, or into a few groups? The coupling census of the C6 design answers it.
- **Q3.** Does the protected surface of a gate fragment need one entry for each file, or one entry for the directory? D8 assumes the directory.

## 11. Amendment of 2026-11-17

Before the console started C1, it read the code and the signals again. Three premises of this plan were wrong:

- The signals already name each conflicted file (research F2.9, corrected).
- Lanes had no code conflict in the 149 runs of cycles 2300 to 2462. The fleet rebase already regenerates the derived outputs (F2.10).
- All 21 items that wait on a cell wait on the `cmd/evolve` package. A fragment file in that package stays in its cell (F2.11).

So the conflicts that this plan counted cost the console, in PRs that stayed open for days. Rule 6 of operating-policy §2 limits those PRs. The lanes lose time to the coarse cell, not to merges.

The loop also did part of the work before this amendment:

- Cycle 2455 wrote [the admission plan for the coarse cell](admission-cmd-evolve-cell-2026-11.md). Its K1 (file cells for leaf packages) is in the build of cycle 2461. Its K2 is the self-registration of verbs.
- Cycle 2460 wrote [the split design for `cmd/evolve`](split-cmd-evolve-2026-11.md). It answers Q2: a few groups, with `inbox` first.

Each component now has this state:

| Id | Before | Now | Reason |
|---|---|---|---|
| C1 | console | withdrawn: built | F2.9 |
| C2 | console | narrowed to the train, and deferred | The fleet rebase regenerates already (F2.10). Only console PRs meet a derived conflict in the train. |
| C3 | lane | kept | It removes the largest central list. A file cell (admission K1) needs it, or the serial map becomes the hot file. |
| C4 | lane | withdrawn: the same work as item `cmd-evolve-self-registering-verbs` (admission plan K2) | one item for each change |
| C5 | console | deferred | The gate lists map to no cell (F2.11), and no lane conflicts on them (F2.10). |
| C6 | lane | first | The design is done. Its six move items are filed. |
| C7 | lane, with C6 | kept, with each move | the split design D5 |
| C8 | console | withdrawn | Lanes have no conflicts for a shadow to measure (F2.10). |

Answers to the open questions:

- **Q1.** The train runs `git merge-tree` with `--attr-source` set to the base (`go/internal/integration/compose.go:274`). So the train reads `.gitattributes` from main. C2 can use a merge driver if it starts again.
- **Q2.** The split design answers it: a few groups, `inbox` first.
- **Q3.** The manifest has directory entries, for example `/go/internal/phases/ship/landing/` (`go/internal/guards/integrity_surface.go:157`). C5 is deferred, so the question waits.

The new order:

1. The six moves of the split design, one for each boundary, starting with `split-cmd-evolve-inboxcmd`.
2. Admission K1 (in build), then C3. Then K2, which replaces C4.
3. The measure of section 8 changes: the target is the count of items that wait on a cell (under 8). The touch counts of the central lists are no longer a target.

The six move items were lost on the first try. The build agent of cycle 2460 wrote them as `inbox-add` requests, and the request channel refuses each kind except `dispatch`. The console filed them from the request files. Item `agent-request-inbox-add-has-no-executor` fixes the channel. The incident record is [the build agent's inbox-add requests were refused](../incidents/2026-11-17-the-build-agents-inbox-add-requests-were-refused.md).

## 12. Amendment of 2026-11-18: C2 starts again

The operator asked: "Continue mergeing the console code to integration as batch merges and take to main as a whole package". The dry run of that batch ejected two console PRs on `docs/research-index.md`. Batch 178 ejected two others on the same file. The incident record is [the train ejected four console PRs on the research index](../incidents/2026-11-18-the-train-ejected-four-console-prs-on-the-research-index.md).

### 12.1 Facts

**F12.1** Two batches ejected four console PRs on `docs/research-index.md`. Batch 178 ejected #1546 and #1548 at 18:40 UTC on 2026-11-17. The next dry run ejected #1550 and #1552 at 23:43. The second rebuild took 14 minutes, two review seats and two closed PRs (the incident record, timeline).
*Implication:* section 11 deferred C2 because only console PRs meet this conflict. Console PRs carry the plans, so the conflict comes back at each boundary.

**F12.2** Q1 has a test answer. In a scratch repository with git 2.54, `git merge-tree --write-tree` ran a merge driver that `.gitattributes` named and git config defined. The merged file held the output of the driver. The train passes `--attr-source` as a global git option (`go/internal/integration/compose.go:274`).
*Implication:* a merge driver is possible in the train. Its command comes from git config, which is not in the repository.

**F12.3** A merge driver gets three versions of one file (`%O`, `%A`, `%B`) and its path. It does not get the other files of the merge ([gitattributes, defining a custom merge driver](https://git-scm.com/docs/gitattributes#_defining_a_custom_merge_driver), [a custom merge driver](https://github.com/Praqma/git-merge-driver)).
*Implication:* a driver for the research index cannot read the shards. It can only merge the sections of the aggregate, and the generator order (by shard name) is not in the aggregate.

**F12.4** When `git merge-tree --write-tree` finds a conflict, it still writes a tree. The conflicted files in that tree hold conflict markers ([git-merge-tree](https://git-scm.com/docs/git-merge-tree)). The shards are separate files, so they merge with no conflict.
*Implication:* the conflicted tree holds every source of the aggregate. A generator that runs on that tree gives the exact aggregate.

**F12.5** The fleet rebase already regenerates a derived output when only derived outputs conflict. It uses `derived.IsDerivedConflict` (`go/internal/derived/conflict.go:7`) and `derived.Worktree.Regenerate`, which runs the generator, runs the check and refuses a leftover marker (`go/internal/derived/regen.go:32`).
The catalog is `derived.Catalog()` (`go/internal/derived/derived.go:40`). The train ejects at `go/internal/integration/compose.go:220`.
*Implication:* the train can call the same catalog and the same regeneration. It needs no second generator.

**F12.7** `Regenerate` refuses a leftover marker through `git diff --check <Base>` on the outputs (`go/internal/derived/regen.go:43`). `core.DerivedWorktree` sets `Base` to `HEAD` (`go/internal/core/ship_recovery.go:537`). Each generator runs as `go run ./cmd/evolve` in the worktree (`go/internal/core/worktree_evolve.go:19`). One run of `evolve docs index check` this way took 3.3 seconds with a warm build cache, on 2026-11-18.
*Implication:* the marker guard works only when `HEAD` of the temporary worktree is a tree with no markers. Each generator run compiles the composed tree.

**F12.8** The train has no lock over a whole batch. The epoch lock covers only the increment of the epoch counter (`go/internal/integration/epoch.go:51`).
*Implication:* the temporary worktree needs a unique directory. Two batches at the same time are out of scope, because the console runs one batch at each boundary.

**F12.6** Current practice for a generated file is to regenerate it, not to merge it by hand. Lockfile guides say to take one side and run the generator ([OpenTofu lock files](https://oneuptime.com/blog/post/2026-03-20-resolve-lock-file-conflicts-opentofu/view), [lockfile conflicts](https://nit-administrative-systems.github.io/AS-CloudDocs/github/appendix-lockfile-resolution)).
*Implication:* a regeneration step follows practice. A union merge of a generated file does not.

### 12.2 Candidate designs

| Design | What it does |
|---|---|
| A. Union attribute | Set `docs/research-index.md merge=union` in `.gitattributes`. |
| B. Merge driver | Set a driver `evolve docs index merge %O %A %B` in `.gitattributes` and in the git config of the train repository. |
| C. Regeneration step in the train | When each conflicted path is a derived output, put the conflicted tree in a temporary worktree, run `derived.Worktree.Regenerate`, and take the new tree. |
| D. One docs PR owns the aggregate | A rule: in each batch, one docs PR carries all shards and the aggregate. This is the manual fix of the incident. |

| Criterion | A | B | C | D |
|---|---|---|---|---|
| The output equals the generator output (F12.3, F12.4) | 0: the union keeps the order of the two sides, not the shard order | 1: it merges sections, and the order within a date can be wrong | 2 | 2 |
| One home of the generator logic (F12.5) | 2 | 0: a second merge logic | 2: the derived catalog | 2 |
| No setting outside the repository (F12.2) | 2 | 0: git config of the train repository | 2 | 2 |
| Console time at each boundary (F12.1) | 2 | 2 | 2 | 0: a rebuild and a review for each code PR with a plan |
| Size of the protected change | 2 | 1 | 1 | 2 |
| **Total** | 8 | 4 | 9 | 8 |

C ties D on criterion 1. C wins by one point, because D costs console time at each boundary (criterion 4). A and D tie at 8. A fails criterion 1: a wrong order makes `evolve docs index check` red on main, and that check is a hard stop. D is the cost that this amendment removes.

### 12.3 Decisions

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D9 | The train regenerates the derived outputs of a commit when each conflicted path is a derived conflict. Any other conflicted path ejects the PR, as now. | F12.4, F12.5 | A union merge: the order is wrong (F12.3). A merge driver: no shards, and a setting outside the repository (F12.2, F12.3). |
| D10 | `apply` classifies: it reads the merged blob of each conflicted path from the conflicted tree with `git cat-file`, and calls `derived.IsDerivedConflict`. It calls the port only when each path passes, so a mixed conflict never reaches the port. A read error or a missing blob counts as not derived, for example when one side deletes the file. Then the PR ejects with the cause `conflict`. The port is `Regen(ctx, onto, conflictedTree, paths) (tree, error)` on `integration.Composer`, where `onto` is the running tip that `mergeTree` got. `cmd/evolve` wires it to `derived.Catalog()` and `derived.Worktree`, and a nil port keeps the current ejection. | F12.5 | Import `core` into `integration`: it turns the dependency direction. A copy of the regeneration code: a duplicate. |
| D11 | The port adds a detached temporary worktree at `onto`, a tree with no markers. It writes the conflicted tree into the index and the work tree only, with `git read-tree -u --reset <conflictedTree>`, with no commit. It gets the generator from `core.DerivedWorktree(dir)`, whose `Base` is `HEAD`. `HEAD` of the worktree is `onto`, so the marker guard compares against a clean tree with no second generator. It runs `Regenerate` for each entry, stages the outputs and runs `git write-tree`. | F12.4, F12.7 | A commit of the conflicted tree as `HEAD`: `git diff --check HEAD` then sees no marker, so the guard does nothing. Render from blobs in memory: the generators read files on disk. |
| D12 | The port is a value with a field `Dir`. `cmd/evolve` sets it to `.evolve/ephemeral/integration-regen/` under the project root and creates it. Each call makes a unique directory in it with `os.MkdirTemp`. The port removes the worktree with `git worktree remove --force` on each return path, under a new context of 30 seconds that the batch context does not cancel. It runs `git worktree prune` before it adds one. | F12.8 | A fixed directory: two batch attempts meet in it. Removal under the batch context: a timed-out batch leaves a registered worktree. |
| D13 | A failed regeneration ejects the PR with the cause `conflict`, the paths and the error. A generator that does not build on the composed tree also ejects the PR. The train never lands a tree with a conflict marker. | F12.7 | Land the regenerated tree with no check: a wrong aggregate reaches main. |
| D14 | The port runs in both stages of the Composer, shadow and enforce, because it is part of the composition, not a gate. The proof of a PR with at least one regenerated commit names the regenerated paths in `Evidence.Derived`, under the rule `derived`. `overlap.Prove` reads no other merge class than `genuine` (`go/internal/overlap/proof.go:22`), so the class stays `clean`. | F12.5 | Regenerate only at enforce: the shadow batch, which lands today, still ejects. |

D9 to D14 replace the C2 row of section 6 and the C2 deferral row of section 11.

Consequences:

- Good: two console PRs that each add a shard land in one batch. The console does not rebuild a PR for a generated file.
- Good: the train and the fleet rebase use one regeneration path.
- Bad: each derived conflict costs a temporary worktree and one compile of `cmd/evolve` in it: 3.3 seconds with a warm cache for each generator (F12.7).
- Bad: a PR that breaks the build of the composed tree ejects, because its generator does not build (D13). The CI of the PR finds that first.

What changes our mind: a generator run in the train that takes more than 2 minutes. Then the regeneration moves after the batch, as one step.

### 12.4 Component C2, as built

| Part | File | Red test first |
|---|---|---|
| The conflicted tree comes back with the paths | `go/internal/integration/compose.go` (`mergeTree`, `apply`) | two branches that each add a shard: `apply` gets one tree and no ejection, and the aggregate holds both sections in shard order |
| The classification in `apply` | `go/internal/integration/compose.go` | a conflict in `docs/research-index.md` and in `a.go` ejects, and the port is not called |
| A missing blob is not derived | `go/internal/integration/compose.go` | a conflict where one side deletes `docs/research-index.md` ejects with the cause `conflict`, and the port is not called |
| Both stages regenerate | `go/internal/integration/compose.go` | with a regenerated commit, a shadow Composer and an enforce Composer each compose with no ejection |
| The port and its nil case | `go/internal/integration/compose.go` (`Composer`) | a nil `Regen` ejects, as now |
| A later commit regenerates onto the running tip | `go/internal/integration/compose.go` | a console PR with two commits, where the second one conflicts on a derived output, lands with the output of both |
| A failed regeneration ejects | `go/internal/integration/compose.go` | a port error ejects with the cause `conflict` and names the error |
| The marker guard works | the port in `go/cmd/evolve/` | a generator stub that leaves a marker makes the port return an error |
| No worktree is left | the port in `go/cmd/evolve/` | after a failed generator, `git worktree list` has no entry of the port |
| The proof records the regeneration | `go/internal/integration/compose.go` | a PR with at least one regenerated commit has the regenerated path in `Evidence.Derived` and the rule `derived` in its proof |
| The wiring | `go/cmd/evolve/cmd_integration_batch.go` | a journey test: `evolve integration batch --dry-run` on a fixture repository composes two shard PRs with no ejection |

The train is protected, so the console builds C2. The red tests above are the gate of each part.

### 12.5 Limits

- C2 does not change the fleet rebase. It already regenerates (F12.5).
- C2 does not resolve a conflict in a source file, or a conflict where a source file and a derived output both conflict.
- The test of F12.2 ran on one git version (2.54). C2 does not depend on a merge driver.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| C1 conflicted files in the signals | dropped (section 11: the signals already name each file, F2.9) | none | none |
| C2 train regenerates derived conflicts | done | [#1558](https://github.com/mickeyyaya/evolve-loop/pull/1558), [`36dee80`](https://github.com/mickeyyaya/evolve-loop/commit/36dee80b35769e24e3cdd34d3616ce4266a09bb7); [#1580](https://github.com/mickeyyaya/evolve-loop/pull/1580), [`f13a5f0`](https://github.com/mickeyyaya/evolve-loop/commit/f13a5f0ddb76f02d3574138b45ba91c301b2d3ed); `go/internal/integration/compose.go:55`, `go/cmd/evolve/cmd_integration_regen.go` | none (consumed) |
| C3 serial tests declare in their own file | not started | none; `go/internal/paralleltest/paralleltest.go` still reads the serial maps | `edit-layout-c3-serial-tests-declare-in-place` |
| C4 verbs self-register | not started (merged into admission K2) | none | `cmd-evolve-self-registering-verbs` |
| C5 gate lists as fragments | not started (deferred in section 11) | none | `edit-layout-c5-gate-lists-as-fragments` |
| C6 split `cmd/evolve` by verb group | partial: inbox, agent and land (pr-ci) moved; gc, wave, land rest and the spine design open | cycle 2503, [`5ed6d76`](https://github.com/mickeyyaya/evolve-loop/commit/5ed6d767432bb82a339e95152dc5832413118bdf) (inboxcmd); cycle 2495, [`fb493ae`](https://github.com/mickeyyaya/evolve-loop/commit/fb493ae1ae2cfc23cd3b2a9d03dbdddcd13f637d) (agentcmd); cycle 2564, [`ca8b6f0`](https://github.com/mickeyyaya/evolve-loop/commit/ca8b6f025676462ddc5ff46dccc1007e2cf7caae) (landcmd pr-ci); `go/internal/cli/` | `split-cmd-evolve-gccmd`, `split-cmd-evolve-wavecmd`, `split-cmd-evolve-landcmd-rest`, `split-cmd-evolve-spine-design` |
| C7 split hot package notes | partial: one note for each moved group; `cmd-evolve.md` is still one note | `docs/architecture/packages/internal-cli-inboxcmd.md`, `internal-cli-landcmd.md` | none (rides with each C6 item) |
| C8 mergiraf in shadow | dropped (section 11) | none | none |

The train regeneration of C2 is on main. The split of `cmd/evolve` runs one group for each boundary, and four of its items are open. C3, C4 and C5 are not built, and each has an open item. Admission K1 landed in [`295d242`](https://github.com/mickeyyaya/evolve-loop/commit/295d242043582038be26009b7a4bc164683c7287) (item `admission-file-cells-for-leaf-packages`, consumed), so C3 is now the next step of the new order. Every unfinished component has an open item or was dropped, so no follow-up draft is needed.

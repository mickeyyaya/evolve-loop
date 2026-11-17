# Plan: concurrent edits — one entry, one file, so that parallel lanes do not meet in the same lines

- **Status:** proposed, 2026-11-17.
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

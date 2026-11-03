# internal/reviewclass

> Plan: [review-routing-2026-11](../../plans/review-routing-2026-11.md), component S1, decisions D1 and D6. Operator decision D5 of 2026-11-03. The verb is `evolve review-class` (`go/cmd/evolve/cmd_review_class.go`, [cli-contract.md](../cli-contract.md) §5).

## Purpose

`internal/reviewclass` is the one table of change classes. It maps each changed path to a class, and each class to its review seats and its deterministic checks. The plan calls this table the matrix of section 6.

Before this package, the commit gate, CI and `gateclass` each had their own class rules (F2.13, F2.17). The plan makes this registry the one home. S4 connects the commit gate, S5 connects CI and S7 connects the lane code-review phase. This change connects no gate.

## A worked example: an inbox deletion with a guard edit

A diff deletes `.evolve/inbox/pending/x.json` and changes `go/internal/guards/role.go`. `Classify` returns three classes:

| Class | Paths |
|---|---|
| C2 Inbox data | `.evolve/inbox/pending/x.json` |
| C6 Go T2 | `go/internal/guards/role.go` |
| C11 T0 protected | `go/internal/guards/role.go` |

Rule R1 gives the union. The seats are `code-simplifier`, `go-reviewer|code-reviewer`, `architecture-reviewer` and `adversarial-review` at the deep tier on another family. The checks are the two inbox checks, the C6 checks, the gate flags and the guard mutation. `TestClassify_EachClassGetsItsSeatsAndChecks` pins this example as the case "R1 a mixed diff gets the union".

## Design

- **The table is data.** `Classes` returns a copy of the 14 rows C0 to C13. Each row has an ID, a name, a detection rule, the seats, the checks and the cost cap of the plan. A caller cannot change the registry through the copy.
- **A seat is a set of alternatives.** `Seat.Any` lists the personas that satisfy the seat. C6 needs `go-reviewer` or `code-reviewer`, so its seat is `go-reviewer|code-reviewer`. A seat can also have a lens, a tier and the other-family rule.
- **A planned item says so.** A seat or a check that a later component builds has `Planned` set to that component, for example `S2` or `S6`. The verb prints `(planned S2)` after it. Thus the output does not claim a check that does not exist yet.
- **The surface is injected.** `Classify` takes a `Surface`: the `Protected` predicate and the `Guarantees` function of a path. The verb gives `guards.IsProtectedSurface` and `guards.SurfaceClassesOf`. The package imports neither `guards` nor `core`, so a reader in `core` (S7) can import it with no cycle. This is the pattern of `gateclass`. A `Surface` with no predicate fails closed: each path is protected, and each T0 path needs the security seat.
- **Each row owns its matcher.** A class row holds its match function, and its detection text comes from the same lists. Thus the table and the matchers cannot drift apart.
- **One path can have more than one class.** Each class has its own matcher, and a path gets each class whose matcher is true. Thus a protected Go file is C6 or C7 and also C11.
- **The matchers.**

  | Class | Matcher |
  |---|---|
  | C0 | the caller proved the Go file comment-only |
  | C1 | `*.md` outside `agents/`, `skills/`, `guidance/` and `.evolve/phases/`, and not a whole generated file |
  | C2 | under `.evolve/inbox/` |
  | C3 | an output of a `derived.Catalog` entry |
  | C4 | one of the four floor-list files |
  | C5 | `*_test.go` or a path under a `testdata/` directory in `go/`, when `gateclass.IsGuardTest` is false |
  | C6, C7 | a Go file, or a file under `go/internal/` or `go/cmd/`, that is not test-only, with tier T2 or T1 from `gateclass.TierOf`. Thus the data file of a package, for example a bridge recipe, is code. |
  | C8 | `*.md` under `agents/` or `.evolve/phases/` (the phase prompts), or a file under `skills/` or `guidance/rules/` |
  | C9 | a profile, a phase spec, a bridge CLI manifest or `.claude-plugin/` |
  | C10 | under `.github/workflows/` |
  | C11 | the `Protected` predicate of the `Surface` |
  | C12 | the caller found a `stage-off-enforce` flag of `gateclass` in the file |
  | C13 | no other matcher is true |

- **R1, the union.** `Classify` merges the seats and the checks of each class that the diff touches. The order is the class order, then the order in each row. A check with the same ID comes once.
- **R2, the highest tier.** Two seats with the same alternatives and the same lens become one seat. The higher tier and the other-family rule stay.
- **R3, raise only.** A matcher can add a class to a path. No matcher removes a class from a path.
- **R4, the narrow waiver.** A proven comment-only Go file gets C0 and no other class, also when the file is protected. The proof compares the syntax tree and the directives, so the code of the file does not change. This is the waiver of the commit gate today.
- **Generated files.** A generated file gets C3. A whole generated file does not also get C1, because no person writes its text. A generated region in a document gets C1 and C3, because a person writes the rest of the file.
- **The conditional security seat.** C11 and C12 have `SecurityWhen`: the names of the sandbox, the dispatch-credential and the policy guarantee classes. `Classify` adds the `security` seat when the `Guarantees` of a path of the class include one of these names. `TestGuaranteeNames_AreTheGuaranteeClassesOfTheManifest` pins the names to the `guards` constants.
- **The generated check.** C3 lists the `Check` verb of each `derived.Catalog` entry that the diff touches, for example `evolve skills check`.
- **C13 fails closed.** A path that no matcher knows gets the C6 seats and the C6 checks. `KnownDefaults` lists the tracked C13 territory, for example `knowledge-base/` and `go/go.mod`. `TestReviewClass_EachTrackedPathHasAClassOrAKnownDefault` in `go/cmd/evolve` uses the production surface. It fails when a tracked path has no class and no entry. It also fails when an entry matches no tracked path.
- **Read.** `Read` takes a git function and a file reader from the caller, so the package runs no command. It reads the changed and the untracked paths against the base. It runs `gateclass.Read` and `gateclass.Classify` for the stage flips. It proves each changed Go file comment-only with `commentaudit.Equivalent`. A file under `testdata/`, a new file and a file that does not parse are not comment-only.

## Invariants

- Each class of the plan table exists, in order, with its seats and its cap (`TestClasses_EveryClassOfThePlanTableExists`).
- Each class has one path set with the expected seats and checks. A mixed diff gets the union, and a path that matches no class gets C13 (`TestClassify_EachClassGetsItsSeatsAndChecks`).
- The security seat of T0 follows the guarantee class (`TestClasses_TheSecuritySeatOfT0FollowsTheGuaranteeClass`).
- The package and its verb file are on the protected manifest. A change to them is class C11, so the registry cannot lower its own review.

## Limits

- Each workflow, diff reviewer persona and diff reviewer profile is on the protected manifest (G5). Thus such a change is always C11 as well, and it gets the T0 seats. The caps of C10 alone and C9 alone apply only to the paths outside the manifest.

- No gate reads the table yet. S4 adds the commit gate at the `review_routing.stage` key, S5 adds CI and S7 adds the lane phase.
- The size rule of graft G1 is absent. Open question Q1 of the plan sets its line count.
- C12 finds only a stage that goes from `enforce` to `shadow` or `off`, because that is the `gateclass` pattern. A flip to `enforce` is not C12.
- `core/floor_lists_regression.go` also names the four floor-list files of C4. The registry cannot import `core`, so the two lists are separate until a leaf package holds the names.
- The verb compares the working tree with the base. It does not read the index alone, so a staged and an unstaged change look the same.

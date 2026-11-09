# Research: a gate that gives one verdict for one tree

> Design set: [ADR-0165](../architecture/adr/0165-every-gate-is-a-typed-unit-with-a-conformance-contract.md) · plan: [gate-contract-2026-11.md](../plans/gate-contract-2026-11.md) · spec: [gate-contract.md](../architecture/gate-contract.md).
> Incident: [2026-11-09 duplicate ratchet](../incidents/2026-11-09-duplicate-ratchet-misses-staged-predicates.md).

The operator asked on 2026-11-10: "What can we learn from this issue? Do we need research or adr amendment to improve the robustness of the pipeline? Ultrathink to improve from the fundamental unit."

This dossier answers the question with local facts and outside practice. The fundamental unit is the gate: a deterministic check whose verdict lets a change commit, land or hand off.

## Terms used throughout

- **Gate:** a deterministic check that blocks a commit, a landing or a phase handoff. Examples: a repo-contract scanner, a phase floor, a ship check.
- **Hermetic:** the verdict depends only on the inputs that the gate declares.
- **Parity:** the gate gives the same verdict on the staged lane tree and on the landed commit of the same content.
- **Owner phase:** the one phase that can still change an artifact.
- **Base-red:** a failure that the base of a change already has, so the change did not cause it.

## 1. Local findings

**F1.1** The duplicate ratchet gave two verdicts for one tree. The base `6e739c76c` with the change of cycle 2250 staged gave `ok`. The same change committed gave "336 copies, above the baseline 334" (measured 2026-11-10, incident F1.3).
*Implication:* a gate that reads hidden state can pass at ship and fail on main.

**F1.2** ADR-0084 I1 made `internal/repostate.TrackedSet` "the one answer to 'what does git track here'" ([ADR-0084](../architecture/adr/0084-gate-integrity-invariants.md)). `testdup` calls `git ls-files` and `git log` itself, and it reads file mtimes (`go/internal/testdup/testdup.go:89-134` at `b33d6492b`). The fix #1382 closed the staged escape with a `git diff HEAD` read, so the gate is still not hermetic. No test refuses a direct git call in a scanner.
*Implication:* a single-source rule without a guard does not hold for the next gate.

**F1.3** Only `testdup` reads commit dates or an mtime. Other gates call git outside `repostate` (non-test files and pack scanners, measured 2026-11-10):

- the floors of `internal/core`: `comment_floor.go`, `floor_lists_base.go`, `gate_classifier_floor.go`, `lost_landing_floor.go`, `size_floor.go` and `ste_floor.go`;
- `internal/fleet/preflight.go` and the hook guard `internal/guards/docdelete.go`;
- pack scanners that are `_test.go` files in `internal/phasecoherence`: `gitignore_birth_test.go`, `tracked_artifact_hygiene_test.go`, `unpaired_tracked_test.go` and `unpaired_tracked_edge_test.go`.

The `"git"` matches in `rawgitratchet.go:139` and `evalqualitycheck/flakylint.go:260,320` are lint rules that look for the string. They are not git calls.
*Implication:* a guard on direct git calls needs a migration, and it must reach `_test.go` scanners. The first draft said "no migration", and the architecture review corrected it.

**F1.4** ADR-0117 found that "the rendered self-check never passes `--worktree`", so the build docs floor "never ran for a builder". It fixed the build phase only. The TDD self-check kept the same hole: `go/internal/phasecontract/render.go:118-120` renders no `--worktree`, and `verifyDeliverable` returns before each worktree check (`go/internal/cli/phasecmd/phase_verify.go:124`).
*Implication:* a fix for one phase did not become a rule for each phase.

**F1.5** A gate can judge an artifact that no later phase can change. The ratchet judged `go/acs/cycle<N>/` at ship. Only TDD can write that directory (`agents/evolve-tdd-engineer.md:100`). The builder of cycle 2242 spent 333 LLM minutes and failed on the frozen check.
*Implication:* the placement of a gate must follow the owner phase of its artifact.

**F1.6** Main was red from 08:24Z to 09:24Z and from 16:10Z on 2026-11-09 (runs of `7452aeec4` and `9a2bbe0e1`). No loop signal reported it. Cycles 2240 and 2249 failed on copies that their base already held.
*Implication:* the loop does not tell a base-red failure from a failure that the change caused.

**F1.7** Review, not a rule, caught each gate file that shipped off the protected surface. The cases are PR #549, ADR-0113, ADR-0117 and this incident ([internal-guards.md](../architecture/packages/internal-guards.md) history). The adversarial review of the follow-up found that the baseline `date` field is a lane-writable off switch for the ratchet.
*Implication:* the inputs of a gate are part of the gate.

**F1.8** The equal-count rule fails when lanes run in parallel. Two lanes that each remove one copy write the same `count=` line.
*Implication:* a count baseline does not compose. A baseline keyed on content can.

## 2. Outside practice

**F2.1** Bazel defines input hermeticity as "whether the inputs Bazel records are the ones the action actually reads". Undeclared host state, such as hidden tools or PATH lookups, makes a cached result unsafe on another machine ([bazel.build](https://bazel.build/basics/hermeticity), [VirtusLab 2.3.1](https://bazel.virtuslab.com/book/2~3~1/)). Sandboxing limits the files an action sees, but timestamps and randomness still change results ([VirtusLab 2.3.2](https://bazel.virtuslab.com/book/2~3~2/)).
*Implication:* a gate must declare each input. Commit dates, the index and mtimes are inputs.

**F2.2** The "not rocket science rule" (Graydon Hoare, bors) keeps a repository that always passes its tests. The system tests the merge result before it lands ([Graphite](https://graphite.com/blog/bors-google-tap-merge-queue), [Tyler Cipriani](https://tylercipriani.com/archives/2022/02/)). The GitHub merge queue exists to enforce it ([matklad](https://matklad.github.io/2023/06/18/GitHub-merge-queue.html)).
*Implication:* the rule holds only when the gate verdict on the tested tree equals the verdict on the landed tree. Parity is a precondition.

**F2.3** Baseline tools hold old violations and fail on new ones. PHPStan records error counts per file ([PHPStan](https://phpstan.org/user-guide/baseline)). ESLint bulk suppressions record counts per file and rule in `eslint-suppressions.json`. ESLint reports a suppression of a resolved violation until `--prune-suppressions` removes it ([ESLint](https://eslint.org/docs/v9.x/use/suppressions)).

Betterer users see frequent merge conflicts in the results file, and the tool has a `merge` command for them ([Betterer](https://phenomnomnominal.github.io/betterer/docs/results-file)).
*Implication:* count baselines conflict across parallel changes. A prune step keeps a baseline honest.

**F2.4** SARIF result managers gate on new findings with a set difference of stable fingerprints. They fall back to a hash of the rule, the normalized path and the message ([Trail of Bits](https://trailofbits.com/skills/sarif-parsing/)).
*Implication:* a baseline keyed on a content hash makes "no new copy" a set test. Two lanes that remove different entries then do not edit the same line.

**F2.5** Google submits all changes to one HEAD in a total order. Culprit finders search the range from the last passing version to the first failing version ([What Breaks Google?](https://research.google/pubs/what-breaks-google/)). A Google patent keeps the last known postsubmit state of each test, which includes "broken" ([US10733084B2](https://patents.google.com/patent/US10733084)).
*Implication:* a gate failure must be compared with the state of the base before the system blames the change.

**F2.6** In Chromium, a failure on a tree-closing builder closes the tree, and the tree reopens when that builder recovers ([Chromium sheriffing](https://chromium.googlesource.com/chromium/src/+/refs/tags/112.0.5598.1/docs/sheriff.md), [sheriff philosophy](https://www.chromium.org/developers/tree-sheriffs/sheriff-philosophy/)).
*Implication:* a red main must stop the landings until main is green, and the stop must end on its own.

## 3. Synthesis

The incident has one root: the pipeline has no type for a gate. Each gate is a function that some caller runs. So each lesson of ADR-0084, ADR-0110 and ADR-0117 fixed one instance and left the next gate free to repeat it (F1.2, F1.4, F1.7).

A lesson holds for each new instance only as a type plus a conformance test. The substitution matrix of ADR-0154 does this for the LLM families. The design that follows from F1 and F2:

| Property | Findings | Rule |
|---|---|---|
| Hermetic | F1.1, F1.2, F1.3, F2.1 | A gate reads only its declared inputs. Git facts come from `repostate` only, and a call-graph guard proves it. |
| Parity | F1.1, F2.2 | Each gate gives one verdict for the staged tree and the committed tree. |
| Owner-placed | F1.4, F1.5 | A gate on an artifact runs at the handoff of its owner phase. The host enforces it, and the self-check runs the same list. |
| Composable | F1.8, F2.3, F2.4 | A baseline holds keys of a body hash and a site, with a prune step at the boundary. |
| Attributable | F1.6, F2.5, F2.6 | A gate that rejects a lane reruns on the lane base. A base-red lane rebases, or parks while main is red. A red main stops each landing except a fix. |
| Protected with its inputs | F1.7 | A gate file and each declared input join the protected surface when the gate lands, and a new gate starts at shadow. |

## 4. Limits

- Parity does not prove that two lanes compose. Two lanes that each add one site of the same new body pass alone. The train tests the merged tree (ADR-0147).
- No public source describes how TAP attributes a failure that is already on HEAD. F2.5 gives the range model only.
- A content key for a helper body is the normalized token stream of `testdup`. A rename of a helper keeps the key. An edit of one token makes a new key.

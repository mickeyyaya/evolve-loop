# ADR-0165: Every gate is a typed unit with a conformance contract

- **Status:** Proposed (2026-11-10). The operator asked for this record on 2026-11-10 ("Ok" to the proposal of the console).
- **Plan:** [gate-contract-2026-11.md](../../plans/gate-contract-2026-11.md). **Research:** [gate-contract-2026-11.md](../../research/gate-contract-2026-11.md). **Design:** [gate-contract.md](../gate-contract.md).
- **Incident:** [2026-11-09 duplicate ratchet](../../incidents/2026-11-09-duplicate-ratchet-misses-staged-predicates.md).
- **Amends:** ADR-0084 and ADR-0117. In ADR-0084, I1 grows from "what git tracks" to each git fact that a gate reads. ADR-0117 gets one handoff floor for each phase. ADR-0157 gains a rule for new gate files. Each amended record holds a note that links here. Their earlier text stays unchanged.
- **Relates to:**
  - [gate-input-inventory.md](../gate-input-inventory.md): the trust classes of each gate input. The registry uses them and does not copy them.
  - ADR-0110: whole-tree tests run before main.
  - ADR-0147: the train tests the merged tree. At its `shadow` stage each lane still lands itself, so the main-red stop holds on both landing paths.
  - ADR-0154: the conformance matrix pattern.
  - ADR-0155 and ADR-0164: the self-heal path and the one goal.
- **Evidence:** research F1.1, F1.2, F1.4, F1.6 and F1.7:
  - two verdicts for one tree;
  - a single-source rule with no guard;
  - the ADR-0117 hole in TDD;
  - base-red failures blamed on lanes;
  - gate files and inputs off the protected surface.
- **Reviews:** three rounds of architecture, adversarial and docs review on 2026-11-10, and one patch check. Refinements R1 to R12 come from them. Each adversarial pass found one HIGH in the fix before it. The last one is R12, and its prescribed fix is in this text. The operator chose to land this record as Proposed without a fourth round (2026-11-10). Each component of the plan gets its own design review and red proof before it is built.

## Context

On 2026-11-09 the duplicate ratchet passed a staged tree at ship and failed the same tree on main. Main was red for hours. Five cycles failed, and the loop had no signal for it.

ADR-0084 and ADR-0117 had recorded each part of this failure before. Each fixed one gate. Nothing made the next gate follow the fix, because the pipeline has no type for a gate. This record makes the gate the unit that carries the lessons.

## Definitions

- **Gate:** a Go function, in a non-test file, that returns a verdict which blocks a commit, a landing or a phase handoff. A `_test.go` file that judges the real tree only calls a gate function.
- **Gate input:** a value that a gate reads. Its trust class comes from [gate-input-inventory.md](../gate-input-inventory.md).
- **Lane tree:** the tree that ship commits: the index plus each untracked file that `.gitignore` does not ignore (`git ls-files --cached --others --exclude-standard`).

## Decision

1. Each gate has one entry in a protected gate registry: name, kind, owners, inputs, stage and a run function.
2. The run function takes a `GateInput` with the root and the base commit. A diff-scoped gate diffs against that base, never against `HEAD`.
3. Each git fact of a gate comes from `internal/repostate`. A call-graph guard refuses git, `.git` reads and mtime reads.
4. The guard covers each function that a run function reaches. `repostate` reads the lane tree and `Base`, never `HEAD`, history, dates or mtimes.
5. A parity harness runs each gate on one fixture in three states with one base: unstaged, staged and committed. The verdicts must match.
6. Each artifact pattern has one owner phase. A gate on it runs at the handoff of that phase.
7. Each phase floor is one named check list. The host enforces it, and the self-check runs the same list.
8. A baseline holds keys of a body hash and a site. A site outside the baseline fails. Removed keys go at the boundary.
9. A baseline is an `authority` input on the protected surface. Only the boundary prune step of the console writes it.
10. A gate that rejects a lane reruns its red tests on the lane base. This holds at build, audit and ship.
11. Red on the base and green on main: the lane rebases onto main and runs again. The lane is not blamed.
12. Red on the base and red on main: the lane parks with its continuation, and the loop signals main-red.
13. While main is red, a landing path lands only a fix. A fix turns a red gate green and no gate red.
14. A main-red stop that lasts past its bound escalates through the ADR-0155 path. Parked cycles never fail silently.
15. A new gate starts at shadow. Only a held proposal or the operator moves a stage, and never back to shadow.
16. A gate file, the registry and each `authority` input join the protected surface in the commit that adds the gate.

## Refinements from the reviews

| Id | Finding | Decision that answers it |
|---|---|---|
| R1 | Base-red at ship only misses cycle 2242, which lost 333 minutes at the build floor. The lane base and main are not the same commit. | 10, 11, 12 |
| R2 | A set of body hashes misses a third copy of a helper that has two copies already. | 8 |
| R3 | A lane-writable baseline silences the gate, through `count` or `date`. | 9 |
| R4 | Phase floors and `phasecoherence` call git outside `repostate`, and some scanners live in `_test.go` files. An mtime read can move into `repostate`. | Definitions, 3, 4 |
| R5 | A registry with no run function cannot run a gate on a fixture. A gate that diffs against `HEAD` gives two verdicts by design. | 1, 2 |
| R6 | [gate-input-inventory.md](../gate-input-inventory.md) already classes gate inputs. A `verified` input such as `go/.apicover-enforce` stays lane-writable. | Relates to, 9, 16 |
| R7 | The TDD floor and the TDD self-check run different checks. | 7 |
| R8 | A main-red stop blocks the landing that repairs main. | 13, 14 |
| R9 | The registry `stage` is a new off switch, and a syntactic guard misses a helper package. | 1, 4, 15 |
| R10 | `testdup` at `b33d6492b` mixes the index with commit dates, mtimes and a `HEAD` diff. #1382 closed the escape and kept the gate non-hermetic. | 2, 4; plan G2 |
| R11 | A `repostate` that answers from `Base` only misses a staged new file. The index equals the tree of the commit, so it is parity-safe, and ADR-0084 I1 reads it. | 4; plan G2 and G3 |
| R12 | An index-only rule misses the untracked predicate files of TDD, so the TDD floor passes with no check. The lane tree adds the untracked files that ship commits. | Definitions, 4, 5; plan G2, G3, G5 |

## Consequences

- Good: a lesson about a gate holds for each later gate, because the harness tests each registry entry.
- Good: the verdict before the landing equals the verdict on main for the same content and base, so the train can trust a lane verdict.
- Good: a lane no longer fails on a red that it did not cause. Its work rebases, or it waits in a continuation.
- Bad: the git reads of the phase floors, `phasecoherence` and `fleet` move into `repostate`. The plan lists them (G2).
- Bad: a base rerun adds one run of the red tests. It runs only on a red verdict.
- Bad: the key baseline replaces the count in `go/.test-dup-baseline`. The predicates of cycle 2231 that read the count stop holding. CI runs only `./acs/regression/...` (`.github/workflows/ci.yml:69`), so they gate nothing today. G6 retires them with a note.
- What changes this decision: a parity failure that the harness cannot express. A base rerun that costs over 10% of the ship time of a wave also changes it.

## Rejected

| Option | Why not |
|---|---|
| Fix each gate when it fails | The record of ADR-0084, ADR-0113 and ADR-0117 shows that each fix stayed local (research F1.2, F1.4, F1.7). |
| A review checklist for new gates | Review caught three of the four protected-surface misses, and it missed the fourth (F1.7). A test does not tire. |
| Run every gate only on the merged tree in the train | The train driver runs at shadow (ADR-0147 decision 11), and a lane needs a verdict that it can fix before ship. Parity makes the two verdicts one. |
| Keep the count baseline and auto-merge its file | Betterer needs a dedicated merge tool for this conflict (research F2.3). A set of keys does not conflict on removals. |
| A set of body hashes as the baseline | It misses a new site of a body that the baseline holds (R2). The SARIF practice keys on the path too (F2.4). |
| A syntactic guard on the gate file only | A helper package hides the git call (R9). |
| Halt the loop on a red main | A halt waits for the operator. The Chromium tree reopens when the builder recovers (F2.6), so the stop ends on its own. |

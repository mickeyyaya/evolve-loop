# Plan: every deliverable has a registered static check and a registered authentic check

- **Status:** proposed, 2026-11-27.
- **Design:** [deliverable-verification](../architecture/deliverable-verification.md). **Decision:** [ADR-0172](../architecture/adr/0172-every-deliverable-has-a-registered-static-check-and-an-authentic-check.md). **Research:** [deliverable-verification-2026-11](../research/deliverable-verification-2026-11.md).
- This plan builds on [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md) for shadow stages and on [acs-predicate-quality-gate](../architecture/acs-predicate-quality-gate.md) for the quality of predicates.

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

The operator asked on 2026-11-27:

1. "Make sure we have static verification for all test deliverables"
2. "I mean all deliverables are required to be properly tested and examed with a static and authentic method"

The operator chose, the same day, the scope of the first guard: "Docs of record + ratchet". Here "ratchet" means two things: the shrink-only baselines of V1 and V4, and the ratchet files of class D14. The existing guards hold D14 (component R in section 6). The operator chose that the research-loop components C2, C3 and C4 are built in the console. The operator decided to build this work: "follow your recommendation and full speed to build the features".

A **static check** is a deterministic check that code runs: a Go test, a lint, a gate verb or a CI step. A reviewer LLM's probe or a builder's claim is not static. An **authentic check** truly exercises the deliverable:

- a new or changed test fails on the base and passes on the change (FAIL_TO_PASS), and the old tests still pass (PASS_TO_PASS);
- a mutant of the changed lines fails a test;
- a proof drives the production entry point;
- a cited test, `path:line`, id, link or number exists and matches.

## 2. Facts

| Fact | Research id |
|---|---|
| The commit gate has no check for a diff of only `.md` files, except a docs-review run record. | F1.1 |
| TDD passes on the heading `## RED Run Output`. No test runs. | F1.2 |
| The base-red floor excuses reds that the base has. No gate demands red on the base. | F1.3 |
| The mutation and revert libraries exist. No gate calls them. | F1.4 |
| V1 is built on branch `cl/docs-named-tests-exist`. A pending PR carries it. | F1.5 |
| 36 package-note lines cite a test that does not exist. About 20 percent of `path:line` cites in specs, incidents and research do not resolve. | F1.6, F1.7 |
| 957 ACS packages exist. 956 are outside their own cycle and never run again. | F1.8 |
| Several gates block only at `enforce`, or only warn. | F1.9 |
| Ship checks only the tree sha of the attestation. A "N/N PASS" line is text. | F1.10 |
| A shrink-only baseline is the proven way to add a guard with a backlog. | F1.11 |
| The outside practice splits tests into FAIL_TO_PASS and PASS_TO_PASS, repeats runs to drop flaky tests and mutates only changed code. | P2.1, P2.2, P2.5 |

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1 Each deliverable class has a registered static check and a registered authentic check. | Rows of the registry with an empty field | 0 |
| G2 A new test must fail on the base. | Added tests that pass on the base, without a marker | 0 (shadow: counted, then enforced) |
| G3 A changed function has a mutant that a test kills. | Changed functions with no killed mutant | 0 (shadow first) |
| G4 A doc of record cites only things that exist. | Names, paths and links that do not resolve, outside the baseline | 0, and the baseline shrinks |
| G5 A claim of a count or of a red is true. | Claims with no matching host record | 0 |
| G6 Old ACS packages run on a schedule. | Packages with no run in 30 days | 0 |

Non-goals:

- This plan flips no gate from shadow or advisory to enforce. Each flip is a question for the operator (section 10).
- This plan does not check that the content of a cited line says what the text says (research gap 1).
- This plan does not design the research-loop components C2, C3 and C4. It only orders them after V6.
- This plan does not replace the review fleet. The review stays as a second line.

## 4. Candidate designs

- **A. Patch each gate where it is weak, one at a time.** Add the missing check to the gate that is nearest to each gap.
- **B. One new end-to-end verification pipeline that replaces the existing gates.** A single verb runs all checks for a diff, and the old gates retire.
- **C. A verification registry with a meta-guard, then staged authenticity gates that reuse the existing guards (shadow first).** One data file maps each class to its checks. A meta-guard keeps the file true. Each new gate starts in shadow.

Scores run from 1 (poor) to 5 (best).

| Criterion | A. Patch each gate | B. New pipeline | C. Registry and staged gates |
|---|---|---|---|
| Finds a class that has no check (G1) | 1: a class with no gate is not seen | 4: the pipeline lists classes | 5: the meta-guard fails on a class with no check |
| Reuses the guards that exist (F1.1) | 4 | 1: it replaces them | 5: it names them by id |
| Risk of a false block on good work | 3 | 2: one big change | 4: shadow first |
| Time to the first value | 4 | 1: it needs a whole pipeline | 4: V0 and V1 land first |
| Cost to keep it true | 2: many private rules | 3 | 4: one file, one meta-guard |
| Fit with the rules of the repository (one home for each rule, shadow first, protected surface) | 2 | 2: a second controller | 5 |
| **Total (of 30)** | **16** | **13** | **27** |

**Why C wins.** A patch (A) leaves a class with no gate invisible. A class that nobody owns is the case that the operator named: "all deliverables". The pipeline (B) copies the guards that exist and adds a second place that must stay true. C keeps each guard where it is. C adds the one thing that is missing: a list that a test keeps honest.

**What it takes from each:**

| Candidate | What C takes | What C leaves |
|---|---|---|
| A | The idea to put a missing check next to its gate. V2, V3 and V5 plug into the commit gate and the build floor. | The one-by-one order, which has no list of classes. |
| B | The end-to-end view of a diff. The worked example in the spec follows one PR through the checks. | The new pipeline and the retirement of the old gates. |
| C | The registry, the meta-guard, the shadow stages and the reuse of the guards. | Nothing. |

## 5. Decisions

| Id | Decision | Because (research) | Rejected options and why |
|---|---|---|---|
| D1 | Build candidate C. | F1.1, F1.9 | A and B (section 4). |
| D2 | Build V0 first. It is one data file and a meta-guard. | F1.1 | Start with V2: V2 needs the list of classes to know where it applies. |
| D3 | A check in the registry is the id of an existing guard: a test name, a verb or a CI step. | F1.1, E1 | A new guard for each class: it copies a guard that exists. |
| D4 | Keep V1 as built and cite it as the first check of D4 and D5. | F1.5 | Rewrite V1: the code works and a PR carries it. |
| D5 | V4 checks existence of paths, lines, links, ids and named numbers. It uses a shrink-only baseline. | F1.7, F1.11, A9 | Check the content of a line: no outside practice and no measure (research gap 1). |
| D6 | V2 runs each added or changed test on the merge base with only the test file overlaid. It must fail there and pass on the change. It repeats the run. | P2.1, P2.2, P2.3 | A judge LLM: not static (A6). |
| D7 | An exempt test (a refactor or a characterization test) needs a marker and a reason, and the ledger counts it. | P2.3, P2.8, R4 | A free exemption: it defeats the check. |
| D8 | V3 reuses the `qualitypanel` mutation code on the changed functions. At least one mutant of each changed function fails a test. | F1.4, P2.5 | A whole-repository score: too costly (A5). |
| D9 | Each V-component starts in shadow with its own stage key, a named stage type and a gate registry seed. A flip to enforce is an operator question (section 10). | F1.9, ADR-0167 | Enforce at once, or block at landing: we have no flake measure, and ADR-0167 gives the flip to the operator. |
| D10 | V5 compares a claim with a run that the host recorded. | F1.2, F1.10, R5 | Read the claim with an LLM: not static. |
| D11 | V6 runs `-tags acs ./acs/...` on a schedule. Each package is promoted to regression or retired with a record. | F1.8 | Run the packages on each PR: too slow, and the failures are legacy. |
| D12 | List the gates that block only at `enforce`, or only warn, with their evidence. Flip none. | F1.9 | Flip the safe ones now: no shadow record backs a flip. |
| D13 | The design obeys its own rule. Each new deliverable has a static check and an authentic check in the spec class table. | F1.1 | A design that exempts itself. |
| D14 | Ratchet files (class D14) keep their existing guards, `sizeratchet.CheckAgainstBase` and the boundary ratchet tests. V0 registers them. | F1.11 | A new ratchet component: it copies a guard that exists. |

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| V0 | **Shadow first, stage key `verifyreg_gate`.** The registry: one data file maps each class (path globs) to its checks. Each check is the id of an existing guard. A meta-guard fails on a path with no class, a class with a missing check, or an id that does not exist. It reuses V1 for test ids. | A class with no authentic check fails. An id that does not exist fails. A path with no class fails. | Console (a protected surface) |
| V1 | Named tests in docs of record exist. Package `go/internal/testrefs` and `TestDocsOfRecord_EveryNamedTestExists`. The merge base is the authority of its baseline, and the baseline is an authority input in `gateregistry`. **Built, in a pending PR. The design PR lands after it.** | Built. | Console, PR pending |
| V2 | **Stage key `f2p_gate`.** FAIL_TO_PASS. Each added or changed test, and each cycle predicate, runs on the merge base with only the test file overlaid. It must fail there and pass on the change, in repeat runs. A counted marker exempts a refactor test. **Shadow first.** | A test that passes on the base fails the check. A flaky test is dropped. An unmarked exemption fails. | Console |
| V3 | **Stage key `mutation_gate`.** Changed-line mutation. It reuses `qualitypanel` mutation code on the changed functions. At least one mutant of each changed function fails a test. **Shadow first.** | A changed function with no killed mutant is reported. | Console |
| V4 | **Shadow first, stage key `cite_gate`.** Doc citations. Code checks `path:line`, relative links, finding and decision ids, and named numbers in docs of record. It runs in the STE lint job, with a shrink-only baseline. | A link to a file that does not exist fails. A `path:line` beyond the end of the file fails. A cited id that no document defines fails. | Console |
| V5 | **Shadow first, stage key `claims_gate`.** Claims. The TDD red evidence, and the "N/N PASS" lines of a build report or a commit body, must match a run that the host recorded. A heading in text is not evidence. | A report with "12/12 PASS" and a record of 11 tests fails. A report with a red claim and no red record fails. | Console |
| V6 | **Shadow first, stage key `acs_history_gate`.** Historical ACS packages. A scheduled run of `-tags acs ./acs/...`. Each package is promoted to regression or retired with a record. | A scheduled run that has a red package files an item. | Console, then lane for the triage |
| R | Ratchet files (class D14). Built: `sizeratchet.CheckAgainstBase` and the boundary ratchet tests. V0 registers them. | The existing tests. | Existing guards |
| Q | Operator questions. They are not built. See section 10. | - | Operator |

## 7. Order

1. **V0.** The registry gives each later component its place, and it finds the classes that have no check. It needs no base run.
2. **V1.** It is built. Its PR lands before this design PR, and V0 cites it for the test ids.
3. **V4.** It has a low cost and the miss rate is high (F1.6, F1.7). It needs no base run and no stage.
4. **V2, in shadow.** It closes the highest risk of the inventory (F1.2, F1.3). It needs the base overlay run, so it comes after the cheap checks have proved the registry.
5. **V3, in shadow.** The library exists (F1.4). It shares the scratch worktree of V2.
6. **V5.** It reads the run records that V2 and the gates write, so it comes after V2.
7. **V6.** It is a scheduled job, and it does not block a PR. Its triage is long, so it starts last.
8. **The research-loop components C3, C2 and C4**, in this order, after V6. The operator chose that the console builds them. This plan does not design them (research gap 6).

## 8. Verification

| Goal | Before | After | How measured |
|---|---|---|---|
| G1 | No registry. 15 classes, and no list of their authentic checks. | 0 rows with an empty field | The meta-guard runs in the repo-contract pack. |
| G2 | TDD passes on a heading (`go/internal/phases/tdd/tdd.go:72`). | Shadow record of each added test. Then enforce. | The Signal Center reading of the V2 stage. |
| G3 | No gate calls the mutation code (F1.4). | Shadow record for each changed function. | The Signal Center reading of the V3 stage. |
| G4 | 381 names in 117 docs in the V1 baseline at the time of the V1 PR. 36 "pinned by" lines point at no test. | A baseline that shrinks. No new miss. | The baseline file in git. |
| G5 | 4 of 1,442 commit bodies carry a "N/N PASS" line, and none is checked. | Each such line matches a record. | The V5 test on a fixture and on the real tree. |
| G6 | 957 packages, 956 outside their cycle, never run again. | A scheduled result for each package. | The scheduled job result. |

A test cannot prove that a mutant set is complete, or that a FAIL_TO_PASS run found every flaky test. The shadow period measures the flake rate before any enforce.

## 9. Risks

| Risk | Control |
|---|---|
| V2 blocks good work because a test is flaky. | Shadow first. A repeat run. A named stage type with chained evidence (ADR-0167). |
| V2 blocks a legitimate refactor test. | The marker with a reason. The ledger counts markers, and a rise is a signal. |
| The registry names a guard that a later change renames. | The meta-guard fails when a registered id does not exist. |
| A builder edits the registry to hide a gap. | The registry and the meta-guard are protected-surface rows. A cycle cannot edit them. |
| V4 floods the docs with failures. | A shrink-only baseline, like V1. |
| V3 is slow on a large diff. | Mutate only the changed functions. Cap the number of functions in shadow, and record the cap. |
| The scheduled V6 run is red for legacy reasons. | Triage each package: promote or retire with a record. |

## 10. Open questions

**Operator questions (not built).** The first table lists the stages of this design. Only a held proposal or the operator moves a stage to enforce (ADR-0167). The second table lists the existing gates that block only at `enforce`, or only warn. Each flip is the decision of the operator. The evidence is in the research.

| Question | State at start | Evidence |
|---|---|---|
| V0 to enforce (`verifyreg_gate`) | Shadow | The shadow record of M1 to M5 on real diffs |
| V2 to enforce (`f2p_gate`) | Shadow | The flake rate and the count of `green_on_base` verdicts |
| V3 to enforce (`mutation_gate`) | Shadow | The count of `lived` and `not_covered` verdicts |
| V4 to enforce (`cite_gate`) | Shadow | The count of new failing cites outside the baseline |
| V5 to enforce (`claims_gate`) | Shadow | The count of claims with no record |
| V6 to enforce (`acs_history_gate`) | Shadow | The count of packages with a result |

| Gate | State today | Evidence |
|---|---|---|
| Manifest gate | Shadow unless `manifest_gate: enforce` | `go/internal/phases/ship/manifest.go:51` |
| Strict audit | A WARN with exit code 1 and a "PASS" text ships, unless `strict_audit` | `go/internal/phases/ship/audit.go:40` |
| Evalgate | Blocks only at `enforce` | `go/internal/evalgate/reviewer.go:56` |
| Top-N gate drift, flaky-shape lint | Advisory | Research F1.9 |
| Docsfloor | WARN | Research F1.9 |
| Inbox dead paths | WARN | Research F1.9 |
| Coverage below 85 percent, apicover outside the enforce list | Warning | Research F1.9 |
| Ship attestation | Checks only `tree_state_sha` | `go/internal/phases/ship/commitgate.go:85` |

**Other open questions.**

1. The repeat count of V2 and its time limit. The shadow period gives the flake rate.
2. The cap on the number of functions that V3 mutates for one diff.
3. The definition of C2, C3 and C4. It is not in this tree.
4. The registry file name and its home. The spec proposes `go/internal/verifyreg/registry.json`. The console decides when V0 is built.

# Plan: a correction round stops on a proven blocker and sends the item to the console

- **Status:** built, 2026-10-19.
- **Design:** [blocker-exit.md](../architecture/blocker-exit.md). **Decision:** this plan (no ADR; the change adds an exit to the ADR-0117 floor loop). **Research:** §2 of this plan.

## Table of contents

1. Request
2. Facts
3. Goals and non-goals
4. Candidate designs
5. Decisions
6. Components
7. Order
8. Verification
9. Risks
10. Open questions

## 1. Request

The console gave the task on 2026-10-18: "there is no exit for a constraint conflict the builder cannot satisfy, so correction rounds are burned and the work is discarded." The task asks for a deterministic exit. The floor must verify the claim of the builder and never trust it. The outcome is "blocked: needs console", not a code FAIL.

A note from the coordinator on the same day added a rule: "keep its core phase-agnostic ... behind an interface or seam that takes the phase's request and output". The build floor is the first caller.

**Terms.**

| Term | Meaning |
|---|---|
| floor failure | One string that the build handoff floor returns. One string is one deterministic check that failed. |
| blocker claim | One failure defect in the `evolve-verdict` sentinel that starts with the word `BLOCKER`. |
| proven blocker | A claim that the host verified, where each floor failure names only the files of proven claims. |
| console route | The `route-console` path of the inbox. The item gets `route: console-manual` and the reason. |

**Worked example (cycle 1946).** The builder wrote `BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:138 <why>`. The file is on the protected-surface manifest. The lane did not change it. The two floor failures name only that file. Thus the round stops after the first rejection, and the item goes to the console with the diagnosis.

## 2. Facts

### 2.1 Local evidence

| Id | Fact | Source |
|---|---|---|
| F1 | Cycle 1946 sealed FAIL after 3 corrections. The only uncovered function was `writeBuildSelfCheckArtifact` in a protected file. | `.evolve/runs/cycle-1946/audit-fail-reason.json` |
| F2 | In rounds n8 and n9 the builder said that the one-line fix is in a file that the phase must not edit. The floor sent the same failure again. | `cycle-1946/build-report.md` §Correction 2 and §Correction 3 |
| F3 | Cycle 1950: `TestC1950_001` demands 100% on `AppendResolvedFingerprint`. `TestC1950_002` forbids any production edit under `go/internal/core`. The builder wrote a FAIL verdict with this conflict. | `cycle-1950/build-report.md:2`, predicates at `a6075085d:go/acs/cycle1950/predicates_test.go` |
| F4 | The floor printed `cover strict: internal/core: 100.0% < 100.0% FAIL`. The `%.1f` format rounded 99.97% up to the floor. | `go/internal/coverstrict/coverstrict.go` `measureFloor` (before this change) |
| F5 | The loop `reviewWithCorrections` had only two exits: approval, or the rung budget spent. | `go/internal/core/cyclerun_correction.go` |
| F6 | The TDD phase already has a console route for a predicate that reads a protected path. It sets a coded diagnostic, and `cycleoutcome` routes the item. | `go/internal/core/tdd_protected_route.go`, `go/internal/cycleoutcome/cycleoutcome.go` `routeRefusal` |
| F7 | `cycleclassify.Classify` lets the sentinel of the last phase outrank a coded record of the same phase. | `go/internal/cycleclassify/classify.go:62` |
| F8 | A FAIL verdict preserves the worktree and stamps the salvage snapshot. | `go/internal/core/cyclerun.go` `finalizeCycle`, `continuation_stamp.go` |

### 2.2 Outside practice (live WebSearch, 2026-10-18)

| Id | Practice | Source |
|---|---|---|
| P1 | LLMs cannot reliably self-correct without an outside signal. The bottleneck is error detection. | [Huang et al., "Large Language Models Cannot Self-Correct Reasoning Yet"](https://huggingface.co/papers/2310.01798); [Kamoi et al., critical survey](https://arxiv.org/pdf/2406.01297) |
| P2 | Self-evaluation is often overconfident or stubborn, so self-reported feedback needs a check. | [Self-Contrast](https://arxiv.org/pdf/2401.02009) |
| P3 | LLM agents are miscalibrated about when to escalate. An escalation decision must weigh the cost of an error against the cost of the escalation. | [Act or Escalate?](https://arxiv.org/pdf/2604.08588) |
| P4 | An agent workflow can abort when a plan is unrecoverable and escalate to a person as a fallback. | [SHIELDA](https://arxiv.org/pdf/2508.07935) |
| P5 | An extrinsic check of task state can decide an early exit and save many steps. | [Early exit for LLM agents](https://arxiv.org/pdf/2505.17616v2) |
| P6 | Practitioners cap retries and send persistent failures to a person, with explicit triggers. | [PromptLayer glossary](https://www.promptlayer.com/glossary/self-correcting-agent); [CoddyKit](https://www.coddykit.com/courses/learn_ai_engineering/human-in-the-loop-escalation-10645986) |
| P7 | A constraint solver explains "unsatisfiable" with a minimal unsatisfiable subset (MUS). The subset names the constraints that conflict. | [Liffiton and Sakallah, CAMUS](https://sun.iwu.edu/~mliffito/publications/jar_liffiton_CAMUS.pdf); [CPMpy MUS](https://cpmpy.readthedocs.io/en/io/api/tools/explain/mus.html) |

### 2.3 Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| P1, P2: do not trust self-report | Adopt | The host verifies each claim with deterministic checks (D2). |
| P3, P4: escalate to a person | Adopt | The console route is the existing escalation (F6). |
| P5: extrinsic early exit | Adopt | The check runs before each correction round (D1). |
| P6: retry cap | Keep | The rung budget stays the round cap. The blocker exit is a third exit. |
| P7: name the conflicting constraints | Adopt, in small form | A `predicate-conflict` claim names the two predicates and the path (synthesis). |
| An LLM judge for the claim | Reject | P1 and P2 show that a judge has the same weakness as the builder. |

## 3. Goals and non-goals

**Goals.**

1. A proven blocker stops the correction rounds at the first rejection that carries it.
2. A claim that the host cannot prove changes nothing. The lane still fails after the rounds.
3. The outcome is a distinct ending: `blocked-needs-console`, with the `BLOCKER_ROUTE` diagnostic.
4. The item goes to the console with the diagnosis, and the salvage snapshot stays.
5. The coverage text never shows a percent below the floor as the floor.
6. The floor text tells the lane when a block is in a protected file, or is a dead error branch.

**Non-goals.**

- The general request, build and check loop of the operator. This plan gives only its "proven blocker" exit.
- A proof that a branch is unreachable. The dead-branch note is a hint and gates nothing.
- New blocker kinds beyond the two in the task.

## 4. Candidate designs

| Criterion (weight) | A: trust the verdict class | B: host proves claims in the review loop | C: classify at closeout only |
|---|---|---|---|
| A lane cannot escape a real FAIL (5) | 1 | 5 | 4 |
| Rounds saved (3) | 5 | 5 | 1 |
| Reuses the console route (2) | 3 | 5 | 5 |
| Phase-agnostic core (2) | 2 | 5 | 3 |
| **Weighted total** | **30** | **60** | **39** |

- **A** adds a failure class such as `blocked`, and the loop trusts it. P1 and P2 rule it out.
- **C** lets the rounds run out, then reads the claim at closeout. F1 shows that the rounds are the cost, so C saves nothing.
- **B** wins. It proves the claim at each rejection and ends the loop at once. It reuses the TDD route pattern (F6).

## 5. Decisions

| Id | Decision | Source |
|---|---|---|
| D1 | The loop checks for a proven blocker before it spends each correction. | F5, P5 |
| D2 | The host proves each claim with file, manifest and predicate checks. It never trusts the text. | P1, P2 |
| D3 | A proof needs every floor failure to name only proven files, with no cut diagnostic. | synthesis |
| D4 | Only coverage and ACS predicate failures can be explained. A plain unit-test failure cannot. | F1, F3 |
| D5 | The core is phase-agnostic: `BlockerProver.Prove(BlockerCase) BlockerProof`. | coordinator note |
| D6 | The ending reuses the coded-refusal path: `BLOCKER_ROUTE` routes to the console. | F6 |
| D7 | `BLOCKER_ROUTE` outranks the sentinel of the lane, through `Disposition.OutranksSentinel`. | F7 |
| D8 | The ending is a FAIL verdict, so the salvage snapshot stays. | F8 |
| D9 | The coverage text adds decimals and the statement count when one decimal reads as the floor. | F4 |
| D10 | The prover, its build caller and the note writer join the protected-surface manifest. | ADR-0064 |
| D11 | The prover cuts each failure into items, and each item must name only proven files. | review round 1 |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| K1 | `coverstrict.belowFloor` | `TestMeasure_APercentJustUnderTheFloorNeverPrintsAsTheFloor` | console |
| K2 | `BlockerProver` and `BlockerRouteDiagnostic` (`blocker_proof.go`) | `TestBlockerProver_AClaimThatIsNotRealStillFails` | console |
| K3 | Build caller and loop exit (`build_blocker_exit.go`, `cyclerun_correction.go`) | `TestBuildBlockerExit_AProvenBlockerEndsTheRoundsAndRoutesTheItemToTheConsole` | console |
| K4 | Floor notes (`floor_blocker_notes.go`) | `TestBlockNoter_AProtectedUncoveredBlockSaysToDeclareABlocker` | console |
| K5 | Code, disposition and classify precedence (`cyclestate`, `cycleclassify`) | `TestClassify_AHostProvenBlockerOutranksTheLanesOwnFailSentinel` | console |
| K6 | Console route wiring proof (`cycleoutcome`) | `TestApplyFailure_AProvenBuildBlockerRoutesTheItemConsoleWithTheDiagnosis` | console |
| K7 | Manifest entries | `TestProtectedSurfaceManifest_CoversTheBlockerProof` | console |

## 7. Order

1. K1 first, because it is independent and fixes the misleading text.
2. K2, because K3 and K4 depend on its types.
3. K4, because K2 reads the uncovered-block lines that K4 writes.
4. K3, which joins K2 to the loop.
5. K5 and K6, which carry the ending to the inbox.
6. K7 last, when the file names are final.

## 8. Verification

| Goal | Measure | Before | After |
|---|---|---|---|
| 1 | Build dispatches on a proven blocker | 4 (1946: n6 to n9) | 1 (`TestBuildBlockerExit_AProvenBlocker...`) |
| 2 | A false claim escapes | not applicable | 0 of 16 refused cases escape (`TestBlockerProver_AClaimThatIsNotRealStillFails`, `...DoesNotExcuseAFailure...`) |
| 3, 4 | Item route after the ending | `failure_count` bump | `route: console-manual` with the reason (`TestApplyFailure_AProvenBuildBlocker...`) |
| 5 | Text for 3412 of 3413 statements | `100.0% < 100.0%` | `99.97% (3412 of 3413 statements) < 100.0%` |
| 6 | Notes on a protected or dead block | none | one note each (`TestBlockNoter_...`) |

## 9. Risks

| Risk | Control |
|---|---|
| A lane claims a protected file to skip a reachable branch. | The ending is not a PASS. The console sees the claim, and the work does not ship. The dead-branch hint must not gate (the request), so this risk stays. |
| A lane edits a predicate, or deletes a test, to make a false proof. | The host compares the predicates with a copy from the phase request. A test file that lost lines in the directory of the path refuses the claim. |
| A failed `git diff` reads as an empty diff. | The caller sends an evidence fault, and the prover refuses. |
| A bare file name in a failure matches a file of the same name in another package. | The claim path must exist and be protected or guarded. The risk stays small (synthesis). |
| A lane edits the floor code to fake the evidence. | D10 puts the prover and the note writer on the manifest. |
| The cycle counts as a FAIL for the halt counters. | This follows the TDD route, which also ends as FAIL (F6). |

## 10. Open questions

- The general loop of the operator can reuse `BlockerProver` for other phases. Each phase must supply its own `Explainable` function.
- A route for a cycle with more than one committed item. Today the route needs exactly one item, as the TDD route does.

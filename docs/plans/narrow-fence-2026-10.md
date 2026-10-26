# Plan: the loop works its own pipeline items behind a narrow fence and a held landing

- **Status:** proposed, 2026-10-27.
- **Design:** [narrow-fence-and-held-proposals](../architecture/narrow-fence-and-held-proposals.md). **Decision:** [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md). **Research:** [narrow-fence-2026-10](../research/narrow-fence-2026-10.md).

## Contents

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

## 1. Request

On 2026-10-27 the operator answered "1 + 2 + 3" to three proposals:

1. The console re-routes the lane-safe console-manual items at the boundary. New items default to `route: lane`.
2. Narrow the protected fence from whole packages to the files and symbols that enforce a guarantee.
3. A lane can work an item on a protected surface, or an item of kind `pipeline-repair` or `pipeline-integrity`. Its ship does not land by itself. It gets an extra adversarial audit and waits for the operator.

The operator stated the goal: "The evo loop must work its own unfinished pipeline tasks." Step 1 is console work at the boundary. This plan covers steps 2 and 3.

## 2. Facts

| Fact | Research id |
|---|---|
| 1 item of 258 is admissible on a copy of the live inbox. | F1.1 |
| 76 items are refused for a protected path, and about half of them (an estimate) fall on whole-package entries. | F1.2, F1.4 |
| The bridge guarantee lives in 18 files and 4 call sites of 94 files. | F1.5 |
| The repository already pins one call in an editable file. | F1.6 |
| A pin under `go/acs/regression/` runs at audit, before the landing. | F1.7 |
| The ledger, the policy parser and the phase-order state machine have no entry. | F1.8 |
| No path exists for a lane to work a pipeline item. | F1.9 |
| A named hold that only an approver removes is common practice. | P2.3 |
| A second party approves, and the approval binds to the exact change. | P2.4, P2.5, P2.8 |

## 3. Goals and non-goals

Goals, in order:

1. The fence keeps each guarantee of the constitution of ADR-0155.
2. A lane can work each pipeline item whose files hold no guarantee.
3. A change to a guarantee lands only when the operator approves the exact tree that the audit saw.
4. The hold is deterministic and visible. No lane can set or bypass the approval.

Non-goals:

- The plan does not move a guarantee out of the fence. It only removes entries that hold no guarantee.
- The plan does not dispatch the extra adversarial audit in PR 3. It records the requirement, and a held item builds the dispatch (section 6, H4).
- The plan does not change the console route of step 1.

## 4. Candidate designs

| Criterion (weight) | A: file fence and held landing | B: keep the package fence, add a per-item allow list | C: a second "pipeline lane" class with its own fence |
|---|---|---|---|
| Keeps each guarantee (3) | 3: each guarantee keeps a file entry or a pin | 2: an allow list is a hole in a gate package | 2: a second fence is a second list to keep right |
| Frees lane work (3) | 3: 25 items leave `console_protected` (section 8) | 1: each item needs a hand entry | 2: only items that the class names |
| Deterministic and checkable (2) | 3: a guard test names each gate file; pins run at audit | 1: the allow list has no oracle | 2: the class needs its own tripwires |
| Cost to build (1) | 2: one manifest change, one pin package, one gate | 3: small | 1: a new lane class in the wave, the ship and the routing |
| Matches outside practice (1) | 3: `per-file` owners and a named hold (P2.2, P2.3) | 1: no source | 2: close to a separate deploy pipeline |
| **Weighted score** | **29** | **15** | **19** |

Design A wins. It is the only design that frees work and keeps each guarantee behind an oracle. Design B moves the judgment into a hand list, which P2.6 names as the usual gap. Design C doubles the fence and the scheduler work for the same result.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | Each manifest entry names one of ten guarantee classes. | R1, F1.3 |
| D2 | Remove an entry only with the five-part evidence of R2. | R2, F1.4, F1.6 |
| D3 | Replace `/go/internal/bridge/` with its rule files and four symbol pins. | R3, F1.5 |
| D4 | The symbol pins live in `go/acs/regression/gatepins`. | F1.7 |
| D5 | Add the ledger, the policy parser, the state machine and the manifest file. | R4, F1.8 |
| D6 | A guard test names each protected file and its class. | F1.6, P2.1 |
| D7 | A held proposal gets a named landing state `held-for-operator`. | R5, P2.3 |
| D8 | Only the operator verb `evolve landing approve` writes an approval. | P2.4, P2.5 |
| D9 | An approval names the tree digest. A new tree voids it. | P2.5, P2.8 |
| D10 | The policy key `held_proposals.stage` starts at `shadow`. | ADR-0155 decision 17 |
| D11 | A held proposal needs an adversarial audit from another family at the deep tier. | R6, P2.7 |

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| N1 | The narrowed manifest with the class field | `TestProtectedGateFiles_EachKeepsItsEntryAndItsClass` | console (PR 2), a protected surface |
| N2 | The symbol pins of the bridge | `TestSandboxPinsHoldInLaneEditableFiles` | console (PR 2), a protected surface |
| N3 | The supply proof with fixture items | `TestSupply_TheNarrowFenceAdmitsNonGateFilesOfBridgeCoreAndTheInboxLifecycle` | console (PR 2) |
| H1 | The policy key and the hold decision | `TestDecide_*` in `heldproposal` | console (PR 3), a new gate |
| H2 | The landing check of the ship | `TestWorktreeShipLand_HoldsAProtectedChangeWithNoApproval` | console (PR 3), a protected surface |
| H3 | The operator verb, the list and the wave status line | `TestLanding_ACellCannotApprove` | console (PR 3) |
| H4 | The dispatch of the extra adversarial audit | an audit with a family of the builder is refused | held proposal, after H2 |
| H5 | The admission of a held item to a lane at `enforce` | a lane that claims a held item can edit its protected file | held proposal, after H4 |

## 7. Order

1. PR 1, the documents. The decision comes before the code.
2. PR 2, N1 to N3. A smaller fence frees lane work at once, with no new trust.
3. PR 3, H1 to H3 at `shadow`. The hold must exist and be proven before a lane can reach a protected file.
4. H4 and H5, as held proposals. They let a lane work a protected item, so they land after the `shadow` proof.

## 8. Verification

The census command is `evolve wave supply --json`, over the same read-only copy of the inbox.

| Measure | Before (`main`, `b7d54a934`) | After PR 2 (measured) |
|---|---|---|
| `admissible` | 1 | 3 |
| `waiting_cell` | 0 | 22 |
| `waiting_deps` | 1 | 2 |
| `console_protected` | 76 | 51 |
| `console_kind` | 46 | 46 |
| `console_route` | 134 | 134 |

The lane-routable supply is the sum of the first three rows: 2 before, 27 after. `admissible` counts only the items that one wave can run together. The freed items share the bridge and core cells, so the census holds them as `waiting_cell` behind one another.

The `shadow` record of PR 3 gives the second measure: the count of lane ships that the gate records as "would hold". The stage moves to `enforce` only after H4 and H5 land and an operator flip.

## 9. Risks

| Risk | Control |
|---|---|
| A freed file holds a guarantee that the evidence missed. | The gate-shaped tripwire, the call-site scanner and the pins still run. A missed guarantee becomes a pin or an entry by a manual ship. |
| A lane keeps a pinned call but ignores its result. | The pin is structural. The audit, the review and the held landing of a protected change are the next layers. |
| A lane writes an approval. | The approvals live under the protected `.evolve/landing/`. The verb is a class D verb. A guard test closes the set of callers. |
| An approval of one tree lands a later tree. | The approval names the tree digest. A different tree is not approved. |
| `enforce` with no audit dispatch holds each pipeline item forever. | The stage stays at `shadow` until H4 lands. |

## 10. Open questions

- **Q1** The class "kernel-loaded instruction" is not in the list of the operator. It keeps the skills, the personas, the schemas and the rule registry that the kernel loads into each agent. The plan keeps them protected. The operator can move them out by a new decision.
- **Q2** The explanation lifecycle call sites stay protected as a group. A later plan can replace them with pins, as D3 does for the bridge.

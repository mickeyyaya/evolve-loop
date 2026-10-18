# Plan: every phase handoff is a request, build and check loop with three exits

- **Status:** accepted, 2026-10-18 (the operator asked for the design and for its build through the loop waves).
- **Research:** [phase-handoff-contract-2026-10](../research/phase-handoff-contract-2026-10.md). **Decision:** [ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md). **Design:** [phase-handoff-contract](../architecture/phase-handoff-contract.md).
- **Decision numbers:** D1 to D12 of this plan are decisions 1 to 12 of ADR-0141, with the same numbers.

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

- **Phase pair:** two phases where the output of the first phase is an input of the second phase. This plan writes a pair as `A → B`.
- **Request:** the numbered asks that A gives to B. Each ask has an id, a text, a source and a kind of evidence.
- **Ledger:** the file that B writes next to its output. For each ask, the ledger gives a status and a pointer to the evidence.
- **Check:** the test of the ledger against the request. The deterministic part is Go code. The judge part is a model.
- **Exit:** the way that one loop of a pair ends: pass, cap or blocked.
- **Revision:** the message that a blocked exit sends back to A, with the ids of the asks that cannot be met.

## 1. Request

On 2026-10-18, after cycles 1946 and 1950 sealed FAIL, the operator asked: "Shouldn't TDD phase cover 100% coverage when the audit rejected the build result? How does issue—coverage shortfalls happened?" Then: "did we expose the deterministic checks and enough suggestions for build phase agent to learn and correct it's mistakes?" Then: "Who check and review the code to see if it fulfills the intent/requests from the previous phase?"

Then the operator gave the design direction: "I feel we should learn from the chain of evidence and each output should include the check to match the request/ask from the previous phase … Let's ultrathink to design the general/common rule with request - build - check between two phases". And: "I feel from phase a to b is actually doing request - build - check looping until it reach certain level of confident and continue move on the next b to c or c to a".

Last: "Make sure we document everything under docs/", "After plan is written, put this new design as top priority and build it through evo loop wave", and "make sure each wave including at least 4 lanes running concurrently".

## 2. Facts

| Fact | Research id |
|---|---|
| Cycle 1946 claimed an item whose own text says that its acceptance needs a protected file. | F1.1 |
| The builder of cycle 1946 named the conflict in round 2. The loop sent the same failure two more times, and the cycle sealed FAIL. | F1.2 |
| The tdd phase of cycle 1950 wrote two predicates that contradict each other. No check found it before the build. | F1.3 |
| The tdd phase and the builder ran the same model in both cycles. | F1.5 |
| Every deterministic check at a boundary compares ids or names, never the content of an acceptance entry. | F1.6 |
| The acceptance travels forward in the task contract, and nothing checks the output against it. | F1.7 |
| The audit chain checks each earlier phase, but runs in shadow and only at the end. | F1.8 |
| Three of five phase pairs have no check of the output against the ask. | F1.10 |
| Each check is its own design, with its own source for the ask. | F1.11 |
| `core.DeliverableReviewer` runs at every boundary and feeds a bounded correction loop. | F1.13 |
| A self-check without outside feedback usually makes the result worse. A check with a tool works. | P2.1, P2.2 |
| A checker of the same family shares the errors of the author. A weaker checker costs accuracy. | P2.3, P2.4 |
| Most of the gain of a refinement loop comes in rounds 1 to 3. | P2.8 |
| Agents are poor at a decision to stop on a request that cannot be met. | P2.11 |
| An attestation binds an output to its input by digest, and a verifier walks the chain. | P2.14 |

## 3. Goals and non-goals

### Goals

| Goal | Measure | Target |
|---|---|---|
| G1. Each phase pair has a check of the output against the request. | pairs with a check, of the 5 pairs in F1.10 | 5 of 5 |
| G2. A request that cannot be met ends its loop in one round, as blocked. | rounds spent on cycles 1946 and 1950, replayed | 1 each, not 3 |
| G3. "Confident" is measured, not stated. | the pass condition | each required ask met, with evidence that the check resolved |
| G4. The checker is independent of the author. | judge seats on the same model as their author | 0 at stage enforce |
| G5. One shared design. | private gate types and sources of the ask added | 0 new |
| G6. The audit validates the result against the first request. | audit chain links that decide | the deterministic links decide at stage enforce |

### Non-goals

- This plan does not replace the build handoff floor, the eval gate or the top-N gate. The checks stay. The new check reads their results as evidence.
- This plan does not sign the ledgers. All phases run in one trust domain (research §3).
- This plan does not add conformal calibration. The shadow stage collects the data for a later decision.
- This plan does not change the phase order or add a phase.

## 4. Candidate designs

### A. One new gate for each gap

Add a separate check for each gap in F1.10: a predicate gate for triage → tdd, a subset check for scout → triage, and so on.

### B. One handoff contract for every pair (the winner)

Each pair declares a request. B writes a ledger. One generic reviewer checks the ledger with deterministic rules first and a judge second. One decision function picks the exit. A blocked exit sends a revision to A. The audit walks the chain of ledgers.

### C. Promote the audit chain to enforce

Make the seven-link chain of the auditor (F1.8) decide the verdict. Add no other check.

### D. Signed in-toto attestations for each phase

Write a signed in-toto statement for each output, and verify the chain with an in-toto layout.

### Scores

Each criterion has a score from 1 (bad) to 5 (good), with its weight.

| Criterion | Weight | A | B | C | D |
|---|---|---|---|---|---|
| Finds a bad request before the build spends rounds (G2) | 3 | 4 | 5 | 1 | 4 |
| Covers all five pairs (G1) | 2 | 3 | 5 | 3 | 5 |
| One shared design (G5) | 2 | 1 | 5 | 4 | 4 |
| A deterministic check first (P2.1) | 2 | 4 | 5 | 1 | 4 |
| Cost to build | 1 | 3 | 3 | 5 | 1 |
| Lane-eligible work, for a 4-lane wave | 1 | 3 | 4 | 1 | 2 |
| **Weighted total** (out of 55) | | **33** | **53** | **26** | **42** |

B wins because it is the only design that checks each pair early, with code first, through one shared seam. A fixes the two cycles that failed, but adds a fifth source of the ask and repeats F1.11. C checks only at the end, after the build spent its rounds, and its check is a model. D gives the same chain as B, but a signature adds keys and no proof inside one trust domain. B keeps the digest of D, so a later move to D stays open.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | Each phase pair declares one request: numbered asks, each with an id, a text, a source and an evidence kind. | R1, F1.11 |
| D2 | The second phase of a pair writes a ledger: for each ask, a status, evidence pointers and the digest of the request. | R2, P2.14 |
| D3 | One handoff reviewer checks every pair. It plugs in at `core.DeliverableReviewer` and reuses the correction loop. | R10, F1.13 |
| D4 | The check runs the deterministic layer first. A judge runs only on asks that code cannot decide. | R3, P2.1, P2.2 |
| D5 | A judge is another family, or a stronger tier, than the author. The ledger records the identity of the checker. | R4, P2.3, P2.4 |
| D6 | A pass means that each required ask is met with evidence that the check resolved. A stated confidence never counts. | R7, P2.10 |
| D7 | Each loop has three exits: pass, cap and blocked. | R5, F1.2 |
| D8 | The cap is the existing redispatch cap. The loop also stops after a round that meets no new ask. | R6, P2.8, P2.9 |
| D9 | A blocked exit needs deterministic proof: an ask needs a protected path, or two asks conflict. A claim alone never blocks. | R8, F1.1, F1.3 |
| D10 | A blocked exit sends a revision to the phase that wrote the request, or to the console. The cycle does not seal FAIL. | R8, P2.11 |
| D11 | The audit walks the chain of ledgers and validates the result against the first request, the inbox acceptance. | R9, P2.16 |
| D12 | Each pair has its own stage, `shadow` or `enforce`, in `.evolve/policy.json`. Each pair starts at `shadow`. | F1.8, F1.13 |

## 6. Components

The code that makes and checks ledgers goes in a new package, `go/internal/handoff`. The package is new and is not a protected surface, so a lane can build it. The hooks into the protected core go to the console. With this split, at least 4 components are ready for a lane at each boundary until H7 is done.

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| H1 | `handoff` types: `Request`, `Ask`, `Ledger`, `Entry`, `Exit`; the ledger JSON schema; parse and validate | a ledger with an ask id that the request does not have is refused | lane |
| H2 | Request builders for each source: the inbox acceptance, the triage committed set, the tdd predicate list and the scout selection | the request for cycle 1946 lists its two acceptance entries with their source | lane (after H1) |
| H3 | Evidence resolvers: a `path:line` exists, a test name is declared, an ACS predicate exists, a command result is recorded | an entry that cites a test that no file declares does not resolve | lane (after H1) |
| H4 | The exit decision: a pure function of the request, the check results, the round history and the policy | a round that meets no new ask returns cap before the round limit | lane (after H1) |
| H5 | The blocker proof: an ask that needs a protected path, and two asks that conflict | the asks of cycle 1950 (`TestC1950_001`, `TestC1950_002`) return blocked with both ids | lane (after H1 and the blocker exit, branch `fix/build-blocker-exit`) |
| H6 | The `handoff` policy block: the pairs, the stage of each pair, the judge tier of each pair, strict decode | an unknown pair name is refused; every pair defaults to `shadow` | lane |
| H7 | The CLI: `evolve handoff check --cycle N --pair A-B [--json]` and `evolve handoff show --cycle N` | `handoff check` on the artifacts of cycle 1950 prints blocked and exits 1 | lane (after H1 to H4) |
| H8 | The handoff reviewer in the reviewer chain, at stage shadow; it writes `handoff-<pair>.json` | a cycle at stage shadow writes the result and keeps its verdict | console (`go/cmd/evolve/cmd_cycle.go`, `go/internal/core`) |
| H9 | The personas of triage, tdd and build write their ledger; the task contract gives the request to each phase | the tdd prompt holds the request block with ask ids | console (`agents/`, `go/internal/core/task_contract.go`) |
| H10 | The judge layer: a fidelity judge for each pair, through the bridge, on an independent seat | a judge seat on the same model as its author is refused at stage enforce | console |
| H11 | The audit chain reads the ledgers; its deterministic links decide at stage enforce | an unmet required ask in a ledger forces the audit to FAIL | console (`go/internal/auditchain`, `go/internal/phases/audit`) |
| H12 | Move each pair to `enforce`, one at a time, on its shadow evidence; first `triage-tdd` and `tdd-build` | the replay of cycles 1946 and 1950 ends blocked in round 1 | console (policy) |

## 7. Order

1. **The docs (this change):** the research, this plan, ADR-0141, the spec and the H1 to H12 inbox items.
2. **H1 and H6, with H5 when the blocker exit (branch `fix/build-blocker-exit`) merges.** They have no dependency between them. Each wave also takes the 25 lane parts from PR #978, so the wave keeps 4 lanes.
3. **H2, H3 and H4** after H1 merges. They use the types of H1 and do not depend on each other.
4. **H7** after H1 to H4. The CLI lets the operator replay old cycles before the wiring.
5. **H8 and H9** at the console. At stage shadow, the reviewer records and does not decide.
6. **H10 and H11** at the console, after two waves of shadow data from H8.
7. **H12**, one pair at a time, when the shadow data meets the condition in §8.

The order puts the pure code first. Thus lanes build most of the plan, and the console touches the protected core only for the hooks. The inbox items come with the docs, so no component waits without an item. That is the lesson of ADR-0129, where M2 to M8 had no item.

## 8. Verification

| Goal | Before (2026-10-18) | After | How it is measured |
|---|---|---|---|
| G1 | 0 of 5 pairs checked by content (F1.10) | 5 of 5 | `evolve handoff show` lists a result for each pair of a cycle |
| G2 | 3 rounds, then FAIL (cycles 1946 and 1950) | blocked in round 1 | replay with `evolve handoff check` on the run directories of both cycles |
| G3 | no pass condition on the asks | each required ask met with resolved evidence | the H4 test table; the ledger of each shipped cycle |
| G4 | tdd and build on one model (F1.5) | 0 judge seats on the model of their author | `evolve cli-routing explain` for each judge seat; the H10 refusal test |
| G5 | 2 private gate types, 4 sources of the ask (F1.11) | no new type and no new source | the H1 package is the one home; a guard test lists the callers |
| G6 | the audit chain is shadow (F1.8) | the deterministic links decide | the H11 test; `audit-chain-shadow.json` is no longer the only output |

**The condition to move a pair to `enforce` (H12):** two waves at shadow, where the check agrees with the audit verdict on each shipped cycle. Also, the console judges no blocked result of a shipped cycle as false.

**What a test cannot prove:** the deterministic layer proves that the evidence exists and runs. It does not prove that the evidence means what the ask means. The judge layer and the final validation in the audit cover that part. The judge is a model, so its result is evidence, not proof.

## 9. Risks

| Risk | Control |
|---|---|
| The ledger becomes a form that the agent fills with false evidence. | H3 resolves each pointer in code. A pointer that does not resolve is not met. The judge samples the met entries. |
| The extra checks and the judge cost tokens. | The deterministic layer costs no tokens. The judge runs only on asks that code cannot decide, and only at stage enforce. |
| A loop runs too long. | D8: the existing cap, plus a stop after a round with no new met ask. |
| A lane escapes a real failure with a false blocked claim. | D9: only deterministic proof blocks. |
| The waves run fewer than 4 lanes. | §6 puts 7 components in an unprotected package. The 25 lane parts of PR #978 fill the rest. The console checks `evolve wave status` after each launch. |
| A pair at stage enforce blocks good work. | D12: each pair starts at shadow and moves only on the §8 condition. The policy can move it back. |

## 10. Open questions

1. **The judge seat.** Only the claude and agy families are available. For a claude author, an agy deep seat (Gemini 4 Argon) is another family. For an agy author, a claude deep seat is another family. H10 decides the seat for each pair from the author of the cycle.
2. **The scout → triage pair.** Its request is the scout selection, and its check is a subset check with no judge. H2 builds it. H12 decides if it needs stage enforce.
3. **Conformal calibration.** The shadow data of H8 is the calibration set. A later plan decides if it uses that set.

# Plan: who fixes a defect after the owner fails twice

- **Status:** accepted, 2026-10-20. The operator chose design C over a direct fix by the finder.
- **Research:** [finder-fix-ownership-2026-10](../research/finder-fix-ownership-2026-10.md). **Decision:** [ADR-0146](../architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md).
- **Amends:** [ADR-0142](../architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md) and its spec [retry-ladder](../architecture/retry-ladder.md).

## Table of contents

- [1. The problem](#1-the-problem)
- [2. Candidate designs](#2-candidate-designs)
- [3. The winner and what it takes from the others](#3-the-winner-and-what-it-takes-from-the-others)
- [4. Decisions](#4-decisions)
- [5. Components and order](#5-components-and-order)
- [6. Verification](#6-verification)
- [7. Limits](#7-limits)

## 1. The problem

When the auditor finds defects in a build, the builder gets a repair round. A repair round fixes about 71% of the cases with a verdict (research F2.1). When it fails, a second round of the same builder adds little (F2.2, R10).

The operator proposes three steps:

1. The builder retries once with the findings.
2. The builder retries at the next effort level.
3. The finder, the auditor here, fixes the defects itself.

The finder knows where the defect is, and that knowledge is the strongest lever for a fix (R9). But the auditor is read-only (F2.3), and the judge must stay independent of what it grades (F2.4, R5, R8). The plan keeps the finder's knowledge and keeps an independent check.

## 2. Candidate designs

| Design | Summary |
|---|---|
| A. Owner ladder only | ADR-0142 as it is: the builder retries, then at more effort, then at the next tier. The auditor only grades. |
| B. The finder fixes directly | After the retry and the effort raise, the auditor edits the code itself. A second auditor grades the fix. |
| C. The finder writes the fix spec; a fresh owner applies it; another check verifies | After the retry and the effort raise, the auditor writes a fix spec with the exact location and a suggested patch. A fresh builder session applies it. The deterministic gates and an auditor instance that did not write the spec verify it. |
| D. A builder of another family | After the owner ladder, a builder of a different model family takes the task. |
| E. Parallel candidates and a critic | Several builders try at once, and a critic picks one. |

Each criterion scores 1 (poor) to 5 (good). "Fixes the stuck defect" counts twice, because it is the purpose of the change.

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Fixes the stuck defect (×2) | 2 (4) | 4 (8) | 5 (10) | 4 (8) | 5 (10) |
| Keeps an independent check | 5 | 1 | 4 | 5 | 4 |
| Cost | 4 | 4 | 3 | 2 | 1 |
| Change size | 5 | 2 | 3 | 3 | 1 |
| Fits the sandbox and the ADRs | 5 | 1 | 5 | 3 | 2 |
| **Total (of 30)** | 23 | 16 | **25** | 21 | 18 |

**C wins.**

- A keeps retrying the owner that already failed twice, and that adds little (F2.2, R10).
- B gives the auditor write access to the code (F2.3), and the auditor then grades its own fix (R5, R8). No surveyed practice lets the finder both write and approve a fix (R1 to R5).
- D gains diversity (R11) but loses the finder's location, which is the strongest lever (R9).
- E costs several builds for each defect.

C follows the pattern of Gerrit, Copilot and CodeRabbit (R1 to R3). The reviewer writes the fix, and another party applies it. A check that did not write the fix approves it.

## 3. The winner and what it takes from the others

| From | What C takes | Why |
|---|---|---|
| A | Rung 2 raises the effort of the owner in a fresh session. The classifier of ADR-0142 still runs first, so a spec failure never climbs. | The operator's second step; a fresh context counters the decay of R10. |
| B | The fix spec holds the exact replacement lines, as Gerrit's "suggest fix" does (R1). Sometimes a deterministic check found the defect, and the patch stays inside the cited lines. Then the harness applies the patch as written and runs the same check again. | The speed of a direct fix where a check, not a judge, decides. |
| D | The fresh builder of rung 3 runs on a different model family when that family has quota headroom. | Different agents fix different defects (R11). |
| E | Not taken now. The fix spec of rung 3 can later go to two builders at once. | The cost is too high for one defect today. |

### The rungs after this change

| Rung | Who acts | What it gets | Who verifies |
|---|---|---|---|
| R0 | the builder | the task | the floor, then the auditor |
| R1 | the builder, fresh session | the auditor's findings | the floor, then the auditor |
| R2 | the builder, fresh session, effort +1 | the findings and the R1 diff | the floor, then the auditor |
| R3 | a fresh builder, another family if it has headroom | the auditor's fix spec | the floor, the tests of the tdd phase, and an auditor instance that did not write the spec |
| Stop | the operator | the dossier | — |

- **Renumbering of ADR-0142.** ADR-0142 had R1 = a fresh session with the digest and effort +1, and R2 = a fresh session with the digest and a tier step. Here R1 keeps the same tier and effort, R2 is the old R1 (effort +1), and R3 is new. The old R2 tier step survives only as the fallback of R2.
- When the family has no higher effort, R2 takes one tier step instead, as the old R2 of ADR-0142 does. When it has neither an effort step nor tier headroom, the ladder skips R2 and goes to R3.
- When the fingerprint of R1 repeats, the ladder skips R2 and goes to R3.
- **Round count and ADR-0126.**
  - ADR-0126 counts rounds after the first judgment (J_0): rounds 1, 2 and 3, and "no round 4".
  - R0 is the first build and its J_0. R1, R2 and R3 are rounds 1, 2 and 3. The ladder has four builder dispatches and three repair rounds.
  - This keeps the limit of ADR-0126. R3 also matches its rung 2, where round 3 changes the strategy: here a new author and the finder's spec.
  - The verify audit of R3 is a judgment, not a round. After R3 the ladder stops (ADR-0146, item 8).
- **Wording amended in ADR-0142.** Its "at most three tries" becomes "at most four tries (R0 to R3)", and its item 5 (R2 fails with an earlier fingerprint becomes `spec`) is replaced by the stop after R3.
- The repair cap today is two rounds (`attempt 1/2` in `ORCHESTRATOR_AUDIT_REPAIR_GRANTED`). F2 raises it to three, which is the cap of ADR-0126.

## 4. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | After R2 fails, the finder writes a fix spec: the location, the expected behavior and the replacement lines. It writes the spec into its own cycle workspace. | R9, F2.3 | The finder edits the code: B. |
| D2 | A fresh builder session applies the fix spec. It runs on another family when that family has headroom. | R10, R11 | The same builder session: R10. |
| D3 | Three checks verify R3: the floor, the tdd tests that the fixer cannot change, and an auditor instance that did not write the spec. The verifying auditor is on another family than the R3 builder, or at a stronger tier (ADR-0141, item 5). The auditor seat is Claude only, so when R3 runs on Claude the builder stays below the auditor's tier. | R5, R8, F2.4, ADR-0141 | The spec author grades the fix: R8. |
| D4 | The ledger records the author of the spec and the author of the fix for each diff. A PASS from the spec author on that diff does not count. | R5 | Trust the order of the phases: it does not prove who wrote what. |
| D5 | A fix of R3 may touch only the files and lines that the spec names. A wider diff fails the verify. | R5 | Any change: the fix can hide new work. |
| D6 | A deterministic finding with a patch inside the cited lines needs no builder. The harness applies the patch and runs the same check again. | B's speed, R4 | A builder for each mechanical fix: slower, same result. |
| D7 | Each rung records which defect it fixed. A rung that fixes nothing over 30 cases is removed. R2 goes first, since it has no outside evidence (R12). | R12 | Keep every rung: the effort raise has no outside evidence. |
| D8 | Later, outside F0 to F6: the rule applies to every phase pair of ADR-0141. The checking phase is the finder, and the producing phase is the owner. | ADR-0141 | Only audit and build: the same problem exists for tdd and triage. |

### Consequences

- Good: a defect that survives two builder rounds gets a fix from the phase that knows where it is.
- Good: the auditor stays read-only, and no fix is graded by its author.
- Bad: R3 costs one more builder session and one more audit.
- Bad: an auditor instance of the same family can share a blind spot with the spec author (R11). The deterministic gates carry part of that check.

### What changes our mind

- R3 fixes fewer than one in three of the defects that reach it over 30 cases. Then D (another family) replaces R3.
- R2 fixes no defect over 30 cases. Then R2 goes, and R1 goes straight to R3.

## 5. Components and order

| # | Component | Home | Route |
|---|---|---|---|
| F0 | The fix spec format and its reader, with the scope guard of D5 | `go/internal/retryladder` | lane |
| F1 | The auditor writes a fix spec on request (persona and contract) | `agents/evolve-auditor.md`, the audit contract | console |
| F2 | Rung R3 in the ladder decider: the fresh builder, the family choice, the skip of R2 | `go/internal/retryladder` | lane |
| F3 | The verifier of R3: the floor, the tdd tests unchanged, and a second auditor instance | `go/internal/core` | console |
| F4 | The ledger records of D4, and the refusal of the spec author's PASS | `go/internal/core`, the ledger | console |
| F5 | The fast path of D6 | `go/internal/core` | console |
| F6 | The rung records of D7 in `evolve ladder report` | `go/cmd/evolve` | lane |

F0 and F2 extend the retry ladder of ADR-0142, so they follow its L1 and L2, which have shipped.

## 6. Verification

- **Rungs:** a decider test replays the recorded runs of cycles 1972 and 1983 (F2.2). It expects R1, R2, then R3 (red test first).
- **Scope:** an R3 fix that touches a file outside the spec fails the verify.
- **Independence:** a PASS from the spec author on the R3 diff does not count, and the second auditor instance has a different session id.
- **Tests:** an R3 fix that edits a tdd test fails the verify.
- **Fast path:** a deterministic finding with a patch inside the cited lines applies the patch and runs the check again, with no builder dispatch.
- **Records:** each rung writes which defect it fixed, and `evolve ladder report` counts them.

## 7. Limits

- The auditor seat runs Claude Code only (`claudeFamilyFloor`), so the second auditor instance has the same family as the spec author. The deterministic gates carry the part of the check that the family cannot. Against the R3 builder, D3 keeps the rule of ADR-0141: another family or a lower builder tier.
- The scope guard of D5 can fail a fix that needs a caller or a test change outside the cited lines. The ladder then stops.
- A fix spec can be wrong. The verify catches a wrong fix, but the ladder then stops and the operator decides.
- The local data has 24 repair rounds. The rates of §1 are a shape, not a measure.

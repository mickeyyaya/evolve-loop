# Plan: the host lineage record decides what a cycle may do to the inbox queue

- **Status:** proposed, 2026-11-05. PR 1 is built and in review on the branch `feat/one-trust-root-for-the-queue`. PR 2 to PR 4 are not built. No code of this design is on `main`.
- **Design:** [queue-trust-root](../architecture/queue-trust-root.md). **Decision:** [ADR-0160](../architecture/adr/0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md). **Research:** [queue-trust-root-2026-11](../research/queue-trust-root-2026-11.md).
- **Replaces:** the design of [PR #1290](https://github.com/mickeyyaya/evolve-loop/pull/1290). The terms of this plan (pin, host record, cell, breaker, brake) are defined in the design file.

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

## 1. Request

The original problem: a system-level failure before build never raises `failure_count`. So each wave claims the same item again. Cycles 2133 and 2151 both failed on one item with `TRIAGE_COMMITMENT_INVALID`.

The operator, 2026-11-05:

- "Ultrathink to redesign the solution".
- "For those tasks been blocked over 3 times, we need to escalate it to redesign the solution from architecture perspective with ultrathink".
- "Make sure we have research, design, plan, adr, ...etc docs written in strict format described under docs/".

## 2. Facts

| Fact | Research id |
|---|---|
| Six review rounds blocked the work: five on two designs, one on PR 1 of the third design. | F1.1 |
| Four agent-writable files give a cycle authority over the queue. The sandbox pin on one of them is off at the default stage `shadow`. | F1.2 |
| 61 lines in 29 files of production code read the pin or the decision file. The console counted about 30 sites. | F1.3 |
| The host already writes `.evolve/lineage/cycle-N.json` before the first dispatch, and the isolation spec already trusts it for lineage. | F1.4 |
| A resume lets the file win over the value of the host. | F1.5 |
| `inputindex` imports `committedset`, so the record type must live in `committedset`. | F1.6 |
| Six sites route an item to the console, and two reason strings embed agent text. | F1.7 |
| The brake of #1290 read `LastCycleNumber` and a file that a failure before build does not write. | F1.8 |
| The PASS path of post-ship still reads `top_n`, the closes list and the superseded list. | F1.9 |
| Host writes into the workspace can follow a symlink that a cell planted. | F1.10 |
| A damaged or unwritable record counts as "no record", and the reader then trusts the file. | F1.11 |
| The guard of PR 1 missed 6 of 8 planted read forms. | F1.12 |
| Four sandbox rows passed on the real sandbox on 2026-11-05. | F1.13 |
| A cycle with no pin takes its ids from `top_n`. | F1.14 |
| No source found on an agent-written file as a control plane. | P2.7 |

## 3. Goals and non-goals

**Goals**

| Goal | Measure |
|---|---|
| G1. No file in a workspace gives authority over the queue | Readers of the pin file outside `committedset` and a short list: 0 (guard test) |
| G2. A forged pin changes no item | A real cycle with a forged pin leaves `b` and `c` with identical item files |
| G3. A cell cannot write the record, and a host write cannot be aimed at it | The four contract rows pass; a planted symlink changes nothing |
| G4. A damaged or unwritable record never gives the file authority | The probes of F1.11 give a signal and a stopped dispatch |
| G5. The PASS path moves only items of the record | A probe with `top_n` `[a, b, c]` and the record `[a]` promotes only `a` (after PR 3) |
| G6. A route reason holds host text only | An injection string never reaches `routed_reason`, the ledger or a signal |
| G7. A shared fault does not park the queue | The brake fires on the real closeout with `state.json` as `finalizeCycle` leaves it |

**Non-goals**

- This plan does not flip `isolation.i5.stage` to `enforce`. The design must hold at `shadow` (F1.2).
- This plan does not change the sequential loop. A cycle with no pin keeps the behavior of `main` (F1.14).
- This plan does not sign any file.
- This plan does not change the retry ceiling in `policy.json`.

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Patch each reader | Validate the ids at each of the 30 or more readers. |
| B. Pin in memory | Pass the assignment as a value from the orchestrator to the closeouts. |
| C. Restore at cell exit | Rewrite `lane-scope.json` from the host value when each cell ends. |
| D. Flip `isolation.i5.stage` | Let the sandbox deny the write of `lane-scope.json`. |
| E. Sign the workspace file | The host signs the pin file, and each reader checks the signature. |
| F. One trust root | The host record decides. One reader serves all consumers. |

The criteria and their scores follow. Each score runs from 1 (poor) to 5 (good). "Closes the class" counts twice, because five rounds failed on it. The scores are synthesis, and the basis follows the table.

| Criterion | A | B | C | D | E | F |
|---|---:|---:|---:|---:|---:|---:|
| Closes the class of a future reader (x2) | 1 (2) | 1 (2) | 2 (4) | 2 (4) | 3 (6) | 5 (10) |
| Reaches a reader in another process | 3 | 1 | 1 | 5 | 4 | 5 |
| Needs no new state or key | 5 | 5 | 5 | 5 | 1 | 5 |
| Few sites to change | 1 | 1 | 2 | 5 | 3 | 4 |
| Holds while the stage stays `shadow` | 1 | 4 | 3 | 1 | 4 | 5 |
| **Total (of 30)** | **12** | **13** | **15** | **20** | **18** | **29** |

Basis of the scores:

- **Class:** A and B protect only the readers that they reach (F1.3). C holds only on a path that restores the file (F1.1, round 2). D protects a path, not a meaning (F1.2). E closes the class only if each reader calls one verifier. F has one reader and a guard.
- **Other process:** B cannot reach the overlap check of a peer lane (`go/internal/core/lane_scope_assembly.go:71`). C cannot act while a cell runs.
- **State:** E needs a key to keep. The record of F exists (F1.4).
- **Shadow:** D needs `enforce`, which is off (F1.2) and has a named blocker.

**F wins.** The criterion that decided is the first: it is the only design that closes the class with no new state. D ranks second because it is cheap, and it fails the last criterion. The score of F is not a proof. F1.9 to F1.12 show that F alone does not close the PASS path, the host writes, the damaged record and the guard. Decisions D14 to D18 and D9 close them.

What F takes from the others: from D, the sandbox deny (D5) as a second layer. From B, the host value that wins at the start of a run (D3). From E, the idea that a reader must check a file before it trusts it (D15).

## 5. Decisions

Decisions D1 to D13 are the ids of the first draft of this plan. D14 to D18 come from the review of PR 1 (F1.9 to F1.12).

| Id | Decision | Research id |
|---|---|---|
| D1 | `committedset.LanePin` reads the host record. With no record at all, it reads the file. | F1.2, F1.4 |
| D2 | `core.LaneScopeIDs`, `loadLaneScope` and `readLanePin` call `LanePin`. Each other reader calls it too. | F1.3 |
| D3 | The host value wins at the start of a run. A new run of a cycle keeps the ids of its record. | F1.5 |
| D4 | When the file differs from the record, the host keeps the record and emits one violation signal that names both lists. | F1.4 |
| D5 | The sandbox grant check and the role guard deny a cell write to `.evolve/lineage`. A contract test proves it. | F1.13 |
| D6 | `CommittedIDsFor` goes away. The closeouts call `committedset.Committed`. | F1.3, F1.9 |
| D7 | Triage routes a protected card when it detects it, if the card is in the pin and the item names the path. | F1.1 |
| D8 | `RouteConsole` takes a typed reason. Only the operator verb can pass free text, through a constructor that says so. | F1.7 |
| D9 | `lifecycle.Mover` refuses an act of a cycle on an item outside its record. The PASS path of post-ship takes its ids from the record. | F1.1, F1.9 |
| D10 | The breaker charges each failure before build, except the host fault codes of a `RefusalDisposition` field. | F1.8, P2.5 |
| D11 | The cause key is the last phase and `SystemFailure.Category`. | F1.8 |
| D12 | When `prebuild_brake_items` distinct items hold one cause inside `prebuild_brake_window_cycles`, nothing parks, and one signal goes out. The first values are 3 and 15. | F1.8, P2.5 |
| D13 | The continuation goes only after the park succeeds. | F1.1 |
| D14 | A record that cannot be written for a cycle with a fleet scope stops the dispatch with a named failure. No cell starts. | F1.11 |
| D15 | Only a missing record reads the file. A damaged record gives no authority, raises a signal and stops the cycle. | F1.11 |
| D16 | A host write into a workspace refuses a symlink. The record has the mode 0444. | F1.10 |
| D17 | The guard fails on any reference to the pin file name or constant outside a short list. | F1.12 |
| D18 | The drift check runs on each exit path of a dispatch, also when a phase fails. | F1.1 |

**What each decision rejects, and the fact that rules it out**

| Id | Rejected option | Fact |
|---|---|---|
| D1 | Always read the file | A cell can write it (F1.2) |
| D2 | Keep three readers | This is how the fault spread (F1.3) |
| D3 | Let the file win | A forged file reaches the record (F1.5) |
| D4 | Stay silent | The operator loses the evidence |
| D5 | Trust the path alone | Cycle 2157 showed that a shell write passes the tool guard (draft plan; I did not verify this) |
| D6 | Keep `CommittedIDsFor` | It reads `top_n` with no pin (F1.9) |
| D7 | Route at the closeout | A closeout needs a record that a failure before build does not leave (F1.8) |
| D8 | A check at each caller | This repeats the fault (F1.7) |
| D9 | A check in each consumer | This repeats the fault. The sink is one place (F1.1) |
| D10 | Skip an empty code | A real scout failure has one |
| D11 | The audit fingerprint | It is empty before build (F1.8) |
| D12 | The last PASS from `state.json` | Each verdict advances it (F1.8) |
| D13 | Shed the continuation first | A braked item loses its salvage |
| D14 | A WARN and start the cell | The cell can write the file during the fault (F1.11) |
| D15 | Treat a damaged record as absent | A damaged record shows that a cell may have run (F1.11) |
| D16 | Deny only the cell write | A host write follows a link that the cell planted (F1.10) |
| D17 | A guard on call shapes | It missed 6 of 8 forms (F1.12) |
| D18 | Signal only on a normal exit | A cell that forges the file and aborts raises no signal (F1.1, round 6) |

## 6. Components

The route follows the rule of the operating policy: a fix to the trust kernel starts in a worktree and gets dual review ([operating-policy](../operations/operating-policy.md)). The owner of each component is the console.

| Id | Component | Red test first | Owner |
|---|---|---|---|
| K1 | The record type and `LanePin` in `committedset`, with a validity check (D1, D2, D3, D6, D15) | A real cycle with a forged pin, through the real claim, closeout and overlap check: `b` and `c` keep identical item files | console |
| K2 | The drift signal on each exit path (D4, D18) | A cell forges the file and the phase fails: one signal names both lists | console |
| K3 | The deny and the contract rows (D5) | The sandbox contract: a cell write to the record fails, on the real sandbox | console |
| K4 | The record-write failure and the symlink refusal (D14, D16) | A blocked `.evolve/lineage` stops the dispatch. A planted link changes nothing | console |
| K5 | The reference guard (D17) | The 8 planted forms of F1.12: each one fails the guard | console |
| T1 | The route at triage (D7) | A protected card routes during triage, and the ledger and the signal center receive it | console |
| T2 | The typed reason (D8) | The compiler refuses a string reason. An injection with no spaces never reaches a sink | console |
| M1 | The mover refusal and the PASS path (D9) | The record `[a]` and `top_n` `[a, b, c]` promote only `a`. A mover act outside the record fails and emits a signal | console |
| B1 | The breaker (D10, D11, D13) | A real scout failure charges the pinned item. A braked item keeps its continuation | console |
| B2 | The brake (D12) | The real closeout fires the brake, with `state.json` as `finalizeCycle` leaves it | console |

Each fix needs a red test first, a journey test through the real entry point, dual review and a wiring proof. The steps and rule 5 of §1 of `docs/operations/operating-policy.md` hold these. The six proof rows are in the [pipeline blockers plan](pipeline-blockers-2026-11.md).

## 7. Order

Each pull request starts from `origin/main`.

| PR | Scope | Why this order |
|---|---|---|
| 1 | K1 to K5 (D1 to D6, D14 to D18): the trust root, its guard, the deny and the failure paths. | It closes the authority of the pin file. It does not close the agent lists on the PASS path. |
| 2 | T1 and T2 (D7, D8): the route at triage and the typed reason. | The compiler lists each route site. |
| 3 | M1 (D9): the mover refusal and the PASS path. It closes the inbox item `promote-inbox-reads-agent-lists-outside-the-lane-pin`. | It needs a true record, so it follows PR 1. |
| 4 | B1 and B2 (D10 to D13): the breaker and the brake. It closes #1290. | The breaker needs PR 1, or it charges forged ids. |

**A correction.** The draft of this plan said of PR 1: "It closes the class. Nothing else needs to change for this to hold." That sentence is false. The review of PR 1 found that `committedInboxIDs` (`go/internal/phases/ship/postship.go:261`) still reads `top_n`, so a forged `top_n` promotes items outside the record (F1.9). The correct statement is:

- PR 1 closes the authority of the pin file. After PR 1, no reader trusts `lane-scope.json` when a valid record exists.
- PR 3 (D9) closes the agent lists on the PASS path: `top_n`, the closes list and the superseded list.
- Until PR 3 lands, a forged `top_n` can promote an item of another lane on a landed PASS. The Limits section of the design says this.

The inbox item `promote-inbox-reads-agent-lists-outside-the-lane-pin` is named in the draft. I did not find it in this worktree, so I did not verify it.

The earlier work is saved at `backups/session-2026-11-04/` (outside the repository), in `pr1290-round3-staged.patch` and `queue-authority-pr1-round2.patch`. A builder can port tests from them.

## 8. Verification

| Goal | Before (2026-11-05) | After | How |
|---|---|---|---|
| G1 | 61 lines in 29 files read the pin or the decision file (F1.3) | One reader, and 3 listed uses: the write, the sandbox pin and the messages | The reference guard test (K5) |
| G2 | `b` and `c` get a continuation stamp from a forged pin (round 2, CRITICAL 1) | `b` and `c` keep identical item files | The journey of K1 |
| G3 | 4 contract rows pass on the branch (console run, not rerun by me) | The same 4 rows pass in CI, and the symlink test passes | `TestAgentSandboxContract`, test of K4 |
| G4 | A damaged record gives `LanePin` = `[b c]` (F1.11) | A signal, a stopped cycle, and no authority from the file | Probe of F1.11 as a test (K1) |
| G5 | The probe returns `[a b c]` (F1.9) | `[a]` | Test of M1, after PR 3 |
| G6 | The probe string reaches `routed_reason` (F1.7) | It does not | Test of T2 |
| G7 | `brakeSignals=0 breakerSignals=3` (F1.8) | One brake signal and 0 parked | Journey of B2 |

The goal of the whole plan: the next review of each PR has 0 CRITICAL findings of the class "an agent file decides". Round 7 is the test of this.

**What a test cannot prove.** The guard proves that no production code names the pin file outside the list. It cannot prove that no code reads an agent id from another file. G5 and the other sinks need their own tests. A mutation sweep proves that a test kills a change. It does not prove that the design has no other writer of a file in a workspace (F1.10 lists about 60 calls that nobody classified).

## 9. Risks

| Risk | Control |
|---|---|
| Another host write follows a link that a cell planted | D16 uses one helper and a guard. Open question Q3 asks for the classification of the 60 calls. |
| The reference guard blocks a legitimate new use | The list has three entries, and a change to the list needs a reviewer. |
| A damaged record stops a good cycle | The stop is loud, and one signal names the cause. An operator can clear the record. This is the safe direction. |
| Between PR 1 and PR 3 a forged `top_n` promotes an item on a landed PASS | The Limits section says so. The time between the two PRs is short, and PR 3 follows PR 1 at once. |
| The brake delays a real poison item | Three real poison items with one cause inside the window brake. They keep their retries until an operator reads the signal. This fails in the safe direction. |
| The first brake values (3 and 15) are wrong | A false brake in two consecutive waves moves the refusal code into the key. |
| A test fixture with no record keeps trusting the file | This is intended (D1). No cell has run in a fixture. |

## 10. Open questions

| Id | Question | Our proposal |
|---|---|---|
| Q1 | Is a cycle number ever reused with another assignment? The code reviewer did not confirm it. | State in D3 that numbers only go up, and add a test. If a number is reused, D3 emits a signal that names both lists. |
| Q2 | `SystemFailure` can be nil on a verdict FAIL. Then the key of D11 is the last phase alone. | PR 4 measures this on real closeouts. |
| Q3 | Which of the 60 non-test calls of `os.WriteFile`, `os.Create` and `os.OpenFile` write into a workspace through a name that a cell chooses? | A builder classifies them in PR 1 round 2, and D16 covers each one. Unanswered today. |
| Q4 | Does PR 3 intersect the agent lists with the record, or does it drop them? | Intersect first. Drop a list when no test needs it. |
| Q5 | Does a flip of `isolation.i5.stage` to `enforce` still add value? | Yes, as a second layer. It is not part of this plan. |

**What changes our mind.** If the contract test shows that a cell can write `.evolve/lineage`, the record moves to a path that the sandbox denies. The decision stays. If round 7 finds a new file in a workspace that gives authority, we revisit D1. We then ask if the record itself needs a second writer check. A false brake in two consecutive waves means that the key is too coarse, so the refusal code joins the key.

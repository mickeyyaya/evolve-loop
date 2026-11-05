# Plan: Each pipeline blocker gets a design check, six test rows and a signal that states its cause

- **Status:** proposed, 2026-11-05. The operator asked for a complete test plan on 2026-11-04. The console chose the six rows. This plan waits for the operator.
- **Design:** [signal-accuracy](../architecture/signal-accuracy.md). **Decision:** [ADR-0161](../architecture/adr/0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md). **Research:** [pipeline-blockers-2026-11](../research/pipeline-blockers-2026-11.md). **Incident:** [2026-11-05 wave 155](../incidents/2026-11-05-wave-155-false-signals-and-the-logout.md).
- **Related:** [wave operations](../operations/wave-operations.md), [operating policy](../operations/operating-policy.md), [convergence policy](../architecture/convergence-policy.md).

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

The operator, 2026-11-04 and 2026-11-05:

- "For any issue blocking the pipeline needs to be addressed immediately with complete test plan building around the issue to avoid the regression."
- "The signal needs to be accurate without any mistake, showing clear guidances for agent to debug"
- "For those tasks been blocked over 3 times, we need to escalate it to redesign the solution from architecture perspective with ultrathink"
- "Do the design and architecture review first before build the code"
- "Also follow strictly on TDD"
- "Why do we have bash script?" and "Could we just build the debug cli in evolve loop for your use and cover with tests?"
- "Make sure we have research, design, plan, adr, ...etc docs written in strict format described under docs/"

## 2. Facts

| Fact | Research id |
|---|---|
| Wave 155 landed two ships (`c9a7ef419`, `09bb331d4`), and `evolve wave status` prints "0 of 11 cycles shipped". | F1.1 |
| The halt starts above the batch floor, so a resumed ship is not counted. | F1.2 |
| A test name in a failure text gave the class `infra-error` to a gate block. | F1.10 |
| A routine filter wrote `ISOLATION_I2_VIOLATION` 1,511 times, 53 for each launch. | F1.11 |
| The claude CLI was logged out, and the signal said quota. The pipeline has no class for a logout. | F1.12 |
| The console train cannot land a normally reviewed PR. | F1.13 |
| Seven fixes got 38 findings (11 CRITICAL, 27 HIGH) from the architecture reviewer. | F1.14 |
| 12 of the 38 findings are false or missing causes and next steps in a signal. | F1.15, F1.16 |
| 9 of the 38 findings are two readers of one fact, or a fix wired to one of several entries. | F1.17 |
| The policy has the fix sequence and the convergence ladder. It has no design check and no signal rule. | F1.18, F1.19 |
| The console used scratch commands because verbs are missing. | F1.20 |
| Outside practice agrees: state the cause only when known, and branch on a typed code. | P2.1, P2.5 |

## 3. Goals and non-goals

**Goals**

| Goal | Measure |
|---|---|
| G1. No pipeline blocker is fixed without its six rows | Share of blocker fixes with a table of six rows, each row met or reasoned: 100% |
| G2. A signal of a blocker fix states cause, evidence and next step | Findings of group 1 of F1.15 in the first review: from 12 of 38 to 0 |
| G3. A task with more than three blocks gets a redesign | Tasks past three blocks with no redesign: 0 |
| G4. A routine event does not fill the log | `ISOLATION_I2_VIOLATION` lines for each launch: from 53 to 0 at WARN |
| G5. The wave count agrees with the ledger | Ships in `evolve wave status` against the ledger for a replay of wave 155: equal |
| G6. A logout reads as a logout | The signal for a logged-out run names the logout and the `/login` step |
| G7. The console writes no scratch script | Scratch scripts in a console session: 0. Each missing step is a verb with a test. |

**Non-goals**

- This plan adds no rule beside the convergence ladder. The redesign is a rung of that ladder (Q1).
- This plan does not audit each other signal code. The inbox item `signal-catalog-accuracy-audit` owns that check.
- This plan does not build a general outage detector (research §3).
- This plan does not rewrite past waves. Past waves keep their old numbers.

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Patch until green | Keep the six rows. Patch each fix until the review passes. Add no step. |
| B. Design first, typed signals, redesign past three blocks | A design check before TDD. A signal contract. A fourth block starts a redesign. Missing steps become verbs. |
| C. One redesign of the whole signal layer | Stop all fixes. Redesign the signal layer, the resume path and the ship check once. |
| D. Mechanical gates only | Add gates: a mutation run, a wiring scan and a message lint. Add no design step. |

Each criterion scores 1 (poor) to 5 (good). "Stops a repeat block" counts twice, because it is the cost that wave 155 showed.

| Criterion | A | B | C | D |
|---|---:|---:|---:|---:|
| Stops a repeat block (×2) | 1 (2) | 4 (8) | 4 (8) | 3 (6) |
| Follows the rules of the operator | 1 | 5 | 3 | 2 |
| Keeps the work already built | 5 | 5 | 1 | 4 |
| Cost to adopt | 5 | 3 | 1 | 2 |
| A machine can check it | 1 | 4 | 2 | 5 |
| Risk of a new regression | 3 | 4 | 1 | 3 |
| **Total (of 35)** | **17** | **29** | **16** | **22** |

Basis of the scores:
- **Repeat block:** A has the history of F1.14 as its evidence. B and C remove the cause of the blocks, which are late design faults (F1.15). D catches test faults but not a wrong design.
- **Rules of the operator:** B has all seven rules. C skips the order "design, then build" for the fixes that wait. D has no design step and no escalation.
- **Built work:** C drops seven staged fixes. D keeps them and adds gates.
- **Cost:** B adds a short step and two verbs. D adds a mutation run, which ADR-0159 rejected for a whole module because of hours of run time (F8.8 of [harness-practices](../research/harness-practices-2026-11.md)).
- **Machine check:** D is the best here, and B takes the part of D that is cheap (the equality test of a signal).

**B wins.** The criterion that decided is "stops a repeat block". B and C tie on it. B wins on the rules of the operator, on the built work and on cost. What B takes from D: the equality test of a whole message (R5). What B takes from C: a redesign, but only for the fix that a point past three blocks reaches.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | A design check comes before the TDD step. The author writes a design card of six fields, and an architecture review reads the card. | F1.14, F1.15, F1.19 |
| D2 | A blocker fix meets six rows. The wiring row has one test for each entry. The journey row uses a shape that production writes. | F1.15, F1.17, R4 |
| D3 | A signal has six parts: operation, item, cause or "unknown", evidence handle, impact and next step. | F1.16, P2.1, P2.2 |
| D4 | The writer of a failure sets its class from a host fact. No class comes from words in text. | F1.10, P2.5 |
| D5 | Each fact has one reader: one count, one parser, one rule for the tree. | F1.2, F1.7, F1.17 |
| D6 | A task with more than three blocks stops its fix rounds, and gets a redesign at the top tier from the architecture side. | F1.14, F1.18 |
| D7 | A logout is its own cause. An unknown cause is stated as unknown and never defaults to quota. | F1.12, P2.1, P2.4 |
| D8 | The test of a signal reads the whole text with equality. | F1.16, R5 |
| D9 | The console writes no scratch script. A step that the console does by hand twice becomes a tested verb. | F1.20, R8 |

## 6. Components

The route follows the rule of [ADR-0074](../architecture/adr/0074-typed-signal-contracts-routing-authority.md): an item that touches a protected surface is `console-manual`. Each item has an inbox entry with the same id as its component or as its blocker. The owner `console-manual` means that the console builds it.

| Id | Component | Red test first | Owner |
|---|---|---|---|
| B1 | `bb69b6092` on PR #1283, merged in batch seven (`5df3fdf77`): a paused scope of a console-routed item does not resume | `TestProductionPoolPlanFn_APausedScopeOfAConsoleRoutedItemLeadsNoLane` is red without the fix. The fix is committed. | console-manual (done) |
| B2 | The triage gate accepts the answer that the pipeline prescribes | A lane pinned on an item that gets routed before triage ends as planned no-work, not FAIL. | console-manual |
| B3 | The resume path proves a blocker, from each Build entry | A resumed build with a declared blocker on a protected path routes to the console with no more corrections. This includes a resume from an earlier phase. | console-manual |
| B4 | The ship check reads the lane tree against the audited base | A protected file written through a shell cannot ship, in the unstaged, staged and committed forms. | console-manual |
| B5 | One count of ships for the status and for the halt | A run that resumes a cycle and ships it reports one ship in the status, in the wave facts and in the halt. | console-manual |
| B6 | A phase cell runs `evolve` of the host | A cell started through the real launch path runs `evolve phase verify` at each stage, and the version matches the host. | console-manual |
| B7 | `sync-main` accepts a quarantine move | `sync-main` passes with a real quarantine move and refuses any other deleted tracked file. | cycle 2153, then the console checks |
| B8 | The class comes from the writer, and one parser reads `go test -json` | The real text of cycle 2157 gives `gate-block`. Real output with a nested fixture lists only real failures. | console-manual |
| B9 | One INFO signal for each launch, and a true WARN for a lost pipeline value | A launch with ten extra launcher variables emits one signal that names all ten. | console-manual |
| B10 | A logout is reported as a logout | A journey test with the real logged-out output reports the logout with the `/login` step. A real quota wall still reports quota. | console-manual |
| B11 | The console train accepts a PR that passed the commit gate | A PR committed through the commit gate reaches `record-pr` through the real verbs. An unreviewed commit is refused, and the text names the commit. | console-manual, design first |
| R1 | The host holds one block count for each subject. It crosses rounds, rebuilds, cycles and resumes | A journey test through a real resume keeps the count. A seat cannot write the count. | console-manual, design first |
| R2 | `convergence.Decide` takes the count and returns `ActionRedesign` past three, then `ActionToOperator` after one redesign. The field `Decision.Redesign` keeps its meaning | A subject at four blocks returns `ActionRedesign`, and at three it does not. A second count at four returns `ActionToOperator`. | console-manual, design first |
| R3 | Audit repair, the code-review loop and `evolve convergence decide` act on `redesign` | Each entry has a wiring test. At `shadow` the flow does not change, and the signal fires. | console-manual, design first |
| R4 | The failure events feed the count, and an item at the task ceiling routes to the console | Each halt keeps its ceiling, and its filed item carries the count and the rung. An item at the ceiling is not quarantined. A quota wall and a logout add nothing. | console-manual, design first |
| R5 | A subject on the redesign rung leaves the lane queue and routes to the console with a digest of its findings | A journey test shows that no lane claims the subject, and that a passed card returns it. Depends on trust root PR 2. | console-manual, design first |
| R6 | The text of `CONVERGENCE_REDESIGN` and `CONVERGENCE_TO_OPERATOR` has all six signal parts | An equality test reads each whole text (P2). | console-manual |
| P2 | An equality helper for the text of a signal | A message with a missing cause fails the helper. A message with all six parts passes. | console-manual |
| P3 | The debug verbs `signals count` and `agent show` (names are proposals). `evolve debug cycle N` of ADR-0162 answers for one cycle | Each verb prints the facts of F1.20 from a fixture, and each has a golden test of its output. | console-manual |

### The six rows

| Row | What it proves |
|---|---|
| Red proof | The journey test fails without the fix, for the reason that the incident shows. |
| Journey | A test goes through the real entry point, with the shapes that production writes. |
| Preservation | A test proves that the behaviour next to the fix stays the same. |
| Wiring | A test fails when a production caller loses the fix. One test for each entry. |
| Mutation | Each changed line has a test that a reverted line fails. |
| Signal | A test reads the whole text with equality. The text states the cause, names the evidence and gives a next step that works. |

A row that does not apply states why.

### The design card

The author writes the card before the first test. The architecture review reads it before the build.

| Field | Content |
|---|---|
| Cause | The chain from the first cause to the symptom, with evidence. |
| Home | The one place that owns the fact (a count, a class, a tree, a parser). |
| Entries | Each reader and each caller of the fact. Examples: fresh and resume, each stage, each driver family. |
| Signal | The exact text of each new signal, with its six parts. |
| Rows | The test of each of the six rows, or the reason that a row does not apply. |
| Limits | What the fix does not do, and what a test cannot prove. |

### The starting point of each design check

The seven staged fixes have review findings. The design check starts from them. This list is an input to the card. It is not a decision.

| Blocker | Where the design changes (source: the review report of 2026-11-04 and 05) |
|---|---|
| B2 | The routed card has its own message with the true cause `route:console-only`. The empty-pin branch returns its diagnostics. The rejection text states the cause that the host knows. A wiring test goes through the real triage runner. |
| B3 | The seal of the predicate base happens at each Build entry of the resume loop. A missing and a corrupt snapshot get their own causes. The no-route text gives a step that works for a lane pin. |
| B4 | The ship message states the tree, the base and the path. The restore and console route comes from one source shared with the floor. The sibling check reads the same tree. The `unstaged` test form is unstaged. |
| B5 | A wave is a time window. The tally counts the cycles above `seen` and each cycle with a resume event since the last boundary, by current outcome. The halt and the status share one ship rule. The halt evidence names only the cycles of the run. |
| B6 | One function sends the host directory to the pane at each stage. A wiring test covers each driver family. The journey runs `evolve phase verify`. |
| B8 | Each host writer sets a class field. One leaf package reads `go test -json` for the floor and for the ship. A parallel test that did nothing wrong is not a failure. The text for omitted failures gives a next step. |
| B9 | `selfheal` excludes `source=pipeline`, or the incident gets a true summary. A table test covers each exclusion. The WARN for a withheld marker gives no step that opens a gate. The catalog text agrees with the code. |

## 7. Order

1. **B11, design first.** Until the train accepts a reviewed PR, each console fix needs a manual landing. The design check and the review come before any test.
2. **R1 to R6, P2 and the design cards of B2, B3, B5 and B8.** R5 depends on trust root PR 2. These four fixes have two blocks each, so two more blocks trigger D6. The cards use the list in section 6.
3. **B10.** A logout near 01:00 was followed by a stop reason of `quota-pause` at 56 percent usage. The link to each cycle is not proven (Q4). The design check is short, and the fix needs a captured real output (Q2).
4. **The cards of B4, B6 and B9, then the builds of B2 to B9.** Each build runs in parallel on its own branch from `origin/main`.
5. **P3.** The first verb is `signals count`, because the audit of wave 155 needed it.
6. **B7.** Cycle 2153 holds it. The console checks the rows when it lands.
7. **One batch.** Each merge waits for the next boundary, with the coverage gates run before CI.

The reason for the order: a landing path (B11) comes first. A rule change (R1 to R6, P2) comes before the fixes that use it. The fix with the largest cost (B10) comes before the fixes that cost minutes.

## 8. Verification

| Goal | Before (2026-11-05) | After | How |
|---|---|---|---|
| G1 | 7 fixes, 6 rows met in none of the first reviews (F1.14) | 100% of fixes with a card and a table of rows | The card of each fix |
| G2 | 12 of 38 findings in group 1 (F1.15) | 0 in the first review of each fix | The review report |
| G3 | 4 fixes with 2 blocks | 0 tasks past three blocks with no redesign | The block count of each task |
| G4 | 53 WARN lines for each launch | 1 INFO line for each launch | `evolve signals tail --code ISOLATION_I2_VIOLATION` on one launch |
| G5 | 0 of 11 for wave 155 (F1.1) | The status of a replay equals the ledger count | The B5 journey test |
| G6 | `quota-pause` for a logout | The signal names the logout and `/login` | The B10 journey test |
| G7 | Scratch commands in the audit of wave 155 | None in the next audit | The debug verbs and their tests |

Each component has 100% coverage on each changed function, a red-first test and the loop-equivalent review. The documents of this set pass `evolve docs ste-lint --strict` and the docs-review skill.

**What a test cannot prove.** A test of a message proves its text. It does not prove that the cause is true in a case that the fixture does not model. The journey test with real output covers the first such case (B10).

## 9. Risks

| Risk | Control |
|---|---|
| The design check slows an urgent fix | The card has six fields. The check reviews the card, not the code. A fix that blocks the pipeline starts the check at once. |
| Authors write "does not apply" for a row to skip it | A review seat reads each reason. A reason without a fact fails the card. |
| A redesign repeats the same fault | The redesign output is a new card. The same architecture review reads it. More than three blocks on the new design escalate it again. |
| The equality test makes a message hard to edit | A message has one home, and its test lives beside it. A change to the text changes one test. |
| A verb for the console grows into a second control surface | Each verb is class A (read-only). A verb that writes needs its own plan and ADR. |
| An unknown cause hides a real quota wall | "Unknown" names the evidence it has, and a quota probe that shows the wall keeps the class `quota`. |

## 10. Open questions

| Id | Question | Our proposal |
|---|---|---|
| Q1 | The operator said "blocked over 3 times". ADR-0126 stops a loop at round 3. How do the two meet? | Closed. The operator ruled on 2026-11-06: "once we failed to 4th review, it should trigger the redesign". [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) decisions 12 to 17 hold the path. |
| Q2 | What does a logged-out claude pane print? | Capture one real output as a fixture before B10 is built. I did not find it in the files that I read. |
| Q3 | Why did the 6 protected files of 2026-09-14 and 15 ship (F1.8)? | Check the 4 commits before B4 is closed. |
| Q4 | Which code wrote `quota-pause` for the logout (F1.12)? | Trace it in the design check of B10. |
| Q5 | What proves that a PR was reviewed (B11)? | A record per commit that the gate wrote, joined over the commits of the PR. This is the proposal of the inbox item, and the design check decides. |
| Q6 | Which ADR number? | The console chose 0161, because the trust root set takes 0160. |

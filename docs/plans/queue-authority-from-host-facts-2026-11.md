# Plan: One host record decides what a cycle does to the inbox queue

- **Status:** proposed, 2026-11-05. Not built.
- **Decision:** [ADR-0160](../architecture/adr/0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md).
- **Replaces:** the design of [PR #1290](https://github.com/mickeyyaya/evolve-loop/pull/1290).
- **Related:** [pipeline blockers](pipeline-blockers-2026-11.md), [LLM isolation spec](../architecture/llm-isolation.md).

## Request

The original problem: a system-level failure before build never raises `failure_count`. So each wave claims the same item again. Cycles 2133 and 2151 both failed on one item with `TRIAGE_COMMITMENT_INVALID`.

The operator then said: "For those tasks been blocked over 3 times, we need to escalate it to redesign the solution from architecture perspective".

## Terms used throughout

| Term | Meaning |
|---|---|
| Pin | The item ids that a lane owns. |
| Host record | `.evolve/lineage/cycle-N.json`. The host writes it. |
| Workspace | The directory of one cycle under `.evolve/runs`. A cell can write in it. |
| Cell | The sandboxed process of one phase agent. |
| Breaker | The rule that parks an item after repeated failures before build. |
| Brake | The rule that stops the breaker when many items fail with one cause. |

## Findings

**F1.1** The work took five blocked rounds, and each found a new reader or a new file with one fault.

| Round | Authority | What the review found |
|---|---|---|
| #1290 round 1 | Agent text | One failed cycle routes each item in the queue. |
| #1290 round 2 | `lane-scope.json` | A cell can rewrite the pin. |
| #1290 round 3 | A new record in the workspace | A cell can forge the record. |
| Rebuild round 1 | The pin in memory, at the closeout | The lane wiring has no test, and the route has no ledger line. |
| Rebuild round 2 | The same, at more closeouts | The claim step still reads the pin file. |

*Implication:* the design is wrong, not the patches. Another patch finds another reader.

**F1.2** About 30 production sites read the pin or the decision file, through `committedset`, `core.LaneScopeIDs`, `loadLaneScope`, `readLanePin` and `CommittedIDsFor`. Some run in another process, for example `readLanePin` of a peer in `go/internal/core/lane_scope_assembly.go:71`.
*Implication:* a value in memory cannot reach each reader. The fix must be at the source that they share.

**F1.3** The host writes the record before the first cell of each run (`go/internal/core/input_index.go:61`). Live cycles hold real ids: `cycle-2166.json` holds `recovery-retrospective-c4-stage-key-publish`. The spec says that the file is "never read for lineage", because "a cell can write its workspace" (`docs/architecture/llm-isolation.md`).
*Implication:* the trust root exists, and the project already made this decision for one reader.

**F1.4** `loadLaneScope` wins over the host value at the start of a run (`go/internal/core/cyclerun.go:421`). A new run ID writes a new record from that value (`cycleRecord`, `go/internal/core/input_index.go:53`).
*Implication:* a forged file can reach the record through a resume. The host value must win, and a record for the same cycle must keep its ids.

**F1.5** `go/internal/inputindex` imports `go/internal/committedset`, so `committedset` cannot import `inputindex`.
*Implication:* the record type and its reader move to `committedset`, which is already "the ONE answer". `inputindex` calls it.

**F1.6** Six sites call `RouteConsole` with a string. Review proved that agent text reaches `routed_reason` through three of them.
*Implication:* a check at each site repeats the fault. A typed reason closes the class.

**F1.7** A dependency outage can make every valid message fail, so a system must not name a message poison during an outage ([Glukhov](https://glukhov.org/app-architecture/integration-patterns/dead-letter-queues/)). A breaker at the boundary stops each message from finding the outage alone ([OneUptime](https://oneuptime.com/blog/post/2026-08-14-poison-message-dead-letter-backoff/view)). Sources are practitioner posts, not standards.
*Implication:* the breaker needs a fleet view.

**F1.8** Two more facts proved the old brake dead. `finalizeCycle` advances `LastCycleNumber` on each verdict (`go/internal/core/cyclerun.go:202`). The fingerprint comes from `audit-fail-reason.json`, which a failure before build does not write.
*Implication:* the brake takes its cause and its window from `CycleResult` and the item stamps.

## Candidate designs

| Criterion | Patch each reader | Pin in memory | Restore at cell exit | Flip `i5` | One trust root |
|---|---|---|---|---|---|
| Closes a future reader | No | No | Only on a covered path | Only for a pinned path | Yes |
| Reaches a reader in another process | Yes | No | Not during a cell | Yes | Yes |
| New state | None | None | None | None | None |
| Sites to change | About 30 | About 30 | Each exit path | One, plus its blocker | One reader, six typed callers |
| Test files to change | Many | Many | Few | Few | Few, because a fixture with no record keeps its file |

**The winner is one trust root.** It is the only option that closes the class with no new state. See the ADR for the rejected options.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `committedset.LanePin` reads the host record. With no record, it reads the file. | F1.2, F1.3 | Always read the file: a cell can write it. |
| D2 | `core.LaneScopeIDs`, `loadLaneScope` and `readLanePin` call `LanePin`. A guard refuses any other read of the file. | F1.1, F1.2 | Keep three readers: this is how the fault spread. |
| D3 | The env value wins at the start of a run, and a new run of a cycle keeps the ids of its record. | F1.4 | Let the file win: a forged file reaches the record. |
| D4 | When the file differs from the record, the host keeps the record and emits one true violation signal. | F1.3 | Stay silent: the operator loses the evidence. |
| D5 | The sandbox and the role guard deny a cell write to `.evolve/lineage`, and a contract test proves it. | F1.3 | Trust the path: 2157 showed that a shell write passes the tool guard. |
| D6 | `CommittedIDsFor` goes away. The failure closeout calls `failedCommitted`. It keeps the pin members that `top_n` names, and `committedset.Unanswered` when none. | F1.2 | Keep it: it reads `top_n` with no pin. |
| D7 | Triage routes a protected card when it detects it, if the card is in the pin and the item names the path. | Rebuild rounds | Route at the closeout: this needs a record. |
| D8 | `RouteConsole` takes a typed reason. Only the operator verb can pass free text, through a constructor that says so. | F1.6 | A check at each caller: this repeats the fault. |
| D9 | `lifecycle.Mover` refuses an act of a cycle on an item outside its record. | F1.1 | Check in each consumer: this repeats the fault. |
| D10 | The breaker charges each failure before build, except the host fault codes of a `RefusalDisposition` field. | F1.7 | Skip an empty code: a real scout failure has one. |
| D11 | The cause key is the last phase and `SystemFailure.Category`. | F1.8 | The audit fingerprint: it is empty before build. |
| D12 | When `prebuild_brake_items` distinct items hold one cause inside `prebuild_brake_window_cycles`, nothing parks, and one signal goes out. The first values are 3 and 15. | F1.7, F1.8 | The last PASS from `state.json`: each verdict advances it. |
| D13 | The continuation goes only after the park succeeds. | Round 3 review | Shed it first: a braked item loses its salvage. |

## Worked example

Lane L has the record `[a]`. Item `a` names `go/internal/guards/role.go`.

| Event | Result |
|---|---|
| A cell writes `lane-scope.json` with `[b, c]` | Each reader still gets `[a]`. The host emits one violation signal that names both lists. |
| The claim step runs | It claims `a`. `b` and `c` do not move. |
| Code asks the mover to release `b` for this cycle | The mover refuses and emits a signal. |
| A cell writes `.evolve/lineage/cycle-N.json` | The sandbox denies the write. |
| Triage writes a card for `a` that names the path | The host routes `a` at once, with a reason that holds only the path. |
| A resume starts a new run of cycle N with the forged file in place | The record keeps `[a]`. |
| Scout fails on `a` in two cycles, and no other item shares the cause | The breaker parks `a`. |
| Scout fails on `a`, `b` and `c` with one cause in the window | The brake fires. Nothing parks, and each continuation stays. |

## Work

Each pull request starts from `origin/main`.

| PR | Scope | Why this order |
|---|---|---|
| 1 | D1 to D6: the trust root, its guard and the deny. | It closes the authority of the pin file. It does not close the agent lists on the PASS path, which PR 3 owns. |
| 2 | D7 and D8: the route at triage and the typed reason. | The compiler lists each route site. |
| 3 | D9: the mover refusal. It closes `promote-inbox-reads-agent-lists-outside-the-lane-pin`. | It needs a true record, so it follows PR 1. |
| 4 | D10 to D13: the breaker and the brake. It closes #1290. | The breaker needs PR 1, or it charges forged ids. |

The earlier work is saved at `backups/session-2026-11-04/`, in `pr1290-round3-staged.patch` and `queue-authority-pr1-round2.patch`. A builder can port tests from them.

## Proof

Each fix meets the six rows of the [pipeline blockers plan](pipeline-blockers-2026-11.md): red proof, journey, preservation, wiring, mutation and signal.

| Test | Proves |
|---|---|
| A real cycle with a forged pin, through the real claim, closeout and overlap check: `b` and `c` keep identical item files | D1, D2, D6 |
| A guard fails on new production code that reads the file | D2 |
| A resume with a forged file keeps the ids of the record | D3 |
| The signal text names both lists, the file and the next step | D4 |
| The sandbox contract: a cell write to the record fails, on the real sandbox | D5 |
| A protected card routes during triage, and the ledger and the signal center receive it | D7 |
| The compiler refuses a string reason, and an injection with no spaces never reaches a sink | D8 |
| A mover act outside the record fails and emits a signal | D9 |
| A real scout failure charges the pinned item | D10 |
| The real closeout fires the brake, with `state.json` as `finalizeCycle` leaves it | D11, D12 |
| A braked item keeps its continuation | D13 |

## Limits

- A cycle with no pin takes its ids from `top_n`. Such a cycle is not a lane. The checked-in `fleet.count` is 4.
- A cycle with no record trusts the file. For a cycle with a fleet scope, no cell ran at that point, because a record that cannot be written stops the dispatch. A cycle with no fleet scope keeps a WARN, and it has no pin to forge.
- `cycleclassify` reads an agent-written report to decide if a failure is charged. D11 replaces this for the breaker only.
- Three real poison items with one cause inside the window brake. They keep their retries until an operator reads the signal. This fails in the safe direction.
- Open question: `SystemFailure` can be nil on a verdict FAIL. Then the key is the last phase alone. PR 4 measures this on real closeouts.

## What changes our mind

If the contract test shows that a cell can write `.evolve/lineage` and a deny cannot stop it, the record moves. A false brake in two consecutive waves means that the key is too coarse, so the refusal code joins the key.

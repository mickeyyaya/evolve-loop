# 2026-11-14: a merge-and-relaunch chain advanced on a timed-out wave stop

Related documents: the rules for a boundary in [wave-operations.md](../operations/wave-operations.md).

## Summary

- **Impact:** none. The console stopped the chain before the batch landed. No pull request merged, and no wave started.
- **Duration:** the date is 2026-11-14. The fact sheet gives no times.
- **Root cause:** a background console chain waited for the line `wave stop rc=` in a file. It checked that the line existed, not that the value was 0.
- **Near miss:** `evolve wave stop --wait 120m` timed out with rc=1. Cycles 2374, 2375 and 2377 were still live. The chain recorded two pull requests and started the batch in the middle of a wave.
- **Fix:** the chain goes on only when `wave stop` gives rc=0. It checks the rc of each later step and stops at the first non-zero rc.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-14 | A background console chain starts. It waits for the line `wave stop rc=` in a file. | The chain script of the console |
| 2026-11-14 | `evolve wave stop --wait 120m` times out and gives rc=1. The brake stays engaged. | "timed out after 2h0m0s ... the brake stays engaged", for cycles 2374, 2375 and 2377 |
| 2026-11-14 | The chain sees the line `wave stop rc=`, records the two pull requests and starts the batch. | The chain output |
| 2026-11-14 | The console reads the full stop output, sees rc=1 and stops the chain with TaskStop. | The console session |
| 2026-11-14 | The two pull requests stay open. The brake stays engaged. No wave starts. | The console session |
| 2026-11-14 | The re-armed chain lands the two pull requests at the real boundary and starts wave 173 at fleet 8. | The console session |

## Root cause

**F1.1** `evolve wave stop --wait` blocks until no run lease is live, for the time given at most. At the end of that time it exits with 1, and the brake stays (`go/cmd/evolve/cmd_wave_stop.go:17`).
*Implication:* rc=1 means that the wave is still running.

**F1.2** For each live run, the timeout prints "timed out after ...: run ... still live (...); the brake stays engaged" and returns 1 (`go/cmd/evolve/cmd_loop_stop.go:103`).
*Implication:* the output carries the failure. A reader of the whole output sees it.

**F1.3** The chain waited for the line `wave stop rc=`. The line appears for rc=0 and for rc=1. The chain tested that the line existed, not its value.
*Implication:* a timed-out stop looked the same as a clean stop.

**F1.4** After the stop, the chain recorded the two pull requests, batched them and was about to release the brake and run `evolve wave next`.
*Implication:* the next steps merge code in the middle of wave 172. Code merges only at a boundary ([wave-operations.md](../operations/wave-operations.md)). They also start wave 173 while wave 172 still runs.

## What changed

| Change | Regression test |
|---|---|
| The chain goes on only when `wave stop` gives rc=0. It checks the rc of the record, batch, release and `wave next` steps, and stops at the first non-zero rc. | None. The chain is a console script and not repository code. The rule is in the console memory. |

The fact sheet names the two pull requests by number. This worktree cannot check a pull request number, so this record does not cite them.

## What we learned

- A chain at a boundary gates on the rc of the stop, never on the presence of its output. This is a rule in the console memory.
- A chain that has many steps checks the rc of each step. A step that gives a non-zero rc stops the chain, so that the brake stays engaged.
- The console reads the full output of a stop before it trusts a chain. In this incident that read stopped the chain before the batch landed.

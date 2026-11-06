# ADR-0160: The host lineage record is the one trust root for what a cycle may do to the queue

> **Note (2026-11-06, operator ruling, [ADR-0164](0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md)):** Rule 10 and ADR-0126 decision 16 are one count. A charge is one block.

- **Status:** Proposed (2026-11-05).
- **Plan:** [queue-authority-from-host-facts-2026-11](../../plans/queue-authority-from-host-facts-2026-11.md). **Research:** [queue-trust-root-2026-11](../../research/queue-trust-root-2026-11.md). **Design:** [queue-trust-root](../queue-trust-root.md).
- **Amends:** the Item id row of [llm-isolation](../llm-isolation.md), which says that a new run replaces the record (rule 13).
- **Relates to:**
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the pipeline is the only controller. This ADR applies that rule to the inbox queue.
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): an agent proposes and the pipeline disposes. A list in a workspace is a proposal.
  - [LLM isolation spec](../llm-isolation.md): it already trusts the lineage record and never reads `lane-scope.json` for lineage (line 776).
- **Evidence:**
  - Six review rounds blocked the work on two designs and on PR 1 of this one (research F1.1).
  - Production code has 61 lines in 29 files that read the pin or the decision file (research F1.3).
  - The host record exists today, and the isolation spec trusts it for one reader (research F1.4).
  - The PASS path of post-ship still reads the `top_n` of the agent (research F1.9).
  - A host write can follow a planted symlink into the record, and a damaged record reads as no record (research F1.10, F1.11).
  - We found no outside source on an agent-written file as a control plane (research P2.7).

## Context

The operator said: "For those tasks been blocked over 3 times, we need to escalate it to redesign the solution from architecture perspective with ultrathink".

PR #1290 and its rebuild took five blocked review rounds. Each round closed one reader of an agent-writable file, and the next round found another. The fault is not in a reader. A file in the workspace holds authority, and each cell can write the workspace.

The first review of the third design (PR 1) also found faults. They are of another class. A reader of the agent list remains on the PASS path. A host write can follow a link that a cell planted. A damaged record reads as no record. The root is right, and the edges are open.

Outside work agrees on the principle. An action needs authority from a trusted source, not from the exposure of a tool (research P2.1, P2.3, P2.4). It does not cover a file as the channel (research P2.7).

## Decision

1. The record `.evolve/lineage/cycle-N.json` is the only source of which items a cycle owns.
2. The host writes the record before the first cell starts. No cell can write it.
3. `committedset.LanePin` is the one reader of the record. Each other reader calls it.
4. Only a missing record reads `lane-scope.json`. A damaged record gives no authority and stops the cycle.
5. `lane-scope.json` is a copy for the agent. When it differs from the record, the host keeps the record and signals.
6. A record that cannot be written for a lane stops the dispatch before a cell starts.
7. A host write into a workspace refuses a symlink. The record is read-only on disk.
8. A route takes a typed reason that host values build. Agent text never reaches a route reason.
9. The mover refuses an act of a cycle on an item outside its record.
10. A failure before build charges the item one block, as in ADR-0126 decision 16. A host fault charges none. A brake stops mass parking.
11. A guard fails on any reference to the pin file name outside a short list.
12. PR 1 closes the pin file. PR 3 closes the agent lists on the PASS path.
13. A new run of a cycle keeps the ids of its record.

Rule 12 corrects the draft of this ADR and of the plan. The draft said that PR 1 "closes the class". It does not. After PR 1 and before PR 3, a forged `top_n` can promote an item of another lane on a landed PASS (research F1.9).

## Consequences

**Good**
- One change closes each present reader and, with the guard, each future reader of the pin file.
- No new state. The record exists, and the isolation spec already trusts it for lineage (research F1.4).
- A forged file raises a true violation signal with its evidence.
- The design holds at `isolation.i5.stage` = `shadow`, the default today (research F1.2).

**Bad**
- Each route caller changes to a typed reason. The compiler lists the six sites (research F1.7).
- A damaged record stops a cycle that the old code ran. The stop is loud, and one signal names the cause.
- A cycle with no record still trusts the file, for example a test fixture or a cycle before its first dispatch. No cell has run at that point.
- The PASS path stays open between PR 1 and PR 3.
- Each host write into a workspace must use one helper. A reviewer found about 60 candidate calls, and nobody classified them (research F1.10).

**Risk**
- The guard proves only that no code names the file outside the list. It does not prove that no code reads an agent id from another file.
- The first brake values (3 items, 15 cycles) are estimates, with no measurement behind them.

**What changes this decision**
- If a contract test shows that a cell can write `.evolve/lineage`, the record moves. The new path is one that the sandbox denies. The decision stays.
- If a review round finds a new file in a workspace that gives authority, we ask if the record needs a second check. We revisit rule 1.
- If a cycle number is ever reused with another assignment, rule 5 gains a signal on the next run (plan Q1).

## Rejected

| Option | The fact that rules it out |
|---|---|
| Patch each reader | Five rounds proved this, and 61 lines in 29 files read the files (research F1.1, F1.3). |
| Pass the assignment in memory | The overlap check of a peer lane runs in another process (research F1.3). |
| Restore the file when each cell exits | A peer reads during a cell, and each path that misses the restore reopens the fault (research F1.1, round 2). |
| Flip `isolation.i5.stage` to `enforce` | The default is `shadow`, a named blocker holds the flip, and it protects a path and not a meaning (research F1.2). |
| Sign the workspace file | It adds a key to keep, to protect a file that this design does not need (research section 3). |
| Labels on each value and a custom interpreter, as in CaMeL | Our authority values are item ids, and the queue code is deterministic Go (research P2.3, section 3). |
| A stronger prompt in each brief | System controls beat prompt defenses in the one study that we read (research P2.2), and the rules existed during five rounds. |
| A guard that matches call shapes | It missed 6 of 8 planted forms (research F1.12). |
| Treat a damaged record as no record | A damaged record shows that a cell may have run (research F1.11). |

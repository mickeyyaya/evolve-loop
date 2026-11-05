# ADR-0160: The host lineage record is the one trust root for what a cycle may do to the queue

- **Status:** Proposed (2026-11-05).
- **Plan:** [Queue authority from host facts](../../plans/queue-authority-from-host-facts-2026-11.md).
- **Relates to:** ADR-0154, which makes the pipeline the only controller. This ADR applies its rule to the inbox queue.
- **Evidence:** the five blocked rounds in the plan (F1.1), 30 readers (F1.2), and the record that exists today (F1.3).

## Context

The operator said: "For those tasks been blocked over 3 times, we need to escalate it to redesign the solution from architecture perspective".

PR #1290 and its rebuild took five blocked review rounds. Each round closed one reader of an agent-writable file, and the next round found another reader. The fault is not in any reader. The fault is that a file in the workspace holds authority, and each cell can write the workspace.

## Decision

1. **One trust root.** The record `.evolve/lineage/cycle-N.json` says which items a cycle owns. The host writes it before the first cell starts. No cell can write it.
2. **One reader.** `committedset.LanePin` reads that record, and each other reader calls it. So the readers do not change, and no reader can read the wrong source.
3. **The workspace file is a copy.** `lane-scope.json` stays, for the agent to read. It grants nothing. When it differs from the record, the host keeps the record and emits a signal.
4. **The reason of a route is a type.** The inbox mover takes a typed reason, and a constructor builds it from host values. Agent text has no way into a route reason, the ledger or a signal.
5. **The mover refuses at the sink.** An act of a cycle on an item outside its record fails, and the host emits a signal. An operator verb is not bound to a cycle, so this rule does not apply to it.

## Alternatives

| Option | Why it is rejected |
|---|---|
| Patch each reader | Five rounds proved this. A new reader reopens the fault. |
| Pass the assignment in memory | A reader in another process cannot get it, for example the overlap check of a peer lane. |
| Restore the file when each cell exits | Each path that misses the restore reopens the fault, and a peer can read during a cell. |
| Flip `isolation.i5.stage` to enforce | It is good defence, but a named blocker holds it, and it protects a path, not a meaning. |
| Sign the workspace file | It adds a key to keep, to protect a file that the design does not need. |

## Consequences

- Good: one change closes each present reader and each future reader. A guard fails the build when new code reads the file.
- Good: no new state. The record exists, and the isolation spec already uses it for lineage.
- Good: the signal for a forged file is a true violation, with its evidence.
- Bad: each route caller changes to a typed reason. This is a compile-time cost, and the compiler lists each site.
- Bad: a cycle with no record still trusts the workspace file, for example a test fixture or a cycle before its first dispatch. For a cycle with a fleet scope, no cell has run at that point, because a record that cannot be written stops the dispatch. A cycle with no fleet scope keeps a WARN on that failure, and a cell can then run with no record. This gives no new power, because such a cycle has no pin to forge, and its agent already controls `top_n`.

## What changes this decision

If a contract test shows that a cell can write `.evolve/lineage`, the record moves to a path that the sandbox denies. The decision stays.

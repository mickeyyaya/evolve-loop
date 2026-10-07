# internal/inboxrank/rankinputs

> The rank it feeds: [internal-inboxrank.md](internal-inboxrank.md) and [ADR-0121](../adr/0121-inbox-priority-is-a-computed-rank.md). The plan step that added it: P2 of [inbox prioritization](../../plans/inbox-prioritization-2026-10.md) (2026-10-06). The ledger it reads: [internal-recurrence.md](internal-recurrence.md). The policy block: [policy-config.md §inbox_priority](../policy-config.md#inbox-priority-rank-inbox_priority).

## Purpose

`rankinputs` loads everything the inbox rank needs for one evolve dir and returns it as an `inboxrank.Inputs`. `Load(evolveDir, now)` reads the dir's `policy.json` for the `inbox_priority` block and its `recurrence-ledger.json` for each item's recurrence count, stamps the clock the caller passes, and returns the warnings it met as text. Every rank consumer composes its inputs here: the wave seed, widen and refill (inside `triagecap.ReadInboxBacklog`), the triage prompt's `inbox_batches` menu, `evolve inbox rank`, `evolve inbox batches` and the dashboard's queue.

## Design

- **One loader for five consumers.** Before P2 only `evolve inbox rank` composed the rank's inputs, in the verb. Wiring four more consumers would have copied the policy read, the ledger read, the projection to counts and the warning texts four times. The loader is the one copy; each consumer only chooses where its warnings go (stderr with its own prefix, or the dashboard's `Snapshot.Warnings`).
- **A sub-package, because the rank must not import the ledger.** The recurrence package's import tree is wide (the disposition router, the failure adapter, cycle state, dossiers), and `internal/inboxrank` stays pure: `go list -deps ./internal/inboxrank` names no `internal/recurrence`. The I/O that joins the two lives here, one level down.
- **A malformed policy ranks with the compiled default and says so.** `policy.Load` returns the zero `Policy` on a parse error, whose `InboxPriorityConfig()` is the compiled default, and the loader adds `policy unreadable (<err>); ranking with the compiled inbox_priority default`. A consumer inside the loop must never stop ranking over a bad policy (the loop's own policy reload reports the file at every wave boundary). `evolve inbox rank` keeps P1's stricter contract by loading the policy itself first and exiting 2 on an error.
- **The ledger is read without its lock.** `recurrence.ReadSnapshot` decodes the ledger without the `<path>.lock` sidecar, so ranking never creates a file; the ledger is replaced by atomic rename, so a lockless read sees the old file or the new one. An unreadable or malformed ledger yields no counts and `recurrence ledger unreadable (<err>); the recurrence factor is 0 for every item`, the text `evolve inbox rank` printed since P1. An absent ledger or an absent policy is not a problem and warns nothing.
- **The clock is a parameter.** The loader never reads the time itself; the consumer passes `now` (the verb's injected clock, the dashboard collector's, or `time.Now()` at the read in the triage prompt and the backlog), so a test pins the age factor.

## Invariants

- **Absent files are the compiled default and no counts, silently** (`TestLoad_AbsentFilesAreTheCompiledDefaultAndNoCounts`).
- **The policy's block and the ledger's item counts pass through unchanged**, keyed by item id through `Ledger.ItemCounts()`, the recurrence package's one linkage (`TestLoad_ReadsThePolicysBlockAndTheLedgersItemCounts`).
- **Every degraded read is named, never silent** (`TestLoad_AMalformedPolicyRanksWithTheCompiledDefaultAndSaysSo`, `TestLoad_AMalformedLedgerCountsNothingAndSaysSo`).
- **Enforced at 100%.** The package is in `go/.apicover-enforce` and in `go/.cover-strict` at 100, like `internal/inboxrank`.

## Findings

- **The live plane has no ledger yet** (2026-10-06): the runtime plane's `.evolve/recurrence-ledger.json` does not exist, so the recurrence factor is 0 for every item there and the loader warns nothing.

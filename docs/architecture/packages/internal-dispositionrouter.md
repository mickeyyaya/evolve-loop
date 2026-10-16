# internal/dispositionrouter

## Purpose

`internal/dispositionrouter` is the deterministic floor layer of the failure-disposition router (S3). It decides where a classified failure goes: to the operator console, or to the priority queue that lanes consume. It also stages the resulting intents in a file that the boundary applier reads.

## Design

- **Closed route vocabulary.** A disposition is operator-owned (`console`) or lane-dispatchable (`queue`). Nothing else exists.
- **Floors first, advice second.** `Decide` applies the two floors and then the advisory LLM route. The advisory route can only raise `queue` to `console`. An unknown or empty `llmRoute` is a no-op.
- **Staging is JSONL.** `StageIntent` appends one record to `.evolve/escalations/pending-actions.jsonl`. It creates the directory on first use. The shared file lock serializes the append, so concurrent lanes cannot tear a line. `LoadIntents` reads the file in append order.
- **Intent shapes.** An intent either escalates the weight of an existing open inbox item, or autofiles a new item for a pattern with no open item. `Weight` is the base weight that the escalation formula starts from. `Recurrence` is the count that it escalates for.
- **Name.** The package is not named `router`. `internal/router` is the unrelated phase and model dispatch router. Two routers in one namespace are a collision, not reuse.
- **Dependencies.** Stdlib and `internal/adapters/flock` only.

## Invariants

- **Floors are deterministic and one-way.** An advisory route may raise a queue disposition to console. It may never lower a floor-forced console back to the queue. Pinned by `TestDecide_FloorsForceConsole`.
- **A guard abort always goes to the console.** A guard abort (`GuardAbortClass`) is pipeline machinery failing by construction, for example a severed statemap. It is never a lane-sized task defect.
- **Recurrence 3 or more always goes to the console.** `RecurrenceConsoleFloor` is 3. A defect that survived two fixes is no longer lane-sized. Re-queueing it a third time only reproduces the fault.
- **Staging never writes the inbox.** Intents land in `.evolve/escalations/`, never in `.evolve/inbox/`. A mid-flight inbox write races the `os.Rename` of `inboxmover.Claim` and resurrects a claimed item into double work across fleet lanes. Only the boundary applier (`recurrence.ApplyBoundary`), after a wave has drained, may touch the inbox.
- **A missing staging file is not an error.** Nothing staged is the common case, and `LoadIntents` returns `(nil, nil)`. A malformed line is a real error that the caller must surface (`TestLoadIntents_MalformedLineErrors`). A staging failure is never swallowed, because a silently dropped intent is a lost escalation (`TestStageIntent_UnwritableDirErrors`).

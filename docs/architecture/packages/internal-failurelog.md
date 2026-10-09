# internal/failurelog

## Purpose

`internal/failurelog` keeps the failure floor of the loop. It holds the failed-cycle taxonomy, the record of each failed cycle in `.evolve/state.json` (`failedApproaches`), the expiry of those records and the carryover-todo lifecycle. The package ports `failure-classifications.sh`, `record_failed_approach` and the prune logic of `cycle-state.sh` from bash. `internal/core` and the loop dispatcher call `Record` on a verify failure. They call `PruneExpired` at dispatcher start. `--reset` calls `PruneByClassification`.

## Design

- **Four files by responsibility.** `classifications.go` holds the taxonomy. `record.go` appends an entry and changes state. `prune.go` removes expired entries. `prune_carryover.go` handles the carryover todos, which share the same state file and expiry rule.
- **One expiry rule.** `isExpired` decides both `failedApproaches` and `carryoverTodos`. The two arrays therefore agree on what has aged out. An entry with `expiresAt` expires at that instant. A legacy entry with only `recordedAt` expires one day later. An entry with neither timestamp is kept, because its age is unknown.
- **The taxonomy is a typed vocabulary.** `Classification` matches the strings that the bash writer stored. `OperatorReset` records an `evolve cycle reset` of a partial cycle. The failure floor counts it as operator action, not as a code defect. `LoopFatal` records a loop-runner fatal exit. The stop reason sits in the entry summary as `stop_reason=<reason>`.
- **Expiry uses Go time arithmetic.** `ComputeExpiresAt` returns UTC at second precision. The bash version failed on v8.23.1. Its `jq fromdateiso8601` read unquoted ISO strings as epoch plus one day. The Go port has no string-to-time step that can fail that way.
- **Legacy classes normalize to the taxonomy.** `NormalizeLegacy` passes canonical values through unchanged. It maps the dispatcher's legacy classes and the older spellings. An empty or unknown input becomes `UnknownClassification`. The caller may log or skip that value.
- **One vocabulary serves the prompt and the gate.** `VocabularyList` renders the known classes as one comma-separated list. The contract block that a prompt carries and the deliverables gate's correction both use this list. An agent therefore picks from the list that the gate checks.
- **The record is a FIFO of 50 entries.** `MaxEntries` caps `failedApproaches`. A new entry drops the oldest entry when the list is full. The bash slice `.[length-50:]` did the same trim.
- **The cycle counter never moves backward.** `appendFailedApproach` keeps the larger of the old `lastCycleNumber` and the entry's cycle. A loop fatal can carry cycle 0. A lower value can reuse cycle numbers and corrupt the workspace history.
- **A missing state file is a soft warning.** The dispatcher's preflight creates `state.json`. If it does not, `Record` returns `ErrStateMissing`. The loop logs a warning and goes on. An unwritable state file is fatal, because each retry overwrites the same diagnostic evidence.
- **Writes keep a symlinked state file a link.** Every state writer renames over `statemap.ResolveWriteTarget(path)`. A rename over a worktree link replaces the link with a regular file. Later writes then go to a detached copy.
- **The summary has a fallback chain.** `resolveRecordSummary` tries these sources in order. It tries an explicit override, then the explicit report path, then the workspace's `orchestrator-report.md`. Last, it tries the verdict reports that a workspace really has. No production code writes `orchestrator-report.md`, and 0 of 241 live workspaces carried one. An empty workspace stays empty, because a made-up summary is worse than none.
- **Summary extraction has fixed bounds.** `extractSummary` joins the first eight lines of a Failure, Verdict or Phase Outcomes section. It joins them into one line of at most 400 characters.
- **Typed entries are stored as plain maps.** `mustMarshalToAny` round-trips a `Recorded` value through JSON. The stored entry then has the same shape as the untyped legacy entries.
- **State round-trips keep unknown keys.** `state.json` holds many fields that `core.State` does not model. The record path decodes the file as `map[string]any` and writes every key back.
- **Carryover todos get a 30-day legacy stamp.** `DefaultCarryoverBackfillTTL` is 30 days. It matches the CodeBuildFail and CodeAuditFail classes. The backfill stamps a legacy todo that has no `expiresAt`, so the prune path can clear the old population. A todo that already has an expiry keeps it.
- **Carryover steps run in a fixed order at boot.** The prune and backfill steps run first, so a todo removed at this boot is not counted. Then `IncrementCarryoverUnpicked` runs. Then `recordFailureLearning` writes new todos. A new todo starts at zero and is not counted in its own cycle.
- **Writes happen only when something changed.** `writePruneResult` skips the write when nothing was removed. This avoids a file-time change and a rename race. `atomicWriteJSON` delegates to `internal/atomicwrite`.

## Invariants

- **A legacy entry with no timestamp is never pruned.** Its age is unknown, so deleting it can lose data. Each prune function keeps an entry that it cannot age or classify. The operator edits `state.json` by hand to drop such entries.
- **A malformed timestamp keeps its entry.** An unreadable `expiresAt` or `recordedAt` is kept. Pinned by `TestPruneExpired_MalformedExpiresAtKept` and `TestPruneExpired_MalformedRecordedAtKept`.
- **`lastCycleNumber` never decreases.** `Record` can raise it and cannot lower it. Pinned by `TestRecord_DoesNotRegressLastCycleNumber`.
- **The backfill is idempotent and never restamps.** A second pass leaves the file byte-identical. A stamped entry keeps its expiry. Pinned by `carryover_lifecycle_test.go`, which also pins the 30-day stamp. Changing that value ages the whole legacy population at once.
- **Every state writer keeps a symlinked state file a link.** Pinned by `symlink_state_test.go`, the cycle-1690 pin of the cycle-999 break.
- **The default class is outside the taxonomy on purpose.** A supervisor-made record of a mid-execution failure ages out on the one-day legacy bucket. Adding the class, or lengthening that TTL, is an operator decision. `TestClassificationMidExecutionFail_IsOutsideTheTaxonomy` (ADR-0103 unit 03b, F11) fails the day that change lands.
- **Missing state is a safe no-op.** Each prune, backfill and increment returns zero and no error when the state file is absent. None of them can stop the loop from starting.

## Findings

- **An invented class.** An agent once used a failure class that the gate did not know. The agent had never been given the vocabulary (cycle 1684). `VocabularyList` now gives the agent the same list that the gate checks (`vocabulary_test.go`).
- **Silent empty summaries.** The `summary` field stayed empty on every recorded failure. The read tolerated the missing `orchestrator-report.md`, and nothing reported it. The verdict prose lives in `audit-report.md` and `build-report.md`, so the fallback chain reads those files (`summary_fallback_test.go`).
- **The expiry epoch bug.** The bash expiry produced epoch plus one day on v8.23.1. The Go port has no string step that can fail in that way.
- **The carryover list had no removal path.** It held 65 entries and 26,601 bytes across cycles 366 to 506. The prune and backfill were added to clear it (`carryover_lifecycle_test.go`, the cycle-507 RED tests).
- **The state-file break.** A writer renamed a regular file over cycle 999's `state.json` link. That cut the worktree off from its state. Every writer now renames over the resolved target.
- **A constant counter.** `cycles_unpicked` was hard-coded to zero, so the advisor cannot see a stale todo. `IncrementCarryoverUnpicked` now makes the field count real cycles.
- **A regressing counter.** A loop fatal with an unknown cycle can move the counter back. The counter now only moves forward.

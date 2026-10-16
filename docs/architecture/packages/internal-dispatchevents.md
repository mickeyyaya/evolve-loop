# internal/dispatchevents

## Purpose

`internal/dispatchevents` writes the dispatcher-level entries of `abnormal-events.jsonl`. The entries are: counter non-advance, circuit-breaker trips, verify failures, classifier verdicts, goal-stall escalations and the per-cycle phase-output survey. The package writes at the cycle boundary. `internal/adapters/observer` writes within a phase. Both write JSONL, to different files, for different readers.

## Design

- **The wire format is the format of the bash dispatcher.** Field names and `EventType` strings match the old shell dispatcher. The `jq` filters of operators keep working.
- **`Severity` is a typed string alias.** The values are `INFO`, `WARN` and `ERROR`. JSON marshaling needs no conversion.
- **`SourcePhase` is `dispatch` for this writer.** `RemediationHint` is a short suggestion for the operator.
- **`Writer` is mutex-guarded.** One `Writer` for each cycle workspace is the intended use. Reuse is safe across goroutines.
- **`NewWriter` does not create the workspace.** The dispatcher created it before any phase ran. A failed `os.MkdirAll` here can hide a real bug. A missing workspace makes `Emit` fail at `OpenFile`. The missing-workspace case in `events_test.go` pins this.
- **`Emit` returns an error where the shell ignored it.** The shell used `|| true`. Go callers must at least log the error. `Emit` fills a zero `Timestamp`.
- **`marshalFn` and `writeFn` are test seams.** `json.Marshal` of the typed `Event` cannot fail in practice. A write fails only on a full disk or a closed pipe. The seams let `events_coverage_test.go` prove the error wrapping.
- **The abnormality decision is not here.** `internal/phaseoutputs` decides. `EmitPhaseOutputsSurveyed` only maps it onto severity. `INFO` is the healthy heartbeat that a dashboard baselines on. `WARN` is the signal for missing review data. `phaseoutputs_event_test.go` pins this.
- **`EmitClassification` uses `INFO` for a recoverable class.** It uses `ERROR` for an integrity breach, the signal that demands human investigation.

## Invariants

- **One home owns the vocabulary.** `abnormal-events.jsonl` has two writers: this package and `internal/phasewatchdog`. `EventStallDetected` is declared here, with its siblings.
- **A value that only one writer declares is invisible to the typed readers of the other writer.** Records that parse but fall outside the vocabulary are skipped silently. The queries that an operator runs when a phase went missing skip them too.
- **`IsKnownEventType` is exported.** The second writer asserts membership and keeps no list of its own. `vocabulary_test.go` pins this.
- **Take the last `EventPhaseOutputsSurveyed` of a cycle.** A cycle that aborts and resumes finalizes twice, so it appears twice. Each snapshot was true at its moment. Consumers must use the last one.
- **A goal-stall event names its breaker.** Two breakers share `EventGoalStallEscalated`. The empty-only breaker reports `empty/blocked`. The non-progress breaker reports `fail/empty/blocked`. The `outcomes` field says which ceiling fired.
- **A mixed FAIL and EMPTY streak must not read as empty-only.** That reading points the diagnosis at the wrong evidence. `goalstall_event_test.go` pins this.

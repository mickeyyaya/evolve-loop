# internal/core/failurelearning

> Decision records: [ADR-0103](../adr/0103-component-breakdown-program.md) (unit 03b) and ADR-0039 section 7 (a phase's own failure report). Design: [03b-failure-learning-engine.md](../decomposition/03b-failure-learning-engine.md).

## Purpose

`internal/core/failurelearning` is the deterministic half of learning from a failed phase. One `Engine` owns three parts:

- the failed-approach recorder, which makes the `FailedRecord` and the P0 todo through the unit-03 `Lifecycle`;
- the deterministic floor, which renders the retrospective report, the lesson YAML and the inbox remediation items when no LLM retrospective can;
- the recurrence closure.

The retro runner, the observer, the cycle-state writes, the ledger, the checkpoint, the disposition gate and the persist stay in `core`.

## Design

- **The engine never persists and never writes stderr.** It holds a clock, the `Lifecycle` and a Signal Center accessor. It reports four failure modes as `failurelearning.warning` under module `failurelearning`.
- **The clock is a closure.** Tests swap the clock of the orchestrator after construction. The Center accessor is read at every use for the same reason. A nil accessor, or one that returns nil, is the Null Object. `SignalsWired` proves that the production roots wired one.
- **`Failure` is the one input and `Learned` the one output.** `RecordFailedApproach` reads `Cycle`, `Phase`, `Err` and `Workspace`. `WriteFloor` reads `Cycle`, `Phase`, `ProjectRoot` and `Workspace`. `Learned` holds the summary, the todo id and the adopted block.
- **`RecordFailedApproach` changes the state that it is given.** The callers read the changes back. It appends the `FailedRecord`. It appends a deduped P0 carryover todo through the `Lifecycle`, so admission events keep flowing. It stamps `LastCycleNumber`. A nil state is a programming error and panics.
- **A phase that can self-report owns its failure description** (ADR-0039 section 7). Its structured block beats the synthesis of the supervisor. The recorder reads the block once and passes it to the floor. `state.json` and the lesson artifacts then cannot diverge on one failure.
- **Adoption is a trust boundary** (`adoptStructuredFailure`). Two rules apply:
  - The block is adopted only if its class normalizes into the canonical taxonomy. A block whose class is unknown is dropped, not adopted.
  - Lists and entries are capped (`maxAdoptedDefects`, `maxAdoptedDefectRunes`, the cap of the carryover unit). A misbehaving agent cannot bloat `state.json` or the lesson corpus.
- **The TTL comes from the final classification.** It is computed once. The todo inherits the stamp of the record.
- **The todo never outlives the carryover TTL** (2026-10-21). The todo takes the earlier of the record stamp and `carryover.BoundedExpiry`, which is now plus the carryover TTL of 30 days. The record keeps the retention of its class. Before, an `intent-rejected` class gave the todo the same 999999999-second retention as the record. Thus `cycle-1063-failed-build` had a 2058 expiry, and the TTL never evicted it. Pinned by `TestRecordFailedApproach_ATodoNeverOutlivesTheCarryoverTTL`.
- **The floor is best effort and ordered.** The LLM retro can fail to run or return a non-canonical verdict. `WriteFloor` then renders the report, the lesson YAML and the remediation items. The lesson survives. A write failure gives one warning and never hides the original phase failure. The recurrence closure runs last, always.
- **Operator config is resolved once.** One read of `<root>/.evolve/policy.json` (`loadPolicy`) gives the near-duplicate bound and the remediation weight. They pass to `faillearn` as options, so `faillearn` stays a leaf without policy. Absence is silent. A read or parse error gives one `CodePolicyLoadFailed` warning and the compiled defaults. It never disarms the gate or suppresses everything.
- **Only self-reported structured defects reach the queue.** The synthesized summary echo restates the failure and is not an item. The filter is `faillearn.StructuredDefects`, the rule that the lesson writer already applies. A `structured != nil` proxy once filed classed-but-defectless blocks as priority-H bugs.
- **The lesson event** takes the defaults of the supervisor first. It then takes class, defects and evidence from the validated and capped block of the failed phase. The workspace is appended last.
- **Remediation ids are stable and injective.** An id has the cycle, a slug of the defect line and `remediationFingerprint`. The fingerprint is a short digest of the full defect title, appended always.
- **The slug alone is lossy twice.** It stops at `remediationSlugMaxRunes`, and it collapses runs of non-alphanumerics. Two defects that differ only late, or only in punctuation, then got one id. The collision refusal suppressed the retrospective and the lesson with the second item.
- **Stable ids make a floor re-run idempotent.** `writeIfAbsent` keeps the first write.
- **The queue is bounded and the bounds are recorded.** `remediationMaxItems` caps the count. `remediationTitleMaxRunes` caps the title. Defect lines are agent-authored and were unbounded. A queue that nobody can triage hides the real item. A defect that cannot be named gives no item.
- **Remediation items carry non-empty provenance.** `inboxbatch.ConsoleRouted` treats an empty `injected_by` as operator-authored.
- **The recurrence closure** writes the failing lesson pattern into the recurrence ledger, keyed by the failing cycle. `Count()` then reflects real history. Escalator and Autofiler are nil there. Escalation apply stays boundary-only. The live consult site reads the ledger and must not race `inboxmover.Claim` from the closeout in the middle of a cycle.
- **A ledger failure gives one warning that names the failing call.** The call is `load`, `record` or `save`. The warning never hides the phase failure. `record` is dormant today. `RecordClosure` fails only through the nil-guarded escalator and autofiler.

## Invariants

- **The floor survives an unavailable retro.** This is the retro-always invariant (inbox gap 1). A lesson never degrades to a stderr line.
- **Every failure mode is a signal.** Four registered conditions replaced seven hand-written stderr lines. Each has its reason.

# Contextual Logging and Model Performance Recovery Plan

**Date:** 2026-09-11
**Batch 2 base:** `a03023277df0c18b48bf539cc2fcfe55a8adc64d`
**Status:** Batch 1 merged; Batch 2 implemented and locally verified, with exact-tree review and shipping in progress
**Runtime:** Go only

## Objective

Make a failed model launch explain what stopped it, where it happened, and what
evidence supports the diagnosis. Separately, make every supported model launch
produce one canonical attempt record from which latency and input/output
performance can be compared honestly.

The work is split into isolated merge batches. The timeout closeout is completed
and reviewed first. Canonical attempt telemetry follows on a new branch from the
merged result. This keeps error semantics independent from telemetry schema and
lets each module be tested, reviewed, and reverted on its own.

## Verified gaps

### Terminal timeout diagnostics

`replWaiter.wait` still performs terminal closeout inline. That block lists the
workspace, writes escalation evidence, checks exhaustion-regex drift, classifies
a transient pane, writes the authoritative marker, and applies the transient
cooldown. These effects have a required order, but no named component owns it.

The authoritative marker currently reports phase, elapsed intervals, reviewer
state, liveness, and a free-form reason. It does not identify the driver, cycle,
artifact name, or a stable cause code. Two concrete failures are consequently
misleading:

- cancellation before the first review produces `last_review=none`,
  `liveness=unknown`, and an empty reason even though cancellation stopped the
  wait;
- a completion-detector fault is logged once during polling and then discarded,
  so the terminal error reports the reviewer's later pause instead of the local
  detector failure that prevented completion.

`Engine.Launch` correctly gives the marker precedence for exit 81 and preserves
`core.ErrArtifactTimeout`. `core.DeliveryFailureCause` reads the marker's
`reason=` field. These are compatibility contracts, not replacement targets.

### Model attempt telemetry

`Engine.recordTokenUsage` combines three responsibilities: resolve token usage,
emit warnings, and append `llm-calls.ndjson`. It returns before appending when the
resolver is absent or fails. A telemetry enrichment failure therefore removes
the entire attempt, including exit and duration evidence. This also affects
`cycleclassify`, which reads the last recorded exit for a phase.

The record's historical `model` field contains the requested value after only an
empty-to-`auto` default. For tmux drivers this is commonly an abstract tier such
as `balanced` or `deep`. The actual model is resolved in the realizer and can be
omitted or clamped again by a driver. Re-resolving it independently in the
recorder would create another source of truth.

`phasetiming.Entry` is a phase-level terminal projection. It must remain separate
from per-attempt telemetry. The dashboard currently joins repeated phase names
by ordinal position, which is not a reliable retry or repair-round correlation.

## Design rules

1. Preserve `bridge: launch exit=N`, exit codes, `errors.Is` sentinels, and retry
   classification.
2. Keep one authoritative exit-81 marker and emit it after all other timeout
   diagnostics so `artifactTimeoutSummary` remains deterministic.
3. Derive cause codes only from host-owned control state. Raw pane, prompt,
   response, argv, and environment content cannot become a cause code.
4. Keep detailed evidence as fields; do not replace it with a vague prose line.
5. Treat missing measurements as unavailable, never as measured zero.
6. Preserve historical fields when extending durable schemas.
7. Record total launch time independently of token resolution time.
8. Describe output-token rate as amortized attempt throughput. It includes tool
   execution and is not provider decode speed.
9. Do not claim universal time to first token. First stdout bytes, first stable
   pane output, and first generated token are different signals.
10. Add no dependency, exported bridge API, goroutine, polling operation, or
    constant boolean condition.

## Batch 1: terminal closeout and contextual errors

Branch: `refactor/tmux-wait-terminal-diagnostics`

### Components

#### Pure timeout diagnostic

A package-local diagnostic value owns the stable marker fields and cause
selection. Its cause vocabulary is closed and host-authored:

- `context_cancelled`: the wait loop stopped because its context ended before a
  deliverable was confirmed;
- `completion_detector_error`: the terminal detector observation returned a
  concrete error;
- `submit_wedged`: bounded submission verification proved that the prompt or
  nudge remained parked;
- `transient_upstream`: the agent-stripped pane matched the launched family's
  manifest-declared transient pattern;
- `review_stop`: a reviewer or fatal-pane gate selected stop;
- `review_pause`: a reviewer selected pause;
- `incomplete`: defensive fallback when none of the known terminal signals is
  present.

Cause precedence follows captured control state that ended or invalidated the
wait:

1. cancellation observed by the wait coordinator's cancellation branch;
2. a completion-detector error on the terminal detector observation;
3. verified submission wedge;
4. manifest-classified transient upstream failure;
5. reviewer stop;
6. reviewer pause;
7. incomplete fallback.

The marker retains the existing fields and adds `cause`, `driver`, `cycle`, and
the artifact base name. Field order begins with `cause` and the bounded
`reason`, before identity and counters, so the legacy
`reason="...submit_wedged..."` consumer cannot lose the delivery classification
to a long driver or artifact name. Phase, driver, and artifact fields are
independently bounded and rendered without terminal controls. Free-form reason
and detector error fields are independently bounded, quoted, and single-line.

Exit-81 summaries get a dedicated 1,024-rune outer bound. The existing 300-rune
`boundCause` remains unchanged for every non-timeout error. Per-field bounds
keep a worst-case marker below the exit-81 budget; a unit contract constructs
maximal Unicode fields and proves every required key plus the complete reason
survives. `core.DeliveryFailureCause` prefers the new host-authored
`cause=submit_wedged` field and retains its legacy `reason=` fallback for old
records. This removes its dependence on escaped quotes in new markers without
breaking historical errors.

#### Effectful closeout

`replWaiter.closeout` owns the existing terminal transaction:

1. return success immediately when completion was confirmed;
2. print the human failure line and bounded workspace listing;
3. write the existing escalation report for pause or stop;
4. strip injected prompt echoes once and run the drift warning;
5. classify the stripped pane for transient evidence;
6. create and emit the authoritative marker last among diagnostics;
7. apply the existing transient redispatch cooldown only for the existing
   short-circuit case and only while the context is live;
8. return the unchanged result and exit code.

The wait coordinator calls this component once after its loop. It retains pane
capture, detector polling, tick interactions, and checkpoint ordering.
Admission failures keep their existing early-return marker path and never enter
the full timeout closeout, which would add workspace, escalation, drift, or
cooldown effects to a pre-wait failure.

#### Retained detector fault

`replWaitState` retains the last concrete detector error in addition to the
existing one-time-log latch, and separately records whether the latest detector
observation errored. An error-free but incomplete poll makes the prior fault
secondary evidence; it does not erase it or prove recovery. The primary cause is
`completion_detector_error` only when the terminal detector observation itself
errored. A later confirmed completion returns success and emits no timeout
diagnostic. Repeated faults remain logged once and survive into the final marker.

The coordinator stores its context error only when it actually takes the
cancellation branch and the detached final poll does not confirm completion.
Closeout never derives the cause from a late `ctx.Err()` observation. Therefore
cancellation triggered inside or after a reviewer callback can still suppress
the existing cooldown without relabelling a decision that already stopped the
wait. A final-poll detector error is retained as secondary evidence.

### TDD contracts

Write and run these public-path tests before production edits:

1. cancellation without a deliverable reports `cause=context_cancelled`, the
   driver, cycle, artifact name, and a non-empty cancellation reason;
2. a terminal detector fault reports `cause=completion_detector_error` and the
   concrete detector error in the marker and `Engine.Launch` error;
3. detector error → incomplete nil observation → reviewer stop retains the
   detector error as secondary evidence but reports `cause=review_stop`;
4. detector error → confirmed completion emits no timeout diagnostic;
5. prompt/nudge submission wedges report `cause=submit_wedged` while
   `core.DeliveryFailureCause` still extracts the existing reason;
6. a manifest-recognized transient pane reports `cause=transient_upstream`
   without copying provider prose;
7. ordinary stop and pause verdicts report their distinct cause codes;
8. cancellation fired from the stop-review callback does not relabel the
   already-selected reviewer stop, while still suppressing cooldown;
9. a long-Unicode identity plus quoted/control-bearing detector text stays
   bounded, cannot inject a terminal line, and cannot truncate the complete
   delivery classification consumed by `core.DeliveryFailureCause`;
10. the marker remains the last diagnostic line before cooldown and continues to
   beat earlier bridge chatter;
11. success emits no timeout marker and performs no timeout closeout effects;
12. cancellation suppresses the transient cooldown;
13. direct pure-cause tests cover every precedence pair and unknown values.

For each new decision assertion, temporarily mutate the corresponding branch and
record the failing result before restoring it. Extract only after the public
contracts are RED.

### Validation

- focused tests with `-race -count=20`;
- Bridge statement coverage, with every new pure decision and closeout function
  at 100%;
- full Bridge race suite;
- Bridge vet and golangci-lint;
- provider-free real tmux happy, timeout, resume, and concurrent-session matrix;
- repository-wide `go test -count=1 ./...` and `go vet ./...`;
- code simplifier, Go code/test reviewer, and architecture reviewer on the exact
  staged tree;
- commit gate and all four GitHub checks before merge.

## Batch 2: canonical model-attempt telemetry

This batch starts from merged Batch 1 on a new isolated branch. Its design is
frozen only after a fresh architecture review of the actual Batch 1 result.

### Canonical record owner

Introduce a leaf package below Bridge and dashboard that owns the record schema,
bounded reader, append operation, and pure aggregation. It depends on the
standard library and `cyclestate.TokenUsage`, not `core`.

One completed `Engine.Launch` lifecycle makes one append attempt regardless of
token resolver state. Token collection is optional enrichment of that record.
The legacy `model`, token, source, duration, and exit fields remain readable.

Additive fields under review:

- `call_id`, start and end timestamps, and an explicit timing scope;
- `requested_model`, optional `dispatched_model`, and dispatch provenance;
- `usage_status`: `measured`, `partial`, `unavailable`, or `resolver_error`;
- optional `first_output_ms` plus its measurement source;
- stable `cause_code` from host-owned failure classification;
- a dispatch correlation identifier only if the phase/dashboard join is fixed in
  the same atomic reader migration.

The dispatched model must be captured from the finalized driver decision that
builds argv. An omitted model is recorded as CLI default with unknown concrete
identity. It must never be guessed from a tier map after the fact.

### Honest performance index

Group by CLI, dispatched identity when known, measurement source, and outcome.
Each row reports sample count, success/failure count, unavailable count, total
input/output/cache tokens, and latency distribution. Output tokens divided by
positive total attempt duration may be exposed as amortized output throughput.

Do not calculate a rate when duration is non-positive, output usage is absent,
or the source is only an output floor. Do not report input throughput until a
real prefill boundary exists. Optional first-output measurements are grouped by
their source; different sources are never pooled as one TTFT number.

### Batch 2 TDD and compatibility gates

1. nil resolver, resolver error, partial result, and full result each append one
   attempt record without changing the launch outcome;
2. duration ends before token resolution begins;
3. requested tier, concrete dispatch, omitted/default model, clamp, and fallback
   attempts are distinct and accurate;
4. direct `LaunchArgs` calls are either explicitly instrumented or documented as
   outside the attempt-ledger boundary; the CLI cannot claim broader coverage;
5. legacy lines, malformed lines, unknown fields, and partial writes degrade
   without fabricated values;
6. zero duration, absent counts, partial/output-floor sources, and non-finite
   calculations produce unavailable rates;
7. independent concurrent appenders produce complete JSON lines;
8. terminal-control characters are removed from human rendering;
9. `cycleclassify` behavior remains unchanged when failed attempts are now
   present even if usage is unavailable;
10. retry and repair-round dashboard joins are correlation-based or explicitly
    left unchanged without a false accuracy claim.

An append/open/lock failure is diagnostic-only and cannot change a successful
launch into failure. The canonical record package remains the sole attempt
writer; `internal/log` may format console output but does not acquire a parallel
model-lifecycle schema.

## Batch 1 implementation record

The implementation follows the approved boundary:

- `driver_tmux_wait.go` retains polling, pane observation, interactions, and
  checkpoint coordination. It now records each detector observation and calls
  one closeout method after the loop.
- `driver_tmux_wait_closeout.go` owns the existing ordered timeout effects. It
  adds no poll, pane capture, file-list operation, escalation write, or sleep;
  it moves the prior transaction intact and emits the marker after the other
  timeout diagnostics.
- `driver_tmux_wait_diagnostic.go` owns the closed cause vocabulary, pure
  precedence function, bounded field formatting, and sole marker write for the
  completion wait.
- `replWaitState` stores cancellation provenance, the last concrete detector
  error, whether the latest detector observation failed, and typed submission
  verification evidence.
- `Engine.artifactTimeoutSummary` now uses a dedicated 1,024-code-point bound.
  It accepts exact `[bridge] artifact-timeout:` lines and selects the final
  match, which is the closeout-owned marker. The generic bridge error bound
  remains unchanged.
- `core.DeliveryFailureCause` treats the marker's leading `cause` field as the
  classification authority and decodes the escaped reason only after
  `cause=submit_wedged`. Historical markers without a cause field retain the
  legacy reason path.

During self-review, an initial implementation inferred `submit_wedged` from
the reviewer reason. A new negative public-path test demonstrated that free-form
reviewer text could forge the cause. The implementation was corrected before
review: prompt and nudge submission verification now set a typed boolean in the
wait state, and cause selection no longer imports or scans free-form text.

The formatter also initially preserved only the beginning of a long detector
error. The public test showed that long relocation paths displaced the leaf
error (`not a directory`). Bounded evidence now preserves both its prefix and
suffix, retaining the failed operation and actionable cause.

The defensive review found that the original substring extractor could accept
a marker forged inside earlier reviewer text. Prefix anchoring fixed inline
text but still allowed a newline-prefixed fake line. The final implementation
quotes and neutralizes reviewer reasons at the checkpoint log, removes Unicode
formatting controls such as bidi overrides, and selects the final exactly
prefixed bridge marker. End-to-end tests cover both inline and newline marker
forgeries through `Engine.Launch` and `core.DeliveryFailureCause`.

### TDD and verification evidence

- The public-path tests failed against the unchanged implementation for every
  missing cause/context field and for typed marker parsing. The pure cause and
  formatter tests initially failed to compile because their components did not
  exist.
- Focused behavior tests pass for cancellation, late cancellation after review,
  terminal and recovered detector faults, confirmed completion, prompt/nudge
  wedges, transient panes, ordinary review pause/stop, size bounds, and legacy
  parsing.
- Ten deliberate mutations were rejected: cause precedence, typed submission
  evidence, terminal detector reset, late-cancellation provenance, typed cause
  parsing, the exit-81 size budget, exact marker prefixing, final-marker
  selection, checkpoint-log neutralization, and Unicode-format neutralization.
  No mutation survives the final test set.
- Every new formatter, cause-selection, detector-observation, closeout, marker
  writer, and exit-81 bound function has 100% statement coverage. The full
  Bridge package reports 93.5% statement coverage.
- The focused suite passed under the race detector for 20 repetitions. Bridge
  and Core vet passed, scoped golangci-lint reported zero issues, and the full
  Bridge and Core packages passed.
- Repository-wide `go test -count=1 ./...` and `go vet ./...` passed.
- The provider-free real tmux integration matrix passed for happy completion,
  artifact timeout, named-session resume, and concurrent-session isolation.

## Review record

The initial architecture review returned **REVISE** on a one-batch design. It
identified the token-resolver coupling and non-authoritative model label as high
risk, phase/attempt indexing and direct `LaunchArgs` coverage as medium risk, and
required explicit measurement semantics. The two-batch boundary, compatibility
gates, actual-dispatch provenance, unavailable measurement states, and honest
throughput terminology resolved those plan findings before Batch 1 began.

Batch 1 then passed code simplifier, Go code/test, and architecture review on the
exact tree and merged through PR #557 as `a03023277df0c18b48bf539cc2fcfe55a8adc64d`.
All four GitHub checks passed before merge.

## Batch 2 implementation record

### Isolated components

The implementation follows the module order from the recovery plan:

1. `internal/llmcalls` owns the additive record schema, bounded validation,
   cross-process append/import transaction, call identity, and pure performance
   aggregation.
2. Bridge owns a call-local attempt state. `Engine.Launch` freezes the start and
   terminal time around one shared `LaunchArgs` invocation; drivers report only
   selectors observed at their actual process, tmux-send, positional, or REPL
   boundary.
3. `evolve models performance`, dashboard, token reporting, cycle
   classification, and model-probe salvage consume the canonical package. No
   consumer persists another performance index.
4. Detailed measurement and operating semantics live in
   `docs/architecture/model-attempt-telemetry.md`; phase timing links to the
   distinct per-attempt scope.

### Correctness decisions

- One completed orchestration-owned `Engine.Launch` produces one append attempt
  even when its token resolver is nil, errors, reports partial usage, or finds no
  source. Direct `LaunchArgs` calls and setup errors before that shared
  invocation remain explicitly outside the boundary.
- The measured end is captured before token resolution. Usage status separates
  measured, partial, unavailable, and resolver-error records. Unavailable
  context fill uses the existing `-1` sentinel.
- Requested and dispatched identities are separate. Generated selectors are not
  credited to a warm named session. Tmux launch identity is observed only after
  successful transport; `/model` identity is observed only after that seed line
  is sent. Codex attribution uses the post-clamp selector.
- Model-like text after `--` is ignored. Selector evidence is derived from the
  exact deduplicated launch vector: manifest-generated flags retain a trusted
  prefix, while surviving profile flags are parsed conservatively. Unknown
  option arity makes dispatch identity unknown, and both ambiguity and the `--`
  terminator remain sticky across the later direct-extra segment. Codex uses one
  provider-specific split/inline grammar for dedicated model flags and
  `-c`/`--config model=...` overrides. A dedicated model flag wins over a
  config override regardless of argv order. Repeated config overrides retain
  ordered override semantics; repeated surviving dedicated flags are a Codex
  argument conflict and therefore produce unknown dispatch attribution. The
  ChatGPT-account clamp and telemetry apply the same precedence.
- The existing Codex subscription clamp covers realized flags, including profile
  raw flags. Direct `BridgeRequest.ExtraFlags` remain the explicit pass-through
  appended after that clamp. Telemetry observes a verified selector in those
  extras, but does not present it as safety-clamped.
- Resolver/provider/file errors are bounded to 512 Unicode code points and
  ASCII-quoted by the shared `internal/log.DiagnosticField` encoder. Attempt
  warnings include `call_id`, CLI, agent, and attempt context; performance-ledger
  read failures name their operation and use the same injection-safe encoding.
  Typed `cause_code` accepts only the leading host-owned timeout cause field.
- Invalid resolver measurements are normalized before JSON persistence:
  negative usage becomes unavailable, invalid peak contributors are discarded,
  and non-finite or sub-sentinel fill values become the existing `-1` sentinel.
  Each repair remains visible as a contextual warning.
- Append and import acquire the repository-standard `<ledger>.lock` flock.
  Import holds it across destination indexing and all writes, preserving exact
  bytes and deduplicating `call_id` across independent salvage processes.
- The 1 MiB writer limit has matching reader delimiter capacity. Readers reject
  JSON null, empty objects, and unrelated envelopes while accepting the minimal
  legacy `phase` contract. Torn and malformed lines remain counted degradation,
  not fabricated attempts.
- Dashboard correlation uses timestamps and consumes an attempt at most once.
  Whole-second legacy ends include the final fractional second. A precise call
  start can establish the later round; timestamp-only calls in an overlap and
  calls inside indistinguishable repeated-phase windows are withheld from both
  rounds.
- The classifier uses the latest phase attempt regardless of token availability,
  so a failed retry cannot inherit an earlier exit-zero result.
- Performance groups separate CLI, verified model, requested selector, dispatch
  source, token measurement source, timing scope, and outcome. Missing latency
  renders as unavailable; measured zero remains distinct. JSON includes attempt,
  malformed-line, and read-warning metadata.
- Output throughput is derived only from complete measured output counts and
  positive Bridge-dispatch durations. Input prefill speed, provider decode speed,
  and universal first-token latency remain unavailable because no comparable
  boundary exists.

### TDD evidence

Tests were introduced before each production component. The RED runs included:

- undefined canonical record/store/aggregate APIs;
- nil and erroring resolvers producing no ledger record;
- missing lifecycle, dispatch, and usage-provenance fields;
- late model overrides, failed dispatch, and named-session resume attribution;
- a slow resolver inflating attempt duration;
- repeated salvage duplicating one `call_id`;
- the latest unavailable-usage failure inheriting an earlier success;
- repeated audit rounds joining the wrong model;
- unsupported `performance` command dispatch;
- tmux REPL selectors credited before their seed was sent;
- headless log-open failure credited as a process dispatch;
- newline and bidi injection through resolver errors;
- a fractional terminal timestamp dropped at a whole-second phase end;
- independent salvage processes duplicating one record;
- exact-limit records accepted by the writer but rejected by `bufio.Scanner`;
- JSON null and unrelated objects counted as attempts;
- failed tmux launch transport claiming a model;
- a quoted `reason` manufacturing `cause=submit_wedged`;
- adjacent same-second rounds sharing one call;
- missing latency rendered as `0s/0s`;
- performance JSON hiding malformed input; and
- incompatible token sources and timing scopes merged into one group.

The final logging pass added two more RED contracts: the shared diagnostic field
encoder did not yet exist, and a newline-bearing ledger path could inject a
second command warning line. Both now pass through one bounded encoder used by
Bridge and the performance command.

The second implementation review added four RED contracts before its fixes:

- an unknown-arity profile option could promote its model-looking value;
- a profile `--` boundary was forgotten before direct extras were parsed;
- a timestamp-only boundary call could migrate into a later repeated phase; and
- indistinguishable repeated-phase windows could claim one precise call.

The provenance effect stored by `Realization` and the explicit phase-call window
component turn those cases green. A separate salvage-path injection contract
also routes every probe-salvage destination, source, and error through the shared
diagnostic encoder.

The next exact-tree review found two post-transformation contracts that were not
yet pinned. An order-preserving dedupe could remove a repeated final selector
after telemetry had already selected it, and the Codex subscription clamp chose
the first repeated selector even though the CLI applies the last. Engine-level
argv-versus-ledger and effective-clamp tests failed first. Attribution now reads
the emitted deduplicated flags with their provenance boundary intact, and the
clamp rewrites the effective selector before `--`.

The following review found that the clamp still recognized fewer selector forms
than telemetry: it could rewrite an earlier split flag while a later
`--model=...` or `-c model=...` override remained effective. Composed tmux-launch
and ledger tests failed first for both forms. A single selector-occurrence parser
now drives attribution, effective-selector lookup, exact-token replacement, and
post-clamp observation.

The fifth exact-tree review checked that shared grammar against Codex itself and
returned **REVISE**. Codex gives `-m`/`--model` precedence over a generic
`-c`/`--config model=...` override regardless of argv order; the implementation
had treated the textually last occurrence as effective. RED unit and composed
tmux/ledger tests reproduced both the unsafe clamp and false attribution. The
selector state now retains its class across realization and direct-extra
segments, applies Codex's provider precedence, supports the long config spelling,
and never interprets another provider's `-c` flag as Codex configuration. A
follow-up provider-contract check also established that repeated dedicated model
flags are rejected rather than resolved last-wins. Identical pairs are removed
before attribution; surviving repetitions are now withheld as unknown, including
when they cross the realization/direct-extra boundary. Empty dedicated values
are withheld by the same conservative rule.

Twenty-nine deliberate semantic mutations were rejected by their focused tests.
They cover exact scanner capacity, minimum record validation, destination-wide
process locking, aggregation dimensions, argument termination, profile/raw
provenance, final-argv attribution, effective-selector clamping, inline and
config selector grammar, Codex selector-class precedence, provider-specific
config parsing, long config spelling, repeated and empty dedicated-selector
attribution, repeated-selector clamping, cross-segment termination, tmux send
ordering, typed cause parsing, three phase-correlation boundaries, latency
availability, mixed-scope headers, latest-attempt classification, resolver
normalization, diagnostic ASCII escaping, and salvage-path escaping. No
mutation survived.

After the corresponding minimal implementations, the complete affected set
passed:

```text
go test ./internal/llmcalls ./internal/bridge ./internal/dashboard \
  ./internal/cycleclassify ./cmd/evolve -count=1
```

The final local verification also passed:

- the focused new contracts under the race detector for 20 repetitions;
- the cross-process import/deduplication integration contract under the race
  detector for 20 repetitions;
- the full affected-package race suite;
- repository-wide `go test -count=1 ./...` and `go vet ./...`;
- scoped golangci-lint with zero issues;
- the provider-free real-tmux integration matrix for happy completion, artifact
  timeout, named-session resume, and concurrent-session isolation; and
- all 29 semantic mutations described above, with zero survivors.

The canonical storage package reports 86.1% statement coverage. Every critical
record lifecycle, workspace path, append, read, scan, identity, and validation
function is covered at 100%. The focused Bridge contracts cover attempt
recording at 96.3% and all warning, diagnostic, selector-observation, and
normalization helpers at 100%. Dashboard phase-window construction is covered at
100%, as are the performance renderer, latency projection, and shared diagnostic
field encoder.

A live read-only run against the runtime workspace indexed 466 historical
attempts with no malformed rows. It correctly labeled their absent schema-v2
evidence as unknown model, `legacy_unverified` dispatch, and
`legacy_unspecified` timing instead of presenting legacy values as verified
measurements. Commit-gate and GitHub CI results belong to the immutable staged
tree and PR, respectively, and are recorded by those durable checks.

### Implementation review iterations

The requested architecture, Go, and defensive/simplifier reviewers examined the
uncommitted implementation after the first green affected-package run and
returned **REVISE**. Their concrete findings were all converted to tests before
the fixes:

- process-local import locking;
- overlapping dashboard windows;
- dispatch observation before failed tmux transport;
- free-form timeout cause parsing;
- unavailable fill and latency presented as zero;
- mixed token sources and timing scopes;
- JSON null accepted as an attempt;
- exact writer/reader size mismatch;
- model-looking pass-through text entering identity; and
- malformed counts absent from JSON output.

The implementation described above resolves those findings.

The second review returned **REVISE** on three additional boundaries. The
architecture and defensive reviewers found that `Realization.LaunchFlags`
already mixed trusted manifest output with arbitrary profile raw flags, and
that separately parsing direct extras forgot a preceding `--`. The Go and
architecture reviewers found that boundary-only calls could migrate to the
later repeated phase. The Go reviewer also required the real-process import
test to move out of the default unit tier and clean up every child on bounded
failure paths.

Those findings produced the RED contracts above. Raw-profile dispatch evidence
is now derived from the final emitted vector while retaining its conservative
provenance boundary, parser uncertainty and termination are sticky, ambiguous
dashboard evidence is withheld, and the subprocess test is integration-tagged
with a 15-second command context plus per-child cleanup. The review verdict for
the final exact tree is recorded in the commit-gate attestation and PR review
evidence.

The third review returned **REVISE** on selector evidence being computed before
the final argv transformation. All three reviewers independently reproduced the
dedupe mismatch, and the defensive reviewer also found the repeated-selector
clamp mismatch. The new RED contracts and fixes above resolve both at the
production invocation boundary. A fresh exact-tree verdict is required after
these changes.

The fourth review returned **REVISE** because the Codex clamp recognized only
separated selectors while telemetry also recognized inline and config forms.
The shared selector-occurrence parser and composed RED/GREEN contracts above
resolve that mismatch without changing the lifecycle, storage, or aggregation
boundaries. The corrected tree is resubmitted for an exact-tree verdict before
the commit gate.

The fifth review returned **REVISE** after verifying selector precedence against
the official pinned Codex source. Dedicated `-m`/`--model` overrides the
generic config model independent of token order. The prior last-token rule could
leave an unsafe dedicated model intact while recording the safe fallback. The
same review then verified Codex's scalar option contract: a repeated dedicated
selector is an argument conflict, not a last-wins override. The new RED
contracts, provider-specific precedence state, and conservative repeated-selector
attribution resolve both findings; the corrected exact tree is resubmitted to
all three reviewers.

The first review of this written two-batch plan also returned **REVISE** before
TDD. It found that the shared 300-rune error bound could truncate the legacy
delivery reason, an incomplete nil-error poll could not prove detector recovery,
and a late cancellation could overwrite the true stop decision if closeout read
`ctx.Err()` directly. The revised plan gives exit 81 a separate bounded budget,
places and bounds the delivery reason first, tracks detector evidence separately
from the terminal-observation status, and captures cancellation only in the
coordinator branch that it actually terminates.

After those corrections, the architecture reviewer returned **PROCEED**. The
implementation review must verify that `cause` is parsed as a real marker field,
never as text embedded inside quoted evidence, that legacy `reason=` parsing is
used only when a cause field is absent, and that the size budget is measured
after quoting and escaping.

The first implementation review returned **REVISE** from the combined code
simplifier/defensive reviewer. It found two provenance gaps: earlier free-form
stderr could impersonate the marker, including through a newline, and Unicode
format controls could visually reorder token fields. Both were reproduced with
RED tests and corrected as described in the implementation record. A mutation
then showed that final-marker selection needed its own direct two-candidate
contract; that test was added and rejects first-match behavior.

The final staged implementation received **PASS** from the code
simplifier/defensive reviewer, Go code and test reviewer, and architecture
reviewer. The architecture reviewer confirmed the coordinator, closeout, and
pure diagnostic boundaries; the Go reviewer confirmed cancellation and detector
semantics, bounds, parser compatibility, determinism, and race safety; the
defensive reviewer confirmed marker provenance, control neutralization, and
side-effect ordering.

## Campaign completion

After both batches merge, rerun the Go-native component/hotspot scan and update
the waiter refactor report. Then run the two requested evo verification waves.
Record cycles, selected work, shipped commits/PRs, failures, token/latency
evidence, and whether each wave produced a useful code or documentation change.
Two zero-ship waves stop the loop and trigger root-cause reporting rather than a
third unbounded run.

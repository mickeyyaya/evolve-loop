# Signal Center codes

> Generated from the code registry — `signalcenter.RegisterCode`, one call per code, in the module
> that owns it — by `evolve signals codes generate`. `evolve signals codes check` (run by a `cmd/evolve`
> test, so by CI) exits 2 when this file drifts from the registry. Design:
> [signal-center-design.md](signal-center-design.md) §5.4 · [ADR-0101](adr/0101-signal-center.md).

Every WARN or INCIDENT signal carries one of these codes; INFO signals carry none. A code names the
**rule** that fired (`MODULE_SNAKE_CASE`, prefix = the owning module); `kind` names the event and
`origin` the function. Triage kind-first (design §9): the kind says what happened, the code says
which rule decided it, `fields` name the artifact to open. A code that reaches the Center without
being registered here is not dropped — it is stamped `SIGNALCENTER_UNREGISTERED_CODE` with the raw
value in `fields.raw_code`, so the drift is visible in the same file.

Ship codes are the `shiperr` vocabulary projected verbatim (`SHIP_` + code, `shiperr.SignalCode`);
the ship phase's own `ship-error.json` and ledger entries keep the unprefixed spelling.

<!-- GENERATED:signal-codes BEGIN — do not edit by hand; run `evolve signals codes generate` -->

### bridge

| Code | Meaning |
|---|---|
| `BRIDGE_CONTEXT_FILL_HIGH` | an attempt's context fill crossed the configured warn threshold; the reason names the fill and the threshold |
| `BRIDGE_TELEMETRY_APPEND_FAILED` | the per-attempt telemetry record could not be appended to the workspace ledger; fields name the path |
| `BRIDGE_TELEMETRY_TRIPWIRE` | a successful attempt ran past the tripwire threshold with no measurable token usage — telemetry blind spot, not a phase failure |
| `BRIDGE_TOKEN_RESOLVER_FAILED` | the token resolver returned an error for a completed attempt; usage recorded as resolver-error |
| `BRIDGE_TOKEN_RESOLVER_MISSING` | the engine was built without a token resolver; lifecycle and outcome records continue without token counts (fail-open) |
| `BRIDGE_TOKEN_USAGE_WARNING` | the token resolver measured the attempt with a caveat (invalid counters, partial measurement); the caveat is the reason |

### carryover

| Code | Meaning |
|---|---|
| `CARRYOVER_PERSIST_FAILED` | state.json could not take this run's failure-learning arrays (fields.step: write = the legacy whole-state write, update = the serialized read-modify-write); the in-memory state stands and the caller proceeds — the FailedRecord and the P0 todo may be lost from disk |
| `CARRYOVER_WORKSPACE_MALFORMED` | a cycle-workspace carryover document is not its documented JSON shape; the file is skipped whole, never aborting the cycle — a persona defect for the memo or audit owner; the reason carries the decode error |
| `CARRYOVER_WORKSPACE_READ_FAILED` | a cycle-workspace carryover document (carryover-todos.json or defect-ledger.json) exists but could not be read (not absence: permissions, a directory at the path); nothing is merged, the closeout proceeds; fields name the document and path |

### config

| Code | Meaning |
|---|---|
| `CONFIG_INERT_PHASE_ENABLE` | a phase is force-enabled (enabled: on) while dynamic_routing is below advisory and it is neither mandatory nor in the static state machine, so the enable never runs it (the cycle-120 confusion); set dynamic_routing>=advisory or remove the enable; fields.step=inert, phase, stage |
| `CONFIG_REGISTRY_MALFORMED` | docs/architecture/phase-registry.json was read but is not valid JSON (a trailing comma is the classic); the same compiled-baseline degrade as CONFIG_REGISTRY_UNREADABLE — no triage, no registry order — with the decoder's error in the reason; fields.step=registry, path, err |
| `CONFIG_REGISTRY_UNREADABLE` | docs/architecture/phase-registry.json exists but could not be read (permissions, a directory at the path — absence is silent); every cycle runs on the compiled baseline, which omits triage, the registry order, the enabled/routing blocks, the goal recipes and the deliverable kinds; fields.step=registry, path, err |
| `CONFIG_SPINE_ORDER` | the registry's phases[] order places ship before audit, so the artifact-backed floor cannot gate ship on a shippable audit by position; the legality graph and the audit verdict branch still block it; fields.step=spine, audit_pos, ship_pos |
| `CONFIG_UNKNOWN_VALUE` | a routing dial's value is outside its closed vocabulary, or a registry contract is incomplete, and the documented fail-safe was taken — never a kill-path enable (a stage word → off, routing_mode → llm, model_routing → static, enabled → content, EVOLVE_SANDBOX → the current mode, a bad conditional rule or max_optional_insertions ignored); fields.step names the resolving step (registry, env or policy), fields.key the dial, fields.value the word, fields.default the fallback |
| `CONFIG_WEAK_SPINE` | mandatory_phases (the registry's or EVOLVE_MANDATORY_PHASES) omits audit and/or ship, so the audit-before-ship guarantee rests on the legality graph and the audit verdict branch alone; the cycle proceeds; fields.step=spine, fields.missing = audit, ship or audit+ship |

### failurediag

| Code | Meaning |
|---|---|
| `FAILUREDIAG_SIDECAR_WRITE_FAILED` | the phase's <phase>-failure-diag.json could not be written (temp write or rename); the reason names the step and the error, fields name the path; the phase abort proceeds unchanged and the diagnosis is lost from disk |

### failurelearning

| Code | Meaning |
|---|---|
| `FAILURELEARNING_FLOOR_WRITE_FAILED` | the deterministic floor could not write retrospective-report.md, the lesson YAML or the inbox remediation (a faillearn error, including the id-collision refusal); the FailedRecord and the P0 todo already stand in memory, the recurrence closure still runs, the phase failure is never masked; fields.step=floor, lessons_dir, workspace |
| `FAILURELEARNING_POLICY_LOAD_FAILED` | .evolve/policy.json exists but could not be read or parsed while writing the deterministic floor (absence is silent); the novelty threshold and the remediation weight fall back to the compiled defaults and the floor proceeds — ONE read where two reads of one file fired two lines before; fields.step=floor, path |
| `FAILURELEARNING_RECURRENCE_LEDGER_FAILED` | the recurrence ledger keyed by the failing pattern could not be loaded, updated or saved (fields.op = load | record | save); Count() stays stale for this pattern, the phase failure is never masked; fields.step=recurrence, path |
| `FAILURELEARNING_REMEDIATION_TRUNCATED` | a failed phase self-reported more defects than the remediation cap; the first ones are filed as inbox items and the rest dropped — fix the emitter or raise the cap; fields.step=floor, reported, filed |

### gate.contract

| Code | Meaning |
|---|---|
| `GATE_CONTRACT_DEMOTED` | the breaker opened after N consecutive blocks and demoted enforce→advisory; the phase advanced UNVERIFIED — inspect the failing phase and policy.gates.contract_gate |
| `GATE_CONTRACT_FAIL_OPEN` | the gate could not decide (unknown phase, read fault) and failed open; re-dispatching an agent cannot fix this — the reason is the error |
| `GATE_CONTRACT_REJECTED` | the gate refused the deliverable at enforce; the reason is the correction directive (one [code] message per violation), fields carry the codes and the breaker count — the orchestrator's ladder re-dispatches |
| `GATE_CONTRACT_SALVAGED` | a sole recoverable bad_verdict was repaired on disk and re-verified clean; the phase advanced on the repaired artifact |
| `GATE_CONTRACT_VERIFIED` | the phase's declared deliverables were found in place (fields name the artifact, its size, the agent-owed files and the effects verified) and the phase advanced |
| `GATE_CONTRACT_WOULD_BLOCK` | the deliverable violated its contract but the stage (shadow/advisory, or the report-size gate's) lets the phase advance; the reason is what enforce would have refused |

### ledger

| Code | Meaning |
|---|---|
| `LEDGER_APPEND_FAILED` | the ledger could not append an entry (lock, chain or I/O failure); the reason is the error, fields name the entry |

### liveness

| Code | Meaning |
|---|---|
| `LIVENESS_PANE_EXHAUSTED` | a tmux pane shows the CLI's quota/rate-limit exhaustion (LivenessCenter edge: exhausted; the exhaustion gate corroborates before rc 85) |
| `LIVENESS_PANE_HUNG` | a tmux pane is hung: no progress and no completion (LivenessCenter edge: hung) |
| `LIVENESS_PANE_STAGNANT` | a tmux pane is busy but its output stopped changing (LivenessCenter edge: busy-stagnant) |

### loop

| Code | Meaning |
|---|---|
| `LOOP_ESCALATION_BOUNDARY` | the escalation boundary staged inbox items (bumped/filed/planned) after a cycle; fields carry the counts and the stage |
| `LOOP_FLEET_LANE_HALT` | a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed |
| `LOOP_HALT` | the batch halted at a wave boundary (plane diverged, sync refused); the reason is the halt error |
| `LOOP_MIN_WIDTH_REPAIR` | the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair) |
| `LOOP_PIPELINE_BLOCKER_HALT` | the pipeline-blocker breaker halted the batch (identical fingerprints, unexplained failures or consecutive failures over the ceiling); fields.rule and fields.fingerprint name the rule, the rest are the system-failure halt's own fields (next, escalation, inbox_item) |
| `LOOP_SYSTEM_FAILURE_HALT` | the batch halted on an ADR-0072 system failure the cycle itself signalled (the pipeline, not the task, is the cause); fields.category names the floor, fields.next is the escalation dossier's next_action, fields.escalation and fields.inbox_item the dossier and the P0 item the halt wrote |

### observer

| Code | Meaning |
|---|---|
| `OBSERVER_EVENTS_SINK_OPEN_FAILED` | the live per-phase observer could not open <phase>-observer-events.ndjson, so the phase runs UNOBSERVED (ADR-0030: the observer never blocks the phase); fields.step=open_sink, path |
| `OBSERVER_EVENT_APPEND_FAILED` | one line of <agent>-observer-events.ndjson was lost (fields.op = marshal | mkdir | open | write); reported ONCE per op for the observer's life, later losses on the same op are silent; an INCIDENT is still retained for the report when the open succeeded; fields.step=emit, op, path, event_type |
| `OBSERVER_KILL_FAILED` | the SIGTERM to the agent's process group returned an error (ESRCH: already gone; EPERM: not ours) — the kill was attempted exactly once; fields.step=respond, pgid, signal, kind |
| `OBSERVER_NUDGE_APPEND_FAILED` | the soft-stall nudge could not be appended to the agent inbox; the soft_stall_nudge envelope still emits and the nudge is not retried; fields.step=nudge, idle_s, threshold_s, agent |
| `OBSERVER_REPORT_WRITE_FAILED` | <agent>-observer-report.json could not be marshalled, its directory created, its .tmp written or renamed into place; the subcommand still exits 0 (the report is best-effort); fields.step=report, path |
| `OBSERVER_STALL_KILL_SENT` | the observer sent SIGTERM to the agent's process group after a stall INCIDENT — an act, not a fault — after the INCIDENT envelope was appended and before the signal; fields.step=respond, kind (stuck_no_output | stuck_no_progress | process_dead), pgid, action (legacy_enforce | kill_retry), action_reason, idle_s/threshold_s when the incident carries them |
| `OBSERVER_STDOUT_TAIL_FAILED` | the agent's stdout log could not be stat'ed (a non-ENOENT error), opened, seeked or fully scanned (a line over the 10 MiB buffer); reported ONCE per op; an absent log stays silent by design (tmux drivers dump it at exit); the prior offset is kept on stat/open/seek, the file size after a scan error; fields.step=tail, op, path |
| `OBSERVER_WATCHER_LEAKED` | the live observer's watcher goroutine did not exit within the bound after the phase finished; the goroutine and its sink fd are leaked on purpose (closing would race its writes) and the OS reclaims them at exit; fields.step=cancel, timeout_s |

### orchestrator

| Code | Meaning |
|---|---|
| `ORCHESTRATOR_CYCLE_FAILED` | the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision |
| `ORCHESTRATOR_GATE_CORRECTION` | the correction ladder ran a rung after a gate rejection — fields name the correction ordinal, the budget (max), the rung, the CLI re-dispatched on and whether that CLI was escalated; the reason is the rejection being corrected |
| `ORCHESTRATOR_PHASE_ABORTED` | the cycle aborted after this phase's outcome (review reject, guard, persistence); the abort reason is in fields.abort_reason |
| `ORCHESTRATOR_PHASE_VERDICT_FAIL` | a phase recorded verdict FAIL; the reason is the phase's own error-severity diagnostics |
| `ORCHESTRATOR_PHASE_VERDICT_WARN` | a phase recorded verdict WARN; the reason carries its error-severity diagnostics, if any |
| `ORCHESTRATOR_QUOTA_PAUSED` | every CLI family is quota-exhausted; the cycle is paused at the named phase and resumable |
| `ORCHESTRATOR_SYSTEM_FAILURE` | an ADR-0072 system-level failure was attached to the cycle (INCIDENT when it halts the loop, WARN otherwise); fields.category names the floor |

### outcome

| Code | Meaning |
|---|---|
| `OUTCOME_SIDECAR_SKIPPED` | the phase's <phase>-usage.json sidecar was not written because the workspace is empty (the CWD-relative leak guard); the in-memory record and the phase.outcome event still stand |
| `OUTCOME_SIDECAR_WRITE_FAILED` | the phase's <phase>-usage.json sidecar could not be encoded or written; the reason names the step and the error, fields name the phase and the path; the in-memory record stands |
| `OUTCOME_TIMING_SKIPPED` | phase-timing.json was not written because the workspace is empty; the composed timing set is returned to the caller unchanged |
| `OUTCOME_TIMING_WRITE_FAILED` | phase-timing.json could not be encoded or written (marshal, temp write or rename); the reason names the step and the error; the composed set is still returned |

### ship

| Code | Meaning |
|---|---|
| `SHIP_ARGS` | args: invalid ship arguments |
| `SHIP_AUDIT_BINDING_ARTIFACT_MISSING` | verify-class: the audit artifact the ledger binds is missing |
| `SHIP_AUDIT_BINDING_ARTIFACT_SHA` | verify-class: the audit artifact's SHA does not match the ledger binding |
| `SHIP_AUDIT_BINDING_AUDITOR_EXIT` | verify-class: the auditor exited abnormally, so its verdict cannot bind |
| `SHIP_AUDIT_BINDING_DUAL_VERDICT` | verify-class: the bound audit report carries two contradicting verdicts |
| `SHIP_AUDIT_BINDING_HEAD_MOVED` | verify-class: main HEAD moved after the audit bound the tree |
| `SHIP_AUDIT_BINDING_MALFORMED_VERDICT` | verify-class: the bound audit report carries no parseable verdict |
| `SHIP_AUDIT_BINDING_NO_AUDITOR` | verify-class: no auditor ledger entry binds this cycle |
| `SHIP_AUDIT_BINDING_NO_LEDGER` | verify-class: the ledger is unreadable, so no audit binding can be verified |
| `SHIP_AUDIT_BINDING_STALE` | verify-class: the audit binding predates the tree being shipped |
| `SHIP_AUDIT_BINDING_TREE_MISMATCH` | verify-class: the audited tree differs from the tree being shipped |
| `SHIP_AUDIT_BINDING_VERDICT_FAIL` | verify-class: the bound audit verdict is FAIL |
| `SHIP_AUDIT_BINDING_VERDICT_WARN_STRICT` | verify-class: the bound audit verdict is WARN under strict audit |
| `SHIP_COMMIT_GATE_MALFORMED` | verify-class: the commit-gate attestation cannot be parsed |
| `SHIP_COMMIT_GATE_MISSING` | verify-class: no commit-gate attestation exists for the staged tree |
| `SHIP_COMMIT_GATE_STALE` | verify-class: the commit-gate attestation is for a different tree |
| `SHIP_COMMIT_PREFIX_GATE` | atomic-ship: the commit-message prefix gate refused the message |
| `SHIP_CONTROL_PLANE_VIOLATION` | verify-class: a cycle-class commit touches the control-plane integrity surface (ADR-0064); ship it manually |
| `SHIP_EGPS_RED_COUNT` | verify-class: the EGPS gate reports red evals; red_count must be 0 to ship |
| `SHIP_EXPLANATION_DOCUMENTATION` | verify-explanation: the change ships without the explanation documentation its class requires |
| `SHIP_GIT_COMMIT_FAILED` | atomic-ship: the commit step failed |
| `SHIP_GIT_DETACHED_HEAD` | atomic-ship: the tree is on a detached HEAD |
| `SHIP_GIT_FF_MERGE_DIVERGED` | atomic-ship: the fast-forward merge to main diverged |
| `SHIP_GIT_FLEET_REBASE_CONFLICT` | atomic-ship: the fleet rebase hit a genuine merge conflict; routed to the debugger (integrity, ADR-0049 G13a) |
| `SHIP_GIT_FLEET_REBASE_NEEDED` | atomic-ship: a peer lane moved main; rebase and re-verify the merged tree (transient, ADR-0049 S5b) |
| `SHIP_GIT_IO` | git I/O failure outside a named stage |
| `SHIP_GIT_PUSH_REJECTED` | atomic-ship: the remote rejected the push |
| `SHIP_GIT_STAGE_FAILED` | atomic-ship: staging failed (transient) |
| `SHIP_INTEGRITY_TREE_DRIFT` | post-ship: the shipped tree drifted from the verified one (integrity) |
| `SHIP_INVALID_CLASS` | verify-class: unknown ship class |
| `SHIP_LANDING_BINARY_RESET_FAILED` | git checkout HEAD -- <binary> before the ff-merge exited non-zero or failed to spawn; the merge still runs and may fail if the tracked binary is dirty; fields.step=integrate, path, git_rc, git_err |
| `SHIP_LANDING_BINDING_WRITE_FAILED` | ship-binding.json could not be created, written or renamed into the run workspace; the push already landed, the caller keeps shipping and keeps its WARN log line; fields.step=binding, path, err |
| `SHIP_LANDING_HEAD_READ_FAILED` | git rev-parse HEAD after the push landed errored or returned empty; the result's CommitSHA (the dossier's delivery identity) stays empty and the ship proceeds; fields.step=push, ref, err |
| `SHIP_LANDING_PUSH_REPAIR_DECLINED` | the inline fetch + fast-forward retry after a rejected push declined at the named probe (fetch, origin_ref, head or push_retry); the original transient GIT_PUSH_REJECTED is returned with repair_attempted/repair_outcome=declined stamped; fields.step=push, branch, probe |
| `SHIP_MANIFEST_GATE` | atomic-ship: a staged path was declared by no build/TDD report (cross-lane leak guard) |
| `SHIP_MANUAL_DECLINED` | verify-class: the operator declined the manual ship |
| `SHIP_MANUAL_NOT_TTY` | verify-class: a manual ship needs an interactive confirmation (or the auto-confirm environment) |
| `SHIP_REPO_CONTRACT_GATE` | atomic-ship: a repo-contract guard suite is RED in the lane worktree; pushing would red main |
| `SHIP_REPO_CONTRACT_INFRA` | atomic-ship: the repo-contract scanner toolchain died twice without a test-level failure (re-dispatchable) |
| `SHIP_SELF_SHA_IO` | verify-self-sha: the SHA pin could not be read or written |
| `SHIP_SELF_SHA_TAMPERED` | verify-self-sha: the ship binary's SHA does not match the pinned one (tamper or unpinned rebuild) |
| `SHIP_STATE_IO` | state file I/O failure |
| `SHIP_TRIVIAL_CRITICAL_PATHS` | verify-class: a trivial-class ship touches critical paths |
| `SHIP_TRIVIAL_NOT_TRIVIAL` | verify-class: a trivial-class ship exceeds the trivial diff bound |
| `SHIP_UNKNOWN` | an unclassified ship failure |
| `SHIP_WORKTREE_RESOLVE` | atomic-ship: the cycle worktree could not be resolved |

### signalcenter

| Code | Meaning |
|---|---|
| `SIGNALCENTER_BAD_ORIGIN` | an event's origin is not a Func or Type.Method name; raw value in fields.raw_origin |
| `SIGNALCENTER_LISTENER_PANICKED` | a listener panicked and was unsubscribed; the panic value is in the reason |
| `SIGNALCENTER_MISSING_CODE` | a WARN or INCIDENT event carried no code |
| `SIGNALCENTER_MISSING_REASON` | an event carried no reason |
| `SIGNALCENTER_SINK_DROPPED` | the durable sink had no path for N events (fields.dropped) before this write |
| `SIGNALCENTER_UNKNOWN_KIND` | an event named a kind outside the closed set; raw value in fields.raw_kind |
| `SIGNALCENTER_UNKNOWN_MODULE` | an event named a module outside the closed set; raw value in fields.raw_module |
| `SIGNALCENTER_UNKNOWN_SEVERITY` | an event carried a severity outside INFO/WARN/INCIDENT; raw value in fields.raw_severity |
| `SIGNALCENTER_UNREGISTERED_CODE` | an event carried a code its module never registered; raw value in fields.raw_code |

<!-- GENERATED:signal-codes END -->

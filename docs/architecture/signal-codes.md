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

### orchestrator

| Code | Meaning |
|---|---|
| `ORCHESTRATOR_CYCLE_FAILED` | the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision |
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

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

### advisor

| Code | Meaning |
|---|---|
| `ADVISOR_CAPTURE_WRITE_FAILED` | a redacted capture artifact (advisor-prompt-, advisor-response- or advisor-span-<kind>) could not be persisted (fields.artifact = prompt / response / span; fields.op = marshal / write — marshal is dormant, a Span always marshals); the decision still returns and the remaining artifacts are still attempted; the ledger binds nothing for an absent capture; fields.step=capture, path, decision, contract |
| `ADVISOR_LAUNCH_FAILED` | the routing/plan dispatch produced no response — the preflight refused (nil bridge, empty workspace, the depth guard; fields.step=preflight) or every CLI in the router profile's fallback chain failed (fields.step=dispatch, cli, chain, exit_code, profile); the error is still returned and the caller degrades to the static spine or keeps the initial plan; fields.decision, contract |
| `ADVISOR_MINT_REJECTED` | a plan entry minted a reserved control-plane identity (router/advisor/failure-advisor and their aliases) and was dropped by the recursion guard; the rest of the plan stands, one event per drop at decision time (never on resume or replay); fields.step=mint, minted_phase, decision, contract |
| `ADVISOR_PROFILE_LOAD_FAILED` | .evolve/profiles/router.json exists but could not be read or parsed (absence is silent); the dispatch degrades to the single primary CLI exactly as before, so a configured fallback chain is silently narrower than the operator believes; fields.step=dispatch, path, decision, contract |
| `ADVISOR_RECON_GIT_FAILED` | the pre-plan recon's recent-files reader (git log over the project root) failed while the recon digest was on; the digest is composed without file facts and planning proceeds; fields.step=compose, project_root, decision, contract |
| `ADVISOR_RESPONSE_UNPARSEABLE` | the launch returned but no decision decoded from its output (fields.cause = no_json / invalid_json / empty); the wrapped error is still returned and the caller degrades — on the per-transition Propose path this is the first visibility the fault ever had; fields.step=parse, stdout_bytes, artifact, decision, contract |

### audit

| Code | Meaning |
|---|---|
| `AUDIT_CIPARITY_CHANGESET_UNDERIVABLE` | the cycle's changed-package set was underivable (git failed — e.g. a concurrent-fleet .git/index.lock race) and a whole-repo CI-parity gate (go vet, acs-durable, integration tier) SKIPPED fail-open with a WARN diagnostic; one event per gate, CI backstops the check; fields.gate, root |
| `AUDIT_CIPARITY_GATE_FAILED` | a CI-parity gate returned offenders — the audit FAILs and the cycle routes to retro; fields.cause ∈ {exit, retake_exec_failed, deadline_with_markers, retake_red, underivable, apicover, ungraduated}, fields.gate, offenders (count), first (the first offender), log (the integration-tier.log when written), exit |
| `AUDIT_CIPARITY_GATE_STEP_FAILED` | a CI-parity gate's pre-verdict step could not run and the gate failed OPEN (the diagnostic is the WARN the audit renders): fields.step ∈ {exec, tier_attempt, tier_list, bin_dir, cover_run, cover_func, write_func_cover, pkg_dirs, measure}; fields.gate, err, cmd (the exec/list/fork steps) |
| `AUDIT_CIPARITY_GRADUATION_DEFERRED` | an ungraduated new package has no production .go surface (test-only or absent) so the graduation gate did not flag it; the enrollment obligation re-raises when production code lands; fields.gate, pkg, dir |
| `AUDIT_CIPARITY_TIER_DEADLINE_NO_VERDICT` | the serialized integration-tier retake hit its per-attempt budget with no test verdict in the truncated output — a deadline kill is not a judgment, degraded to WARN; fields.gate, budget, log, lock_note |
| `AUDIT_CIPARITY_TIER_ENV_EXCLUSIVE_SKIPPED` | touched package(s) are env-exclusive under a live loop: scope=all — the integration tier skipped with a WARN; scope=mixed — the runnable remainder ran without them; fields.gate, scope, pkgs, remainder, backstop (the record's honest backstop, verbatim) |
| `AUDIT_CIPARITY_TIER_FLAKE_ABSORBED` | the integration tier was RED under fleet contention but GREEN on the serialized clean-env retake — a contention flake absorbed as a WARN, not a code defect; fields.gate, attempt1_exit, log, lock_note |
| `AUDIT_CIPARITY_TIER_LOCK_UNAVAILABLE` | the cross-lane integration-tier retake lock degraded (best-effort by design — the retake ran unserialized): fields.reason ∈ {no_root, error, timeout}; fields.gate, path, err, wait |
| `AUDIT_CIPARITY_TIER_LOG_WRITE_FAILED` | integration-tier.log could not be opened or appended for an attempt (an empty Workspace is a declared no-op, not a fault); an earlier successful write keeps its pointer in the offenders; fields.gate, path, attempt, err |
| `AUDIT_CIPARITY_TIER_RETAKE_EXEC_FAILED` | the serialized integration-tier retake could not START (silent before unit 14); the gate still FAILs on attempt 1's offenders — a real red is never laundered by retake infra trouble — and a GATE_FAILED cause=retake_exec_failed follows; fields.gate, err, log |
| `AUDIT_LEDGER_ANCESTOR_EMPTY` | the ancestor left no reconcilable defect-ledger.json (absent or empty), so NO inherited defect is enforced this cycle; expected for an ancestor that predates the ledger, but a deleted ledger looks identical — recorded, never assumed benign; fields.step=grade, ancestor_cycle, path |
| `AUDIT_LEDGER_DEFECTS_UNACCOUNTED` | inherited defects are neither FIXED with resolving evidence nor DEFERRED with a reason (fields.count; the first ids in fields.ids, ids_truncated when more); the cycle is blocked — disposition each id in defect-dispositions.json (the per-id reasons are in the diagnostic and the written-back ledger); fields.step=grade, ancestor_cycle, path |
| `AUDIT_LEDGER_DISPOSITIONS_INCOMPLETE` | defect-dispositions.json covers only some inherited OPEN ids (fields.open, covered, the first uncovered ids, uncovered_truncated when more); the cycle is blocked — finish the file that exists; fields.step=preflight, ancestor_cycle, path |
| `AUDIT_LEDGER_DISPOSITIONS_MISSING` | a continuation owing dispositions holds no defect-dispositions.json at all (fields.open inherited OPEN ids); the cycle is blocked — author the file from scratch, one entry per inherited id; fields.step=preflight, ancestor_cycle, path |
| `AUDIT_LEDGER_DISPOSITIONS_UNREADABLE` | defect-dispositions.json is present but cannot be read or parsed (fields.op = read | parse; an object, number or bool evidence shape is a parse fault); the cycle is blocked — the diagnostic carries the expected schema; fields.step=read, path |
| `AUDIT_LEDGER_EMIT_FAILED` | a rejecting audit's defect ledger could not be read or written while recording this cycle's defects (fields.op = read | parse | write); the verdict stands, a later continuation has nothing to reconcile against — repair the workspace ledger; fields.step=emit, path |
| `AUDIT_LEDGER_LINEAGE_DISAGREES` | the workspace manifest and the root-owned registry name different ancestors; the rewritable copy is the suspect and the cycle is blocked until the disagreement is resolved; fields.step=arm, manifest_cycle, registry_cycle |
| `AUDIT_LEDGER_MANIFEST_MISSING` | the root-owned continuation registry binds this lane's scope to an ancestor but the workspace holds no manifest — it was deleted or never written; inherited defects are reconciled from the registry binding and the cycle is blocked; the missing manifest is the finding; fields.step=arm, registry_path, ancestor_cycle |
| `AUDIT_LEDGER_MANIFEST_UNREADABLE` | the workspace continuation-manifest.json is present but unreadable, so the continuation cannot be graded against its lineage; the cycle is blocked from PASS (cycle-1285 F2) — repair the manifest; fields.step=arm, workspace |
| `AUDIT_LEDGER_OVERFLOW` | a rejecting audit carried more defects than the ledger cap; the first rows are recorded and ONE synthetic OPEN row stands for the truncated tail (fields.overflow, cap) — fix the emitter or widen the cap; fields.step=emit, path |
| `AUDIT_LEDGER_PROMPT_DEGRADED` | while composing the audit prompt the continuation manifest (fields.reason=manifest; fallback = registry | none) or the ancestor ledger (fields.reason=ledger, op = read | parse) could not be read, so the auditor was NOT told its inherited ids; stream-only — the same fault blocks at Classify under its own WARN code; fields.step=prompt, path |
| `AUDIT_LEDGER_UNREADABLE` | the ancestor's or this cycle's own defect-ledger.json is present but unreadable (fields.which = ancestor | own, op = read | parse); the continuation is blocked — repair the file named by fields.path; fields.step=grade, ancestor_cycle |
| `AUDIT_LEDGER_WRITEBACK_FAILED` | the reconciled ledger could not be written back into the workspace; an invisible disposition is not a disposition, so the cycle is blocked; fields.step=grade, path |

### bridge

| Code | Meaning |
|---|---|
| `BRIDGE_BOOT_STRIKE_CLEAR_FAILED` | the boot-strike store could not clear the driver's consecutive boot-timeout strike after a non-80 exit (the REPL booted); the strike count may stay stale and bench the driver early; fields step=clear_boot_strike, call_id, cli, agent |
| `BRIDGE_BOOT_STRIKE_RECORD_FAILED` | the boot-strike store could not record the driver's boot-timeout strike after exit 80; the bench never escalates for this driver; the launch error still wraps the transient sentinel; fields step=record_boot_strike, call_id, cli, agent |
| `BRIDGE_CONTEXT_FILL_HIGH` | an attempt's context fill crossed the configured warn threshold; the reason names the fill and the threshold |
| `BRIDGE_EXIT_ARTIFACT_TIMEOUT` | the artifact never appeared within the wait window (exit 81); wraps core.ErrArtifactTimeout — one code whatever the sub-cause, which rides cause_code (context_cancelled, completion_detector_error, submit_wedged, transient_upstream, review_stop, review_pause, incomplete); the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_BAD_FLAGS` | a launch died in the validate gauntlet — bad flags, a missing profile, an unreadable or empty prompt, no driver for the CLI (exit 10); plain failure; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_COMMAND_TIMEOUT` | the driver was killed by a command-level timeout (exit 124, the gnu timeout convention); transient — infra weather, the sibling of 81; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_COST_LEAK` | a launch refused to leak a forbidden credential into the inner CLI's environment (exit 3); plain failure; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_DRIVER_ERROR` | a launch exited with a code the table does not name; plain failure, ledger cause driver_error; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_MISSING_BINARY` | a required external binary is missing (exit 127); deliberately plain — an absent CLI is an environment defect, and the family fallback sees the raw 127; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_REPL_BOOT_TIMEOUT` | a tmux REPL never showed its prompt marker within the boot budget (exit 80); transient — the boot strike is recorded before this event; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_REQUIRED_TIER_UNAVAILABLE` | --require-full was set and the full model tier is unavailable (exit 99); plain failure; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_RESPOND_LOOP_GUARD` | the auto-respond loop guard tripped (exit 86); transient; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_SAFETY_GATE` | a launch died in a safety gate (exit 2: e.g. --human-input without the host opt-in); plain failure; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_SIGNAL_DEATH` | the driver died of a signal or failed to start (exit -1): transient under a cancelled context (our own teardown — the deliverable-authority door), plain with a live one (a start failure); the ledger cause stays driver_error; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_UNKNOWN_PROMPT` | the auto-responder met an interactive prompt it could not answer and wrote the escalation report (exit 85); transient; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify |
| `BRIDGE_FRESH_SESSION_RETRY` | a fatal-pane fast-fail ended an ephemeral-session launch on a session-recoverable cause (dead_shell, cli_self_updated: the REPL process is gone, the CLI and account are fine); the dead dispatch is on the ledger marked fresh_session_retry and ONE fresh session of the same CLI runs before the caller's chain walks on — never for a named session, a delivered run, or a deadline with no room for another wait interval (F31); fields call_id (the dead dispatch), cli, agent |
| `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED` | the captured launch stderr could not be persisted as <workspace>/<agent>-launch-error.txt after a non-zero exit (the forensic file a validate-gauntlet death leaves); the classified error is unchanged and the BRIDGE_EXIT_* event carries no launch_error field; fields step=persist_launch_error, path, call_id, cli, agent |
| `BRIDGE_RESULT_READ_FAILED` | the launch exited 0 but its result (the artifact, or the stdout scrollback under the stdout completion contract) could not be read into the response; Launch still returns nil with an empty Stdout — the on-disk report is the verdict source; fields step=read_result, path, completion, call_id, cli, agent |
| `BRIDGE_SUBAGENT_ADAPTER_EXEC_FAILED` | the bridge adapter returned an infrastructure error (exit -1 on a launch fault); the ledger line is still written and the verdict fields carry what the artifact ladder found; the ONE outcome signal of such a run; fields.step=exec, exit_code, verdict, integrity |
| `BRIDGE_SUBAGENT_ARTIFACT_HASH_FAILED` | the artifact stood the ladder (PASS or FAIL) but could not be hashed, so the ledger line stamps artifact_sha256="" — silent before unit 16; fields.step=hash, artifact |
| `BRIDGE_SUBAGENT_ARTIFACT_INTEGRITY_FAIL` | the adapter returned but the artifact failed the integrity ladder (fields.rung = missing | stale | unreadable | empty | token_missing); the reason is the ladder's diagnostic the run's result used to drop; fields.step=verify, artifact, exit_code |
| `BRIDGE_SUBAGENT_GIT_STATE_UNKNOWN` | git HEAD or the tree-diff sha could not be captured (an error or an empty value), so the ledger line stamps "unknown" — silent before unit 16; fields.step=provenance, head, tree_state, project_root |
| `BRIDGE_SUBAGENT_LEDGER_WRITE_FAILED` | the agent_subprocess ledger line or its tip could not be written (fields.op = mkdir | chain_link | open | write | close | tip_tmp | tip_rename); the returned error keeps its bare text and masks any adapter error, exactly as before; fields.step=ledger, path |
| `BRIDGE_SUBAGENT_LLM_RESOLVE_FALLBACK` | the LLM router returned an error, so the cli was taken from the profile's cli field with source=profile — silent before unit 16; fields.step=cli, source, cli |
| `BRIDGE_SUBAGENT_PREPARE_FAILED` | the artifact directory, the challenge token, the prompt read or the prompt temp file failed before the adapter ran (fields.step = artifact_dir | token | prompt | stage; fields.op = create | write on stage) |
| `BRIDGE_SUBAGENT_REQUEST_REJECTED` | an `evolve subagent run` request was rejected at admission (no prompt reader, an unknown agent role, a negative cycle, a missing workspace, the retired in-process escape hatch, or the recursion depth cap); no port was consulted; fields.step=validate, reason_class |
| `BRIDGE_SUBAGENT_RESOLUTION_FAILED` | the dispatch could not resolve its profile, cli, driver, model tier or capability manifest (fields.step names which: profile | cli | driver | tier | capability; the profile step's reason carries the read error the returned text drops) |
| `BRIDGE_SUBAGENT_VERDICT_FAIL` | the artifact is sound but the adapter exited non-zero (the bridge's exit codes reach here as fields.exit_code); fields.step=verify, cli, artifact |
| `BRIDGE_SUBAGENT_WORKTREE_FALLBACK` | WorktreePath was not propagated, so WORKTREE_PATH falls back to the project root and the agent runs against the main tree (the Warns entry callers print is kept); fields.step=env, worktree, project_root |
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

### inbox

| Code | Meaning |
|---|---|
| `INBOX_CLAIM_MOVE_FAILED` | a claim could not create processing/cycle-N/ (fields.step=mkdir, dest_dir) or could not rename the item into it (fields.step=rename — the item may already be claimed); ErrMvFailed (exit 2), the item stays where it was; fields.task_id, err |
| `INBOX_CLAIM_NOT_FOUND` | a lane claim named an id the inbox root does not hold (absent, already claimed into processing/, or parked); the claim returns ErrNotFound (exit 1) and moves nothing — the FAIL closeout's lane-scope claim of an id the triage persona already claimed reports this for an expected state; fields.step=locate, task_id, inbox_dir |
| `INBOX_CLAIM_REFUSED` | a lane claim named an operator-owned item (route:console-* or a protected fix surface, ADR-0074 I1); the claim returns ErrConsoleRouted (exit 3) and the item stays at the root; fields.step=route, task_id, reason |
| `INBOX_CONTINUATION_MANIFEST_UNREADABLE` | the FAILed cycle's continuation manifest exists but could not be read or parsed; every item of the drain releases unstamped (a later claim starts fresh instead of resuming the salvage); fields.step=manifest, workspace, err |
| `INBOX_ITEM_REWRITE_FAILED` | an item record could not be rewritten atomically: the closeout's route-console rewrite (step=route), the quarantine release's counter reset (fields.step=counter_reset — the item releases with its stale count), the drain's failure_count bump (fields.step=failure_bump — quarantine is skipped, the item releases to the root) or the drain's continuation stamp (fields.step=continuation_stamp — the item releases unstamped); fields.task_id, path, err |
| `INBOX_ITEM_ROUTED_CONSOLE` | the FAIL closeout routed an item to console-manual because a phase gate refused it deterministically (a top_n card naming a protected surface): fields.task_id, path, reason, step=route — the item is operator-owned from here on and no lane claims it again (the per-item breaker of the 2026-09-14 poison-loop incident) |
| `INBOX_LANDED_CHECK_FAILED` | the landing probe could not answer for a processed-promotion's sha — in production the host's git probe (shaLandedOnMain) on an exec fault or a git exit outside {0,1}: an unknown sha, no local main, a non-git ProjectRoot; the promote fails OPEN (the sha is treated as landed) so a gate fault never blocks a promotion, and the item lands in processed/ with this line as the only trace; fields.step=landing, task_id, sha, err |
| `INBOX_PROMOTE_MOVE_FAILED` | a promote could not create its destination dir (fields.step=mkdir — ErrMvFailed, NoOp false, ledger promote-warn/mkdir-failed) or could not rename the item into it (fields.step=rename — the compat NoOp success, ledger promote-warn/mv-failed); the item stays where it was; fields.task_id, state, src_rel, dest, err |
| `INBOX_PROMOTE_NOT_FOUND` | a promote named an id neither processing/cycle-*/ nor the inbox root holds — already moved; the ship.sh-compat NoOp success is returned; fields.step=locate, task_id, state |
| `INBOX_PROMOTE_UNLANDED_SHA` | a processed-promotion carried a ship sha the landing probe says is not on main; the item is rerouted to retry/ under reason ship-promote-retry-unlanded-sha instead of buried in processed/; fields.step=landing, task_id, sha, state=retry |
| `INBOX_QUARANTINE_FAILED` | an item at the ADR-0072 S5 retry ceiling could not be parked in quarantine/ (fields.outcome=error: the park's promote errored; outcome=noop: the park's rename failed) and falls open to a root release — the poison item WILL be re-picked; the preceding INBOX_PROMOTE_MOVE_FAILED from Mover.Promote names the cause; fields.step=quarantine, task_id, failure_count, ceiling |
| `INBOX_RELEASE_DOUBLE_MOVE` | the cycle drain found the item's basename already at the inbox root (a concurrent release landed it first); the root copy is never clobbered, the processing copy stays and is not counted; fields.step=release_cycle, task_id, base |
| `INBOX_RELEASE_MOVE_FAILED` | an item could not be renamed back to the inbox root — by the cycle drain (fields.step=release_cycle, origin Mover.Release) or by orphan recovery (fields.step=recover_orphans, origin Mover.RecoverOrphans); the item stays in processing/cycle-N/ and the walk continues; fields.task_id, base, err |
| `INBOX_ROUTE_NOT_FOUND` | the FAIL closeout could not find the item it was told to route (fields.task_id, inbox_dir, step=locate) — or found it held by another cycle's claim (fields.held_by_cycle) — the refusal is NOT recorded on it and it WILL be re-picked; neither processing/cycle-*/ nor the inbox root holds the id |

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
| `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED` | the boundary-refresh-log.jsonl audit entry could not be appended AFTER the state.json pin had already moved; the refresh continues to re-exec — the pin and the audit trail now disagree; fields.batch, path, error |
| `LOOP_BOUNDARY_REFRESH_SKIPPED` | the boundary binary refresh degraded to 'continue on the current binary' at fields.step (ahead_check | lane_check | lane_active | breaker | rebuild | target | repin | argv | arm | reexec): the ahead-check errored, a sibling fleet lane held a fresh lease (or the check was unverifiable), the loop breaker refused a second refresh for the same build commit, `make -C go build` failed, no executable at go/bin/evolve, the state.json re-pin was refused, the re-exec argv was empty, the breaker marker could not be written, or exec failed; fields.batch, commit (the running build commit), error |
| `LOOP_CHAIN_BATCH_ERROR` | a chained batch exited with a non-continuable rc (not 0/3/5) and the chain stopped, propagating rc; the batch's own signals name the failure (an rc=2 batch may have emitted none); fields.batch, rc, stop_reason=chain_batch_error |
| `LOOP_CHAIN_INBOX_ITEM_INVALID` | a root-level .evolve/inbox/*.json did not parse as an inbox item (no object with a non-empty id) and is NOT counted as pending work — usually a real todo lost to a typo; fields.batch, name, path |
| `LOOP_CHAIN_INBOX_UNREADABLE` | the chain could not read .evolve/inbox (a read error other than not-exist) and stopped rather than loop blind; fields.batch, path, error, stop_reason=chain_inbox_unreadable, exit=2 |
| `LOOP_CHAIN_QUOTA_DEFER` | a chained batch exited with the resumable rc=5 quota-pause code; the chain defers instead of relaunching into the wall (resume with evolve loop --resume); fields.batch, cycle, wake_at, source are the checkpoint block's (empty when no block is on disk) |
| `LOOP_ESCALATION_BOUNDARY` | the escalation boundary staged inbox items (bumped/filed/planned) after a cycle; fields carry the counts and the stage |
| `LOOP_FLEET_LANE_HALT` | a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed |
| `LOOP_HALT` | the batch halted at a wave boundary (plane diverged, sync refused); the reason is the halt error |
| `LOOP_MIN_WIDTH_REPAIR` | the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair) |
| `LOOP_PIPELINE_BLOCKER_HALT` | the pipeline-blocker breaker halted the batch (identical fingerprints, unexplained failures or consecutive failures over the ceiling); fields.rule and fields.fingerprint name the rule, the rest are the system-failure halt's own fields (next, escalation, inbox_item) |
| `LOOP_SYSTEM_FAILURE_HALT` | the batch halted on an ADR-0072 system failure the cycle itself signalled (the pipeline, not the task, is the cause); fields.category names the floor, fields.next is the escalation dossier's next_action, fields.escalation and fields.inbox_item the dossier and the P0 item the halt wrote |
| `LOOP_WAVE_ALL_LANES_STALE` | every planned lane of the wave was stale at the last-moment freshness gate (consumed, or a declared dep still unmet) and the backlog refill found nothing, so the wave launched nothing — a shorter wave, never a doomed lane; the per-lane skips are fleet's own freshness-gate lines; fields.planned, skipped |
| `LOOP_WAVE_DISPATCH_FAILED` | a wave (fields.path=wave) or the min-width repair (path=repair) could not dispatch: the control-plane preflight refused (step=preflight — uncommitted control-plane edits in the main checkout), the triage plan could not be produced (step=plan — no prior triage decision and the inbox seed found fewer than 2 file-disjoint lanes, or the last-cycle read failed) or the plan could not be adapted into disjoint lanes (step=adapt); the batch falls back to the sequential path, the only unisolated execution mode; fields.error is the step's own error |
| `LOOP_WAVE_EMPTY_PLAN` | the wave planned zero lanes and the batch falls back to sequential: cause=empty_triage_plan — fleet.count <= 1 so the one-lane repair guard is not met (the operator wanted one lane; fires whatever the dispatcher's own reason was); cause=empty_backlog — the guard was met but the one-lane repair found no disjoint candidate; fields.desired = fleet.count, realized = the wave-sized count |

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
| `ORCHESTRATOR_AUDIT_REPAIR_DECLINED` | an audit FAIL earned no repair round and the cycle goes to retro; fields.reason is the retry envelope's verdict (unrecognised class, budget spent, system-level or non-retry class, no class declared), fields.declared_class the audit's own class |
| `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` | an audit FAIL earned a repair round; fields.next is the re-entry phase (tdd | build), fields.attempt the repair attempt about to be spent, fields.reason the envelope's basis |
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

### runner

| Code | Meaning |
|---|---|
| `RUNNER_DELIVERABLE_UNVERIFIED` | a CONTRACTED deliverable was still not well-formed after the settle window on the clean-exit path AND the final verdict is FAIL — the ship guard downgraded a clean-ship verdict (fields.downgraded=true) or Classify itself returned FAIL on the malformed or absent bytes; fault-only, a legitimate WARN/SKIPPED pass-through emits nothing; fields.codes, verdict_before, verdict, settle_attempts, deliverable |
| `RUNNER_OPTIONAL_PHASE_DEGRADED` | an optional phase hit a bridge infra teardown with no trustworthy deliverable and degraded to WARN; the cycle continues — a stream and console ADDITION (the arm was a response diagnostic only); fields.teardown, exit, cause, verr, stale_leftover, settle_attempts, deliverable |
| `RUNNER_RECONCILED` | a bridge infra teardown was overridden by a well-formed deliverable (fields.via=verify) or by the ACS deterministic floor (via=acs_floor, overridden_codes); the response carries Reconciled=true and core files the reconciled_timeout ledger disposition — ONE event where two INFO lines (ACS-FLOOR, RECONCILED) fired before; fields.verdict, deliverable, teardown, exit, settle_attempts |
| `RUNNER_STDOUT_FILTER_FAILED` | the clean-stdout companion of the phase's raw log (the logfilter writer's <phase>-stdout.clean.txt) could not be written; the phase continues and the raw log stays the forensic source; fields.workspace (the phase is the event's own) |
| `RUNNER_TEARDOWN_FAIL` | a bridge INFRA teardown (artifact-wait timeout or transient failure) ended the session and no trustworthy deliverable rescued it — a mandatory phase FAILs and core's retry loop classifies the wrapped sentinel; the reason is the FAIL diagnostic's own text; fields.teardown = timeout / transient, exit, cause = stale_leftover / unverifiable / malformed, codes, verr, roots (ws= wt= evolve=), report and acs (size=N tail=… or absent), stale_leftover, settle_attempts (re-probes: never flushed vs malformed), deliverable |

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

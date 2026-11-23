# Signal Center codes

> Generated from the code registry — `signalcenter.RegisterCode`, one call per code, in the module
> that owns it — by `evolve signals codes generate`. `evolve signals codes check` (run by a `cmd/evolve`
> test, so by CI) exits 2 when this file drifts from the registry. Design:
> [signal-center-design.md](signal-center-design.md) §5.4 · [ADR-0101](adr/0101-signal-center.md).

Every WARN or INCIDENT signal carries one of these codes; an INFO signal can carry one too. A code names the
**rule** that fired (`MODULE_SNAKE_CASE`, prefix = the owning module); `kind` names the event and
`origin` the function. Triage kind-first (design §9): the kind says what happened, the code says
which rule decided it, `fields` name the artifact to open. The Center does not drop a code that
is not registered here. It stamps the event `SIGNALCENTER_UNREGISTERED_CODE` and keeps the raw
value in `fields.raw_code`, so the drift is visible in the same file.

Ship codes are the `shiperr` vocabulary projected verbatim (`SHIP_` + code, `shiperr.SignalCode`);
the ship phase's own `ship-error.json` and ledger entries keep the unprefixed spelling.

<!-- GENERATED:signal-codes BEGIN — do not edit by hand; run `evolve signals codes generate` -->

### advisor

| Code | Meaning |
|---|---|
| `ADVISOR_CAPTURE_WRITE_FAILED` | a redacted capture artifact (advisor-prompt-, advisor-response-, or advisor-span-<kind>) failed to persist (fields.artifact = prompt / response / span; fields.op = marshal / write). The decision returns, remaining artifacts run, and the ledger binds nothing for absent captures. Fields: fields.step=capture, path, decision, contract |
| `ADVISOR_LAUNCH_FAILED` | the routing/plan dispatch gave no response. Preflight refused (fields.step=preflight) or every CLI in the fallback chain failed (fields.step=dispatch, cli, chain, exit_code, profile). The error returns and caller keeps the static spine or first plan. fields.decision, contract |
| `ADVISOR_MINT_REJECTED` | a plan entry minted a reserved control-plane identity (router, advisor, failure-advisor, or aliases) and was dropped by the recursion guard. The rest of the plan stands; one event per drop at decision time (never on resume or replay). Fields: fields.step=mint, minted_phase, decision, contract |
| `ADVISOR_RECON_GIT_FAILED` | the pre-plan recon git log reader over the project root failed while the recon digest was on. The digest composes without file facts and planning proceeds. Fields: fields.step=compose, project_root, decision, contract |
| `ADVISOR_RESPONSE_UNPARSEABLE` | the launch returned, but no decision decoded from its output (fields.cause = no_json / invalid_json / empty). The wrapped error returns and the caller degrades; on Propose this adds visibility. Fields: fields.step=parse, stdout_bytes, artifact, decision, contract |

### audit

| Code | Meaning |
|---|---|
| `AUDIT_CASCADE_SHADOW` | at audit_cascade.stage shadow, the audit records if enforce escalates this cycle's audit. fields.would_escalate (true or false) and fields.changed_files (changed file count) |
| `AUDIT_CIPARITY_CHANGESET_UNDERIVABLE` | the cycle's changed-package set was underivable (git failed — e.g. a concurrent-fleet .git/index.lock race) and a whole-repo CI-parity gate (go vet, acs-durable, integration tier) SKIPPED fail-open with a WARN diagnostic; one event per gate, CI backstops the check; fields.gate, root |
| `AUDIT_CIPARITY_GATE_FAILED` | a CI-parity gate returned offenders — the audit FAILs and the cycle routes to retro; fields.cause ∈ {exit, retake_exec_failed, deadline_with_markers, retake_red, underivable, apicover, ungraduated}, fields.gate, offenders (count), first (the first offender), log (the integration-tier.log when written), exit |
| `AUDIT_CIPARITY_GATE_STEP_FAILED` | a CI-parity gate's pre-verdict step could not run and the gate failed OPEN (the diagnostic is the WARN the audit renders): fields.step ∈ {exec, tier_attempt, tier_list, bin_dir, cover_run, cover_func, write_func_cover, pkg_dirs, measure}; fields.gate, err, cmd (the exec/list/fork steps) |
| `AUDIT_CIPARITY_GRADUATION_DEFERRED` | an ungraduated new package has no production .go surface (test-only or absent) so the graduation gate did not flag it; the enrollment obligation re-raises when production code lands; fields.gate, pkg, dir |
| `AUDIT_CIPARITY_TIER_BASE_RED_ATTRIBUTED` | an integration tier red was rerun on the lane base and every red test is red there too, so the audit warns and does not blame the lane; fields.gate, keys (package.Test of each base-red test), base, main_red (the keys that are red on main too), took, log |
| `AUDIT_CIPARITY_TIER_BASE_RED_UNJUDGED` | an integration tier red could not be rerun on the lane base, so the audit blames the lane as before and a GATE_FAILED follows; fields.gate, keys (package.Test of each red test), err, log |
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
| `BRIDGE_AUTH_BENCH_CLEARED` | a launch, live smoke or usage read of a CLI family succeeded. The cli-health store removed the auth bench (auth_recheck) of that family, while quota benches stay. Fields: family, reason, source (clihealth.ClearSource: launch, smoke or usage_probe), key |
| `BRIDGE_BOOT_STRIKE_CLEAR_FAILED` | the boot-strike store failed to clear the driver's consecutive boot-timeout strike after a non-80 exit (REPL booted). The strike count can stay stale and bench early. fields step=clear_boot_strike, call_id, cli, agent |
| `BRIDGE_BOOT_STRIKE_RECORD_FAILED` | the boot-strike store failed to record the driver's boot-timeout strike after exit 80. The bench never escalates for this driver, and the error keeps the transient sentinel. fields step=record_boot_strike, call_id, cli, agent |
| `BRIDGE_CLOUD_LAUNCHED` | a cloud seat started a cloud session. The CLI printed the session line and view line within its manifest deadline. Outcome is launched, and the deliverable is a later pull request. Fields: cli, session_id, url, title, repo, branch, profile |
| `BRIDGE_CLOUD_LAUNCH_REFUSED` | a cloud seat refused a launch before starting a session. Causes: detached HEAD, no upstream, unpushed commits, a remote outside the manifest host list, or a remote branch head that differs from the local head or is absent. The reason names the cause. Fields: cli, profile, dir |
| `BRIDGE_COMPLETED_ON_WORKTREE_EVIDENCE` | a correction re-dispatch of a source-writing phase went idle without rewriting its deliverable after changing the worktree. Host state under .evolve/, the workspace, and deliverable paths never count. The bridge completed on the carried deliverable for verify and review to judge. Fields: deliverable, changed_paths, paths (at most eight, then +N more), cli |
| `BRIDGE_CONTEXT_FILL_HIGH` | a try's context fill crossed the configured warn threshold; the reason names the fill and the threshold |
| `BRIDGE_DISPATCH_MODEL_MISMATCH` | a target with launch_model_verification showed no footer label of its model_family within label_wait_s after REPL boot. Only the footer-prefix line counts; missing means unreadable (87), and two families give no evidence. The session stops and exits 87 to walk to the next CLI. Usage evidence queries the family; an exhausted verdict benches it. Fields: fields.expected, observed, label, cli, phase |
| `BRIDGE_DISPATCH_PROCESS_SURVIVED` | a process of a finished dispatch was still alive after SIGTERM, the grace time and SIGKILL; it carried the dispatch tag or was in the pane tree recorded before kill-session; fields dispatch, pids |
| `BRIDGE_DISPATCH_REAP_FAILED` | the sweep at dispatch end could not list the processes or could not send a signal; the processes of the dispatch can still run, and gc is the backstop; fields dispatch, errors |
| `BRIDGE_DISPATCH_SWEEP_REFUSED` | the sweep at dispatch end did not look for orphans by cwd, because the worktree is not a safe base: it is relative, it cannot be resolved, it is not the top of a git checkout, or it holds the project root or the home directory; the tag and pane-tree proofs still run; fields dispatch, reason |
| `BRIDGE_EXIT_ARTIFACT_TIMEOUT` | the artifact never appeared within the wait window (exit 81); wraps core.ErrArtifactTimeout. Sub-cause rides cause_code (context_cancelled, pane_lost, completion_detector_error, submit_wedged, transient_upstream, review_stop, review_pause, incomplete). The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_BAD_FLAGS` | a launch died in the validate gauntlet: bad flags, missing profile, unreadable or empty prompt, or no driver (exit 10). Plain failure. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_COMMAND_TIMEOUT` | the driver was killed by a command-level timeout (exit 124, gnu timeout convention). Transient infra weather, sibling of 81. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_COST_LEAK` | a launch refused to leak a forbidden credential into the inner CLI environment (exit 3). Plain failure. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_DRIVER_ERROR` | a launch exited with a code the table does not name. Plain failure, ledger cause driver_error. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_MISSING_BINARY` | a required external binary is missing (exit 127). Deliberately plain; family fallback sees raw 127. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_MODEL_MISMATCH` | a tmux REPL booted a model outside model_family or showed no readable model label (exit 87). Plain failure that walks to the next CLI; BRIDGE_DISPATCH_MODEL_MISMATCH carries expected and observed. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_QUOTA_WALL` | a launch exited 85 because the pane showed a quota or usage-limit wall (cause_code rate_limit or exhausted). Transient; named for the quota so pause reads the wall. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_REPL_BOOT_TIMEOUT` | a tmux REPL never showed its prompt marker within the boot budget (exit 80). Transient; the boot strike is recorded before this event. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_REQUIRED_TIER_UNAVAILABLE` | --require-full was set and the full model tier is unavailable (exit 99). Plain failure. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_RESPOND_LOOP_GUARD` | the auto-respond loop guard tripped (exit 86). Transient. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_SAFETY_GATE` | a launch died in a safety gate (exit 2: for example, without --allow-bypass, or --human-input with no audit). Plain failure. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_SEAT_TOKEN_REFUSED` | a claude seat refused to start because the seat token is empty, unreadable, not mode 0600, invalid, or unconfined (exit 88). Plain failure; no retry or fallback fixes it. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_SIGNAL_DEATH` | the driver died of a signal or failed to start (exit -1): transient under cancelled context, plain with a live one. Ledger cause stays driver_error. The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_EXIT_UNKNOWN_PROMPT` | the auto-responder escalated on an interactive prompt or a checkpoint corroborated a quota wall (exit 85). Transient; cause_code names the pattern (rate_limit, model_unsupported, or unknown_prompt). The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify |
| `BRIDGE_FRESH_SESSION_RETRY` | a launch runs as the ONE fresh session of the same CLI started by core.LaunchRecoverable after dead_shell or cli_self_updated. The dead row is marked session_recoverable and this row fresh_session_retry. Never set for named sessions, delivered runs, cancelled contexts, or tight deadlines (F31). Fields: call_id, cli, agent |
| `BRIDGE_INBOX_DRAIN_FAILED` | the inbox of the running dispatch failed a read at a tick. Envelopes after the read offset stay unread until the next tick tries again. Fields: dispatch, agent, error |
| `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED` | the captured launch stderr failed to persist as <workspace>/<agent>-launch-error.txt after a non-zero exit. The classified error stays unchanged and BRIDGE_EXIT_* drops launch_error. fields step=persist_launch_error, path, call_id, cli, agent |
| `BRIDGE_LOGIN_REQUIRED` | a paused cycle met a wall where at least one family needs a login; WARN. Reason renders the wall fact table, evidence, claim impact, and next step. fields.stop_reason=login-required, phase |
| `BRIDGE_RESULT_READ_FAILED` | the launch exited 0, but its result (artifact or stdout scrollback) failed to read into the response. Launch returns nil with empty Stdout, and the on-disk report holds the verdict. fields step=read_result, path, completion, call_id, cli, agent |
| `BRIDGE_SUBAGENT_ADAPTER_EXEC_FAILED` | the bridge adapter returned an infrastructure error (exit -1 on a launch fault); the ledger line is still written and the verdict fields carry what the artifact ladder found; the ONE outcome signal of such a run; fields.step=exec, exit_code, verdict, integrity |
| `BRIDGE_SUBAGENT_ARTIFACT_HASH_FAILED` | the artifact stood the ladder (PASS or FAIL) but could not be hashed, so the ledger line stamps artifact_sha256="" — silent before unit 16; fields.step=hash, artifact |
| `BRIDGE_SUBAGENT_ARTIFACT_INTEGRITY_FAIL` | the adapter returned but the artifact failed the integrity ladder (fields.rung = missing | stale | unreadable | empty | token_missing); the reason is the ladder's diagnostic the run's result used to drop; fields.step=verify, artifact, exit_code |
| `BRIDGE_SUBAGENT_GIT_STATE_UNKNOWN` | git HEAD or the tree-diff sha could not be captured (an error or an empty value), so the ledger line stamps "unknown" — silent before unit 16; fields.step=provenance, head, tree_state, project_root |
| `BRIDGE_SUBAGENT_LEDGER_WRITE_FAILED` | the agent_subprocess ledger line or its tip could not be written (fields.op = mkdir | flock | chain_link | open | write | close | tip_tmp | tip_rename); the returned error keeps its bare text and masks any adapter error, exactly as before; fields.step=ledger, path |
| `BRIDGE_SUBAGENT_LLM_RESOLVE_FALLBACK` | the LLM router returned an error, so the cli was taken from the profile's cli field with source=profile — silent before unit 16; fields.step=cli, source, cli |
| `BRIDGE_SUBAGENT_PREPARE_FAILED` | the artifact directory, the challenge token, the prompt read or the prompt temp file failed before the adapter ran (fields.step = artifact_dir | token | prompt | stage; fields.op = create | write on stage) |
| `BRIDGE_SUBAGENT_REQUEST_REJECTED` | an `evolve subagent run` request was rejected at admission (no prompt reader, an unknown agent role, a negative cycle, a missing workspace, the retired in-process escape hatch, or the recursion depth cap); no port was consulted; fields.step=validate, reason_class |
| `BRIDGE_SUBAGENT_RESOLUTION_FAILED` | the dispatch could not resolve its profile, cli, driver, model tier or capability manifest (fields.step names which: profile | cli | driver | tier | capability; the profile step's reason carries the read error the returned text drops) |
| `BRIDGE_SUBAGENT_VERDICT_FAIL` | the artifact is sound but the adapter exited non-zero (the bridge's exit codes reach here as fields.exit_code); fields.step=verify, cli, artifact |
| `BRIDGE_SUBAGENT_WORKTREE_FALLBACK` | WorktreePath was not propagated, so WORKTREE_PATH falls back to the project root and the agent runs against the main tree (the Warns entry callers print is kept); fields.step=env, worktree, project_root |
| `BRIDGE_TELEMETRY_APPEND_FAILED` | the per-try telemetry record failed to append to the workspace ledger; fields name the path |
| `BRIDGE_TELEMETRY_TRIPWIRE` | a successful try ran past the tripwire threshold with no measurable token usage — telemetry blind spot, not a phase failure |
| `BRIDGE_TOKEN_RESOLVER_FAILED` | the token resolver returned an error for a completed try; usage recorded as resolver-error |
| `BRIDGE_TOKEN_RESOLVER_MISSING` | the engine was built without a token resolver; lifecycle and outcome records continue without token counts (fail-open) |
| `BRIDGE_TOKEN_USAGE_WARNING` | the token resolver measured the try with a caveat (invalid counters, partial measurement); the caveat is the reason |
| `BRIDGE_TURN_APPEND_FAILED` | the per-turn lines of a call failed to append to llm-turns.ndjson. The launch and llm-calls row stay unchanged (telemetry is fail-open). Fields: call_id, cli, agent |
| `BRIDGE_TURN_SUM_MISMATCH` | the per-turn token counts of a call do not sum to the llm-calls row. The row is unchanged and the turn sidecar stays for diagnosis. Fields: call_id, cli, agent |
| `BRIDGE_USAGE_EVIDENCE` | a CLI failed with a quota-like symptom, so the failure path queried usage in a fresh session and recorded a family verdict. Verdicts: exhausted (family window used up, benched until reset), healthy (quota ruled out), unavailable (query failed), or unknown. The reason is the verdict summary. Fields: driver, cli, family, verdict, trigger (exit N or stall), exit_code, cached, and record_error on write failure |

### build

| Code | Meaning |
|---|---|
| `BUILD_BASE_RED_ATTRIBUTED` | a build floor red was rerun on the lane base and every red test is red there too, so the floor does not refuse the lane; fields.keys (package.Test of each base-red test), main_red (the keys that are red on main too), took |
| `BUILD_BASE_RED_UNJUDGED` | a build floor red could not be rerun on the lane base, so the lane is blamed as before; fields.keys (package.Test of each red test), cause |
| `BUILD_FLOOR_CONVERGING` | a build floor rejection after a correction held fewer findings, or the same count of different findings. Build got one more correction inside retry.floor_convergence_rounds. Fields: fields.before, after, round, bound |
| `BUILD_FLOOR_FLAKY_NEW_TEST` | a build floor test run was RED on a package with a new test file, and its serialized retake was GREEN. The floor fails the run like the ship added-test backstop. Fields: fields.pkg, tests, retake=green, class=flaky_new_test |
| `BUILD_FLOOR_RETAKE_GREEN` | a build floor test run was RED once and GREEN on its serialized retake under the integration-tier lock, so the build goes on. fields.pkg names the package or run; tests lists failed first-run names (used by evolve ci classify); retake=green |
| `BUILD_FLOOR_RETAKE_RED` | a build floor test run was RED and its serialized retake under the integration-tier lock was RED again, so the floor fails. fields.pkg names the package or run; tests lists failed retake names (used by evolve ci classify); retake=red |
| `BUILD_LINT_FLOOR_SKIPPED` | the build floor skipped golangci-lint on changed packages because the tool is missing or failed. The floor passes this check and CI lint stays the backstop. fields.cause |
| `BUILD_SELFCHECK_FINDINGS` | a build closed without a GREEN self-check receipt, and the handoff floor found findings (ADR-0158 decision 18). fields.stage is shadow or enforce; fields.source is gate (shadow) or bridge (enforce). fields.decision is would_return_in_session (shadow), delivered, or delivery_failed (fields.error names why). fields.receipt is absent, red, or stale; fields.findings gives the count |

### carryover

| Code | Meaning |
|---|---|
| `CARRYOVER_PERSIST_FAILED` | state.json failed to save this run's failure-learning arrays (fields.step: write = legacy write, update = serialized read-change-write). In-memory state stands and caller proceeds, so FailedRecord and P0 todo can be lost |
| `CARRYOVER_WORKSPACE_MALFORMED` | a cycle-workspace carryover document is not its documented JSON shape. The file is skipped whole without aborting the cycle (memo or audit persona defect). The reason holds the decode error |
| `CARRYOVER_WORKSPACE_READ_FAILED` | a cycle-workspace carryover document (carryover-todos.json or defect-ledger.json) exists, but the reader failed to read it (absence is silent). Nothing merges and closeout proceeds. Fields name the document and path |

### ci

| Code | Meaning |
|---|---|
| `CI_RUN_RED` | a watched CI run completed with a conclusion other than success; fields.sha, workflow, conclusion, run_id and run_url name the run |

### cli

| Code | Meaning |
|---|---|
| `CLI_QUEUE_FAILED` | the boundary failed to apply a queued CLI request (cli-contract.md §4 rule 3). The request moved to .evolve/queue/failed/. Fields: verb, request, rc, shelf |
| `CLI_QUEUE_UNLANDED` | the applied queue left tracked changes beyond the loop stamps; the next sync-main refuses them until an operator lands them (cli-contract.md §4.6); fields paths |
| `CLI_SEAT_TOKEN_MOVED` | the operator ran evolve auth seat-token set or clear; fields.verb names the verb and fields.at the time; the token is never in the signal |

### config

| Code | Meaning |
|---|---|
| `CONFIG_CATALOG_REFRESH_DEGRADED` | a catalog refresh failed to list the live models of one CLI family. The prior live pick of that family stays in the committed catalog. The fields name the family and fallback reason, and the origin names the composition step |
| `CONFIG_INERT_PHASE_ENABLE` | a phase is force-enabled (enabled: on) while dynamic_routing is below advisory and it is not mandatory or in the static state machine. The enable never runs it (cycle-120 confusion). Set dynamic_routing>=advisory or remove the enable. fields.step=inert, phase, stage |
| `CONFIG_PHASE_ROUTING_DEFECT` | loading or routing the phase catalog found a defect that the cycle survives: unloadable builtin registry, dropped/clamped/demoted user spec, or unrouted invalid spec. Also minted phases rejected or clashing, or unresolved anchors placed before audit (pre-build after build stays unrouted). Reason is warning verbatim; origin names step |
| `CONFIG_REGISTRY_MALFORMED` | docs/architecture/phase-registry.json was read, but it is not valid JSON (for example, a trailing comma). It triggers the same compiled-baseline degrade as CONFIG_REGISTRY_UNREADABLE (no triage, no registry order). The reason holds the decoder error. fields.step=registry, path, err |
| `CONFIG_REGISTRY_UNREADABLE` | docs/architecture/phase-registry.json exists, but the reader failed to read it (permissions or directory; absence is silent). Every cycle runs on the compiled baseline. The baseline omits triage, registry order, enabled/routing blocks, goal recipes, and deliverable kinds. fields.step=registry, path, err |
| `CONFIG_SPINE_ORDER` | the registry's phases[] order places ship before audit. The artifact-backed floor cannot gate ship on a shippable audit by position. The legality graph and audit verdict branch still block it. fields.step=spine, audit_pos, ship_pos |
| `CONFIG_UNKNOWN_VALUE` | a routing dial value is outside its closed vocabulary, or a registry contract is incomplete. The documented fail-safe ran (never a kill-path enable: stage → off, routing_mode → llm, model_routing → static, enabled → content). Bad conditional rules or max_optional_insertions are ignored. fields.step names the step (registry or policy). Other fields: fields.key, fields.value, fields.default |
| `CONFIG_WEAK_SPINE` | the mandatory_phases of the phase registry omit audit or ship. The audit-before-ship guarantee rests on the legality graph and audit verdict branch alone. The cycle proceeds. fields.step=spine, fields.missing = audit, ship or audit+ship |

### control

| Code | Meaning |
|---|---|
| `CONTROL_BYPASS_USED` | an audited bypass is in use (ADR-0156 I-16, X18d). It comes from an operator parameter after a control_bypass ledger write, or a bridge-start policy opt-in. Fields: name, actor, reason, at |

### convergence

| Code | Meaning |
|---|---|
| `CONVERGENCE_ACCEPTED_LIMITS` | a component landed with its limits accepted; fields.component is the component and fields.certificate its fail-safe certificate |
| `CONVERGENCE_CONCENTRATION` | open findings gathered in one component; fields.component, share, prev_share and window give the concentration over the window of rounds |
| `CONVERGENCE_DEFERRED` | findings were deferred to follow-ups; fields.count is the number deferred and fields.followups the follow-ups to open |
| `CONVERGENCE_DISPUTED` | findings are disputed; fields.count and critical count them and fields.adjudicator names who decides |
| `CONVERGENCE_FILED` | findings were filed instead of fixed; fields.late, capability and info count the late, capability and info findings |
| `CONVERGENCE_NO_HEADROOM` | the model tier of a role has no higher tier to raise to. fields.role, family, from (current tier), and to (next tier, or none) |
| `CONVERGENCE_NO_PROGRESS` | a fix round did not lower the open-finding mass at the blocking bar; fields.round, mass_prev, mass and bar give the comparison |
| `CONVERGENCE_REPAIR_DAMAGE` | a fix round broke more than it repaired; fields.round, damage (findings the round introduced) and repairs (findings it closed) |
| `CONVERGENCE_RUNG` | the convergence policy chose the next rung of the audit-repair loop after an audit FAIL. fields.stage is shadow (repair flow unchanged) or enforce (Stop ends repair). fields.loop, fields.round, fields.action, fields.rung, and fields.blocking_bar give the decision; fields.fresh_context skips failed context |
| `CONVERGENCE_SPLIT` | the decision split a component out of the loop; fields.component is the component and fields.followup the follow-up that carries it |
| `CONVERGENCE_STOP` | the decision stopped the loop; fields.round, open (findings still open) and land_round (the round the policy planned to land) |

### cycle

| Code | Meaning |
|---|---|
| `CYCLE_SPEND_LIMIT` | cycle spend passed a workflow.cycle_spend limit at a phase boundary (ADR-0158 decisions 14-16). fields.decision is would_stop (shadow), over (enforce, stop illegal), or stop (enforce at failed audit; passing audit still ships). fields.stage is shadow or enforce; fields.partial is true if ledger load was incomplete. Fields: limit, value, max, phase |

### failurediag

| Code | Meaning |
|---|---|
| `FAILUREDIAG_SIDECAR_WRITE_FAILED` | the phase's <phase>-failure-diag.json failed to write (temp write or rename). The reason names the step and error; fields name the path. The phase abort proceeds unchanged and the diagnosis is lost |

### failurelearning

| Code | Meaning |
|---|---|
| `FAILURELEARNING_FLOOR_WRITE_FAILED` | the deterministic floor failed to write retrospective-report.md, lesson YAML, or inbox remediation (including id collisions). FailedRecord and P0 todo stand in memory, recurrence closure runs, and phase failure stays intact. fields.step=floor, lessons_dir, workspace |
| `FAILURELEARNING_POLICY_LOAD_FAILED` | .evolve/policy.json exists, but failed to read or parse while writing the deterministic floor (absence is silent). Novelty threshold and remediation weight fall back to compiled defaults, and the floor proceeds. fields.step=floor, path |
| `FAILURELEARNING_RECURRENCE_LEDGER_FAILED` | the recurrence ledger keyed by the failing pattern failed to load, update or save (fields.op = load | record | save). Count() stays stale for this pattern, and the phase failure is never masked. fields.step=recurrence, path |
| `FAILURELEARNING_REMEDIATION_TRUNCATED` | a failed phase self-reported more defects than the remediation cap. First items file as inbox items, and the rest drop; fix the emitter or raise the cap. fields.step=floor, reported, filed |

### gate.contract

| Code | Meaning |
|---|---|
| `GATE_CONTRACT_DEMOTED` | the breaker opened after N consecutive blocks and demoted enforce→advisory; the phase advanced UNVERIFIED — inspect the failing phase and policy.gates.contract_gate |
| `GATE_CONTRACT_FAIL_OPEN` | the gate failed to decide (unknown phase, read fault) and failed open; re-dispatching an agent cannot fix this — the reason is the error |
| `GATE_CONTRACT_REJECTED` | the gate refused the deliverable at enforce. The reason is the correction directive (one [code] per violation). Fields hold the codes and breaker count (0 for breaker-exempt optional evaluate phases). The orchestrator ladder re-dispatches |
| `GATE_CONTRACT_SALVAGED` | a sole recoverable bad_verdict was repaired on disk and re-verified clean; the phase advanced on the repaired artifact |
| `GATE_CONTRACT_VERIFIED` | the phase's declared deliverables were found in place (fields name the artifact, its size, the agent-owed files and the effects verified) and the phase advanced |
| `GATE_CONTRACT_WOULD_BLOCK` | the deliverable violated its contract, but the stage (shadow, advisory, or report-size) lets the phase advance. The reason names what enforce refuses |

### gate.repo

| Code | Meaning |
|---|---|
| `GATE_REPO_CAPACITY_PARKED` | a repo-contract gate hit its deadline twice in a row on the same lane tree. The orchestrator parked the lane without an LLM re-audit; fields.code is the ship error code. The lane needs more CPU time or a smaller closure, not a code change |
| `GATE_REPO_MAIN_CI_RED` | the loop tick polled the CI of origin/main, found a completed failed check-run, and no stop held, so the main-red stop opened with one CI key for each failed job; ship refuses each landing until the poll sees a green main; fields.failed (each failed job as name=conclusion), main (the origin/main commit), fix_item, fix_item_error (empty when the item is in the inbox) |
| `GATE_REPO_MAIN_RED_PARKED` | ship found the red tests of the lane red on its base and on main, so the orchestrator parked the cycle and did not blame the lane (ADR-0165 G4); the cycle has a main-red checkpoint and a row in main-red-stop.json, and it resumes when main is green; fields.keys (package.Test of each key of the stop), main, fix_item, parked (the parked cycles), checkpoint_error and stop_error (each empty when its write passed; a write that failed makes the signal an incident, because the cycle cannot resume from the park) |
| `GATE_REPO_MAIN_RED_STOP_ESCALATED` | a stop held longer than gates.main_red_stop_bound_minutes, so the bound check escalated it one time (ADR-0165 G4); main-red-stop.json has escalated_at, and an open main-red incident under .evolve/incidents has the cause and the resume condition, so the healing catalog can propose a revert of the last landing; fields.keys (package.Test of each key of the stop), main, fix_item, parked (the parked cycles), opened_at, bound, incident (the id of the incident record) |
| `GATE_REPO_MAIN_RED_STOP_OPENED` | a build floor or ship found tests that are red on main, and no stop held, so the stop opened (ADR-0165 G4); main-red-stop.json has the keys and the fix item is in the inbox, where it leads the rank while the stop holds; fields.keys (package.Test of each key that is red on main), main, fix_item, fix_item_error (empty when the item is in the inbox; an item that was not filed makes the signal an incident) |
| `GATE_REPO_MAIN_RED_STOP_RELEASED` | each key of the stop is green, through a fix that landed or through a rerun on the main tip, so ship or the train released the stop (ADR-0165 G4); the parked lanes resume in the next pool fill, and a fix item that is not consumed moves to processed; fields.main, fix_item, released_by (the cycle and commit of the fix, the main commit of the rerun, or the batch tip that the train moved main to), parked (the parked cycles), fix_item_error (empty when the inbox is consistent; an item that did not move makes the signal a warning) |
| `GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE` | a stop holds, the landing is not a proven fix, and gates.main_red_stop is at shadow, so ship or the train lands it and records that enforce refuses it (ADR-0165 G4); fields.keys (package.Test of each key of the stop), landing (the lane, or the batch of the train), base (the commit that the landing is on), admission (rebase when the landing is not on the main tip, not-a-fix when it turns no key green), main (the main tip for rebase, the commit of the stop for not-a-fix), fix_item |

### healing

| Code | Meaning |
|---|---|
| `HEALING_ESCALATED` | the catalog validator sent an incident to the operator: a second refusal, two failed repairs, an open breaker or a halt-and-escalate action; the same fields as HEALING_PROPOSAL_REFUSED (llm-isolation.md §19.1, §19.2, §24) |
| `HEALING_I10_VIOLATION` | the gate classifier flagged a loop cycle diff that changes a kernel surface or weakens a gate (invariant I-10, llm-isolation.md §20.3); the build floor fails and ship refuses the cycle; fields.patterns names each pattern, fields.paths each flagged path |
| `HEALING_INCIDENT_OPENED` | the incident detector opened an incident record under .evolve/incidents; fields incident, class, fingerprint, tier, state, transition, recurrence, stage, and from the live reaction on a sealed FAIL also reaction: would_brake, braked, brake_already_engaged or brake_failed (llm-isolation.md §18, §24) |
| `HEALING_INCIDENT_REOPENED` | the incident detector reopened a closed incident, because its signal fired again after the close; the same fields as HEALING_INCIDENT_OPENED (llm-isolation.md §18, §22) |
| `HEALING_PROPOSAL_REFUSED` | the catalog validator refused a proposal of the orchestrator agent; fields incident, class, tier, action, verdict, precondition, reason, stage (llm-isolation.md §19, §19.2, §24) |

### host

| Code | Meaning |
|---|---|
| `HOST_DISK_LOW` | free disk space on the host volume is below the pause line, so the cycle start waits before it dispatches. fields.free_pct, pause_pct and path give the reading, and fields.outcome is paused (the first wait) or gave_up (the maximum pause from preflight.disk_guard.max_pause_s passed and the cycle start stopped) |
| `HOST_VERIFY_LOCK_WAIT` | a verification run asked for the host verification lock. fields.source names the caller (build-floor or acs-suite), fields.wait_ms the time that it waited, and fields.outcome is acquired or unserialized (the wait bound passed or the lock failed, so the run went on without the lock) |

### inbox

| Code | Meaning |
|---|---|
| `INBOX_CLAIM_MOVE_FAILED` | a claim could not create processing/cycle-N/ (fields.step=mkdir, dest_dir) or could not rename the item into it (fields.step=rename — the item may already be claimed); ErrMvFailed (exit 2), the item stays where it was; fields.task_id, err |
| `INBOX_CLAIM_NOT_FOUND` | a lane claim named an id the inbox root does not hold (absent, already claimed into processing/, or parked); the claim returns ErrNotFound (exit 1) and moves nothing — the FAIL closeout's lane-scope claim of an id the triage persona already claimed reports this for an expected state; fields.step=locate, task_id, inbox_dir |
| `INBOX_CLAIM_REFUSED` | a lane claim named an operator-owned item (route:console-* or a protected fix surface, ADR-0074 I1); the claim returns ErrConsoleRouted (exit 3) and the item stays at the root; fields.step=route, task_id, reason |
| `INBOX_CLAIM_STALE` | the stale-claim sweep found a claim in processing/cycle-N/ whose cycle already sealed (a closeout exists) — the cycle exited without draining its claim, so the sweep releases it; fields.id, closeout, step=stale_sweep |
| `INBOX_CONTINUATION_MANIFEST_UNREADABLE` | the FAILed cycle's continuation manifest exists but could not be read or parsed; every item of the drain releases unstamped (a later claim starts fresh instead of resuming the salvage); fields.step=manifest, workspace, err |
| `INBOX_CURATOR_PROPOSAL` | the inbox curator proposes consuming, rerouting, merging, or quarantining one open item. Deterministic proposals apply only at enforce on wave boundaries; judgment proposals wait for operator or orchestrator. Fields: kind, id, target, trigger, deterministic, stage, applied, evidence |
| `INBOX_ITEM_FILED_CONSOLE` | an item was filed as console-owned (protected fix surface, console kind, or authored console route) and no lane claims it; fields.class = protected | kind | route, task_id, kind, path, reason, step=intake |
| `INBOX_ITEM_REWRITE_FAILED` | an item record could not be rewritten atomically: the closeout's route-console rewrite (step=route), the quarantine release's counter reset (fields.step=counter_reset — the item releases with its stale count), the drain's failure_count bump (fields.step=failure_bump — quarantine is skipped, the item releases to the root) or the drain's continuation stamp (fields.step=continuation_stamp — the item releases unstamped); fields.task_id, path, err |
| `INBOX_ITEM_ROUTED_CONSOLE` | a closeout routed an item to console-manual: the FAIL closeout when a phase gate refused it deterministically (a top_n card naming a protected surface — the per-item breaker of the 2026-09-14 poison-loop incident), or the planned-no-work closeout when a fleet lane's triage answered for its scoped item without committing it (F30 — the reason names the bucket and the lane's evidence): fields.task_id, path, reason, step=route — the item is operator-owned from here on and no lane claims it again |
| `INBOX_LANDED_CHECK_FAILED` | the landing probe could not answer for a processed-promotion's sha — in production the host's git probe (shaLandedOnMain) on an exec fault or a git exit outside {0,1}: an unknown sha, no local main, a non-git ProjectRoot; the promote fails OPEN (the sha is treated as landed) so a gate fault never blocks a promotion, and the item lands in processed/ with this line as the only trace; fields.step=landing, task_id, sha, err |
| `INBOX_PROMOTE_MOVE_FAILED` | a promote could not create its destination dir (fields.step=mkdir — ErrMvFailed, NoOp false, ledger promote-warn/mkdir-failed) or could not rename the item into it (fields.step=rename — the compat NoOp success, ledger promote-warn/mv-failed); the item stays where it was; fields.task_id, state, src_rel, dest, err |
| `INBOX_PROMOTE_NOT_FOUND` | a promote named an id neither processing/cycle-*/ nor the inbox root holds — already moved; the ship.sh-compat NoOp success is returned; fields.step=locate, task_id, state |
| `INBOX_PROMOTE_UNLANDED_SHA` | a processed-promotion carried a ship sha the landing probe says is not on main; the item is rerouted to retry/ under reason ship-promote-retry-unlanded-sha instead of buried in processed/; fields.step=landing, task_id, sha, state=retry |
| `INBOX_QUARANTINE_FAILED` | an item at the ADR-0072 S5 retry ceiling could not be parked in quarantine/ (fields.outcome=error: the park's promote errored; outcome=noop: the park's rename failed) and falls open to a root release — the poison item WILL be re-picked; the preceding INBOX_PROMOTE_MOVE_FAILED from Mover.Promote names the cause; fields.step=quarantine, task_id, failure_count, ceiling |
| `INBOX_RELEASE_DOUBLE_MOVE` | the cycle drain found the item's basename already at the inbox root (a concurrent release landed it first); the root copy is never clobbered, the processing copy stays and is not counted; fields.step=release_cycle, task_id, base |
| `INBOX_RELEASE_MOVE_FAILED` | an item could not be renamed back to the inbox root — by the cycle drain (fields.step=release_cycle, origin Mover.Release) or by orphan recovery (fields.step=recover_orphans, origin Mover.RecoverOrphans); the item stays in processing/cycle-N/ and the walk continues; fields.task_id, base, err |
| `INBOX_ROUTE_NOT_FOUND` | the FAIL closeout could not find the item it was told to route (fields.task_id, inbox_dir, step=locate) — or found it held by another cycle's claim (fields.held_by_cycle) — the refusal is NOT recorded on it and it WILL be re-picked; neither processing/cycle-*/ nor the inbox root holds the id |
| `INBOX_ROUTING_FALLBACK` | the lane-routing predicate failed to load the build profile (fields.profile, fields.project_root; reason holds the error). Routing judges protected surface and frozen checks only, so a sandbox-denied item can reach a lane. Emitted once per predicate |
| `INBOX_STALE_DROP_REPEATED` | the same inbox item was stale-dropped a second time after a console re-queue; the item stays console-manual and no third lane is dispatched; fields.task_id, stale_drop_count |

### isolation

| Code | Meaning |
|---|---|
| `ISOLATION_CONTEXT_BUDGET_OVER` | the cell context index: required entries of a layer passed its token budget and lost the flag; fields layer, limit_tokens, over (cell-context-index.md §6) |
| `ISOLATION_CONTEXT_REQUIRED_UNOPENED` | the cell context index: a cell closed without a read of a required entry; fields.unopened lists the ids, and fields.source is sandbox (cell-context-index.md §8) |
| `ISOLATION_ENV_REFUSED` | I-2 hermetic environment at enforce: the allowlist refused launcher variables that no earlier dispatch of the process reported; one INFO event for each dispatch. Fields: invariant, action, stage, cell, family, rule, source, count, names (names holds no value and is capped below the log field limit, with a +N more tail) (llm-isolation.md I-2) |
| `ISOLATION_I1_VIOLATION` | I-1 single launcher: a code path started an LLM binary outside the bridge launch API; fields.target names the binary (ADR-0154) |
| `ISOLATION_I2_VIOLATION` | I-2 hermetic environment: the allowlist would drop a launcher variable, and shadow keeps it (action=logged); sent at shadow only, while enforce sends ISOLATION_ENV_REFUSED. Fields: invariant, stage, action, cell, family, target, rule, source (ADR-0154) |
| `ISOLATION_I3_VIOLATION` | I-3 no nested agency: a cell tried to start an LLM binary or an evolve dispatch; fields.target names it (ADR-0154) |
| `ISOLATION_I4_VIOLATION` | I-4 pipeline-mediated communication: a cell tried to read raw output or to write its inputs; fields.target names the path (ADR-0154) |
| `ISOLATION_I5_VIOLATION` | I-5 read isolation: a cell tried to read outside its index, its worktree and its CLI files; fields.target names the path (ADR-0154) |
| `ISOLATION_I6_VIOLATION` | I-6 family-agnostic flow: a launch resolved through a compiled family default, not through routing data; fields.cell names the seat (ADR-0154) |
| `ISOLATION_I7_VIOLATION` | I-7 CLI-neutral guarantees: a gate or the fence stopped a write that no hook stopped; fields.target names the path (ADR-0154) |
| `ISOLATION_I8_VIOLATION` | I-8 typed flow decisions: a flow parser fell back to stdout or prose; fields.cell names the seat (ADR-0154) |
| `ISOLATION_I9_VIOLATION` | I-9 proven substitution: no run-time emitter; the conformance matrix TestE2ECycleCLIMatrix is its guard (ADR-0154) |
| `ISOLATION_LANE_PIN_FORGED` | the workspace lane-scope.json lists other items than host record .evolve/lineage/cycle-N.json. The host kept its record and no reader used the file; emitted once per cycle after a cell. fields.record, fields.file, fields.path |
| `ISOLATION_READ_UNMEASURED` | the cell read report is incomplete, so its reads count as unmeasured and no open rate counts the cell; fields cell, family, stage, cause (fence-deadline, cancelled, fence-write, report-read), elapsed (only for fence-deadline and cancelled). A cancelled launch is INFO because an operator stop caused it; the other causes are WARN (cell-context-index.md §8) |
| `ISOLATION_REQUEST_FAILED` | the request channel approved a request of a cell, and the dispatch of the agent failed; the same fields as ISOLATION_REQUEST_REFUSED (llm-isolation.md §7.2.1) |
| `ISOLATION_REQUEST_REFUSED` | the request channel refused a cell request through validator (schema, kind, role, input, cap, or depth) or orchestrator agent. Fields: request, kind, role, decision, precondition, reason, stage (cli-contract.md §4.5, llm-isolation.md §7.2.1) |

### ladder

| Code | Meaning |
|---|---|
| `LADDER_AUDIT_FIX` | the audit_fix rule ran after an audit FAIL or on residual build-floor findings. fields.source is audit or build_floor; fields.audit_fix is `would` (shadow), chosen (enforce), or decline; fields.audit_fix_rule names the rule. Other fields hold findings, prescriptions, files, lines, code lines, tokens, and handler registration |
| `LADDER_DECISION` | the retry ladder chose the rung of the next phase try (ADR-0142). fields.stage is shadow (dispatch does not change) or enforce. fields.origin is the retry path, fields.rung is the rung before, and fields.next is the chosen rung. fields.action is raise, retry, or stop. fields.class and fields.fingerprint give the failure. fields.tier and fields.effort give the target |
| `LADDER_NO_HEADROOM` | no family of the chain has headroom at the next tier, so the ladder holds at R1 or stops; fields.rung is the rung before |
| `LADDER_REDESIGN` | the ladder ended (R3 failed, or a spec stop) and routed the subject to the redesign rung of ADR-0126 once (ADR-0164 ruling 1). fields.fingerprint names the failed check; fields.origin names the retry path. A second ladder end goes to the operator with no second redesign |
| `LADDER_STOP_SPEC` | two tiers failed on the same check, so the ladder reclassified the failure as spec and stopped; fields.fingerprint names the check |

### latency

| Code | Meaning |
|---|---|
| `LATENCY_BUDGET_EXCEEDED` | two waves in a row breached a bound of the wave latency check. fields.waves lists them, fields.reasons names the breached rules, and fields.item the filed item |

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
| `LIVENESS_PHASE_STALLED` | the per-phase observer saw no progress for the stall threshold between review checkpoints. Tmux checks pane-watch hash (kind pane.liveness); headless checks stdout and workspace growth (kind observer.warning). Log and signal only; nothing is killed. Fields: source (pane | stdout), session, busy, stall_s |

### log

| Code | Meaning |
|---|---|
| `LOG_ACCESS_SHADOW` | at log_access shadow, a phase read a log layer that its phase.json log_access does not declare, and the read went on. fields.layer (D0 to D3), declared (the layers of log_access) and verb (the evolve logs verb) give the read |

### loop

| Code | Meaning |
|---|---|
| `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED` | the boundary-refresh-log.jsonl audit entry failed to append AFTER the state.json pin moved. Refresh continues to re-exec; pin and audit trail now disagree. fields.batch, path, error |
| `LOOP_BOUNDARY_REFRESH_SKIPPED` | the boundary binary refresh degraded to continue on the current binary at fields.step (ahead_check, lane_check, lane_active, breaker, interrupted, rebuild, target, repin, argv, arm, or reexec). Causes include check errors, active sibling leases, breaker hits, signals, build failure, missing binary, or exec failure. Fields: fields.batch, commit, error |
| `LOOP_CHAIN_BATCH_ERROR` | a chained batch exited with a non-continuable rc (not 0/3/5) and the chain stopped, propagating rc. The batch's own signals name the failure. fields.batch, rc, stop_reason=chain_batch_error |
| `LOOP_CHAIN_INBOX_ITEM_INVALID` | a root-level .evolve/inbox/*.json failed to parse as an inbox item (no object with a non-empty id). It is NOT counted as pending work. fields.batch, name, path |
| `LOOP_CHAIN_INBOX_UNREADABLE` | the chain failed to read .evolve/inbox (a read error other than not-exist) and stopped rather than loop blind; fields.batch, path, error, stop_reason=chain_inbox_unreadable, exit=2 |
| `LOOP_CHAIN_QUOTA_DEFER` | a chained batch exited with resumable rc=5. The chain defers instead of relaunching into the wall or red main (resume with evolve loop --resume). fields.pause names the pause; fields.batch, cycle, wake_at, source come from checkpoint |
| `LOOP_ESCALATION_BOUNDARY` | the escalation boundary staged inbox items (bumped/filed/planned) or refused a staged autofile intent without priority_class after a cycle. fields carry the counts (with refused, each refused id printed as WARN) and the stage |
| `LOOP_FLEET_LANE_HALT` | a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed |
| `LOOP_FLEET_POLICY_INVALID` | the loop failed to load .evolve/policy.json at batch start (unreadable, malformed, or strict-schema error). Fleet config fell back to defaults and runs sequential (count 1) until fixed. The reason holds the load error; fields.path |
| `LOOP_HALT` | the batch halted at a boundary: plane diverged, sync refused, main CI red, disk below floor, or boundary CLI smoke test failed. fields.families and fields.versions name the failed update. The reason is the halt error |
| `LOOP_HOST_PRESSURE_BLIND` | the host-pressure reader failed smooth_samples times in a row, so the controller holds the cap and cannot see the host. Published once per failed streak; WARN. Fields: fields.error, os |
| `LOOP_LOST` | a reader saw a loop process end without its loop.exit event. A reader makes this record, and no log holds it. Fields: fields.pid, run_id, exit (crash or unknown), historical (true if the loop stopped before the reader looked) |
| `LOOP_MIN_WIDTH_REPAIR` | the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair) |
| `LOOP_ORCH_ADMISSION_OPTIMISTIC` | at fleet.orchestrator.admission enforce (ADR-0143 W1b), the pool launched a lane admitted only by the optimistic rule. held_by names the first reach holder and travels in EVOLVE_FLEET_REACH_HOLDER; INFO. Fields: fields.todo, held_by, reason |
| `LOOP_ORCH_ADMISSION_SHADOW` | at fleet.orchestrator.admission shadow (ADR-0143 W1), import-reach held a todo that the optimistic rule admits; reach still governs. Published once per todo and holder in a pool run; INFO. Fields: fields.todo, held_by, rank, cells, reason |
| `LOOP_ORCH_ADMIT_WAIT` | the admission shadow (fleet.orchestrator.stage, ADR-0139 O3) held a planned todo whose package cell or id family overlaps an earlier-ranked todo. In shadow, it records the hold and changes no lane. Fields: todo, rank, cells, blocked_by, blocked_by_lane, stage |
| `LOOP_ORCH_EJECTION_BRIEF_FAILED` | oversight (ADR-0147 I3) failed to send the ejection brief (no brief port, unreadable file, or send error). Lane cells stay held, and the lane misses why its deliverable returned; WARN. Fields: fields.lane, cycle, error |
| `LOOP_ORCH_FAMILY_PAUSED` | a bench left a CLI family with no healthy fallback in a chain, so sizing drops a slot and stops new lanes (ADR-0139 decision 13). In-flight lanes finish; shadow sets applied false; INFO. Fields: fields.family, target, stage, applied |
| `LOOP_ORCH_HALT_DRAINING` | a lane exited with the ADR-0072 system-failure halt code, so the pool stops its backfill and drains: the running lanes seal and the wave ends. INCIDENT. Fields: fields.todo, rc, cause (system_halt), live |
| `LOOP_ORCH_HOST_PRESSURE` | the host-pressure controller (fleet.orchestrator.window.adaptive) decided an admission cap from host load, memory and swap. One event per change of the cap; shadow sets applied false and leaves the window alone; INFO. Fields: fields.old, new, sized, cause (pressure_high, pressure_low or hold), load1, cpus, load_per_core, mem_available_pct, swap_used_mb, score, stage, applied |
| `LOOP_ORCH_LANDING_UNLOCATED` | oversight received a landing or seal for a cycle that no live lane holds. It runs landing-state cleanup, counts nothing, and acts on no lane; WARN. fields.cycle, event |
| `LOOP_ORCH_LANE_BACKFILLED` | the orchestrator pool gave a freed slot to the next admissible todo at once; INFO; fields.todo, live, target |
| `LOOP_ORCH_LANE_DEADLINE` | a lane passed its lease deadline (p95 of last 50 cycles without max, min deadline_floor_minutes, ADR-0139 O5). Shadow records action; enforce cancels context, reaps dispatches, releases claim, and frees cells. Fields: fields.todo, stage, action, deadline_minutes |
| `LOOP_ORCH_LANE_EXITED` | a lane of the orchestrator pool exited and freed its slot and its cells; INFO; fields.todo, live, target |
| `LOOP_ORCH_LANE_LATE_EXIT` | a lane exited after its lease deadline passed (shadow measure of ADR-0139 O5). status ok means a deadline stop stops a passing lane. Fields: fields.todo, stage, status, rc, stopped |
| `LOOP_ORCH_LANE_LAUNCHED` | the orchestrator pool (fleet.orchestrator.stage enforce, ADR-0139 O4) launched a lane in its first fill; INFO; fields.todo, live (lanes in flight), target |
| `LOOP_ORCH_LANE_RECOVERY_FAILED` | after a deadline stop, the orchestrator failed to release the inbox claim (step release) or park with route console (step park). The todo needs the console. Fields: fields.todo, step, error |
| `LOOP_ORCH_LANE_STALLED` | a lane had no live signal (cycle signals, pane-watch, events, or stdout) for fleet.orchestrator.stall_minutes (ADR-0139 O5). Published once per lane; missing cycles have unknown heartbeats and keep their slot. Fields: fields.todo, stage, last_heartbeat, stall_minutes |
| `LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED` | an optimistically admitted lane (ADR-0143 W1b) hit a composed-tree gate failure or fleet rebase conflict. Published once at first failure; D3 counts failures only when admission of that todo is in-wave; WARN. Fields: fields.todo, held_by, gate |
| `LOOP_ORCH_OVERSIGHT_ACTION` | the oversight loop (ADR-0139 O6) mapped a cycle, ship, errors, or loop event to a lane action. At shadow applied is false; at enforce applied is true; INFO. Fields: fields.actions, lane, cycle, event, stage, applied, verdict/target |
| `LOOP_ORCH_POOL_HELD` | a pool fill round stopped below its width because admission refused every pending todo while a lane was in flight. Fires once per live/pending count change at wave start and round end; INFO. Fields: fields.live, target |
| `LOOP_ORCH_POOL_IDLE` | an orchestrator pool slot stays free after a lane exit: no admissible todo, empty supply, operator stop, or run_max_lanes reached; INFO. Fields: fields.live, target, cause (family_held, run_cap, supply_empty, admission_refused, bench, operator_stop, host_pressure or system_halt) |
| `LOOP_ORCH_POOL_RESUPPLIED` | an orchestrator pool slot stayed free after a lane exit, so the pool re-ran the plan source and added a ready todo. One event per added todo; INFO. Fields: fields.todo, live, target |
| `LOOP_ORCH_POOL_UNDERFILLED` | the pool had an idle or held transition with fewer than fleet.min_lanes in flight while not stopped; WARN. family_held names family-held todos so they do not read as supply failures. Fields: fields.live, target, min_lanes, ready, cause (family_held, run_cap, supply_empty, admission_refused, bench, operator_stop, host_pressure or system_halt), family_held |
| `LOOP_ORCH_SUPPLY` | the supply census (ADR-0139 O6, spec section 4) before a plan counts inbox items in 7 classes plus window and console items. The full record is .evolve/waves/supply.json; INFO. Fields: seven class names, window, console |
| `LOOP_ORCH_SUPPLY_SHORT` | the supply census found fewer admissible items than the window, so the plan lanes cannot all be filled. Refusal causes sit in the supply record and evolve wave supply; INFO. Fields: seven class names, window |
| `LOOP_ORCH_WINDOW` | the adaptive orchestrator pool window (ADR-0139 O8) changed size: +1 per clean landing, halved on conflict (min window.min), following sized count. Publishes once per change in enforce pool only; INFO. Fields: fields.old, new, sized, cause |
| `LOOP_PIPELINE_BLOCKER_HALT` | the pipeline-blocker breaker halted the batch on repeated fingerprints, unexplained failures, or consecutive failures over the ceiling. It also trips on consecutive lane deferrals (rule lane-deferrals, category lane-provisioning) or planned no-work hand-offs in failure_policy.thresholds.no_work_handoff_window at no_work_handoff_halt_ceiling. For no-work hand-offs: rule no-work-handoffs, category no-work-handoff, fingerprint no-work-handoffs|cycle|<N>. fields.rule and fields.fingerprint name the rule. The remaining fields match LOOP_SYSTEM_FAILURE_HALT (next, escalation, inbox_item) |
| `LOOP_SYSTEM_FAILURE_HALT` | the batch halted on an ADR-0072 system failure that the cycle signalled (the pipeline, not the task, is the cause). fields.category names the floor. fields.next is the escalation dossier's next_action. fields.escalation and fields.inbox_item name the dossier and P0 item |
| `LOOP_WAVE_ALL_LANES_STALE` | every planned wave lane was stale at the freshness gate (consumed or unmet dep) and backlog refill found nothing. The wave launched nothing (never a doomed lane); fleet logs per-lane skips. fields.planned, skipped |
| `LOOP_WAVE_DISPATCH_FAILED` | a wave (fields.path=wave) or min-width repair (path=repair) failed to dispatch. Steps: preflight (uncommitted control-plane edits), plan (missing triage or fewer than 2 disjoint lanes), or adapt. The batch falls back to sequential mode. fields.error holds the step error |
| `LOOP_WAVE_EMPTY_PLAN` | the wave planned zero lanes and falls back to sequential. cause=empty_triage_plan when fleet.count <= 1 (one-lane repair guard unmet); cause=empty_backlog when the guard met no disjoint candidate. fields.desired = fleet.count, realized = wave count |
| `LOOP_ZERO_SHIP_HALT` | the zero-ship breaker halted the batch after consecutive waves (fleet) or cycles (sequential) landed no ship up to failure_policy.thresholds.zero_ship_halt. Failed and no-work lanes both count. fields.rule and fields.waves name the rule and run. The remaining fields match LOOP_SYSTEM_FAILURE_HALT (next, escalation, inbox_item) |

### observer

| Code | Meaning |
|---|---|
| `OBSERVER_EVENTS_SINK_OPEN_FAILED` | the live per-phase observer failed to open <phase>-observer-events.ndjson, so the phase runs UNOBSERVED (ADR-0030: the observer never blocks the phase); fields.step=open_sink, path |
| `OBSERVER_EVENT_APPEND_FAILED` | one line of <agent>-observer-events.ndjson was lost (fields.op = marshal | mkdir | open | write); reported once per op. An INCIDENT still stays for the report if open succeeded. fields.step=emit, op, path, event_type |
| `OBSERVER_KILL_FAILED` | the SIGTERM to the agent's process group returned an error (ESRCH: already gone; EPERM: not ours). The kill ran once. fields.step=respond, pgid, signal, kind |
| `OBSERVER_NUDGE_APPEND_FAILED` | the soft-stall nudge failed to append to the agent inbox. The soft_stall_nudge envelope still emits and is not retried. fields.step=nudge, idle_s, threshold_s, agent |
| `OBSERVER_POLICY_LOAD_FAILED` | .evolve/policy.json exists, but `evolve phase-observer` or `evolve phase-watchdog` failed to read or decode it (absence is silent). The poll, stall, nudge, watchdog thresholds, and --enforce stall artifact extends fall back to compiled defaults. fields.step=policy, path |
| `OBSERVER_REPORT_WRITE_FAILED` | <agent>-observer-report.json failed at marshal, mkdir, .tmp write, or rename. The subcommand still exits 0 (best-effort report). fields.step=report, path |
| `OBSERVER_STALL_KILL_SENT` | the observer sent SIGTERM to the agent's process group after a stall INCIDENT, after appending the envelope and before the signal. fields.step=respond, kind, pgid, action, action_reason, idle_s/threshold_s |
| `OBSERVER_STDOUT_TAIL_FAILED` | the agent's stdout log failed stat (non-ENOENT), open, seek, or scan (line over 10 MiB); reported once per op (absent logs stay silent). Prior offset stays on stat/open/seek, file size on scan error. fields.step=tail, op, path |
| `OBSERVER_WATCHER_LEAKED` | the live observer watcher goroutine did not exit within the bound after phase end. The goroutine and sink fd leak on purpose until OS exit cleanup (closing races writes). fields.step=cancel, timeout_s |

### orchestrator

| Code | Meaning |
|---|---|
| `ORCHESTRATOR_AUDIT_REPAIR_DECLINED` | an audit FAIL earned no repair round and the cycle goes to retro. fields.reason is the retry envelope's verdict (unrecognised class, budget spent, non-retry class, or none declared). fields.declared_class is the report class or the one derived from deterministic gates |
| `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` | an audit FAIL earned a repair round; fields.next is the re-entry phase (tdd | build), fields.attempt the repair try about to be spent, fields.reason the envelope's basis |
| `ORCHESTRATOR_BLOCKER_REFUSED` | a phase's verdict declared a BLOCKER defect that the host did not accept; a refused claim keeps the correction rounds with the refusal in each directive, while an unroutable proof or an unreadable predicate base ends the rounds and the cycle; the reason states which |
| `ORCHESTRATOR_BLOCKER_ROUTED` | a phase's gate failures are all explained by a blocker the host proved (a protected path the fix needs, or two of the cycle's own predicates that conflict); the correction rounds stop, the cycle seals FAIL with termination reason blocked-needs-console and the BLOCKER_ROUTE diagnostic routes the item to the console; the reason is the proof and the lane's diagnosis |
| `ORCHESTRATOR_COMPOSED_GATE_DECLINED` | a composed-tree gate did not report pass at a MissingComposedGates site (RUNG 2 scopedMergeCarryForward or ADR-0105 B3 identityCarryForward). fields.tail_<gate> holds each failing gate tail. The cycle falls back to full re-audit |
| `ORCHESTRATOR_CYCLE_FAILED` | the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision |
| `ORCHESTRATOR_EXPLANATION_CORRECTION_EXHAUSTED` | a passing audit still reviews the explanation document as NEEDS_CORRECTION after the budget is spent. The verified code ships with the review on record |
| `ORCHESTRATOR_EXPLANATION_CORRECTION_RUNG` | a passing audit reviewed the explanation document as NEEDS_CORRECTION. Build corrects only that document and audit re-reviews it before ship; fields.round and fields.budget count the rung |
| `ORCHESTRATOR_EXPLANATION_REAUTHOR_ROUTED` | a post-Build source writer made the Build explanation stale (cited path missing or Changed Areas drift), so the cycle routed to Build to re-author it. fields.next is the re-entry phase. fields.charged is false if every legal successor leads to Build (tdd), else true (spends one code-audit-fail try). Fields: fields.attempt, fields.envelope; reason is the validator verdict |
| `ORCHESTRATOR_FLEET_REBASE_CONFLICT` | the ship-recovery fleet rebase hit a merge conflict, and the cycle routes to debugger (ADR-0143 W1b reads it for optimistic admission). Not a landing-conflict error, so oversight does not double-count it; WARN. fields.paths (conflicted paths) |
| `ORCHESTRATOR_FLEET_REBASE_INBOX_RESOLVED` | ship-recovery fleet rebase hit a conflict on the cycle's inbox item (also stamped on main) and resolved it by rule. Main stamps stay, cycle field changes and consumption land, and code stays untouched; WARN. fields.paths (resolved inbox paths) |
| `ORCHESTRATOR_GATE_CORRECTION` | the correction ladder ran a rung after a gate rejection. Fields name the correction ordinal, budget (max), rung, re-dispatched CLI, and if that CLI was escalated. The reason is the rejection |
| `ORCHESTRATOR_HANDOFF_NO_ROUTE` | a cycle sealed handed-off, and the console route has no single committed item to name. The reason names the hand-off, committed-set source (lane pin, triage decision, or neither), and console steps. The cycle still ends handed-off with no routed item |
| `ORCHESTRATOR_HOST_EFFECT_FAILED` | the host failed to perform a declared effect before the phase's review; the effects gate still judges it |
| `ORCHESTRATOR_IDENTITY_PROOF_SKIPPED` | the fleet-rebase recovery did not run the identity proof, so Build re-authors the explanation; fields.reason says why the proof could not run (for example, the rebased change is committed, not pending on the new base) |
| `ORCHESTRATOR_LANE_BINDING_SETTLED` | a lane cycle held no manifest while the root registry bound its scope to an older cycle. Runtime settled the binding before the audit, at a lane resume or after the triage adoption of a fresh cycle: action=released drops it, action=recorded writes it to the manifest. standing is absent, settled, owed, unreadable, or newer. Release with standing=absent drops an unproven debt (trace event, WARN). Fields: fields.scope_id, ancestor_cycle, action, standing, origin (lane-resume or fresh-start) |
| `ORCHESTRATOR_LANE_DEFERRED` | a fleet lane failed to provision its worktree (after fetch retries and one re-provision) and exited 5 as DEFERRED. Claims release unbumped with no digest, learning, or retro. lane_deferral_halt_ceiling consecutive deferrals halt the batch (LOOP_PIPELINE_BLOCKER_HALT rule lane-deferrals). fields.step=worktree, cause |
| `ORCHESTRATOR_LANE_RESUMED` | a fleet lane found its todo's last cycle paused at a wall and resumed it at the paused phase. The dispatch chain picks the first healthy family. Resume uses the checkpoint auto-resume budget; INFO. Fields: fields.scope, phase, attempts, max_attempts |
| `ORCHESTRATOR_PHASE_ABORTED` | the cycle aborted after this phase's outcome (review reject, guard, persistence); the abort reason is in fields.abort_reason |
| `ORCHESTRATOR_PHASE_VERDICT_FAIL` | a phase recorded verdict FAIL; the reason is the phase's own error-severity diagnostics |
| `ORCHESTRATOR_PHASE_VERDICT_WARN` | a phase recorded verdict WARN; the reason carries its error-severity diagnostics, if any |
| `ORCHESTRATOR_PLAN_PHASE_GATED` | the phase registry removed a planned phase at its turn: skip_when fired (rule skip-when-gates-plan, for example bugfix on document cycles). It also fires if insert_when failed (rule insert-when-gates-plan). fields.phase, rule, next_phase. The walk continues and routing-decision-<n>.json keeps the clamp |
| `ORCHESTRATOR_QUOTA_PAUSED` | the dispatch ended at a wall with at least one family on quota, or no recorded cause. The cycle pauses at the named phase and stays resumable. A wall of only login or other causes does not emit this code |
| `ORCHESTRATOR_REBASE_REENTRY_ABORTED` | after a debugger resolved a fleet-rebase conflict, the host failed to carry, rebase, pend, or route the resolved tree (or recovery budget ended). The reason names the fault. The cycle ended instead of reshipping on the old base |
| `ORCHESTRATOR_RESUME_CARRIER_PENDED` | a resume found this cycle's Evolve-Carrier commit at HEAD (crash between rebase and pend) and pended it on its fork point. The resumed cycle sees the pending change instead of an empty diff |
| `ORCHESTRATOR_SCOPE_DELTA` | the build handoff floor found changed paths outside declared scope (ADR-0087). fields.total, fields.disposition_keep, fields.disposition_carve, and fields.disposition_refuse hold the counts; scope-delta.json lists the paths. WARN for carve or refuse, INFO when all are keep; reports only (shadow) |
| `ORCHESTRATOR_SHIP_RECOVERY_PATH` | a ship error naming a moved main (AUDIT_BINDING_HEAD_MOVED or GIT_FLEET_REBASE_NEEDED) took a recovery path. fields.path is rebind (identity carry re-bound audit), reaudit, rebuild, debugger, end, or next phase. Fields: fields.code (routed), fields.original_code (raised), fields.reason (why moved HEAD re-audited) |
| `ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN` | the ship-window lease was broken. A ship recovery broke the lease of a live holder whose heartbeat passed the TTL (fields.owner_pid and fields.heartbeat_at name the broken holder). Or the holder of a recovery lost its own lease at a beat (fields.role is holder, and fields.reason names the cause). Two lanes can overlap in the window |
| `ORCHESTRATOR_SYSTEM_FAILURE` | an ADR-0072 system-level failure was attached to the cycle (INCIDENT when it halts the loop, WARN otherwise); fields.category names the floor |

### outcome

| Code | Meaning |
|---|---|
| `OUTCOME_SIDECAR_SKIPPED` | the phase's <phase>-usage.json sidecar was not written because the workspace is empty (the CWD-relative leak guard); the in-memory record and the phase.outcome event still stand |
| `OUTCOME_SIDECAR_WRITE_FAILED` | the phase's <phase>-usage.json sidecar failed to encode or write. The reason names the step and error; fields name the phase and path. The in-memory record stands |
| `OUTCOME_TIMING_SKIPPED` | phase-timing.json was not written because the workspace is empty; the composed timing set is returned to the caller unchanged |
| `OUTCOME_TIMING_WRITE_FAILED` | phase-timing.json failed to encode or write (marshal, temp write or rename). The reason names the step and error; the composed set still returns |

### parallel.evaluate

| Code | Meaning |
|---|---|
| `PARALLEL_EVALUATE_SHADOW` | at parallel_evaluate.stage shadow, the orchestrator computed the parallel evaluate batch of enforce and dispatched it serially. fields.batch lists the phases in batch order |

### quality

| Code | Meaning |
|---|---|
| `QUALITY_CQ1_BREACH` | the whole 95% interval of CQ1 (unit conformance) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_CQ2_BREACH` | the whole 95% interval of CQ2 (packages per ship) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_CQ3_BREACH` | the whole 95% interval of CQ3 (interface depth) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_CQ4_BREACH` | the whole 95% interval of CQ4 (duplication) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_CQ5_BREACH` | the whole 95% interval of CQ5 (judgment surface) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_JQ1_BREACH` | the whole 95% interval of JQ1 (seeded-defect recall) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_JQ2_BREACH` | the whole 95% interval of JQ2 (false-PASS rate) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_JQ3_BREACH` | the whole 95% interval of JQ3 (independent-review escalation) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_OQ1_BREACH` | the whole 95% interval of OQ1 (escape rate) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_OQ2_BREACH` | the whole 95% interval of OQ2 (14-day churn) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_OQ3_BREACH` | the whole 95% interval of OQ3 (revert rate) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_OQ4_BREACH` | the whole 95% interval of OQ4 (recovery count) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_PQ1_BREACH` | the whole 95% interval of PQ1 (ship rate) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_PQ2_BREACH` | the whole 95% interval of PQ2 (pass^k per phase) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_PQ4_BREACH` | the whole 95% interval of PQ4 (deadline rate) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_PQ8_BREACH` | the whole 95% interval of PQ8 (idle-cause share) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_SOURCE_ABSENT` | a declared source of a quality index does not exist; fields.index and fields.source name it (ADR-0153) |
| `QUALITY_SOURCE_UNREADABLE` | a source of a quality index exists but does not parse; fields.source names it, the reason holds the error (ADR-0153) |
| `QUALITY_TQ1_BREACH` | the whole 95% interval of TQ1 (mutation efficacy on changed functions) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_TQ2_BREACH` | the whole 95% interval of TQ2 (revert sensitivity) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_TQ3_BREACH` | the whole 95% interval of TQ3 (test value) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_TQ4_BREACH` | the whole 95% interval of TQ4 (held-out share) is on the bad side of its threshold at stage report or gate (ADR-0153) |
| `QUALITY_VACUOUS_DENOMINATOR` | a quality reading has n = 0 where the index expects data; a rule that holds on an empty set proves nothing (ADR-0153) |

### repair

| Code | Meaning |
|---|---|
| `REPAIR_HANDOFF` | the fixability check found a hand-off finding before a repair round (ADR-0158). fields.stage is shadow or enforce; fields.decision is none, would_stop, or stop (seals handed-off). fields.reason is handoff_blocker on stop or would_stop. Fields: blockers, handoff, lane, console_items, salvage |

### review

| Code | Meaning |
|---|---|
| `REVIEW_AGENT_RUN_VERDICT_MISMATCH` | evolve agent run found a failing report body when the verdict field passes. The body states FAIL, states no verdict, or lists a CRITICAL or HIGH finding. The run verdict is FAIL. run_id names the run. Fields: fields.field_verdict, fields.stated, fields.critical, fields.high |
| `REVIEW_AGENT_RUN_VERDICT_PARSE_DEFECT` | evolve agent run found no stated verdict in a review report that lists no CRITICAL or HIGH finding. The parser or report format has a defect. The run verdict stays FAIL. run_id names the run, and fields.field_verdict names the field verdict |
| `REVIEW_FINDINGS` | a code-review dispatch completed and its findings were recorded in the cycle's defect-ledger.json (source code-review); INFO, or WARN when the rows could not be recorded (fields.error) or the findings_repair or quality_index config fell back (fields.config_warning); fields.findings (counts by severity), verdict, would_repair (any finding or a scores gap: the round-1 projection of the loop's decision), stage, threshold, round, recorded, scores (the quality-index vector), gaps (empty when the scores qualify), overflow |
| `REVIEW_SKIPPED` | a code-review dispatch was skipped and recorded nothing, so the walk continued and the audit stays the only gate; WARN; fields.reason malformed (the contract-correction ladder exhausted on the report, or its verdict stayed non-canonical after every retry), fields.round |

### runner

| Code | Meaning |
|---|---|
| `RUNNER_ATTEMPT_POSTMORTEM_FAILED` | the runner failed to list try postmortems (fields.step=read, skips Previous attempts), read one record (fields.path), or write an abnormal-end postmortem (fields.step=collect). The reason is the error (ADR-0130) |
| `RUNNER_DELIVERABLE_UNVERIFIED` | a CONTRACTED deliverable was still not well-formed after the settle window on the clean-exit path AND the final verdict is FAIL — the ship guard downgraded a clean-ship verdict (fields.downgraded=true) or Classify itself returned FAIL on the malformed or absent bytes; fault-only, a legitimate WARN/SKIPPED pass-through emits nothing; fields.codes, verdict_before, verdict, settle_attempts, deliverable |
| `RUNNER_HOST_EFFECT_FAILED` | the runner failed to perform a declared host effect before judging the phase. The engine and gate still judge the effect; reason holds the error |
| `RUNNER_OPTIONAL_PHASE_DEGRADED` | an optional phase hit a bridge infra teardown with no trustworthy deliverable and degraded to WARN; the cycle continues — a stream and console ADDITION (the arm was a response diagnostic only); fields.teardown, exit, cause, verr, stale_leftover, settle_attempts, deliverable |
| `RUNNER_RECONCILED` | a bridge infra teardown was overridden by a well-formed deliverable (fields.via=verify) or by the ACS deterministic floor (via=acs_floor, overridden_codes); the response carries Reconciled=true and core files the reconciled_timeout ledger disposition — ONE event where two INFO lines (ACS-FLOOR, RECONCILED) fired before; fields.verdict, deliverable, teardown, exit, settle_attempts |
| `RUNNER_SELF_REVIEW_MISSING` | a source-writing dispatch loaded code-review-simplify and handed off an accepted report without a Self-Review section. Advisory only; verdict stays unchanged. fields.skill, section |
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
| `SHIP_BACKSTOP_FLAKE` | the importer backstop named red tests that were green when their packages re-ran by themselves, without the pack's concurrent load; the ship proceeded on that evidence — recurrence is a hygiene item, not proof of a timing window |
| `SHIP_BASE_RED_MAIN_RED` | the scanner pack was red, every red test is red on the lane base and on main (or main could not be judged), so ship parks the lane instead of blaming it; fields.keys (package.Test of each base-red test), base, main (empty when unjudged), took |
| `SHIP_BASE_RED_REBASE` | the scanner pack was red, every red test is red on the lane base too and green on main, so ship asks for the fleet rebase instead of blaming the lane; fields.keys (package.Test of each base-red test), base, main, took |
| `SHIP_CARRY_NOT_REPROVEN` | an identical-rebase carry record named the bound audit, but ship failed to re-prove it. Reason names the failed check (record, ledger chain, ancestry, tree bytes, or patch-id); change re-audits. fields.carry_cycle |
| `SHIP_COMMITTED_DOC_DELETED` | atomic-ship: the landing deletes a path under docs/ or knowledge-base/ that the target branch holds; a rename into docs/, or within knowledge-base/, ships |
| `SHIP_COMMIT_GATE_MALFORMED` | verify-class: the commit-gate attestation cannot be parsed |
| `SHIP_COMMIT_GATE_MISSING` | verify-class: no commit-gate attestation exists for the staged tree |
| `SHIP_COMMIT_GATE_STALE` | verify-class: the commit-gate attestation is for a different tree |
| `SHIP_COMMIT_PREFIX_GATE` | atomic-ship: the commit-message prefix gate refused the message |
| `SHIP_CONTROL_PLANE_VIOLATION` | verify-class: a cycle-class commit touches the control-plane integrity surface (ADR-0064); ship it manually |
| `SHIP_EGPS_RED_COUNT` | verify-class: the EGPS gate reports red evals; red_count must be 0 to ship |
| `SHIP_EXPLANATION_DOCUMENTATION` | verify-explanation: the change ships without the explanation documentation its class requires |
| `SHIP_FOREIGN_PREDICATE_PACKAGE` | atomic-ship: a cycle landing adds a file under go/acs/cycle<M>/ of a cycle other than the shipping one; archive the superseded package with its explanation record |
| `SHIP_GATE_WEAKENED` | verify-class: the gate classifier flags a cycle-class commit that changes a kernel surface or weakens a gate (I-10, HEALING_I10_VIOLATION); ship it manually |
| `SHIP_GIT_COMMIT_FAILED` | atomic-ship: the commit step failed |
| `SHIP_GIT_DETACHED_HEAD` | atomic-ship: the tree is on a detached HEAD |
| `SHIP_GIT_FF_MERGE_DIVERGED` | atomic-ship: the fast-forward merge to main diverged |
| `SHIP_GIT_FLEET_REBASE_CONFLICT` | atomic-ship: the fleet rebase hit a genuine merge conflict; routed to the debugger (integrity, ADR-0049 G13a) |
| `SHIP_GIT_FLEET_REBASE_NEEDED` | atomic-ship: a peer lane moved main; rebase and re-verify the merged tree (transient, ADR-0049 S5b) |
| `SHIP_GIT_IO` | git I/O failure outside a named stage |
| `SHIP_GIT_LANDING_UNWIND_DECLINED` | atomic-ship: a failed landing also failed to unwind the lane to the audited shape, so the cycle stops. Journal holds commit; main stays still (integrity) |
| `SHIP_GIT_LANE_NOT_ON_ORIGIN` | atomic-ship: nothing to commit, but the lane holds unpushed commits without landing intent. Ship refuses PASS without push (integrity) |
| `SHIP_GIT_PUSH_POLICY_REFUSED` | atomic-ship: origin refused the push by policy (protected branch, hook, rule, token scope, or access). Ship skips retry, unwinds to audited shape, and ends without re-audit (precondition) |
| `SHIP_GIT_PUSH_REJECTED` | atomic-ship: the remote rejected the push |
| `SHIP_GIT_STAGE_FAILED` | atomic-ship: staging failed (transient) |
| `SHIP_HELD_FOR_OPERATOR` | atomic-ship: at held_proposals.stage enforce, a lane change touching the fence or pipeline item waits for evolve landing approve. Main did not move (ADR-0157) |
| `SHIP_HELD_PROPOSAL_REJECTED` | atomic-ship: the operator rejected this tree of a held proposal with evolve landing reject; main did not move (ADR-0157) |
| `SHIP_HOLD_SHADOW` | at held_proposals.stage shadow, a lane landing that touches the fence or works a pipeline-kind item lands, and the ship records that enforce would hold it for the operator (ADR-0157); fields.cycle, tree, reasons |
| `SHIP_INTEGRATION_COMPOSED` | a deliverable composed onto the integration branch and passed its L1 gates; fields.wave, fields.cycle and fields.batch name it |
| `SHIP_INTEGRATION_DELIVERED` | a fleet lane delivered its record to the wave integration (kind ship.delivered). At enforce, lane ship stopped before main. Fields: fields.wave, fields.cycle, fields.commits, fields.record |
| `SHIP_INTEGRATION_DRIVER_FAILED` | the integration driver failed a step. fields.step names the step and fields.error the error; failure writes to the wave record too, so lost signals cannot hide it |
| `SHIP_INTEGRATION_EJECTED` | a deliverable went back to its lane. fields.cause names why (conflict, gate, or proof); fields.paths lists conflicted paths. fields.brief_file holds the long brief path |
| `SHIP_INTEGRATION_FLUSHED` | a batch PR opened; fields.trigger names the flush trigger and fields.pr the PR |
| `SHIP_INTEGRATION_MERGED` | a batch PR merged into main by a fast-forward to the tested tip; fields.pr names the PR and fields.tip the tip |
| `SHIP_INTEGRATION_REVERTED` | the check after a merge reverted one cycle commit alone; fields.cycle names it and fields.commit the reverted commit |
| `SHIP_INTEGRITY_TREE_DRIFT` | post-ship: the shipped tree drifted from the verified one (integrity) |
| `SHIP_INVALID_CLASS` | verify-class: unknown ship class |
| `SHIP_LANDING_BINARY_RESET_FAILED` | git checkout HEAD -- <binary> before the ff-merge exited non-zero or failed to spawn; the merge still runs and may fail if the tracked binary is dirty; fields.step=integrate, path, git_rc, git_err |
| `SHIP_LANDING_BINDING_WRITE_FAILED` | ship-binding.json could not be created, written or renamed into the run workspace; the push already landed, the caller keeps shipping and keeps its WARN log line; fields.step=binding, path, err |
| `SHIP_LANDING_HEAD_READ_FAILED` | git rev-parse HEAD after the push landed errored or returned empty; the result's CommitSHA (the dossier's delivery identity) stays empty and the ship proceeds; fields.step=push, ref, err |
| `SHIP_LANDING_MAIN_ADVANCE_FAILED` | after the push landed, the fast-forward of the plane main to the pushed commit failed; origin holds the commit, and the ship continues; the next landing fast-forwards main to origin under the ship lock when the ship journal holds each commit between them; fields.step=integrate, branch, commit, git_rc, git_err |
| `SHIP_LANDING_PUSH_REPAIR_DECLINED` | the inline repair after a rejected push declined at the named probe (fetch, origin_ref, head, push_retry, or transport_retry after the bounded backoff); the original transient GIT_PUSH_REJECTED returns with repair_attempted/repair_outcome=declined; fields.step=push, branch, probe |
| `SHIP_LANDING_RESUME_FETCH_FAILED` | the git fetch of origin before a landing resume failed; the resume decides on the last known origin ref, and its push reports an outage; fields.step=resume, branch, git_rc, git_err |
| `SHIP_MAIN_CI_RED` | atomic-ship: the CI of origin/main is red and the main-red stop holds a CI key. No lane can prove a CI job green, so ship refuses the landing until the loop poll sees a green main (precondition) |
| `SHIP_MAIN_FAST_FORWARDED` | a ship fast-forwarded the local main of the plane to origin inside the ship window, because a ship failure named a moved main and origin held commits the local main lacked. fields.old and fields.new name the heads, fields.cycle the ship |
| `SHIP_MAIN_RED_PARK` | atomic-ship: the lane's red tests are red on its base and on main too. Ship does not blame the lane and parks it until main is green (precondition, ADR-0165 G4) |
| `SHIP_MANIFEST_GATE` | atomic-ship: a staged path was declared by no build/TDD report (cross-lane leak guard) |
| `SHIP_MANUAL_DECLINED` | verify-class: the operator declined the manual ship |
| `SHIP_MANUAL_NOT_TTY` | verify-class: a manual ship needs an interactive confirmation (or the audited --auto-confirm=<reason> parameter) |
| `SHIP_REPO_CONTRACT_CAPACITY` | atomic-ship: a repo-contract gate timed out on an unchanged tree, so the lane lacks CPU time and no LLM fixes it. Ship retries once and parks on a second kill (transient) |
| `SHIP_REPO_CONTRACT_GATE` | atomic-ship: a repo-contract guard suite is RED in the lane worktree. Pushing reds main |
| `SHIP_REPO_CONTRACT_INFRA` | atomic-ship: the repo-contract scanner toolchain died twice without a test-level failure (re-dispatchable) |
| `SHIP_SCAN_SHADOW_ESCAPE` | the scan shadow found a stored green for a package content key that failed in the real run. The content key misses a test input; cache cannot enforce until closed |
| `SHIP_SCAN_SHADOW_FAULT` | the scan shadow panicked and recovered; the ship scan and its verdict were not affected, and the shadow records for this scan are incomplete |
| `SHIP_SELF_SHA_IO` | verify-self-sha: the SHA pin failed to read or write |
| `SHIP_SELF_SHA_TAMPERED` | verify-self-sha: the ship binary's SHA does not match the pinned one (tamper or unpinned rebuild) |
| `SHIP_STATE_IO` | state file I/O failure |
| `SHIP_TRIVIAL_CRITICAL_PATHS` | verify-class: a trivial-class ship touches critical paths |
| `SHIP_TRIVIAL_NOT_TRIVIAL` | verify-class: a trivial-class ship exceeds the trivial diff bound |
| `SHIP_UNKNOWN` | an unclassified ship failure |
| `SHIP_WINDOW_LEASE_BROKEN` | the ship-window lease was broken. A ship broke the lease of a live holder whose heartbeat passed the TTL (fields.owner_pid and fields.heartbeat_at name the holder). Or the holder lost its own lease at a beat (fields.role is holder, and fields.reason names the cause). Two lanes can overlap in the window |
| `SHIP_WORKTREE_RESOLVE` | atomic-ship: the cycle worktree failed to resolve |

### signal

| Code | Meaning |
|---|---|
| `SIGNAL_EXPECTED_ABSENT` | a stage emitted its start signal, and its end signal did not follow before the policy deadline. fields.pair names the pair, start_kind and end_kind the kinds, started the start time, deadline_seconds the limit, and key the start. cycle, phase, and run_id fill fields and reason when present |

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

### tokens

| Code | Meaning |
|---|---|
| `TOKENS_ROW_UNATTRIBUTED` | a cycle workspace wrote an llm-calls row whose request carried no cycle number, so it cannot be attributed. Emitted at the enforce stage only. Fields: call_id, cli, agent |

### triage

| Code | Meaning |
|---|---|
| `TRIAGE_CAP_COMPANION_MALFORMED` | triage-decision.json beside the triage report is malformed (bad JSON or non-string-list committed_floors or deferred_floors). Capacity clamp falls back to counting report prose; WARN. Reason names file and error |
| `TRIAGE_CAP_DEMOTION_DEFECT_UNWRITTEN` | triage capacity clamp demoted to shadow for one cycle (ADR-0046 Layer 2), but auto-filed defect write failed. Cycle is relieved, but relief record is missing, so later cycles can re-trigger; WARN. Reason names file and error |

<!-- GENERATED:signal-codes END -->

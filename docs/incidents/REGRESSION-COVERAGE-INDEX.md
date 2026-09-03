# Incident → Regression Coverage Index

> **Purpose.** A living map from every documented incident to its root-cause failure mode(s) and the durable regression test that pins each one. This is how we incrementally build a stable, robust pipeline: every past failure becomes a test that fails if the bug returns. New incidents MUST add a row here when they ship a fix.
>
> **Generated:** 2026-05-29 (multi-agent coverage sweep over `docs/incidents/*`, verified against `go/**/*_test.go`, `acs/`, `tests/`). Confidence column reflects assessment certainty at sweep time; verify before relying on a `partial`.

## Legend

| Coverage | Meaning |
|---|---|
| ✅ covered | A durable test exists that WOULD FAIL if the bug were reintroduced. |
| 🟡 partial | A related test touches the area but does not pin this exact failure mode. |
| ❌ none | No regression test pins this mode. |
| ⛔ untestable | Depends on external infra/CLI behavior (vendor rate limit, interactive modal) a unit test cannot reproduce; mitigate by design + docs, not a unit test. |

## Summary (sweep 2026-05-29, 13-agent parallel coverage map)

| | Count |
|---|---|
| Incidents mapped | 14 |
| Distinct failure modes | 73 |
| ✅ covered (a test would fail if the bug returned) | 40 |
| 🟡 partial (related test, doesn't pin the exact mode) | 20 |
| ❌ none (no regression test) | 10 |
| ⛔ untestable (external infra / live CLI) | 3 |

**40 of 73 modes are truly pinned; 30 have concrete gap-test proposals.** The
prioritized backlog below lists the highest-value ones. The coverage map below
is the original hand-pass (kept for the per-incident narrative); the counts
above and the backlog reflect the fuller parallel sweep.

## Coverage map

| Incident | Failure mode | Suspect file | Coverage | Pinning test / proposal |
|---|---|---|---|---|
| [2026-09-03 repair rounds blind and flat](2026-09-03-repair-rounds-blind-and-flat.md) | declared `audit_retry_2plus` tier override had no producer; every repair round re-dispatched at the same tier | `core/retry_tier_escalation.go` | ✅ | `core/repair_tier_escalation_test.go` (`TestRepairRoundTier_*` + live-loop `TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop`) |
| 2026-09-03 repair rounds blind and flat | repair brief carried gate strings only; auditor's HIGH findings never reached the builder | `core/repair_brief.go` | ✅ | `core/repair_brief_test.go` (`TestComposeRepairBrief_*`, live-loop `TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt`) |
| 2026-09-03 repair rounds blind and flat | round prompts overwritten in place (no forensic trail of what round N was told) | `core/repair_brief.go` | ✅ | `TestArchiveRepairPrompts_RenamesOnceNeverClobbers` |
| 2026-09-03 repair rounds blind and flat | effort flat across an escalated tier | `bridge/launch.go` | ✅ | `bridge/effort_overrides_test.go` |
| [2026-08-09 zero-ship batch](2026-08-09-zero-ship-batch.md) | ship gate bound untracked minted profile stubs (false RED, 3 audit-green ships blocked) | `phasecoherence/unpaired_test.go` | ✅ | `phasecoherence/unpaired_tracked_test.go` + `unpaired_tracked_edge_test.go` (stderr fidelity, empty-set, staged, nested-alias) |
| 2026-08-09 zero-ship batch | disposition contract unsatisfiable (schema never shown; array evidence rejected as unparseable) | `phases/audit/defect_ledger.go` | ✅ | `audit/defect_ledger_evidence_shape_test.go` + `_evidence_edge_test.go` + `_schema_singlesource_test.go` |
| 2026-08-09 zero-ship batch | varied-fingerprint failure streak never halts (10 cycles burned) | `core/blocker_breaker.go` | ✅ | `core/blocker_breaker_consecutive_test.go` + `_edge_test.go`; `policy/policy_failure_consecutive_test.go` |
| 2026-08-09 zero-ship batch | retro completion detector cuts session after first deliverable (disposition.json never written) | `bridge/completion.go` / retro phase contract | ✅ | #432 Phase B: `bridge/completion_secondary_test.go` (holds while secondary missing, empty≠present, no-secondaries legacy) + `phases/audit/secondary_artifacts_test.go` (continuation-only arming) |
| [2026-08-10 persona-strip lobotomy](2026-08-10-persona-strip-lobotomy.md) | CompactPrompts stripped 73% of the auditor persona (verdict rules, stop criterion, MANDATORY disposition contract) — 15/30 FAILs, 0/11 continuation passes; compaction tests guarded only that bytes were REMOVED | `prompts/prompts.go` strip + `agents/evolve-auditor.md` marker placement | ✅ | `phasecoherence/persona_strip_operational_test.go` (incident anchors + fleet-wide sentinel keep-guard, shrink-only exception list) |
| [2026-08-10 absorbing-FAIL](2026-08-10-continuation-absorbing-fail.md) | declined registry binding outlives lineage → permanent audit auto-FAIL | `core/continuation_stamp.go` + `continuation/registry.go` | ✅ | `core/continuation_decline_release_test.go`; TOCTOU pin `continuation/registry_delete_test.go` |
| 2026-08-10 absorbing-FAIL | minted phase dispatched with no path disclosure (600s artifact-timeout) | `adapters/bridge/bridge.go` | ✅ | `adapters/bridge/bridge_contract_minted_test.go` (+ impossible-instruction negative pins) |
| 2026-08-10 absorbing-FAIL | ledger appends not chain-safe under fleet concurrency; `ledger anchor` can bind backward on shared seqs | `adapters/ledger/` | ❌ | **GAP:** inbox `ledger-fleet-concurrency-chain` (0.9) — fleet-safe appends + native rebaseline + unique-seq anchor guard |
| cycle-109-116 | Go orchestrator dropped per-cycle worktree provisioning (role-gate denied all writes) | `core/orchestrator.go` | ✅ | `core/orchestrator_test.go` worktree-provision path |
| cycle-119 | relative `--project-root` → ExitArtifactTimeout (artifact poll wrong dir) | `phases/runner/runner.go` | 🟡 | **GAP:** runner test asserting artifact path resolves absolute when root is relative |
| cycle-121 | codex REPL boot timeout; no fallback to next CLI | `phases/runner/cli_chain.go` | 🟡 | **GAP:** cli_chain advances to next CLI on boot-timeout exit 80 |
| cycle-122 | codex permission modal blocked run; fallback didn't fire | `bridge/driver_codextmux.go` | 🟡 | partial: `bridge/autorespond_decision_test.go`; **GAP:** modal-prompt → auto-respond decision |
| cycle-123 | codex edit-approval modal + empty fallback chain hard-failed | `phases/runner/cli_chain.go` | 🟡 | **GAP:** empty fallback list degrades gracefully (no panic, clear error) |
| cycle-124-137 | challenge token minted by two paths → diverged per phase | `bridge/driver_common.go` | ✅ | `bridge/coverage_batch7_test.go::TestPreparePrompt_ReadsExistingChallengeToken` |
| cycle-124-137 | ledgerverify counted only bash `kind=agent_subprocess`, not Go `kind=phase` | `ledgerverify/verify.go` | ✅ | `ledgerverify/verify_test.go::TestVerifyCycle_GoNativePhaseVocabulary` (+ MixedVocabularies, GoNativeIntentAndMemoPhases) |
| cycle-124-137 | ACS predicates hand-rolled `grep PASS` without `-v` → false RED | `acs/lib/assert.sh` | ✅ | `acs/lib/assert_test.sh` (11 assertions; exit-code based) |
| cycle-124-137 | stub `fakeLedger.Append` accumulated → masked the verify bug (green suite, broken prod) | `cmd/evolve/cmd_loop_m4_test.go` | ✅ | Append made no-op; the 6 dispatcher failure-path tests now genuinely fail verify |
| cycle-132-141 | orchestrator gaming: cycle number advanced without phase evidence | `core/orchestrator.go` | 🟡 | partial: ledgerverify empty-ledger → incomplete; **GAP:** orchestrator must not advance lastCycleNumber without ledger phase evidence |
| cycle-31 | worktree orphaned on rate-limit failure; builder WIP lost | `core/reset.go` | ❌ | **GAP:** SealCycle/reset archives an orphaned worktree's state rather than dropping it |
| cycle-46 | ship refused: tree-state SHA at ship ≠ audited SHA | `phases/ship/audit.go` | ✅ | `phases/ship/native_test.go` audit-binding matrix |
| cycle-61 | (thin doc) audit/ship friction, underspecified | — | ❌ | doc too thin to pin a mode; backfill the incident first |
| cycle-62 | ship refused: release audit verdict heading format mismatch | `phases/ship/audit.go` | 🟡 | **GAP:** audit-binding accepts canonical `## Verdict\n**PASS**`, rejects malformed |
| cycle-94-98 | watchdog over-fired, killed healthy phases | `adapters/observer/observer.go` | 🟡 | partial: `observer_test.go::TestObserver_NoStallWhenLogGrowing`; verify it pins the growing-stdout case |
| cycle-94-98 | soft-stall nudge not emitted before hard SIGTERM | `adapters/observer/observer.go` | ❌ | **GAP:** nudge emitted when `NUDGE_S < idle < STALL_S` before kill |
| gemini-forgery | cross-CLI: phase output not bound to cycle's real challenge token | `phases/scout/scout.go` | 🟡 | partial: `scout_test.go::TestComposePrompt_InjectsChallengeTokenFromRequest` pins the inject path; **GAP:** cross-CLI trust enforcement (auditor rejects a report whose token ≠ cycle token) |
| cycle-141 | builder (cwd=worktree) wrote `build-report.md` to worktree; driver polled only workspace → `exit 81` artifact timeout | `bridge/driver_common.go` | ✅ | `bridge/driver_artifact_relocate_test.go::TestArtifactReady_RelocatesFromWorktreeRoot` (+WorktreeWorkspaceSubdir, WorkspaceSubdirWinsOverWorktree, NoWorktreeConfigured_UnchangedBehavior) |
| cycle-141 | auto-spawn observer false `stall_no_output` on tmux drivers (live output → scrollback, stdout-log flat) | `adapters/observer/observer.go` | ✅ | `observer_test.go::TestWatch_WorkspaceActivityResetsStallTimer` (+WorkspaceConfiguredButIdle_StillStalls, ObserverEventsFileDoesNotMaskStall) |
| [cycle-1523/1524/1526 — transient upstream error inside an artifact timeout](2026-08-18-transient-529-inside-artifact-timeout.md) ([ADR-0090](../architecture/adr/0090-transient-disclosure-as-cause-data.md)) | transient upstream error (`API Error: 529 Overloaded`) inside an artifact timeout recognized by nothing → 600s burned per router invocation, routing degraded to static spine | `bridge/driver_tmux_repl.go`, `bridge/usageclassify.go`, `bridge/manifests/*-tmux.json` | ✅ | `bridge/artifact_timeout_transient_test.go::TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane` (verbatim cycle-1523 pane) + `TestEveryTmuxFamilyDeclaresTransientRecognition` (all 4 tmux families, wall/chatter disjointness) + `TestRunTmuxREPL_ArtifactTimeout_TransientFieldIsFamilyAgnostic` (codex) + `TestRunTmuxREPL_ArtifactTimeout_EchoedPromptIsNotTransient` (prompt-echo veto) + `TestClassifyTransientPane_UnknownDriverFailsOpen` |
| [cycle-1551 — persona-less menu phase kills a lane at load-agent](2026-08-24-personaless-menu-phase-lane-kill.md) | advisor SELECTs a catalog phase whose persona doc exists nowhere → runner load-agent fails → lane dies rc=4, ADR-0072 halt | `cmd/evolve/phaseroots.go`, `phases/runner/runner.go`, `core/errors.go`, `core/orchestrator.go`, `core/cyclerun_dispatch.go`, `prompts/prompts.go` | ✅ | `core/agent_doc_missing_test.go` (admission + floor/mandatory negatives + end-to-end ledger-kind replay) + `phases/runner/agent_doc_missing_wiring_test.go` (missing/unreadable/nil-source on the REAL load path) + `cmd/evolve/demote_personaless_test.go` (registration-seam demotion, helper + composed path) + `core/advisor_catalog_ondemand_test.go::TestRepoPhaseCatalog_MenuPhasesResolveAPersona` (tracked-menu guard) + `prompts` zero-loader both-sentinels pin — 11 mutants killed |
| [cycle-1550 — anchor-order-sensitive splice mis-slots planned phases](2026-08-24-anchor-order-splice-misslot.md) | spec anchored to an alphabetically-later spec misses its anchor at splice time → silent before-audit fallback → red-first Evaluate phase executes post-build → audit FAILs the lane on its own deliverable | `phasespec/routing.go` | ✅ | `phasespec/routing_anchor_fixpoint_test.go` (cycle-1550 shape + reversed chain + unresolvable-anchor warning + cycle termination + transitive-block anchor honor) + `cmd/evolve/routing_order_realtree_test.go` (production-seam order pin, red on unfixed code with both live victims) — 6 mutants killed |
| [cycle-1552→1553 — triage bookkeeping defeats in-commit consumption](2026-08-24-wave2-consumption-id-linkage.md) | PASS lane ship resolves committed ids from triage top_n only; a dropped/decomposed assigned id consumes nothing → stale item re-picked, next wave burns a lane on finished work | `phases/ship/postship.go`, `inboxmover/outcome.go` | ✅ | `phases/ship/consume_lanescope_union_test.go` (per-id rule: dropped consumes, deferred stays, declined-menu preserved, decomposition unions) + `consume_integration_test.go` cycle-1552 end-to-end replay — 6 mutants killed |
| [cycle-1550 family — completion detector certifies the prior attempt's stale artifact](2026-08-25-stale-artifact-completion-baseline.md) | correction re-dispatch: detector's stability window starts on first sight, so an untouched pre-dispatch leftover 'stabilizes' and completes — the prior verdict re-grades itself every retry | `bridge/completion.go`, `bridge/driver_tmux_repl.go`, `bridge/engine.go`, `phases/runner/runner.go` (the reconcile-on-teardown second door) | ✅ | `bridge/completion_baseline_test.go` (salvaged 1554 red test + finality both-ways + coarse-mtime size pin + withDefaults tripwire + full-engine LaunchArgs replay) + `phases/runner/runner_reconcile_stale_test.go` (both reconcile doors refuse the leftover; rewritten still reconciles) — 12 mutants killed |
| [cycle-1558 — manual inbox consume leaves the continuation binding immortal](2026-08-25-manual-consume-immortal-binding.md) | operator consume moves the file but not the scope-keyed binding → next wave mints a lane off the dead binding → zero-delivery burn (the binding half of the cycle-1553 class) | `cmd/evolve/cmd_inbox_consume.go`, `cmd/evolve/cmd_loop_blockerbreaker.go`, `continuation/registry.go` | ✅ | `cmd/evolve/cmd_inbox_consume_binding_test.go` (consume releases + prior preserved, sweep strays-only, breaker-boot wiring pin) — 6 mutants killed |
| [v22.20.0 release red — submit-verify false-wedge on silent REPLs](2026-08-25-submitverify-false-wedge-release-red.md) | pane-echo heuristic reads a silently-consuming REPL as parked → instant exit 81 on happy paths; landed unseen (quiet-host-skipped local bars + unwatched post-push CI) | `bridge/driver_tmux_repl.go` | ✅ | `bridge/driver_tmux_delivery_failure_test.go` (parked+delivered OK / parked+stale still fast-fails) + RealTmux no-resend pin — 4 mutants killed |
| [cycle-1571 H3 — audit FAIL invisible to ship: binding fail-open across cycles](2026-08-26-audit-binding-fail-open.md) | FAIL verdict emitted no auditor binding + run-scope lookup miss silently bound a sibling lane's audit → observed: foreign HEAD_MOVED masks this run's FAIL, burns a re-audit slot; latent: FAILed cycle ships on sibling's PASS when HEADs coincide | `core/phase_bindings.go`, `phases/ship/audit.go`, `cmd/evolve/cmd_composition_wiring.go`, `core/composition_carryforward.go`, `internal/subagent/`, `internal/verdictcache/`, `internal/releasepreflight/` | ✅ | `core/phase_bindings_fail_verdict_test.go` (FAIL records binding, cache-Put refused with WARN vacuity control) + `phases/ship/runscope_test.go` (foreign-run + unstamped refusal, flipped fallback pin) + `cmd/evolve/cmd_composition_runscope_test.go` (third reader run-scoped). **Follow-up 2026-08-27** (architect review of the fix found 3 HIGH + 1 live release block): `core/ledger_runid_writers_guard_test.go` (writer set closed, every writer calls the resolver), `core/runworkspace_runid_test.go`, `subagent/runid_stamp_test.go` (stamped/omitted/reader-decodable), `cmd/evolve/cmd_composition_verdict_guard_test.go` (rejection refused, wired before git work), `core/ship_recovery_runid_seam_test.go` (seam receives this run's id), `verdictcache/reusable_test.go`, `releasepreflight/recent_audit_scope_test.go` (veto scoped both directions) — 6 further mutants killed |
| [2026-08-27 — plan-mode dialogs: a live-reachable hang no rule matched](2026-08-27-plan-mode-dialog-blind-spot.md) | both tmux CLIs reach a blocking plan-mode dialog from the production launch config (claude enters plan mode even under --dangerously-skip-permissions; codex collaboration_modes graduated-on) and no interactive_prompts rule matched either; separately, a manifest regex that fails to compile is silently skipped, making a rule dead on arrival | `bridge/manifests/{claude,codex}-tmux.json`, `bridge/autorespond.go`, `bridge/manifest.go` | ✅ | `bridge/autorespond_planmode_test.go` (live dialogs fire; answered/scrolled-up panes and the repo's own doc+fixture files do NOT; multi-question form answered to the guard limit) + `bridge/manifest_prompt_regex_compile_test.go` (every rule in every manifest compiles) — verified against raw untranscribed captures; 5 mutants killed |
| [wave-3 zero-ship — an audit FAIL was terminal, and prose outranked the deterministic floor](../architecture/adr/0092-audit-repair-loop.md) | cycles 1572/1573/1574 spent 367 lane-minutes for 3 FAILs and 0 ships; two root causes were the STAGED INDEX not the code, yet no path re-entered build. Compounding: all three halted on the agent-authored failure-decision.json category while their failure-dossier.json recorded floor_candidate:"" and the SAME agent's disposition.json said legit-rejection | `core/repair_eligibility.go`, `core/decision_branch.go`, `core/cyclerun_dispatch.go`, `core/disposition_gate.go`, `cyclestate/state.go`, `policy/policy.go`, `phases/{build,tdd}` | ✅ | `core/repair_eligibility_test.go` (11-case table + incoherence-requires-contradiction; gate-1-absolute and cap off-by-one mutants killed) + `core/decision_branch_repair_test.go` (repairs when contradicted / STILL HALTS with the byte-identical inputs minus disposition.json — the ADR-0072 guard / deterministic floor beats legit-rejection / cap / cap-0-disables) + `core/audit_repair_grant_test.go` (bound increments only on a repair reason; both branch surfaces consume it — resume-drop mutant killed; reason prefix single-sourced; breaker counts one cycle once) + `core/audit_repair_context_test.go` (dispatch SETS the key, no mutation of the shared ctxSnap, missing artifact degrades) + `phases/build/audit_repair_prompt_test.go` and `phases/tdd/audit_repair_prompt_test.go` (both prompts RENDER the verbatim reason, DATA-fenced) — both wiring-proof halves mutation-killed |
| [waves 3-4 zero-ship: retro conflated analysis, classification and retry-gating](../architecture/adr/0093-retry-envelope-and-terminal-retro.md) | retro PASS ("post-mortem complete") routed as "cycle recovered" sent a rejected, byte-identical tree to ship; the routed arm additionally DISCARDED the ADR-0072 floor signal on that path; the retro gate looked for failure-lesson*.yaml in the workspace while the persona writes .evolve/instincts/lessons/inst-L*.yaml (220/238 retro FAILs, 92%, were false); and ADR-0072's declarative retry policy (action/max_retries/fix_type) was consumed nowhere while a parallel knob was built beside it | `core/retry_envelope.go`, `core/audit_fail_decision.go`, `core/retry_adjudicator_agent.go`, `core/decision_branch.go`, `core/statemachine.go`, `phases/retro/retro.go`, `policy/policy_failure.go`, `docs/architecture/phase-registry.json` | ✅ | `core/retry_envelope_test.go` (11-case envelope + cap-comes-from-the-table wiring proof; 4 mutants killed incl. a hardcoded cap) + `core/retry_adjudication_test.go` (clamp cannot widen, halt not negotiable, unjustified rejected, dispatch skipped when no choice; 4 mutants killed) + `core/audit_fail_decision_test.go` (6 dispositions, floor-not-overturnable, nil-adjudicator-safe, re-entry edges legal, and a FULL-CYCLE live test that the retry actually happens and the budget binds) + `core/retro_verdict_semantics_test.go` (retro PASS is not recovery; PASS and FAIL reach the same ladder; the routed arm cannot drop the floor signal) + `phases/retro/lesson_resolution_test.go` (cycle-scoped lesson resolution, legacy shape still accepted, persona/gate path single-sourced; rubber-stamp AND regression mutants both killed) + `policy/retry_policy_for_test.go` + `core/retry_adjudicator_agent_test.go` (every malformed proposal degrades to nil; profile path matches the shipped profile) |
| [cycle-1603 — repair round inherits superseded acs-verdict; diagnosed downgrade halts as forgery](2026-09-02-repair-round-stale-acs-verdict.md) | (1) EVERY audit re-dispatch (repair loop, bookkeeping regrade, ship-error recovery, RERUN_PHASE) left the previous round's acs-verdict.json/audit-report.md at canonical paths; verdict-exists gate replayed the stale ship_eligible=false into every fresh round → repaired PASS force-downgraded, repair structurally cannot succeed; (2) applyFailureDecisionFloor path-2 halted on prose verdict-incoherence despite persisted substantive fail reasons (deterministic SubstantiveError guard had declined) | `core/audit_round_artifacts.go`, `core/cyclerun_dispatch.go`, `core/resume.go`, `core/decision_branch.go`, `core/system_failure.go`, `cyclestate/state.go` | ✅ | `core/audit_round_artifacts_test.go` (helper contract: round-suffixed archives, round-0 honors operator pre-stage, no-clobber; LIVE wiring pins on BOTH dispatch surfaces — `TestAuditRedispatch_RetiresPreviousRoundVerdicts` + `TestResumedAuditRedispatch_RetiresPreviousRoundVerdicts` assert retirement at the re-dispatch moment; both seam mutants killed; `TestResumedCrashedAudit_RetiresTheDeadAttemptsVerdict` + `TestSupersedePreviousAuditRound_*` pin the dispatch-persisted `AuditDispatches` index for the crashed-mid-audit shape and the legacy-checkpoint completion floor; increment mutant killed) + `core/decision_branch_floor_test.go::TestDecideAfterRetroFloor_Cycle1603DiagnosedDowngradeIsNotForgery` + `::_DiagnosedShipReasonsAlsoContradictProse` (both halves of hasSubstantiveFailReasons pinned; predicate + suppression mutants killed) + `::_ProseIncoherenceWithoutDiagnosedReasonsStillHalts` (forgery floor preserved) |
| [cycles 1603/1604/1605 — the auditor rewrites the builder's tree; the explanation digest gate fires on the auditor's mutations](2026-09-03-auditor-mutates-the-worktree.md) | a read-only phase's agent has a shell; nothing fenced the worktree; the audit's digest check ran after the agent on the polluted tree; the profile's read-only sandbox was not applied (`sandbox=false`) | `phases/runner/runner.go` (no fence), `core/cyclerun_dispatch.go` (permission predicate not on the request) | ✅ | `internal/treefence/fence_test.go::TestRestore_UndoesEveryKindOfWriteAndLeavesIgnoredAlone`; `internal/phases/runner/worktree_fence_test.go::TestRun_ReadOnlyPhaseWorktreeIsRestoredAndReported`; `internal/core/cyclerun_remediate_test.go::TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot` |
| [cycles 1604/1605/1606 — the explanation review rejected on shape: list-valued Evidence, range citations, backticks, other field names](2026-09-03-auditor-mutates-the-worktree.md) | `reportdoc.Fields` treated a repeated `Evidence` as a duplicate, `citationLine` rejected `:23-48`, backticked values never matched, prose citations were invisible | `reportdoc/reportdoc.go` | ✅ | `internal/reportdoc/review_fields_test.go::TestFields_EvidenceIsListValued_Cycle1604`, `TestCitation_AcceptsLineRangesAndColumns_Cycle1605`, `TestFields_StripsBackticksAroundValues_Cycle1606`; `internal/phases/audit/audit_test.go::TestValidateExplanationReview_ReadsTheSectionAsAuditorsWriteIt` |
| [cycles 1601/1603 — a missing `## Explanation Documentation` section was a terminal audit FAIL](2026-09-03-auditor-mutates-the-worktree.md) | the audit gate forced FAIL on a missing section; no correction rung asked the auditor to add it | `phasecontract/contract_registry.go` (no conditional section), `deliverable/reviewer.go` | ✅ | `internal/deliverable/reviewer_explanation_section_test.go::TestReviewer_ExplanationSectionRequiredOnlyWhileContractActive`; `internal/phasecontract/explanation_sections_test.go::TestAuditContract_ExplanationSectionIsConditional` |

## Prioritized gap backlog (13-agent sweep; agent confidence in parens)

Highest value first (regression likelihood × blast radius × testability). Each
target was verified by an agent that read the incident and searched the suite.

**cycle-138 result (Bug A+B validation — POSITIVE).** Verified from
`/tmp/cycle138.log` + ledger + audit-report:
- All 7 phases ran; **audit verdict PASS, red_count 0** — the first clean audit
  in the 130-138 sequence.
- `VerifyCycle(138)` against the real ledger returns `OK=true, missing=[]`
  (probe test confirmed). **Bug A's fix works — NO dispatcher false-negative
  this cycle** (cycle-137's "missing [scout builder auditor]" did NOT recur).
- Bug B's fix was exercised live: the TDD-engineer authored 5 ACS predicates
  that source `acs/lib/assert.sh` (`assert_go_test_pass`, `assert_go_coverage_ge`)
  instead of hand-rolling `grep PASS`.
- `FinalVerdict=SHIPPED_VIA_BUILD` (v12.2 disambiguation: HEAD moved during the
  cycle → build committed inline; this is a *success* label, not a failure). The
  `cost $0` reflects OAuth-session billing (CLAUDE.md note), not zero work.

**UPDATE after cycle-139 (supersedes the "cosmetic" note above).** cycle-138
shipping "via build" rather than via the formal ship phase is NOT cosmetic — it
is the visible symptom of a real defect found in cycle-139: the audit phase's
EGPS gate requires `acs-verdict.json` (red_count==0) to PASS, treats a MISSING
file as FAIL by design, and **nothing in the autonomous loop generates that
file** → every autonomous audit is structurally forced to FAIL → `audit→retro`,
no formal ship. cycle-138 shipped only because its build committed inline before
the forced-FAIL audit; cycle-139's build did not, so it shipped nothing
(`SKIPPED_UNKNOWN`). Full analysis + approved fix:
[cycle-138-140-egps-verdict-not-generated-in-autonomous-loop.md](cycle-138-140-egps-verdict-not-generated-in-autonomous-loop.md).
This is gap #0 below — THE current blocker for two clean *shipped* cycles.

## Prioritized gap backlog continued

0. **EGPS verdict not generated in autonomous loop (cycle-138/139, ❌none, 1.0)** —
   THE blocker. Audit forces FAIL on missing `acs-verdict.json`, which the
   autonomous `evolve loop` never produces. Approved fix: the **audit phase
   generates it** via `acssuite.Run`+`WriteVerdict` when absent (honoring a
   pre-staged file). Regression test: a cycle with `acs/cycle-N/` predicates +
   no pre-staged verdict → audit generates it → red_count 0 → PASS → `audit→ship`.
   Files: `phases/audit/audit.go` (Classify), `acssuite` (Run/WriteVerdict),
   `statemachine.go:96`. See the dedicated incident doc.
1. **cli_chain empty-fallback (cycle-123, ❌none, 0.95)** — a profile with no
   `cli_fallback` key + a fallback-trigger exit (81) attempts NO fallback; cycle
   aborts. The "any CLI any phase" invariant. →
   `runner/runner_fallback_test.go::TestRun_FallbackOnArtifactTimeout_EmptyProfileFallback`
   asserting `calls==[primary]`, plus a sibling where a populated chain DOES
   advance. `runner/cli_chain.go:resolveCLIChain`.
2. **Cross-CLI trust bypass (cycle-119 + gemini-forgery, ❌none, 0.9)** — a
   read-only phase run via a non-Claude driver can write to the main tree
   (Claude-Code PreToolUse hooks don't bind other CLIs). → `internal/core`
   integration test: run a read-only phase via a non-Claude driver in a worktree,
   assert main-tree source files unchanged post-phase (diff guard).
3. **Observer auto-spawn wiring (cycle-122, 🟡partial, 0.9)** —
   `wireOrchestratorDeps` wires `WithObserver(NewCoreAdapter())` when
   `EVOLVE_OBSERVER_AUTOSPAWN!=0`, noop when `=0`. → `cmd/evolve` wiring test.
4. **ledgerverify anti-gaming (cycle-132-141, 🟡partial)** — a cycle whose ledger
   has zero phase entries is reported incomplete AND `lastCycleNumber` does not
   advance. Builds directly on the cycle-137 verify fix; cheap.
5. **Boot-scrollback load-bearing (cycle-121, 🟡partial, 0.75)** — codex-tmux boot
   with `bootScrollback=0` + trust modal → `ExitREPLBootTimeout`. `driver_tmux_repl.go`.
6. **codex per-edit-approval modal (cycle-123, 🟡partial, 0.85)** — synthetic
   apply_patch fixture → modal appears and is auto-dismissed by the
   `interactive_prompts` regex (manifest→pane integration). `driver_codextmux.go`.
7. **stall-vs-progress (cycle-109, 🟡partial, 0.75)** — artifact-wait extends on
   pane progress, pauses on stall (StopReviewer). `internal/bridge`.

Lower tier (hand-pass, still valid): ship audit-binding format (cycle-62), runner
relative-root (cycle-119), reset orphan-worktree (cycle-31), observer
nudge-before-kill (cycle-94-98). 30 modes total carry concrete gap proposals.

## Untestable-by-unit (mitigate by design + docs, not a test)

- codex/ChatGPT vendor rate limit (cycle-128) — operator-account state; mitigate via CLI fallback + clear operator message.
- Interactive CLI modals as raw terminal state (cycle-122/123) — the *decision logic* is testable (auto-respond mapping) but the live modal render is not.

## Contract for new incidents

When an incident ships a fix, add a row here with the pinning test path::name, and flip its coverage to ✅. A fix without a regression row is incomplete (CLAUDE.md Rule 9 — tests verify intent). This index is the single source of truth for "is the pipeline getting more stable over time."

# Verification wave 1 after the decomposition train — findings (2026-09-14)

> **Purpose.** The research record of the first loop wave run on the merged Signal Center + ADR-0103
> decomposition train (15 units, PRs #587–#601; main `80b348e6`, runtime plane `292cc033`, loop pid
> 3105, log `runtime/.evolve/loop-20260914-1140-verify.log`). It answers the program's own test —
> *can the root cause of a failed cycle be read from a few signal lines?* — and records what the
> wave found, what was fixed the same day, and what is still open. Companion docs:
> [program review memo](../architecture/decomposition/00-program-review-2026-09-14.md) ·
> [Pipeline Factory Rules](../operations/pipeline-factory-rules.md) ·
> incidents [triage-refusal poison loop](../incidents/2026-09-14-triage-refusal-poison-loop.md) and
> [codex prompt submit wedge](../incidents/2026-09-14-codex-prompt-submit-wedge.md).
> Operator goal for the wave: *"continue to fix the pipeline issues until we see 5 consecutive shipped results"* — this wave contributed **0 ships**, and two of the three reasons are now fixed.

## 1. Method

- Reconcile the runtime plane onto main at a wave boundary (`git merge origin/main` → rebuild → `evolve reset-sha -operator` → fresh launch with the plane's `goal.txt`), fleet width 3.
- Measure with `measure-wave.sh` (memo §6): lanes/ships per wave from the loop log; every `signals.ndjson` written since launch grouped by module × severity and by code; registry-drift events; console WARN/INCIDENT lines by module tag; suite litter on the plane.
- For every FAIL, triage from the stream first, then the durable artifacts (`phase-timing.json`, `*-interactions.ndjson`, `.evolve/ledger.jsonl`), and only then the code.

## 2. What the stream showed (first 25 minutes)

| Module × severity | Events | Notable codes |
|---|---|---|
| ledger INFO | 133 | `ledger.appended` (kind-only) |
| bridge WARN | 30 | `BRIDGE_TOKEN_USAGE_WARNING` 17 · `BRIDGE_TELEMETRY_TRIPWIRE` 9 · `BRIDGE_EXIT_ARTIFACT_TIMEOUT` 2 · `BRIDGE_CONTEXT_FILL_HIGH` 2 |
| gate.contract INFO | 13 | `GATE_CONTRACT_VERIFIED` |
| orchestrator INFO / WARN | 12 / 2 | `phase.outcome`; `ORCHESTRATOR_PHASE_VERDICT_FAIL`, `ORCHESTRATOR_CYCLE_FAILED` |
| liveness WARN · inbox WARN | 1 · 1 | `LIVENESS_PANE_STAGNANT` · `INBOX_CLAIM_NOT_FOUND` |
| registry drift / unknown module | **0** | every producer registered — the closed vocabularies held |

Console lines carried the module tag and code in every case; no Center-less emit was observed on the lane path.

## 3. Findings

### F1 — one item drew nine lanes: a triage refusal had no failure class (P1, fixed — PR #606)

Cycle 1675's story was four stream lines: `BRIDGE_EXIT_ARTIFACT_TIMEOUT cause=submit_wedged` (scout) → `ORCHESTRATOR_PHASE_VERDICT_FAIL phase=triage … names protected surface go/internal/bridge/streamjson_verdict.go` → `ORCHESTRATOR_CYCLE_FAILED phases_run=2` → `INBOX_CLAIM_NOT_FOUND` for an item that was right there. The ledger then showed the same item recovered untouched on cycles 1650, 1653, 1655, 1658, 1661, 1665, 1669, 1672 and 1675 — nine waves of scout + triage tokens with zero progress. Mechanism: the triage gate's own refusal carried a prose-only diagnostic; `cycleclassify` had no class for it; the closeout read system-level; the ADR-0072 S5 drain never bumped `failure_count`; and the closeout's re-claim of an already-claimed id raised the false not-found. Fix (same day): a structured refusal code and subject on the C1 record and in the signal (`diagnostic_codes=`), the task-level class `phase-refusal`, a per-code disposition table, and a first-hit route of the refused card to `console-manual` (`INBOX_ITEM_ROUTED_CONSOLE`). **Design goal verified:** the root cause was read from the stream and the ledger before any code was opened.

### F2 — "submit_wedged": the driver declared a 35 KB codex prompt wedged 2.5 s after pasting it (P1, fixed — this change)

Two of three scouts (attempt 1) and one recovery build timed out with `cause=submit_wedged`; every other codex-tmux phase in the wave needed 2–3 Enter re-sends to land. Mechanism: a fixed 1 s settle plus three 500 ms re-sends while the TUI was still ingesting the paste. Fix: one delivery tail (`paste_settle.go`) with a size-scaled settle, a bounded wait for the pane to stop changing, backoff between re-sends, and the paste evidence recorded in the submit-verify ledger on the success path. Follow-up filed in the incident doc: a WARN signal when re-sends ≥ 2 so the next timing regression is one grep away.

### F3 — a peer moved main mid-pipeline: `SHIP_GIT_FLEET_REBASE_NEEDED` (observed, by design)

Cycle 1673's ship found main advanced (four PRs from a parallel operator session landed during the wave) and took the documented recovery (`ship aborted after verdict=FAIL: … recovering via build (attempt 1/4)`). The recovery then hit F2. Nothing to fix in the ship path; the cost was the re-dispatch, which F2's fix reduces. Standing rule confirmed: tracked-path landings from operators belong at batch boundaries.

### F4 — `AUDIT_CIPARITY_GATE_STEP_FAILED` and `ADVISOR_RESPONSE_UNPARSEABLE` (both fixed: advisor half #614, apicover half = F9)

Cycle 1673's audit CI-parity apicover step failed inside the lane worktree and the advisor's replan proposal was not JSON (`invalid character '.'`). Both are now single, coded stream lines (units 14 and 04 respectively) — they were the ones the memo predicted would become readable first. Neither blocked the cycle by itself; both are queued as inbox items for the next wave rather than fixed by hand, because neither is deterministic on its own evidence yet.

### F5 — the poison set on the plane

Ledger `recover` actions since cycle 1640 named eight ids; only `verdict-sentinel-as-tool-call` (9×) was still pending at the inbox root — routed `console-manual` by hand as the mitigation before F1 landed. The others had already been consumed or routed.

### F6 — the repo-contract gate ran `go test` in the lane's IPC environment (P1, fixed — this change)

Lane 1677's importer backstop (the layer #612 added) went RED on twenty-odd tests in `cmd/evolve`, `guards`, `ship` and `core` — every one env-sensitive (cycle-reset lease fencing, "outside a cycle", seal role, fleet-off goldens) and every one green in the same worktree under `env -i PATH HOME`. The gate's `go test` inherited `EVOLVE_FLEET=1` and `EVOLVE_CYCLE_STATE_FILE=<the lane's run dir>` from the lane process; core already scrubbed those for its own `go test` spawns with a private `sanitizeEnv`, the ship gate never did. Fixed by `ipcenv.Scrub` (the namespace owner projects the scrub) wired into the ship runner and the four core sites. Cost on this wave: 1677's ship aborted and a recovery audit + re-ship are spent under the running plane. Record: `docs/incidents/2026-09-14-ship-gate-inherits-the-lane-ipc-env.md`.

### F7 — codex's deep tier is pinned to a model the account rejects, and the bridge had no rule for it (P1, fixed — this change; pin = operator's call)

Both wave-2 lanes dispatched build on `codex-tmux@deep`, got `400 invalid_request_error: The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account`, and idled for the full 1500 s artifact window before the runner fell back to Claude (exit 81) — the operator read it as "codex limit reached". Not a limit: a dead model, unrecognised because the auto-responder knows walls only by manifest rule. Fixed with a `model_unsupported` escalate rule (exit 85 → immediate family fallback, no clihealth bench). Codex launched fine for scout, triage and tdd in the same wave (the balanced/fast pins are accepted), so the dead pin is the deep/top row alone; the re-pin needs the operator's cost decision. Record: `docs/incidents/2026-09-14-codex-deep-tier-model-rejected.md`.

### F8 — Claude's session wall went unrecognised: tmux indents it with U+00A0 (P1, fixed — this change)

The account's rolling session limit hit at 19:03 (reset 19:50). Both lanes' recovery audits parked on `⎿  You've hit your session limit · resets 7:50pm` and the guarded fast-fail never armed — the captured line carries a NO-BREAK SPACE after `⎿`, and the session-wall branch of claude's `exhausted_regex` admitted only `[ \t]`, so `ClassifyExhausted` said no while the broad `drift_probe_regex` said yes (`POSSIBLE EXHAUSTION-REGEX DRIFT` at teardown). Each dispatch ran the full 2400 s artifact window and was re-dispatched into the same wall. Fixed by `[\t\p{Zs}]` in that regex, red-first with the live bytes. The memory rule from 2026-07-18 (validate regexes against the REAL pane) applied verbatim. Record: `docs/incidents/2026-09-14-claude-session-wall-nbsp-drift.md`.

### F9 — the audit's CI-parity apicover step inherited the lane env too (P2, fixed — this change)

The `cover_run` half of F4: `coverageProfile` spawned its scoped `go test -tags "integration acs" -coverprofile … ./internal/core` through the gate's raw runner while the tier step beside it scrubs, so the lane's `EVOLVE_CYCLE_STATE_FILE`/`EVOLVE_FLEET` flipped core's env-sensitive tests and the step failed on every audit of both waves (fail-open → WARN + a ~7-minute coverage run each). Reproduced with the two variables in a clean worktree. Fixed by running the step under `scrubbedRun`, red-first with the gate's fake runner (it received a nil env = inherit). Record: `docs/incidents/2026-09-14-ship-gate-inherits-the-lane-ipc-env.md` (second-site section).

### F10 — the retro never entered the fallback walk, and eleven profiles had no chain (P1, fixed — ADR-0104)

Both wave-2 retrospectives timed out on codex's dead deep model and the cycles sealed with no disposition, while the same exit on the same CLI in build had fallen back to Claude: the runner walks `DispatchTiered`, the retro (and the failure advisor, phase judge, retry adjudicator, swarm launcher) called the bridge directly. Behind it: the universal tail was appended only when the configured chain was *absent*, and eleven claude-primary profiles (auditor, tdd-engineer among them) had no `cli_fallback`. Fixed by making the walk a property of the bridge handle (`bridgechain.Walking`, wrapped once at the composition root), appending the tail unconditionally, moving the agy ban to `workflow.universal_fallback_exclude`, and giving every agent a chain (in-family only on the five Claude-family-floor agents). Records: ADR-0104, `docs/incidents/2026-09-14-retro-timeout-sealed-the-cycle.md`.
### F11 — push-only could not recover the strand it exists for (P1, fixed — this change)

Lane 1678's push was rejected after its gate passed and its commit was minted (a console PR had been merged mid-wave — now a standing rule); `evolve ship --push-only` refused the commit by name ("lack ship provenance") because the ship journal was appended only on finalize's success path, after the push. The journal now records every MINTED commit. Recovery until then: a normal `evolve ship --class manual` on the plane after merging origin. Record: `docs/incidents/2026-09-14-ship-gate-found-what-the-audit-could-not.md`.

### F12 — the ship gate found a red the floor and the audit could not see, and the repair ladder had no owner for the fix (P1, fixed — this change)

Lane 1679's added `//go:build acs` package was invisible to the floor's default-context run and first executed by the ship's added-test backstop — red on the eval the scout had materialized without a `[code]` grader, a rule the scout gate advertised but never checked. The builder's sandbox denies `.evolve/evals`, so two repair rounds burned before the tdd phase applied the builder's own remedy file. Fixed: `internal/addedtests` shared by floor and ship (tag-gated added packages run at the floor under their tags), the scout gate enforces the grader rule, and the remedy routing for "unappliable by this phase" is recorded as F13. Record: same incident.

### F13 — a builder-declared "unappliable by this phase" failure is re-run instead of routed to the path's owner (P2, open — design)

Round 2 of cycle 1679 declared class `infrastructure-systemic` with the remedy written to `eval-append.crossartifact-invariant-stack.md`; the orchestrator re-audited and rebuilt. The retry envelope should route such a failure to the phase whose sandbox owns the named path (`.evolve/evals` → scout/tdd) with the remedy attached. With F12's two gates the eval case cannot recur; the routing stays open for the general class.

### F14 — an unsubstituted template slot passed the build handoff floor (P2, fixed — this change)

Cycle 1679 round 5's `build-report.md:203` read "`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER" under a heading promising every number was executed this round; the deterministic floor (`DefaultBuildFloorChecks`) had no check for the persona scaffold's slot tokens, so the audit spent a round naming it (M3 verification-gap). Fix: `core.PlaceholderTokenFailures` refuses any `UPPER_SNAKE_PLACEHOLDER` token in the report, one failure per token per line naming the report line, wired into the default floor engine (`build_placeholder_check.go`; wiring proof `TestDefaultBuildFloorChecks_IncludesThePlaceholderTokenCheck`).

### F15 — four repo tests red under the audit sandbox for touching the live tree (P2, fixed — this change)

The audit profile's `deny_subpaths` covers `.evolve/profiles` and `docs/private`; two tests planted decoy profiles INTO the live `.evolve/profiles` (`phasecoherence/TestTrackedProfiles_RealTreePlantedDecoyNotBound`, `profiles/TestRealTreeProfiles_ExcludesUntrackedDecoy`) and two repo-wide walkers descended into `docs/private` (`acs/regression/flagreaders`, `acs/regression/noorphan`). Ledger `d890b99a` (cycle 1676) recorded the class as an instrument fault and DEFERRED it; it recurred in 1679 with a new pair, and each recurrence costs the audit a classification. Fix at the class: the decoy tests mirror the real tracked profiles into a temp git repo and plant the decoy there (`mirrorTrackedProfiles`; the profiles funnel is root-parameterised as `treeProfiles(t, root)` with `RealTreeProfiles` delegating), and the two walkers skip a permission-denied directory visibly (`[flagreaders]`/`[noorphan] NOTE: … denied by the sandbox — skipped`) instead of failing the gate. No sandbox exemption was added: a test that mutates tracked repo config is the defect, not the deny.

### F16 — a recovery rebuild after a WARN audit carried none of the audit's findings (P2, fixed — this change)

1679's audit round 4 PASSED with WARN naming three MEDIUM defects; WARN ships, the ship hit `GIT_FLEET_REBASE_NEEDED`, and the recovery rebuilt via build. That build's brief (`build-prompt.txt`, 00:33) had no audit section — the repair brief seeds only behind a rejection grant (`cs.AuditRepairActive`, `seedAuditRepairContext`) — while the round-2 brief (a rejection) carried `## Audit Repair`. Round 5 then found the same defects "standing" (M1 "second consecutive round", M2 "unaccounted"). Fix: when a tdd/build re-entry follows a ship-error recovery and no repair grant is active, the last audit's actionable findings ride the brief under `CtxKeyStandingAuditFindings`, rendered by build and tdd as `## Standing Audit Findings` (fenced as data); a rejection grant still outranks it. The recovery marker is PERSISTED cycle state (`CycleState.ShipRecoveryCode`, set by the recovery route, cleared by the ship latch) so the live loop and crash-resume seed identically — the in-process `ship_error_code` snapshot (now `core.CtxKeyShipErrorCode`) does not survive a resume (architecture review). Note the builder's disposition duty is inherited ids only (persona line 83, auditor persona line 162) — round 5's M2 asked for the cycle's own ledger too; the standing-findings section is how that duty now reaches the builder without widening the deterministic ledger gate.

### F17 — the boundary binary refresh was blocked by a sealed lane's still-fresh lease (P2, fixed — this change)

`LOOP_BOUNDARY_REFRESH_SKIPPED step=lane_active` fired at both wave boundaries on 2026-09-15 with no lane running: `loopchain.FleetLaneActive` counted every `gc.Discover` Live dir whose lease was not the loop's own, and Live is heartbeat freshness (10 min TTL) — 1679 sealed at 17:03Z with a 17:02Z heartbeat, so its lease read as an active sibling for a TTL and the plane kept a binary behind HEAD (1679 changed `go/`). Fix: liveness is the owner, not the timestamp — `FleetLaneActive` consults `runlease.OwnerLive` with the ONE pid probe `runlease.PIDAlive` (the swarm reaper's `ExecPidAlive` now delegates to it); a fresh lease whose owner pid has exited is a sealed lane, an ownerless lease stays a sibling.

### F18 — a shipped inbox item was re-pinned to a lane because its retirement was invisible (P1, fixed — this change)

Cycle 1682 sealed FAIL in two phases with `triage-empty-commitment-claimable-work`: its lane scope pinned exactly `crossartifact-invariant-stack`, which 1679 had shipped (`inbox/consumed/` in commit c8b64e42, `inbox/processed/cycle-1679/c8b64e42-…`). `inboxmover.ResolveDispatchState` looked for retired items in a FLAT `processed/` (the promoter nests by cycle — `lifecycle.promoteDestPath`) and had no `consumed/` state at all, so every shipped item resolved `unknown`; the plan-time prune, widen's `triagecap.PruneConsumed` and the launch freshness probe all treat unknown as "keep" and the lane burned a scout + triage to discover the work was done. The resolver's own test modelled the flat layout (the fixture-vs-production drift class again). Fix: `StateConsumed`; ONE `retirementStates` list (shared with `scopeRetiredAt`, which carried its own literals) scanned flat AND by `cycle-<N>` through one `inboxbatch.CycleDirs`, newest cycle first (the nested trees grow one dir per cycle for good — a compaction/archive of `processed/cycle-*` is a follow-up); the promoter now spells the nested dir through `inboxbatch.CycleDir` so writer and readers share the layout; both prunes drop consumed; the loopwave fixture writes the promoter's real layout. Follow-up: the plan-time gate could route `kind: pipeline-repair` items console-manual before a scout runs (1683 spent scout + triage to have the breaker refuse a protected-surface item) — open.

### F19 — an audit class outside the vocabulary silently forfeited the direct repair grant, and the retro-routed retry rebuilt blind (P1, fixed — this change)

Cycle 1684's audit FAILed on one red predicate (`acs/regression/cycle1515` `TestC1515_006`, contradicted by the commissioned change) and declared `failure.class: superseded-predicate-contradiction` — a class no policy category knows. The deliverables gate verified the report (it checked only that a block with a class existed), `computeRetryEnvelope` declined on "audit declared an unrecognised class" with no log line and no signal, the cycle ran a full retrospective (deep tier, ~10 min), the retro adjudicated a retry, and the tdd/build re-entry carried none of the audit's findings — the round-2 build brief had zero mentions of the predicate; only a generic `audit.failure_class` label reached tdd. Three fixes at the class: (a) the deliverables gate validates the audit's `failure.class` against `failurelog`'s vocabulary (`CodeFailureClassUnknown`, correction names the offending class and the vocabulary; `contractArtifactDetermined` treats it as sentinel-rewritable) and the audit contract block renders the vocabulary (`failurelog.VocabularyList`, the ONE spelling); (b) `decideAfterAuditFail` emits the decision as a coded signal on both branches — `ORCHESTRATOR_AUDIT_REPAIR_DECLINED` (WARN, the envelope's reason and the declared class) / `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` (INFO, re-entry phase and attempt); (c) a tdd/build re-entry straight after a retrospective that followed an audit-fail decline carries the last audit's actionable findings as `## Standing Audit Findings`; the decline is PERSISTED (`CycleState.AuditDeclineReason`, set by `consumeAuditRepairGrant` on the decline branch, cleared by a grant or the ship latch) so the predicate is the decision itself, not phase order (a retro reached from a dispatch error is not re-audited work), and `core.StandingFindingsIntro` names the route and the envelope's reason as the one intro sentence for both prompts. Review round (architecture): the vocabulary the prompt advertises (failurelog, 13 classes) was not the vocabulary the retry table is keyed by (policy, 7 categories) — `policyCategoryFor` is now the ONE join, and the envelope's decline reasons say which happened (outside the vocabulary / no retry policy row / system-level / budget); the prompt exemplar draws its class from the vocabulary (`exemplarClass`: code-audit-fail for the audit, code-build-fail otherwise) instead of synthesizing `code-<phase>-fail`. The class check stays scoped to the audit's unconditional block (its class feeds the envelope on FAIL and the failure record on FAIL and WARN); the PhaseIO self-report path's class feeds no decision today.

### F20 — the loop ignored SIGINT and SIGTERM while its pre-wave probes ran (P1, fixed — this change)

At the wave-4 boundary (2026-09-15 04:15) the loop was sent SIGINT twice and SIGTERM and sat for over a minute in `[loop] usage-probe: checking [agy claude codex ollama] for quota caps before dispatch`; it needed SIGKILL. `runUsageProbe` handed the prober `context.Background()`, the prober's `WaitGroup` waited on a bridge session that does not return on cancel, and `defaultLiveProbe` (the CLI-health canary that runs just before it) derived its 4-minute bound from `context.Background()` too — so the loop's `signal.NotifyContext` never reached either probe. The same window then printed `[loop] wave 2: 0/2 lanes ok`: `prepareIteration` checks the interrupt only at its top, so an interrupt that lands mid-probe still dispatched a wave that was cancelled at spawn. Fix: both probes take the loop's context (`runUsageProbe(ctx, …)`, `defaultLiveProbe(ctx, …)`; the campaign hook captures the campaign's); `usageprobe.Prober.Run` returns at once on an already-cancelled context and, mid-probe, waits on the WaitGroup only until the context is done — abandoning the in-flight probes with one line naming the count — and `probeOne` never benches on a pane read after the interrupt; `prepareIteration` re-checks the interrupt after the probes and stops with the same "received interrupt" disposition instead of dispatching. Review round (architecture, two CRITICALs): (1) once the canary's probe derived from the loop's context, an interrupt made the smoke test return "not a wall" and the canary's default arm CLEARED the expired bench — a cancelled probe is not evidence, so `runCLIHealthCanary(ctx, …)` touches no bench once the context is done (before the loop and after each probe); (2) the campaign runner's `BeforeWave func()` could not carry the re-check, so the two runners' pre-wave protocols diverged — `runPreWaveProbes(ctx, …) error` is now the ONE protocol (canary, then usage probe, returning the context's error), `campaign.RunOptions.BeforeWave` is `func(context.Context) error` and `RunWaves` aborts before dispatching the wave, and the loop maps the error to `interruptReturn`, which now shares the one interrupt disposition (`signalStop`) with `emitSignalStop`. The interrupt also terminates where the probe actually sat: the bridge's `captureControl` settle loop checks the context each tick, so a probe returns instead of being merely abandoned; the abandonment line counts probes still running (launched minus completed); `probeOne` re-checks the context before `BenchWall`. Documented gap: a `usage-probe` tmux session driven by an abandoned probe is reaped by the tmux session GC, not by the interrupt. Note: an interrupted campaign exits 1 (its executor returns the context error) while an interrupted loop exits 130 — an asymmetry that predates this change. Tests: `usageprobe/cancel_test.go` (a probe that ignores cancel; a pre-cancelled context launches nothing; the count names only probes still running), `cmd/evolve/cli_health_canary_cancel_test.go` (a cancelled context touches no bench), `campaign/executor_test.go` (a pre-wave error aborts before the wave).

### Review follow-ups from F14–F18 (architecture review 2026-09-15, non-blocking)

- A resumed tdd/build re-entry seeds `ship_error_code` alone, so `assembleErrorContext` (phaseio shadow) sees a code-only envelope where the live path carries code/class/stage/debug — persist class/stage beside `ShipRecoveryCode`, or give the prompt-facing code its own key.
- `ship_error_class` / `ship_error_stage` / `ship_error_debug` are still bare literals on both sides; three consts beside `core.CtxKeyShipErrorCode`.
- `continuation_retire.go`'s rationale prose still enumerates the retirement set that `retirementStates` now owns.
- The promoter spells the retirement PARENT dirs (`"processed"`, `"rejected"`) as literals while readers use `State*`; `lifecycle` cannot import `inboxmover`, so the shared home is `inboxbatch`.
- `processed/cycle-*` and `rejected/cycle-*` grow one dir per cycle; a compaction/archive is the follow-up (never a cap on the scan).
- The PhaseIO self-report path's class feeds no decision and is not validated (F19 scoped the gate to the audit; its exemplar now draws from the vocabulary).
- Signal: the audit-fail decision reuses `(cycle, phase=audit, attempt)` coordinates with the audit's own phase.outcome row; `Origin`/`Code` disambiguate — any future aggregation keyed on the tuple must read them.

## 4. Verdict on the design

| Claim (memo §4) | Evidence from the wave |
|---|---|
| root cause from a few signal lines | F1: four lines + one ledger grep; F2: two lines + the per-phase `submit_verify` ledger; F3/F4: one line each |
| module-tagged console with registered codes | every WARN in §2 carried `[module] kind SEVERITY CODE`; drift 0 |
| the closed vocabularies survive the train | 192 → 194 registered codes with F1's two inbox codes (`evolve signals codes check` green); F2 adds evidence to an existing ledger payload, no new code |
| efficiency-neutral at the leaf | no new cost measured; the two fixes remove the wave's two largest token sinks |

Where the design was **not yet enough**: the triage gate's refusal was structured on the stream but not on the classifier's input (fixed by F1's `Diagnostic.Code`), and the driver's timing evidence lived only on stderr, which survives the failure path only (fixed by F2's ledger payload). Both are instances of the same rule the memo already stated: *every producer that stamps a code retires a regex; every diagnostic that matters must reach a durable record on the success path too.*

## 5. Open items for the next wave

1. ~~Re-launch on a plane carrying #606 and the wedge fix; count consecutive ships from that wave (goal: 5).~~ Done: wave 2 ran 16:11–21:05 on plane 83a019aa and shipped 0/2 (§6); wave 3 launched 21:06 on plane e2819460 (#613–#617). The count toward 5 restarts there.
2. ~~`AUDIT_CIPARITY_GATE_STEP_FAILED` inside lane worktrees (F4) — needs a second occurrence to classify.~~ Classified on the second wave (every audit): the apicover coverage run inherited the lane env — F9, fixed #617.
6. **Done the same day (F6):** the acs/cycle8 `--simulate` walk that littered every checkout it ran in (dossier commits, salvage snapshots, cycle worktrees/branches, live CLI probes) — the simulate root never mutates git now; record [2026-09-14-simulate-runs-against-the-checkout](../incidents/2026-09-14-simulate-runs-against-the-checkout.md). The two console-first P1 items lanes 1673/1674 burned on were routed `console-manual`; lanes must not draw pipeline-integrity work (operating-policy §1).
3. ~~`ADVISOR_RESPONSE_UNPARSEABLE` (F4)~~ — **root-caused and fixed 2026-09-14:** not a non-JSON reply — the proposal decision read the REPL scrollback while its prompt asked the model to write `routing-proposal.json` (which it did); the decision now uses the artifact contract; record [2026-09-14-router-proposal-read-the-scrollback](../incidents/2026-09-14-router-proposal-read-the-scrollback.md).
4. The re-send WARN signal (F2 follow-up) and the chip `+N lines` positive signal if the stability wait proves insufficient.
5. The memo's remaining recommendations: the Center-less operator roots unit before unit 05, unit 05 as a series, the four deferred "what happened" signals.
7. **Done the same day (F7):** the lane ship that redded main from 4db205a8 until #590 — the ship gate ran in the project root (a tree without the lane's changes), seeded from an index the ship had not yet populated, and never looked at importers. The gate now runs in the lane worktree against its base, seeds from the working tree, and runs the reverse-dependency closure (test imports included) before the push; record [2026-09-14-lane-ship-gate-package-scoped-tests](../incidents/2026-09-14-lane-ship-gate-package-scoped-tests.md).
8. **Operator decision:** codex's `deep`/`top` tiers are pinned to `gpt-5.6-sol`, which this account rejects (F7); the `balanced`/`fast` pins are accepted. Until re-pinned, every codex deep dispatch fails over to Claude — in seconds since #616, not after the artifact window.
9. acs baseline drift on main, not from any wave change: `acs/cycle1253` (`TestC1253_003_NewExportCovered` — the `ImporterClosure` coverage line after #612) and `acs/cycle1632` (`TestC1632_008` — a tokenopt-handoff inbox item no longer present) fail on a clean `origin/main`; `acs/cycle764` is a two-floors-at-once contention flake. The console floor baseline now carries the first two; both need a re-anchor.
10. `TestChannel_EndToEnd` (bidirectional channel) is timing-based and redded PR #614 once on the Ubuntu Go 1.23 race runner; queued as inbox item `2026-09-14T10-20-00Z-channel-e2e-timing-flake`.
11. ~~A FAIL retrospective dispatched on a walled CLI costs a full artifact window (both wave-2 retros: 30 min each on codex's dead deep model).~~ Root-caused as F10: the retro had no fallback walk at all. Fixed — ADR-0104 (#619).

## 6. Wave 2 — 2026-09-14 16:11–21:05, plane 83a019aa (#606, #609, #611, #612), width 3, 2 lanes dispatched

**Outcome: 0 ships of 2 lanes.** Both lanes built green and audited (WARN) and then lost every ship attempt to one pipeline defect that was latent until #612 — the ship gate's `go test` ran in the lane's IPC environment (F6). The wave also met two walls the bridge did not recognise (F7 codex, F8 Claude) and a second site of the F6 leak in the audit (F9). All four are fixed on main (#615, #616, #617); the loop halted itself at the boundary (`LOOP_HALT plane_diverged_halt`, the plane 2 dossier commits ahead and 4 fixes behind), the plane was reconciled (e2819460) and wave 3 launched at 21:06.

| Cycle | Item | Phases (verdict, minutes) | Seal |
|---|---|---|---|
| 1676 | crossartifact-invariant-stack (0.85) | scout PASS 3.3 · triage PASS 1.8 · tdd PASS 17.0 · build PASS 64.5 (25 of it idle on codex's rejected model, then Claude) · audit WARN 9.6 · ship FAIL · audit PASS 13.3 · ship FAIL · audit PASS 3.4 · ship FAIL · retro FAIL 30.6 (codex wall) | FAIL, 11 phases |
| 1677 | ledger-verify-seal-anchor (0.7) | scout PASS 2.2 · triage PASS 2.3 · tdd PASS 9.4 · build PASS 56.6 (same stall) · audit WARN 9.2 · ship FAIL · audit WARN 2.8 · ship FAIL · audit WARN 25.2 (session wall, two 40-min timeouts across the recovery audits) · ship FAIL · retro FAIL 30.5 | FAIL, 11 phases |

Every `ship FAIL` is `SHIP_REPO_CONTRACT_GATE`: the importer backstop (1677) or the added-test backstop (1676) RED on twenty-odd env-sensitive tests in `cmd/evolve`, `guards`, `ship`, `core` that pass in the same worktree under `env -i PATH HOME`.

**Signals since launch (per-cycle `signals.ndjson`, module × severity):** ledger INFO 278 · bridge WARN 44 · orchestrator WARN 14 / INFO 14 · gate.contract INFO 14 / WARN 4 · loop INFO 11 / INCIDENT 4 · liveness INFO 9 · ship WARN 6 · runner WARN 4 · audit WARN 4 · advisor WARN 4.

**Top coded lines:** `BRIDGE_EXIT_ARTIFACT_TIMEOUT` 8 (codex 400 ×2, Claude session wall ×4, retros ×2) · `SHIP_REPO_CONTRACT_GATE` 6 · `ORCHESTRATOR_PHASE_ABORTED` 6 · `RUNNER_TEARDOWN_FAIL` 4 · `ADVISOR_RESPONSE_UNPARSEABLE` 4 (F4, fixed #614 mid-wave — the plane never carried it) · `GATE_CONTRACT_REJECTED` 4 (audits written without a verdict/section while walled) · `AUDIT_CIPARITY_GATE_STEP_FAILED` 3 (F9) · `LOOP_HALT` 4.

**What the stream bought.** Each of F6–F9 was one readable line to the console: `SHIP_REPO_CONTRACT_GATE … failing: TestRunCycleReset_LeaseFencing…` named the class of test (environment-sensitive) before any log was opened; `POSSIBLE EXHAUSTION-REGEX DRIFT` in the bridge stderr named F8's cause outright; `AUDIT_CIPARITY_GATE_STEP_FAILED gate=apicover_enforce step=cover_run` named F9's step. The gaps were in the reactions, not the signals: a 4xx model rejection had no rule, a wall with a non-breaking space in it had no match, and a `go test` spawned by a gate had no scrubbed environment.

**Cost.** Two builds of ~60 min (each with a 25-min idle stall on the dead codex model), six ship attempts, six audits (three of them 40-min session-wall timeouts), two 30-min retros on the dead model; the Claude session limit was reached 19:03–19:50 during the recovery audits. No tokens were spent while walled; the wall-clock was.

**Carried into wave 3:** #613 (dashboard phase plan), #614 (proposal artifact), #615 (ship-gate env scrub + tee race), #616 (codex model rule, Claude session-wall regex), #617 (apicover env scrub). Open: items 8–11 above.

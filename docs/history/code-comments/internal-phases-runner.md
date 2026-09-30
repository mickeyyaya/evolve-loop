# Comment history: `internal/phases/runner`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/runner/agent_doc_missing_wiring_test.go:3` — above `import (`

```text
// agent_doc_missing_wiring_test.go — the runner's REAL load path must wrap a
// nonexistent persona in core.ErrAgentDocMissing, or the optional-skip
// admission one layer up never fires (the week's tenth candidate for a correct
// component nothing calls). Driven through Run with a loader whose FS simply
// lacks the doc — the exact cycle-1551 shape.
```

### `go/internal/phases/runner/basecontext_test.go:3` — above `import (`

```text
// RED-phase contract for cycle-249 task `runner-base-cycle-context`:
// BaseCycleContext(body, req) is the single source for the "## Cycle
// Context" core block that 10 phase files currently copy-paste. The
// helper must emit the four mandatory fields BYTE-IDENTICALLY to the
// duplicated block so callers can swap to it with zero prompt drift.
//
// These tests fail at baseline because BaseCycleContext does not exist
// yet (compile error: undefined) — that is the correct RED signal.
```

### `go/internal/phases/runner/budget_scale_thread_test.go:3` — above `import (`

```text
// budget_scale_thread_test.go — ADR-0076 slice A: the runner must thread
// PhaseRequest.BudgetScale onto the BridgeRequest verbatim, or the dispatch's
// difficulty multiplier dies between core and the engine (the I2 dead-link
// class this campaign exists to kill).
```

### `go/internal/phases/runner/cli_chain_apicover_salvage_test.go:3` — above `import (`

```text
// cli_chain_apicover_salvage_test.go — apicover Phase-5 naming coverage for the
// salvaged cycle-943 export (false-RED salvage, post-v22.4.2): NAMES + EXERCISES
// FormatSkillOverlayLog. Behavioral (Rule 9): pins the exact observability line
// shape operators/graders grep for, including the empty-set rendering that
// distinguishes "no overlay resolved" from "the line never ran".
```

### `go/internal/phases/runner/cli_health.go:3` — above `import (`

```text
// cli_health.go — the runner's two hooks into the CLI-health bench store
// (cycle-283 forensics): consult the bench when building the dispatch chain,
// and write a bench when a dispatch dies on a classified wall. Both are
// disabled by EVOLVE_CLI_HEALTH=0 and bypassed entirely under a policy pin.
```

### `go/internal/phases/runner/contract_verifier_test.go:16` — above `func TestRun_ContractVerifierSalvagesBeforeClassification(t *testing.T) {`

```text
// Cycle 1685 (2026-09-15): the engine verified with the plain verifier and
// classified the unrepaired bytes ("no parseable verdict → FAIL"); the gate
// then salvaged, persisted and approved the repaired file, and a red_count=0
// cycle sealed FAIL with no failure class. With the gate's own Reviewer as the
// engine's verifier there is ONE verifier: the salvage happens before
// classification, the classifier sees the repaired sentinel, the result is
// OK (no ship-guard downgrade), and the file on disk is what was judged.
```

### `go/internal/phases/runner/dispatch.go:43` — above `fence := takeWorktreeFence(ctx, phase, req)`

```text
// WS-G1: dispatch through the chain via llmroute.Dispatch — the SAME
// chain-walk implementation the advisor uses (cycle-435,
// [[never_duplicate_centralize_via_design_patterns]]), rather than a
// hand-rolled copy of it. Each attempt: build BridgeRequest for the
// candidate CLI, Launch, normalize events. On a trigger exit (default
// {80, 81, 124, 127} per cli_chain.go:defaultFallbackOnExit —
// REPL-boot-timeout / artifact-timeout / coreutils-timeout /
// missing-binary) Dispatch advances to the next candidate. Any other exit
// (or success) stops the walk — a legitimate FAIL verdict from a model
// never silently routes to a different CLI. Final attempt's (bres,
// bridgeErr) is what the rest of the function consumes; events file
// reflects the final CLI's stdout so cycleclassify sees what actually
// happened last.
// Worktree fence (ADR-0097): a phase without write permission hands
// downstream the exact tree it was given. Snapshot before the first
// attempt, restore after the last — the classify hooks below (the audit's
// explanation binding among them) must judge the builder's tree, not the
// auditor's probes (cycles 1603-1605).
```

### `go/internal/phases/runner/dispatch.go:91` — above `overlayDispatch.Signals = req.Signals`

```text
// ADR-0099 slice 3: the objective signals the `when` selector reads are
// core's projection (PhaseRequest.Signals, one digest per dispatch); the
// runner copies, never re-reads the workspace.
```

### `go/internal/phases/runner/dispatch.go:131` — above `if bridgeErr != nil && bres.ExitCode == 85 {`

```text
// CLI-health bench: an exit-85 with a fresh benchable escalation
// report (rate_limit class) is remembered ACROSS dispatches — run on
// every candidate including the last, so the wall is recorded even
// when no fallback remains (cycle-283). Staleness is judged against
// the RUN start: the guard exists to exclude cross-PHASE leftovers in
// the shared workspace, not earlier attempts of this same run.
```

### `go/internal/phases/runner/lanescope.go:9` — above `func LaneScope(req core.PhaseRequest) string {`

```text
// LaneScope resolves the fleet lane scope pinned for this run (ADR-0049 E;
// cycle-766 lane-scope.json → Context["fleet_scope"]): the typed envelope at
// enforce, the legacy Context map below it (byte-identical — Active() is
// false unless enforce). The ids come from the advisor's (LLM-authored)
// backlog, so control chars are collapsed before the value reaches a prompt —
// a newline in an id would otherwise forge a new context bullet (prompt
// injection via the data channel). Empty string ⇒ not a fleet lane; phases
// must render nothing (sequential cycles stay byte-identical).
```

### `go/internal/phases/runner/lanescope_test.go:3` — above `import (`

```text
// Cycle-776 — direct contract for the shared LaneScope resolver (consumed by
// scout/build/tdd/triage ComposePrompt; phase-level rendering is pinned in
// each phase's lanescope_prompt_test.go).
```

### `go/internal/phases/runner/model_routing_overlay_test.go:105` — above `func TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack(t *testing.T) {`

```text
// TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack (mr4-projection
// AC5, I3): the advisor proposes codex-tmux as the overlay CLI, but codex-tmux
// is ACTIVELY BENCHED (2-strike boot-timeout bench, same mechanism as
// cycle-426 driver bench). Because the overlay is SOFT (pin==nil), the
// dispatch chain must still fall back to the profile's original primary
// (claude-tmux) — a benched advisor choice must never collapse the chain to
// a single, unavailable candidate the way an absolute policy.Pin would.
```

### `go/internal/phases/runner/persona_available_test.go:10` — above `func TestBaseRunner_PersonaAvailable(t *testing.T) {`

```text
// TestBaseRunner_PersonaAvailable — 2026-09-09 token-waste root cause #2: an
// optional phase whose persona doc does not exist must never enter a
// selectable plan. The runner already knows its persona name and loader; it
// answers the availability question before any dispatch, with the same
// sentinel the dispatch path raises (core.ErrAgentDocMissing) so the planner
// and the skip classifier agree on the cause.
```

### `go/internal/phases/runner/prompt_echo_wiring_c672_test.go:3` — above `import (`

```text
// RED wiring test for cycle-672 top_n task `echo-veto-wiring-completion`,
// caller side of AC1: BaseRunner composes the phase prompt (runner.go ~307)
// and invokes the events producer per dispatch attempt (~551), but the default
// producer builds phasestream.ProduceConfig WITHOUT the prompt — so the live
// classifier has no echo context and an agent quoting its own prompt text is
// classified infra_failure (cycle-656 retro D3, third recurrence of the
// cycle-641 lesson).
//
// This test drives the REAL default events producer (no EventsProducer
// override) through Run() with a bridge fake that writes the phase logs, then
// asserts on the emitted <phase>-events.ndjson. RED today behaviorally: the
// echoed-line case emits infra_failure because the composed prompt never
// reaches phasestream.Produce. DO NOT MODIFY — Builder threads the composed
// prompt from Run into the producer (and ProduceConfig.InjectedPrompt) to make
// it GREEN; the genuine-frame case must STAY green (anti-over-suppression).
```

### `go/internal/phases/runner/runner.go:141` — above `EventsProducer func(workspace, phase, cli string, cycle int, prompt string) error`

```text
// EventsProducer is the seam for the post-phase <phase>-events.ndjson
// writer (ADR-0020). When nil, defaults to phasestream.Produce. Unlike
// StdoutFilter this is load-bearing: cyclecost + cycleclassify read the
// events stream, so it is always-on (no disable flag). prompt is the
// composed phase prompt, threaded to the Classifier's echo-veto
// (ProduceConfig.InjectedPrompt, cycle-672) so agent-quoted prompt text
// never classifies infra_failure.
```

### `go/internal/phases/runner/runner.go:149` — above `Optional bool`

```text
// Optional marks this phase as non-essential to the cycle. When true, a
// bridge ErrArtifactTimeout degrades to a WARN that lets the cycle
// advance (the state machine's successor is verdict-unconditional for
// optional phases like build-planner) instead of aborting. Set by the
// owning phase (e.g. buildplanner.New). Default false = hard-fail, the
// historical behavior for mandatory phases. See Workstream D / cycle-120.
```

### `go/internal/phases/runner/runner.go:163` — above `ContractVerifier func() ContractVerifier`

```text
// ContractVerifier is the deliverables gate's own verifier offered to the
// verdict engine so there is ONE verifier: the bytes the engine classifies
// are the bytes the gate will approve (a sole recoverable bad_verdict is
// salvaged, persisted and reported before classification — cycle 1685
// sealed FAIL on the unrepaired bytes while the gate approved the
// repaired file). The composition root injects the same Reviewer it
// appends to the orchestrator's reviewers; it is an accessor because the
// Reviewer is built after the runners (it needs the merged phase
// catalog) — the Signals precedent. nil, or an accessor returning nil
// (tests, gate off), falls back to the catalog-aware verify. VerifyFn
// (tests) outranks it.
```

### `go/internal/phases/runner/runner.go:175` — above `SleepFn func(time.Duration)`

```text
// SleepFn is the seam for the delay between the verdict engine's bounded
// settle-retry attempts (verdict.Engine.settle — see its doc for the
// cycles 824/825 rationale). When nil, defaults to settleSleep (time.Sleep).
// Per-instance so t.Parallel() tests can inject a no-op for determinism,
// mirroring NowFn.
```

### `go/internal/phases/runner/runner.go:181` — above `PhaseIO config.Stage`

```text
// PhaseIO is the EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.10). When VerifyFn
// is nil, it is threaded into the catalog-aware reconcile default so the
// reconcile-on-timeout rung honors the same stage-gated failure-context
// requirement as the host gate. Zero value (StageOff) keeps every existing
// Options{} literal byte-identical — only build/scout/triage set it (the
// phases with a RequireFailureContextPhaseIO contract).
```

### `go/internal/phases/runner/runner.go:208` — above `Diag log.Console`

```text
// Diag is the injectable diagnostics logger (T3, cycle-463): the MR4c
// advisor-overlay observability lines route through it so a test can
// capture them instead of the global log.Diag() stderr sink. Zero value
// (both sinks nil) defaults to log.Diag() — production behavior is
// unchanged.
```

### `go/internal/phases/runner/runner.go:214` — above `Signals func() *signalcenter.Center`

```text
// Signals is the Signal Center accessor the verdict engine reports through
// (ADR-0103 unit 11). Nil ⇒ New adopts the Center the injected Bridge
// carries when it exposes one (the production Adapter), else the Null
// Object. Explicit so tests and foreign roots can inject one.
```

### `go/internal/phases/runner/runner.go:242` — above `contractVerifier func() ContractVerifier`

```text
// judge is unit 11's (ADR-0103): the verdict engine, built ONCE by New over
// the resolved probe, clock, stdout filter, optional flag and Center
// accessor — which live in the engine only (wiredVerdictEngine).
```

### `go/internal/phases/runner/runner.go:316` — above `func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {`

```text
// Run implements core.PhaseRunner. The template:
//
//  1. validate deps (bridge, prompts), load the agent prompt body and compose
//     the final prompt via the hooks; snapshot the artifact pre-dispatch
//  2. resolve the dispatch plan (policy pin / profile / advisor overlay, the
//     CLI chain, the tier)
//  3. dispatch through the bridge across the fallback chain, inside the
//     worktree fence
//  4. judge the outcome through the verdict engine (ADR-0103 unit 11): the
//     bounded settle ladder, the teardown reconcile arms, the verdict-source
//     rule (the contracted file, never the pane), Classify via hook, the ship
//     guard, the response
//
// Bridge errors and missing-prompts errors short-circuit to a FAIL
// response with the error attached as a diagnostic.
```

### `go/internal/phases/runner/runner_amplify_test.go:87` — above `func TestRun_NonTimeout_ContractedDeliverableFailsVerification_ShipVerdictDowngraded(t *testing.T) {`

```text
// TestRun_NonTimeout_ContractedDeliverableFailsVerification_ShipVerdictDowngraded guards
// the anti-gaming half of the verdict-source rule (ADR-0072). A CONTRACTED deliverable
// that FAILS its well-formedness/anti-gaming contract (here: a missing challenge token)
// carries a PASS sentinel in the file, and the pane also shows PASS. Classify would
// extract PASS — the exact laundering vector the earlier sentinel-authoritative attempt
// was blocked for. The ship-guard must downgrade the ship-eligible verdict to a coherent
// FAIL, and the contract Codes must surface as diagnostics. (Previously this fell back to
// the pane, leaving a fabricated verdict reachable.)
```

### `go/internal/phases/runner/runner_challengetoken_test.go:3` — above `import (`

```text
// runner_challengetoken_test.go — cycle-269 incident, prompt half: the
// bash→Go migration dropped the challenge-token prompt injection entirely
// (builder.json's challenge_token_required had NO Go consumer;
// resolved-prompt.txt carried zero mentions), so whether a builder echoed the
// token depended on it spontaneously reading scout-report line 2 — the claude
// fallback didn't, and a perfect build FAILed at audit. The runner now
// appends a deterministic token block (the TurnBudgetHint append precedent)
// for phases whose CONTRACT requires the echo, sourced from the minted
// <workspace>/challenge-token.txt.
```

### `go/internal/phases/runner/runner_cleanexit_test.go:20` — above `func TestRun_NonTimeout_CleanExitIdle_DeliverableSettlesOnRetry_PrefersFile(t *testing.T) {`

```text
// TestRun_NonTimeout_CleanExitIdle_DeliverableSettlesOnRetry_PrefersFile pins the
// cycle-603/899 class (≥10 identical-goal_hash false-FAILs, 877→899). An agent
// EXITS CLEANLY (exit 0 — no bridge error, no timeout, no ctx-cancel) after
// writing a valid PASS deliverable, then IDLES ("Contemplating…") without a clean
// completion signal. The presence probe (verifyFn) races that transient idle
// state and misreports the deliverable not-OK on the first check(s). The runner
// must settle-retry the on-disk deliverable — exactly as the reconcile path
// already does for the timeout race — and record PASS from the FILE, never
// synthesize FAIL from multi-phase-contaminated scrollback (which echoes other
// phases' Deliverable-Contract example sentinels).
//
// Before the fix the clean-exit path did a SINGLE-SHOT verify (runner.go:715), so
// a racy first check flipped a genuine PASS to a scrollback-synthesized FAIL.
```

### `go/internal/phases/runner/runner_cleanexit_test.go:35` — above `noisyStdout := "Deliverable Contract example (PASS):\n" +`

```text
// Raw tmux scrollback contaminated with OTHER phases' prompt-echoed example
// sentinels — exactly what cycle-899 Classified into a false FAIL.
```

### `go/internal/phases/runner/runner_cleanexit_test.go:149`

```text
// ── Verdict-authority regression matrix (the "deliverable is the source of truth"
// invariant, unified across every completion path) ─────────────────────────────
//
// The recorded verdict must come from the AGENT'S ON-DISK DELIVERABLE whenever it
// enforce-verifies, and NEVER from multi-phase-contaminated bridge scrollback. Each
// completion path re-verifies with the SAME bounded settle-retry
// (verifyReconcileDeliverable). This suite pins the full grid:
//
//   completion path        | valid PASS file | settles on retry | genuine FAIL file | never-settles / absent
//   -----------------------|-----------------|------------------|-------------------|-----------------------
//   clean-exit (exit 0)    | PrefersFile...  | CleanExitIdle... | ...GenuineFAIL... | ...NeverSettles...     ← THIS FILE (was the gap: cycle-603/899)
//   artifact-timeout (81)  | Timeout_Well... | Timeout_Settles..| Timeout_Sentinel- | Timeout_NeverSettles..
//   transient (80/85/86)   | Transient_Well..| Transient_Settle.| Transient_Sentinel| Transient_NotWell...
//   ctx-cancel (-1)        | (→ transient via IsInfraTeardownError; engine classifies -1+ctx.Err → transient)
//
// Invariants pinned across ALL cells:
//   • PASS-preservation: a genuine on-disk PASS is recorded PASS (never dropped to a scrollback FAIL).
//   • FAIL-preservation: a genuine on-disk FAIL is recorded FAIL (never laundered to a scrollback PASS).
//   • Bounded + fail-open: an absent/never-settling deliverable falls back to stdout after ≤ reconcileSettleRetries+1
//     verifies — the retry can only UPGRADE toward the real deliverable, never invent one, and the loop cannot spin.
```

### `go/internal/phases/runner/runner_clihealth_story_test.go:3` — above `import (`

```text
// Slice-5 full-story replay (deterministic clock): the complete
// detect → remember → react → recover loop the cycle-283 incident lacked.
//
//	t0      dispatch: codex walls (85 + rate_limit report) → benched, claude
//	        carries the phase; benched_until comes from the pane's own
//	        "try again at 6:11 AM" hint
//	t0+5m   dispatch: chain starts at claude — zero codex boots
//	t0+7h   dispatch (bench expired): codex gets its canary shot first; it
//	        walls AGAIN → re-benched with strikes=2
//	then    dispatch: codex demoted again while the strike bench is active
```

### `go/internal/phases/runner/runner_clihealth_test.go:3` — above `import (`

```text
// RED contract for the CLI-health bench hooks (cycle-283 forensics): when a
// candidate exits 85 and the bridge's escalation-report.json classifies a
// benchable pattern (rate_limit), the runner must REMEMBER it — bench the CLI
// FAMILY in .evolve/cli-health.json — and the NEXT dispatch must start at a
// healthy CLI instead of re-burning the benched primary's boot window.
```

### `go/internal/phases/runner/runner_clihealth_test.go:22` — above `const cycle283WallTail = "■ You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), " +`

```text
// cycle283WallTail is the verbatim wall line from the real cycle-283
// escalation report — the fixture the whole feature was built from.
```

### `go/internal/phases/runner/runner_clihealth_test.go:81` — above `func TestRun_Exit85RateLimitBenchesFamily(t *testing.T) {`

```text
// TestRun_Exit85RateLimitBenchesFamily: the cycle-283 replay. codex exits 85
// with a fresh rate_limit escalation report → the runner benches family
// "codex" with reason rate_limit; the fallback completes the phase.
```

### `go/internal/phases/runner/runner_dispatch_amplify_test.go:12` — above `func TestRunnerDispatch_AllDefaultTriggerExitCodesFallBackThroughRun(t *testing.T) {`

```text
// Test Amplification (cycle 435, black-box adversarial pass on top of the
// TDD-authored runner_dispatch_dedup_test.go). Reuses the scriptedBridge /
// writeFallbackProfile / fakeHooks / fakePromptsFS fixtures already present
// in this package (runner_fallback_test.go, runner_test.go) without reading
// runner.go's Run implementation. Covers the same gaps closed at the
// llmroute/core layers -- the full default trigger-code set and chains
// longer than 2 -- but end-to-end through r.Run(), proving the post-dedup
// delegation to llmroute.Dispatch preserves chain behavior for every
// trigger code, not just the one (exit=80) the pre-existing suite scripts.
```

### `go/internal/phases/runner/runner_dispatch_amplify_test.go:22` — above `func TestRunnerDispatch_AllDefaultTriggerExitCodesFallBackThroughRun(t *testing.T) {`

```text
// TestRunnerDispatch_AllDefaultTriggerExitCodesFallBackThroughRun (basic,
// table-driven, end-to-end): runner_fallback_test.go's canonical case only
// exercises exit=80. The cycle-435 goal names the full set [80 81 85 124
// 127] as what the shared llmroute.Dispatch must honor; this proves the
// runner's delegation carries all five through r.Run(), not just the one
// pre-existing example.
```

### `go/internal/phases/runner/runner_dispatch_dedup_test.go:15` — above `func readRunnerSource(t *testing.T) string {`

```text
// TDD RED (cycle 435, task runner-dispatch-dedup / A2, dependsOn
// advisor-cli-fallback-chain / A1): the runner's WS-G1 fallback walk
// (runner.go:485-537, the inline `for i, candidateCLI := range
// plan.Candidates` loop) and the llmroute.Dispatch extracted for the advisor
// are the SAME "advance the CLI chain on a trigger exit" algorithm.
// [[never_duplicate_centralize_via_design_patterns]] is the user's HIGHEST
// rule: once Dispatch exists (go/internal/llmroute/dispatch_test.go), the
// runner's hand-rolled copy must be deleted and replaced with a call to it —
// coexistence of both is the violation this task closes.
//
// AC map (1:1, R9.3 floor-binding — runner-dispatch-dedup is a ## top_n task):
//
//	AC1 runner fallback/clihealth tests green under -race (regression)  → pre-existing GREEN: runner_fallback_test.go, runner_clihealth_test.go, runner_driver_bench_test.go (re-run post-refactor by go/acs/cycle435/predicates_test.go)
//	AC2 inline loop removed                                 (negative)  → TestRunnerDispatch_NoInlineFallbackLoop (RED today)
//	AC3 delegates to llmroute.Dispatch, chain still works    (positive) → TestRunnerDispatch_CallsLlmrouteDispatch (RED today)
//	AC4 non-trigger exit still never reroutes (regression)              → pre-existing GREEN: TestRun_NoFallbackOnNonTriggerExit (runner_fallback_test.go)
//	AC5 exit-85 bench side effect preserved (regression)                → pre-existing GREEN: TestRun_Exit85RateLimitBenchesFamily (runner_clihealth_test.go)
//
// Adversarial diversity (SKILL §6): AC2 is the negative (a refactor that
// keeps BOTH the old loop and a new Dispatch call would pass every OTHER
// AC — this is the one that catches "added the call but never deleted the
// duplicate"). TestRunnerDispatch_CallsLlmrouteDispatch pairs its source
// check with an ACTUAL r.Run() fallback (behavioral), so a dead reference to
// "llmroute.Dispatch" in a comment can't game it — the predicate-quality
// rule bars a source-text-only assertion from being the SOLE evidence.
```

### `go/internal/phases/runner/runner_driver_bench_amplify_test.go:3` — above `import (`

```text
// runner_driver_bench_amplify_test.go — adversarial amplification for cycle-426
// T1 (wire-driver-bench-consumer).  Targets gaps in the Build tests:
//
//  1. Expired driver bench does not demote — time-expiry on the NEW driver-bench
//     path (BootTimeoutPattern) was verified for family benches but not for the
//     driver-bench consumer added in this cycle.
//  2. All-driver-benched least-recently-benched-first ordering — verified for
//     family benches (TestRun_AllFamiliesBenchedNeverStrands) but NOT for the
//     new ApplyDriverBench code path.
```

### `go/internal/phases/runner/runner_driver_bench_amplify_test.go:59` — above `func TestAmplify_C426_AllDriverBenchedLeastRecentlyFirst(t *testing.T) {`

```text
// TestAmplify_C426_AllDriverBenchedLeastRecentlyFirst: when both candidates are
// driver-benched (BootTimeoutPattern), the one benched EARLIER must be tried
// first (least-recently-benched policy).
//
// Verified for family benches in TestRun_AllFamiliesBenchedNeverStrands; this
// test ensures the ApplyDriverBench path added in cycle-426 honours the same
// ordering invariant.  Also confirms bench is advice, never a veto — dispatch
// must not strand when all candidates are driver-benched.
```

### `go/internal/phases/runner/runner_driver_bench_test.go:3` — above `import (`

```text
// runner_driver_bench_test.go — RED contract for the driver-bench consumer
// (cycle-426 T1): applyBenchToPlan must route BootTimeoutPattern entries from
// clihealth.Active() to llmroute.ApplyDriverBench (driver-keyed) so a 2-strike
// codex-tmux is demoted behind claude-tmux in the dispatch chain.
//
// Currently RED: applyBenchToPlan calls only ApplyBench (family-keyed).
// Active() returns the boot entry keyed "codex-tmux", ApplyBench looks for
// Family("codex-tmux")="codex" → key mismatch → no demotion.
// GREEN after fix: entries with Reason==BootTimeoutPattern are routed to
// ApplyDriverBench (driver-keyed), which matches "codex-tmux" and demotes it.
```

### `go/internal/phases/runner/runner_fallback_test.go:89` — above `func TestRun_FallbackOnBootTimeout_PrimaryFailsSecondarySucceeds(t *testing.T) {`

```text
// TestRun_FallbackOnBootTimeout_PrimaryFailsSecondarySucceeds is the
// canonical cycle-121 fix: primary cli returns exit=80 (REPL boot timeout),
// fallback cli succeeds, runner returns PASS without surfacing the primary
// failure as an error.
```

### `go/internal/phases/runner/runner_fallback_test.go:201` — above `func TestRun_FallbackOnArtifactTimeout_DefaultTriggerListIncludes81(t *testing.T) {`

```text
// TestRun_FallbackOnArtifactTimeout_DefaultTriggerListIncludes81 is the
// cycle-122 cross-workstream contract test (Fix 2 of the cycle-122
// remediation). It pins the WS-B↔WS-G integration that the prior
// session shipped without: WS-B introduced ExitArtifactTimeout (81)
// as the bridge's coarse stall-detection signal; WS-G's fallback chain
// triggers on a list of exit codes. The default trigger list MUST
// include 81 so artifact-timeout failures route to the next CLI
// instead of aborting the cycle.
//
// Without this guarantee, cycle-122's codex-tmux tdd-phase hang
// (which the artifact-timeout caught at exit=81) was not retried on
// any other CLI — see docs/incidents/cycle-122-...md for the full
// failure analysis.
//
// Profile sets cli + cli_fallback but DOES NOT set
// cli_fallback_on_exit, so the default trigger list is exercised.
```

### `go/internal/phases/runner/runner_fallback_test.go:259` — above `func TestRun_FallbackOnGNUTimeout_124(t *testing.T) {`

```text
// TestRun_FallbackOnGNUTimeout_124 is the defensive companion to the
// cycle-122 fix: coreutils `timeout(1)` exits 124 when its time limit
// trips. If anything wraps a CLI in `timeout`, that 124 should retry
// on the next CLI rather than abort. Same default-trigger-list contract.
```

### `go/internal/phases/runner/runner_fallback_test.go:298` — above `func TestRun_NoFallback_ByteIdentical(t *testing.T) {`

```text
// TestRun_NoFallback_ByteIdentical pins the opt-out contract: a profile
// without cli_fallback set + no env override behaves exactly like pre-G —
// single launch, single error path. This is the regression guard for the
// 6 cycle-119/120 workstreams that ship with no fallback configured.
```

### `go/internal/phases/runner/runner_fallback_test.go:327` — above `func TestRun_FallbackOnArtifactTimeout_CarriesVerdictCostDuration(t *testing.T) {`

```text
// TestRun_FallbackOnArtifactTimeout_CarriesVerdictCostDuration is the
// cycle-262 link at the runner level (ADR-0044 C1 / Slice 1): primary CLI
// exits 81 (artifact timeout — exactly what codex's mid-phase self-upgrade
// produced), the fallback CLI succeeds, and the runner's response must carry
// the FINAL attempt's verdict + cost + boot, a positive duration, and a nil
// error. Baseline-GREEN pin: the dispatch chain already behaves this way (the
// 262 recording loss was downstream, in the orchestrator's abort paths) —
// this test makes the link regression-proof while C1 reshapes the recording.
```

### `go/internal/phases/runner/runner_optional_test.go:20` — above `func TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn(t *testing.T) {`

```text
// TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn is the cycle-120 fix
// (Workstream D): an OPTIONAL phase (build-planner) whose artifact never
// appears must degrade to WARN with a NIL error so the orchestrator advances
// the cycle, instead of the unconditional FAIL+error that aborted cycle-120.
```

### `go/internal/phases/runner/runner_perphase_env_test.go:10` — above `func TestRun_PerAgentModelEnvKey_AgentKeyedNotPhaseKeyed(t *testing.T) {`

```text
// runner_perphase_env_test.go — Bug A regression guard. cmd_loop.go writes
// per-agent model overrides as `EVOLVE_<AGENT>_MODEL` (e.g. `EVOLVE_BUILDER_
// MODEL`), matching the same convention as `EVOLVE_<AGENT>_CLI` and
// `EVOLVE_<AGENT>_PERMISSION_MODE` already use. The runner's model resolver
// at runner.go:284 had drifted to read `EVOLVE_<PHASE>_MODEL`, which silently
// dropped the override for every phase where phase ≠ agent (tdd/tdd-engineer,
// build/builder, audit/auditor, retro/retrospective). Cycle-124 V1 verification
// proved the drop in production: `--model builder=gpt-5.5` reached the loop
// dispatcher but never reached the runner, so codex fell back to the profile
// default (`sonnet` → `gpt-5.4`) and the operator's ChatGPT account 400'd.
// This table-driven test pins the agent-keyed contract — one row per
// known-mismatch phase pair.
```

### `go/internal/phases/runner/runner_phaseio_test.go:17` — above `func TestRun_DefaultProbeHonorsPhaseIO(t *testing.T) {`

```text
// TestRun_DefaultProbeHonorsPhaseIO (ADR-0050 Phase 3.10 Slice 1): with no
// VerifyFn injected, the verdict engine judges with the catalog-aware probe
// New resolves over Options.PhaseIO — at enforce a build FAIL-without-block
// report is caught (parity with the host gate: the ship guard turns the hook's
// PASS into a coherent FAIL and the violation is on the response) and below
// enforce it stays dormant (byte-identical: PASS). Pinned THROUGH Run since
// review fold F4 — the probe lives in the engine only, so the pin observes
// what the engine judged with, not a field New set. Kills `stage dropped from
// the default probe`, `default probe not handed to the engine`.
```

### `go/internal/phases/runner/runner_profile_amplify_test.go:3` — above `import (`

```text
// runner_profile_amplify_test.go — adversarial profile-loading boundary tests
// for cycle-276 T3 (bridge-profile-contract-symmetry). The builder's
// runner_test.go proves that profiles-dir-present + profile-absent → fast-fail
// with the path in the error message. These tests probe the complementary
// boundaries: absent profiles dir → existing behavior preserved (no fast-fail),
// and profile-present → no fast-fail.
```

### `go/internal/phases/runner/runner_reconcile_ctxcancel_test.go:57` — above `func TestRun_CancelledCtx_TeardownReconcileStillSettles(t *testing.T) {`

```text
// TestRun_CancelledCtx_TeardownReconcileStillSettles is the SCOPE guard on the
// cancellation bail, and the reason the teardown call site passes WithoutCancel. On
// the reconcile-on-teardown path a cancelled ctx is frequently the CAUSE of the
// teardown: the tmux driver, on ctx.Err(), takes one final completion poll and
// otherwise exits ExitArtifactTimeout — "laundering a finished session into a
// timeout … the runner's settle-retry was the only thing standing between that
// mislabel and a false FAIL" (driver_tmux_repl.go). So a deliverable that settles on
// a later rung must STILL be caught with a dead ctx, or the cycles-824/825 false-FAIL
// class re-opens: a genuinely-PASS audited cycle discarded and requeued from scratch.
```

### `go/internal/phases/runner/runner_reconcile_stale_test.go:3` — above `import (`

```text
// runner_reconcile_stale_test.go — the SECOND door of cycle-1550 (go-review
// CRITICAL on the bridge baseline fix): after a bridge artifact-timeout the
// runner's reconcile-on-teardown re-reads the canonical artifact and trusts a
// well-formed deliverable. A byte-identical leftover from a PRIOR attempt is
// well-formed, carries the cycle-scoped challenge token, and pre-fix would be
// resurrected as this session's verdict — same wrong outcome as the bridge
// door, five minutes slower. The runner now snapshots the artifact
// PRE-DISPATCH and refuses to reconcile anything still byte-identical to it.
// The cycle-254/255 contract is untouched: a deliverable the agent wrote
// DURING the session (fakeBridge.writeArtifact — new mtime) still reconciles,
// pinned by TestRun_Timeout_WellFormedPASS_ReconcilesToPass.
```

### `go/internal/phases/runner/runner_reconcile_stale_test.go:73` — above `func TestRun_Timeout_RewrittenLeftoverStillReconciles(t *testing.T) {`

```text
// The contract-preserved complement: same stale leftover, but the agent
// REWROTE the report during the session (fakeBridge.writeArtifact — even with
// identical bytes the mtime advances) — reconcile behaves exactly as the
// cycle-254/255 contract requires.
```

### `go/internal/phases/runner/runner_reconcile_test.go:16` — above `func verifyReturns(res deliverable.Result, err error) func(string, phasecontract.Roots) (deliverable.Result, error) {`

```text
// Reconcile-on-timeout (self-healing): when the bridge reports ErrArtifactTimeout
// but the agent's contracted deliverable is on disk and WELL-FORMED, the runner
// must trust the deliverable's verdict (via Classify) instead of synthesizing
// FAIL. This is the deeper fix behind the cycle-254/255 false-FAILs: a complete
// PASS audit report was discarded because the bridge gave up on the wait window.
// The verifyFn seam lets these tests drive the well-formedness branch directly
// without coupling to per-phase contract sections; the real deliverable.Verify +
// EGPS gate is exercised end-to-end in the audit package.
```

### `go/internal/phases/runner/runner_reconcile_test.go:211` — above `type noisyStdoutBridge struct {`

```text
// noisyStdoutBridge simulates a non-timeout completion (err=nil) where the
// agent wrote a well-formed deliverable to disk but the captured stdout
// scrollback is noisy — e.g. it contains the Deliverable Contract's own
// prompt-echoed PASS/FAIL example sentinel lines. This is the cycle-603
// failure mode: NOT a timeout, so the existing reconcile fallback (gated on
// ErrArtifactTimeout, runner.go:585) never engages, and classification falls
// straight through to raw bres.Stdout (runner.go:655-662).
```

### `go/internal/phases/runner/runner_reconcile_test.go:235` — above `func TestRun_NonTimeout_WellFormedDeliverable_PrefersFileOverNoisyStdout(t *testing.T) {`

```text
// TestRun_NonTimeout_WellFormedDeliverable_PrefersFileOverNoisyStdout —
// cycle-603: a non-timeout completion whose captured stdout contains BOTH a
// PASS-example and a FAIL-example contract-style sentinel line (the
// Deliverable Contract's own printed examples, not the agent's real verdict)
// must not classify off that noise. When the on-disk deliverable exists and
// verifies well-formed (OK), the runner must prefer the file — generalizing
// the already-tested timeout-reconcile pattern to every completion path, not
// just ErrArtifactTimeout.
```

### `go/internal/phases/runner/runner_reconcile_test.go:284` — above `func TestRun_Timeout_DeliverableSettlesOnRetry_ReconcilesToPass(t *testing.T) {`

```text
// TestRun_Timeout_DeliverableSettlesOnRetry_ReconcilesToPass — cycles 824/825
// (width-2 storm): a next-phase (retrospective) context-cancel tore down the
// audit bridge session and was LAUNDERED into ErrArtifactTimeout at the exact
// instant the auditor's PASS deliverable was still SETTLING to disk. A
// single-shot verify at that instant reads a half-written file, so the mandatory
// audit phase hard-FAILs and a genuinely-PASS audited cycle (build PASS, tdd
// RED->green, adversarial PASS, audit PASS 0.95) is discarded and requeued from
// scratch. The reconcile must re-verify across a bounded settle window so the
// settled deliverable is caught and the cycle reconciles to the agent's real PASS
// — honoring the reconcile block's own documented intent (trust a deliverable
// written just as the bridge gave up on the wait window).
```

### `go/internal/phases/runner/runner_reconcile_transient_test.go:15` — above `func transientBridgeErr(code int) error {`

```text
// Reconcile-on-transient (cycle-835): a TRANSIENT bridge failure (exit 80/85/86 —
// quota exhaustion, liveness-exhaustion) is an infra teardown, NOT a verdict, in
// exactly the same way an artifact-wait timeout (exit 81) is. The agent may have
// written its contracted deliverable before the infra tore the session down. The
// classic case: agy+codex are quota-walled, so the deep-tier Opus auditor runs on
// an overloaded claude and hits its OWN exit=85 at the tail — AFTER writing a
// complete PASS audit report. Before this fix that report was discarded (the
// non-timeout `else` branch hard-failed without consulting disk) and a genuinely
// PASS-audited cycle was recorded FAIL. The reconcile trigger must cover both
// infra-teardown shapes so a well-formed deliverable is trusted regardless of
// WHICH infra event ended the session.
```

### `go/internal/phases/runner/runner_reconcile_transient_test.go:34` — above `func TestRun_TransientError_WellFormedPASS_ReconcilesToPass(t *testing.T) {`

```text
// TestRun_TransientError_WellFormedPASS_ReconcilesToPass — the core cycle-835
// fix: a transient (exit 85 quota) teardown + a well-formed PASS deliverable →
// reconcile to PASS with a nil error, Reconciled=true, and Classify actually ran
// (proves the fall-through, not a bare sentinel read). RED before the fix: the
// non-timeout `else` branch hard-fails, so err != nil and Classify never runs.
```

### `go/internal/phases/runner/runner_reconcile_transient_test.go:186` — above `func TestRun_TransientError_DeliverableSettlesOnRetry_ReconcilesToPass(t *testing.T) {`

```text
// TestRun_TransientError_DeliverableSettlesOnRetry_ReconcilesToPass — the bounded
// settle-retry applies to transient teardowns too: a deliverable still settling to
// disk (first verifies miss, a later one within the window catches the PASS) must
// reconcile, exactly as on the timeout path (cycles 824/825 settle-race).
```

### `go/internal/phases/runner/runner_settlewindow_test.go:13` — above `func TestRun_CleanExitIdle_DeliverableSettlesLate_WidenedWindowCatchesIt(t *testing.T) {`

```text
// runner_settlewindow_test.go — the verdict-incoherence hotfix (ADR-0072, cycle-921):
// the settle-retry window must outlast a clean-exit-idle agent's worst-case flush
// latency, or a genuinely-valid-but-late-flushed deliverable is missed by Verify and
// the runner falls back to the (sentinel-lost) pane → a false FAIL that contradicts
// the green artifact.
```

### `go/internal/phases/runner/runner_solution_overlay_test.go:11` — above `func TestRunner_OverlaySignalsRideThePhaseRequest(t *testing.T) {`

```text
// TestRunner_OverlaySignalsRideThePhaseRequest — ADR-0099 slice 3: the runner
// hands the dispatch's kernel-projected signals (core.PhaseRequest.Signals) to
// overlay resolution unchanged — it digests nothing itself — so the SCOUT
// dispatch of a document cycle (no report exists yet; the kind is the project
// default core projected) carries solution-scout on BridgeRequest.Skills, the
// build dispatch carries solution-build, and a code or signal-less dispatch
// stays byte-identical (no skills at the profile-default tier).
```

### `go/internal/phases/runner/runner_teardown_acsfloor_test.go:14` — above `const acsFloorToken = "chal-tok-acsfloor-0001"`

```text
// Teardown ACS deterministic floor (verdict-incoherence family: cycles
// 603/921/924/931/3). The failure: a tmux auditor writes a complete PASS
// audit-report.md + a ship-eligible acs-verdict.json, then never runs the
// `evolve phase verify` completion handshake and idles until the runner
// ctx-cancels the session. The ctx-cancel maps to a transient teardown
// (engine 635510d7), so control reaches the reconcile door — but the
// teardown-time deliverable.Verify returns not-OK on a report that
// standalone-verifies OK (a lifecycle artifact, not a defect), so the mandatory
// phase hard-FAILs and a green/verify-OK/ship-eligible cycle is discarded, then
// the ADR-0072 coherence floor sees recorded-FAIL vs on-disk-PASS and HALTS.
//
// The fix: before the teardown default-FAIL, consult the NON-LLM ground truth a
// session stall cannot corrupt — the acssuite verdict. When it is PASS AND the
// report carries THIS cycle's challenge token with a PASS sentinel (anti-gaming
// preserved), reconcile to the agent's own report via the same Classify path the
// clean exit uses. This is exactly the (audit==PASS && acs==PASS) condition the
// coherence floor flags, prevented at the source.
```

### `go/internal/phases/runner/runner_teardown_acsfloor_test.go:54` — above `func TestRun_Teardown_VerifyNotOK_ACSShipEligible_ReconcilesToPass(t *testing.T) {`

```text
// TestRun_Teardown_VerifyNotOK_ACSShipEligible_ReconcilesToPass — the fix. A
// mandatory audit phase, teardown error, and a teardown-time Verify that returns
// NOT-OK (the cycle-3 divergence — a stray/section/lifecycle code on a report
// that is genuinely on disk and PASS). Because the acssuite verdict is PASS and
// the report carries this cycle's token with a PASS sentinel, the runner must
// reconcile to PASS via Classify, not synthesize FAIL.
```

### `go/internal/phases/runner/runner_teardown_acsfloor_test.go:65` — above `notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeStrayInWorktree, Messa…`

```text
// Teardown-time Verify diverges to NOT-OK — the exact cycle-3 symptom.
```

### `go/internal/phases/runner/runner_test.go:269` — above `func TestRun_InvokesEventsProducer(t *testing.T) {`

```text
// TestRun_InvokesEventsProducer — the runner calls the EventsProducer seam
// post-phase with (workspace, phase, cli, cycle), so cyclecost/cycleclassify
// get their <phase>-events.ndjson (ADR-0020 wiring).
```

### `go/internal/phases/runner/runner_test.go:671` — above `func TestRun_CLIResolutionPrecedence(t *testing.T) {`

```text
// TestRun_CLIResolutionPrecedence pins the precedence chain:
//
//	EVOLVE_CLI env var > profile.cli field > "claude-p" default
//
// Before this fix the runner only read EVOLVE_CLI and defaulted to
// claude-p, silently ignoring profile.cli. Operators who edited a
// phase profile to `"cli": "codex"` got claude-p anyway, and the
// dispatch log gave no hint why.
//
// Source: cycle 107 (2026-05-25) attempted-codex smoke that ran
// against claude-sonnet-4-6 despite cli=codex in every profile.
```

### `go/internal/phases/runner/runner_tier_fallback_reproduction_test.go:56` — above `func TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier(t *testing.T) {`

```text
// TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier is the bug-reproduction
// FAIL_TO_PASS pin for cycle-876's wire-tier-fallback-chain task.
//
// scout-report.md's Conclusion and fault-localization-report.md's #1 suspect
// (confidence 0.95, runner.go:467,500-524,545-582) both identify the same
// gap: when the resolved tier is quota-exhausted (exit 85) across every CLI
// in the chain, dispatch must step down ONE policy.TierRank via
// llmroute.TierChain and re-walk the SAME CLI chain at the lower tier before
// giving up (Acceptance Criteria #2/#5, scout-report.md).
//
// This cycle's build (tier_fallback.go: TierChain/DispatchTiered,
// llmroute.go: Plan.Tiers) added the primitives but — per build-report.md's
// own "Design Notes"/"Discovery" sections — never swapped the runner's actual
// production dispatch call site (BaseRunner.Run, runner.go:561-582) from the
// CLI-only llmroute.Dispatch to llmroute.DispatchTiered. That closure still
// captures a single `model := plan.Model` (runner.go:524) by value for every
// attempt and never consults plan.Tiers, so the fix is INERT in production —
// mirroring the c41fa94b→95f3e79f "plumbed upstream, never consumed at the
// call site" failure shape called out in fault-localization-report.md.
//
// A profile with model_tier_default="opus" (TierRank 3) resolves
// plan.Tiers = ["opus","balanced"] (steps down to the universal "balanced"
// floor, tier_fallback.go:TierChain). Once the runner is fixed to dispatch
// via DispatchTiered, an all-85 CLI chain must be walked TWICE — once at
// "opus", once at "balanced" — for a total of 4 attempts. Today only 2
// happen, both at "opus": the runner gives up at the first tier instead of
// stepping down.
```

### `go/internal/phases/runner/runner_verdict_source_test.go:15` — above `func diagsContain(diags []core.Diagnostic, substr string) bool {`

```text
// runner_verdict_source_test.go — the file-authoritative verdict-source rule (ADR-0072
// verdict-incoherence). For a CONTRACTED phase the on-disk deliverable is the SOLE
// verdict source; the lossy, prompt-contaminated terminal pane is never classified.
// This closes the incoherence class at the architecture level (not by widening a
// settle window): timing can no longer flip a valid verdict, and a rejected deliverable
// can no longer be laundered from either the malformed file or the pane.
```

### `go/internal/phases/runner/runner_verify_roots_cycle_test.go:3` — above `import (`

```text
// runner_verify_roots_cycle_test.go — ADR-0100 slice 2: the runner's own
// verify (the classification check and the teardown reconcile) hands the
// verifier the SAME cycle the host gate and the agent self-check use, so a
// declared effect (judged under processing/cycle-N/) is decided by all three
// with one belief. A runner that omitted it would make the verifier fail OPEN
// on every effect-declaring phase — silently keeping the pane as the verdict
// source.
```

### `go/internal/phases/runner/staticprefix_test.go:3` — above `import (`

```text
// staticprefix_test.go — behavior + apicover naming tests for StaticPrefix,
// the read half of the cache-stable prompt-prefix contract (cycle-535 ship).
// CI's apicover -enforce flagged it UNCOVERED (no test named it); these tests
// close the gap by pinning the contract from both ends of the shared
// cycleContextBoundary literal, not by merely naming the identifier.
```

### `go/internal/phases/runner/verdict_engine.go:3` — above `import (`

```text
// verdict_engine.go — unit 11 (ADR-0103, design decomposition/11-phaserunner.md):
// the runner's seam onto the verdict engine. Every old caller keeps its
// spelling: runner.New(Options{…}) at all fifteen production construction
// sites, the ten embedders, the swarmrunner Decorator and phaseregistrar never
// learn the unit exists; the three settle tests keep reading the bound under
// its old names; the engine's ONE construction (which is also the ONE
// resolution of its seams), the ONE projection onto its input and the Center
// derivation live here.
```

### `go/internal/phases/runner/verdict_engine.go:59` — above `return v.VerifyForClassification(gatesignal.Check{Cycle: id.Cycle, RunID: id.RunID, Phase: id.Phase}, phase, roots)`

```text
// The gate's own verifier: ONE verifier for gate and engine (F22).
```

### `go/internal/phases/runner/verdict_engine.go:114` — above `func (b *BaseRunner) ContractVerifierWired() bool {`

```text
// ContractVerifierWired reports whether the composition root SUPPLIED the
// engine's verifier — the gate's Reviewer when the contract gate is on, its
// Null-Object PlainVerifier when off — i.e. the root made the choice; it is
// not a gate-liveness proof (ADR-0103 unit 11 shape).
```

### `go/internal/phases/runner/verdict_engine_test.go:3` — above `import (`

```text
// verdict_engine_test.go — the unit-11 seam (ADR-0103 §6 tests 35-41): the
// ONE construction of the verdict engine, the lazy accessor for a literal
// runner, the ONE projection onto the engine's input, the Center derivation
// (explicit → the bridge's → Null Object), the stream per scenario, and the
// consumer pins on the two beliefs preparation.go now reads from the leaf.
```

### `go/internal/phases/runner/verdict_golden_test.go:3` — above `import (`

```text
// verdict_golden_test.go — ADR-0103 unit 11, step 1: the characterization
// goldens of the verdict engine, captured on 8e8f080f before any code moved
// (verdict/testdata/*.golden.*) and held byte-for-byte across the extraction.
// GREEN on the pre-extraction code; each named mutant was hand-applied once to
// prove the pin bites (doc §6).
```

### `go/internal/phases/runner/verdict_pins_test.go:3` — above `import (`

```text
// verdict_pins_test.go — ADR-0103 unit 11, step 1: the response-shape pins of
// the classify half (diagnostic order, the violation trail, the ship guard) and
// the teardown-FAIL diagnostic. GREEN on 8e8f080f; each named mutant was
// hand-applied once to prove the pin bites (doc §6).
```

### `go/internal/phases/runner/verdict_pins_test.go:125` — above `func TestRun_TeardownFail_DiagnosticCarriesTheCause(t *testing.T) {`

```text
// Test 5 (retargeted at the fold) — the teardown-FAIL diagnostic carries the
// cause: the FIRST violation's message, or the stale-leftover refusal. The
// [VERDICT-FORENSIC] stderr line this test captured on 8e8f080f is kept as
// the record (verdict/testdata/stderr_forensic.golden.txt) and is reproduced
// field-for-field by the RUNNER_TEARDOWN_FAIL event (leaf test
// TestForensicFail_EmitsOneEventWithTheGoldenFields; the stream per scenario by
// TestRun_StreamIsEmptyOnTheHappyPathAndCarriesOneCodePerFaultPath). Kills
// `Violations[0] → [1]`, `refusal text dropped from the diagnostic`.
```

### `go/internal/phases/runner/verdict_scenarios_test.go:3` — above `import (`

```text
// verdict_scenarios_test.go — the byte-identity harness of ADR-0103 unit 11
// (the phase runner's verdict engine): one scenario table drives the response
// goldens, the probe/sleep-count golden and the stream-sequence pins, so the
// pre-extraction behaviour of every reconcile/classify arm is captured ONCE on
// 8e8f080f and replayed through the leaf afterwards. The fixtures are the
// package's own (fakeHooks, verifiedFrom, artifactTimeoutErr, the ACS-floor
// workspace, the stale leftover); only the bridge double is new, because the
// goldens pin CostUSD/Tokens/BootMS/ExitCode on every arm.
```

### `go/internal/phases/runner/worktree_fence.go:3` — above `import (`

```text
// worktree_fence.go — the runner's use of the read-only phase worktree fence
// (ADR-0097). core marks a dispatch WorktreeReadOnly from its one
// write-permission predicate; the runner opens the fence before the first
// attempt and closes it after the last, so the tree every downstream reader
// sees — the audit's explanation binding inside Classify, the retro, the ship
// — is the tree the phase was given. What the fence did is a diagnostic on the
// phase response (report, dashboard, retro) and a WARN in the phase log; it is
// never silently kept and never silently dropped, and it never changes a
// verdict. The retro phase, which calls the bridge itself, holds the same
// treefence.Fence around its launch.
//
// Assumption, stated: the agent is gone when the fence closes. Launch is
// synchronous and the bridge tears the session down before returning; a
// driver that hands back a still-running REPL (resume-preserve) could write
// after the close — those writes reach the ship's own explanation
// verification, which stays armed.
```

### `go/internal/phases/runner/worktree_fence_test.go:14` — above `type mutatingBridge struct{ launches int }`

```text
// mutatingBridge plays the cycle-1603 auditor: it rewrites a material file in
// place and drops a probe test into the package, then writes its report.
```

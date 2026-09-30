# Comment history: `internal/adapters/bridge`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/adapters/bridge/bridge.go:76` — above `resolver phasecontract.Resolver`

```text
// resolver resolves the deliverable contract injected into each phase's
// prompt. Defaults to built-in-only; SetContractResolver upgrades it to a
// catalog-aware resolver so user/minted phases get their spec-derived
// contract block + exact-path footer (WS-A, ADR-0034).
```

### `go/internal/adapters/bridge/bridge.go:81` — above `signals *signalcenter.Center`

```text
// signals is the ADR-0101 Signal Center every engine this Adapter builds
// produces into; injected at construction (NewDefault), nil = Null Object.
```

### `go/internal/adapters/bridge/bridge.go:84` — above `phaseIO config.Stage`

```text
// phaseIO is the EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8b). At
// >=StageAdvisory the injected contract block instructs build/scout/triage to
// self-report failure via a structured sentinel; default StageOff keeps the
// dispatched prompt byte-identical to pre-3.8b.
```

### `go/internal/adapters/bridge/bridge.go:89` — above `recoveryStage string`

```text
// recoveryStage is the ADR-0044 Unified Phase Recovery stage (channel,
// ask-broker, transient-dwell), seeded from policy.json by NewDefault and
// overridden by the cycle root with the Loader-resolved cfg.PhaseRecovery.
```

### `go/internal/adapters/bridge/bridge.go:93` — above `fatalPaneStage string`

```text
// fatalPaneStage is the C2 fatal-pane fast-fail's OWN stage (F27), seeded
// and overridden the same way from cfg.FatalPane. Both stages reach the
// engine through productionEngineDeps, the builder every path shares.
```

### `go/internal/adapters/bridge/bridge.go:100` — above `contextFillWarnPct int`

```text
// contextFillWarnPct is the policy-resolved context-fill WARN threshold
// (cycle-1444), loaded alongside bridgeConfig at NewDefault. Zero (bare
// New(), or an unreadable policy.json) leaves the engine to apply its own
// built-in default, so the fail-open path matches the configured one.
```

### `go/internal/adapters/bridge/bridge.go:133` — above `a.recoveryStage, a.fatalPaneStage = pol.BridgeRecoveryStages()`

```text
// A failed load leaves pol zero, so the recovery dials resolve to their
// compiled defaults — the same fail-open as the timings above — parsed by
// the Loader's own trichotomy (policy.BridgeRecoveryStages). Roots that
// never call the setters (the per-phase registry factories) still carry
// policy's dials; the cycle root overrides them via wireBridgeStages.
// Deliberately BOTH dials: before F27 such roots pinned the program dial
// to shadow whatever policy.json said, so an operator's explicit
// `recovery.phase_recovery` now reaches them too (one source for every
// production path) — the compiled default is still shadow.
```

### `go/internal/adapters/bridge/bridge.go:150` — above `func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps {`

```text
// productionEngineDeps builds the gobridge.Deps shared by every production
// composition path in this Adapter (NewDefault's engineFactory and the
// onStopReview branch of Launch) so they cannot drift apart. Wires
// TokenResolver via tokenusage.DefaultResolver against the env's HOME (see
// configRoot) — the fix for the confirmed cycle-612+ bug where production
// launches got silent zero token telemetry.
```

### `go/internal/adapters/bridge/bridge.go:173` — above `CorroborateWall: gobridge.DefaultWallCorroborator(nil, os.Stderr),`

```text
// Wall corroboration (2026-08-15 false-wall incident): a pane
// exhaustion match escalates rc 85 only after a live one-token probe
// corroborates it — subject-matter wall vocabulary (a lane editing
// the exhaustion fixtures) can no longer forge a quota wall. Wired
// HERE, the production root, so tests keep the legacy nil seam.
```

### `go/internal/adapters/bridge/bridge.go:219` — above `func (a *Adapter) SetPhaseIOStage(stage config.Stage) {`

```text
// SetPhaseIOStage wires the EVOLVE_PHASE_IO rollout stage so the injected
// contract block activates the build/scout/triage self-report-failure
// instruction at >=StageAdvisory (ADR-0050 §3.8b). Default (unset) is StageOff:
// the dispatched prompt is byte-identical to pre-3.8b.
```

### `go/internal/adapters/bridge/bridge.go:227` — above `func (a *Adapter) SetRecoveryStage(stage string) {`

```text
// SetRecoveryStage wires the ADR-0044 Unified Phase Recovery stage (channel,
// ask-broker, transient-dwell) so the engine reads the policy-resolved value
// instead of the retired EVOLVE_PHASE_RECOVERY env var. "" normalizes to
// "shadow" (behavior-neutral) in channel.ResolveStage.
```

### `go/internal/adapters/bridge/bridge.go:235` — above `func (a *Adapter) SetFatalPaneStage(stage string) {`

```text
// SetFatalPaneStage wires the C2 fatal-pane fast-fail's OWN stage (F27) —
// independent of SetRecoveryStage, so arming the fast-fail never arms the
// channel. "" normalizes to "shadow" in the engine (an unwired stage observes).
```

### `go/internal/adapters/bridge/bridge.go:325` — above `func (a *Adapter) contractResolver() phasecontract.Resolver {`

```text
// injectContract wraps the prompt body with the Deliverable Contract (ADR-0034)
// when the selected protocol has a registered contract: the invariant instruction block is
// prepended (cacheable prefix) and the volatile exact-path footer is appended
// (last line). Agents with no contract (non-phase bridge callers) pass through
// unchanged. The path is surfaced in the prompt TEXT here, not just in the
// BridgeRequest.ArtifactPath flag the engine uses to poll — closing the gap that
// forced the agent to infer its own output path.
//
// Resolution runs through a.resolver: built-ins always, plus spec-derived
// contracts for user/minted phases when a catalog resolver is wired (WS-A). A
// nil resolver (zero-value Adapter in a test) degrades to built-in-only.
```

### `go/internal/adapters/bridge/bridge.go:351` — above `c = phasecontract.Contract{Phase: contractID, AgentName: contractID, ArtifactName: filepath.Base(artifactPath)}`

```text
// Unresolved agent WITH a pollable artifact = a minted/unregistered
// phase. The engine will wait on artifactPath either way, so the
// prompt must disclose it — a naked pass-through here is exactly the
// cycle-1424 halt (600s artifact-timeout: the agent was never told
// the path). FOOTER only, not RenderContractTail: the tail embeds a
// `evolve phase verify <agent>` self-check that is guaranteed exit 10
// for a resolver-miss agent — an impossible instruction, the same
// class this branch exists to close (adversarial-review BLOCK).
```

### `go/internal/adapters/bridge/bridge.go:362` — above `includePhaseIO := a.phaseIO >= config.StageAdvisory`

```text
// ADR-0050 §3.8b: at >=StageAdvisory, instruct build/scout/triage to
// self-report failure via a structured sentinel. Gated >=advisory (NOT
// enforce) so the advisory soak exercises the emitted sentinels before the
// enforce flip; off/shadow keep the prompt byte-identical (the classifier's
// always-on Pass 0 must not see new sentinels in production).
```

### `go/internal/adapters/bridge/bridge.go:448` — above `func (a *Adapter) SignalsWired() bool { return a.signals != nil }`

```text
// SignalsWired reports whether a Signal Center was injected at construction —
// the composition root's wiring proof (ADR-0101 S3).
```

### `go/internal/adapters/bridge/bridge.go:452` — above `func (a *Adapter) Signals() *signalcenter.Center {`

```text
// Signals is the carrier's read seam (ADR-0103 unit 11): the phase runner
// adopts the Center the injected Adapter carries, so every runner built over
// the production bridge reports into the root's Center without a fifteen-site
// wiring change. Nil for the Null Object; a nil receiver reads the same way.
```

### `go/internal/adapters/bridge/bridge_contract_minted_test.go:3` — above `import (`

```text
// bridge_contract_minted_test.go — pins the cycle-1424 infra-systemic halt
// class: a MINTED phase (agent name unknown to the contract resolver) was
// dispatched with a naked 2.1KB prompt — no "## Deliverable Contract" block,
// no "DELIVERABLE PATH:" footer — while the engine polled for its report
// artifact. The agent was never told the path; 600s artifact-timeout, exit 81,
// SYSTEM-FAILURE HALT. Rule: whenever the request carries an ArtifactPath the
// engine will poll, the prompt MUST name that path, contract resolution hit or
// miss. Non-phase bridge callers (no ArtifactPath) stay byte-identical.
```

### `go/internal/adapters/bridge/bridge_contract_spec_test.go:14` — above `func TestLaunch_InjectsContract_ForUserPhase(t *testing.T) {`

```text
// TestLaunch_InjectsContract_ForUserPhase proves a config-only phase gets the
// exact-output-path footer + required-section block injected — closing the
// ADR-0034 "agent infers/misses its output path" failure class for user/minted
// phases, with zero Go change to add the phase.
```

### `go/internal/adapters/bridge/bridge_contract_spec_test.go:50` — above `func TestLaunch_PhaseIOFailureInstruction_GatedByStage(t *testing.T) {`

```text
// TestLaunch_PhaseIOFailureInstruction_GatedByStage — Phase 3.8b (ADR-0050):
// when EVOLVE_PHASE_IO>=advisory the dispatched build prompt instructs the agent
// to self-report failure via a FAIL/WARN sentinel carrying a structured block.
// At off/shadow (default) the prompt is byte-identical (no such instruction) —
// the classifier (Pass 0) is NOT PhaseIO-gated, so a sentinel emitted at off
// would change cycle classification. build has no Verdicts, so "evolve-verdict"
// appears in its prompt ONLY via this instruction — a clean discriminator.
```

### `go/internal/adapters/bridge/bridge_contract_spec_test.go:103` — above `func TestSetRecoveryStage_WiresField(t *testing.T) {`

```text
// TestSetRecoveryStage_WiresField confirms SetRecoveryStage stores the stage value
// so the bridge can inject it into the engine's Deps.RecoveryStage (ADR-0044 DI seam).
```

### `go/internal/adapters/bridge/bridge_contract_tail_test.go:93` — above `func TestLaunch_UnregisteredAgentGetsPathDisclosureNotPrefix(t *testing.T) {`

```text
// TestLaunch_UnregisteredAgentGetsPathDisclosureNotPrefix — an unregistered
// agent no longer passes through naked (that was the cycle-1424 halt: the
// engine polls ArtifactPath while the agent is never told it). It gets the
// SYNTHESIZED minimal tail (path disclosure), but not the registered-contract
// prefix block, and the body still leads the prompt.
```

### `go/internal/adapters/bridge/bridge_resolver_restore_test.go:59` — above `func TestInjectContract_ZeroValueAdapter_DegradesToBuiltin(t *testing.T) {`

```text
// TestInjectContract_ZeroValueAdapter_DegradesToBuiltin pins the documented
// degradation of a zero-value Adapter (resolver field nil): injectContract must
// fall back to the built-in resolver rather than nil-panic. For a phase the
// built-in resolver does not know, the body now leads and the synthesized
// path-disclosure tail follows (cycle-1424: naked pass-through starved the
// agent of its polled artifact path); with no artifact path the body passes
// through unchanged.
```

### `go/internal/adapters/bridge/bridge_test.go:110` — above `func TestLaunch_InjectsDeliverableContract(t *testing.T) {`

```text
// TestLaunch_InjectsDeliverableContract — for a registered agent the prompt the
// engine receives carries the Deliverable Contract block AND a footer with the
// EXACT artifact path as (essentially) the last line. The per-cycle path must
// appear only in the suffix, not in the cacheable prefix (cache-safety). ADR-0034.
```

### `go/internal/adapters/bridge/bridge_test.go:228` — above `if !strings.Contains(body, "builder body") {`

```text
// The Deliverable Contract block (ADR-0034) is orthogonal to interactive
// policy and is still injected for a registered agent; assert only that the
// original body survives and no policy block was added.
```

### `go/internal/adapters/bridge/bridge_test.go:301` — above `prefix1 := body1[:strings.Index(body1, "BODYTOKEN1")]`

```text
// The cacheable prefix is everything BEFORE the per-run body. With the
// Deliverable Contract (ADR-0034) the volatile per-cycle path lives in a
// footer AFTER the body, so the prefix (policy + invariant contract block)
// must still be byte-identical across runs.
```

### `go/internal/adapters/bridge/contextfill_wiring_test.go:3` — above `import (`

```text
// contextfill_wiring_test.go — composition-root WIRING proof for cycle-1444
// task `context-fill-warn-threshold`. Engine-side behaviour is proven in
// internal/bridge/contextfill_warn_test.go; this file proves the OTHER half —
// that the operator's policy.json value actually reaches the engine. Without
// it, ContextFillWarnPct is a Deps field only tests ever set (dead config).
//
// RED: Adapter does not resolve context_fill from policy.json and
// gobridge.Deps.ContextFillWarnPct does not exist — this file fails to COMPILE
// until Builder adds them (compile-fail = RED evidence).
```

### `go/internal/adapters/bridge/recovery_dials_test.go:9` — above `func TestSetFatalPaneStage_WiresField(t *testing.T) {`

```text
// recovery_dials_test.go — F27: the ADR-0044 program dial (recoveryStage) and
// the C2 fatal-pane fast-fail's OWN dial (fatalPaneStage) reach the engine
// through the ONE production Deps builder, on every production path.
```

### `go/internal/adapters/bridge/signals_wiring_test.go:3` — above `import (`

```text
// signals_wiring_test.go — ADR-0101 S3: the production Adapter receives the
// Signal Center at construction (explicit DI, never a setter that can be
// forgotten) and threads it into EVERY engine it builds, so bridge.warning /
// bridge.tripwire / pane.liveness from any phase dispatch reach the Center the
// orchestrator listens to. Mirrors the TokenResolver wiring proofs.
```

### `go/internal/adapters/bridge/signals_wiring_test.go:40` — above `func TestAdapter_SignalsReturnsTheInjectedCenter(t *testing.T) {`

```text
// ADR-0103 unit 11 (test 42): Signals() is the carrier's read seam — the phase
// runner adopts the Center the injected Adapter carries. It returns exactly the
// injected Center (never a fresh one), nil for the Null Object, and survives a
// nil receiver (the typed-nil hazard of an optional-interface adoption).
```

### `go/internal/adapters/bridge/tokenresolver_wiring_test.go:3` — above `import (`

```text
// tokenresolver_wiring_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Confirmed bug: `grep -rn TokenResolver go/internal/adapters/bridge/bridge.go`
// returns zero non-test hits — NewDefault's production engineFactory builds a
// gobridge.Deps that never sets TokenResolver, so recordTokenUsage's
// `if e.deps.TokenResolver == nil { return }` guard (internal/bridge/
// engine.go:527) fires on every real launch: token telemetry has been
// silently all-zero since at least cycle 612 (fail-open masks the gap).
//
// Fix contract (Builder implements): a new unexported method
//
//	func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps
//
// that sets TokenResolver: tokenusage.DefaultResolver(configRoot) (configRoot
// resolved from env["HOME"], falling back to os.Getenv("HOME") — same
// precedent as internal/bridge/doctor.go's doctorHome() + ".claude", see
// internal/bridge/billing.go:47 for the exact join). NewDefault's
// engineFactory closure must build its gobridge.Deps via this method (in
// place of the current hand-rolled literal) so the two IDENTICAL DI wiring
// call sites in this file collapse to one. This file — and the
// HasTokenResolver accessor from internal/bridge/hastokenresolver_test.go —
// are both undefined today, so `go vet`/`go build` fails on this package:
// the intended RED signal. DO NOT modify this file; implement production
// code only.
```

### `go/internal/adapters/bridge/validate_leaf_test.go:3` — above `import (`

```text
// validate_leaf_test.go — ADR-0103 unit 10 (design §6 test 39): the adapter's
// request gauntlet projects the host's ONE rule (gobridge.ValidateRequest) —
// the same four strings in the same order as the engine's Launch.
```

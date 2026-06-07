# ADR-0039: Failure Floor + Advisor Failure-Path Vocabulary + Failure-Signal Contract

- Status: Accepted
- Date: 2026-06-07
- Extends: ADR-0024 (integrity floor), ADR-0033 (verdict single source), ADR-0034/0035 (deliverable contracts), ADR-0038 (phase plugins)

## Context

Learning from failure was best-effort: an LLM retrospective ran on FAIL/WARN verdicts *when
nothing else was broken*. Three campaign incidents showed the gaps were exactly where learning
mattered most:

1. **Cycle 243** — the retro bridge died mid-phase (orchestrator degradation branch): the cycle
   ended with NO retrospective artifact and NO lesson. A dead phase cannot self-report.
2. **Cycle 244** — `evolve cycle reset` sealed the run directory without extracting anything:
   operator resets learned nothing (the cycle-245 reset then also ate an in-progress fix).
3. **Loop fatals** — `cmd_loop` abnormal exits (batch cap, circuit breaker, verify-failed stop,
   integrity breach, resume failure) recorded no `failedApproaches` entry; the next session
   started blind.

Separately, the failure-learning *configuration* was scattered (env enable-chain
`EVOLVE_DISABLE_AUTO_RETROSPECTIVE`, registry `enable_var_inverted`, in-phase `req.Env` check)
and the advisor had no vocabulary to make failure-path decisions (retry vs end; full retro vs
memo) — the kernel decided alone, with no forensic trail of what an advisor would have chosen.

## Decision

### 1. Deterministic failure floor — the integrity-floor mirror

The integrity floor guarantees `ship ⇒ build ∧ audit` no matter what an advisor proposes. The
**failure floor** is its mirror on the failure branch: **every abnormal termination produces
learning artifacts no matter what is broken**, deterministically, with the LLM layer as
enrichment on top:

- `go/internal/faillearn` (leaf, stdlib-only): `FailureEvent` →
  `RenderRetrospectiveMarkdown`/`RenderLessonYAML` (golden-byte pinned).
- Orchestrator degradation branches call `writeDeterministicLearning` when the retro errors or
  returns a non-canonical verdict (closes cycle-243).
- `SealCycle` writes artifacts into the sealed archive AND records `failurelog.Record(OperatorReset)`
  BEFORE seal's state read — ordering is load-bearing and pinned (closes cycle-244).
- `cmd_loop.emitFatal` records `LoopFatal` with a `stop_reason=` summary at every abnormal-exit
  site (closes the loop-fatal gap; the exclusion list is documented in commit `532df7a`).
- `failurelog` gained `OperatorReset`/`LoopFatal` classifications, a `Summary` override, and a
  monotonic `lastCycleNumber` so cycle-0 records cannot regress the counter.

**SIGKILL of the orchestrator itself** is closed *downstream*, not by a signal handler: an
orphaned cycle forces reset/resume on the next invocation, and both paths now learn (floor at
seal/reset + resume). A supervisor synthesizes what a dead process cannot self-report.

### 2. Advisor failure-path vocabulary (above the floor, never instead of it)

`Proposal` += `LearningRichness ("full"|"memo")` and `RecoveryAction ("retry"|"end")`;
`retroDecision`/`applyFailureProposal` adopt advisor choices ONLY where the failure-adapter
permits — BLOCK is non-overridable, every clamp is recorded (`failure-proposal-clamped`), and a
memo choice can pick *which* learning phase runs but never *none*. Retry may insert
fault-localization / bug-reproduction (the `failureInsertPhases` kernel map) ahead of tdd.
Happy-path prompts stay byte-identical (prompt-prefix cache); the failure vocabulary renders only
at failure transitions.

**R5 (standing decision):** "retry on a fallback CLI" is already satisfied by the runner's chain
walk (`runner.go:398-438` — exit codes 80/81/124/127 advance CLIs inside ONE `Run` call). An
orchestrator-level CLI switch would invent API the runner already owns. Deterministic artifacts
ARE the fallback when every CLI fails.

**R7 (standing decision):** there are two `failedApproaches` appenders (orchestrator + cmd_loop).
Unification is deferred; `failedrecord_shape_test.go` pins shape parity (Recorded keys ⊆
FailedRecord) so they cannot drift apart silently.

### 3. One user surface: `policy.json:failure_floor` (Phase 4a)

```json
{ "failure_floor": { "always_learn": true, "audit_fail_routes_to": "retrospective" } }
```

- Closed vocabulary {`retrospective`, `memo`}; unknown values fall back to the default (the floor
  guarantees SOME learning phase routes).
- The composition root folds policy → `cfg.AuditFailRoutesTo`; router Rule 1 honors it AHEAD of
  the deprecated enable-chain. Empty ⇒ legacy behavior for one more release.
- `always_learn=false` downgrades only the DEFAULT route; an explicitly written
  `audit_fail_routes_to:"retrospective"` wins (explicit beats derived — `FailurePolicy()` launders
  defaults, so the fold checks the raw field).
- The deterministic floor (§1) is **non-configurable** — like the integrity floor, policy tunes
  only the LLM layer.

### 4. Rubric as a projection (Phase 4b — never-duplicate)

The advisor's decision rubric is rendered by ONE renderer (`writeRubricLines`) as a projection of
the structured routing data the kernel already walks: `insert_when` triggers (derived
`field op value → insert <phase>` lines), `conditional_mandatory` rules (ops negated into skip
exemptions), and `router.FailureInsertPhases()` (failure vocabulary). Registry
`routing.rubric_hint` carries ONLY judgment guidance with no structured counterpart, each line on
exactly one card. A threshold can never disagree between the walk and the prompt. The FORBIDDEN
ship-without-audit line stays in Go — kernel invariant, not phase data.

### 5. Defense-in-depth twins (Phase 4c deviation)

`router.EvaluatorFloorPhase` and policy's unexported `evaluatorFloorPhase` remain twins: the
reverse import would cycle (`router/policy.go` imports `policy`), and each layer independently
guaranteeing the evaluator is deliberate. The never-duplicate rule is satisfied by a tripwire —
`TestEvaluatorFloorPhase_SingleSource` — divergence is loud, not silent. This is the sanctioned
pattern for unavoidable twins.

### 6. Migration (Phase 5) and archaeology (Phase 4d)

- `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` is deprecated: honored one more release, `config.Load`
  WARNs `deprecated-flag` with migration guidance whenever it is set; `failure_floor` wins when
  both are set (structurally true — the policy route bypasses `enableOf`; pinned by
  `TestAuditFail_RoutesPerFailurePolicyNotEnableVar`). Net flags: −1 next release.
- `.evolve/llm_config.json` (untracked runtime file, live tree) carries a `_deprecated` note: no
  runtime reader since Step 9 (`resolvellm` resolves from profiles only — see
  `TestResolve_IgnoresLLMConfig`); kept for archaeology.

### 7. Generalized failure-signal contract (Phase 6 design)

Failure signals are unified at CONVERGENCE (one FailureEvent/renderer/appender/floor) but were
heterogeneous at ORIGINATION (8 detection sites building events from thin summary strings). The
fix makes self-describable failures CONTRACTUAL while keeping crash-class failures
supervisor-synthesized (cycle 243 proved a dead phase can't self-report; the floor is the
contract's fallback):

- **Carrier**: the verdict sentinel (ADR-0033) extends to `schema_version: 2` with an optional
  `failure` block — `{"class", "defects": [...], "evidence_paths": [...]}` — one contracted
  artifact, one parser, v1-compatible forever (absent block legal for PASS and old artifacts).
- **Contract conditionality**: contracts with `RequireFailureContext` make a missing/empty
  failure block on FAIL/WARN a Violation (`failure-context-missing`) → the existing
  correction-retry machinery re-dispatches with the exact reason.
- **Digest lifting**: `<phase>.failure_class` / `<phase>.defect_count` become objective signals
  (generic plane), so failure-phase insertion is DATA-driven on the walk; the Phase-3
  `failureInsertPhases` map remains ONLY for the retro-branch retry path (different mechanism).
- **faillearn consumption**: structured defects/evidence flow into lessons (supervisor synthesis
  stays the fallback); `cycleclassify` gains a Pass-0 sentinel read mapped via
  `failurelog.NormalizeLegacy` (unknown classes fall through to regex passes).

## Consequences

- No abnormal termination is silent: kill -9 a retro bridge, `evolve cycle reset`, or a loop
  fatal all leave a retrospective + lesson + failedApproaches entry (live-verified for reset).
- Failure-learning policy has exactly one user surface; the env flag retires next release.
- The advisor participates in failure routing with full forensics (routing-decision artifacts,
  clamps), but can never weaken the floor.
- Open follow-ups: persona cards (`agents/evolve-router.md`) still name failure-insert phases
  statically (queued for the Phase-6 personas pass); R7 appender unification deferred.

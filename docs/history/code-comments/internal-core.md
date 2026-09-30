# Comment history: `internal/core`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/core/activating_fields_guard_test.go:3` — above `import (`

```text
// activating_fields_guard_test.go — PA-BIG S4 (ADR-0058): the hard registry
// guard (trust anchor). ADR-0058 made the kernel config-driven with a
// byte-identical LITERAL backstop, so dropping an activating field from the
// shipped registry would silently revert that phase to literal-as-SSOT with NO
// observable behavior change — invisible to every behavior test. This guard
// makes the drift LOUD by asserting the shipped registry (and the control seam)
// AGREE with the literal kernel: config must stay the live source, the literal a
// pure backstop. Expectations are DERIVED from the literal (not a fresh golden),
// so the guard catches drops AND divergence.
```

### `go/internal/core/activating_fields_guard_test.go:20` — above `func TestRegistryActivatingFields_AgreeWithLiteralKernel(t *testing.T) {`

```text
// TestRegistryActivatingFields_AgreeWithLiteralKernel loads the shipped registry
// and asserts the ADR-0058 activating fields are present and resolve to the same
// successors the literal kernel would pick.
```

### `go/internal/core/advisor_catalog_ondemand_test.go:3` — above `import (`

```text
// advisor_catalog_ondemand_test.go — the advisor's SELECT menu must be a menu,
// not an inventory.
//
// Measured 2026-08-23 on the runtime plane: 65 non-control phases are projected
// as advisor SELECT cards against maxEnrichedCatalogCards = 12, so 53 render in
// the degraded overflow form. Of those 65, 47 have NEVER been selected in 120
// cycles. The 12 enriched slots are therefore allocated by registry order and
// Optional-ness — not by usefulness — and genuinely useful rare phases lose
// their metadata to phases the advisor has never once chosen.
//
// This is the bloated-tool-set failure mode with a number on it. The fix is NOT
// to delete the unused phases: a phase like migration-safety-check is exactly
// what you want available for the one cycle that needs it, and deleting it
// because it has not fired is deleting the fire extinguisher because there has
// been no fire. The measured harm is advisor CONTEXT, not execution.
//
// So a phase may decline a SELECT slot while staying installed, dispatchable by
// explicit plan, and mintable. It is HIDDEN FROM THE MENU, NOT REMOVED — the
// declined set is still indexed in one line so neither the advisor nor an
// operator loses discoverability.
```

### `go/internal/core/advisor_catalog_ondemand_test.go:288` — above `func TestRepoPhaseCatalog_MenuPhasesResolveAPersona(t *testing.T) {`

```text
// A tracked phase with NO resolvable persona is undispatchable by construction
// — the runner's load-agent step fails before any work happens. It must not
// hold a SELECT slot: cycle-1551 (soak-20260824a) had the advisor insert
// defect-disposition-preflight, whose persona exists nowhere, and the load
// failure killed the whole lane rc=4. Four catalog phases carried the defect;
// two were on the menu. The fail-soft skip (optionalInfraSkip +
// ErrAgentDocMissing) contains the blast radius when one is dispatched anyway;
// this guard keeps them off the menu in the first place. The cure for a phase
// caught here: write agents/evolve-<agent>.md, add a phase-local agent.md, or
// mark it catalog:"on-demand" until someone does.
```

### `go/internal/core/advisor_depth.go:3` — above `func AdvisorDepthExceeded(_ map[string]string) bool {`

```text
// AdvisorDepthExceeded is the injectable recursion-depth guard for PhaseAdvisor
// (ADR-0052 §4.3, defense-in-depth). The PRIMARY guard is the mint denylist in
// mintConfigsFrom; this injectable seam is the secondary backstop.
//
// EVOLVE_ADVISOR_DEPTH was removed in cycle-10 flag-reduction. The guard now
// always returns false — the env-map signal is retired; the mint denylist
// (reservedAdvisorNames) is the sole active recursion gate.
```

### `go/internal/core/advisor_fixture_test.go:3` — above `import (`

```text
// advisor_fixture_test.go — the ONE rich RouteInput the unit-04 goldens
// (ADR-0103, docs/architecture/decomposition/04-advisor.md §6 step 1) are
// captured over on the pre-extraction code and replayed through the leaf:
// every prompt section rendered at once — a 13-card catalog spanning the
// three enrichment buckets, three on-demand names, 23 carryover todos across
// every priority spelling with one 700-rune action, two benches (one walled),
// recall memory, two unavailable phases, conditional rules + triggers +
// rubric hints, a 4100-rune goal and all four signal blocks.
```

### `go/internal/core/agent_doc_missing_test.go:3` — above `import (`

```text
// agent_doc_missing_test.go — an OPTIONAL phase whose persona does not exist
// skips with a WARN; it must not kill the lane.
//
// soak-20260824a, cycle-1551: the advisor inserted defect-disposition-preflight
// (optional, on the SELECT menu), whose phase.json declares no agent and whose
// derived persona (agents/evolve-defect-disposition-preflight.md) exists
// nowhere. The load failed, the cycle died rc=4, the ADR-0072 halt stopped the
// whole batch — a full lane killed by an optional extra's missing file. Four
// catalog phases share the defect (two were on the menu).
//
// The remedy reuses optionalInfraSkip's guard rails wholesale: mandatory
// phases, ship-floor phases, and non-optional phases still fail LOUD — only a
// genuinely optional phase degrades, and the skip is recorded, never silent.
```

### `go/internal/core/agent_doc_missing_test.go:80` — above `func TestOptionalPhaseMissingPersonaSkipsShipsAndLedgersOwnKind(t *testing.T) {`

```text
// End-to-end (cycle-1551 replay): an advisor-scheduled OPTIONAL phase whose
// runner dies with the missing-persona sentinel must not abort the cycle —
// audit+ship still run — and the ledger files the skip under its OWN kind
// (optional_missing_persona_skip), never the infra key: zero retries and no
// infra event happened, and forensics must not merge the classes.
```

### `go/internal/core/agent_doc_missing_test.go:126` — above `func TestOptionalPhaseMissingPersona_LearnsDeterministicallyWithoutRetroAgent(t *testing.T) {`

```text
// 2026-09-09 token-waste root cause #2: cycles 1619/1620 spent 344 s and 311 s
// in a retrospective AGENT for a missing optional persona — a deterministically
// known configuration absence. The skip is still learned (FailedRecord,
// carryover todo, deterministic lesson artifact), but no LLM is dispatched for
// it: there is nothing a retrospective could discover that the sentinel does
// not already say.
```

### `go/internal/core/agent_identity.go:5` — above `type AgentIdentity = advisor.Identity`

```text
// AgentIdentity is the immutable dispatch identity shared by the control-plane
// advisors (PhaseAdvisor, FailureAdvisor) — the fields that select WHICH llm
// brain answers, independent of the per-call operand (prompt / artifact file /
// completion contract, which vary Plan vs Propose vs Advise and stay per-call
// params). It formalizes the byte-identical {cli,model,profile,persona} field
// set both advisors carried separately (ADR-0052 WS1-S1, Value Object): one
// home per identity belief, never two structs drifting apart. Since ADR-0103
// unit 04 that home is the advisor leaf's Identity; this alias keeps every
// core spelling (the failure advisor, the retry adjudicator, the tests).
//
// It is deliberately NOT the bridge-launch call itself — the advisors thread
// context differently (PhaseAdvisor uses context.Background; FailureAdvisor
// threads the caller's ctx) — so only the field-set used to build BridgeRequest
// is shared, per the ADR's critic note.
```

### `go/internal/core/apicover_mergerung2_test.go:3` — above `import (`

```text
// apicover_mergerung2_test.go — ADR-0050 Phase 5 public-API coverage for the
// RUNG-2 scoped-merge review surface (mergerung2.go): NAMES + EXERCISES the
// three exports the apicover per-symbol gate flagged uncovered on main
// (ScopedMergeDisposition, ScopedMergeResult, ScopedMergeReviewer) after cycle
// ship 6459f9ae landed them without a naming test. Each test asserts real
// RunScopedMergeReview behavior (Rule 9 — no `_ = pkg.X` padding).
```

### `go/internal/core/apicover_misc_test.go:3` — above `import (`

```text
// apicover_misc_test.go — ADR-0050 Phase 5 public-API coverage: white-box
// (`package core`) tests that NAME + EXERCISE the last exported symbols in
// internal/core the apicover gate still flags uncovered. Each test asserts a
// real behavior of the symbol it covers (Rule 9 — no `_ = pkg.X` padding):
//
//   - FailureAdvisor / FailureAdvisorOption (failure_advisor.go) — option EFFECT
//     reaches the bridge request via a real Advise call.
//   - PhaseAdvisor / PhaseAdvisorOption (phase_advisor.go) — option EFFECT reaches
//     BridgeRequest.{CLI,Model} via a real Plan call.
//   - Observer (observer.go) — compile-time conformance + a real Start/cancel.
//   - StateUpdater (alloc.go) — compile-time conformance + a real allocate RMW.
//   - WorktreeProvisioner (worktree.go) — compile-time conformance + Create/Cleanup.
//   - ThroughputRecorder (throughput_hook.go) — the func-typed seam fires on a
//     shipped cycle and mutates the State it is handed.
//   - SealResult (reset.go) — every field, via a SealCycle dry-run.
//   - StateMachine (statemachine.go) — Next / CanTransition over the spine.
//   - VerdictReason (verdict.go) — ReasonFromDiagnostics folds a FAIL diagnostic.
//   - Orchestrator.FailureAdviserWired (failure_hook.go) — true with the adviser
//     option, false on a bare orchestrator (executed, not just named).
//   - PhaseBoundaryCheckpointer (orchestrator.go) — the package var the
//     checkpoint package sets via init(); core cannot import checkpoint
//     (circular), so we assign a recording closure, drive a full RunCycle (the
//     real consumer in cyclerun_record.go invokes it), and assert it fired.
//   - CycleStateFile (runworkspace.go) — the constant SealCycle reads from.
//   - PhaseSwarmPlan (phase.go) — Phase.IsValid()/String() over the const.
```

### `go/internal/core/apicover_ports_test.go:3` — above `import (`

```text
// apicover_ports_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises the exported port symbols in ports.go that apicover flags
// UNCOVERED: the Bridge/Guard/Ledger/Storage interfaces, the
// GuardDecision/GuardInput/BatchAccrual/TriageThroughputEntry DTOs, and the
// LedgerEntry.UnmarshalJSON method. Each test reuses the existing in-package
// fakes (fakeStorage, fakeLedger, fakeBridge) and asserts a real contract
// (Rule 9): satisfaction is proven by binding the fake to the port AND driving
// one method through it.
```

### `go/internal/core/apicover_shiperror_test.go:9` — above `type shipCodeCase struct {`

```text
// apicover_shiperror_test.go — public-API coverage (ADR-0050, Phase 5) for the
// ShipError protocol in shiperror.go. The ship phase is a pure executor: every
// failure it cannot execute through is reported as a *ShipError carrying a
// precise Code, a severity Class, the Stage it failed at, and a Debug map. The
// orchestrator errors.As-matches it to decide recovery. These tests name AND
// exercise the 4 protocol types, the 2 previously-uncovered ShipStage consts,
// and all 33 previously-uncovered ShipErrorCode consts, asserting the REAL
// construction + rendering + class-pairing contracts (not value padding).
//
// There is intentionally NO code->class mapper function in core: the class is
// chosen per call-site (internal/phases/ship/{verify,gitops}.go via the shipErr
// wrapper) and recorded verbatim by NewShipError. So the contract this file
// pins is: (a) NewShipError faithfully records {code,class,stage,message,debug};
// (b) Error() renders all three identity fields; (c) the canonical class each
// code is constructed with in production round-trips through errors.As. The
// per-code wantClass column below is transcribed from the live ship call-sites,
// so a drift between this table and production is a real protocol regression.
```

### `go/internal/core/apicover_shiperror_test.go:64` — above `{CodeControlPlaneViolation, ShipClassPrecondition, StageVerifyClass},`

```text
// verify-class — pipeline control-plane integrity boundary (ADR-0064)
```

### `go/internal/core/audit_fail_decision.go:11` — above `type RetryAdjudicator interface {`

```text
// audit_fail_decision.go — the disposition of an audit FAIL, decided AT THE AUDIT
// CHOKEPOINT.
//
// Before this, an audit FAIL always routed to retro, and retro carried three
// unrelated jobs: analysing the failure, classifying it, and GATING whether a
// retry could happen. That conflation is what made ADR-0092's repair reachable on
// only 3 of 16 failures — the retry depended on retro's prose, which is usually
// absent. Here the retry depends on the audit's OWN machine-readable class and the
// ADR-0072 policy table, both of which are always present.
//
// Retro is not removed; it is moved to where it belongs. It is now reached exactly
// when the disposition is DECLINE — the terminal learning step, once, off the retry
// path, so its 20-47 minutes are paid per CYCLE rather than per ATTEMPT.
```

### `go/internal/core/audit_fail_decision.go:47` — above `func (o *Orchestrator) decideAfterAuditFail(cs CycleState) (Phase, string, *SystemFailureSignal) {`

```text
// decideAfterAuditFail returns the successor phase, an operator-readable reason,
// and a halting signal when the ADR-0072 floor binds.
//
// Order is the safety order: deterministic evidence first, policy second, judgment
// last, and judgment only among options the first two already allowed.
```

### `go/internal/core/audit_fail_decision.go:94` — above `suffix = " [adjudicated: " + proposal.Justification + "]"`

```text
// Surface the REASONING, not just the verdict word. clampAdjudication
// rejects an unjustified proposal precisely because the justification is
// this phase's deliverable; requiring it and then discarding it is the
// defect shape ADR-0092's Incoherent flag had.
```

### `go/internal/core/audit_fail_decision_test.go:55` — above `name:         "task-level audit fail re-enters the dev cycle",`

```text
// The wave-3/4 shape: a task-level rejection now retries, with no
// dependency on retro having written anything.
```

### `go/internal/core/audit_fail_decision_test.go:89` — above `name:         "a system-level class halts at audit, before any retry",`

```text
// The floor still binds, and binds BEFORE any retry — the whole point
// of moving the chokepoint without weakening ADR-0072.
```

### `go/internal/core/audit_fail_decision_test.go:222` — above `runners[PhaseAudit] = &classDeclaringAuditRunner{t: t}`

```text
// A real auditor writes a report declaring its failure CLASS — verified
// against cycles 1572/1574/1576/1577, all of which declare "code-audit-fail".
// The plain fakeRunner writes no artifact, so the disposition would correctly
// (but unrealistically) decline for want of a class.
```

### `go/internal/core/audit_fail_decision_test.go:301` — above `func TestDecideAfterAuditFail_SurfacesTheAdjudicatorsReasoning(t *testing.T) {`

```text
// The adjudicator's REASONING must reach the operator-visible reason. The clamp
// rejects an unjustified proposal because the justification is this phase's whole
// deliverable — computing it, requiring it, and discarding it is the defect shape
// ADR-0092's Incoherent flag had (review finding #6).
```

### `go/internal/core/audit_repair_context_test.go:121` — above `func TestSeedAuditRepairContext_ShipRecoveryRebuildCarriesStandingFindings(t *testing.T) {`

```text
// Cycle 1679 (2026-09-15): audit round 4 PASSED with WARN (three MEDIUM
// defects) and the cycle went to ship; GIT_FLEET_REBASE_NEEDED sent it back
// to build, and that rebuild's brief carried no audit section — the repair
// brief seeds only behind a rejection grant — so round 5 found the same
// defects standing. A recovery rebuild is re-audited by the same rubric: the
// last audit's actionable findings ride the brief under their own key, and a
// rejection grant (the repair path) still outranks them.
```

### `go/internal/core/audit_repair_context_test.go:169` — above `func TestSeedAuditRepairContext_RetroRoutedReentryCarriesStandingFindings(t *testing.T) {`

```text
// Cycle 1684 (2026-09-15): the audit FAILed with a class outside the
// vocabulary, the envelope declined the direct grant, the retrospective
// adjudicated a retry, and the tdd/build re-entry carried NONE of the audit's
// findings (only a generic "audit.failure_class" label) — the builder rebuilt
// blind to "retire the superseded predicate". A retro-routed re-entry is
// re-audited by the same rubric, so it carries the standing findings exactly
// as a ship-error recovery does; the intro names which route brought it back.
```

### `go/internal/core/audit_repair_decision_signal_test.go:12` — above `func TestDecideAfterAuditFail_EmitsTheRepairDecision(t *testing.T) {`

```text
// Cycle 1684 (2026-09-15): the audit FAILed with an invented class, the retry
// envelope declined the repair round ("unrecognised class"), and the cycle
// went to retro with NO line anywhere — the decision that costs a full
// retrospective before any retry was invisible. The decision is a coded signal on
// both branches: WARN when declined (the reason is the envelope's), INFO when
// a repair round is granted (which phase, which attempt).
```

### `go/internal/core/audit_round_artifacts.go:3` — above `import (`

```text
// audit_round_artifacts.go — round-scoped audit artifact retirement
// (cycle-1603, 2026-09-02).
//
// The auditor persona pre-writes acs-verdict.json and audit.Classify honors a
// pre-staged file (the verdict-exists gate skips regeneration). So ANY re-audit
// — the ADR-0092/0093 repair loop, a bookkeeping regrade, a ship-error
// recovery re-audit, a debugger RERUN_PHASE — that leaves the previous round's
// verdict at its canonical path replays SUPERSEDED evidence into the fresh
// round: in cycle-1603 round-1's ship_eligible=false amendment forced every
// repaired PASS back to FAIL, making the repair loop structurally unable to
// succeed. The belief "a re-dispatch of audit supersedes the previous round's
// audit evidence" lives at ONE seam: the audit pre-dispatch block, beside
// resetFloorFailReason, mirrored on both dispatch surfaces (cyclerun_dispatch
// and resume) — never at a branch site, which would cover only a subset of the
// re-audit paths and open an artifact blackout window across the re-entered
// tdd/build phases.
```

### `go/internal/core/audit_round_artifacts.go:63` — above `func retireSupersededAuditArtifacts(workspace string, round int) {`

```text
// retireSupersededAuditArtifacts renames the superseded round's verdict
// artifacts to round-suffixed archives (acs-verdict.round<N>.json,
// audit-report.round<N>.md), so the fresh audit regenerates its verdict from
// execution rather than replaying the previous round's, while the old round's
// evidence survives for forensics (the cycle-1434 acs-verdict.foreign.json
// precedent). round is the completed-round count; 0 (first dispatch) is a
// no-op. Absent files are the normal fresh shape. A retirement that cannot
// archive falls back to removing the stale file — a stale verdict left behind
// silently recreates the cycle-1603 class — and every evidence-losing or
// failed outcome is reported loudly.
```

### `go/internal/core/audit_round_artifacts_test.go:3` — above `import (`

```text
// audit_round_artifacts_test.go — regression contract for the cycle-1603
// stale-verdict-artifact class (2026-09-02).
//
// Any audit RE-dispatch (the ADR-0092/0093 repair loop, a bookkeeping regrade,
// ship-error recovery, debugger RERUN_PHASE) that leaves the previous round's
// verdict artifacts at their canonical paths replays superseded evidence
// through audit.Classify's verdict-exists gate: in cycle-1603 round-1's
// agent-amended ship_eligible=false forced every repaired PASS back to FAIL,
// so the repair loop was structurally unable to succeed. The retirement lives
// at the audit pre-dispatch seam (beside resetFloorFailReason, both dispatch
// surfaces) — the unit tests pin the helper's contract, and the live test pins
// the wiring through a real repair cycle.
```

### `go/internal/core/audit_round_artifacts_test.go:101` — above `func TestAuditRedispatch_RetiresPreviousRoundVerdicts(t *testing.T) {`

```text
// THE LIVE PATH — the cycle-1603 shape driven through a real repair cycle. The
// round-1 auditor pre-writes acs-verdict.json (the persona instructs exactly
// that) and FAILs with a repairable class; the granted repair re-enters
// tdd/build; at the round-2 audit DISPATCH the stale verdict must already be
// retired — asserted from inside the runner, at the exact moment the real
// verdict-exists gate would have read it.
```

### `go/internal/core/audit_round_artifacts_test.go:135` — above `type verdictStagingAuditRunner struct {`

```text
// verdictStagingAuditRunner reproduces the live auditor's artifact behavior:
// round 1 pre-writes the acs verdict + a FAIL report with a repairable class;
// round 2 checks the canonical paths were retired before it started and PASSes.
```

### `go/internal/core/audit_round_artifacts_test.go:177` — above `func TestResumedAuditRedispatch_RetiresPreviousRoundVerdicts(t *testing.T) {`

```text
// Resume-surface parity: a cycle that crashed and resumed AT audit must retire
// superseded verdicts on its re-audits exactly like the live loop — the resume
// loop has its own dispatch seam (resume.go), and a miss there would revive
// the cycle-1603 replay on precisely the surface that exists for recovery.
```

### `go/internal/core/audit_round_artifacts_test.go:206` — above `func TestSupersedePreviousAuditRound_AdvancesThePersistedDispatchCounter(t *testing.T) {`

```text
// The primitive's counter math IS the crash-correctness (review-2 HIGH): each
// call retires by the persisted dispatch count and then advances it, so a
// dispatch that never completes still marks its round superseded. A dropped
// increment would leave every later dispatch at round 0 — permanently
// honoring stale verdicts.
```

### `go/internal/core/blocker_breaker.go:3` — above `import (`

```text
// blocker_breaker.go — mid-batch pipeline-blocker breaker (ADR-0072 extension,
// operator directive 2026-07-22: a pipeline blocker must be fixed directly,
// never passed to following cycles). The ADR-0072 floor halts on FORGED
// verdicts; this breaker halts on the other blocker signature — the same
// failure identity recurring across a batch's cycles, which honest-looking
// per-cycle FAILs never surface on their own (batch-5 burned six cycles on one
// class; the 862–899 storm burned 37 on byte-identical defect strings).
//
// Two deterministic rules over the S1 failure digests (failure_digest.go),
// evaluated batch-scoped by the loop after every iteration:
//
//	Rule A "guard-class"           — guard-abort digests ≥ GuardClassCeiling.
//	                                 A guard abort is pipeline machinery
//	                                 failing by construction, never task-legit.
//	Rule B "identical-fingerprint" — one exact fingerprint ≥
//	                                 IdenticalFingerprintCeiling. Identical
//	                                 failure identities cannot be distinct
//	                                 honest defects.
//
// Same-task repeats are S5 quarantine's job (task_retry_ceiling) — the breaker
// is task-agnostic so a healthy batch of many DIFFERENT honest rejections
// (batch-2's shape) never trips it. A zero ceiling disables its rule (the
// policy escape hatch, mirroring the positive-overrides-win threshold merge).
```

### `go/internal/core/blocker_breaker.go:45` — above `UnexplainedCeiling int`

```text
// UnexplainedCeiling halts when this many digests carry NO machine-
// readable failure reason (the degenerate empty-evidence bucket) — a
// diagnosability breakdown, deliberately named apart from the identical-
// fingerprint rule (batch-6: three DIFFERENT failures shared one empty
// fingerprint).
```

### `go/internal/core/blocker_breaker.go:51` — above `AckedFingerprints map[string]bool`

```text
// AckedFingerprints excludes acknowledged fingerprints from Rule B's
// identical-fingerprint count (the .evolve/resolved-fingerprints.json
// ledger — LoadResolvedFingerprints/AppendResolvedFingerprint). This is
// the fix for the cycle-1329 recurrence: a fingerprint whose root cause
// was already diagnosed and consumed kept re-tripping the breaker on
// every relaunch because the breaker had no memory across invocations.
// The exclusion is scoped to ONE named fingerprint at a time, never a
// blanket Rule B disable — a different, unacked fingerprint still halts
// at the ceiling unchanged. Nil/empty = no exclusions (zero value,
// byte-identical behavior for every pre-existing caller).
```

### `go/internal/core/blocker_breaker.go:62` — above `ConsecutiveFailuresCeiling int`

```text
// ConsecutiveFailuresCeiling halts when this many CYCLES fail
// back-to-back (cycle numbers n, n+1, …, no PASS between) REGARDLESS of
// fingerprint identity (operator directive 2026-08-10: the 2026-08-09
// batch burned 10 failed cycles / 0 ships before the identity-keyed rule
// tripped — varied failure modes evaded it for 7 extra cycles). Acked
// digests break the streak: a batch resumed to verify a fix must not
// insta-halt on the history it is verifying. 0 disables. Evaluated LAST
// so the specific rules above, whose reasons carry actionable repro
// hints, name the halt when they also trip.
```

### `go/internal/core/blocker_breaker.go:153` — above `var consumptionFingerprintRe = regexp.MustCompile('fingerprint\s*=?\s*"?([A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+\|[A-Za-z0-9_…`

```text
// consumptionFingerprintRe anchors on the literal `fingerprint` token
// (never a bare pipe-delimited substring — a `docs/a|b|c.md` path or any
// other triplet-shaped text with no `fingerprint` keyword must not match)
// followed by an optional `=`, an optional opening quote, and the
// pipe|pipe|pipe triplet shape every failure fingerprint takes
// (failure_digest.go). Matches both consumed_by's unquoted
// `fingerprint ship|unknown|76d0f4fca190` and notes' quoted
// `fingerprint "ship|unknown|76d0f4fca190"` shapes with one pattern.
```

### `go/internal/core/blocker_breaker.go:212` — above `func isUnexplainedDigest(d FailureDigest) bool {`

```text
// isUnexplainedDigest reports a digest whose fingerprint asserts NO defect
// identity. Two shapes: the degenerate empty-evidence digest (no reason
// artifact — phase and pre-class degraded to unknown), and the self-marked
// content-free digest (reasons were empty or pure agent-graded router
// boilerplate; batch-14: three DISTINCT auditor findings shared one
// boilerplate fingerprint and false-tripped the identical rule). These MUST
// NOT count as "identical" defects — distinct failures collapse into these
// buckets by construction; the UnexplainedCeiling rule owns them under its
// honest diagnosability-breakdown name.
```

### `go/internal/core/blocker_breaker_consecutive_edge_test.go:3` — above `import "testing"`

```text
// blocker_breaker_consecutive_edge_test.go — edge-case pins for the #423
// consecutive-failures rule, added with the 2026-08-09 zero-ship batch
// postmortem (docs/incidents/2026-08-09-zero-ship-batch.md). The base suite
// pins the core semantics; these close the reviewer-flagged corners: rule
// precedence against guard-class, ceiling-1 hair trigger, duplicate digests
// for one cycle, and an ack that splits a long run into two sub-ceiling
// halves versus one that leaves an independently-tripping half.
```

### `go/internal/core/blocker_breaker_consecutive_test.go:3` — above `import "testing"`

```text
// blocker_breaker_consecutive_test.go — pins the consecutive-failures halt
// rule (operator directive 2026-08-10): 3 consecutive FAILed cycles halt the
// batch REGARDLESS of fingerprint identity. The 2026-08-09 batch ran 10
// failed cycles / 0 ships before the identical-fingerprint rule finally
// tripped — varied failure modes (unsatisfiable disposition contract,
// false-RED ship gate, citation blocks) evaded an identity-keyed ceiling for
// 10 cycles of burned quota while two of the failures were pipeline defects
// a cycle-3 deep-dive would have caught.
```

### `go/internal/core/blocker_breaker_consecutive_test.go:37` — above `digests := []FailureDigest{`

```text
// Cycle 7 passed (no digest); 5,6 and 8,9 are two separate 2-streaks.
```

### `go/internal/core/blocker_breaker_consecutive_test.go:61` — above `cfg := consecCfg(3)`

```text
// Cycle 6's fingerprint was acked (root cause fixed, pending verify) —
// counting it would re-halt a batch resumed exactly to verify the fix.
```

### `go/internal/core/blocker_breaker_consumption_test.go:3` — above `import (`

```text
// blocker_breaker_consumption_test.go — RED contract for cycle-1334's
// auto-ack-on-consumption gap (scout-report.md Task 1
// "recurrence-ack-consumption-wiring", inbox item
// 2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json).
//
// Cycle-1332 wired the MANUAL branch of the fix (`evolve loop --reset
// --fingerprint <fp>`, cmd_loop_fingerprint_ack_test.go). The inbox item's
// "fix" field's OTHER branch — the item explicitly says "the ack fix must
// clear BOTH stores" and names transactional consumption as the eventual
// path — is still unimplemented: a pipeline-defect P0 item can be moved to
// .evolve/inbox/consumed/ carrying the failure fingerprint in its
// consumed_by narrative (or, before a narrative is written, in its
// auto-filed notes field) and NOTHING parses it back out to ack the ledger.
// The operator has to hand-retype the fingerprint into --reset
// --fingerprint — "exactly the toil-and-tamper-surface the sanctioned flows
// exist to avoid" (the inbox item's own words).
//
// This file pins the two new exported symbols that close that gap:
//
//	ParseConsumptionFingerprint(text) (fp, ok)      — free-text extractor
//	ConsumePipelineDefectFingerprint(...) (fp, err) — the ledger writer
//
// both of which do not exist yet in blocker_breaker.go — every test below
// fails to compile until Builder adds them (a clean RED, not a false pass).
```

### `go/internal/core/blocker_breaker_consumption_test.go:148` — above `unackedCfg := defaultBreakerCfg()`

```text
// Without consumption: unchanged ADR-0072 halt behavior.
```

### `go/internal/core/blocker_breaker_test.go:3` — above `import (`

```text
// blocker_breaker_test.go — RED contract for the mid-batch pipeline-blocker
// breaker (operator directive 2026-07-22: a pipeline blocker must be fixed
// directly, not passed to following cycles). Batch-5 burned SIX cycles on one
// recurring class with every signal on disk and no mechanism acting mid-batch;
// the 862–899 storm burned 37 with byte-identical defect strings. Two
// deterministic rules over the S1 failure digests:
//
//	Rule A — guard-abort class ≥ ceiling (default 2): guard aborts are
//	         pipeline machinery failures by construction, never task-legit.
//	Rule B — byte-identical fingerprint ≥ ceiling (default 3): three
//	         identical failure identities cannot be three honest defects.
//
// Same-task repeats stay S5 quarantine's job (task_retry_ceiling) — the
// breaker is batch-scoped and task-agnostic.
```

### `go/internal/core/blocker_breaker_test.go:68` — above `v := EvaluateBlockerBreaker([]FailureDigest{`

```text
// Batch-2's healthy shape: many FAILs, all distinct task-level catches —
// the breaker must never halt a batch of honest, different rejections.
```

### `go/internal/core/blocker_breaker_test.go:123` — above `func TestLoadResolvedFingerprints_ReadsLedgerRecords(t *testing.T) {`

```text
// --- Cycle-1332: resolved-fingerprints ack ledger ---
//
// Incident: cycle-1329's identical-fingerprint halt (ship|unknown|76d0f4fca190)
// was diagnosed and fixed (#415), consumed twice, and re-tripped the breaker
// on every relaunch — the breaker re-scans disk fresh every call with no
// memory of "already diagnosed and consumed". These tests pin the ack
// ledger (LoadResolvedFingerprints/AppendResolvedFingerprint) and its
// exclusion wiring into EvaluateBlockerBreaker's Rule B.
```

### `go/internal/core/blocker_breaker_test.go:165` — above `fp := "ship|unknown|76d0f4fca190"`

```text
// Literal cycle-1329 reproduction: 3x identical-fingerprint digests, one
// of them acked — must NOT halt.
```

### `go/internal/core/bookkeeping_apicover_named_test.go:3` — above `import "testing"`

```text
// bookkeeping_apicover_named_test.go — apicover named binding for the two
// exported bookkeeping-reason classifiers (issue #433 class: a new exported
// surface needs a NAMED covering test in its OWNING package; the
// phases/audit singlesource pin exercises them cross-package, which apicover
// does not count). Semantics are pinned by bookkeeping_regrade_test.go and
// the audit-package producer pin; this test binds the exported names.
```

### `go/internal/core/bookkeeping_regrade.go:3` — above `import (`

```text
// bookkeeping_regrade.go — the bookkeeping-regrade micro-cycle (2026-08-10
// three-perspective investigation; inbox bookkeeping-fail-regrade-microcycle).
//
// The class it repairs: an audit FAIL whose ONLY explanations are
// bookkeeping-contract gates (continuation-disposition preflight,
// closure-claim citations) while the auditor's own narrative was PASS/WARN.
// Measured cycles 1390-1429: 6 such cycles died to full continuation
// re-drives (~2M tokens each, 0/11 continuation pass rate) to author one JSON
// artifact. The regrade instead re-dispatches AUDIT once in the same cycle on
// the same snapshot — the auditor re-runs with its restored persona (#434),
// authors the bookkeeping artifact, and the deterministic gates re-evaluate.
//
// Placement in the decision stack (top outranks bottom):
//   ADR-0072 floor (verdict-incoherence / infra-systemic)  → HALT
//   bookkeeping regrade (this file)                        → retro→audit, once
//   routing strategy / failure adapter                     → tdd | ship | end
//
// Trust boundary: eligibility reads CycleState.AuditFailReasons — orchestrator
// memory, set at the recordFloorVerdictFailure chokepoint — never a workspace
// file. The once-per-cycle bound is CycleState.BookkeepingRegradeAttempted,
// same ownership. An agent can neither trigger a regrade (worst case: one
// extra audit dispatch, still gate-graded) nor unbound it.
```

### `go/internal/core/bookkeeping_regrade_test.go:3` — above `import (`

```text
// bookkeeping_regrade_test.go — RED contract for the bookkeeping-regrade
// micro-cycle (inbox 0.92, three-perspective investigation 2026-08-10).
//
// The disease: cycles 1390-1429 show 6 FAILs where the auditor graded the work
// PASS/WARN and only deterministic bookkeeping gates (continuation-disposition
// preflight, closure-claim citations) forced FAIL. Each burned a full
// continuation re-drive (~2M tokens, measured 0/11 continuation pass rate)
// to author one JSON artifact. The fix: at the retro chokepoint, a FAIL whose
// ONLY explanations are bookkeeping-class routes to a bounded same-cycle audit
// re-dispatch (retro→audit, once per cycle) instead of dying to a continuation.
//
// Trust boundary: eligibility reads CycleState.AuditFailReasons (orchestrator
// memory, the ADR-0072 pattern) — never a workspace file an agent could author.
// Bound: CycleState.BookkeepingRegradeAttempted, also orchestrator-owned.
```

### `go/internal/core/bookkeeping_regrade_test.go:157` — above `func TestDecideAfterRetro_FloorOutranksRegrade(t *testing.T) {`

```text
// Floor supremacy: a floor-category classification still halts an otherwise
// regrade-eligible cycle — the regrade sits BELOW the ADR-0072 floor.
```

### `go/internal/core/boot_preflight.go:13` — above `func classifyDirtyPaths(paths []string) (quarantine, ignored []string) {`

```text
// boot_preflight.go — boot-time recovery primitives for a dirty/tampered main
// tree (cycle 507, task wire-boot-recovery-functions; the piece cycle 506's
// audit F1 flagged as unwired). When a leak escapes into the main tree, EVERY
// subsequent cycle's tree-diff guard FAILs, attributing the pre-existing dirt to
// whichever phase runs first and wedging the loop until a human `git stash`es.
// These functions let runLoop's boot path self-heal before the first cycle
// dispatches — non-destructively (stash, not checkout) and only for tracked
// source, never the loop's own managed dirs.
```

### `go/internal/core/boot_preflight_test.go:3` — above `import (`

```text
// boot_preflight_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for the loop's boot-time recovery from a dirty
// main tree. Function-level behavior contract for the two recovery primitives
// the Builder (re)implements in boot_preflight.go.
//
// Root cause (scout-report.md cycle 506/507 Key Finding 1/3): when a leak
// escapes into the main tree (from any source), EVERY subsequent cycle's
// tree-diff guard FAILs, attributing the pre-existing dirt to whichever phase
// runs first, wedging the loop until a human `git stash`es. The boot path must
// self-heal: quarantine tracked-source dirt (non-destructively, via stash)
// BEFORE the first cycle dispatches, while leaving the loop's own managed dirs
// (.evolve/, knowledge-base/) untouched, and surface a ship-binary SHA mismatch
// at boot rather than only when the ship phase fails (the 498/500/502 cascade).
//
// Cycle 506 built these functions but NEVER WIRED them (audit F1, CRITICAL);
// the cycle was reset so they no longer exist on disk. This cycle re-establishes
// the function contract HERE and the wiring contract in
// cmd/evolve/cmd_loop_boot_recovery_test.go (the piece 506 lacked).
//
// References classifyDirtyPaths / QuarantineDirtyTree / ShipSHAMismatch, which
// the Builder implements. RED now (undefined symbols → core test package fails
// to compile). Do NOT modify this file — implement the production seam.
```

### `go/internal/core/boot_preflight_test.go:60` — above `func TestClassifyDirtyPaths_ExcludesShipBinary(t *testing.T) {`

```text
// AC2b (regression, cycle 514): the ship binary go/bin/evolve is verified and
// re-pinned by the SAME boot-recovery pass, so quarantine must NEVER stash it —
// stashing a rebuilt binary would revert on-disk to the old committed one and
// re-open the SELF_SHA mismatch the auto-repin just healed (the 508-513 cascade).
```

### `go/internal/core/build_floor_acs_enrollment_test.go:3` — above `import (`

```text
// build_floor_acs_enrollment_test.go — regression pin for the cycle-1145
// gate-block (fingerprint build|gate-block|866df5da1e50), which recurred into
// cycle-1147 and blocked the lane for three attempts.
//
// Mechanism: cycles 1141/1144/1145 added ./acs/cycle<N> lines to
// go/.apicover-enforce under an invented "completeness invariant". Enrollment
// routes a package into the build floor's ENFORCED coverage run, which reads
// the untagged "build constraints exclude all Go files … [setup failed]" SETUP
// result as a TEST failure — so every subsequent cycle whose base diff carried
// those packages rejected its own handoff. These tests pin both halves of the
// fix: the enrollment file itself, and the floor's tag-visibility filter.
```

### `go/internal/core/build_floor_addedtests_test.go:12` — above `func TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags(t *testing.T) {`

```text
// TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags
// reproduces cycle 1679 (2026-09-14): the lane ADDED go/acs/cycle1676
// (`//go:build acs`), the floor's default-context run could not see it
// (buildTagVisiblePackages drops a package with no GoFiles in that context),
// the audit passed, and the ship's added-test backstop was the first to run
// it — red, after the builder had handed the tree over. The floor now runs
// every added tag-gated package under its own tags, through the same seed
// and grouping the ship gate uses, and reports a red like any other floor
// failure, so the builder fixes it while it still owns the tree.
```

### `go/internal/core/build_floor_committed_test.go:69` — above `write("go/.apicover-enforce", "./bad\n")`

```text
// RED-2 (the 5-instance apicover parity class, cycle-1022 et al.): an
// ENFORCED package gaining an unnamed export must be caught AT HANDOFF —
// the floor runs the same AST naming check CI's api-coverage-enforce runs.
```

### `go/internal/core/build_floor_diagnostic_test.go:3` — above `import (`

```text
// build_floor_diagnostic_test.go — cycle-1270 blocker (B-5).
//
// Cycle-1268 died with this recorded failure reason:
//
//	./cmd/evolve: unit tests FAIL
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//
// 400 bytes of a benign, repeated warning and not one word about what failed.
// The floor kept output[:400], but `go test` writes its `--- FAIL` lines,
// panics and stack traces at the END. The truncation was pointed at exactly the
// region where the diagnosis was not — the difference between a cycle that is
// broken and a cycle that is undiagnosable.
```

### `go/internal/core/build_floor_protected_integration_test.go:5` — above `import (`

```text
// build_floor_protected_integration_test.go — F37 with real git: the cycle
// 1689 shape. The builder's committed work changes an unprotected file AND a
// protected one; the floor names exactly the protected path, judged on the
// cycle-base diff (so committing the work cannot hide it) plus untracked files.
```

### `go/internal/core/build_floor_protected_integration_test.go:74` — above `func TestProtectedSurfaceFloorChecks_SeesARenameOutOfTheSurface(t *testing.T) {`

```text
// TestProtectedSurfaceFloorChecks_SeesARenameOutOfTheSurface (architecture
// review F37 M1): with rename detection on, `git diff --name-only` prints only
// the NEW path of a committed `git mv`, so a protected file moved to an
// unprotected name — and weakened while it stays >50% similar — would pass.
```

### `go/internal/core/build_floor_protected_test.go:3` — above `import (`

```text
// build_floor_protected_test.go — F37 (2026-09-26): the build handoff floor
// asks the ship tripwire's question (ship/integrity.go, ADR-0064) at the one
// phase that can act on it. Cycle 1689's builder rewrote the protected
// go/internal/core/cyclerun.go through a shell tool the Edit/Write role guard
// never sees; nothing looked again until ship refused the diff after the
// audit — and that refusal recovered into a re-audit of the same diff.
```

### `go/internal/core/build_floor_reviewer.go:3` — above `import (`

```text
// build_floor_reviewer.go — the shift-left build handoff floor (operator
// directive 2026-07-21): deterministic checks move to the FRONT, as part of
// build-phase verification, while the judgment phases (audit, adversarial-
// review) still follow as the final verdict layer. Mounted in the E2
// DeliverableReviewer chain for phase==build only: a red deterministic
// self-check REJECTS the build deliverable, which the existing correction
// ladder converts into a bounded in-phase builder fix — closing the
// cycle-1008 class where the builder recorded ./cmd/evolve failing in
// build-selfcheck.json and handed off anyway, burning four downstream phases
// before the ACS toolchain gate refused ship.
//
// The reviewer owns POLICY only; the deterministic ENGINE is injected
// (production: the existing phase_bindings selfcheck/gofmt machinery via
// BuildFloorChecks). Fail-open floors: a nil engine or an engine that cannot
// run approves loudly — downstream deterministic gates (ACS toolchain,
// apicover, CI) stay armed, so the floor can never false-block a build over
// its own plumbing.
```

### `go/internal/core/build_floor_reviewer.go:76` — above `func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {`

```text
// DefaultBuildFloorChecks is the production deterministic engine: the
// changed-package selfcheck engine plus every check that must run REGARDLESS
// of the changed set. RemovalClaimFailures and personaBudgetFailures are
// deliberately composed OUTSIDE changedPackageFloorChecks: that engine returns
// early when the diff yields no Go test packages, and both of their triggering
// diffs derive exactly zero packages — a build whose only claim is "I deleted
// X" (cycle-660), and a lane whose only change is an agents/evolve-*.md
// persona doc (cycle-1101). The early return is the precise blind spot each
// would otherwise hide behind.
//
// The changed-path set is derived ONCE here and passed down: the two path-
// driven engines must adjudicate the same diff, and one `git diff` per handoff
// is the standing floor rule.
```

### `go/internal/core/build_floor_reviewer.go:94` — above `docsFloorWarn(in, paths)`

```text
// ADR-0077 docs floor: WARN-only, so it rides the SAME derived change set
// rather than returning a failure — an architecture change with no doc is a
// finding for the auditor, never a handoff REJECT.
```

### `go/internal/core/build_floor_reviewer.go:101` — above `func ProtectedSurfaceFloorChecks(member func(string) bool) BuildFloorCheckFn {`

```text
// ProtectedSurfaceFloorChecks is the handoff floor's copy of the ship
// tripwire's question (ship/integrity.go verifyNoControlPlaneEdits, ADR-0064),
// asked at the one phase that can act on it: every changed path on the
// protected control plane is a failure the builder undoes in-phase, through the
// correction ladder. The role guard sees only Edit/Write; a shell tool reaches
// the tree unseen (F37: cycle 1689's builder rewrote core/cyclerun.go that
// way), and before this floor the violation surfaced only at ship — after the
// audit. member is the membership predicate (guards.IsProtectedSurface,
// injected by the composition root: guards imports core). The paths are the
// floor's axis — the cycle-base diff (HEAD when no base is recorded) plus
// untracked files, which is the set ship judges once the post-record
// soft-reset puts HEAD back at the base — with rename detection OFF on both
// sides: with it on, `git diff --name-only` prints only a rename's NEW path,
// so a file moved out of a protected path would be judged by its new name
// alone (architecture review F37 M1).
```

### `go/internal/core/build_floor_reviewer.go:167` — above `func docsFloorWarn(in ReviewInput, paths []string) {`

```text
// docsFloorWarn evaluates the ADR-0077 documentation floor over the handoff's
// change set and prints its WARN to stderr (the same channel every other
// fail-open floor signal uses, so it lands in the phase log the auditor reads).
// Stage comes from .evolve/policy.json `docs_floor.stage`; an unreadable policy
// falls back to the compiled default, which the empty Config.Stage encodes.
```

### `go/internal/core/build_floor_reviewer.go:179` — above `v := docsfloor.Evaluate(cfg, docsfloor.Input{`

```text
// Label with the blocking-grade classifier (docsfloor.IsArchitectureClass):
// it drops test-only diffs — which document nothing and were the WARN's main
// false positive — and picks up the trust-kernel, new-package and phase-spec
// surfaces the broad label misses. The VERDICT stays WARN (ADR-0077): only
// the precision of "is this architecture" improves here.
```

### `go/internal/core/build_floor_reviewer.go:211` — above `func changedPackageFloorChecks(ctx context.Context, in ReviewInput, paths []string) []string {`

```text
// changedPackageFloorChecks reuses the EXACT selfcheck machinery the advisory
// post-build binding runs
// (changedWorktreePaths → changedGoTestPackages → runBuildSelfCheck with the
// real go-test runner) — the flip from advisory to rejecting is the whole
// change (the cycle-1008 smoking gun: the artifact recorded the failure and
// nothing acted on it). Returns one line per failing package. Any inability
// to run (no worktree, no packages) is GREEN — fail-open, downstream gates
// stay armed.
```

### `go/internal/core/build_floor_reviewer.go:231` — above `taggedFails := addedTaggedTestFailures(ctx, in, moduleDir)`

```text
// Added tag-gated packages are invisible to the default-context run above
// (no GoFiles in that context) and were first executed at SHIP until
// 2026-09-14 (cycle 1679). They run here under their own tags regardless
// of whether anything default-visible changed.
```

### `go/internal/core/build_floor_reviewer.go:259` — above `namingFails := apicoverNamingFailures(ctx, moduleDir, enforced, paths)`

```text
// The apicover parity class (5 live instances: 3 main REDs, a console PR
// red, and cycle-1022's invisible audit override): an ENFORCED changed
// package with an unnamed export dies at HANDOFF, not at audit/CI.
```

### `go/internal/core/build_floor_reviewer.go:331` — above `func floorFailureDiagnostic(output string) string {`

```text
// floorFailureDiagnostic trims a failing package's `go test` output to the
// TAIL, not the head.
//
// Cycle-1268 is the record of why the direction matters: the floor kept
// output[:400], but `go test` writes its `--- FAIL` lines, panics and stack
// traces at the END, after whatever the package logged on the way. The recorded
// reason was therefore 400 bytes of repeated `[engine] WARN: Deps.TokenResolver
// is nil` and nothing else — the operator was handed noise from exactly the
// region where the diagnosis was not. Keeping the tail costs the same bytes and
// carries the verdict lines.
```

### `go/internal/core/build_floor_reviewer.go:415` — above `changedByDir := changedFileBasenamesByDir(moduleDir, dirs, changedPaths)`

```text
// Diff-scope (cycle-1048): only violations in files THIS change touched
// hard-fail; a touched package's pre-existing debt WARNs in the report.
```

### `go/internal/core/build_floor_reviewer.go:466` — above `if release, lerr := verifylock.Acquire(ctx, filepath.Dir(moduleDir), os.Stderr); lerr == nil {`

```text
// ADR-0080 P1: the coverage run doubles as the enforced packages'
// selfcheck — a full go-test execution, host-wide single-flight for the
// same reason as the EGPS suite (batch-16 contention false-reds). A lock
// failure degrades to unserialized, never to skipped verification.
```

### `go/internal/core/build_floor_reviewer_test.go:3` — above `import (`

```text
// build_floor_reviewer_test.go — the shift-left half of the 2026-07-21
// operator directive ("shift deterministic gate to the front as part of build
// phase verification"): the build deliverable is REJECTED while the changed
// packages' deterministic self-check fails, so the EXISTING E2 correction
// ladder fixes it in-phase — instead of the defect surfacing at a later gate
// (or worse: cycle-1008's builder recorded ./cmd/evolve failing in
// build-selfcheck.json and handed off anyway; the FAIL then cost 4 more
// phases + the cycle).
```

### `go/internal/core/build_persona_budget_check.go:3` — above `import (`

```text
// build_persona_budget_check.go — the in-lane half of the persona line-budget
// gate (inbox item `persona-budget-inlane-gate`; third instance of the
// "per-cycle-gate ≠ repo-wide-gate" class after warnship-apicover-ci-gap and
// acs-predicate-compile-gate-at-build-exit).
//
// The gap: `changedPackageFloorChecks` derives its test set from
// `changedGoTestPackages`, which keeps ONLY paths matching `go/**.go`. A lane
// that grows `agents/evolve-*.md` past the 751-line budget pinned by
// go/internal/prompts' TestPersonaStopCriterionDedupe_CombinedLineCountReduced
// therefore yields ZERO packages, hits that engine's `len(pkgs) == 0 → nil`
// early return, and hands off green — the breach lands on main's CI after
// ship, reddening the build for every concurrent lane on the branch (observed
// twice on 2026-07-23). The post-ship visibility half was already closed by
// adding `agents/**` to the go CI path filter (0a732192); this closes the
// pre-handoff half.
//
// Shape follows RemovalClaimFailures deliberately: composed OUTSIDE
// changedPackageFloorChecks in DefaultBuildFloorChecks, for exactly the reason
// that file's comment already documents — a check that must run regardless of
// what changedGoTestPackages derives cannot live behind its early return.
//
// The gate is the CONJUNCTION (a persona doc changed AND the prompts package is
// red), never path-presence alone: touching a persona doc is not itself a
// violation, and a floor that rejected on the path would false-block every
// legitimate persona edit. Fail-open on plumbing (no worktree, no changed
// paths) per this floor's documented policy — downstream gates stay armed.
```

### `go/internal/core/build_persona_budget_check_test.go:3` — above `import (`

```text
// build_persona_budget_check_test.go — permanent regression cover for the
// cycle-1101 in-lane persona-budget gate. The cycle-scoped ACS predicates
// (go/acs/cycle1101) exercise the same behaviour against a real go module and
// vanish with the cycle; these keep the CONJUNCTION (persona path changed AND
// internal/prompts red) pinned for every future cycle, hermetically: the
// go-test subprocess is replaced at the buildSelfCheckRunner seam so no test
// here spawns `go test`.
```

### `go/internal/core/build_placeholder_check.go:12` — above `var placeholderTokenRE = regexp.MustCompile('\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*_PLACEHOLDER\b')`

```text
// placeholderTokenRE matches an UPPER_SNAKE template token ending in
// _PLACEHOLDER (FULLSUITE_PLACEHOLDER, ACS_RESULT_PLACEHOLDER): the shape a
// builder's own report scaffolding uses for a slot it must overwrite with an
// executed result (no shipped persona emits the token — cycle 1679 minted it
// itself — so the grammar here is the only home of the rule). Prose mentioning a "placeholder", a lowercase
// identifier, or a token that merely starts with PLACEHOLDER is not a slot.
```

### `go/internal/core/build_placeholder_check.go:20` — above `func PlaceholderTokenFailures(_ context.Context, in ReviewInput) []string {`

```text
// PlaceholderTokenFailures is the build handoff floor's deterministic check
// that no template slot survived into build-report.md. Cycle 1679 round 5
// handed off "`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER" under a
// heading promising every number was executed; the floor passed it and the
// audit spent a round naming it (M3 verification-gap). One failure per token
// per line, naming the report line, so the correction ladder's instruction is
// exact. No report → nothing to refuse (the deliverables gate owns absence).
```

### `go/internal/core/build_placeholder_check_test.go:11` — above `func TestPlaceholderTokenFailures_RefusesAnUnsubstitutedTemplateToken(t *testing.T) {`

```text
// Cycle 1679 (2026-09-15): round 5's build-report.md carried the literal
// template token FULLSUITE_PLACEHOLDER where the mandatory full-suite result
// belongs, inside a section headed "every number below was executed this
// round"; the handoff floor passed it and the audit spent a round naming it
// (M3 verification-gap). An unsubstituted template token is a deterministic
// fact about the report, so the floor refuses it before handoff.
```

### `go/internal/core/build_removal_check.go:3` — above `import (`

````text
// build_removal_check.go — deterministic truth-check for the removal claims a
// build report makes about its own worktree (inbox item `tdd-topn-binding-gate`,
// acceptance criterion 2: "a build report claiming a removal that did not happen
// fails build-selfcheck deterministically").
//
// The cycle-660 incident this closes: build-report.md asserted that orphaned RED
// scaffolds were "already removed by a concurrent actor" while the files were
// still sitting in the worktree, and the false claim passed review undetected —
// the prose was the only evidence and nobody checked the tree. A claim about
// tree state is machine-checkable, so it is checked by machine, not read.
//
// Prose is deliberately NOT parsed: a natural-language matcher on "removed"
// would false-block honest reports and is exactly the proxy-signal failure this
// gate exists to end. The claim surface is a structured fenced ```json block
// carrying a "removedPaths" array (mirroring the fenced-JSON handoff contract
// topngate already uses); anything else is invisible to this check.
//
// The filesystem is only half of a tracked deletion: a path absent on disk but
// still present in the Git index returns after a fresh checkout. Every Git
// ambiguity remains fail-open, so the floor cannot false-block a build over a
// missing repository or its own plumbing.
````

### `go/internal/core/build_removal_check_index_test.go:3` — above `import (`

```text
// build_removal_check_index_test.go — RED contract for the cycle-1591 task
// `retire-stale-retro-prompt-delivery-stall`.
//
// The incident: the live inbox record
// `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json` has been
// "retired" more than once by a filesystem-only removal — a plain delete, or a
// move into the .gitignore'd `.evolve/inbox/processed/` destination — with no
// matching `git rm`. The path stayed in the Git INDEX, so the next fresh
// checkout restored it and the item reopened as live, burning an empty lane.
//
// `RemovalClaimFailures` is the build-floor gate that exists to catch a false
// "I removed this" claim, but it asks only the worktree filesystem
// (`os.Stat`, build_removal_check.go:60-62). A path absent from disk yet still
// tracked reads to it as an honest removal, so the false retirement passed the
// floor. The claim is about the state of the REPOSITORY, not of one working
// tree, so the index is the second half of the truth check.
//
// Contract under test (production change is Builder's job — none of it exists
// at RED time): when a claimed path is absent from the worktree filesystem but
// still present in that worktree's Git index, RemovalClaimFailures must return
// exactly one failure naming the path. Every existing disposition is preserved:
// an untracked absent path is still an honest removal, a non-repo worktree
// still fails open, and a path still on disk still produces exactly one
// failure (never two).
```

### `go/internal/core/build_removal_check_index_test.go:56` — above `func trackedThenDeleted(t *testing.T, claimed string) ReviewInput {`

```text
// trackedThenDeleted builds the exact incident shape: a claimed path that is
// committed to the fixture worktree's index and then removed from disk ONLY.
```

### `go/internal/core/build_removal_check_index_test.go:110` — above `func TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce(t *testing.T) {`

```text
// AC3 (edge — no double-counting): a path that is BOTH still on disk and still
// tracked is one false claim, not two. The cycle-660 message stays the one the
// operator reads for the on-disk case.
```

### `go/internal/core/build_removal_check_test.go:3` — above `import (`

````text
// build_removal_check_test.go — RED contract for the cycle-1076 task
// `build-selfcheck-removal-claim-check` (inbox item `tdd-topn-binding-gate`,
// acceptance criterion 2: "a build report claiming a removal that did not
// happen fails build-selfcheck deterministically").
//
// The cycle-660 incident this pins: build-report.md asserted that orphaned RED
// scaffolds were "already removed by a concurrent actor" while the files were
// still present in the worktree, and the false claim passed review undetected.
//
// Contract under test (production code is Builder's job — none of it exists at
// RED time):
//
//	func RemovalClaimFailures(ctx context.Context, in ReviewInput) []string
//
// It is a BuildFloorCheckFn: it reads the build report from
// <workspace>/build-report.md (falling back to
// <workspace>/deliverables/build-report.md), parses every fenced ```json block
// for an object carrying a "removedPaths" string array, and returns one failure
// line per claimed path that STILL EXISTS under the worktree. Every ambiguity
// is fail-open (nil): no workspace/worktree, no report, no parseable block,
// malformed JSON, empty list, or a path escaping the worktree. It must also be
// composed into DefaultBuildFloorChecks so a false claim actually REJECTs the
// build deliverable — the wiring proof, not just the unit.
````

### `go/internal/core/buildleak_recover_test.go:109` — above `func realWorktree(t *testing.T) (string, string) {`

```text
// buildleak_recover_test.go — Option A for the cycle-160 incident
// (docs/operations/multicli-validation-run-2026-05-31.md §"Implementation plan for A").
//
// A non-Claude builder (agy/codex in tmux) is not bound by the Claude-only role-gate,
// and the OS sandbox is off on nested-macOS, so it can write its build output to the
// MAIN tree instead of its worktree. recoverBuildLeak relocates that leaked output into
// the worktree (staging ONLY the relocated paths, so the auditor's `git diff HEAD` sees
// it without pollution) and restores the main tree.
//
// These tests use a REAL `git worktree add` (not two independent repos) so the worktree
// shares the main repo's tracked directory structure — the production topology where an
// earlier independent-repo test masked a directory-rename bug.
```

### `go/internal/core/buildleak_recover_test.go:141` — above `func TestRecoverBuildLeak_RelocatesIntoRealWorktree(t *testing.T) {`

```text
// Relocate a leaked NEW file written into an EXISTING tracked directory in main —
// the real cycle-160 shape (agy wrote go/internal/phases/backfill/* into main).
```

### `go/internal/core/buildleak_recover_test.go:196` — above `func TestRecoverBuildLeak_RelocatesTrackedEditWhenWorktreeClean(t *testing.T) {`

```text
// A modified TRACKED file leaked into main, where the worktree has NOT independently
// touched that file (its copy is still at HEAD), is the real cycle-162 shape: a
// non-Claude builder edited an existing tracked source file (orchestrator.go) in the
// MAIN tree instead of the worktree. The builder's real work must be PRESERVED — the
// leaked content is relocated into the worktree (overwriting its HEAD copy) and the
// main tree restored. Covers the staged-only ("M ") case that `git checkout -- p`
// would no-op.
```

### `go/internal/core/buildleak_recover_test.go:289` — above `func TestRecoverBuildLeak_DiscardsRebuiltArtifactEvenWhenWorktreeClean(t *testing.T) {`

```text
// A rebuilt tracked release binary (go/evolve) leaked into main must be DISCARDED even
// when the worktree's copy is at HEAD — relocating it would commit binary drift
// (cycle-153). go/evolve is re-committed only by the release pipeline, never a cycle.
```

### `go/internal/core/buildleak_recover_test.go:319` — above `func TestRecoverBuildLeak_SkipsEvolveRuntimeStateAndNestedWorktreeDir(t *testing.T) {`

```text
// recoverBuildLeak must SKIP the orchestrator's own runtime state under .evolve/ —
// it is never build output. In the live repo .evolve/ is gitignored (invisible to
// `git status`); a minimal fixture without that .gitignore exposed recoverBuildLeak
// (a) relocating .evolve/ledger.tip into the worktree (pollutes the audit diff) and
// (b) CHOKING on the nested worktree dir .evolve/worktrees/cycle-1/ — `git status
// -uall` reports a nested working tree as a bare directory, which moveFile cannot
// relocate, so it returned false and aborted the cycle (415a9a7 regression caught by
// the e2e ship-path tests). Both must be skipped; the cycle proceeds.
```

### `go/internal/core/buildleak_recover_test.go:352` — above `func TestRecoverBuildLeak_SkipsNestedEvolveRuntimeState(t *testing.T) {`

```text
// Issue #11 (cycle-176): guard hooks run with cwd set to subdirectories and write
// NESTED `<subdir>/.evolve/guards.log`. The top-level-only skip missed these, so
// recoverBuildLeak relocated them and the gitignored `git add` failed → batch abort.
// Nested `.evolve/` paths (path contains `/.evolve/`) must be skipped like top-level.
```

### `go/internal/core/buildleak_recover_test.go:361` — above `for _, d := range []string{"go", "go/internal/phases"} {`

```text
// Nested .evolve/ runtime state under tracked subdirs (mirrors cycle-176).
```

### `go/internal/core/buildleak_recover_test.go:410` — above `func TestRecoverBuildLeak_RelocatesUntrackedEvalDeliverable(t *testing.T) {`

```text
// cycle-268 (and the cycle-262 carryover the loop kept dying on): `.evolve/`
// DELIVERABLE locations — eval files, phase configs, profiles, the tracked
// prefix-scope/policy configs — are repo content that legitimately ships
// with cycles, not runtime state. The blanket `.evolve/` skip made any agent
// that wrote one into the MAIN tree unrecoverable (relocation refused → the
// tree-diff guard aborted the cycle; cycle-268's tdd died writing its OWN
// eval). Deliverable subpaths now relocate exactly like any other repo path;
// runtime state (runs/, worktrees/, ledger, state.json…) stays skipped —
// pinned by the two Skips tests above.
```

### `go/internal/core/buildleak_recover_test.go:443` — above `func TestRecoverBuildLeak_RelocatesTrackedEvolveConfigEdit(t *testing.T) {`

```text
// The cycle-262 shape: a TRACKED `.evolve/` config edited in main (worktree
// clean for that path) must relocate via the existing tracked-edit branch.
```

### `go/internal/core/bytestability_test.go:18` — above `var legacyStateKeys = []string{`

```text
// legacyStateKeys is the pre-Track-C state.json surface (frozen 2026-06-11).
```

### `go/internal/core/bytestability_test.go:52` — above `"worktree_base_sha",`

```text
// cycle-156 resume parity
```

### `go/internal/core/bytestability_test.go:53` — above `"audit_fail_reasons",`

```text
// ADR-0072 diagnosed-downgrade signal (cycles 930-932 false-HALT fix)
```

### `go/internal/core/bytestability_test.go:54` — above `"failed_at",`

```text
// ADR-0072 S4 dossier non-progress counters (per-cycle history mirror)
```

### `go/internal/core/bytestability_test.go:55` — above `"ship_fail_reasons",`

```text
// ADR-0072 ship-phase explained-failure carrier (pipeline-defect-pipeline-blocker, cycle-1329)
```

### `go/internal/core/bytestability_test.go:56` — above `"bookkeeping_regrade_attempted",`

```text
// once-per-cycle bound of the retro→audit bookkeeping regrade (2026-08-10 investigation)
```

### `go/internal/core/bytestability_test.go:57` — above `"audit_repair_attempts",`

```text
// in-cycle audit-repair loop bound (wave-3 cycles 1572/1573/1574)
```

### `go/internal/core/bytestability_test.go:59` — above `"audit_dispatches",`

```text
// audit round-supersession index; dispatch-persisted so a crashed round retires on resume (cycle-1603)
```

### `go/internal/core/bytestability_test.go:65` — above `"shipped",`

```text
// ADR-0100 PR-3: this cycle's own ship latch, persisted so a pause/resume after Ship keeps it; read by the outcome label + post-ship observer degrade on both roots,
```

### `go/internal/core/bytestability_test.go:66` — above `"ship_recovery_code",`

```text
// 2026-09-15: the ship-error recovery marker the standing-audit-findings brief keys on (research F16)
```

### `go/internal/core/bytestability_test.go:67` — above `"audit_decline_reason",`

```text
// 2026-09-15: the audit-fail decline marker a retro-routed re-entry keys on (research F19)
```

### `go/internal/core/carryforward_filter.go:1` — above `package core`

```text
// carryforward_filter.go — deterministic, zero-LLM carry-forward candidate
// screen (cycle 962, inbox item scout-carryforward-real-cherrypick-filter,
// weight 0.94, campaign merge-efficiency-2026-07).
//
// The fleet-rebase carry-forward candidate selection was driven by the
// LLM scout/triage phase running raw git with a bare 1-arg `git merge-tree`
// as its conflict oracle. That legacy form is NOT a 3-way merge, produces no
// index, and reports "clean" on real conflicts — and it has no
// functional-duplicate screen — so already-superseded orphan `cycle-*`
// branches were mis-selected and burned operator-prioritized rebase cycles
// (cycle-826 Opus auditor reproduction: 6 conflict indicators / 36 markers on
// a merge-tree-"clean" candidate).
//
// The root-cause fix is a deterministic Go filter the phase can call instead
// of eyeballing raw git: a REAL 3-way merge dry-run (`git merge-tree
// --write-tree`, which writes only to the object DB — HEAD/index/worktree are
// untouched) plus an is-ancestor / patch-id supersession screen. All git runs
// through the gitCapture seam so it is fakeable in fast-tier tests.
```

### `go/internal/core/carryforward_filter.go:83` — above `func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error) {`

```text
// ClassifyFleetRebaseCandidate is the deterministic, zero-LLM pre-screen for the
// fleet-rebase recovery path. It REUSES CarryforwardCandidateLandable (giving that
// otherwise-inert cycle-962 surface its first production caller) and splits its
// false result into the two outcomes that must be handled differently:
//
//	FleetRebaseClean         → landable (clean 3-way merge, not superseded)
//	FleetRebaseAlreadyLanded → not landable BECAUSE superseded (short-circuit)
//	FleetRebaseConflict      → not landable BECAUSE a genuine conflict (→ debugger)
//	err                      → git-infrastructure failure only; never masked as a verdict.
//
// The naive "landable==false ⇒ already landed" reading is WRONG: landable is false
// for BOTH a superseded candidate AND a real conflict, so a second supersession
// screen disambiguates them — mislabelling a conflict as landed would silently drop
// real overlapping work.
```

### `go/internal/core/carryforward_filter_test.go:3` — above `import (`

```text
// carryforward_filter_test.go — fast-tier coverage for the deterministic
// carry-forward candidate screen (cycle 962). These are white-box tests: core
// cannot import test/fixtures (that package imports core), so they drive the
// package gitRunner seam directly through a SCRIPTED fake that answers each git
// subcommand independently — the single-canned-response gitRec is too blunt for
// functions that branch on distinct exit codes across is-ancestor / cherry /
// merge-tree.
//
// The assertions probe INTENT, not surface: (1) a genuine 3-way conflict is
// (false, nil) and NEVER an error, and (2) the supersession screen runs BEFORE
// the merge dry-run so an already-landed candidate is rejected without the
// merge-tree that would mask it as "clean".
```

### `go/internal/core/carryover_fingerprint_test.go:3` — above `import (`

```text
// carryover_fingerprint_test.go — RED contract for the carryoverTodos P0-flood
// dedupe (2026-08-10 investigation, agent C): 124 of 254 live entries were
// near-identical per-FAIL generic P0s ("cycle N failed during audit: …"),
// distinct only by the cycle number baked into their IDs — carryoverTodoExists
// dedupes by ID, so every FAIL minted a fresh duplicate, and the 20-slot
// router window (phase_advisor) was 100% saturated by them, permanently
// shadowing memo/product carryovers. Dedupe key: the Action text with cycle
// tokens normalized — same defect, different cycle ⇒ ONE entry.
```

### `go/internal/core/carryover_lifecycle.go:3` — above `import (`

```text
// carryover_lifecycle.go — unit 03 (ADR-0103, design decomposition/03-carryover-lifecycle.md):
// the orchestrator's seam onto the carryover unit. Every old caller keeps its
// spelling: the persist and mint facades are Orchestrator methods (so the
// lifecycle reaches the orchestrator's Signal Center), the exported
// package-level facades survive for ship, the ACS-named tests and the fifteen
// direct test call sites on the Null Object, and the unexported facades keep
// alloc.go's union, the adoption cap and the failure summary in place.
```

### `go/internal/core/carryover_lifecycle.go:109` — above `func truncateRunes(s string, max int) string { return carryover.TruncateRunes(s, max) }`

```text
// truncateRunes is the advisor prompt's rune cap (the goal section and the
// catalog card hints) and the remediation title's — carryover.TruncateRunes,
// the third cap the unit enumerates beside CapRunes and Summary (ADR-0103
// unit 03b); a facade so phase_advisor.go and task_recall.go keep their spelling.
```

### `go/internal/core/carryover_lifecycle_test.go:3` — above `import (`

```text
// carryover_lifecycle_test.go — unit 03 (ADR-0103): the orchestrator's seam
// onto the carryover unit — the accessor pair, the persist facade reading the
// store at call time, the RMW branch's golden bytes, the consumer pin of the
// priority vocabulary, the one-construction guard and the closeout order at
// the real finalizeCycle.
```

### `go/internal/core/carryover_lifecycle_test.go:74` — above `func TestWriteFailureLearningState_RMWBranchIsByteIdenticalToTheGolden(t *testing.T) {`

```text
// ADR-0103 item 4: the RMW branch's bytes, captured on the pre-extraction code
// (f7c2d85d) with this exact fixture — the first core test to drive it.
```

### `go/internal/core/carryover_lifecycle_test.go:92` — above `func TestCarryoverPriorityRank_RanksTheUnitsVocabulary(t *testing.T) {`

```text
// The consumer pin of the unit's priority vocabulary: core's consts project
// the unit's (the advisor's rank table — the other consumer — pins the same
// vocabulary from its own package since ADR-0103 unit 04).
```

### `go/internal/core/carryover_lifecycle_test.go:153` — above `func TestFinalizeCycle_CloseoutKeepsTheThreeStepOrder(t *testing.T) {`

```text
// The cycle-1538 pin at the seam: through the REAL finalizeCycle, a memo that
// re-supplies a triage-dropped id cannot resurrect it (retire runs after both
// merges), and the prescription merge runs too.
```

### `go/internal/core/carryover_merge_test.go:3` — above `import (`

```text
// carryover_merge_test.go — RED contract for cycle-667 task
// `chronicle-s4-carryover-orphan-merge` (triage `## top_n`, weight 0.92).
//
// ORPHAN BEING FIXED: evolve-memo (the PASS-branch scribe, dispatched post-ship)
// writes <workspace>/carryover-todos.json every PASS cycle, and the retro path
// writes the same file on FAIL — but NO Go code ever reads it. The PASS-branch
// learning channel is fire-and-forget: the queued todos never reach
// state.json:carryoverTodos, so they never surface to the next cycle's planner.
// grep confirms zero readers of any workspace carryover-todos.json.
//
// FIX (Builder authors go/internal/core/carryover_merge.go + wires the call site
// in finalizeCycle): a cycle-terminal hook, beside the persistCycleEndState
// PASS-parity point, that — if <workspace>/carryover-todos.json exists —
// tolerant-decodes [{id, action, priority, evidence_pointer}] (skipping entries
// missing id/action), caps the action via the capRunes idiom, maps priority,
// stamps FirstSeenCycle + a future ExpiresAt (so failurelog.PruneExpiredCarryoverTodos
// can converge), and merges into state.CarryoverTodos via the existing
// mergeCarryoverTodos (dedup by id ⇒ idempotent on re-entry).
//
// These tests are authored by the TDD engineer and are RED now (they will not
// even compile until MergeWorkspaceCarryover exists — a valid RED per the
// compile-failure rule). The Builder must make them GREEN by adding production
// code ONLY; it must NOT modify this file.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : TestRunCycle_MergesMemoCarryoverTodosIntoState — the real
//     terminal path (finalizeCycle) persists the memo todos. This is the
//     load-bearing anti-no-op signal: a helper that exists but is never wired
//     into the terminal hook leaves the orphan unfixed and FAILS here.
//   - Semantic       : TestMergeWorkspaceCarryover_DedupesById — re-entry is
//     idempotent, never duplicates an id.
//   - Edge           : TestMergeWorkspaceCarryover_CapsActionRunes — an oversized
//     action is bounded, so a memo todo cannot bloat every future router prompt.
//   - Negative/OOD   : TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails —
//     malformed JSON and id/action-less entries are tolerated (WARN, no panic,
//     no fatal), never aborting the cycle-terminal hook.
//   - Semantic       : TestMergeWorkspaceCarryover_StampsExpiryForPrune — every
//     merged todo carries a future RFC3339 ExpiresAt so the loop-start prune
//     ages the array out instead of letting it grow unboundedly.
```

### `go/internal/core/carryover_merge_test.go:55` — above `func writeMemoCarryover(t *testing.T, dir, contents string) {`

```text
// writeMemoCarryover writes a memo-shaped carryover-todos.json (the cycle-646
// live schema: an array of {id, action, priority, evidence_pointer}) into dir.
```

### `go/internal/core/carryover_prompt_order_test.go:3` — above `import (`

```text
// carryover_prompt_order_test.go — RED tests (cycle 507, task
// fix-carryover-prompt-truncation-order). White-box (`package core`) because
// writeCarryoverTodos is unexported.
//
// writeCarryoverTodos (phase_advisor.go) is the SOLE injection site for
// carryoverTodos into any router/advisor prompt. It caps the rendered COUNT at
// maxCarryoverTodosInPrompt (20) but slices `todos[:20]` in on-disk INSERTION
// order (oldest-first). With 65 entries on disk today, the prompt permanently
// shows only ~cycles 366-402 and hides everything newer behind "... N more
// omitted" — including the two most severe still-open items (cycle 502
// SELF_SHA_TAMPERED, cycle 505 evolve-bin leak). Routing/advisor decisions have
// therefore been made blind to the last 100+ cycles of carryover.
//
// The fix: when the array exceeds the cap, render the HIGHEST-PRIORITY /
// MOST-RECENT entries, not a naive insertion-order prefix. These tests pin the
// observable outcome (severe+recent survives the cut; malformed priority is
// safe; the cap boundary is exact), not a specific comparator. RED now:
// insertion-order slicing hides the tail entry. Do NOT modify this file —
// implement the ordering in writeCarryoverTodos.
```

### `go/internal/core/carryover_prompt_order_test.go:31` — above `func TestWriteCarryoverTodos_SevereRecentSurvivesTheCut(t *testing.T) {`

```text
// AC (positive, the headline bug): given more than the cap, with the single
// most severe + most recent entry placed LAST in insertion order (exactly where
// today's todos[:20] slice drops it), the rendered prompt MUST still include it.
// A no-op insertion-order slice fails: the P0/cycle-505 entry is entry #21 and
// gets omitted.
```

### `go/internal/core/carryover_prompt_order_test.go:47` — above `const critical = "cycle-505-failed-changelog-sync"`

```text
// The 21st entry: the newest and highest priority — the one insertion-order
// slicing silently hides. This is the cycle-505 evolve-bin leak carryover.
```

### `go/internal/core/carryover_retire_test.go:3` — above `import (`

```text
// carryover_retire_test.go — RED contract for cycle-1440 task
// `carryover-pass-retirement`.
//
// Defect: mergeCarryoverTodos (failure_learning.go) unions disk+incoming and
// dedupes by ID, but NOTHING ever removes an entry. A carryover todo whose work
// actually shipped therefore persists forever, saturating the router prompt's
// 20-slot carryover window with already-done work (the 2026-08-10 investigation
// found 124 of 254 live entries were stale duplicates of a few classes).
//
// Contract under test (not yet implemented — these tests MUST fail RED until
// Builder adds the PASS-closeout deletion path):
//
//	RetireCarryoverTodos(todos, committedIDs) []CarryoverTodo
//
// retires (a) every entry whose ID is in the committed set, and (b) every entry
// sharing a retired entry's cross-cycle Action fingerprint
// (carryoverActionFingerprint) — the per-cycle re-mints of the SAME class that
// the ID-keyed dedupe never collapsed. Everything else survives untouched, in
// order.
```

### `go/internal/core/carryover_todo_length_test.go:3` — above `import (`

```text
// carryover_todo_length_test.go — cycle-488 RED tests for
// tighten-carryover-todo-creation-length (Task 1).
//
// Two creation paths write a router.CarryoverTodo.Action that later renders
// into EVERY router/advisor prompt (phase_advisor.writeCarryoverTodos). Today
// they are asymmetric and lossy:
//
//   1. recordFailureLearning (failure_learning.go ~L246) prepends the fixed
//      58-byte boilerplate "Review the failed cycle learning and fix before
//      retrying: " to every todo — pure repetition the router prompt's own
//      section header already states.
//   2. ApplyDefectsAsCarryoverTodos (failure_learning.go ~L535) applies NO
//      length cap to `defect`, unlike its sibling failureLearningSummary
//      (maxFailureLearningSummaryChars = 500). A single unbounded audit-gate
//      diagnostic (e.g. a long strings.Join(offenders, "; ")) can inject an
//      arbitrarily large Action that bloats every future router prompt.
//
// These tests encode the acceptance criteria. They MUST fail before the Builder
// touches failure_learning.go (RED). The Builder makes them GREEN by dropping
// the prefix and bounding the defect text — WITHOUT modifying this file.
```

### `go/internal/core/carryover_triage_drop_reproduction_test.go:10` — above `func TestFinalizeCycle_RetiresTriageDroppedCarryover(t *testing.T) {`

```text
// TestFinalizeCycle_RetiresTriageDroppedCarryover reproduces cycle 1538: a
// no-ship WARN cycle records a stale carryover in triage-decision.json, but the
// terminal path persists that same carryover for the next cycle.
```

### `go/internal/core/carryover_ttl_stamp_test.go:3` — above `import "testing"`

```text
// carryover_ttl_stamp_test.go — RED test (cycle 507, task
// prune-stale-carryover-todos) for the CREATION-TIME half of the TTL contract.
//
// state.json:carryoverTodos grows unboundedly (65 entries / 26,601 bytes today,
// cycles 366→506) because CarryoverTodo — unlike its structurally-parallel
// sibling failedApproaches — carries NO expiry and has NO prune path. The fix
// mirrors the sibling: stamp a TTL when the todo is created, then prune expired
// entries at loop start (the prune half lives in
// internal/failurelog/prune_carryover_test.go).
//
// ApplyDefectsAsCarryoverTodos already receives a FailedRecord whose ExpiresAt
// is the 7-day stamp the failedApproaches path computes; a created carryover
// todo must INHERIT that stamp so the prune pass can age it out. Inheriting the
// existing stamp (rather than recomputing one) keeps the two arrays' TTL logic
// single-sourced (never_duplicate_centralize_via_design_patterns).
//
// References CarryoverTodo.ExpiresAt, which the Builder adds to
// cyclestate.CarryoverTodo. RED now (no such field → core test package fails to
// compile). Do NOT modify this file — implement the production seam.
```

### `go/internal/core/catalog_publisher_boundary_amplify_test.go:10` — above `type amplifyFakeStorage struct{}`

```text
// catalog_publisher_boundary_amplify_test.go — Test Amplifier (cycle-1429).
//
// Black-box adversarial tests for WithCatalogPublisher / CatalogPublisherWired
// and verdictInputsFor, authored from the exported-symbol contract only
// (`go doc ./internal/core WithCatalogPublisher`, `go doc ./internal/core
// Orchestrator`, plus the requiredSymbols signature block in this cycle's
// test-report.md), never from orchestrator.go / system_failure.go's control
// flow or the build diff.
//
// WithCatalogPublisher's own doc is explicit: "Nil is ignored, leaving the
// no-publish default (byte-identical legacy behavior)." That is a promise
// about a null input, and the null-input class is exactly what this phase
// is chartered to probe — including the combinatorial case (a nil option
// applied AFTER a valid one) the frozen TDD suite's
// TestRegisterMintedPhases_NoPublisherIsNoop does not exercise (that test
// covers "no option at all", not "nil option after a valid one").
//
// amplifyFakeStorage/amplifyFakeLedger are minimal port doubles satisfying
// this package's exported Storage/Ledger interfaces (per `go doc
// ./internal/core Storage` / `Ledger`) purely so NewOrchestrator can be
// constructed; none of their methods are exercised by these tests, which
// only probe post-construction predicates (never RunCycle).
```

### `go/internal/core/changed_worktree_paths_export_test.go:11` — above `func changedPathsRepoFixture(t *testing.T) string {`

```text
// RED contract for cycle-1150 / wire-docsfloor-verify-cli.
//
// The CLI self-check (`evolve phase verify build`) needs the same changed-path
// derivation the host-side docs-floor reviewer already uses. That logic lives
// in the unexported changedWorktreePaths (phase_bindings.go): re-implementing
// git-diff derivation in internal/cli/phasecmd would put two answers to "what
// did this cycle change?" in the tree, and the gate and the self-check would
// drift (the ADR-0034 no-drift invariant). This test pins the single source
// with a projection: one EXPORTED wrapper, same semantics.
```

### `go/internal/core/chokepoint_escape_test.go:1` — above `package core_test`

```text
// chokepoint_escape_test.go — ADR-0044 C1 invariant regression (inbox
// cycle-terminal-path-escapes-c1-chokepoint, weight 0.98; the cycle-492 escape).
//
// Contract: when RunCycle's bounded dispatch loop exhausts its iteration budget
// without reaching PhaseEnd (a transition-table cycle keeps re-selecting phases),
// the cycle MUST record an explicit terminal abort so cyclehealth.ClassifyOutcome
// classifies it FAILED_EXPLAINED — never the FAILED_UNEXPLAINED alarm bucket. The
// escape is a CYCLE-level failure (loud + diagnosable), never batch-fatal.
//
// Driven deterministically: WithMaxPhaseIterations caps the loop below the spine
// length, so an all-PASS cycle exits via the iteration bound (the exact escape
// path) instead of reaching ship→end. Shares the core_test harness (recStorage,
// fakeLedger, newRunners) from orchestrator_recovery_test.go.
```

### `go/internal/core/composition_carryforward.go:1` — above `package core`

```text
// composition_carryforward.go — wires the RUNG 0 composition-verdict writer
// into the live fleet-rebase recovery path (cycle 801, inbox weight 0.98,
// campaign merge-efficiency-2026-07).
//
// Ship's trivial-rebase carry-forward reader (internal/phases/ship/
// composition.go) and the ledger's kernel-recomputable writer
// (internal/adapters/ledger/composition.go) were built and unit-tested in
// cycle-786, but no production call site ever produced a composition-verdict
// entry: recoverFromShipError's clean fleet-rebase branch always fell
// through to a full re-audit. internal/core cannot import
// internal/adapters/ledger directly (ledger already imports core — an import
// cycle), so this wires the same Option-injected-closure seam core already
// uses for catalogRefresh/modelCatalogLookup/directivesProvider: the
// composition root (cmd/evolve) binds the closures to the real ledger
// adapter, and core stays adapter-agnostic.
```

### `go/internal/core/composition_carryforward_wired_test.go:1` — above `package core`

```text
// composition_carryforward_wired_test.go — cycle-804 TDD contract (inbox
// weight 0.98, wire-rung0-composition-writer-into-fleet-rebase).
//
// cycle-786/801 built the RUNG 0 composition-verdict writer (ledger),
// reader (ship), and core seam (composition_carryforward.go) — three
// Option-injected closures (WithCompositionSnapshot/GateRunner/
// VerdictWriter), all nil by default. Nothing in cmd/evolve ever binds
// them, so compositionCarryForward's nil-guard (composition_carryforward.go
// :96) always trips and every clean fleet rebase falls through to a full
// re-audit — the exact behavior the writer was built to eliminate.
//
// This file pins the "wired" check itself (mirrors FailureAdviserWired /
// failure_hook.go:56 — same AND-of-three-closures observability pattern
// already established for a different Option trio). The production
// composition root's use of it is pinned separately in
// cmd/evolve/cmd_cycle_composition_test.go.
//
// RED today: CompositionFastPathWired is undefined on *Orchestrator.
```

### `go/internal/core/composition_scoped_review.go:1` — above `package core`

```text
// composition_scoped_review.go — RUNG 2 of the merge ladder (campaign
// merge-efficiency-2026-07, inbox weight 0.987).
//
// When a fleet-rebase composition is NOT a clean rung-0 carry-forward (the
// composed patch-id drifts from the pre-rebase audited snapshot) and the
// changes still have overlapping file footprints, rung 2 dispatches a SCOPED
// merge-review of ONLY the conflicting hunks instead of a full re-audit
// (rung 3). This file holds the pure, adapter-agnostic building blocks that
// logic needs:
//
//   - IntersectingHunks  — computes the reviewer payload (conflict regions only)
//   - ScopedReviewVerdict — the compatible|entangled reviewer enum + routing
//   - ReverifyResolution  — rung-0 re-entry for LLM-proposed resolutions
//
// Overlap detection delegates to the CANONICAL, production-wired primitives in
// mergerung2.go (parseUnifiedDiffToHunks + rangesOverlap, OLD-side pre-image
// comparison) so this file and RunScopedMergeReview can never disagree on what
// "intersecting" means — the two rung-2 code paths share one overlap semantic
// (see mergerung2_consolidation_test.go for the regression contract). This file
// keeps the byte-payload shape and the exported names the cycle-942 ACS
// predicates pin; only the overlap engine is reconciled. LLM merges are
// suggestion-grade (MergeBERT-lineage evidence) — verified via patch-id,
// never trusted.
```

### `go/internal/core/console_lease.go:3` — above `import (`

```text
// console_lease.go — ADR-0080 S4, defense in depth for the tree-diff guard.
// The plane separation (S1) makes operator edits in the runtime tree
// structurally absent; this lease covers the residual case where an operator
// MUST touch the runtime tree while lanes run. It is the loud, bounded
// variant of the "operator allowlist" the ADR rejected as a primary design:
// exact paths only, hard expiry required, malformed = armed guard, and every
// waived path WARNs so a waiver is never silent.
//
// Written by `evolve console-lease` (cli/opscmd); ADOPTED once at cycle
// start (cyclerun.go) and consulted by the guard's classifier chain in
// cyclerun_review.go beside the mint/eval exemptions.
//
// LOCATION (review BLOCK): the lease lives in the git COMMON dir (the hub,
// resolved via plane.CommonGitDir) — outside every worktree. A lane phase
// writing ".evolve/console-lease.json" inside ANY checkout writes a file no
// reader honors. Adoption at
// cycle start means a mid-cycle write — even to the hub — cannot waive the
// cycle that wrote it.
```

### `go/internal/core/console_lease_test.go:3` — above `import (`

```text
// console_lease_test.go — ADR-0080 S4, defense in depth: an EXPLICIT,
// time-bounded operator lease can waive tree-diff attribution for named
// paths. Review BLOCK hardening pinned here: the lease is HUB-resident
// (git common dir — outside every worktree, unreachable by the .evolve/
// legitimacy blanket), adopted once at cycle start, exact-path, expiring,
// and LOUD per waiver.
```

### `go/internal/core/continuation_adopt_test.go:3` — above `import (`

```text
// continuation_adopt_test.go — ADR-0076 slice C, consume side. A new cycle
// whose scoped item carries a valid continuation provisions its worktree FROM
// the snapshot commit (standard provisioning path — work inherits via git
// history, never via dirty-state adoption), sets the review base to the
// ORIGINAL base so cumulative work is reviewed whole, and serves the prior
// attempt's findings to the build phase. Any validation failure falls back to
// fresh provisioning, loudly.
```

### `go/internal/core/continuation_baseadvance_test.go:3` — above `import (`

```text
// continuation_baseadvance_test.go — pins for the worktree-base heal at
// continuation adoption; incident narrative and degrade contract: see
// continuation_baseadvance.go. The heal fixture reproduces the live
// cycle-1365 shape (stale base predating a landed .gitignore carve-out).
```

### `go/internal/core/continuation_baseadvance_test.go:24` — above `if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/*\n!.evolve/evals/\ngo/bin/\n"), 0o644); err !…`

```text
// Main advances with the cycle-1365-shape fix: a .gitignore carve-out.
// The real #418 ladder shape: parent excluded by glob (not by directory,
// which would make re-inclusion impossible), evals carved back in.
```

### `go/internal/core/continuation_decline_release_test.go:3` — above `import (`

```text
// continuation_decline_release_test.go — pins the decline-release half of the
// 2026-08-10 absorbing-FAIL fix: when adoption REJECTS a stale binding
// (snapshot landed / worktree gone), the orchestrator must RELEASE that
// registry binding — otherwise the root-owned entry outlives its manifest and
// the defect-ledger gate's out-of-band check auto-FAILs every future lane on
// that scope (cycles 1412/1418). The release is orchestrator-side, so the
// gate's cycle-1285 anti-tamper block (workspace manifest deleted while a
// LIVE binding exists ⇒ blocked) is untouched — deletion by an agent still
// blocks; declination by the adopter now releases.
```

### `go/internal/core/continuation_lanescope_test.go:3` — above `import (`

```text
// continuation_lanescope_test.go — ADR-0076 slice C, G2 (cycle-1104). G1 binds
// preserved work to an INBOX CLAIM. A lane whose scope came from the wave
// planner has no claim file, so cycle-1078's FAIL had nothing to stamp and its
// snapshot was orphaned. G2 adds the second scope-identity class: the preserve
// decision ALSO registers the manifest under the lane's todo ids (the
// authoritative <workspace>/lane-scope.json pin), and adoption resolves claims
// first, lane scope second.
//
// Everything here rides the EXISTING gate: only work the carry-forward screen
// classifies Clean is registered, and every registry failure is a WARN that
// never fails finalization.
```

### `go/internal/core/continuation_lanescope_test.go:198` — above `func TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim(t *testing.T) {`

```text
// TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim — the full
// cycle-1078 story, end to end and through the production resolver: a FAILed
// lane-scope-only attempt registers its snapshot; a later cycle scoped to the
// same todo id — with NO inbox item and NO processing claim in the repo at all —
// re-seeds its worktree from that snapshot and builds on the preserved work.
```

### `go/internal/core/continuation_snapshot_test.go:3` — above `import (`

```text
// continuation_snapshot_test.go — ADR-0076 slice C (C1/C2): at the preserve
// decision a FAILed cycle's dirty worktree is SNAPSHOT-COMMITTED onto its
// cycle branch (an immutable ref — adoption never trusts mutable dirty state)
// and a continuation manifest is stamped into the workspace, gated on the
// carry-forward screen classifying the snapshot Clean against main.
```

### `go/internal/core/continuation_snapshot_test.go:170` — above `func TestAbnormalEpilogue_StampsContinuation(t *testing.T) {`

```text
// TestAbnormalEpilogue_StampsContinuation pins the G1 gap the FIRST live FAIL
// exposed (cycle-1078, 2026-07-23): a review-gate rejection exits RunCycle via
// the ERROR path, which never reaches finalizeCycle — the worktree was
// preserved but no continuation manifest was written, so the resumption
// machinery had nothing to bind. The unskippable abnormal epilogue must stamp
// too (idempotent with the finalize-path stamp; runs after the failure digest
// so FindingsPath has content).
```

### `go/internal/core/continuation_stamp.go:3` — above `import (`

```text
// continuation_stamp.go — ADR-0076 slice C, produce side. At the preserve
// decision (finalizeCycle, FAIL verdict) the cycle's dirty worktree is
// snapshot-committed onto its cycle branch and a continuation manifest is
// written into the workspace — IF the carry-forward screen classifies the
// snapshot Clean against main. The inbox mover later stamps released items
// from this manifest, transactionally with the release; the next claim adopts
// the snapshot ref. Everything here is best-effort and LOUD: a salvage failure
// must never fail cycle finalization, and must never be silent.
```

### `go/internal/core/continuation_stamp.go:218` — above `func (cr *cycleRun) adoptContinuationAfterTriage() error {`

```text
// adoptContinuationAfterTriage is the ADR-0076 slice C adoption seam, invoked
// right after the triage phase completes — the moment this cycle's claims
// exist on disk, so the resolver reads REAL scope (architect finding #1: any
// earlier and processing/cycle-N does not exist yet). On a valid stamped
// claim it RE-SEEDS the cycle worktree from the salvage snapshot (CreateFrom
// force-recreates the same lane-namespaced path, so downstream phases pick it
// up unchanged), moves the review base to the ORIGINAL attempt's base — or to
// main's tip when the base-advance heals a stale one (continuation_baseadvance.go);
// either way the cumulative work is reviewed and shipped whole, copies the continuation
// manifest into THIS cycle's workspace (ship's manifest reconciliation unions
// the prior attempt's declared paths from it; a re-FAIL overwrites it at the
// next preserve), and serves the prior findings to the build prompt. Every
// validation failure keeps fresh provisioning. After seeding, any failure
// stops the cycle and preserves the worktree for diagnosis.
```

### `go/internal/core/continuation_stamp.go:246` — above `releaseDeclinedBinding(cr.req.ProjectRoot, scopeIDs, c)`

```text
// Release the declined binding, or the root-owned registry entry
// outlives its lineage and the defect-ledger gate's out-of-band check
// auto-FAILs every future lane on this scope (the 1412/1418
// absorbing-FAIL state). Orchestrator-side only — an agent deleting
// the WORKSPACE manifest still hits the gate's cycle-1285 block.
```

### `go/internal/core/continuation_stamp_projectroot_test.go:3` — above `import (`

```text
// continuation_stamp_projectroot_test.go — a salvage snapshot (ADR-0076) is
// `git add -A` + commit of a cycle's ISOLATED worktree. When the active
// worktree is the project root itself — the --simulate root reads the root in
// place; a resume checkpoint can name it — that would commit the operator's
// own tree (the second mutation the 2026-09-14 simulate incident found after
// the dossier commit). The stamp refuses, out loud, and stamps no
// continuation. The refusal is the package's ONE "is this the live
// repository" predicate (sameDirectory), so a symlink alias or a relative
// spelling of the root is refused exactly like the absolute path.
```

### `go/internal/core/contract_escalation.go:3` — above `import (`

```text
// contract_escalation.go — CONTRACT-BLOCK CLI ESCALATION (inbox
// contract-block-cli-escalation, P1 weight 0.95).
//
// The defect this closes, confirmed live twice: the correction ladder in
// reviewAndGuard re-dispatches the SAME profile CLI after a deliverable-contract
// block. The profile's cli_fallback chain fires only on infra exit codes
// {80,81,85,124,127} — never on a contract violation — so a CLI that
// systematically mis-formats a deliverable burns every correction and the
// contract-gate breaker opens, demoting enforce→advisory for the rest of the run.
// Batch-19 (cycles 1171/1172, adversarial-review) and batch-21 (cycle-1215,
// triage) both ended that way: a FORMAT-compliance failure silently WEAKENED a
// gate. The correct escape hatch is CLI escalation, not gate demotion.
//
// Three deliberate scoping constraints:
//
//  1. ESCALATE THE RE-DISPATCH, NOT THE PHASE (from the inbox item). The
//     non-compliance lives on the rare failure path (triage's v1 FAIL sentinel,
//     adversarial-review's section headings) while the same CLI ships the common
//     path fine. Rerouting the phase would change 99% of dispatches to fix 1%. So
//     the escalation is applied to PhaseRequest.ModelRoutingCLI on the
//     re-dispatch only — a SOFT overlay (llmroute.ApplySoftOverlay) that promotes
//     the target to chain primary while keeping the profile's own chain behind it
//     — and is reverted when the ladder ends. The profile on disk is never touched.
//
//  2. NOT ON THE FIRST BLOCK. One malformed turn is a bad turn, not a CLI
//     verdict. Escalation starts at the CONTRACT GATE'S OWN second consecutive
//     block (ReviewResult.Blocks, reported by the breaker that will open the
//     circuit — never a locally re-counted correction ordinal, which desyncs from
//     the breaker whenever a prior cycle left the count hot or the salvage rung
//     consumed a block). That is still before the third strike, so the circuit
//     stays the last resort.
//
//  3. A CONTRACT BLOCK ESCALATES — AND, since 2026-08-23, SO DOES A
//     REMEDIATION-CARRYING REJECTION (ReviewResult.Remediation, set only by
//     evalgate) at its second identical in-cycle round. The original constraint
//     read "a different CLI is not the remedy" for every Blocks==0 rejection;
//     the measured record disproved that for the CREATE-a-missing-artifact
//     class: every eval-materialization failure since cycle-1450
//     (1471/1476/1504/1531/1540/1545) was one CLI family, 0-for-all correction
//     rounds on that family — including rounds whose directive named the exact
//     writable paths (cycle-1545, verified) — while the other family never
//     failed the gate once (0/26). Remediation-LESS Blocks==0 rejections
//     (topngate / triagecap / the build floor) are capacity or task-binding
//     failures and keep the original rule: they never escalate.
//
//  4. ONLY AN IDENTICAL BLOCK ESCALATES (cycle-1289). ReviewResult.Blocks counts
//     blocks, not defects: two genuinely DIFFERENT contract violations on one
//     phase (block 1 misses a section heading, block 2 misses the verdict
//     sentinel) are two honest defects, not one incapable-CLI signature, and
//     round 2's budget should not buy a different CLI family for them. The
//     trigger is therefore gated on failure IDENTITY as well as count — see
//     contractBlocksShareIdentity.
//
//  5. A TRIGGER WITH NO TARGET STILL GETS A REMEDY (cycle-1300, from this item's
//     LIVE EVIDENCE note of 2026-08-05). When a phase's whole dispatch chain is
//     one CLI family, contractEscalationCLI returns ok=false and — before this
//     cycle — the ladder did nothing at all: the same incapable CLI got the same
//     plain directive a third time and the breaker opened, so the ratchet failed
//     OPEN purely because there was nowhere to escalate to. The TOP-FAMILY
//     remedy is a structured re-prompt (composeContractSalvageRetry) on the
//     re-dispatch the ladder was already performing. It is disjoint from
//     escalation — a phase WITH a target escalates and does not re-prompt, so one
//     block's budget is never spent twice — and it is BREAKER-NEUTRAL: no extra
//     dispatch, no extra correction, no change to ModelRoutingCLI (there is no
//     other family: that is the whole premise), so the circuit still opens on the
//     third strike as the last resort.
```

### `go/internal/core/contract_escalation.go:126` — above `func salvageClosing(remediation string) string {`

```text
// salvageClosing picks the salvage rung's final clause. The default forbids
// collateral edits; when the gate supplied a remedy that clause must not fire,
// because a remediation exists only for violations whose fix is to CREATE
// something.
//
// REACHABILITY, revised 2026-08-23: LIVE. The escalation trigger now admits
// remediation-carrying rejections (the evalgate class) at their second
// identical round, so a phase whose whole chain is one CLI family CAN reach
// this arm with a remediation in hand — the forward-looking parity below is
// now the production path for that shape. Historical context: before the
// revision, Blocks was set solely by internal/deliverable and Remediation
// solely by internal/evalgate, so no gate satisfied the trigger with a
// remediation; the 0-for-4 Gate A failures were "rejected after 2
// correction(s)" via maxCorrections exhaustion in the ORDINARY
// correction loop, which goes through composeCorrection — that is the call that
// actually closes the gap. This exists so the two directive paths cannot drift
// if a future gate ever sets both.
```

### `go/internal/core/contract_escalation.go:171` — above `func (cr *cycleRun) contractEscalationProfile(phase Phase) (*profiles.Profile, string) {`

```text
// contractEscalationProfile resolves the profile governing a phase, plus the
// AGENT name whose EVOLVE_<AGENT>_CLI env key the dispatch resolver reads.
//
// Two lookups, in precedence order: the built-in phase→agent table, then the
// `<phase>.json` convention every MINTED/user phase follows. The second is
// load-bearing, not defensive: phaseAgentName covers only the 10 built-in spine
// phases, so adversarial-review — the phase in this fix's own batch-19 evidence,
// which has a real .evolve/profiles/adversarial-review.json — resolves to nil
// through the built-in table alone and could never have escalated.
```

### `go/internal/core/contract_escalation.go:242` — above `func contractBlocksShareIdentity(prev contractBlockIdentity, reason string) bool {`

```text
// contractBlocksShareIdentity reports whether the contract block now on the
// ladder is the SAME defect as the block that triggered the previous correction,
// which is the second half of the escalation trigger (scoping constraint 4).
//
// Identity is the block's VIOLATION-CODE SET, not its rendered text (cycle-1291,
// repairing the cycle-1289 audit defect). The reason under comparison is
// deliverable.summarize() — a "; "-joined rendering of EVERY violation on the
// block as "[code] message" — so a whole-string compare reads a PARTIALLY
// REPAIRED defect set as a different defect: block 1 reports
// {missing_section, missing_verdict}, the correction closes one, block 2 reports
// {missing_verdict} alone, the two strings differ, and the escalation is
// suppressed exactly where the incapable-CLI signature is strongest (the CLI
// demonstrably cannot close the remaining violation). Superset regressions and
// re-ordered/re-worded renderings of ONE set fail the same way.
//
// deliverable.Violation.Code is the stable identity primitive, untouched by
// prose rewording or violation order, so two blocks are the SAME defect exactly
// when their code sets INTERSECT — which covers subset, superset and equal, and
// still separates the disjoint sets constraint 4 exists to keep apart. The codes
// reach here as plain data parsed out of the rendered reason: internal/deliverable
// imports internal/core (reviewer.go, verifier.go) and core imports deliverable
// nowhere, so a []deliverable.Violation field on ReviewResult would be an import
// cycle.
//
// FAIL-SAFE: not every reason on this path is a summarize() rendering. When
// EITHER block yields no code, identity falls back to failure_digest.go's
// normalizeReasonForFingerprint — the blocker breaker's own primitive, which
// projects a reason onto its defect identity by dropping identity-noise tokens
// (go-test durations, narrative verdicts). Reading "no codes on either side" as
// "∅ ∩ ∅ ⇒ different defect" would silently delete the ladder for every
// non-summarize reason shape.
//
// The rule is "prior reason known AND differing ⇒ suppress", NOT "equal ⇒
// escalate". The difference is the hot-breaker edge: the contract-gate breaker is
// process-global, so a cycle that aborted mid-ladder leaves it hot and the next
// phase can arrive at Blocks >= contractEscalateAtBlock on its ladder's FIRST
// block — where no prior block exists to compare. Requiring equality there would
// silently delete the escape hatch that constraint 2 deliberately keeps open, so
// the zero-value prev (no block observed yet) reports true.
```

### `go/internal/core/contract_escalation.go:315` — above `func contractArtifactDetermined(code string) bool {`

```text
// contractArtifactDetermined reports whether repairing the deliverable
// violation named by code NECESSARILY rewrites the bytes of the watched
// artifact. It is the precondition a hash-equality check needs before it may
// read "the artifact hash did not move" as "the agent did no new work": that
// inference is sound only for violations whose ONLY repair is an edit to the
// artifact itself.
//
// The counter-example the classifier exists for is deliverable.CodeStrayInWorktree
// (deliverable.go:306), which is repaired by DELETING a stray copy elsewhere in
// the worktree — the watched artifact is left byte-identical, so an unchanged
// hash there is not evidence of inaction and a short-circuit reading it that way
// would turn a repairable contract block into a deterministic ladder abort
// (inst-L1508a).
//
// ALLOWLIST, not denylist, and therefore fail-closed: a code this function has
// never heard of is one whose repair mechanics are unknown, and the only safe
// answer for an unknown repair mechanic is false — "do not skip the re-verify".
// False negatives cost one redundant verification; a false positive silently
// drops a real repair.
//
// Deliberately NARROWER than the criterion in one respect, stated here so the
// comment is true as written: deliverable.CodeInvalidJSON and
// deliverable.CodeFailureContextMissing also repair by editing the artifact, and
// this cycle's contract (triage top_n; the cycle-1510 eval's vocabulary-drift
// grader) pins the allowlist by membership, not by count. Widening to those two is queued as
// follow-up work rather than taken here — the omission errs in the fail-closed
// direction, so it cannot cause the unsound inference above. See the Amendments
// section of the cycle-1510 build report.
//
// The codes arrive as plain strings, never deliverable.Violation values:
// internal/deliverable imports internal/core (reviewer.go, verifier.go), so the
// reverse import would be a cycle — the same constraint documented for
// contractViolationCodeRE above. That makes drift between the two vocabularies
// invisible to the compiler, which is why it is asserted in a test instead
// (TestContractArtifactDetermined_CodesMatchDeliverableVocabulary).
//
// NO PRODUCTION CALLER YET, by design: the hash short-circuit this classifies
// for is not scheduled, and building the short-circuit alongside it would be
// scope creep (Core Rule 2). The exercised caller is the test above.
```

### `go/internal/core/contract_escalation.go:396` — above `func formatContractGateDemotionWarn(phase, cli string, d contractDispatch, reason string) string {`

```text
// formatContractGateDemotionWarn renders the operator-facing line for a contract
// gate that demoted itself. It names the PHASE, the CLI the blocks are
// attributable to, WHICH remedy actually ran, and the last violation — the
// batch-19 line named none of the four, which is why the same class recurred
// twice before anyone noticed.
//
// It takes the whole contractDispatch rather than a bare escalated bool because
// there are now two remedies to report and "did NOT run" is a line an operator
// trusts and acts on: it must be false only when it is false.
```

### `go/internal/core/contract_escalation.go:439` — above `Action:   fmt.Sprintf("demote enforce->advisory: cli=%s escalated=%v salvage_attempted=%v blocks=%d: %s", cli, d.escalat…`

```text
// Action carries the decision verb + evidence so the demotion survives
// beyond transient stderr (a Kind/Role-only entry is the content-free
// fingerprint shape that blinded the breaker diagnostics in cycle-1117).
//
// salvage_attempted rides ALONGSIDE escalated= (never instead of it): the
// two remedies are disjoint, so recurrence analytics reading this entry
// can tell "no remedy was possible" from "a structured re-prompt was
// tried and the CLI still could not comply".
```

### `go/internal/core/contract_escalation_target_test.go:44` — above `func TestContractEscalation_MintedPhaseResolvesItsOwnProfile(t *testing.T) {`

```text
// TestContractEscalation_MintedPhaseResolvesItsOwnProfile is the coverage fix for
// the phase this whole item's batch-19 evidence came from. phaseAgentName maps
// only the 10 built-in spine phases, so `adversarial-review` — which HAS a real
// .evolve/profiles/adversarial-review.json — resolves to a nil profile through
// the built-in table alone, and a nil profile means "primary is claude-tmux",
// which means "same family", which means NO escalation. Every minted/user phase
// under .evolve/phases/ shares that shape, and they are exactly the phases an
// operator points at a non-claude CLI.
```

### `go/internal/core/contract_escalation_test.go:3` — above `import (`

```text
// contract_escalation_test.go — RED-first coverage for CONTRACT-BLOCK CLI
// ESCALATION (inbox contract-block-cli-escalation, P1 weight 0.95; twice
// confirmed live: batch-19 cycles 1171/1172 adversarial-review and batch-21
// cycle-1215 triage, both on agy-tmux).
//
// The live defect: reviewAndGuard's correction ladder re-dispatches the SAME
// profile CLI after a contract block. cli_fallback fires only on infra exit
// codes {80,81,85,124,127}, never on a contract violation, so a CLI that
// systematically mis-formats a deliverable burns every correction and the
// contract-gate breaker demotes enforce→advisory — a gate-WEAKENING outcome.
//
// These tests drive the REAL Orchestrator.profileForModelRouting seam (real
// .evolve/profiles/*.json on disk) through RunCycle, so they exercise the
// production resolution path rather than a stub.
```

### `go/internal/core/contract_escalation_test.go:53` — above `type escalationProbe struct {`

```text
// escalationProbe is BOTH the phase runner and the deliverable reviewer for one
// phase, coupled through the single fact the live defect turns on: WHICH CLI
// produced the deliverable now under review. It reproduces the production
// contract gate's breaker semantics (deliverable.Reviewer): each non-compliant
// deliverable is a BLOCK, `threshold` consecutive blocks demote enforce→advisory
// (Approve + Demoted), and a compliant deliverable resets the counter.
//
// compliantCLI == "" models batch-19/21: no re-dispatch ever complies.
```

### `go/internal/core/contract_escalation_test.go:68` — above `reasonPerBlock []string`

```text
// reasonPerBlock, when non-empty, supplies the violation text per consecutive
// block (block n uses index n-1, the last entry repeating for later blocks).
// Empty ⇒ every block reports the same reason, which is what the pre-cycle-1289
// tests assume. This is the ONE axis the fingerprint gate turns on: whether
// block 2's violation is the SAME defect as block 1's.
```

### `go/internal/core/contract_escalation_test.go:387` — above `func TestFormatContractGateDemotionWarn(t *testing.T) {`

```text
// TestFormatContractGateDemotionWarn pins the operator-facing WARN's content:
// the demoted phase and the CLI must both be named (the batch-19 log line named
// neither, which is why two batches lost cycles to an invisible demotion).
```

### `go/internal/core/contract_escalation_test.go:478` — above `func TestContractCorrection_DifferingBlockReasonsDoNotEscalate(t *testing.T) {`

```text
// ============================================================================
// cycle-1289 — FINGERPRINT-GATED ESCALATION TRIGGER
//
// The gap the landed PR #390 mechanism leaves open: the trigger at
// cyclerun_review.go counts blocks (`rr.Blocks >= contractEscalateAtBlock`) and
// never asks whether block 2 is the SAME defect as block 1. Two genuinely
// different contract violations on one phase (block 1 misses a section heading,
// block 2 misses the verdict sentinel) then read as one incapable-CLI signature
// and spend round 2's budget on a different family for no reason. The inbox item
// (.evolve/inbox/2026-08-04T07-15-00Z-contract-block-cli-escalation.json) states
// the fix: "integrate with the fingerprint breaker so identical blocks share
// identity" — i.e. reuse failure_digest.go's normalizeReasonForFingerprint, the
// blocker breaker's OWN identity primitive, rather than invent a second one.
//
// Three axes, encoded below and by the pre-existing tests:
//
//	NEGATIVE  differing violations       → NO escalation  (TestContractCorrection_DifferingBlockReasonsDoNotEscalate)
//	POSITIVE  same defect, noisy text    → escalates      (TestContractCorrection_NormalizedIdenticalReasonsEscalate)
//	EDGE      no prior reason observed   → escalates      (TestContractCorrection_HotBreakerEscalatesOnFirstCorrection, above)
//
// The EDGE axis is load-bearing and is why the gate is "prior reason known AND
// differing ⇒ suppress", not "equal ⇒ escalate": a breaker left HOT by an
// earlier cycle arrives at Blocks>=2 on this ladder's FIRST block, so there is
// no prior reason to compare. Requiring equality there would silently delete the
// hot-breaker escape hatch that PR #390's review established.
// ============================================================================
```

### `go/internal/core/contract_escalation_test.go:572` — above `func TestContractCorrection_SubsetRepairStillEscalates(t *testing.T) {`

```text
// ============================================================================
// CYCLE-1291 — VIOLATION-CODE-SET IDENTITY (the cycle-1289 audit defect)
//
// cycle-1289 shipped contractBlocksShareIdentity as a WHOLE-STRING compare of
// normalizeReasonForFingerprint(reason). The audit rejected it HIGH:
//
//	"contractBlocksShareIdentity compares whole summarize() strings, so a
//	 partially-repaired violation set (subset) reads as a different defect and
//	 suppresses [escalation]"
//	(.evolve/runs/cycle-1289/audit-fail-reason.json)
//
// The reason under comparison is deliverable.summarize() — a "; "-joined
// rendering of EVERY violation on that block ("[code] message"). So when block 1
// reports {missing_section, missing_verdict} and the correction closes ONE of
// them, block 2 reports {missing_verdict} alone. That is the SAME defect getting
// partially repaired — the strongest possible incapable-CLI signature, since the
// CLI demonstrably cannot close the remaining violation — yet the two rendered
// strings differ verbatim, normalizeReasonForFingerprint (which masks only
// durations and narrative verdicts, never violation-set MEMBERSHIP) leaves them
// differing, and the escalation is suppressed exactly when it is most warranted.
//
// The fix is to compare violation-CODE SETS, not rendered text. deliverable.
// Violation.Code is the stable identity primitive (go/internal/deliverable/
// deliverable.go:33-36) and is untouched by prose rewording or violation order.
// Same defect ⇔ the two blocks' code sets INTERSECT.
//
// IMPORT-CYCLE CONSTRAINT (cycle-644 reachability obligation — compiler-proven
// this cycle, do not re-litigate): internal/deliverable imports internal/core
// (reviewer.go:12, verifier.go:18) and core imports deliverable NOWHERE. So
// core.ReviewResult can NOT carry []deliverable.Violation — that is an import
// cycle and the criterion would be permanently unsatisfiable. The code set must
// reach core as plain data (codes parsed out of the rendered Reason, or a
// []string field on ReviewResult). These tests pin BEHAVIOUR through the real
// RunCycle ladder and deliberately do NOT pin either shape.
//
// Four axes, all driven through the production caller (RunCycle → reviewAndGuard):
//
//	POSITIVE  subset repair    {A,B} → {B}     → escalates  (001, THE audit defect)
//	POSITIVE  superset regress {B}   → {A,B}   → escalates  (002)
//	NEGATIVE  disjoint sets    {A,B} → {C,D}   → NO escalate (003)
//	EDGE      reordered/reworded same set      → escalates  (004)
//	EDGE      no [code] token at all           → escalates  (005, fail-safe)
// ============================================================================
```

### `go/internal/core/contract_escalation_test.go:616` — above `func TestContractCorrection_SubsetRepairStillEscalates(t *testing.T) {`

```text
// TestContractCorrection_SubsetRepairStillEscalates is THE cycle-1289 audit
// defect, encoded. Block 1 carries two violations; the correction closes the
// first, so block 2 carries only the second — a strict SUBSET of block 1's code
// set. Under whole-string identity the two summaries differ verbatim and the
// escalation is suppressed. Under code-set identity the sets intersect on
// missing_verdict, the blocks are one partially-repaired defect, and the second
// consecutive block must still escalate off the failing CLI family.
```

### `go/internal/core/contract_escalation_test.go:783` — above `func TestContractArtifactDetermined_Table(t *testing.T) {`

```text
// TestContractArtifactDetermined_Table is the RED contract for cycle-1510 task
// `contract-correction-hash-freshness-classification` (carryover from the
// cycle-1508 audit FAIL, defects H1/L1).
//
// What it pins. contractArtifactDetermined(code) answers exactly one question:
// "does repairing this violation NECESSARILY change the bytes of the watched
// deliverable artifact?" — the only precondition under which an unchanged
// artifact hash is sound evidence of "no new work was done". Six codes qualify,
// because their repair IS an edit to the artifact. deliverable.CodeStrayInWorktree
// is the canonical counter-example the cycle-1508 audit was built around: it is
// repaired by DELETING a stray worktree copy, which leaves the watched artifact
// byte-identical — so a hash-equality short-circuit that does not consult this
// classifier converts a repairable contract block into a deterministic ladder
// abort (inst-L1508a).
//
// NEGATIVE + fail-closed axis. Unknown codes must return false. The classifier
// is an ALLOWLIST, not a denylist: a code this function has never heard of is,
// by construction, one whose repair mechanics are unknown, and the only safe
// answer for an unknown repair mechanic is "do not let anyone skip re-verify".
// A denylist implementation (`return code != "stray_in_worktree"`) passes the
// six positive rows and the stray row and is still wrong — the unknown-code
// rows below are what separate it from a correct allowlist.
```

### `go/internal/core/contract_escalation_test.go:844` — above `func TestContractArtifactDetermined_StrayInWorktreeIsFalse(t *testing.T) {`

```text
// TestContractArtifactDetermined_StrayInWorktreeIsFalse is the crux assertion
// stated on its own so a regression names itself in the failure output: the
// exact pair that made cycle-1508's proposed hash short-circuit unsound.
```

### `go/internal/core/contract_salvage_retry_test.go:3` — above `import (`

```text
// contract_salvage_retry_test.go — RED-first coverage for the cycle-1300
// fleet-scoped tasks on inbox item contract-block-cli-escalation:
//
//   1. breaker-neutral-salvage-retry-when-no-escalation-family-exists
//   2. demotion-ledger-records-salvage-attempted-vs-no-remedy-possible
//
// The live gap (inbox LIVE EVIDENCE 2026-08-05). contractEscalationCLI returns
// ok=false when a phase's whole dispatch chain is ONE CLI family: there is no
// other family to escalate to. Today the ladder then does nothing at all — the
// same incapable CLI gets the same plain correction directive a third time and
// the breaker opens, so the ratchet fails OPEN purely because there was nowhere
// to escalate to. The remedy this cycle pins is a TOP-FAMILY remedy distinct
// from escalate: the correction that WOULD have escalated instead becomes a
// structured re-prompt (the verbatim validator reason carried under a distinct
// heading), spending round-2 budget on making the diagnosis explicit before the
// last-resort circuit trips.
//
// BREAKER-NEUTRAL is the load-bearing adjective and the anti-no-op axis: the
// remedy must ENRICH the re-dispatch the ladder already performs, never add a
// dispatch and never spend an extra correction. An implementation that inserts
// its own extra retry round fails TestContractEscalation_SalvageRetry_WhenNoOtherFamily's
// dispatch-count assertion.
//
// These tests drive the REAL production path — Orchestrator.RunCycle →
// reviewAndGuard's correction ladder → contractEscalationCLI — against real
// .evolve/profiles/*.json on disk. Nothing here calls the new seam directly, so
// a seam wired into nothing stays RED.
```

### `go/internal/core/contract_salvage_retry_test.go:79` — above `func TestContractEscalation_SalvageRetry_WhenNoOtherFamily(t *testing.T) {`

```text
// TestContractEscalation_SalvageRetry_WhenNoOtherFamily is the cycle-1300 crux
// (task 1). A profile already on the universal-fallback family with no declared
// chain has NO escalation target — contractEscalationCLI returns ok=false. The
// correction that would have escalated must instead re-dispatch the SAME CLI
// with a STRUCTURED RE-PROMPT: the verbatim validator reason under
// contractSalvageRetryDirectiveHeading.
//
// Three assertions, and all three must hold:
//
//	(a) breaker-neutral — exactly 3 dispatches, the same count as before this
//	    feature. The remedy rides the existing re-dispatch; it neither adds a
//	    round nor spends an extra correction, so the breaker's block accounting
//	    is untouched by the retry itself.
//	(b) it is NOT a shuffle — the re-dispatch stays on the phase's own routing
//	    (ModelRoutingCLI == ""), because there is no other family to move to.
//	(c) the directive is a structured re-prompt carrying the verbatim reason —
//	    and correction 1's directive is NOT (one bad turn is not a CLI verdict;
//	    the remedy belongs to the block that would have escalated).
```

### `go/internal/core/correction_directive_class_test.go:3` — above `import (`

```text
// correction_directive_class_test.go — a correction directive must not forbid
// the action the gate requires.
//
// The defect: composeCorrection emits ONE directive for every deliverable
// rejection, written for a single failure class — "the contracted artifact
// exists but is malformed". Gate A (evals-materialized) rejects for a DIFFERENT
// class: a required SIDECAR artifact was never created. For that class the
// generic directive misdirects on every clause. It says "fix THE deliverable"
// (singular — pointing at a scout-report.md that is already well-formed), frames
// the defect as "required sections / valid structure", and closes with
// "Do not change unrelated files" — which forbids creating the eval sidecars,
// the one action that would satisfy the gate.
//
// Live consequence: every scout|gate-block failure in recorded history is this
// gate (cycles 1471, 1476, 1504, 1531) and all four read "rejected after 2
// correction(s)". 0-for-4 recovery. A merely weak directive recovers sometimes.
//
// Contract: a gate that knows how to fix its own violation supplies a
// remediation; when one is present the directive carries it and drops the
// clause forbidding file creation. When absent, the directive is byte-identical
// to today.
```

### `go/internal/core/correction_directive_class_test.go:31` — above `const gateARejection = "scout did not materialize evals for selected slug(s): " +`

```text
// gateARejection is the real Gate A reject reason, verbatim from cycle-1531's
// dispatched prompt (.evolve/runs/cycle-1531/scout-prompt.txt line 5).
```

### `go/internal/core/correction_ladder.go:3` — above `import (`

```text
// correction_ladder.go — ADR-0045 I2: the orchestrator-side executors for the
// graduated correction ladder (the DECISION lives in interaction.NextCorrection,
// a pure leaf). Rung 1 (salvage) turns the cycle-265 class — a valid
// deliverable at the wrong path, burned through two full re-dispatches — into
// an atomic relocate + breaker-neutral verify, no agent involved. Rung 3's
// directive is enriched with kernel-verified evidence so correction attempts
// stop retrying blind (cycle-265: attempt 2 carried only the violation text).
//
// Salvage safety posture (threat S2 — salvage as a smuggling vector):
//   - candidates are constructed by the KERNEL from the phase's own roots
//     (worktree, workspace, cwd) + the CONTRACTED basename — never from agent
//     input, so traversal cannot be steered;
//   - lstat gate: only regular files move (a planted symlink at the stray
//     location must not be followed into the contracted path);
//   - size + staleness caps bound what a hostile or ancient stray can inject;
//   - relocate FIRST, verify the DESTINATION after (TOCTOU: the pre-move copy
//     is never the trusted artifact — what landed at the contracted path is
//     what gets verified and gated);
//   - salvage NEVER upgrades a verdict: a verified relocation still faces the
//     review gate (the breaker-touching FINAL outcome) like any native artifact.
```

### `go/internal/core/correction_ladder_test.go:3` — above `import (`

```text
// correction_ladder_test.go — ADR-0045 I2 (§8): the orchestrator-side ladder.
// White-box: reuses fakeStorage / fakeLedger / buildRunners /
// sequencedReviewer / recordingReviewer and the slice-1 ledger readers.
```

### `go/internal/core/correction_ladder_test.go:82` — above `type strayWriterRunner struct {`

```text
// strayWriterRunner delegates to fakeRunner and plants a stray deliverable at
// the LIVE workspace root on its first dispatch — modeling the cycle-265
// agent that wrote a valid report at the wrong path. (RunCycle provisions a
// fresh workspace, so pre-seeding from the test would be wiped.)
```

### `go/internal/core/covering_tests_ensure_test.go:3` — above `import (`

```text
// covering_tests_ensure_test.go — cycle-1270 Task 3
// (`test-amplification-context-scope`), the open residual.
//
// writeCoveringTests is called from inside `if completed == PhaseBuild`
// (phase_bindings.go). On any path where test-amplification runs without a
// fresh build completion in the SAME process — a resume past build, or a future
// insertion after a different phase — the artifact is absent and the phase
// silently reverts to the whole-repo Grep the corpus exists to remove.
//
// SILENT is the defect, not slow. The corpus exists to make a before/after
// token measurement interpretable (5.4M cache-read tokens/run baseline); a run
// that quietly degrades produces a number nobody can read. The fail-open
// contract is untouched: an underivable diff still leaves the phase working
// exactly as it does today — it just says so.
```

### `go/internal/core/covering_tests_injection_test.go:3` — above `import (`

```text
// covering_tests_injection_test.go — RED contract for cycle-1268 task
// `test-amplification-context-scope`, the one half of it that is still open.
//
// The derivation half (changedpkgs.CoveringTests + DirectImporters, the
// covering-tests.md artifact, the phase-spec input, the fail-open guard and the
// truncation log) arrived in this lane already landed via the ADR-0076
// continuation snapshot 79d130d4 and is verified GREEN in-tree — see the
// disposition table in test-report.md. What is NOT closed is the audit finding
// that attempt's OWN auditor raised against the code it shipped:
//
//	D3/F1 MEDIUM — renderCoveringTests interpolates paths unescaped into
//	covering-tests.md, a document .evolve/phases/test-amplification/agent.md
//	declares authoritative. A filename carrying a backtick closes the code span,
//	and one carrying a newline injects top-level markdown directives into a
//	code-writing agent — subverting exactly the anti-bias isolation (no diff, no
//	implementation) the corpus was introduced to preserve.
//
// The path set is attacker-influenced by construction: it is derived from
// filenames in the worktree diff, and any commit can add a file with a hostile
// name. Rendering it verbatim into an authoritative agent input is the
// vulnerability; a task cannot be called landed while its own deliverable
// carries it.
//
// The assertions pin the INVARIANT (one list item per path, no structural
// breakout) and not a particular escape strategy — stripping, escaping, or
// quoting are all acceptable implementations.
```

### `go/internal/core/covering_tests_truncation_log_test.go:3` — above `import (`

```text
// RED contract for cycle-1267 Task 1 (`scope-test-amplification-context`) —
// the "no silent caps" half of the inbox item's how_to_apply:
//
//	cap corpus bytes with a loud truncation note (no silent caps)
//
// Today the cap is enforced and a `TRUNCATED:` note is written INTO the
// artifact, so the amplification agent can see it. The OPERATOR cannot: nothing
// reaches the cycle log, so a lane whose corpus was trimmed to 60% is
// indistinguishable at the console from one that was injected whole — and the
// item's own success metric is a before/after token measurement, which a silent
// cap makes uninterpretable ("did the corpus shrink, or did the cap eat it?").
// A cap that only the consumer of the artifact can see is a silent cap from the
// operator's seat.
//
// Two pins below:
//
//  1. renderCoveringTests must REPORT how many paths it dropped, rather than
//     burying the fact in its own output. That count is the single source for
//     both the in-artifact note and the operator warning — deriving it twice
//     (e.g. by re-scanning the rendered string for "TRUNCATED:") would put two
//     answers to one question in the tree.
//  2. writeCoveringTests must emit that count to stderr when it is non-zero,
//     and stay silent when nothing was dropped (a warning on every clean cycle
//     is noise that trains the operator to ignore the real one).
//
// Pin (1) changes an UNEXPORTED signature, so the two existing call sites in
// covering_tests_artifact_test.go must be updated to the two-value form. That
// is a mechanical call-site update, not a weakening: their assertions stay
// byte-identical.
```

### `go/internal/core/crossartifact_invariants.go:3` — above `import (`

```text
// crossartifact_invariants.go — the ADVISORY half of the cycle-1676
// cross-artifact invariant stack. internal/coherence computes the four weak
// verifiers; this is the one place the real cycle-close path records them.
//
// Two properties are load-bearing and are pinned at this seam
// (crossartifact_invariants_wiring_test.go):
//
//  1. The stack is bound to the LANE worktree (cs.ActiveWorktree), never the
//     projectRoot argument — in fleet mode a project-root snapshot names a tree
//     this lane did not write, which is the #612 lesson.
//  2. A finding NEVER blocks. It changes no verdict, raises no system failure,
//     and is recorded on EVERY cycle, all-ok included: a false-positive rate
//     that is never recorded can never be evidenced, and that evidence is the
//     only door to graduating any of these invariants to blocking (the
//     1054/1060 breaker lesson, and the inbox record's own rule).
```

### `go/internal/core/crossartifact_invariants_wiring_test.go:3` — above `import (`

```text
// crossartifact_invariants_wiring_test.go — THE WIRING HALF of the cycle-1676
// RED contract for inbox item `crossartifact-invariant-stack`.
//
// internal/coherence/crossartifact_test.go pins WHAT the aggregate computes.
// This file pins that it FIRES from the real cycle-close path (finalizeCycle,
// the terminal segment RunCycle always reaches) and that firing it changes
// nothing about whether the cycle blocks. A checker nothing calls is the same
// defect class the stack was written to catch (#373: wired into one path only
// is the same defect); an advisory that quietly gained teeth is the 1054/1060
// breaker lesson the inbox record explicitly forbids repeating.
//
// THE CONTRACT (Builder implements; this file is frozen):
//  1. finalizeCycle evaluates coherence.CheckCrossArtifactInvariants over the
//     cycle's workspace and its LANE worktree (cs.ActiveWorktree) — never the
//     projectRoot argument, which in fleet mode names a tree this lane did not
//     write (#612).
//  2. It records the result at <workspace>/crossartifact-invariants.json:
//     {"advisory":true,"invariants":[{"name","status","evidence"},...]} — always,
//     including an all-ok cycle, because a false-positive rate that is never
//     recorded can never be evidenced, and that evidence is the only door to
//     graduating any of these invariants to blocking.
//  3. It NEVER mutates result.FinalVerdict or result.SystemFailure, and never
//     disturbs the ADR-0072 verdict-incoherence floor that runs beside it.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Wiring/positive: the artifact appears on the real path (anti-no-op: a
//     helper that exists but is never called fails here and nowhere else).
//   - NEGATIVE       : four violations must NOT change the verdict, and the
//     lane-binding case must stay violated when the cited path exists only
//     under the project root.
//   - Semantic       : the pre-existing forgery halt is byte-for-byte unchanged.
```

### `go/internal/core/crossartifact_invariants_wiring_test.go:200` — above `func TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot(t *testing.T) {`

```text
// TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot — AC3 at
// the seam. The cited evidence path exists ONLY under the projectRoot argument,
// so an implementation that resolves against the project root reports ok while
// the lane's own tree never had the file. Binding cs.ActiveWorktree is the
// #612 lesson: a cross-artifact check must read the tree the lane actually
// wrote. The second half proves the check is not simply always-violated.
```

### `go/internal/core/crossartifact_invariants_wiring_test.go:247` — above `func TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory(t *testing.T) {`

```text
// TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory — AC4's
// no-regression half at the seam. The ADR-0072 forgery halt keeps firing
// exactly as before with the advisory aggregate running beside it, and the
// advisory record is still written on that path.
```

### `go/internal/core/cycle420_amplified_test.go:3` — above `import (`

```text
// cycle420_amplified_test.go — Adversarial amplification for cycle-420 task T1.
//
// Probes gaps NOT covered by phase_advisor_catalog_test.go (AC1–AC4):
//
//   - Single-line pointer: overflow pointer must be exactly one line containing
//     "phase-inventory.json", not multiple lines.
//   - O(1) pointer size: a large overflow (50 extra cards) must not produce a longer
//     output than a minimal overflow (1 extra card) — pointer is a fixed string.
//   - One-below-cap boundary: maxEnrichedCatalogCards-1 optional cards must not
//     trigger the pointer (tighter negative guard than the at-cap AC4 test).
```

### `go/internal/core/cycle_closeout.go:24` — above `cr.emitCycleClose(cr.result, "cycleRun.completeCycle")`

```text
// ADR-0101 S2a: the cycle's event stream ends here on both roots —
// system.failure (if the floors attached one) then cycle.sealed; the
// abnormal path seals from abnormalEpilogue.
```

### `go/internal/core/cycle_closeout.go:28` — above `if derr := writeCycleDossier(cr.o.gitMutationLock, cr.dossierParams(cr.result.FinalVerdict)); derr != nil {`

```text
// ADR-0055: emit this completed cycle's closeout dossier to
// <ProjectRoot>/knowledge-base/cycles/cycle-N.json. Best-effort — the cycle
// has already finalized, so a closeout-artifact write error must not fail it
// (presence is enforced separately by `evolve dossier verify` against the
// policy floor). Goal text comes from Context["goal"]; falls back to the goal
// hash so the dossier's required Goal is never blank.
```

### `go/internal/core/cycle_outcome.go:11` — above `func isScoutEvalMaterialization(phase Phase, p string) bool {`

```text
// isScoutEvalMaterialization reports whether a main-tree write is scout
// performing its documented eval-materialization contract. Scout writes the
// SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN
// tree (internal/evalgate/materialization.go reads them there for Gate A), so
// that write is scout's JOB, not a deliverable escape. Without this carve-out
// a later cycle iterating the same coverage target re-materializes the same
// slug, MODIFYING the prior cycle's committed eval (soak-#6 cycle 318→319
// ledger-seal-io-coverage), and the tree-diff guard aborts the cycle. Scoped
// to scout + .evolve/evals/<slug>.md only: a code phase leaking an eval, or
// scout writing a non-.md file or any other deliverable (phases/, commit-
// prefix-scope.json) or a source file, all still fire the guard.
```

### `go/internal/core/cycle_outcome.go:26` — above `func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision string, shipped bool) string {`

```text
// finalizeOutcome translates a bare SKIPPED cycle verdict into a specific
// CycleOutcome label. PASS/FAIL/WARN pass through untouched.
//
// SHIPPED_VIA_BUILD requires THIS cycle's own ship latch (CycleState.Shipped,
// set by latchShippedState only when the ship phase PASSed and survived the
// deliverable review, on either dispatch root). It is never
// inferred from main HEAD movement: in fleet mode a sibling lane moves HEAD
// constantly, and cycle 1630 — scout, triage, an honest empty commitment, no
// ship — was credited with a sibling's landing (ADR-0100, PR-3). A SKIPPED
// verdict without a ship keeps its no-work label so IsTriageNoWorkResult and
// the throughput recorder read the cycle truthfully.
```

### `go/internal/core/cycle_worktree_teardown.go:8` — above `func (o *Orchestrator) teardownCycleWorktree(projectRoot, wtPath string, preserve, completedNormally bool) {`

```text
// teardownCycleWorktree is the ONE worktree-disposal rule both entrypoints
// apply at cycle exit. It was previously an anonymous closure inside
// newCycleRun's cleanup stack, which made it reachable only from RunCycle:
// RunCycleFromPhase registers the other three exit actions (lock release,
// run-ID clear, lease stop) by hand and had no counterpart for this one, so
// every resumed cycle leaked its worktree regardless of verdict.
//
// The decision itself is unchanged, and is deliberately conservative in one
// direction only — it prunes just the spent trees:
//
//   - preserve (finalizeCycle's preserveOnVerdict, or a recorded ship-stage
//     failure) means the tree holds audited, possibly uncommitted work that
//     `evolve loop --resume` or `evolve cycle reset` reclaims BY this path.
//   - !completedNormally means the cycle died before adjudicating its work at
//     all, which is the same situation with less information.
//   - Otherwise the cycle finished and ship has merged the worktree into main,
//     so the tree is spent; pruning it is what keeps the checkout count bounded.
//
// Erring the other way is not symmetric: a missed prune costs disk, while a
// wrong prune is the cycle-7 incident (an entire PASS cycle's work destroyed).
// That asymmetry is why both no-prune conditions stay, and why the resume path
// gets this exact rule rather than a resume-specific policy.
//
// This rule decides WHETHER to dispose; the provisioner decides whether a
// path is one it may dispose of. gitWorktree.Cleanup refuses the project root
// (a resume checkpoint can name it) and returns an error, which the cerr
// branch below treats exactly like any other failed prune: the path stays
// named so resume/reset can still reach it.
```

### `go/internal/core/cyclelevel_failure_test.go:1` — above `package core_test`

```text
// cyclelevel_failure_test.go — cycle-234 task `cycle-level-bridge-failure` (RED).
//
// Invariant 3 root fix (retro I-9, batch deaths c225/c230/c231): a bridge or
// phase error is a CYCLE-level failure — the batch must survive it. Only
// kernel-integrity invariants (phase gate denial, broken ledger chain, lock
// contention) stay batch-fatal.
//
// Contract encoded here:
//   - core exposes an ErrCycleLevelFailure wrapper (Phase + Cause) with
//     errors.As/errors.Is roundtrip;
//   - RunCycle wraps the bridge-exhaustion abort path in it;
//   - integrity breaches are NEVER wrapped (they must keep killing the batch);
//   - the audit↔ship recovery loop is budget-bounded and its exhaustion is
//     itself cycle-level, not batch-fatal (the c230 signature: 3 PASSed
//     audits, 0 ships, batch dead).
//
// RED note: this file references core.ErrCycleLevelFailure, which does not
// exist yet — the compile error "undefined: core.ErrCycleLevelFailure" is the
// intended RED signal. Builder defines it in go/internal/core/errors.go.
// Shares the core_test harness from orchestrator_recovery_test.go.
```

### `go/internal/core/cyclelevel_failure_test.go:182` — above `func TestOrchestrator_RecoveryDepthBudget(t *testing.T) {`

```text
// TestOrchestrator_RecoveryDepthBudget — scout AC: the audit↔ship recovery
// loop is capped at maxRecoveryDepth (2). The c230 incident signature was 3
// PASSed audits with 0 ships, then a batch-fatal abort. Under Invariant 3:
//   - the traversal stays bounded: 1 initial audit + at most 2 recovery
//     re-audits, same bound for ship attempts;
//   - exhaustion of a PRECONDITION-class recovery is a cycle-level failure
//     (ErrCycleLevelFailure), NOT batch-fatal — only integrity breaches kill
//     the batch (covered by TestOrchestrator_IntegrityBreach_StillBatchFatal
//     and the existing TestRunCycle_ShipIntegrityError_AbortsLoud).
```

### `go/internal/core/cyclerun.go:36` — above `const defaultMaxPhaseIterations = 32`

```text
// defaultMaxPhaseIterations bounds RunCycle's dispatch loop against a
// transition-table cycle (a phase order that keeps re-selecting phases and never
// reaches PhaseEnd). It is a genuine safety oracle (ADR-0044 C1) — it must exist
// even when config is absent. WithMaxPhaseIterations overrides it; tests set it
// low to drive RunCycle into the chokepoint-escape guard deterministically. 0 ⇒
// this default.
```

### `go/internal/core/cyclerun.go:58` — above `consoleLeased  map[string]bool`

```text
// consoleLeased: ADR-0080 S4 operator lease ADOPTED at cycle start (hub-
// resident; a mid-cycle write cannot waive the cycle that made it).
```

### `go/internal/core/cyclerun.go:91` — above `replanDepth       int`

```text
// ADR-0052 WS2-S5: post-scout re-plans run this cycle; capped by cfg.RePlanMaxDepth (check-before-increment)
```

### `go/internal/core/cyclerun.go:93` — above `shipLease *shipwindow.Lease`

```text
// shipLease serializes the audit→ship critical section across lanes
// (cycle-778): acquired in recordAndBranch just before the audit-binding
// HEAD snapshot, released after the next completed phase (normally ship,
// post-push) and by RunCycle's exit defer. nil ⇒ not held.
```

### `go/internal/core/cyclerun.go:202` — above `func (cr *cycleRun) recordChokepointEscape(reason string) {`

```text
// recordChokepointEscape closes the ADR-0044 C1 invariant on RunCycle's
// bounded-loop exit. If the dispatch loop exhausts its iteration budget without
// reaching PhaseEnd (a transition-table cycle), no phase recorded a terminal
// outcome, so cyclehealth.ClassifyOutcome would page the cycle FAILED_UNEXPLAINED
// — the alarm bucket (the cycle-492 escape). Recording an explicit abort here
// routes the escape through the C1 chokepoint (recordPhaseOutcome) so the outcome
// is FAILED_EXPLAINED and names the phase the cursor stalled on, and feeds
// failure-learning so the loop's retro sees a real, diagnosable failure.
```

### `go/internal/core/cyclerun.go:235` — above `recordCrossArtifactInvariants(cycle, cs.WorkspacePath, cs.ActiveWorktree)`

```text
// Cross-artifact invariant stack (cycle-1676): four weak deterministic
// verifiers over this cycle's own artifacts, bound to the LANE worktree
// (#612), recorded beside them. Purely ADVISORY — it runs before the
// ADR-0072 floor below precisely so a finding can be read next to the
// floor's decision without ever influencing it. See
// crossartifact_invariants.go.
```

### `go/internal/core/cyclerun.go:243` — above `if result.SystemFailure == nil {`

```text
// ADR-0072 Go floor: verdict-coherence. If the cycle recorded a negative
// verdict but the phases' own on-disk artifacts (audit-report + acs-verdict)
// are green, the pipeline forged the verdict — a SYSTEM-level failure, not a
// task failure. Mark it so the batch loop HALTS + escalates for pipeline
// diagnosis instead of re-selecting the same inbox task (which would just
// reproduce the forged verdict — the cycle 862→899 livelock). This floor is
// non-negotiable: independent of orchestrator judgment and strict_audit.
```

### `go/internal/core/cyclerun.go:254` — above `fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d verdict-coherence SELF-HEAL: recorded %s but on-disk audit=PASS, acs=PAS…`

```text
// Clean-exit-late-write self-heal: the recorded-negative was contradicted
// by green artifacts AND a fully-valid audit-report — a benign timing race,
// not a forged verdict. Reconcile the recorded verdict to PASS and do NOT
// halt (the alternative was the cycles-930/931/932 false-HALT batch-killer).
```

### `go/internal/core/cyclerun.go:261` — above `result.SystemFailure = sig`

```text
// Announced by the closeout's system.failure INCIDENT (ADR-0101
// S2a): one line format on stderr, one line in signals.ndjson.
```

### `go/internal/core/cyclerun.go:267` — above `if result.SystemFailure == nil {`

```text
// A cycle can also lose its landing OUTRIGHT: ship ran, hit landing-queue
// contention, and the recovery correctly routed elsewhere — leaving a PASS
// verdict from the audit floor attached to a cycle that delivered nothing
// (wave-20260822a-verify: cycle-1535 rebased into a genuine conflict on a
// peer lane's file, went to the debugger, and closed out PASS). Runs AFTER
// the incoherence block so a self-healed PASS is checked too, and keyed on
// the cycle's OWN ship artifacts rather than a HEAD delta, which in fleet
// mode belongs to whichever sibling landed last. See lost_landing_floor.go.
```

### `go/internal/core/cyclerun.go:284` — above `if shouldWarnSkippedUnknown(*result) {`

```text
// Notice the silent no-ship (Fix C): the cycle ran phases but ended without
// its own ship landing and without an audit-advisory "would-have-blocked" record —
// i.e. work may have been produced and then discarded with the worktree
// (cycle-148: a genuine PASS mis-graded FAIL routed audit→retro→end). The
// outcome label alone is advisory and easily missed in a batch summary, so
// surface it loudly here. Not an error — some cycles legitimately produce no
// change — but always worth an operator's eyes.
```

### `go/internal/core/cyclerun.go:315` — above `if preserveWorktree {`

```text
// ADR-0076 slice C: a preserved (FAILed) worktree is snapshot-committed and
// its continuation manifest stamped NOW — while the worktree is live and
// before the inbox release reads the workspace. Best-effort + loud; a
// system-failure halt still stamps (the preserved work is exactly what the
// next attempt should resume once the pipeline is healthy again).
```

### `go/internal/core/cyclerun.go:324` — above `o.carryover().Closeout(state, cs.WorkspacePath, cycle, time.Now().UTC())`

```text
// The cycle-terminal carryover order — the memo merge, the prescription merge,
// then the triage-dropped retirement — lives in the unit (carryover.Closeout)
// with its own test: retiring BEFORE the merges let a merge resurrect the very
// id triage had just dropped (the cycle-1538 reproduction, pinned at this seam
// by TestFinalizeCycle_RetiresTriageDroppedCarryover).
```

### `go/internal/core/cyclerun.go:383` — above `release := func() error { return nil }`

```text
// ADR-0049 S6 / root-cause R1: under the fleet supervisor (EVOLVE_FLEET=1)
// skip the whole-cycle global project lock (LOCK_NB) so M cycles run
// concurrently instead of refusing each other. Safe because every shared
// resource is now serialized by its OWN flock — state.json (UpdateState /
// withStateLock, S2), the ledger chain (CA.1), the .evolve/ship.lock
// integrator (S5) — and each cycle is isolated by its per-run worktree +
// workspace with run-scoped ship reads (S3) and audit binding (S4). Default
// off → the live sequential loop keeps the global lock, byte-identical.
```

### `go/internal/core/cyclerun.go:441` — above `if os.Getenv(ipcenv.FleetKey) != "" && cs.WorkspacePath != "" {`

```text
// Fleet cycle-state isolation (ADR-0049): under the fleet supervisor two
// lanes run concurrently. Point THIS lane's cycle-state reads+writes at its
// OWN per-run file so a peer lane's Phase/CycleID write never clobbers this
// lane's — the singleton clobber that made a lane's phase-gate (guards.Phase
// reads cycle state) see the wrong phase and stall before audit. os.Setenv
// propagates to every child guard subprocess this orchestrator spawns, so the
// orchestrator and its gate checks agree on this lane's phase. Sequential loop
// (EVOLVE_FLEET unset) keeps the host-global singleton, byte-identical. Cleared
// on exit for hygiene (each fleet lane is its own process, but a reused process
// must not leak a stale override to a later cycle).
```

### `go/internal/core/cyclerun.go:461` — above `guardDisabled := req.DisableWorkspaceGuard`

```text
// Guard against workspace pollution from a prior killed attempt at
// the same cycle number. If `<workspace>/` exists and has files,
// rename to `<workspace>.polluted-<UTCnano>/` BEFORE any phase runs.
// Without this, leftover scout-report.md / build-report.md from the
// killed attempt cause Scout to short-circuit (read pre-existing
// artifacts in seconds instead of redoing discovery) and steer
// downstream phases via the OLD task selection.
// Source incident: cycle-108 meta-loop attempts 1-4 (2026-05-26).
// Opt-out via EVOLVE_DISABLE_WORKSPACE_GUARD=1 — used by tests that pre-seed
// workspace files to simulate phase state, and by operators via the shell
// (captured into req.Env from filterEvolveEnv(os.Environ()) at cycle launch,
// cmd_cycle.go). ADR-0049 N9: read ONLY the per-cycle env SNAPSHOT, never live
// os.Getenv — under concurrent fleet cycles a peer's env (or a mid-flight
// mutation) must not flip this cycle's guard. The launch snapshot already
// carries the operator's shell value, so this is behavior-preserving for the
// live loop while restoring per-cycle isolation.
```

### `go/internal/core/cyclerun.go:485` — above `mainDirtyBaseline := porcelainDirtySet(ctx, req.ProjectRoot)`

```text
// Full main-tree dirty baseline (tracked + untracked) captured BEFORE any
// phase runs. recoverBuildLeak (cycle-160 / Option A) subtracts it so it only
// relocates paths the build introduced, never the operator's pre-existing work.
```

### `go/internal/core/cyclerun.go:490` — above `if wtPath, werr := o.worktree.Create(req.ProjectRoot, cycle); werr != nil {`

```text
// Provision the per-cycle source worktree (ADR-0027): tdd/build write code
// here, isolated from the live tree. cs.ActiveWorktree gates source writes
// in the role-gate and drives worktree-aware ship. Creation remains
// best-effort: on failure the source phases are denied by the role-gate
// (loud, not silent). A created worktree is checked for an occupied copy of
// this fresh cycle identity after the upstream fetch; a collision is fatal
// before state persistence or dispatch. A rejected worktree is preserved
// because Create may have reused it and does not return ownership metadata.
// Safe worktrees are cleaned on cycle exit (after ship has merged the
// worktree→main).
// cs.WorktreeBaseSHA (persisted) is the worktree HEAD at creation == the
// cycle base. After the build phase we soft-reset to it so a committing
// builder's work becomes pending again (see normalizeWorktreeToBase + the
// cycle-156 incident). Persisted in CycleState so the crash-resume path
// can run the same normalize.
// preserveWorktree (ADR-0039 §8, D10 fix): set when a ship-stage failure
// is recorded and cleared only when a later ship attempt succeeds. While
// set, the exit cleanup below SKIPS pruning so audited (possibly
// uncommitted) work survives for recovery — `evolve loop --resume` or an
// explicit `evolve cycle reset` reclaims it. Cycle 7 lost its entire
// PASS work to this prune; cycle 12 survived only via operator snapshot.
```

### `go/internal/core/cyclerun.go:552` — above `stopLease := startRunLease(cs.WorkspacePath, runID, o.now, leaseRefreshInterval())`

```text
// ADR-0049 G16: write + heartbeat the per-run .lease so gc's liveness check
// (runlease.Fresh) never reaps a concurrent fleet sibling's run dir mid-cycle.
// startRunLease creates the run dir itself; no-op for worktree-less / test
// cycles (empty WorkspacePath). Stopped on every exit (deferred).
```

### `go/internal/core/cyclerun.go:568` — above `func (o *Orchestrator) clearActiveWorktree(wtPath string) {`

```text
// clearActiveWorktree drops a PRUNED worktree path from the persisted cycle
// state. cs.ActiveWorktree was write-only until cycle-1278: the teardown above
// deleted the directory but left the record naming it, so the next reader
// (retro's dispatch, resume, checkpoint) handed a deleted path to the bridge,
// whose IsDir guard refuses the launch — the cycle-1255 CRITICAL's root cause.
// Widening retroWorktree's fallback contains that symptom; this removes it.
//
// Read-modify-write against storage rather than rewriting the newCycleRun-era
// local: by teardown the cycle run has persisted phase progress through its own
// CycleState copy, and writing the stale init snapshot back would discard it.
// The path guard makes this a no-op when the record has since moved on (a lane
// re-provisioned, or the field already cleared) — only the tree we just pruned
// gets unnamed. A detached context is deliberate: teardown runs on the exit
// path, where the cycle context is routinely already done (same reason as the
// abnormal epilogue's epilogueCtx).
```

### `go/internal/core/cyclerun.go:718` — above `if ls := loadLaneScope(cs.WorkspacePath); ls != nil {`

```text
// ADR-0049 E + lane-scope pin (cycle-640): the fleet scope every phase sees
// via Context["fleet_scope"] comes from <workspace>/lane-scope.json when a
// supervisor (or a prior attempt of this orchestrator) provisioned one —
// the on-disk pin is authoritative over the env snapshot, so cross-lane env
// drift can no longer split lane identity. Absent file ⇒ legacy env-snapshot
// fallback (sequential loop byte-identical), and an env-scoped run pins its
// own lane-scope.json here, BEFORE any phase runs.
```

### `go/internal/core/cyclerun.go:731` — above `if scope := ctxSnap["fleet_scope"]; scope != "" && o.scopePathFor != nil {`

```text
// Disclose each scoped id's LIVE inbox record beside the id list
// (cycle-1548: a bare name resolved to a two-week-old consumed namesake —
// 17 records shared the id — and every phase worked the cured ghost). Only
// PENDING ids get an entry; carryover/non-inbox ids resolve to "" and are
// silently omitted (fail-open, same as before). Nil resolver = no key,
// Context byte-identical.
```

### `go/internal/core/cyclerun.go:759` — above `if _, alreadySet := ctxSnap["challengeToken"]; !alreadySet {`

```text
// PR 6 (cycle-135 followup): mint the cycle's challenge token here —
// ONCE per cycle, at orchestrator start, BEFORE any phase runs. Surface
// it to every phase via Context["challengeToken"] (scout's ComposePrompt
// reads it at scout.go:64) AND persist it to <workspace>/challenge-
// token.txt so the agent-templates.md PR 5 fallback source is populated.
// Pre-PR-6, no Go code injected the token; scout invented its own
// (cycle 134 audit C1: "no-token-manual-run-cycle-134"; cycle 135 audit
// C1: scout minted `59576594e2e8d5c3` instead of using `5b96ecb69a0c848f`
// from challenge-token.txt). The mint is the same 8-byte-hex shape as
// bridge.defaultChallengeToken so post-cycle ledger entries are
// indistinguishable from the bridge-minted ones used pre-cycle-135.
```

### `go/internal/core/cyclerun.go:801` — above `benchedCLIs := benchedCLIsForRouting(req.ProjectRoot)`

```text
// Upfront whole-cycle plan (ADR-0024 §2). At Stage>=Advisory with a planner,
// ask the advisor once which phases to run, CLAMP the answer to the integrity
// floor (ship⇒build∧audit∧tdd), persist it, and thread the clamped plan into
// every routing decision below. The clamp is the non-bypassable kernel floor:
// it can only COMPLETE the ship-chain, never weaken it, so a hallucinated or
// adversarial plan cannot reach ship without a real build+audit. Any
// failure leaves clampedPlan nil ⇒ routing falls back to the configurable
// never-skip spine (fail-safe to static). Below Advisory, no plan is computed.
// This is the SINGLE gate for the upfront plan: Stage>=Advisory (the advisor
// drives) AND Mode==DynamicLLM (static mode makes no LLM calls) AND a planner
// is wired. The composition root passes WithPlanner unconditionally; the
// Mode check lives here so the invariant ("LLM plan iff DynamicLLM+Advisory")
// has one source of truth rather than two gates that could drift.
// CLI-health snapshot, taken ONCE at cycle start and threaded to both the
// whole-cycle plan input and every per-transition Decide: the advisor and
// the dispatcher must reason from the SAME bench state (review H2 — two
// reads could diverge when a bench expires mid-planning).
```

### `go/internal/core/cyclerun_chronicle_test.go:3` — above `import (`

```text
// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// seed-digest-at-cycle-start). newCycleRun must resolve the chronicle policy
// ONCE and, per stage:
//
//   off     → write nothing, inject nothing (byte-identical cycle start).
//   shadow  → assemble DigestInput (last-N dossiers from
//             knowledge-base/cycles/cycle-*.json, entriesFromRecords(state.FailedAt),
//             best-effort recurrence ledger) and WriteDigest into the run
//             workspace — but do NOT inject Context["recent_outcomes"].
//   enforce → same write, PLUS Context["recent_outcomes"] carries the digest
//             bytes into every phase request (scout/triage render it).
//
// WriteDigest is best-effort: a digest failure logs a WARN to stderr and the
// cycle proceeds (mirrors the archivePollutedWorkspace idiom two blocks away).
//
// API pin (mirrors WithRetryConfig/WithWorkflowConfig, orchestrator.go): the
// resolved policy.ChronicleConfig is injected at the composition root via
// core.WithChronicleConfig; the zero-option default is the compiled default
// (shadow). Builder implements; must NOT modify these tests.
```

### `go/internal/core/cyclerun_correction.go:67` — above `irec := interaction.NewRecorder(cr.cs.WorkspacePath)`

```text
// ADR-0045 I1: a correction re-dispatch is an interaction — every
// rung of ONE correction decision shares a DecisionID, and each
// re-dispatch records an outcome resolved by its verdict + the
// re-review. The I2 ladder's salvage/live-fix rungs will join
// this same decision when they ship.
```

### `go/internal/core/cyclerun_correction.go:77` — above `rungBudget := map[string]int{`

```text
// ADR-0045 I2: graduated correction ladder. The DECISION is the
// pure interaction.NextCorrection CoR (salvage → live_fix →
// redispatch, cheapest first); EXECUTION is stage-gated here.
// Salvage gets budget only when a breaker-neutral verifier is
// wired. Rung 2 (live_fix) is decision-complete but
// execution-dormant at v1: the orchestrator does not yet request
// named sessions, so NamedREPL is hard-false until the session
// request + reaper plumbing lands (the C1→C3 deferred-unification
// precedent; see interaction/correction.go).
```

### `go/internal/core/cyclerun_correction.go:176` — above `escalEligible := rr.Blocks >= contractEscalateAtBlock ||`

```text
// Escalation eligibility has two typed doors, one per rejection class:
//   - rr.Blocks: the deliverable contract gate's own breaker count.
//   - rr.Remediation + the local correction ordinal: a
//     remediation-carrying rejection (set ONLY by evalgate) whose
//     second identical round is starting. The evalgate keeps no
//     breaker, so the in-cycle ordinal is its only honest counter —
//     and constraint 2's desync warning is about the DELIVERABLE
//     breaker's cross-cycle file, which this class does not have.
//     Measured basis (2026-08-23): every eval-materialization
//     failure since cycle-1450 (1471/1476/1504/1531/1540/1545) was
//     one CLI family, 0-for-all correction rounds on that family,
//     including rounds whose directive named the exact writable
//     paths — for the CREATE-a-missing-artifact class, a different
//     CLI is demonstrably the remedy. Remediation-LESS Blocks==0
//     rejections (topngate/triagecap/build floor) keep constraint 3
//     exactly: they never enter here.
```

### `go/internal/core/cyclerun_correction.go:327` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCou…`

```text
// ADR-0044 C1: the phase ran and produced its own verdict;
// the reject is recorded as the abort reason, not a rewrite.
```

### `go/internal/core/cyclerun_dispatch.go:28` — above `if next.IsValid() || isConfiguredMandatory(cr.o.cfg, string(next)) {`

```text
// The routing surface (registry order + catalog) can know phases
// the dispatch surface cannot run (cycle-265: registry-listed
// `memo` had no .evolve/phases config ⇒ no specrunner; the static
// order walked into it post-ship and killed a PASSING batch). A
// non-dispatchable OPTIONAL USER phase is skipped loudly and the
// walk continues; a missing BUILT-IN or configured-mandatory
// runner stays fatal — that is a wiring bug, not routing-surface
// drift (built-ins always have factories; only registry/user
// phases can be known to routing yet unregistered).
```

### `go/internal/core/cyclerun_dispatch.go:56` — above `supersedePreviousAuditRound(&cr.cs)`

```text
// Same supersession rule for the round's verdict ARTIFACTS: a
// re-dispatched audit must not replay the previous round's pre-staged
// acs-verdict.json/audit-report.md through the verdict-exists gate
// (cycle-1603: round-1's ship_eligible=false force-FAILed every
// repaired PASS). First dispatch (round 0) retires nothing, so an
// operator/CI pre-stage keeps its honor.
```

### `go/internal/core/cyclerun_dispatch.go:68` — above `cr.o.ensureFailureDigest(cr.cycle, cr.req.ProjectRoot, cr.cs.WorkspacePath, string(cr.current),`

```text
// S1 assembler, verdict path (ADR-0074 I2; cycle-1046 live gap): the
// failure digest must exist BEFORE the retro agent runs — it is the
// identity the disposition gate cross-checks and the blocker breaker
// reads. Idempotent with the phase-error path in recordFailureLearning.
// Router line + per-failure distinguisher (defect-first) so distinct
// failures never collide (1054/1060 cross-task pin; batch-14
// same-task-distinct-defects pin) — composed in failure_digest.go
// beside the content-free detector it must never drift from.
```

### `go/internal/core/cyclerun_dispatch.go:80` — above `phaseWorktree := cr.cs.ActiveWorktree`

```text
// CB.1 (concurrency campaign W4): EVERY phase runs with cwd = the cycle
// worktree — not just the source writers (tdd/build, role-gate-permitted)
// and audit (issue #9: its verification commands must inspect the
// builder's pending work). A read-only phase's cwd in the main tree let
// stray writes and guard misfires land in the live checkout (cycle-280);
// with the worktree provisioned at cycle start, no phase subprocess
// touches main at all. cwd is NOT write permission: the write axis
// (role-gate / tree-diff guard / normalize) still keys off worktreePhase.
// Empty when provisioning failed — the pre-existing degraded mode.
```

### `go/internal/core/cyclerun_dispatch.go:172` — above `phaseReq.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, next, cr.workflowConfig.PhaseEnables, cr.cs.WorkspacePath)`

```text
// ADR-0050 Phase 3.7: at advisory+, serve the build phase's upstream
// build-plan via the typed envelope (read once here at the seam) instead of
// an ad-hoc disk read inside the phase. Off/shadow leave it empty → the phase
// reads disk as before (byte-identical dispatch).
```

### `go/internal/core/cyclerun_dispatch.go:180` — above `if cr.o.cfg.PhaseIO >= config.StageShadow {`

```text
// ADR-0050 Phase 3.4 (SHADOW) + Phase 3.10 (ENFORCE input). When
// EVOLVE_PHASE_IO>=shadow, assemble the typed Upstream view from the same
// upstream this phase is about to receive, compare it to the legacy routing
// digest, and record any divergence (shadow artifact + ledger). At >=enforce,
// the same pass also returns the authoritative typed PhaseInput the phase
// consumes in place of the legacy Context map. At EVOLVE_PHASE_IO=off (default)
// this is skipped entirely and phaseReq.Input stays the zero value —
// byte-identical dispatch; below enforce the assembled Input is still zero, so
// only the flip to enforce changes what a phase observes.
```

### `go/internal/core/cyclerun_dispatch.go:192` — above `var resp PhaseResponse`

```text
// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
// (default when WithObserver wasn't used) is byte-identical to
// the pre-fix cycle. Real implementations spawn a stall detector
// that watches <workspace>/<agent>-stdout.log and emits stall
// events to <workspace>/<agent>-observer-events.ndjson.
// Self-heal (Fix D): a bridge ArtifactTimeout (exit=81) is the
// recoverable "agent produced no artifact within the wait window" case
// — a stalled launch where a fresh relaunch usually succeeds. Retry the
// phase a bounded number of times on THAT sentinel only; every other
// error (and exhaustion of the budget) aborts the cycle as before. A
// deterministic timeout (e.g. a misconfigured agent) simply fails again
// and aborts after the cap — at most one wasted retry. The observer is
// (re)started per attempt so each launch is watched.
```

### `go/internal/core/cyclerun_dispatch.go:214` — above `var attemptExits []int`

```text
// attemptExits collects each failed attempt's bridge exit code so the
// exhaustion arm can recognize the all-families quota-terminal signature
// (every attempt exit=85 — cycle-656) and checkpoint-and-defer instead of
// failing forward.
```

### `go/internal/core/cyclerun_dispatch.go:253` — above `if retryHooks.quotaExhausted(attemptExits) {`

```text
// All-families quota exhaustion (cycle-656 D2): every attempt
// returned exit=85, so the cross-family failover (cycle-393)
// has no remaining target — quota is a resource that resets in
// hours, not a per-attempt transient. Spending more attempts
// (or degrading an optional phase to WARN and advancing the
// next LLM phase into the same wall) guarantees a FAIL plus a
// quota-consuming retro. Instead: write a quota-likely
// checkpoint (resumeFromPhase = this phase, completed phases +
// worktree preserved) and abort with the typed sentinel so the
// loop stops resumable (rc=5) and classification is DEFERRED,
// not FAILED. Checked FIRST in the arm — before backfill
// (disjoint: exit-81-only) and before optionalInfraSkip, which
// would otherwise fail forward. Single-family 85 with a healthy
// sibling never reaches here all-85 (the sibling attempt's exit
// differs), so normal failover is unchanged.
```

### `go/internal/core/cyclerun_dispatch.go:278` — above `if retryHooks.optionalInfraSkip(next, err) {`

```text
// Optional-phase infra skip (Workstream-D intent on
// ErrArtifactTimeout; cycle-283): an enrichment phase must not
// veto completed spine work. When backfill could not reconstruct
// the artifact, a catalog-Optional, non-floor phase whose
// exhaustion is infra-shaped degrades to a synthesized WARN and
// the cycle advances toward audit/ship. The failed attempts stay
// in failure-learning and the ledger — recovered, never silent.
```

### `go/internal/core/cyclerun_dispatch.go:302` — above `if retryHooks.postShipObserverSkip(next) {`

```text
// Post-ship observer skip (cycle-574 memo-phase-tier-envelope):
// a best-effort RoleControl observer (memo / post-ship-monitor)
// that fails AFTER a healthy ship must not turn a shipped cycle
// abnormal. Unlike optionalInfraSkip this fires on ANY error
// shape (the memo tier/envelope error is a policy error, not
// infra), gated on ship having already landed and the same
// floor/mandatory guards so it can never weaken the integrity
// floor. Record SKIPPED with a warning and advance; the failed
// attempts stay in failure-learning and the ledger — recovered,
// never silent.
```

### `go/internal/core/cyclerun_dispatch.go:318` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, phaseEr…`

```text
// ADR-0044 C1: record the dispatch outcome BEFORE the
// failure-learning retro so the timing record stays
// chronological (failed phase, then retro). No canonical
// agent verdict exists on this path → synthesized FAIL.
```

### `go/internal/core/cyclerun_dispatch.go:324` — above `cr.o.adviseOnUnclassifiedFailure(cr.ctx, cr.cycle, cr.cs.WorkspacePath, cr.req.ProjectRoot, next, err, cr.envSnap)`

```text
// ADR-0044 C3: enforce-only, best-effort — classify the
// unclassified pane via the LLM tail and promote, so the
// NEXT occurrence is deterministic. Never alters the abort.
```

### `go/internal/core/cyclerun_dispatch.go:347` — above `if degraded, ok := cr.o.nonFloorExhaustionDegrade(next, cr.cs.WorkspacePath, cr.o.floorAlreadyCompleted(cr.cs.CompletedP…`

```text
// Cycle-802 Task 3 (contract-exhaustion-degrades-non-floor,
// subsumes advisory-phase-contract-degrade): an unparseable
// verdict after retries exhausted is cycle-fatal ONLY for a
// floor/ship phase. A non-floor post-verdict phase degrades to
// SKIPPED+WARN and the cycle advances — recordFinalVerdict then
// records the degrade into VerdictsNotAdopted without clobbering the
// floor verdict, closing the same storm from the contract side.
```

### `go/internal/core/cyclerun_dispatch.go:370` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, ferr.Er…`

```text
// ADR-0044 C1: a non-canonical verdict is never recorded
// raw and never upgraded — phaseOutcomeFrom synthesizes FAIL.
```

### `go/internal/core/cyclerun_epilogue.go:3` — above `import (`

```text
// cyclerun_epilogue.go — the abnormal-exit epilogue (cycle-1048): NO exit path
// may leave a started cycle without its evidence trail. The loopAbort path
// returned immediately, skipping finalizeCycle — so the seal, dossier, and
// coherence floors never ran, and every downstream detector (monitors read
// dossiers; floors run in finalize) was blind to a cycle the loop itself had
// already logged as failed. Detectors must not depend on artifacts produced
// by the failure path they monitor: visibility is guaranteed by construction
// here (a defer), not by the success of the thing being watched.
```

### `go/internal/core/cyclerun_epilogue.go:36` — above `func (cr *cycleRun) abnormalEpilogue(cause error) {`

```text
// abnormalEpilogue is deferred by RunCycle and fires ONLY when the cycle did
// not reach the normal closeout (cycleCompletedNormally=false). A graceful
// cancellation is a resumable pause and is checkpointed separately, so it
// must not create or commit terminal failure evidence. Best-effort + loud:
// the hard-failure path must never mask the original error, and each step
// tolerates the others failing.
//
// cause is the error RunCycle is about to return — the abort's ONE
// distinguishing fact (nil on a bare bounce AND on a panic, which reaches
// this defer with retErr unset: both stay honestly Unexplained). Appended to
// the constant template it makes the digest content-bearing and
// cause-distinct; without it three distinct same-phase aborts share one
// Unexplained fingerprint and the diagnosability breaker's only move is
// halting the batch (batch-19 cycle-1208: 1197/1199/1207 all
// "build|unknown|7c02ce1f4f95").
//
// Teardown-shaped causes (IsInfraTeardownError: artifact timeout, transient
// bridge death) are marked "teardown=" instead of "cause=". Their identical
// fingerprints STAY in the identical-fingerprint population DELIBERATELY
// (review HIGH, decision pinned): one systemic infra condition mowing down
// three lanes is exactly the recurring-infra shape ADR-0072's halt doctrine
// wants stopped at the ceiling — the marker makes that shape legible in the
// halt message instead of reading as "identical defects".
```

### `go/internal/core/cyclerun_epilogue.go:93` — above `sealed := cr.result`

```text
// ADR-0101 S2a: the cycle's event stream ends with a seal on this path
// too — FAIL, the abort reason as the termination reason — so "how did
// cycle N end" has one answer in signals.ndjson on every path.
```

### `go/internal/core/cyclerun_epilogue.go:102` — above `cr.o.stampContinuationManifest(epilogueCtx, cr.cs, cr.cycle, cr.req.ProjectRoot)`

```text
// ADR-0076 slice C (G1, cycle-1078): error-path aborts never reach
// finalizeCycle, so the preserved worktree would carry no continuation
// manifest and the resumption machinery would have nothing to bind. Stamp
// here too — after the failure digest above, so FindingsPath has content.
// Idempotent with the finalize-path stamp; no-op when no worktree exists.
```

### `go/internal/core/cyclerun_epilogue_test.go:3` — above `import (`

```text
// cyclerun_epilogue_test.go — cycle-1048 pins: every started cycle leaves a
// dossier + digest + coherent state on EVERY exit path. The abort path
// (`return cr.result, err` at loopAbort) skipped finalizeCycle, leaving a
// two-hour stale phase=retro record and a dossier-less, monitor-invisible
// failure.
```

### `go/internal/core/cyclerun_postreview.go:37` — above `if latchShippedState(&cr.cs, next, dr.resp.Verdict) {`

```text
// cr.cs.Shipped is also the latch read by postShipObserverSkip on the
// dispatch abort path (a post-ship observer failure degrades to WARN rather
// than turning a shipped cycle abnormal — cycle-574) and by the outcome
// label at closeout; it lives on cr.cs so the next persist checkpoints it.
```

### `go/internal/core/cyclerun_postreview.go:42` — above `cr.preserveWorktree = false`

```text
// Ship landed AND survived the deliverable review gate above — the
// worktree is merged, normal exit cleanup applies. Deliberately
// AFTER the review gate: a review-rejected ship abort must still
// preserve the worktree for triage (ADR-0039 §8 / D10).
```

### `go/internal/core/cyclerun_postreview.go:82` — above `leaked := res2.Leaked`

```text
// Filter the leaked set through isLegitimateMainTreePath for EVERY
// phase — the same classification recoverBuildLeak applies (R9: one
// shared vocabulary). Non-worktree phases need it for their
// .evolve/ workspace writes (R7); worktree phases need it because
// orchestrator-side gates write their own untracked runtime state
// (.evolve/contract-gate-breaker.json) into the main tree mid-phase
// — recovery skips those by design, so a strict guard here turned
// every contract-gate trip into a false cycle abort (the cycle-274
// salvage CI regression). PLUS a guard-only second classifier,
// isScoutEvalMaterialization: scout writes its selected evals to the
// main tree by contract (materialization.go), which recoverBuildLeak
// never sees (scout is not a WorktreePhase) so it lives only here
// (soak-#6 cycle 318→319). PLUS a third guard-only classifier,
// isActiveMintPhasePath: in fleet mode a CONCURRENT lane's advisor
// mint persists .evolve/phases/<name>/phase.json into the SHARED
// tree this lane diffs, charging the mint to an innocent phase
// (cycle-967 false-abort). The registrar records minted names in
// the shared mintregistry before persisting, so a registered,
// TTL-fresh name is mint infrastructure, not a leak; an
// UNREGISTERED phase-config write still aborts. A registry read
// error only disables the exemption (guard stays armed — the
// fail-safe direction). Real escapes stay armed: source files and
// non-scout/non-eval deliverable paths classify as leaks, and
// porcelainDirtySet emits both rename sides so a deliverable renamed
// to a .evolve/evals/ look-alike still aborts via its source path.
```

### `go/internal/core/cyclerun_postreview.go:110` — above `fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL tree-diff: mint registry unreadable (%v); mint exemption disabled for th…`

```text
// ABNORMAL, not WARN: a corrupt registry is either damage or a
// deliberate availability attack (a lane-wide exemption outage
// reproduces the cycle-967 false-abort). Quarantine bounds the
// outage to this one check; the guard stays armed either way.
```

### `go/internal/core/cyclerun_postreview.go:135` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCou…`

```text
// ADR-0044 C1 — THE cycle-262 path: the build ran, PASSed,
// and burned tokens before the guard caught its main-tree
// leak. The abort is correct; erasing the outcome was not.
```

### `go/internal/core/cyclerun_record.go:33` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCou…`

```text
// ADR-0044 C1: the phase completed; a persistence failure must
// not erase its outcome from the timing/usage record.
```

### `go/internal/core/cyclerun_record.go:39` — above `if next == PhaseAudit && (dr.resp.Verdict == VerdictPASS || dr.resp.Verdict == VerdictWARN) {`

```text
// Cycle-778 ship-window lease: audit's binding snapshot (`git rev-parse
// HEAD` inside emitPhaseBindings→recordAuditBinding) opens the window a
// sibling landing on main would turn into a deep-tier re-audit — acquire
// BEFORE a shippable snapshot; any later completed phase (normally ship, after its
// push) releases below. No-op for every phase but audit; fail-open.
```

### `go/internal/core/cyclerun_record.go:60` — above `if next == PhaseBuild {`

```text
// Cycle-636 (ship-sha-repin-after-build): close the frozen-pin
// SELF_SHA_TAMPERED cascade (denied ship on 625->634). A legitimate in-version
// rebuild replaces go/bin/evolve, but the cycle-514 boot healer only re-pins at
// boot — so re-pin here too, immediately after a successful build, through the
// SAME provenance-gated primitive (phaseintegrity.RepinIfDrifted) the boot path
// uses. Fail-open: refusal/error WARNs; the ship gate stays the backstop.
```

### `go/internal/core/cyclerun_record.go:69` — above `if reason := buildGraduationCheck(cr.ctx, cr.cs.ActiveWorktree); reason != "" {`

```text
// Cycle-675 (new-package-graduation-buildentry-gate, 3rd recurrence):
// a go/internal package NEW this cycle and absent from
// go/.apicover-enforce fails the build phase HERE, with an explicit
// abort_reason — after the pre-review worktree normalize (so a committing
// builder's work is pending again) and before the phase is marked
// completed. Deliberately abort-capable, unlike the WARN-only
// buildSelfCheck: graduation is the builder's own obligation, and the
// audit-side twin (apicoverNewPackageGraduationDefault) firing two
// attempts later is exactly the recurrence this closes.
```

### `go/internal/core/cyclerun_record.go:106` — above `if cr.current == PhaseAudit && dr.resp.Verdict == VerdictFAIL {`

```text
// Audit-FAIL disposition (retry + retro redesign). Decided HERE, at the audit
// chokepoint, from the audit's OWN declared failure class and the ADR-0072
// policy table — not after a full retrospective, and not from agent prose. The
// table has always declared code-audit-fail as {task, retry-with-fix,
// MaxRetries: 2}; nothing consumed it until now.
//
// scheduledNext is the authoritative-injection seam, so the dynamic-routing
// override cannot second-guess this decision — the same protection the retro
// branch has, and structurally the fix for the class of defect where a router
// silently ate a granted repair.
```

### `go/internal/core/cyclerun_record.go:132` — above `if cr.o.successorStrategy(cr.current) == phasespec.BranchingHistory {`

```text
// Retro is the one phase whose successor isn't verdict-driven: the
// failure-adapter consults cycle history (state.FailedAt) and the retro
// verdict to pick {ship | tdd | end}. Set scheduledNext so the next loop
// iteration runs the chosen phase. The history-branch gate is config-driven
// (ADR-0058) — successorStrategy resolves the completed phase's
// branching_strategy and owns the byte-identity degrade.
```

### `go/internal/core/cyclerun_record.go:143` — above `cr.cs.FailedAt = cr.state.FailedAt`

```text
// Carry the cross-cycle failure history onto the per-cycle checkpoint so
// the ADR-0072 S4 dossier composes its non-progress counters from live
// evidence (additive; keeps the judgment layer non-inert).
```

### `go/internal/core/cyclerun_record.go:177` — above `if gateErr := cr.o.finalizeRetroCompletion(cr.cs.WorkspacePath); gateErr != nil {`

```text
// S2 disposition gate, verdict path (mirrors recordFailureLearning's
// contract — cycle-1046 live gap): an absent/invalid disposition is
// surfaced loudly in RetroDecision, never silently recorded clean.
```

### `go/internal/core/cyclerun_record.go:185` — above `if sysFail != nil && cr.result.SystemFailure == nil {`

```text
// ADR-0072 S4: a floor category classified at the retro chokepoint is a
// SYSTEM-level failure — mark it so the batch loop HALTS + escalates for
// pipeline diagnosis instead of re-selecting the same task. finalizeCycle
// sees SystemFailure already set and skips its own re-detection.
```

### `go/internal/core/cyclerun_record.go:201` — above `if cr.o.successorStrategy(cr.current) == phasespec.BranchingSignal {`

```text
// The debugger phase is decision-driven (RESHIP / RERUN_PHASE / BLOCK), not
// verdict-driven — mirror the retro branch. The debugger runner surfaces its
// decision on PhaseResponse.Signals; decideAfterDebugger maps it to the next
// phase, which the next iteration runs via scheduledNext. The signal-branch
// gate is config-driven (ADR-0058) — successorStrategy resolves the debugger's
// branching_strategy from the builtinControlSpec seam and owns the degrade.
```

### `go/internal/core/cyclerun_remediate.go:3` — above `import (`

```text
// cyclerun_remediate.go — graduated remediation (operator directive
// 2026-07-21; inbox graduated-remediation-fix-forward, ADR pending): when a
// configured DETERMINISTIC gate phase returns a FAIL verdict, dispatch the
// builder ONCE per round with the gate's report as a correction directive,
// then re-run the SAME gate and adopt its fresh verdict. The economics: a
// mechanical, gate-prescribed defect (missing tests, format, naming) costs a
// bounded in-phase fix (~1-2M tokens) instead of discarding a sound cycle
// (~12.5M) — the 983/992/1007/1019/1020 waste class, capped by cycle-1019
// where the audit-PASSed ADR-0072 S5 implementation was thrown away over
// three missing test files the gate itself had prescribed, and cycle-1020
// then re-implemented it from scratch and failed the same gate the same way.
//
// Integrity floors:
//   - the SAME gate must pass — remediation never overrides a verdict, it
//     re-earns one; every downstream phase (audit, EGPS, ship gates) runs
//     unchanged after it;
//   - the round cap is hard (config workflow.remediation_rounds, default 1 at
//     the composition root; ZERO in core's zero-value config so untouched
//     tests and legacy paths are byte-identical);
//   - only phases listed in workflow.remediable_phases participate —
//     deterministic gates only by contract; judgment phases must never be
//     listed;
//   - provenance is loud: CycleResult.Remediations records every round and
//     outcome, so a remediated cycle is never a silent PASS.
```

### `go/internal/core/cyclerun_remediate.go:85` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCou…`

```text
// Record the ORIGINAL failing gate attempt through the ADR-0044 C1
// chokepoint before anything else — the re-run gets its own window below,
// so the record honestly shows gate-FAIL, fix, gate-rerun.
```

### `go/internal/core/cyclerun_remediate.go:126` — above `fixAbort := ""`

```text
// ADR-0044 C1: the fix dispatch burned tokens — record it whatever happens,
// under its own label so it never clobbers the build phase's own records.
```

### `go/internal/core/cyclerun_remediate.go:184` — above `if act, err := cr.reviewAndGuard(next, dr); act == loopAbort || err != nil {`

```text
// ADR-0100 §4: the re-run's deliverable meets the same reviewer the
// original did. Before this it reached recordAndBranch on the strength of
// its verdict alone, so a re-run that omitted a declared deliverable — or
// failed any contract check — was recorded as if reviewed.
```

### `go/internal/core/cyclerun_remediate_test.go:3` — above `import (`

```text
// cyclerun_remediate_test.go — graduated remediation (operator directive
// 2026-07-21; inbox graduated-remediation-fix-forward): when a configured
// DETERMINISTIC gate phase FAILs, the orchestrator dispatches the builder ONCE
// with the gate's report as a correction directive, re-runs the SAME gate, and
// records the final verdict — instead of discarding a sound cycle over a
// mechanical, prescribed defect (the 983/992/1007/1019/1020 waste class:
// cycle-1019's audit-PASSed S5 implementation was thrown away over three
// missing test files the gate itself had prescribed; 1020 then re-implemented
// it from scratch and failed the same gate the same way).
//
// Integrity properties pinned here: nothing downstream is bypassed (the SAME
// gate must pass and the spine continues normally); the round cap is hard; a
// zero-value workflow config means ZERO remediation (byte-identical legacy
// behavior — compiled defaults live at the composition root, not in core).
```

### `go/internal/core/cyclerun_remediate_test.go:209` — above `func TestRemediation_RecordsFixDispatchInPhaseRecord(t *testing.T) {`

```text
// TestRemediation_RecordsFixDispatchInPhaseRecord pins the ADR-0044 C1
// chokepoint parity: the remediation fix dispatch appears in the phase record
// under its own label (never clobbering the build phase's own records).
```

### `go/internal/core/cyclerun_remediate_test.go:229` — above `type diagRunner struct{ name Phase }`

```text
// diagRunner returns a FAIL with error-severity diagnostics — the audit
// in-process override shape (cycle-1022).
```

### `go/internal/core/cyclerun_remediate_test.go:239` — above `func TestFailReasonsSurfaceInResult(t *testing.T) {`

```text
// TestFailReasonsSurfaceInResult pins the cycle-1022 lesson: a floor-override
// FAIL's explanation must reach the RESULT (summary + dossier surfaces), not
// just workspace artifacts and orchestrator memory.
```

### `go/internal/core/cyclerun_remediate_test.go:261` — above `func TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot(t *testing.T) {`

```text
// TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot is the core half
// of the worktree fence (ADR-0097): every dispatched request carries
// WorktreeReadOnly derived from the ONE write-permission predicate
// (worktreePhase) — read-only phases (scout, audit) true, the declared source
// writers (tdd, build) false — and a remediation builder fix never inherits
// the fenced gate's flag (a fenced builder would have its fix silently
// undone). Uses scout as the remediable read-only gate so the inherited-flag
// path is actually exercised.
```

### `go/internal/core/cyclerun_replan.go:12` — above `type rePlanner interface {`

```text
// cyclerun_replan.go — the ADR-0052 WS2 post-scout re-plan.
```

### `go/internal/core/cyclerun_replan.go:14` — above `type rePlanner interface {`

```text
// rePlanner is the OPTIONAL re-invokable extension of router.Planner: a planner
// that can produce a SECOND, post-scout plan from measured signals (ADR-0052
// WS1-S3 RePlan). The composition root's PhaseAdvisor implements it; the re-plan
// type-asserts the wired planner to it and no-ops when the planner is not
// re-invokable (fail-safe to the initial plan). Kept separate from router.Planner
// so a non-re-invokable planner (e.g. a scripted test proposer) need not grow a
// RePlan method.
```

### `go/internal/core/cyclerun_replan.go:32` — above `func (cr *cycleRun) postScoutReplan() {`

```text
// postScoutReplan is the WS2-S0 hook point + WS2-S3 shadow body (ADR-0052):
// invoked once per cycle immediately after scout's handoff has been recorded
// (CompletedPhases appended + cycle-state persisted + phase-boundary checkpoint,
// all inside recordAndBranch) and BEFORE the next selectNext. Firing post-record
// is precisely what keeps the re-plan from widening the run-set or bypassing
// SpineSatisfiedUpTo — the completed scout anchor already exists when it runs.
//
// SHADOW (this slice, EVOLVE_ROUTER_REPLAN=shadow default): the re-plan is
// computed from MEASURED scout signals, clamped to the integrity floor, and
// recorded (phase-replan.json) for soak diffing — but the cycle keeps driving on
// the INITIAL clampedPlan; static still wins. WS2-S6 flips to a swap at
// EVOLVE_ROUTER_REPLAN=advisory. Off ⇒ nothing (byte-identical). Every failure
// path (not re-invokable, no signals, RePlan error) fails safe to the initial plan.
```

### `go/internal/core/cyclerun_replan.go:112` — above `if cr.o.cfg.RouterReplan == config.StageAdvisory {`

```text
// WS2-S6 advisory flip (the one behavior change, opt-in): at
// EVOLVE_ROUTER_REPLAN=advisory the re-plan REPLACES the drive plan — but only
// the CLAMPED re-plan. ClampPlanToFloorWith ran just above and re-asserts the
// integrity floor (ship⇒build∧audit∧tdd) on the re-plan path, so a re-plan can
// NEVER weaken ship; the clamp is the sole trust boundary (ADR-0052 D1).
// registerMintedPhases is idempotent — its runner-existence guard skips any
// phase the stage-1 plan already wired (runners/catalog/routing all gated on
// that check), so re-minting A while minting B leaves A once and adds B once.
// Below advisory (shadow, the default) the re-plan is recorded only — static
// still drives, so nothing flips silently.
```

### `go/internal/core/cyclerun_replan.go:128` — above `func (cr *cycleRun) recordReplanEscalation(maxDepth int) {`

```text
// recordReplanEscalation appends a forensic marker (ADR-0052 WS2-S5) when the
// re-plan depth cap is hit: the cycle escalates rather than re-planning again, so
// a persistent mismatch surfaces to the operator/debugger instead of looping.
// Best-effort — a ledger failure WARNs and is swallowed.
```

### `go/internal/core/cyclerun_replan_rejections_test.go:15` — above `const (`

```text
// Cycle-1155 RED contract — replan-rejections-telemetry.
//
// The enforcement half is already shipped: ClampPlanToFloorWith drops
// unknown-phase entries on BOTH the upfront (cyclerun.go) and the post-scout
// re-plan (cyclerun_replan.go) path. The telemetry half is not:
// router.ValidatePlan has exactly one call site (cyclerun.go:676), so a re-plan
// that hallucinates a phase is dropped SILENTLY — zero forensic trail, which is
// precisely what dropUnknownPhases' own doc comment (floor.go:151) exists to
// close. Compounding it, recordPlanRejections writes advisor-rejections.json
// UNCONDITIONALLY, so a naive second call site would overwrite the upfront
// record instead of accumulating.
//
// These tests pin BEHAVIOUR, not a file format. The builder is free to
// accumulate as a kind-keyed object in advisor-rejections.json, as sibling
// advisor-rejections-<kind>.json files, or any other shape — the assertions go
// through collectWorkspaceRejections/replanRecordPresent, which read every
// advisor-rejections*.json in the workspace and recover rejections from any
// nesting, attributing each to a plan-kind via the filename and the JSON keys on
// its path. What is NOT negotiable: a re-plan's rejections must be recoverable
// and attributable to the re-plan, and recording them must not destroy the
// upfront plan's record.
```

### `go/internal/core/cyclerun_replan_test.go:14` — above `func TestCycleLoop_PostScoutHookFiresOncePreBuild(t *testing.T) {`

```text
// TestCycleLoop_PostScoutHookFiresOncePreBuild pins the WS2-S0 hook call site
// (ADR-0052): the post-scout re-plan hook fires EXACTLY ONCE per cycle, after
// scout's handoff has been recorded (scout ∈ CompletedPhases) and BEFORE build
// (build ∉ CompletedPhases) — the pre-build ordering that lets the re-plan run
// without contradicting a completed anchor. Uses the postScoutReplanProbe DI seam.
```

### `go/internal/core/cyclerun_review.go:57` — above `func filterRealLeaks(next Phase, leaked []string, mints, leased map[string]bool, warn io.Writer) (real []string, waived …`

```text
// filterRealLeaks applies the tree-diff guard's classifier chain to the
// leaked set: workspace legitimacy, scout eval materialization, registered
// TTL-fresh mints, and — LAST, loud (ADR-0080 S4) — the cycle-start-adopted
// console lease, which waives EXACT paths only and prints one WARN per
// waiver so a leased leak can never masquerade as a clean phase.
```

### `go/internal/core/cyclerun_select.go:117` — above `if next != PhaseEnd && !cr.o.sm.SpineSatisfiedUpTo(next, signals, cr.o.cfg) {`

```text
// Full spine-integrity check on the SELECTED next (static OR
// override). R5 (cycle-283 fix): the gate now fails CLOSED at
// EVOLVE_PHASE_RECOVERY=enforce when the absence is CLEAN —
// Digest distinguishes a transient read miss (DigestDegraded)
// from a genuine gap, which was the original fail-open
// rationale. Sequence: re-digest once (the artifact may have
// landed between the routing digest and this check); a
// still-unsatisfied spine with a degraded digest, or any miss
// below enforce, keeps the loud-WARN fail-open (shadow =
// byte-compatible until the R8.5 dial flip); a clean absence
// at enforce aborts FAILED-EXPLAINED with the worktree
// preserved. The operator waiver stays cfg.Mandatory
// (isConfiguredMandatory) — no new escape hatch.
```

### `go/internal/core/cyclerun_select.go:141` — above `case cr.o.cfg.SpineFloor == config.StageEnforce && cleanAbsence:`

```text
// Transient: the handoff appeared on re-read. Proceed.
// (dec was decided from the stale signals — diagnostic
// record only; the gate, not Decide, owns blocking.)
// R8.5 (2026-07-16): the abort keys on the spine floor's OWN
// dial (SpineFloor, default enforce), NOT PhaseRecovery — that
// dial is overloaded (bidirectional channel + failure-adviser
// promotion) and stays shadow. See config.RolloutStages.SpineFloor.
```

### `go/internal/core/cyclerun_select.go:177` — above `missing, ok := cr.o.sm.UnsatisfiedSpineAnchor(next, signals, cr.o.cfg)`

```text
// …and record it (cycle-1166): stderr is not a surface an
// operator or a later sweep can count. The anchor comes
// from the SAME signals the gate blocked on (not the
// re-digest, which may differ) so the record explains THIS
// fail-open. The reporter is the gate's exact complement,
// so ok is true here; "" would be a contradiction, not a
// reason to drop the event — record it as unknown instead.
```

### `go/internal/core/cyclerun_worktree_teardown_test.go:3` — above `import (`

```text
// cyclerun_worktree_teardown_test.go — cycle-1278
// `retro-fleet-stale-worktree-fallback`, AC2 (the root-cause companion).
//
// cs.ActiveWorktree = wtPath (cyclerun.go:515) is the SOLE assignment; nothing
// clears it. When the lane teardown callback prunes the worktree
// (o.worktree.Cleanup, cycle_worktree_teardown.go:53 — the rule both RunCycle
// and RunCycleFromPhase now apply) the persisted cycle state keeps pointing
// at the now-deleted directory, and the next dispatch to read that file hands the
// stale path to the bridge — where isDir() refuses the launch. Widening
// retroWorktree's fallback (AC1) contains the symptom; clearing the field at
// teardown removes the source.
//
// These drive the REAL production seam: newCycleRun is what RunCycle calls, and
// the closure it returns is the one RunCycle defers. The assertion is on the
// PERSISTED cycle state (fakeStorage.cycleState — the last WriteCycleState), not
// on an in-memory local, because the persisted file is what a later dispatch
// actually reads.
```

### `go/internal/core/cyclerun_worktree_teardown_test.go:31` — above `st := &fakeStorage{state: State{LastCycleNumber: 1277}}`

```text
// cycle 1278
```

### `go/internal/core/cyclerun_worktree_teardown_test.go:65` — above `func TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree(t *testing.T) {`

```text
// TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree is the negative axis,
// and it is load-bearing: `evolve loop --resume` and `evolve cycle reset` reclaim
// a preserved lane BY that path. Clearing it unconditionally would trade a stale
// path for permanently orphaned audited work — the cycle-7 lost-work incident.
```

### `go/internal/core/debugger_gate_test.go:3` — above `import (`

```text
// debugger_gate_test.go — PA-BIG S3 (ADR-0058): the debugger decision-branch
// gate is config-driven, mirroring the retro history gate (S2). The debugger is
// a CONTROL phase with no registry home, so its branch metadata
// (branching_strategy: signal) comes from the builtinControlSpec seam
// (ADR-0058 §5), overlaid by Orchestrator.specFor with registry precedence and
// degrading to the literal phase-identity default (debugger→signal) backstop.
```

### `go/internal/core/decision_branch.go:45` — above `detNext, extraEnv, detReason, sig := o.decideAfterRetro(cs, retroVerdict, history)`

```text
// Deterministic baseline: branch, kernel-owned SetEnv, the operator-facing
// reason contract ("proceed:"/"retry-with-fallback:"/…) that dashboards and
// scenario pins grep for, and the ADR-0072 S4 floor signal (non-nil ⇒ a
// floor category was detected — see decideAfterRetro).
```

### `go/internal/core/decision_branch.go:50` — above `if sig != nil {`

```text
// NOTE: there is deliberately no retro-PASS early return here. The previous one
// returned `nil` for the signal, DISCARDING a deterministic ADR-0072 floor
// candidate whenever the retrospective happened to be well written — a system
// failure could escape the "non-bypassable" halt by writing a good post-mortem.
// Retro is reached only from an audit FAIL, so a PASS on this arm is the
// PHASE's verdict, never the cycle's: it is a failure branch like any other and
// takes the same floor → regrade → router path.
// ADR-0072 S4 (F1): the Go floor sits ABOVE the router. A floor category
// HALTS even when the routing strategy would propose a retry — "orchestrator
// decides, Go enforces floor". Enforced here, before o.strategy.Decide, so a
// routed tdd upgrade can never survive a floor category.
```

### `go/internal/core/decision_branch.go:104` — above `func (o *Orchestrator) applyFailureDecisionFloor(cs CycleState, retroVerdict string) *SystemFailureSignal {`

```text
// applyFailureDecisionFloor is the ADR-0072 S4 Go floor at the retro-branch
// chokepoint. It builds the evidence dossier, writes it for per-cycle forensics,
// and returns a HALTING SystemFailureSignal when EITHER the deterministic dossier
// candidate OR the orchestrator's own failure-decision.json classifies the cycle
// into a floor category (verdict-incoherence / infra-systemic). A proposed retry
// cannot survive a floor category — the deterministic candidate is checked first
// (caught even with no orchestrator running), then the orchestrator's judgment.
// Returns nil when no floor bites; the caller then routes / falls back normally.
```

### `go/internal/core/decision_branch.go:126` — above `dec, _ := readFailureDecision(cs.WorkspacePath)`

```text
// (2) Orchestrator judgment — a floor-category classification halts even when
// its own proposed action is a retry (the F2-b cycle-1001 shape).
//
// NARROWED (wave-3 cycles 1572/1573/1574): this gate consumes PROSE. When the
// deterministic gate is silent AND the same agent's own disposition.json says
// legit-rejection, the agent has contradicted itself, and prose alone does not
// outrank two corroborating deterministic signals. Every other shape — an
// uncontradicted claim, or an absent/indeterminate/unverifiable disposition —
// halts exactly as before.
//
// This is the CORROBORATION half of ADR-0092. Its retry half is gone: retries
// are now decided at the audit chokepoint from the audit's own declared class
// and the ADR-0072 policy table (audit_fail_decision.go), so this gate no
// longer gates anything but the halt it was always about.
```

### `go/internal/core/decision_branch.go:148` — above `if dec.Category == policy.CategoryVerdictIncoherence && hasSubstantiveFailReasons(cs) {`

```text
// Cycle-1603: a prose claim of verdict-incoherence is ALSO contradicted when
// the recorded FAIL carries persisted substantive fail reasons. The
// deterministic detector already adjudicated exactly this question and
// declined via its SubstantiveError guard — a diagnosed gate downgrade
// (there: the EGPS ship_eligible=false override over a stale repair-round
// acs-verdict.json) is a justified negative verdict, not a forgery. Prose
// alone does not outrank that deterministic evidence; the cycle stays a
// task-level FAIL and routes through retro normally. Loud, not silent —
// failure-decision.json still asserts the category on disk, so an operator
// tracing a missing halt needs the overrule on record.
```

### `go/internal/core/decision_branch.go:175` — above `cycleVerdict := retroVerdict`

```text
// A retro verdict answers "is the post-mortem deliverable complete?" — retro.go
// computes it as "retrospective non-empty AND a failure-lesson exists". It has
// never answered "did the cycle recover", and this branch used to read it as if
// it did, short-circuiting a retro PASS straight to ship
// ("retro-recovered: ship", pinned since 2026-05-23).
//
// It cannot be recovery. Retro is reached ONLY from an audit FAIL, and the retro
// persona is read-only outside its own artifacts — so the tree ship would commit
// is BYTE-IDENTICAL to the one the auditor rejected. The route looked viable only
// while ship's audit binding could be satisfied by another cycle's PASS entry;
// once #503 made ship fail closed with CodeAuditBindingVerdictFail, its only
// possible outcome became a guaranteed ShipError. See
// retro_verdict_semantics_test.go and
// docs/architecture/retry-architecture-review-2026-08-27.md.
//
// So a retro PASS now falls through to the SAME ladder a retro FAIL takes. The
// cycle's verdict is what the floor and the dossier must see: retro is reached
// only on failure, so a PASS here is the PHASE's verdict, never the CYCLE's.
// Substituting keeps every FAIL/WARN path byte-identical (CheckVerdictCoherence
// receives exactly what it received before) and stops a well-written
// retrospective from making a failed cycle look coherent.
```

### `go/internal/core/decision_branch.go:200` — above `s := o.applyFailureDecisionFloor(cs, cycleVerdict)`

```text
// ADR-0072 S4: the Go floor is the FIRST disposition of a failed cycle — a
// floor category (verdict-incoherence / infra-systemic) HALTS before any
// adapter or router branch, the non-bypassable "Go enforces floor" boundary
// that applies to every stage and the resume path alike.
```

### `go/internal/core/decision_branch.go:322` — above `o.emitShipError(cycle, cs, se, artifactPath)`

```text
// ADR-0101 S2a: the recorded error is also the ship.error signal.
```

### `go/internal/core/decision_branch.go:391` — above `func (o *Orchestrator) recordPlanRejections(ctx context.Context, cycle int, cs CycleState, rejections []router.PlanRejec…`

```text
// recordPlanRejections persists the WS2-S1 ValidatePlan findings to
// advisor-rejections.json (ADR-0052 WS2-S2). STANDALONE telemetry — decoupled
// from the WS3-S3 decision span and from phase-plan.json — and best-effort /
// fail-open: a capture failure WARNs but never affects the cycle. It NEVER
// mutates the plan; the integrity floor (ClampPlanToFloorWith) remains the sole
// disposer. An empty finding set still writes ("[]" = validated-clean, distinct
// from "validation never ran"); nil ⇒ [] so the artifact is always well-formed.
// The artifact is hash-bound into the ledger like every sibling decision
// artifact (recordPhasePlan / recordRoutingDecision), so a post-hoc mutation is
// tamper-evident — "standalone" means a separate file, not outside the chain.
//
// This is the back-compat entry point (kind "plan" → advisor-rejections.json);
// the post-scout re-plan records via recordPlanRejectionsKind with "replan-<n>".
```

### `go/internal/core/decision_branch_bind_test.go:14` — above `func sha256Hex(t *testing.T, path string) string {`

```text
// WS3-S2 (ADR-0052): recordPhasePlan must hash-bind the WS3-S1 capture
// artifacts (advisor-prompt-plan.txt / advisor-response-plan.txt) into the
// ledger, so a post-hoc mutation of a persisted routing prompt/response is
// detectable. The binding reuses the existing ArtifactPath+ArtifactSHA256
// shape, one bound entry per artifact; the ledger's hash chain then carries
// the tamper-evidence.
```

### `go/internal/core/decision_branch_floor_test.go:3` — above `import (`

```text
// decision_branch_floor_test.go — cycle-1002 RED contract for ADR-0072 S4
// Task 3 (wire-floor-override-consumption). decideAfterRetro and
// decideAfterRetroRouted gain a 4th return value (*cyclestate.SystemFailureSignal)
// and consume failure-decision.json under a Go-floor override:
//
//   - a floor category (verdict-incoherence / infra-systemic) HALTS even when
//     the orchestrator decision (or the router) proposes a retry — the override
//     must bite in the LIVE routed path, ABOVE the router (F1);
//   - the cycle-1001 audit-declared system class halts both deterministically
//     (dossier candidate, dec absent) and via judgment (dec says halt);
//   - with no artifact and no floor, the branch/env/reason fall back
//     BYTE-IDENTICAL to failureadapter.Decide (R4 regression guard).
//
// These fail RED until Builder adds the 4th return + applyFailureDecisionFloor.
```

### `go/internal/core/decision_branch_floor_test.go:67` — above `func TestDecideAfterRetroFloor_Cycle1001DeterministicHalt(t *testing.T) {`

```text
// F2 / R6-a — the cycle-1001 shape caught DETERMINISTICALLY (orchestrator
// absent). The audit self-declared a structured system class; the dossier
// candidate is infra-systemic; with no failure-decision.json the Go floor still
// halts.
```

### `go/internal/core/decision_branch_floor_test.go:91` — above `func TestDecideAfterRetroFloor_Cycle1001JudgmentHalt(t *testing.T) {`

```text
// F2 / R6-b — the cycle-1001 shape caught via JUDGMENT. The audit's structured
// class is task-level (code-audit-fail) so the deterministic dossier candidate
// is empty, but the orchestrator classified it infra-systemic in
// failure-decision.json → the floor halts on the orchestrator's own category.
```

### `go/internal/core/decision_branch_floor_test.go:140` — above `func TestDecideAfterRetroFloor_Cycle1603DiagnosedDowngradeIsNotForgery(t *testing.T) {`

```text
// Cycle-1603 REGRESSION (2026-09-02). Round-2 of an ADR-0092 audit repair
// inherited round-1's agent-amended acs-verdict.json (ship_eligible=false), so
// the EGPS override downgraded the repaired PASS to FAIL with a persisted,
// substantive fail reason. The DETERMINISTIC incoherence detector correctly
// declined to fire (CheckVerdictCoherence's SubstantiveError guard: a diagnosed
// downgrade is a justified negative verdict, not a forgery) — but the
// orchestrator's PROSE failure-decision.json classified verdict-incoherence
// anyway, and path (2) of applyFailureDecisionFloor halted the batch on prose
// that the deterministic evidence had already contradicted.
//
// Contract: a prose verdict-incoherence claim is CONTRADICTED when the recorded
// FAIL carries persisted substantive fail reasons. The cycle stays a task-level
// FAIL (normal retro routing); no SystemFailureSignal is produced.
```

### `go/internal/core/decision_branch_rejections_test.go:14` — above `func TestPlanCycle_RecordsValidationRejections(t *testing.T) {`

```text
// WS2-S2 (ADR-0052): planCycle records the WS2-S1 ValidatePlan findings to
// advisor-rejections.json for forensics — STANDALONE telemetry, decoupled from
// the WS3-S3 decision span, and never altering the disposed plan (the floor stays
// the sole disposer).
```

### `go/internal/core/decision_branch_repair_test.go:3` — above `import (`

```text
// decision_branch_repair_test.go — RED contract for the audit-repair branch.
//
// This is the file that proves ADR-0072 survives the change. The repair loop
// narrows exactly ONE authority — an agent-authored floor category that is
// contradicted by BOTH the deterministic dossier candidate AND the agent's own
// disposition — and nothing else. Every other halt path must behave byte-identically,
// which is why the contrast case below writes the same failure-decision.json as
// TestDecideAfterRetroFloor_Cycle1001JudgmentHalt and differs ONLY by the presence
// of disposition.json.
//
// Live evidence this encodes (wave 3, 2026-08-27): cycles 1572/1573/1574 each
// halted under gate 2 on an agent "infra-systemic" category, while their
// failure-dossier.json recorded floor_candidate:"" and the same agent's
// disposition.json recorded legitimacy:"legit-rejection". 367 minutes of lane
// time produced three FAILs that two deterministic signals called task-level.
```

### `go/internal/core/decision_branch_repair_test.go:62` — above `const agentFloorClaim = '{"category":"infra-systemic","level":"system","evidence":"prose-declared SYSTEM-class","action"…`

```text
// agentFloorClaim is the exact failure-decision.json body from the cycle-1001
// judgment-halt test. Shared by both cases below so the ONLY variable between
// "halts" and "repairs" is the disposition.
```

### `go/internal/core/decision_branch_repair_test.go:67` — above `func TestDecideAfterRetro_RepairsWhenAgentFloorIsContradicted(t *testing.T) {`

```text
// The wave-3 shape. Deterministic gate silent + agent claims a floor + the same
// agent's disposition says legit-rejection ⇒ the contradiction is recorded and
// the cycle is REPAIRED rather than halted.
```

### `go/internal/core/decision_branch_repair_test.go:80` — above `_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)`

```text
// The CORROBORATION half of ADR-0092 survives the retry redesign: prose alone
// does not outrank two corroborating deterministic signals. Its RETRY half is
// gone — retries are decided at the audit chokepoint from the audit's own
// declared class — so this asserts the halt disposition only.
```

### `go/internal/core/decision_branch_repair_test.go:133` — above `func TestDecideAfterRetro_RefusesRepairOnFabricatedFailureIdentity(t *testing.T) {`

```text
// ---- C1: the disposition is agent-authored PROSE and must not be trusted on its
// own word. crossCheckAgainstDigest exists precisely to stop an agent inventing a
// failure identity, and the repair rule was reading legitimacy straight past it.
// A fabricated, stale, or copied disposition could therefore convert a genuine
// ADR-0072 system-failure HALT into a granted repair.
```

### `go/internal/core/dispatch_signals.go:11` — above `var kindReportPhases = []Phase{PhaseScout, PhaseTriage}`

```text
// dispatch_signals.go — ADR-0099 slice 3: core is the ONE digester feeding the
// dispatch-time selectors (the skill-overlay `when` rule). The runner copies
// PhaseRequest.Signals onto policy.OverlayDispatch and never re-reads the
// workspace, so overlay selection and the kernel's own classification
// (DocumentCycle) read the same digest and degrade the same way.
```

### `go/internal/core/dispatch_signals_test.go:30` — above `func TestDispatchSignals(t *testing.T) {`

```text
// TestDispatchSignals — ADR-0099 slice 3: core is the ONE digester feeding the
// dispatch-time selectors (the skill-overlay `when` rule). The kind is the
// DECLARED one when a report spoke (triage > scout), else the project default
// (.evolve/domain.json), else code — so a document-domain project's very
// first scout dispatch (no report exists yet) already carries the solution
// persona, and a code project stays byte-identical.
```

### `go/internal/core/disposition_gate.go:3` — above `import (`

```text
// disposition_gate.go — S2 disposition-contract-gate (cycle-1034, item
// failure-disposition-router). The retro phase gains a MANDATORY disposition.json
// deliverable. VerifyDisposition is the fail-HARD counterpart to
// readFailureDecision's fail-SOFT boundary: a required deliverable, so an
// absent/malformed/invalid disposition is a LOUD error (retro cannot complete),
// never a silent (nil,nil) fallback. It also cross-checks the disposition's
// fingerprint+recurrence against the S1 failure-digest.json so the agent cannot
// INVENT a failure identity in retro.
//
// disposition.json schema: {cycle, fingerprint, recurrence, legitimacy,
// root_cause:{layer,summary}, salvage:{worktree_has_value,pointer}, urgency,
// justification, routing, proposed_item}.
```

### `go/internal/core/disposition_gate.go:59` — above `if err := crossCheckAgainstDigest(workspace, d); err != nil {`

```text
// IDENTITY FIRST (adversarial review, CRITICAL). A disposition is
// agent-authored prose ABOUT a failure; the only part of it a machine
// computed is the fingerprint/recurrence pair, and crossCheckAgainstDigest
// exists precisely to stop an agent inventing a failure identity. Reading
// legitimacy past that check let a fabricated, stale, or copied disposition
// convert a genuine ADR-0072 system-failure HALT into a granted repair.
//
// Unverifiable is NOT verified-good: a missing or malformed digest returns
// "" (no repair), because the safe direction here is to decline.
```

### `go/internal/core/disposition_gate.go:93` — above `const dispositionSchemaExample = '{`

```text
// dispositionSchemaExample is the single-sourced LEGAL example of the
// disposition contract. The retro persona (agents/evolve-retrospective.md,
// "Required deliverable: disposition.json") carries this exact document as its
// literal example; disposition_gate_singlesource_test.go parses both as JSON,
// asserts equality, and feeds this const through VerifyDisposition against a
// matching digest — so a drifted example fails CI instead of failing the
// agent (ADR-0084 invariant 2; the pre-2026-08-10 prose "example" was
// placeholder pseudo-JSON that would itself have failed this fail-HARD gate).
```

### `go/internal/core/disposition_gate.go:146` — above `if d.Salvage.WorktreeHasValue && d.Salvage.Pointer == "" {`

```text
// Salvage floor: preserved worktree value must be pointed at, never silently
// dropped (cycles 984/1000 salvage precedent).
```

### `go/internal/core/disposition_gate_singlesource_test.go:3` — above `import (`

```text
// disposition_gate_singlesource_test.go — three-legged single-source pin for
// the disposition contract (ADR-0084 invariant 2, #422 pattern): (1) the
// retro persona's literal example is byte-for-byte the same JSON document as
// the Go-side dispositionSchemaExample; (2) that document passes the real
// VerifyDisposition against a matching digest; (3) drift in either projection
// fails CI here instead of failing a live retro against a fail-HARD gate.
// The pre-2026-08-10 prose "example" was placeholder pseudo-JSON ("<int>",
// "P0 | P1 | P2 | P3") — an agent copying it failed its own gate.
```

### `go/internal/core/disposition_gate_test.go:3` — above `import (`

```text
// disposition_gate_test.go — RED contract for the S2 disposition-contract-gate
// (cycle-1034, item failure-disposition-router slice S2).
//
// The retro phase gains a MANDATORY disposition.json deliverable. VerifyDisposition
// is the fail-HARD counterpart to readFailureDecision's fail-SOFT boundary: a
// required deliverable, so absence/invalidity is a LOUD error (retro cannot
// complete), not a silent (nil,nil) fallback. It also cross-checks the
// disposition's fingerprint+recurrence against the S1 failure-digest.json so the
// agent cannot INVENT a failure identity in retro.
//
// disposition.json schema: {cycle, fingerprint, recurrence, legitimacy,
// root_cause:{layer,summary}, salvage:{worktree_has_value,pointer}, urgency,
// justification, routing, proposed_item}.
//
// RED today: VerifyDisposition and (*Orchestrator).finalizeRetroCompletion do
// not exist → this file fails to COMPILE (correct RED for new surface).
```

### `go/internal/core/disposition_gate_test.go:177` — above `func TestDispositionGate_SalvagePointerRequiredWhenValue(t *testing.T) {`

```text
// AC4 (edge) — salvage floor: worktree_has_value=true REQUIRES a non-empty
// pointer (cycles 984/1000 salvage precedent — preserved worktree value must be
// pointed at, never silently dropped). worktree_has_value=false with an empty
// pointer is accepted (nothing to salvage).
```

### `go/internal/core/disposition_gate_test.go:275` — above `dir := t.TempDir()`

```text
// Regression pin for the I2 wiring (assembler was landed callerless by
// cycle-1034): a pre-seeded foreign digest is REPLACED by the assembler
// pre-retro, so a disposition copying the foreign identity fails the
// cross-check — the digest on disk after completion is the assembler's.
```

### `go/internal/core/disposition_seed.go:3` — above `import (`

```text
// disposition_seed.go — the disposition-skeleton preseed (2026-08-10
// investigation; inbox disposition-skeleton-preseed). At continuation
// adoption the orchestrator already knows the ancestor's OPEN defect ids, so
// it writes <workspace>/defect-dispositions.json as a skeleton — one
// status-OPEN entry per inherited OPEN id — and the auditor only UPGRADES
// entries to FIXED (resolving evidence) or DEFERRED (reason).
//
// Gate semantics are untouched and unweakened: the disposition preflight sees
// the file as present and covering (never MISSING/INCOMPLETE), while the
// per-id reconcile rejects status OPEN ("not FIXED or DEFERRED") — a seeded
// entry the auditor never touches still blocks the cycle by name. A seeded
// DEFERRED would have been laundering; OPEN is the only honest seed.
//
// Known interaction (accepted, documented): the seeded file also satisfies
// the audit phase's Phase-B secondary-artifact hold immediately, so session
// teardown no longer waits for the UPGRADE — the per-id gate plus the
// ADR-0086 bookkeeping regrade cover an auditor that finishes without
// upgrading.
//
// The ancestor ledger is decoded through its owner, internal/core/defectledger
// (ADR-0103 unit 09): the seeder reads the same Doc and the same OPEN
// vocabulary the audit gate writes and grades, so the two sides cannot drift.
// phases/audit/disposition_seed_singlesource_test.go still feeds one real
// ledger document through both and asserts the same OPEN id set.
```

### `go/internal/core/disposition_seed_apicover_named_test.go:3` — above `import (`

```text
// disposition_seed_apicover_named_test.go — apicover named binding for the
// exported SeedDispositionSkeleton (issue #433 class: a new exported surface
// needs a NAMED covering test in its owning package; the phases/audit
// singlesource pin exercises it cross-package, which apicover does not
// count). Behavior is pinned by disposition_seed_test.go and the audit-side
// gate-semantics pin; this test binds the exported name.
```

### `go/internal/core/disposition_seed_test.go:3` — above `import (`

```text
// disposition_seed_test.go — RED contract for the disposition-skeleton preseed
// (inbox disposition-skeleton-preseed 0.9; 2026-08-10 investigation). The
// orchestrator KNOWS the inherited OPEN ids at adoption; making the auditor
// hand-enumerate them from the ancestor ledger was an avoidable failure
// surface (15/30 FAILs cycles 1390-1429 on disposition-preflight). The seam
// writes a skeleton — one status-OPEN entry per inherited OPEN id — that the
// gate treats as present-but-undispositioned (per-id block, never MISSING,
// never laundered: OPEN is not FIXED/DEFERRED, so nothing passes untouched).
```

### `go/internal/core/disposition_seed_unit09_test.go:14` — above `func TestSeedDispositionSkeleton_DecodesThroughTheLeaf_AndReadsAreSignalFree(t *testing.T) {`

```text
// Test 48 (ADR-0103 unit 09) — the adoption seeder decodes the ancestor ledger
// through the leaf: a leaf-written ledger with OPEN and FIXED rows seeds only
// the OPEN ids, byte-identical to the G6 skeleton captured on 8e8f080f; a
// directory at the ancestor ledger path seeds nothing; and the leaf's Read is
// Center-free — a carryover lifecycle reading the same broken file reports
// CARRYOVER_WORKSPACE_READ_FAILED and never an AUDIT_* code.
```

### `go/internal/core/dossier_producer.go:3` — above `import (`

```text
// dossier_producer.go — ADR-0055 cycle-dossier producer wiring.
//
// Before the 2026-06-22 doc↔impl audit the dossier subsystem (internal/dossier:
// Build/Write/Render) had ZERO production callers: finalizeCycle never emitted a
// dossier, knowledge-base/cycles/ stayed empty, and the policy `floor` gate
// "dossier-closeout" enforced an artifact nobody wrote (Potemkin enforcement).
// This file is the missing producer — RunCycle calls writeCycleDossier after
// finalizeCycle so every completed cycle leaves a committed, validated record.
//
// The write is BEST-EFFORT (RunCycle logs a WARN on error, never fails the
// cycle): a cycle has already finalized by the time we write its closeout
// artifact, so a dossier write failure must not destabilize the loop. Presence
// is enforced separately by `evolve dossier verify` against the policy floor.
```

### `go/internal/core/dossier_producer.go:51` — above `func defaultGitMutationLock(projectRoot string) (func(), error) {`

```text
// defaultGitMutationLock is the production locker: a blocking cross-process flock
// on the SHARED integrator lock (flock.ShipLockPath → <projectRoot>/.evolve/ship.lock,
// the SAME file internal/phases/ship acquireShipLock takes), so a lane's dossier
// commit and a sibling lane's ship commit are MUTUALLY EXCLUSIVE on the one shared
// .git/index. The kernel releases it on process death, so a crashed lane cannot
// wedge the fleet. Safe against the cycle-819 self-deadlock: the dossier commit
// runs in finalizeCycle AFTER ship has released its own lease, so this is a fresh
// acquire of a lock the lane does not already hold.
```

### `go/internal/core/dossier_producer.go:96` — above `PhaseTimings: p.PhaseTimings,`

```text
// The LIVE per-phase evidence. phase-timing.json is written by a
// DEFERRED call in RunCycle and lands AFTER this producer runs, so a
// dossier that read only the file recorded no phases on the normal
// path (cycle-1623). Passing what we already hold removes the ordering
// dependency entirely.
```

### `go/internal/core/dossier_producer_params_test.go:10` — above `func goldenDossierParams(projectRoot string) cycleDossierParams {`

```text
// dossier_producer_params_test.go — cycle 1652 RED contract for the inbox item
// dossier-producer-params-struct: writeCycleDossier grew to ELEVEN positional
// parameters by accretion (the 11th, live phase timings, touched every call
// site to append one value), so the producer takes ONE named params value
// mirroring dossier.BuildOpts and a field addition touches only the producer
// and the site that supplies it.
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 writeCycleDossier(lock gitMutationLocker, p cycleDossierParams) error —
//	    two parameters; every current input preserved as a NAMED field:
//	      ProjectRoot, WorkspacePath, Cycle, Goal, RunID, Outcome,
//	      SkippedPhases, VerdictsNotAdopted, SpineFailOpens, PhaseTimings
//	    (BuildOpts' spellings where BuildOpts has the field; Outcome keeps the
//	    producer's current name because it is the RAW cycle outcome that
//	    dossierVerdict maps — BuildOpts.FinalVerdict is the mapped value).
//	AC2 keyed construction: a caller that names only the fields it has compiles
//	    and runs — the property that makes a field addition non-breaking.
//	AC3 no behavior change: the bytes for a fixed input are unchanged. The
//	    golden under testdata/dossierparams/ was captured from the 11-argument
//	    producer at HEAD 287aa81c BEFORE this refactor — the JSON and Markdown
//	    the refactored producer emits for the same input must match byte for
//	    byte. The projection is clock-free (no time.Now in internal/dossier), so
//	    the pin is exact, not fuzzy.
//
// RED today as a compile failure: cycleDossierParams does not exist and
// writeCycleDossier has eleven parameters. Restore compilation FIRST (the
// struct + signature), then every other core test runs again.
```

### `go/internal/core/dossier_producer_simulate_test.go:3` — above `import (`

```text
// dossier_producer_simulate_test.go — the closeout dossier's git commit is a
// decision the ROOT makes, not the producer: a --simulate walk (no-LLM
// plumbing check) writes its dossier files but never commits into the
// operator's repo. Before this, `evolve campaign run --simulate` from a
// checkout left `dossier: cycle-N closeout` commits on the operator's branch
// (acs/cycle8 on every whole-module floor — the 2026-09-14 verification wave;
// docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).
```

### `go/internal/core/dossier_producer_systemfailure_test.go:3` — above `import (`

```text
// dossier_producer_systemfailure_test.go — cycle 1663 RED contract for the
// inbox item lost-ship-dossier-evidence (re-filed 2026-08-23 after the
// cycle-1546 salvage excised the draft).
//
// finalizeCycle stamps the landing-lost SystemFailureSignal onto CycleResult
// (lost_landing_floor.go, PR #482) and correctly downgrades the verdict to
// WARN — but writeCycleDossier receives only the terminal outcome string, so
// the committed record (knowledge-base/cycles/cycle-N.{json,md}) carries a
// WARN indistinguishable from any other WARN. An operator reading the dossier
// cannot see WHY the cycle was downgraded; the only evidence is in gitignored
// runtime (ship-error.json vs ship-binding.json, diffed by hand per cycle).
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 a landing-lost cycle's committed dossier carries the signal's
//	    structured category AND evidence text, driven through the REAL
//	    writeCycleDossier path — i.e. its two production callers,
//	    cycleRun.completeCycle (cycle_closeout.go) and
//	    cycleRun.abnormalEpilogue (cyclerun_epilogue.go). A seam that only a
//	    direct writeCycleDossier(…) call can reach is dead code.
//	AC2 the landed sibling of the SAME race (same transient ship error, but a
//	    ship-binding) carries NO landing-lost evidence; an ordinary PASS
//	    dossier stays byte-clean (golden captured from the pre-change
//	    producer, the cycle-1652 pin shape).
//	AC3 acs/cycle1544 predicates 004-005 restored — bound to the tests here.
//
// Wire shape pinned here (the one decision the tests must make so they can
// assert on something): the record lands under the top-level JSON key
// `system_failure`, an object carrying at least `category` and `evidence`.
// Generic — mirrors CycleResult.SystemFailure and the signal's own JSON tags
// — so the second producer of SystemFailureSignal (detectVerdictIncoherence)
// is not precluded later; a key named for one category would be. Extra keys
// (level, halt) are allowed, never required. The Go field/type names in
// internal/dossier and the BuildOpts spelling are the Builder's call — the
// FailureRecord/d.Failure pattern (failure.go, dossier.go:54-59) is the
// precedent to mirror; TestSchema_NoDrift enforces the schema half.
//
// Evidence is carried VERBATIM (asserted equal to the signal the floor
// produced, not merely "contains the code"): the floor formats one bounded
// line whose tail is the operator instruction ("rebase + re-verify …"), so a
// FailureRecord-style byte cap would cut exactly the actionable part.
//
// Fixtures are the REAL artifacts of the wave-20260822a-verify race
// (testdata/lostlanding): cycle-1535 lost its landing, cycle-1536 hit the
// same GIT_FLEET_REBASE_NEEDED and landed. The two are distinguishable ONLY
// by ship-binding.json, which is exactly what makes 1536 the right negative.
```

### `go/internal/core/dossier_producer_systemfailure_test.go:102` — above `func TestDossierSystemFailure_LostLandingReachesTheCommittedDossier(t *testing.T) {`

```text
// TestDossierSystemFailure_LostLandingReachesTheCommittedDossier — AC1, the
// headline wiring test. cycle-1535's real artifacts through the real
// completeCycle: the floor fires (that half already ships — asserted as a
// precondition so a RED here can only mean the dossier sink), and the
// committed pair must carry the signal's category and its evidence verbatim,
// while the verdict routing dossierVerdict already does (WARN) is unchanged.
```

### `go/internal/core/dossier_producer_systemfailure_test.go:157` — above `func TestDossierSystemFailure_LandedSiblingCarriesNone(t *testing.T) {`

```text
// TestDossierSystemFailure_LandedSiblingCarriesNone — AC2, the negative that
// stops the feature from turning every contended wave into a wall of false
// evidence. cycle-1536 recorded the SAME transient ship error and LANDED
// (ship-binding commit adcbddb2): through the same real path its dossier
// carries no system-failure object, no landing-lost text, no leaked error
// code — and DOES carry the binding's commit, which proves the producer read
// this very workspace rather than an empty one.
```

### `go/internal/core/dossier_producer_systemfailure_test.go:189` — above `func goldenPassDossierParams(projectRoot string) cycleDossierParams {`

```text
// goldenPassDossierParams is the FIXED ordinary-PASS input the byte-clean
// golden was captured from through the PRE-change producer (HEAD b9c0df76,
// before any system-failure field existed). Never shipped anything the
// workspace can prove (nonexistent workspace ⇒ no binding, no ship-error): the
// plain "ordinary PASS" every healthy cycle writes. Do not change a value here
// without regenerating the golden through a producer with NO signal support.
```

### `go/internal/core/dossier_producer_test.go:54` — above `func TestWriteCycleDossier_WritesValidArtifact(t *testing.T) {`

```text
// TestWriteCycleDossier_WritesValidArtifact is the core of the ADR-0055 fix: a
// completed cycle writes knowledge-base/cycles/cycle-N.json and it is valid.
```

### `go/internal/core/dossier_verdict_not_adopted_test.go:3` — above `import (`

```text
// dossier_verdict_not_adopted_test.go — dossier-retro-skipped-mislabel. Every
// FAIL dossier from cycles 1028/1035 and 1105-1117 (and on through 1198) carried
//
//	"skipped_phases": [{"phase": "retro", "reason": "FAIL"}]
//
// while the run dir held a 15-24KB retrospective-report.md: retro RAN. The record
// contradicted its own artifacts, which is worse than no record — the disposition
// assembler and recurrence analysis read dossiers to learn which judgment phases
// executed.
//
// Mechanism (final_verdict_floor.go): recordFinalVerdict appends to the list when
// a non-floor phase that RAN returns non-PASS after the floor verdict is set, so
// the field always meant "verdict not adopted" and Reason carried the VERDICT, not
// a skip cause. The fix is naming/semantics — those records get their own field
// (phases_run_verdict_not_adopted) and skipped_phases is left to phases that
// genuinely did not run (the abnormal-epilogue closeout entry). NOT "make retro
// run on FAIL": it already does, and that edit would break the cycle-802 clobber
// guard this code exists to enforce.
```

### `go/internal/core/earlyexit_test.go:3` — above `import (`

```text
// earlyexit_test.go — PA-DDK DDK-7 (ADR-0060): the early-exit set is
// config-driven (per-phase early_exit), with the shipPlanned guard staying Go.
// Phases resolved via the kerneltest fixture — no hardcoded names.
```

### `go/internal/core/empty_commitment_claimable_test.go:10` — above `const claimableInboxItem = '{`

```text
// empty_commitment_claimable_test.go — cycle 1652 RED contract for the inbox
// item triage-empty-commitment-still-dispatches-spine (the split-out fourth
// acceptance line of the cycle-1623 P0).
//
// Cycle 1623: triage could not claim its selected item, wrote
// triage-decision.json with top_n [] and one deferral, and the orchestrator ran
// twelve more phases against no committed task. The round-2 gate
// (decideTriageTermination + the PhaseTriage branch of selectNext) now stops
// EVERY explicit empty commitment before tdd — but it stops it as
// CycleTerminationTriageNoWork, the LEGITIMATE planned-no-work disposition, with
// no regard for whether the inbox still held claimable work. A claim race
// between fleet lanes, a loader that drops an item, or an agent that mis-writes
// the decision therefore all get credited as "nothing to do" and the loop
// reads a defect as a clean SKIPPED. The distinguishing input is "was there
// work to claim" (the inbox record's own words); this file pins it where the
// decision is CONSUMED — the composed RunCycle / RunCycleFromPhase paths — so
// a router-layer-only fix cannot stay GREEN (the cycle-1623 H1 lesson).
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 claimable inbox + top_n [] ⇒ no tdd/build/audit/ship dispatch, a NAMED
//	    terminal reason that is NOT the legitimate no-work reason, never PASS,
//	    and never credited as IsTriageNoWorkResult.
//	AC2 the legitimate empty-inbox case keeps recordPlannedNoWorkOutcome's
//	    SKIPPED disposition (regression pin).
//	anti-no-op: committed work beside a still-populated inbox must advance.
//
// The exact spelling of the named reason is the Builder's (the inbox names the
// two candidates: claim-failed / no-commitment); what is frozen is that it is
// non-empty, distinct from CycleTerminationTriageNoWork, and that the cycle is
// not classified as planned no-work.
```

### `go/internal/core/empty_commitment_claimable_test.go:125` — above `func TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation(t *testing.T) {`

```text
// TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation — AC1 on the
// fresh dispatch root. Two rows: the raw claim-race shape (inbox item present,
// decision silent about it) and the narrated shape (the decision defers the
// item it failed to claim — cycle 1623's literal artifact). Both must stop
// before tdd with a named, non-no-work reason. Triage may be re-dispatched at
// most ONCE (the inbox's "re-dispatch triage once with the claim error"
// option); a loop that keeps re-asking is bounded here.
```

### `go/internal/core/empty_commitment_claimable_test.go:145` — above `name:     "claim-failure-narrated-as-deferral",`

```text
// cycle-1623 verbatim: the unclaimable item narrated as a deferral.
```

### `go/internal/core/empty_commitment_claimable_test.go:151` — above `name:     "failed-triage-with-claimable-inbox",`

```text
// A FAIL verdict over the same evidence must not regress to the
// cycle-1639 pre-fix behavior (plain FAIL, no disposition) NOR be
// granted planned no-work: the claimable item is still the fact.
```

### `go/internal/core/empty_commitment_claimable_test.go:202` — above `func TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation(t *testing.T) {`

```text
// TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation
// is the resumed-root twin (the cycle-1639 lesson: a paused-and-resumed run
// must not diverge from a fresh one on identical on-disk evidence). Resume
// routes triage termination through the same o.triageTermination call, but
// closeout's recordPlannedNoWorkOutcome reclassifies independently — both
// seams must see the claimable item.
```

### `go/internal/core/empty_commitment_claimable_test.go:256` — above `"only-console-routed-item",`

```text
// A console-routed item is operator-owned: a lane can never claim
// it, so it is NOT claimable work and must not turn a legitimate
// empty commitment into a claim failure (ADR-0074 I1).
```

### `go/internal/core/empty_commitment_dispatch_test.go:33` — above `func writeTriageWorkspace(t *testing.T, committed int) string {`

```text
// Cycle 1623, audit round 2, finding H1 (CRITICAL).
//
// Round 2 gated the empty triage commitment at router.Route. Route's decision
// is only a PROPOSAL: cyclerun_select.go:103 hands it to enforceNext, whose
// PhaseEnd branch (routing_dispatch.go:59-63) asks
// StateMachine.CanTerminateEarly(current, shipPlanned) — which returns false
// unconditionally when shipPlanned is true (statemachine.go:230-233). Cycle
// 1623's own clamped plan schedules ship, so the proposal was discarded and the
// orchestrator dispatched tdd, build and audit against a task no phase was
// authorized to own. The router-layer test stayed GREEN through all of it.
//
// This is the table pinning the decision where it is CONSUMED. It is the
// in-package twin of the cycle predicate
// TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath, which
// drives the same contract through a whole RunCycle: this one isolates the
// authority, that one proves it is reached.
//
// The signals are always derived by running the real router.Digest over a real
// on-disk workspace. That is deliberate — TriageSignals.commitmentKnown is
// unexported in package router, so a hand-built literal here could not express
// the distinction between "committed nothing" and "no decision artifact", which
// is the whole point of the gate. It also means this test fails if the
// committed count ever stops being plumbed from triage-decision.json.
```

### `go/internal/core/empty_commitment_dispatch_test.go:169` — above `name:        "failed-triage-with-empty-decision",`

```text
// Cycle 1623 audit round 1 (H2, M2, M1) / cycle 1639 pinned defect:
// decideTriageTermination gave VerdictFAIL precedence over an
// explicit empty commitment, so this row previously stayed a plain
// FAIL (breaker-counting failure, no worktree cleanup) even though
// triage committed no work at all. An explicit empty top_n is
// authoritative no-eligible-work evidence regardless of the phase
// verdict that carried it.
```

### `go/internal/core/empty_commitment_dispatch_test.go:300` — above `{"empty-commitment-terminates-even-when-ship-planned", PhaseTriage, PhaseTDD, VerdictPASS, 0, true, PhaseEnd, true},`

```text
// The defect. Cycle 1623's live configuration: triage committed nothing
// and the clamped plan scheduled ship.
```

### `go/internal/core/errors.go:26` — above `ErrArtifactTimeout = errors.New("core: bridge artifact timeout")`

```text
// ErrArtifactTimeout is wrapped into the Bridge.Launch error when a
// driver returns ExitArtifactTimeout (81) — the agent's contracted
// artifact never appeared within the wait window. It lives on the
// Bridge port (not the concrete bridge adapter) so the generic phase
// runner can errors.Is-match it WITHOUT importing a specific driver:
// an OPTIONAL phase that hits this degrades to WARN+advance instead of
// aborting the whole cycle (Workstream D — cycle-120 build-planner).
```

### `go/internal/core/errors.go:35` — above `ErrTransientBridgeFailure = errors.New("core: transient bridge failure")`

```text
// ErrTransientBridgeFailure is wrapped into the Bridge.Launch error when a
// driver returns exit 80, 85, 86, or 124 (boot timeout / unknown prompt /
// respond-loop guard / command-level timeout kill) — transient infra issues —
// OR when the driver subprocess exits -1 (signal death) while our own context
// is cancelled (a completion-wait / phase-timeout teardown SIGKILL'd it;
// cycle-859). 127 (missing binary) is deliberately NOT transient: an absent
// CLI is an environment defect that must fail loud, recovered only by the
// exit-code-triggered family fallback.
```

### `go/internal/core/errors.go:45` — above `ErrAgentDocMissing = errors.New("core: agent persona doc missing")`

```text
// ErrAgentDocMissing marks a phase-dispatch failure whose cause is the
// agent persona doc not existing on disk (soak-20260824a cycle-1551: an
// optional menu phase with no persona anywhere killed its whole lane
// rc=4). Wrapped by the runner at the load-agent step; consumed by
// optionalInfraSkip so a genuinely OPTIONAL phase degrades to a recorded
// skip while mandatory/floor phases still fail loud.
```

### `go/internal/core/errors.go:53` — above `ErrAllFamiliesExhausted = errors.New("core: all CLI families quota-exhausted (exit=85)")`

```text
// ErrAllFamiliesExhausted marks the quota-terminal exhaustion case
// (cycle-656): every retry attempt for a phase returned exit=85, meaning
// every CLI family in the fallback chain is quota-drained. The dispatch
// seam writes a quota-likely checkpoint before aborting with this, so the
// batch stops resumable (`evolve loop --resume`) instead of failing
// forward into the same wall.
```

### `go/internal/core/errors.go:69` — above `ErrUnsafeConfig = errors.New("core: unsafe transition config")`

```text
// ErrUnsafeConfig means the loaded transition config (legality graph, gates,
// verdict branches) violates a safety invariant — a flow that could ship
// without the integrity floor. The composition root computes the violations
// via ValidateSafetyInvariants at construction; RunCycle/RunCycleFromPhase
// fail closed with this before any phase runs (PA-DDK DDK-5, ADR-0060 §1a).
```

### `go/internal/core/errors.go:77` — above `func IsInfraTeardownError(err error) bool {`

```text
// IsInfraTeardownError reports whether err is a bridge INFRA teardown — an
// artifact-wait timeout (ErrArtifactTimeout, exit 81) OR a transient bridge
// failure (ErrTransientBridgeFailure, exit 80/85/86/124: quota exhaustion,
// liveness-exhaustion, command-timeout kill; and exit -1 under ctx-cancel: a
// context-cancellation SIGKILL of the driver — cycle-859). Both end the SESSION
// without implying the agent failed: it may have written its contracted
// deliverable before the teardown.
// The phase runner uses this as the single-source trigger for reconciling
// against the on-disk deliverable instead of synthesizing FAIL (cycle-254/255
// timeout false-FAIL; cycle-835 quota false-FAIL). Substantive errors
// (launch/boot/safety/cost) are NEITHER sentinel and are intentionally excluded —
// their output is untrustworthy, so those hard-fail without consulting disk.
```

### `go/internal/core/errors.go:93` — above `func isArtifactTimeout(err error) bool { return errors.Is(err, ErrArtifactTimeout) }`

```text
// isArtifactTimeout is the timeout-ONLY gate unit 02 (ADR-0103) injects into
// the failure-diag writer: it decides exit_code 81 and whether a delivery
// cause may be attributed. Never widen it to the IsInfraTeardownError union
// (TestTimeoutOnlySites_NotWidenedToUnion pins its body and both injection
// sites).
```

### `go/internal/core/errors.go:100` — above `func IsOptionalSkippableError(err error) bool {`

```text
// IsOptionalSkippableError is the FULL admission predicate for
// optionalInfraSkip's error gate: infra teardown (IsInfraTeardownError — the
// original Workstream-D class) OR a missing agent persona doc
// (ErrAgentDocMissing, cycle-1551 — a config defect whose blast radius must
// not exceed the phase carrying it). Single-sourced beside the sentinels so
// the dispatch site and its invariant tests share one spelling; widen ONLY
// alongside a paired incident + regression pin, never ad hoc at a call site.
```

### `go/internal/core/evalgate_escalation_test.go:3` — above `import (`

```text
// evalgate_escalation_test.go — a remediation-carrying rejection escalates the
// re-dispatch CLI at the second identical block, exactly as a contract block
// does.
//
// The measured evidence that revises scoping constraint 3 (contract_escalation.go):
// every eval-materialization failure since cycle-1450 — 1471, 1476, 1504, 1531,
// 1540, 1545 — was scout on codex-tmux (claude-scout: 0 of 26), and every one
// burned its full correction budget on the SAME CLI without recovering,
// INCLUDING after #480 made the correction name the exact writable paths
// (cycle-1545's directive verified byte-perfect; the agent idled at its prompt
// without writing the files). For the CREATE-a-missing-artifact class, a
// different CLI is demonstrably the remedy: constraint 3's "task-binding
// rejections don't escalate" survives for topngate/triagecap/build-floor —
// which carry NO remediation — via the typed discriminator
// ReviewResult.Remediation, set only by evalgate.
```

### `go/internal/core/evalgate_escalation_test.go:68` — above `writeCLIProfile(t, root, "builder", "codex-tmux", []string{"claude-tmux"})`

```text
// Pin the phase's profile to a NON-claude family (the live incidents'
// shape: scout on codex-tmux) so escalation has a real target — with no
// profile the phase resolves to the universal claude family and there is
// nowhere to escalate (the constraint-5 salvage-retry arm fires instead,
// which is a different behavior with its own tests).
```

### `go/internal/core/evaluate_batch.go:98` — above `return cr.retryPhaseRunner(phase, req, cr.evaluateBatchRetryOpts())`

```text
// Delegate — never a second hand-maintained loop. The batch's divergence
// from the sequential path is declared in evaluateBatchRetryOpts (ship
// recovery and backfill disabled), not re-derived here (cycle-1166).
```

### `go/internal/core/evaluate_batch.go:104` — above `func (cr *cycleRun) dispatchEvaluateBatch(batch []Phase) (loopAction, error) {`

```text
// dispatchEvaluateBatch runs the parallelizable post-build checking phases
// CONCURRENTLY (ParallelEvaluate=enforce), then folds their outcomes in a single
// SERIALIZED merge. Concurrency is bounded by cfg.ParallelEvaluateConcurrency.
//
// Safety: only runner.Run (the minutes-long LLM slice) runs concurrently; every
// shared-state mutation — recordPhaseOutcome (the ADR-0044 C1 chokepoint),
// CompletedPhases, phase-completion ledger, cycle-state — happens in the single-goroutine merge,
// except admitted skip records appended through the synchronized ledger by workers. Verdict merge is weakest-link (FAIL>WARN>PASS). A hard
// dispatch error is all-or-nothing: every phase's outcome is still recorded
// (C1-complete) and the cycle aborts on the first error.
//
// v1 fidelity gap (documented; closed before the enforce flip): the
// deliverable-correction ladder and the tree-diff leak guard do NOT run for
// batched phases — acceptable because evaluate phases are read-only and the
// feature ships DORMANT (StageOff default), activated only after a shadow soak.
```

### `go/internal/core/evaluate_batch_retry_parity_test.go:3` — above `import (`

```text
// evaluate_batch_retry_parity_test.go — RED contract for the fable5 deep-scan
// finding evaluate-batch-retry-parity (inbox weight 0.82, cycle-618 scout).
//
// Context. The sequential dispatch loop (cyclerun_dispatch.go) applies TWO
// skip predicates before treating a phase's exhausted retries as a
// cycle-level failure: optionalInfraSkip (an Optional, non-mandatory,
// off-floor phase whose exhaustion is infra-shaped degrades to SKIPPED with warning + advance)
// and postShipObserverSkip (a best-effort post-ship Control observer's
// failure never turns an already-shipped cycle abnormal). dispatchRunnerWithRetry
// (evaluate_batch.go) — the SAME per-phase retry loop, reused for the
// parallel-evaluate batch — has NO calls to either predicate: it returns the
// raw error on exhaustion unconditionally. A batched Optional evaluate phase
// (or a post-ship Control observer that happened to land in a batch) that
// exhausts retries therefore aborts the WHOLE cycle in the batched path where
// the identical phase would have degraded to SKIPPED with warning + advance in the sequential
// path — the copy-adapted-control-flow class of defect already fixed once in
// this codebase (statefile-rmw-flock-single-source, cycle 617). This blocks
// the parallel-evaluate enforce flip (memory: phase_timing_evidence) because
// flipping today would silently narrow the fail-open surface for every
// batched phase.
//
// RED today: dispatchRunnerWithRetry ignores both skip predicates, so the
// assertions below (err==nil, verdict SKIPPED) fail against the current
// unconditional-error return — a real behavioral RED, not a compile error.
```

### `go/internal/core/explanation_advisory_record_test.go:10` — above `func TestRecordPhaseOutcome_AnAdvisoryOnAPassRidesTheRecord(t *testing.T) {`

```text
// ADR-0102: an explanation-review advisory raised on a PASS verdict reaches
// the C1 record — the warning trail #577 keeps on a PASS — which is the
// persistence the decision's feedback loop rests on. The other half (the
// audit classification emits the prefixed advisory on a PASS) is pinned in
// internal/phases/audit.
```

### `go/internal/core/explanation_section_ladder_test.go:9` — above `func TestAuditContractRejection_RedispatchesTheAuditorWithTheReason(t *testing.T) {`

```text
// TestAuditContractRejection_RedispatchesTheAuditorWithTheReason is the core
// half of "a missing ## Explanation Documentation section is a correction, not
// a terminal FAIL" (cycles 1601/1603): when the deliverable reviewer rejects
// the AUDIT report with the conditional-section reason, the ladder re-dispatches
// the audit runner once with that reason as its correction directive, and the
// cycle proceeds on the approved second report. The reviewer side is proven in
// deliverable (the real Reviewer at enforce); this proves the ladder accepts an
// audit rejection like any other phase's.
```

### `go/internal/core/extra_coverage_test.go:64` — above `func TestEnforceNext_EmptyOrderSkipAdvance(t *testing.T) {`

```text
// TestEnforceNext_EmptyOrderSkipAdvance pins the cycle-240 e2e regression: with
// an EMPTY cfg.Order (no phase-registry.json — the e2e fixtures and any bare
// repo), the skip-advance loop must NOT rewrite a skipped staticNext to
// PhaseEnd (nextInOrder returns PhaseEnd for any phase absent from an empty
// order, which silently terminated cycles after tdd). The original staticNext
// is kept — pre-skip-advance parity — and no advance is reported.
```

### `go/internal/core/extra_coverage_test.go:189` — above `func TestRecordPhasePlan_HappyAndClamps(t *testing.T) {`

```text
// --- recordPhasePlan: happy + clamp log + error tolerance (ADR-0024 §2) ------
```

### `go/internal/core/failreasons_backfill.go:3` — above `import (`

```text
// failreasons_backfill.go — "no FAIL without a reason" (inbox
// null-failreasons-capture; 2026-08-06, three instances in one night). A
// cycle failing at phase-infra level (launch refusal, timeout, abort) sealed
// FinalVerdict FAIL with FailReasons null: retros fingerprinted a
// content-free identity (the bc2e3236 gate-block class in task-FAIL form)
// and the identical-fingerprint breaker could not see genuine recurrence.
//
// The seal backfills from the phase-timing record — orchestrator memory
// written at the recordPhaseOutcome chokepoint, never an agent-writable
// workspace file, so a failure identity cannot be forged from the workspace.
```

### `go/internal/core/failreasons_backfill_test.go:3` — above `import (`

```text
// failreasons_backfill_test.go — RED contract for "no FAIL without a reason"
// (inbox null-failreasons-capture 0.85): cycles failing at phase-infra level
// (launch refusal, timeout, abort) sealed FinalVerdict FAIL with FailReasons
// null — retros fingerprinted nothing (content-free identity, the bc2e3236
// gate-block class in task-FAIL form) and the breaker could not see genuine
// recurrence. Three instances in one night. The seal now backfills from the
// trusted phase-timing record (orchestrator memory, never a workspace file).
```

### `go/internal/core/failreasons_backfill_test.go:82` — above `func TestBackfillFailReasons_PhaseOwnDiagnosticsNameTheReason(t *testing.T) {`

```text
// A phase that returns FAIL by its OWN Classify — triage's protected-surface
// admission rejection (cycles 1634 and 1636, 2026-09-12/13) — recorded no
// abort reason, so the seal labelled it "phase-infra class": a deterministic,
// reasoned rejection paged as infrastructure. The C1 record now carries the
// phase's own diagnostics; the seal names the error-severity ones and keeps the
// infra marker only for a FAIL that truly recorded nothing.
```

### `go/internal/core/failure_advisor.go:21` — above `type FailureAdvisor struct {`

```text
// FailureAdvisor is the ADR-0044 LLM escalation TAIL: it reads a fatal-looking
// pane the deterministic FatalPaneDetector could NOT classify (CauseUnknown)
// and returns a typed cause + the novel pane substring to promote + a
// human-readable justification. Built exactly like PhaseAdvisor — bridge-
// dispatched, persona-injected, strict-JSON-parsed — and fail-safe the same
// way: EVERY failure (nil bridge, launch error, malformed output, vocabulary
// violation) returns an error and the caller escalates to the operator
// instead of acting on garbage. Deterministic-first, LLM-last (Core Agent
// Rule 5): this advisor is never on the hot loop for a known failure — its
// verdicts get PROMOTED into the deterministic registry
// (recovery.PromoteAdvice), so each novel state is paid for once.
```

### `go/internal/core/failure_advisor.go:34` — above `identity AgentIdentity`

```text
// ADR-0052 WS1-S1: shared dispatch identity (same value object as PhaseAdvisor)
```

### `go/internal/core/failure_advisor.go:133` — above `func (a *FailureAdvisor) composePrompt(in FailureAdviseInput, artifact string) string {`

```text
// composePrompt renders persona (when injected) + the per-incident evidence,
// the same layering every phase prompt uses. The inline fallback keeps the
// advisor functional before the composition root wires the persona file.
```

### `go/internal/core/failure_advisor.go:148` — above `b.WriteString("# Recent pane tail\n")`

```text
// ADR-0045 I5: pane text is untrusted input. It reaches this (quarantined)
// LLM only as a neutralized, framed digest — secrets redacted, house
// markers defanged, fence-breakout softened — never raw (threats S1/S6).
```

### `go/internal/core/failure_advisor_framing_test.go:3` — above `import (`

```text
// ADR-0045 I5 full: the ADR-0044 FailureAdvisor's prompt is the one shipped
// LLM consumption of raw pane text — it must traverse panetrust.Frame
// (untrusted preamble, neutralized fenced digest, secrets redacted).
```

### `go/internal/core/failure_advisor_test.go:3` — above `import (`

```text
// failure_advisor_test.go — ADR-0044 Slice 5 RED tests: the LLM failure
// advisor (the AI escalation TAIL — reached only for CauseUnknown terminal
// states the deterministic registry cannot classify; Core Agent Rule 5).
// Modeled on phase_advisor_test.go: fakeBridge scripts the LLM, the advisor
// must parse a strict-JSON verdict, and EVERY failure mode (nil bridge,
// malformed JSON, invalid vocabulary) returns an error so the caller
// escalates instead of acting on garbage — fail-safe-to-deterministic.
```

### `go/internal/core/failure_decision.go:3` — above `import (`

```text
// failure_decision.go — ADR-0072 S4 Task 2 (failure-decision-schema-reader).
// The orchestrator (retrospective agent) may emit failure-decision.json with its
// classification of a cycle's failure. This reader is the FALLBACK BOUNDARY:
// a malformed / absent / schema-invalid artifact yields (nil, nil) — the signal
// to fall back to the deterministic failureadapter — NEVER an error that aborts
// the cycle (retro_always_on_failure). Only an in-vocabulary {action, level}
// decision is honored, so a garbled artifact can never drive an unrecognized
// branch.
//
// The symbol is deliberately UNEXPORTED (JSON-tagged fields only) — no new
// apicover-gated public surface.
```

### `go/internal/core/failure_decision_test.go:3` — above `import (`

```text
// failure_decision_test.go — cycle-1002 RED contract for ADR-0072 S4 Task 2
// (failure-decision-schema-reader). The reader is the fallback boundary: a
// malformed / absent / schema-invalid artifact MUST yield (nil, nil) — the
// signal to fall back to the deterministic failureadapter — NEVER an error that
// aborts the cycle (retro_always_on_failure). Fails RED until Builder adds
// readFailureDecision + the failureDecision type in failure_decision.go.
```

### `go/internal/core/failure_decision_wiring_test.go:3` — above `import (`

```text
// failure_decision_wiring_test.go — cycle-1002 RED contract for ADR-0072 S4
// Task 4 (orchestrator-emits-failure-decision), Go half. The inert-API guard:
// the EXACT failure-decision.json shape the retrospective agent's instructions
// document must round-trip cleanly through the Task-2 reader — proving the
// emitter and consumer share one schema, so the consumption path is not
// permanently fallback-only. The instruction-file wiring proof (the emit
// directive + write-allowlist widening) is asserted by the ACS predicate
// C1002_005 against agents/evolve-retrospective.md. Fails RED until Builder
// adds readFailureDecision.
```

### `go/internal/core/failure_delivery_evidence_test.go:3` — above `import (`

```text
// failure_delivery_evidence_test.go — RED contract for cycle-1562 task
// `retrospective-delivery-evidence-contract` (the core half; the bridge half
// lives in internal/bridge/driver_tmux_delivery_failure_test.go).
//
// Evidence (.evolve/runs/cycle-1510/retrospective-launch-error.txt): the retro
// died with exit 81 and its terminal <phase>-failure-diag.json recorded only a
// flat error_message. The reason the launch failed — the driver had verified,
// in milliseconds, that the prompt was never submitted — existed nowhere
// machine-readable. Any downstream reader (failure learning, the failure
// adviser, a human triaging a repeat) has to substring-parse a free-text
// error to tell an undelivered prompt from an agent that simply went quiet,
// and those two failures have opposite remedies: relaunch the pane vs. raise
// the phase's artifact budget.
//
// Contract: failurediag.Sidecar must carry the classified delivery-failure cause
// as its OWN field, populated from the driver's `reason=` marker text, and it
// must stay empty for every failure that is not an evidenced delivery failure.
// The negative half is the load-bearing one — a field that is always populated
// carries no information and would mislabel every slow phase.
```

### `go/internal/core/failure_diag.go:3` — above `import (`

```text
// failure_diag.go — unit 02 (ADR-0103, design decomposition/02-failure-diagnostics.md):
// the orchestrator's seam onto the failurediag unit. Every abort site keeps
// calling writePhaseFailureDiag (a method now, so the writer reaches the
// orchestrator's clock and Signal Center); retro and the bridge binding tests
// keep calling the exported DeliveryFailureCause.
```

### `go/internal/core/failure_diag_test.go:3` — above `import (`

```text
// failure_diag_test.go — unit 02 (ADR-0103): the orchestrator keeps the seam
// every abort site uses (writePhaseFailureDiag, now a method) and the exported
// DeliveryFailureCause facade; the unit-02 writer is built once, reads the
// clock and the Center live, and has ONE construction site. RED first.
```

### `go/internal/core/failure_digest.go:3` — above `import (`

```text
// failure_digest.go — S1 failure-digest-assembler (cycle-1034, item
// failure-disposition-router). The deterministic post-FAIL / pre-retro step that
// converts a failed cycle's forensic artifacts into a STABLE failure identity the
// S2 disposition gate cross-checks against, so the retro agent can no longer
// INVENT the failure's identity (closes lesson_to_action_gap).
//
// SEAM (Core Rule 3): the fingerprint/bucket source is the single workspace SSOT
// artifact audit-fail-reason.json ({schema_version, phase, reasons[]}, emitted by
// the coherence floor), mirroring readFailureDecision's workspace-file boundary.
// Reading it is fail-SOFT: an absent/malformed artifact degrades to the "unknown"
// bucket and STILL writes a digest — a genuinely novel failure must always yield a
// triage artifact. Only a real write IO failure is returned as an error.
```

### `go/internal/core/failure_digest.go:37` — above `Unexplained bool 'json:"unexplained,omitempty"'`

```text
// Unexplained marks a digest whose reason set carries NO distinguishing
// content (empty, or exactly the content-free agent-graded router line).
// Its fingerprint asserts no defect identity: distinct failures collapse
// into this bucket by construction, so the blocker breaker must route it
// to the diagnosability rule, never the identical-fingerprint rule
// (batch-14: cycles 1137/1139/1143 — three DISTINCT auditor findings on
// one task, one shared boilerplate fingerprint, false halt).
```

### `go/internal/core/failure_digest.go:170` — above `var cycleNumberToken = regexp.MustCompile('(?i)\bcycle[ -]\d+(?:-\d+)*')`

```text
// cycleNumberToken matches the cycle-numbered tokens that per-cycle ARTIFACT
// PATHS bake into reason text — ".evolve/runs/cycle-1365/audit-report.md",
// ".evolve/worktrees/cycle-42824668-1440/go", and the bare "cycle 1365" of
// prose reasons. The trailing `(?:-\d+)*` is load-bearing for the worktree
// shape, whose name carries BOTH a lane hash and a cycle number. Same shape as
// the carryover unit's cycleTokenRE (internal/core/carryover/identity.go), extended for multi-segment ids.
// Only the NUMBER folds: the path around it (which dir, which artifact FILE)
// is untouched, so two different artifacts in one cycle dir stay two defects.
```

### `go/internal/core/failure_digest.go:189` — above `func normalizeReasonForFingerprint(reason string) string {`

```text
// normalizeReasonForFingerprint projects a reason onto its DEFECT IDENTITY,
// dropping tokens that are load-bearing for a human reader but pure noise for
// identity. Display and identity are two projections of the one reason string:
// the digest, the dossier and audit-fail-reason.json all keep the reason
// verbatim — only the hash input is normalized.
//
// Today that is exactly one token. The audit verdict-conflict record names the
// auditor's own verdict (`narrative=PASS|WARN|SKIPPED`); three canonical values
// are reachable for ONE recurring defect — the same gate red on three retries —
// which would split it into three fingerprint buckets against
// IdenticalFingerprintCeiling=3 and stop the identical-fingerprint breaker from
// ever halting the batch (cycle-1127 audit C1). Bounding the value (IsVerdict)
// makes it finite but not STABLE; the cycle-1124 lesson requires both. Same
// principle as egpsRedIDCycleTokens stripping cycle numbers from ac_ids — with
// the normalization at the hash boundary instead of the message, because here
// the varying token is the very fact the operator needs to see.
//
// Deliberately narrow: the defect-identifying content of the same reason set
// (which gate, which predicate) is untouched, so two DIFFERENT defects never
// collapse into one fingerprint.
```

### `go/internal/core/failure_digest.go:216` — above `func (o *Orchestrator) ensureFailureDigest(cycle int, projectRoot, workspace, fallbackPhase, fallbackReason string) {`

```text
// ensureFailureDigest is the single-source wiring shared by BOTH retro
// dispatch paths (recordFailureLearning for phase errors; cyclerun dispatch
// for verdict FAILs — cycle-1046 proved wiring only the first blinds the
// disposition contract AND the blocker breaker for verdict-path failures).
// Ledger load is fail-soft (nil counter → recurrence 0); a digest write
// failure only WARNs — retro must never be blocked by forensics plumbing.
// Idempotent: identical artifacts yield an identical digest.
// fallbackPhase/fallbackReason are the caller's own evidence (failed phase +
// error/verdict text), written INTO audit-fail-reason.json when no floor wrote
// one — batch-6 cycles 1044/1045/1047 each failed differently with no reason
// artifact, collapsed to one empty-evidence fingerprint, and false-tripped the
// identical-fingerprint breaker rule. A floor-written artifact always wins;
// the fallback never overwrites (F8: one evidence trail for humans + digest).
```

### `go/internal/core/failure_digest.go:261` — above `if fb, ok := phasecontract.ReadFailureBlock(workspace, phase); ok {`

```text
// Defect identity FIRST — it is the only layer that separates the common
// case of the SAME task re-audited with a DIFFERENT finding each retry
// (batch-14: 1137/1139/1143 were three progressing findings on one task;
// the task-id layer is constant across them by definition). Task identity
// is the weakest signal and therefore the LAST resort, not the first.
//
// The sentinel read goes through phasecontract.ReadFailureBlock — the SAME
// authority the verdict path, dossier and failure-learning consult — not a
// hand-rolled parse (diff-review HIGH: a local reader dropped the
// placeholder-echo guard, so the Deliverable Contract's printed example
// "<one line per defect>" echoed into a report would mint a CONSTANT
// cross-task defect identity — the exact false-trip this file exists to
// prevent — and it also settles first-sentinel precedence and the
// per-phase report name in one call).
```

### `go/internal/core/failure_digest.go:307` — above `var freeTextCycleTokens = regexp.MustCompile('\b(?:[Cc]ycle[-_ ]?\d+|TestC\d+_)')`

```text
// freeTextCycleTokens matches cycle-numbered chrome INSIDE prose defect text
// ("acs/cycle1141", "cycle-1141", "TestC1141_004_…") — the in-text
// counterpart of the audit phase's anchored egpsRedIDCycleTokens. Both
// require the literal cycle/TestC prefix plus digits, so two DIFFERENT
// defects never collapse; only the retry-varying token folds.
```

### `go/internal/core/failure_digest.go:333` — above `func causeHead(errText string) string {`

```text
// causeHead projects an abort-cause error string onto its stable identity:
// cycle-normalized, single-line, and — unlike defectHead — TAIL-kept when over
// budget. Error chains grow prefix-first ("phase build: attempt N: bridge:
// <long path>: <root cause>"), so the distinguishing content lives at the END;
// a head-kept cut would collapse two different roots under one long shared
// prefix into one fingerprint (review M1 on the batch-19 cycle-1208 fix).
```

### `go/internal/core/failure_digest.go:407` — above `func agentGradedFailReason(phase, workspace string) string {`

```text
// agentGradedFailReason composes the dispatch loop's fallback fail-reason for
// an agent-graded FAIL: the router line plus the strongest per-failure
// distinguisher the workspace offers (cycles 1054/1060: a constant fallback
// collided two different tasks; batch-14: a task-first distinguisher collided
// three different defects of ONE task).
```

### `go/internal/core/failure_digest_ensure_test.go:3` — above `import (`

```text
// failure_digest_ensure_test.go — RED contract for the single-source digest
// helper both retro dispatch paths share. Cycle-1046 (batch-6 live-fire, first
// FAIL on the new binary) proved the S1 assembler was wired ONLY at
// recordFailureLearning (phase-error path); verdict-path FAILs reach retro via
// cyclerun dispatch and produced NO digest and NO disposition — blinding both
// the disposition contract and the blocker breaker for the most common FAIL
// class. unit-green != live-green, again.
```

### `go/internal/core/failure_digest_identity_test.go:3` — above `import (`

```text
// failure_digest_identity_test.go — the identical-fingerprint breaker's
// identity must be CONTENT-BEARING (batch-14 halt, 2026-07-28). Cycles
// 1137/1139/1143 were three DISTINCT, progressing auditor findings on one
// task (zero coverage → grace never defaulted → gc.mode=off ignored), yet all
// three digests hashed the same content-free router line
// ("phase audit verdict FAIL routed to retro …") to one fingerprint and
// false-tripped the breaker: verdictFailDistinguisher's task-id layer cannot
// separate same-task retries, and its defect layer only matched "- D" bullet
// formatting while these auditors emit the schema-v2 sentinel defects list.
```

### `go/internal/core/failure_digest_identity_test.go:72` — above `func TestFailureDigest_SameTaskDistinctDefectsGetDistinctFingerprints(t *testing.T) {`

```text
// TestFailureDigest_SameTaskDistinctDefectsGetDistinctFingerprints is the
// batch-14 live pin: the three real first-defect heads, same committed task,
// must mint three DIFFERENT fingerprints.
```

### `go/internal/core/failure_digest_identity_test.go:249` — above `func TestAbnormalEpilogue_CauseBecomesDistinguisher(t *testing.T) {`

```text
// TestAbnormalEpilogue_CauseBecomesDistinguisher — live pin (batch-19 halt at
// cycle-1208): cycles 1197/1199/1207 aborted in phase build for THREE distinct
// reasons, but the epilogue wrote only the constant template, so all three
// digests were Unexplained with ONE fingerprint and the diagnosability
// breaker had to halt the batch to get an operator. The abort CAUSE — the
// error RunCycle was about to return — is the missing distinguisher: with it
// appended, the digest is content-bearing and distinct per cause; without it
// (nil cause: a bare bounce) the template stays honestly Unexplained.
```

### `go/internal/core/failure_digest_identity_test.go:300` — above `func TestAbnormalEpilogue_TeardownMarkedAndTailKept(t *testing.T) {`

```text
// TestAbnormalEpilogue_TeardownMarkedAndTailKept pins the review decisions on
// the cycle-1208 fix: (1) a teardown-shaped cause (IsInfraTeardownError) is
// marked "teardown=" — its identical fingerprints DELIBERATELY stay in the
// identical-fingerprint population (one infra condition mowing down three
// lanes SHOULD stop the batch at the ceiling; the marker keeps the halt
// legible) — and (2) truncation keeps the error-chain TAIL, where identity
// lives, so two roots under one long shared prefix never collapse.
```

### `go/internal/core/failure_digest_named_test.go:3` — above `import "testing"`

```text
// failure_digest_named_test.go — apicover per-symbol naming (the two-signal
// convention: every exported symbol named by a test with a REAL assertion).
// RecurrenceCounter is the assembler's read-only ledger seam; this test pins
// that a custom implementation's count flows through into the digest — the
// 6th recurrence of the "exported but never named" parity class, caught on
// PR #350's CI.
```

### `go/internal/core/failure_digest_pathvariance_test.go:3` — above `import "testing"`

```text
// failure_digest_pathvariance_test.go — RED contract for cycle-1440 task
// `fingerprint-normalizer-path-variance`.
//
// Defect (PR #442 diff-review LOW): normalizeReasonForFingerprint strips exactly
// two identity-noise tokens — narrative=<verdict> and go-test durations. Every
// other per-cycle-varying token still splits ONE recurring defect into N
// fingerprints, so the identical-fingerprint breaker (ceiling 3, standing rule
// three_consecutive_fails_halt) never reaches its ceiling and the batch keeps
// burning cycles on the same defect. The two live shapes are:
//
//	1. cycle-numbered PATHS — ".evolve/runs/cycle-1365/audit-report.md" vs the
//	   same artifact one cycle later. Same defect, two fingerprints.
//	2. ATTEMPT DENOMINATORS — "attempt 1/3" vs "attempt 2/3" in a retry-loop
//	   abort reason. Same defect, three fingerprints — exactly the count the
//	   breaker needs to see as ONE.
//
// Contract: both fold to a stable token; the DEFECT-identifying content (which
// gate, which predicate, which artifact FILE) stays untouched, so two different
// defects can never collapse into one fingerprint (the over-normalization
// hazard the current doc comment calls out as deliberately avoided).
```

### `go/internal/core/failure_digest_test.go:3` — above `import (`

```text
// failure_digest_test.go — RED contract for the S1 failure-digest-assembler
// (cycle-1034, item failure-disposition-router slice S1).
//
// The assembler is the deterministic post-FAIL / pre-retro step that converts a
// failed cycle's forensic artifacts into a stable failure IDENTITY the S2
// disposition gate can cross-check against — closing lesson_to_action_gap (the
// agent can no longer INVENT the failure's identity in retro).
//
// SEAM CHOICE (surfaced per Core Rule 3, "no silent changes"): scout-report.md
// lists a rich input set (audit-fail-reason.json + CycleResult.FailReasons +
// phase outcomes + dossier + git state + infra signals). This contract reads the
// single workspace SSOT artifact `audit-fail-reason.json` (schema:
// {schema_version, phase, reasons[]}, already emitted by the coherence floor —
// system_failure_test.go:202) as the fingerprint/bucket source, mirroring
// readFailureDecision's workspace-file boundary. Folding the other signals into
// that one artifact keeps the input surface minimal (Rule 2) without weakening
// the identity — the Builder MAY widen the input later, but the tests below pin
// only the observable contract (bucket, determinism, recurrence, fail-soft,
// atomic write), never an internal input shape.
//
// RED today: AssembleFailureDigest / FailureDigest / RecurrenceCounter do not
// exist, so this file fails to COMPILE — the correct RED for a new-surface task.
```

### `go/internal/core/failure_digest_test.go:66` — above `{"c1329_ship_repo_contract_gate", "ship",`

```text
// c1329 (pipeline-defect-pipeline-blocker Task 2): the ship-time
// repo-contract-gate rejection's actual message vocabulary. Before
// this fix no needle matched it, so it fell into "unknown" —
// indistinguishable from every other unclassified failure, which let
// 3 recurrences trip the identical-fingerprint breaker
// (ship|unknown|76d0f4fca190) and false-halt the batch.
```

### `go/internal/core/failure_dossier.go:3` — above `import (`

```text
// failure_dossier.go — ADR-0072 S4 Task 1 (evidence-dossier-builder). The
// dossier is the orchestrator's independent evidence packet for classifying a
// failure: it composes the verdict-coherence signal, the audit's SELF-declared
// failure envelope, and the non-progress counters — NEVER the recorded verdict
// alone (the forged-verdict lesson from the clean-exit storm). It is written
// per failing cycle so retros/operators have the untruncated "why" on disk.
//
// The symbols are deliberately UNEXPORTED (JSON-tagged fields only): the dossier
// is an internal decision primitive, so it adds no apicover-gated public surface.
```

### `go/internal/core/failure_dossier.go:24` — above `type auditDeclared struct {`

```text
// auditDeclared is the audit phase's SELF-declared failure envelope (ADR-0039
// §7), surfaced verbatim so the orchestrator judgment layer can classify a
// system-class fault the deterministic floor cannot catch (the cycle-1001
// prose-only shape).
```

### `go/internal/core/failure_dossier_test.go:3` — above `import (`

```text
// failure_dossier_test.go — cycle-1002 RED contract for ADR-0072 S4 Task 1
// (evidence-dossier-builder). The dossier composes INDEPENDENT evidence — the
// coherence signal, the audit's self-declared failure envelope, and the
// non-progress counters — never the recorded verdict alone (the forged-verdict
// lesson). These tests fail RED until Builder adds buildFailureDossier /
// writeFailureDossier + the failureDossier type in failure_dossier.go.
```

### `go/internal/core/failure_dossier_test.go:20` — above `func writeAuditWithFailure(t *testing.T, dir, verdict, class string, defects ...string) {`

```text
// writeAuditWithFailure writes an audit-report.md carrying a v2 verdict sentinel
// with a structured failure block (ADR-0039 §7) — the shape the dossier parses
// to surface the audit's SELF-declared class/defects.
```

### `go/internal/core/failure_dossier_test.go:108` — above `t.Run("cycle1001_prose_system_surfaced_for_judgment", func(t *testing.T) {`

```text
// (c) CYCLE-1001 PROSE-ONLY: the audit self-declares a SYSTEM-class fault in
// its defects PROSE while the structured class stays task-level
// (code-audit-fail). The deterministic floor cannot catch it (FloorCandidate
// empty) — but the dossier MUST surface the class + defects so the
// orchestrator judgment layer can classify it. This is the exact shape the
// live cycle-1001 case looped through as task-level.
```

### `go/internal/core/failure_dossier_test.go:134` — above `t.Run("ship_phase_explained_no_floor_candidate", func(t *testing.T) {`

```text
// (d) SHIP-PHASE EXPLAINED (cycle-1329, pipeline-defect-pipeline-blocker
// Task 1's failure_dossier.go twin): green audit + green ACS, but a real
// post-audit ship-gate rejection (REPO_CONTRACT_GATE) is recorded via
// cs.ShipFailReasons. The dossier's SubstantiveError computation
// (failure_dossier.go:86) must fold this carrier in exactly like
// system_failure.go:184 does — a diagnosed ship failure is coherent, so
// it must NOT propose the verdict-incoherence floor candidate. This is
// the dual-call-site lockstep the cycle-1046 comment requires: fixing
// only system_failure.go and leaving this twin stale reproduces the
// exact bug class.
```

### `go/internal/core/failure_floor_verdict_test.go:12` — above `func TestFloorVerdictError_JoinsErrorSeverityDiagnostics(t *testing.T) {`

```text
// Class fix (skills-drift storm, cycles 836/838/841/843/849): a FLOOR phase
// (audit) that returns a FAIL verdict with NO dispatch error — audit's in-process
// CI-parity gates override a narrative PASS to FAIL — was never fed to
// failure-learning, so state.FailedAt stayed empty and the failure-adapter +
// Scout could not learn the recurrence. These tests pin the new success-path
// learning: the synthesized reason and the FailedAt/carryover record.
```

### `go/internal/core/failure_hook.go:3` — above `import (`

```text
// failure_hook.go — ADR-0044 C3: the orchestrator's escalate→advise→promote
// hook, the composition point where the chain's ActionAdvise verdict becomes
// a real LLM consultation. Runs ONLY when the program dial is at enforce
// (cfg.PhaseRecovery — shadow, the default, never dispatches an advisor) and
// ONLY for an artifact-timeout abort whose escalation report carries a pane
// the deterministic registry cannot classify. Strictly best-effort: every
// failure is a WARN; the hook never alters the abort control flow — it runs
// AFTER the phase outcome is recorded and the failure diag is written, and
// its only durable side effect is a validated promotion
// (recovery.PromoteAdvice) that makes the NEXT occurrence deterministic.
```

### `go/internal/core/failure_hook.go:97` — above `if cause, _, known := det.Detect(recovery.StripAgentContent(report.FinalPane, "", nil)); known {`

```text
// Scan the agent-STRIPPED pane, not the raw escalation evidence. The
// short-circuit below is deterministic-FIRST, so a false match here does not
// kill a phase — it does something quieter and worse: a genuinely NOVEL
// wedge whose pane merely carries agent-authored diff content quoting a
// seeded signature (an agent editing the registry, a builder writing a
// fatal-pane fixture) reads as "already classified". No advisor, no
// promotion, nothing learned, and the next occurrence burns the ~20 min
// maxExtends backstop again — ADR-0044's learning loop switched off by the
// agent's own text. Same stripper, same rules as the C2 bridge seam
// (recovery/strip.go). The empty prompt is deliberate: for a fatal-pane scan
// the echo half is neutered by the protect-list anyway (D2), so plumbing the
// phase prompt in here would add I/O and zero behaviour. The protect list is
// nil for the same reason — it is read only inside the echo branch, which an
// empty prompt disables (salvage review LOW: det.Signatures() here was a
// provably dead allocation per aborted phase).
```

### `go/internal/core/failure_hook_strip_c1123_test.go:3` — above `import (`

```text
// failure_hook_strip_c1123_test.go — cycle-1123 RED tests for
// `fatalpane-strip-agent-content` at the SECOND raw-Detect call site:
// adviseOnUnclassifiedFailure (failure_hook.go:87).
//
// The hook's deterministic-first short-circuit — "pane already classified, skip
// the advisor" — reads report.FinalPane RAW. So a phase that aborted on a
// GENUINELY NOVEL wedge, whose final pane happens to carry agent-authored diff
// content quoting a seeded signature (an agent editing recovery/detector.go,
// a builder writing a fatal-pane regression fixture, this very cycle's work),
// is misread as "known". The C3 advise→promote path never runs, nothing is
// learned, and the next occurrence burns the maxExtends backstop again.
//
// The fix is the SAME stripper the bridge seam already uses, lifted to the
// registry's owning package and called here before Detect:
//
//	recovery.StripAgentContent(report.FinalPane, "", det.Signatures())
//
// The empty injectedPrompt is DELIBERATE, not an omission: for a fatal-pane
// scan the echo half is neutered by the protect-list anyway (D2 — a line
// carrying a seeded signature is never echo-stripped), so plumbing the phase
// prompt into the hook would add I/O and zero behaviour. The diff half is what
// closes this class. Passing "" is the documented fail-open value.
//
// DO NOT MODIFY THESE TESTS to make them pass. The two negative tests below
// (real chrome, and a real anchored seed under agent diff content) are the
// guard against the lazy over-fix — stripping so eagerly that the
// deterministic-first path stops working at all.
```

### `go/internal/core/failure_hook_strip_c1123_test.go:93` — above `writeEscalation(t, ws, "retro", "⏺ There's an issue with the selected model (auto). It may not exist.")`

```text
// The real cycle-262 claude pane — no diff prefix, genuine chrome.
```

### `go/internal/core/failure_hook_strip_c1123_test.go:106` — above `func TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor(t *testing.T) {`

```text
// TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor is D1 at THIS call site:
// the pane carries a genuine zsh continuation prompt (a real dead shell)
// directly below agent diff content. Blanked in place the anchor survives and
// the pane stays classified; deleted, the survivor loses its leading "\n", the
// dead shell reads as novel, and the hook spends a 3-minute LLM consultation
// re-learning a signature seeded since cycle-274.
```

### `go/internal/core/failure_hook_test.go:3` — above `import (`

```text
// failure_hook_test.go — ADR-0044 C3 (Slice 6) tests: the orchestrator's
// escalate→advise→promote hook. The plan's named invariant
// (TestPhaseRecovery_ShadowDefault_NoCorrectiveAction) lives here: below
// enforce the advisor is NEVER consulted, and even at enforce the
// deterministic registry is checked FIRST (known panes never pay for an LLM
// call — Rule 5).
```

### `go/internal/core/failure_hook_test.go:104` — above `writeEscalation(t, ws, "retro", "⏺ There's an issue with the selected model (auto). It may not exist.")`

```text
// The real cycle-262 claude pane — seeded, therefore deterministic.
```

### `go/internal/core/failure_learning.go:38` — above `func phaseOutcomeFrom(phase Phase, resp PhaseResponse, attempts int, abortReason, startedAt string) recovery.PhaseOutcom…`

```text
// phaseOutcomeFrom builds the single-source outcome record for one phase
// dispatch (ADR-0044 C1). The verdict reconciliation rule lives HERE and only
// here: a canonical agent verdict is recorded as-is; anything else (empty,
// non-canonical, error-path zero response) synthesizes FAIL. A synthesized
// PASS is structurally impossible — reconciliation only ever describes what
// the agent itself reported.
```

### `go/internal/core/failure_learning.go:65` — above `func (o *Orchestrator) recordPhaseOutcome(result *CycleResult, timings *[]phaseTimingEntry, workspace string, out recove…`

```text
// recordPhaseOutcome is the C1 recording chokepoint (ADR-0044): EVERY
// terminal disposition of a dispatched phase — happy advance AND each abort
// return (exhausted retries, non-canonical verdict, review-gate reject,
// ship-error recovery, worktree-leak recovery failure, tree-diff guard,
// ledger/state persistence failure) — funnels through here exactly once, so
// PhasesRun, phase-timing.json, and <phase>-usage.json always reflect what
// actually ran. cycle-262: the build ran, PASSed, and burned tokens, but the
// tree-guard abort path skipped all three records — the divergence this
// chokepoint makes structurally impossible. Paths where the phase never
// dispatched (no runner registered, pre-phase state-write failure) have no
// outcome to record and stay bare.
```

### `go/internal/core/failure_learning.go:78` — above `if out.AbortReason != "" {`

```text
// ADR-0048 Slice A (SHADOW): grade the abort reason. Observe-only — logs the
// tier graduated-enforcement WOULD apply; changes nothing (the floor still
// aborts). Evidence is conservative here (the per-site benign-churn /
// verified-rebuild predicates are plumbed in the enforce slice), so only the
// always-correctable classes surface in shadow today.
```

### `go/internal/core/failure_learning.go:139` — above `func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {`

```text
// recordFailureLearning is the chokepoint every failed phase reaches — the
// coordinator of the failure-learning spine (ADR-0103 unit 03b): the gate,
// the carrier, the recorder (invariant D: before the doc-missing arm and the
// runner lookup), the deterministic arms, the retro dispatch and its
// completion. Every stderr line below is verbatim; unit 05 codes them.
```

### `go/internal/core/failure_learning.go:159` — above `if errors.Is(fl.Err, ErrAgentDocMissing) {`

```text
// A missing persona doc is a deterministically KNOWN configuration absence
// (cycle-1551 class; cycles 1619/1620 spent a deep-tier agent on exactly
// this): learn it deterministically — the FailedRecord and carryover todo
// above, the failure digest and the lesson artifact — no LLM dispatch.
```

### `go/internal/core/failure_learning.go:293` — above `func (o *Orchestrator) recorder() *outcome.Recorder {`

```text
// recorder returns the unit-01 recorder (ADR-0103). NewOrchestrator builds it
// eagerly with the root's Center. An Orchestrator assembled as a literal —
// the test constructions this package keeps — lazily builds and caches that
// same live recorder on first use. A nil orchestrator (a bare cycleRun with
// no orchestrator at all) gets a fresh bare, no-op Recorder on every call, so
// every path that records or flushes still has ONE writer.
```

### `go/internal/core/failure_learning_engine.go:11` — above `func (o *Orchestrator) failureLearning() *failurelearning.Engine {`

```text
// failureLearning returns the unit-03b engine (ADR-0103): NewOrchestrator
// builds it eagerly with the root's Center; an Orchestrator assembled as a
// literal — the remediation and floor-verdict tests build one — lazily builds
// and caches the same live engine on first use. No nil-orchestrator branch:
// every caller is an Orchestrator method.
```

### `go/internal/core/failure_learning_expiry_test.go:1` — above `package core_test`

```text
// failure_learning_expiry_test.go — cycle-516 task
// `carryover-todo-expiry-never-set` (RED).
//
// state.go:87-91 documents CarryoverTodo.ExpiresAt as "inherited from the
// FailedRecord that created this todo" so PruneExpiredCarryoverTodos
// (wired into cmd_loop.go, called at every loop start) can age entries out.
// But recordFailureLearning — the ONLY non-test call site that creates both
// the initial per-cycle-failure CarryoverTodo and its sibling FailedRecord —
// never assigns ExpiresAt on either. Live evidence: none of the 71
// cycle-N-failed-* entries in .evolve/state.json carry an expiresAt key, so
// the already-wired prune pass is a permanent no-op in production.
//
// This isn't a prune-logic bug (PruneExpiredCarryoverTodos and
// ApplyDefectsAsCarryoverTodos already have full, GREEN coverage — see
// prune_carryover_test.go and carryover_ttl_stamp_test.go). It's a
// never-populated-input bug at the creation site. These tests exercise the
// REAL creation path end-to-end (not a hand-built fixture), which the scout
// report flags as the actual gap: "no test asserts the two compose correctly
// on the real creation path".
//
// Shares the core_test harness (newRunners / newTestOrchestrator /
// seedCycleStateFile / alwaysErrRunner / recordingRetroRunner) defined in
// orchestrator_recovery_test.go and orchestrator_phaseboundary_test.go.
```

### `go/internal/core/failure_learning_spine_test.go:3` — above `import (`

```text
// failure_learning_spine_test.go — ADR-0103 unit 03b, commit 1: the pins on
// recordFailureLearning's spine that were comments (or nothing) before the
// split. Every test here is green on the pre-split code and red against the
// named mutant recorded in docs/architecture/decomposition/03b-failure-learning-engine.md
// §6/§8. Fakes and temp dirs only; the stderr-capturing tests must not run in
// parallel (os.Stderr is process-global — captureStderr's contract).
```

### `go/internal/core/failure_learning_spine_test.go:28` — above `func spineRequest(dir string, err error) failureLearningRequest {`

```text
// spineRequest is the 11-field DTO the seven by-name tests build literally,
// built here ONCE for the spine pins (audit failed, cycle 1034, the workspace
// and project root in dir).
```

### `go/internal/core/final_verdict_floor.go:5` — above `func (o *Orchestrator) isAuthoritativePhase(phase Phase) bool {`

```text
// final_verdict_floor.go — cycle-802 floor-gated FinalVerdict guard
// (retro-bridge-timeout-width10).
//
// Root cause: both the main dispatch loop (cyclerun_record.go) and the resume
// loop (resume.go) wrote `result.FinalVerdict = <phase verdict>` UNCONDITIONALLY
// at the end of every phase. So a POST-verdict, non-floor phase (retrospective,
// memo, the *-scans, router/advisor) failing under quota (exit=85) or artifact
// timeout (exit=81) clobbered an already-PASS audit verdict back to FAIL and
// zeroed the whole wave (waves 17-19 of the 2026-07-13 resume batch went 0/3).
//
// Fix: only an AUTHORITATIVE phase — a ship-floor phase (tdd/build/audit, or the
// user-configured Policy.FloorPhases override, resolved via resolvedShipFloor)
// or ship itself — may set FinalVerdict directly. Once one has, a non-floor
// phase's non-PASS outcome is preserved-around and recorded into
// CycleResult.VerdictsNotAdopted (surfaced in the dossier as
// phases_run_verdict_not_adopted), never overwriting the
// floor verdict. Before any authoritative verdict exists (pre-audit phases,
// scout-only investigation cycles) legacy behavior is preserved: the phase's
// verdict stands, so no cycle loses its outcome.
//
// The floor-already-recorded predicate is DERIVED from cs.CompletedPhases (the
// persisted phase log) rather than a mutable flag, so the resume path — which
// re-enters mid-cycle after audit already completed in a prior session — gets
// the identical guard for free.
```

### `go/internal/core/final_verdict_floor.go:80` — above `func (o *Orchestrator) nonFloorExhaustionDegrade(phase Phase, workspace string, floorAlreadyRecorded bool) (PhaseRespons…`

```text
// nonFloorExhaustionDegrade decides whether a phase that exhausted its retries
// with a non-canonical verdict should degrade to SKIPPED+WARN instead of
// aborting the cycle (cycle-802 Task 3, subsuming advisory-phase-contract-
// degrade). It degrades ONLY a POST-verdict non-floor phase — a non-floor phase
// running after the floor already passed (retro/memo/scans/advisor). An
// authoritative phase, OR any non-floor phase BEFORE the floor is established
// (scout/triage/intent, whose unparseable verdict must stay cycle-fatal — you
// cannot proceed without a scout), returns ok=false and keeps the abort.
```

### `go/internal/core/final_verdict_floor_test.go:9` — above `func hasNotAdopted(recs []VerdictNotAdopted, phase string) bool {`

```text
// final_verdict_floor_test.go — cycle-802 (retro-bridge-timeout-width10)
// acceptance tests. These are the NAMED internal/core unit tests the ACS
// predicates in go/acs/cycle802 invoke as a `-race` subprocess. Each exercises
// the floor-gated FinalVerdict guard (final_verdict_floor.go) — the fix for the
// storm where a non-floor retro/memo FAIL clobbered an audit PASS.
//
// The completed-phase log passed to floorAlreadyCompleted mirrors the two write
// sites (cyclerun_record.go / resume.go), where CompletedPhases already includes
// the phase being recorded.
```

### `go/internal/core/final_verdict_floor_test.go:19` — above `func hasNotAdopted(recs []VerdictNotAdopted, phase string) bool {`

```text
// hasNotAdopted reports whether a phase's declined verdict was preserved. The
// records live in CycleResult.VerdictsNotAdopted (dossier
// phases_run_verdict_not_adopted); they used to be filed under SkippedPhases,
// which made the dossier claim a phase that RAN had been skipped
// (dossier-retro-skipped-mislabel). What cycle-802 requires is unchanged: the
// degrade is PRESERVED, never dropped.
```

### `go/internal/core/final_verdict_floor_test.go:154` — above `func TestDossier_RecordsSkippedPhases(t *testing.T) {`

```text
// AC6: degraded/skipped non-floor phases are surfaced in the dossier, never
// dropped. The record is SPLIT BY WHAT ACTUALLY HAPPENED (dossier-retro-skipped-
// mislabel): a phase that RAN and had its verdict declined goes to
// CycleResult.VerdictsNotAdopted → phases_run_verdict_not_adopted, while
// skipped_phases keeps its literal meaning for a phase that did not run.
// Cycle-802's requirement — the outcome survives instead of clobbering the floor
// verdict — is what both halves preserve.
```

### `go/internal/core/fleet_lock_test.go:8` — above `func TestRunCycle_FleetMode_SkipsGlobalLock(t *testing.T) {`

```text
// TestRunCycle_FleetMode_SkipsGlobalLock pins ADR-0049 S6 / root-cause R1: under
// the fleet supervisor (EVOLVE_FLEET=1) a cycle must NOT take the whole-cycle
// global project lock (LOCK_NB), which refuses concurrent runs. Concurrent fleet
// cycles run in separate processes, each isolated by its per-run worktree +
// workspace and serialized on every SHARED resource by that resource's own flock
// (state.json via UpdateState/withStateLock, the ledger chain, the .evolve/
// ship.lock integrator) — the safety nets S2–S5 put in place. RED before the
// fleet gate (lockCount=1), GREEN after (0).
```

### `go/internal/core/floor_activation_scenarios_test.go:3` — above `import (`

```text
// End-to-end orchestrator scenario catalog for the ADR-0024 §1 conditional
// integrity floor ACTIVATION (PR-5 live-wiring): the orchestrator computes the
// upfront whole-cycle plan, clamps it to the floor, and lets it DRIVE phase
// selection at Stage>=Advisory. Proves the advisor drives the non-mandatory
// surface, the floor forces the ship-chain regardless of how small the operator
// makes cfg.Mandatory, and a planner failure degrades cleanly to the static
// spine. Built on the configurable routingtest framework.
```

### `go/internal/core/floor_activation_scenarios_test.go:71` — above `Scenario("hybrid cadence: Propose fires only at branch transitions under a plan",`

```text
// Hybrid cadence (ADR-0024 §2): with the upfront plan driving, the
// per-transition Proposer fires ONLY at branch transitions (post-build,
// post-audit) — not at start/scout/tdd/ship. Proves the double-spend is
// removed without changing the routed sequence.
```

### `go/internal/core/floor_debt_named_test.go:3` — above `import (`

```text
// floor_debt_named_test.go — pays the cycle-1048 debt: four core exports were
// exercised only under integration tags, reading 0% (false-green) in every
// scoped coverage run and floor-blocking all core-touching lanes. Default-tag
// tests with real assertions.
```

### `go/internal/core/gate_signal_test.go:3` — above `import (`

```text
// gate_signal_test.go — ADR-0101 S2b: the correction ladder is a producer.
// Every rung the orchestrator runs after a gate rejection is ONE gate.corrected
// INFO under module orchestrator, on BOTH dispatch roots, naming the correction
// ordinal, the budget, the rung and the CLI it re-dispatched on — so the stream
// shows "rejected → corrected (1/2, redispatch) → passed" for a phase boundary.
```

### `go/internal/core/gate_test.go:3` — above `import (`

```text
// gate_test.go — PA-DDK DDK-4 (ADR-0060): the artifact-floor THRESHOLDS are
// config-driven. The evaluator's verdict requirement comes from the loaded
// registry gate (config), evaluated against the trusted Go signal digest. Phases
// are resolved through the kerneltest fixture — no hardcoded names.
```

### `go/internal/core/git_porcelain.go:14` — above `var gitRunner sysexec.RunFunc = sysexec.DefaultRunner`

```text
// gitRunner is core's git-execution seam: every git invocation in this package
// — defaultGitHEAD, gitCapture and the ~25 callers that funnel through it, plus
// the per-cycle worktree/resume/correction git calls — routes through it, so
// the fast test tier fakes git via a sysexec.RunFunc instead of shelling out.
// Tests swap it (see git_seam_test.go); production uses sysexec.DefaultRunner.
// S4.5 (ADR-0050) replaced this package's hardcoded exec.Command calls with the
// internal/gitexec leaf driven by this seam.
```

### `go/internal/core/git_porcelain.go:32` — above `func porcelainDirtySet(ctx context.Context, dir string) map[string]bool {`

```text
// emitPhaseBindings writes the per-agent provenance ledger entries ship's
// verification requires after a phase completes (see recordAuditBinding /
// recordBuildBinding for the per-entry contracts). Shared by RunCycle and
// RunCycleFromPhase — the resume path originally skipped these, so a resumed
// audit→ship bound to a stale auditor entry from an earlier cycle and always
// failed AUDIT_BINDING_HEAD_MOVED (cycle-294 incident, 2026-06-12).
// Best-effort: failures are logged to stderr; ship then refuses to bind.
```

### `go/internal/core/git_porcelain.go:70` — above `func gitCapture(ctx context.Context, dir string, args ...string) (string, int, error) {`

```text
// recoverBuildLeak relocates build-phase writes that escaped into the main tree
// back into the worktree, then restores the main tree — the self-heal for the
// cycle-160 incident (Option A). Non-Claude builders (agy/codex in tmux) are not
// bound by the Claude-only role-gate, and the OS sandbox is off on nested-macOS,
// so they can write to project_root instead of the worktree. Rather than abort
// the cycle, move the build's output to where audit/ship expect it.
//
// baseline (file-granular via `git status --porcelain -uall`) = paths already
// dirty in projectRoot before the build (operator / pre-existing work) — never
// touched. For each NEW dirty path:
//   - untracked ('?')                 → os.Rename(projectRoot/p → worktree/p);
//     the relocated paths (and ONLY those) are then `git add --`'d in the worktree
//     so the auditor's `git diff HEAD` sees them without sweeping in unrelated
//     worktree content (same visibility reason as normalizeWorktreeToBase).
//   - rebuilt release binary (buildArtifacts: go/evolve, go/bin/evolve) → always
//     discard (git checkout HEAD -- p); never relocate, or the cycle would commit
//     binary drift (cycle-153). go/evolve is re-committed only by the release pipeline.
//   - modified tracked ('M') → real builder work edited in the MAIN tree (cycle-162:
//     orchestrator.go). If the worktree has NOT independently touched p (its copy is
//     at HEAD) → relocate the leaked content into the worktree (preserve the work) +
//     stage it. If the worktree diverged for p → discard the main leak (worktree is
//     authoritative).
//   - added/deleted tracked ('A'/'D') → git checkout HEAD -- p (discards staged AND
//     unstaged; plain `git checkout -- p` would no-op a staged-only change).
//   - rename/copy/other → not safe to auto-recover → return false.
//
// Returns true iff every NEW leak was handled and the main tree is clean of them;
// the caller continues. On false the caller ABORTS the cycle — the tree-diff
// guard only backstops tracked leaks, so an unrecovered (esp. untracked) leak
// must not be allowed to slip past into audit. "Couldn't determine" cases degrade
// to true (let the guard be the backstop). Best-effort + loud WARNs throughout.
// evolveDeliverablePrefixes are the `.evolve/` subpaths that are REPO CONTENT
// — locations agents legitimately write as cycle deliverables. A leak under
// one of these relocates into the worktree like any other repo path
// (cycle-262: tracked commit-prefix-scope.json edit; cycle-268: a NEW eval
// file — both previously hit the blanket `.evolve/` skip, were unrecoverable,
// and killed their cycles at the tree-diff guard). Everything else under
// `.evolve/` is never relocated: runtime state (runs/, worktrees/,
// state.json, ledger.jsonl, instincts/, nested guards.log — pinned by the
// Skips tests) AND the TRUST-SENSITIVE operator-privilege documents
// (.evolve/profiles/, .evolve/policy.json) — those configure the gates and
// the auditor's own constraints, so an agent write there must stay
// unrecoverable (the guard kills the cycle, forcing human review; an auditor
// cannot safely review the file that redefines the auditor).
```

### `go/internal/core/git_porcelain.go:134` — above `func defaultGitDirtyPaths(ctx context.Context, repoRoot string) ([]string, error) {`

```text
// defaultGitDirtyPaths runs `git status --porcelain -uall` in repoRoot and
// returns the list of dirty paths (tracked-modified AND untracked), one per
// entry. Porcelain granularity is required for the tree-diff guard to catch
// NEW UNTRACKED files written by inserted/non-worktree phases — the tracked-
// only `git diff --name-only HEAD` baseline that preceded this missed them
// (the cycle-270 root cause). Errors propagate so the guard degrades to
// "snapshot missed" rather than misreport leaks.
```

### `go/internal/core/hermetic_project_root_test.go:3` — above `import (`

```text
// hermetic_project_root_test.go — no test anywhere under go/internal may pin a
// shared, machine-global project root. Every RunCycle writes real state under its
// ProjectRoot (.evolve/runs/, worktree bases, archived-polluted moves), so a
// FIXED path shared by every test binary on the host is cross-process mutable
// state: concurrent fleet lanes each running this suite sampled each other's
// writes. Measured live 2026-07-27: the shared root had accumulated 19,521
// run entries, and its pollution signature ("archived polluted workspace…",
// "fatal: not a git repository") is verbatim what failed the suites_stay_green
// meta-predicates of cycles 1107 and 1116 — false FAILs on an idle-host-green
// suite. Per-test t.TempDir() makes each test own its root; this guard keeps
// the class dead.
//
// Reach (cycle-1128): the walk covers the whole go/internal tree, recursively.
// A package-local scan only kept the class dead in internal/core — a re-pin in
// any sibling or nested package would have shipped undetected, which defeats
// the guard's stated purpose.
```

### `go/internal/core/infra_teardown_scan_scope_test.go:3` — above `import (`

```text
// infra_teardown_scan_scope_test.go — cycle-1270 Task 1
// (`infra-teardown-predicate-single-source`), the open residual.
//
// The consolidation itself is landed and green: IsInfraTeardownError is adopted
// at orchestrator.go and cyclerun_dispatch.go, and
// TestInfraTeardownUnion_SpelledExactlyOnce guards it. But that guard scans
// `.` — internal/core ONLY. The predicate's consumers live OUTSIDE it
// (phases/runner, bridge), so a re-spelled union in either package passes the
// "spelled exactly once" check untouched.
//
// No live duplicate exists today, which is precisely why this is pinned now:
// the guard's whole purpose is the item's own "if a THIRD sentinel is ever
// added" concern, and a guard that cannot see two thirds of its own blast
// radius does not serve it.
```

### `go/internal/core/infra_teardown_single_source_test.go:3` — above `import (`

```text
// infra_teardown_single_source_test.go — RED contract for cycle-1166 Task 2
// (infra-teardown-predicate-single-source, inbox weight 0.86).
//
// The union predicate "is this a bridge infra teardown?" —
//
//	errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure)
//
// — now has a single-source home (core.IsInfraTeardownError, errors.go:81) but
// is STILL hand-spelled at call sites (most notably optionalInfraSkip in
// orchestrator.go, which spells the De Morgan negation
// `!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)`).
// That is the 7th spelling of one concept: if a THIRD teardown sentinel is ever
// added, every hand-spelled site must be found and updated or the definitions
// silently diverge.
//
// This is a BEHAVIOR-PRESERVING refactor, so most assertions here are pins that
// must stay green before AND after (they encode "you did not change semantics").
// The one genuinely RED assertion is the UNIQUENESS scan.
//
// The item's own loudest warning is the anti-goal: "this item's whole risk is a
// blind widen of a timeout-only or transient-only site into the union." The
// negative tests below encode exactly that.
```

### `go/internal/core/infra_teardown_single_source_test.go:111` — above `func TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError(t *testing.T) {`

```text
// TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError pins the gate
// to its single-source predicate: for a phase that clears every OTHER guard
// (optional, off-floor, non-mandatory), optionalInfraSkip's verdict must equal
// IsOptionalSkippableError's verdict for every error shape — infra teardown
// AND the missing-persona class (cycle-1551) alike. If these ever disagree,
// the site has grown a private error taxonomy and must be re-single-sourced
// through core/errors.go before any consolidation touches it.
```

### `go/internal/core/infra_teardown_timeout_only_test.go:3` — above `import (`

```text
// RED contract for cycle-1267 Task 2
// (`verify-infra-teardown-predicate-consolidation`, inbox
// infra-teardown-predicate-single-source, w=0.86) — the acceptance criterion
// that has NO pin today:
//
//	"NO site that is timeout-only or transient-only was incorrectly widened to
//	 the union predicate"
//
// The item calls this its whole risk: "this item's whole risk is a blind widen
// of a timeout-only or transient-only site into the union."
//
// The transient-ONLY half is already pinned
// (infra_teardown_single_source_test.go: TestIsTransientBridgeError_StaysTransientOnly),
// and the uniqueness of the union spelling is pinned there too. The TIMEOUT-only
// half is not pinned anywhere: the two sites the inbox item and the cycle-1267
// scout report both name —
//
//	failure_learning.go  writePhaseFailureDiag        (exit-code 81 mapping)
//	failure_hook.go      adviseOnUnclassifiedFailure  (pane-classification gate)
//
// — check ErrArtifactTimeout ALONE and must keep doing so. Nothing stops a
// future "consolidation" from folding either into IsInfraTeardownError, at
// which point a quota bounce (ErrTransientBridgeFailure) would be recorded as
// exit 81 and fed to the fatal-signature classifier as if it carried a
// diagnosable pane. This file makes that regression fail.
//
// This is a behaviour-PRESERVING contract: both pins encode "you did not change
// semantics", which is exactly what the item asks for ("no verdict changes").
```

### `go/internal/core/infra_teardown_timeout_only_test.go:108` — above `gate string`

```text
// gate is the timeout-only expression the body must reference; "" means
// the sentinel itself. Unit 02 (ADR-0103) moved the sidecar writer into
// internal/core/failurediag, which cannot import either sentinel, so the
// gate the orchestrator injects (isArtifactTimeout) is what the pin reads.
```

### `go/internal/core/interaction_telemetry_test.go:3` — above `import (`

```text
// interaction_telemetry_test.go — ADR-0045 I1 (slice 1), orchestrator side:
// the contract-correction re-dispatch (PR #60) is an interaction and must
// record a typed Outcome resolved by the re-dispatch verdict; RunCycle's
// deferred persistence writes the per-cycle interaction-summary.json rollup
// beside phase-timing.json. White-box: reuses the fakeStorage / fakeLedger /
// buildRunners / sequencedReviewer harness.
```

### `go/internal/core/judgment_lesson_test.go:3` — above `import (`

```text
// judgment_lesson_test.go — a JUDGMENT phase's FAIL verdict must leave a lesson.
//
// The defect: a judgment phase (premise-challenge, plan-review,
// adversarial-review) returns FAIL as a VERDICT with err == nil. Neither
// learning path catches that shape:
//
//   - recordFloorVerdictFailure fires only for isAuthoritativePhase() — the
//     resolved ship floor {tdd,build,audit} plus ship. No judgment phase is in it.
//   - recordFailureLearning early-returns unless fl.Err != nil. A FAIL verdict
//     carries no error.
//
// So a challenged premise left NO trace: the next cycle's Scout had no memory of
// it and re-derived the same doomed premise. Live instance — cycle-1528's
// premise-challenge FAIL produced a CRITICAL objection that survived only because
// a human copied it into an inbox item by hand.
//
// The fix records ONLY a carryover todo — deliberately NOT a FailedRecord.
// Appending to state.FailedAt would feed the failure adapter, which carries two
// documented halt vectors: tailInfraTransientStreak breaks on any foreign class,
// and sameClassStreak manufactures a streak from consecutive same-class records.
// A phase that TEACHES must not also be able to HALT the batch.
```

### `go/internal/core/lanescope.go:3` — above `import (`

```text
// lanescope.go — lane-identity pin (cycle-640 incident: scout scouted lane A's
// goal while triage was handed lane B's fleet_scope, so the run had no coherent
// lane identity). The fleet supervisor (or this orchestrator, from the per-cycle
// env snapshot) materializes <workspace>/lane-scope.json BEFORE any phase runs;
// that file — not the env — is then the authoritative fleet_scope source for
// every phase, and its goal_hash anchors the scout→triage coherence gate.
// Every degraded path here fails OPEN (WARN + legacy behavior): a guard that
// false-aborts healthy sequential cycles would recreate the cycle-760..762
// destruction class.
```

### `go/internal/core/lanescope.go:26` — above `var scoutArtifactName = func() string {`

```text
// scoutArtifactName is the scout deliverable's filename, DERIVED from the
// phasecontract registry (the report-filename SSOT) rather than re-typed here.
// Cycle-1141: lane-scope reconciliation reads the scout report by name, so a
// frozen copy of that name silently degrades goal_hash normalization into its
// fail-open branch the moment the registry moves — a lane-identity bug that
// looks like an absent report.
```

### `go/internal/core/lanescope.go:142` — above `func normalizeScoutGoalHash(workspace string) {`

```text
// normalizeScoutGoalHash is the scout→triage lane-identity reconciliation
// (supersedes the cycle-640 hard-abort gate). The scout prompt asks the LLM to
// echo the pinned goal_hash into its Decision Trace, but that echo proved a
// fragile signal: a DETERMINISTIC transcription flip (cycles 945/947/... —
// greedy decoding reproduces the same wrong digit every run, so retries and
// batch re-runs can never self-heal) made the old gate false-abort healthy
// cycles before triage. The pinned lane-scope.json goal_hash is the
// AUTHORITATIVE lane identity — and the echo verified nothing the per-cycle
// workspace isolation + the fleet_scope directive don't already guarantee (the
// LLM echoes the pin regardless of what it actually scouted, so the echo never
// even caught the cycle-640 split it was added for). So on a divergence, this
// machine-STAMPS the pin into the report (triage then runs on a coherent lane)
// and WARNs so the mis-echo stays visible — never a silent proceed, never a
// false abort. The stamp is guarded: it fires only when the mis-echoed value is
// itself a canonical goal hash, so the whole-file replace can never corrupt
// unrelated report content off a malformed echo. Fail-open with a WARN on every
// unexpected degraded path (unreadable report, non-canonical echo, write
// failure); a truly absent report / pin / goal_hash key is a silent no-op.
```

### `go/internal/core/lanescope_apicover_test.go:3` — above `import (`

```text
// lanescope_apicover_test.go — ADR-0050 Phase-5 public-API coverage for the
// fleet lane-scope pin (cycle-808 soak-invariants-reconcile: the landed
// lane-scope sweep left LaneScope / LaneScopeFile unnamed by any test, so the
// apicover per-package hard-fail gate flagged them UNCOVERED and turned the go
// workflow RED on main). This white-box test NAMES + EXERCISES both symbols
// against the production writer — no `_ = pkg.X` padding.
```

### `go/internal/core/lanescope_normalize_test.go:3` — above `import (`

```text
// lanescope_normalize_test.go — focused unit coverage for
// normalizeScoutGoalHash, the scout→triage lane-identity reconciliation that
// SUPERSEDED the cycle-640 hard-abort gate. The orchestrator-level pin test
// (lanescope_pin_test.go) exercises only the mismatch→stamp→proceed happy path;
// this file pins the branches that matter most for NOT regressing to a
// false-abort AND for the blast-radius / no-silent-failure guards the reviewer
// flagged: every degraded input (no pin / no report / no echo) is a silent
// no-op, a coherent report is left byte-identical, a NON-canonical echo is
// refused (never a blind whole-file replace), a valid mis-echo is corrected at
// every occurrence, and a write failure is a loud no-op — never a swallowed
// mismatch.
```

### `go/internal/core/lanescope_normalize_test.go:22` — above `var (`

```text
// pinGoalHash / misGoalHash are canonical 64-hex goal hashes differing by a
// single digit — mirroring the real deterministic transcription flip
// (…c05376e pinned vs …c05356e echoed) that motivated the whole fix.
```

### `go/internal/core/lanescope_normalize_test.go:134` — above `func TestNormalizeScoutGoalHash_FailOpen(t *testing.T) {`

```text
// TestNormalizeScoutGoalHash_FailOpen pins the degraded inputs that MUST be
// no-ops — the cycle-760..762 lesson: a coherence step that touches a
// healthy/absent cycle is worse than the incoherence it chases.
```

### `go/internal/core/lanescope_pin_test.go:3` — above `import (`

```text
// Cycle-766 RED contract — fleet-lane-provisioning-split (inbox id
// fleet-lane-provisioning-split, cycle-640 incident: scout scouted lane A's
// goal while triage was handed lane B's fleet_scope, so the run had no
// coherent lane identity).
//
// Contract encoded here (Builder implements, must NOT modify these tests):
//
//  1. PIN: when `evolve fleet` provides EVOLVE_FLEET_SCOPE, the orchestrator
//     materializes <workspace>/lane-scope.json ({"todo_ids":[...],
//     "goal_hash":"..."}) BEFORE any phase runs.
//  2. INJECT: when <workspace>/lane-scope.json exists (supervisor- or
//     orchestrator-written), Context["fleet_scope"] handed to every phase is
//     derived from THAT file (comma-joined todo_ids) — authoritative over the
//     env snapshot, so cross-lane env drift can no longer split lane identity.
//     Absent file ⇒ legacy env fallback (sequential loop byte-identical).
//  3. COHERENCE (superseded — scout-goalhash-machine-stamped): after scout
//     completes and before triage runs, a scout-report whose Decision Trace
//     goal_hash differs from lane-scope.json's goal_hash is MACHINE-STAMPED to
//     the pin (the authoritative lane identity) + WARNed, and triage PROCEEDS —
//     it no longer aborts. The old hard-abort false-fired on a deterministic LLM
//     transcription flip (retries could never self-heal), and the echo verified
//     nothing the workspace isolation + fleet_scope directive don't already
//     guarantee. Missing report / goal_hash key / lane-scope.json stay
//     fail-open (no-op) — never a false abort on a healthy cycle.
```

### `go/internal/core/lanescope_pin_test.go:108` — above `func TestLaneScopePin_TwoLanesSeeOnlyOwnScope(t *testing.T) {`

```text
// INJECT/negative: two lanes with distinct lane-scope.json each see ONLY their
// own scope even when the env snapshot carries the OTHER lane's scope — the
// exact cycle-640 cross-lane drift. lane-scope.json must win over env.
```

### `go/internal/core/lanescope_pin_test.go:195` — above `func TestLaneScopePin_ScoutGoalHashMismatchNormalizesAndProceeds(t *testing.T) {`

```text
// COHERENCE (superseded contract, scout-goalhash-machine-stamped): a scout-report
// goal_hash ≠ lane-scope.json goal_hash is MACHINE-STAMPED to the pin and triage
// PROCEEDS — it no longer aborts. The old hard-abort false-fired on a
// deterministic LLM transcription flip (cycles 945/947/... — greedy decoding
// reproduces the same wrong digit, so retries never self-heal), and the echo
// verified nothing the workspace isolation + fleet_scope directive don't already
// guarantee. The pin is authoritative; the divergence is reconciled, not fatal.
```

### `go/internal/core/lanescope_pin_test.go:249` — above `func TestLaneScopePin_ScoutReportWithoutGoalHashProceeds(t *testing.T) {`

```text
// COHERENCE/fail-open edge: a scout-report with NO goal_hash key (legacy or
// malformed Decision Trace) must proceed — an over-strict gate that destroys
// healthy cycles would recreate the cycle-760..762 abort class.
```

### `go/internal/core/leak_recoverable_phase_test.go:5` — above `func TestLeakRecoverablePhase_CoversAllWorktreePhases(t *testing.T) {`

```text
// TestLeakRecoverablePhase_CoversAllWorktreePhases is the scout-mandated
// verification anchor (verifiableBy) for
// decouple-leak-recovery-from-worktree-phase-gate: every phase that runs with
// an active cycle worktree (triage, audit, scout, bug-reproduction, tdd,
// build — per the cycle-564 scout finding, cross-referencing the 9 recorded
// tree-diff-leak cycle failures spanning exactly these phases) must be
// eligible for leak recovery, not just the two WorktreePhase (role-gate
// write-permission) phases.
```

### `go/internal/core/leak_recovery.go:35` — above `func recoverBuildLeak(ctx context.Context, projectRoot, worktree string, baseline map[string]bool, sourceWriter bool) bo…`

```text
// recoverBuildLeak relocates/discards a main-tree leak from a phase that runs
// with an active worktree, so the cycle continues instead of hard-aborting on
// the tree-diff guard. sourceWriter distinguishes the two recovery regimes:
//   - true  (tdd/build): full recovery — relocate AND stage untracked + tracked
//     edits into the worktree so the auditor's `git diff HEAD` sees builder work.
//   - false (triage/audit/scout/bug-reproduction, added cycle-564 via
//     LeakRecoverablePhase): SAFE SUBSET only — relocate genuine untracked junk
//     out of main (no staging: an artifact leak is not source, and the worktree
//     may not be a git tree for those phases). Evolve deliverables (scout eval
//     materialization) and tracked edits are left UNTOUCHED so the tree-diff
//     guard's own carve-out / abort (with forensics preserved) still governs
//     them. This keeps recovery from becoming a write-permission hole for
//     non-source-writer phases while still clearing the untracked-temp leaks
//     behind the 9 recorded tree-diff-leak failures.
```

### `go/internal/core/leak_recovery.go:75` — above `if isLegitimateMainTreePath(p) && !buildArtifacts[p] {`

```text
// Skip paths isLegitimateMainTreePath classifies as runtime state (R9: one
// classification path shared with the every-boundary guard check). This
// covers .evolve/ non-deliverable state at any nesting depth (cycle-176) and
// bare directory entries from -uall (cycle-1 worktree dir abort).
// evolveDeliverablePrefixes are NOT skipped — they relocate into the worktree.
```

### `go/internal/core/leak_recovery.go:111` — above `if err := discardMainLeak(ctx, projectRoot, p); err != nil {`

```text
// go/evolve is the marketplace-tracked binary, re-committed ONLY by the
// release pipeline (releasepipeline.go) and reset to HEAD by the ship phase
// (ship/gitops.go). A mid-cycle rebuild leaked into the main tree must be
// discarded, never relocated into the worktree — relocating would commit
// binary drift (the cycle-153 hazard). Discard regardless of status code.
```

### `go/internal/core/leak_recovery.go:128` — above `if worktreeCleanForPath(ctx, worktree, p) {`

```text
// A non-Claude builder may edit an EXISTING tracked source file in the
// MAIN tree instead of its worktree (cycle-162: orchestrator.go). That is
// real builder work — preserve it by relocating the leaked content into the
// worktree, but ONLY when the worktree has not independently modified the
// same file: a divergent worktree edit is authoritative, and overlaying
// would clobber it, so discard the main leak in that case.
```

### `go/internal/core/leak_recovery.go:166` — above `args := append([]string{"add", "-f", "--"}, relocated...)`

```text
// Stage ONLY the relocated files (not `git add -A`, which would also stage
// unrelated worktree content and pollute the auditor's `git diff HEAD`),
// so the relocated work is visible to audit + the binding — the same
// visibility reason as normalizeWorktreeToBase. Use -f: a relocated path may
// be gitignored in the WORKTREE (a builder that edited .gitignore, or a leak
// that only main's status surfaced) — a plain `git add` exits 1 on an ignored
// path and would abort the whole batch (cycle-176 / issue #11). The path is a
// real leak we deliberately moved here for audit, so force-stage it.
```

### `go/internal/core/leak_recovery.go:266` — above `var buildArtifacts = map[string]bool{`

```text
// buildArtifacts are tracked build outputs a builder may rebuild into the main tree.
// go/evolve is the marketplace-tracked binary, re-committed ONLY by the release pipeline
// (releasepipeline.go) and reset to HEAD by the ship phase (ship/gitops.go); a mid-cycle
// rebuild leaked here must be DISCARDED, never relocated into the worktree (relocating
// would commit binary drift — cycle-153). go/bin/evolve is gitignored and normally never
// appears in `git status`, but is listed defensively.
```

### `go/internal/core/leak_recovery.go:312`

```text
// preserveOnVerdict reports whether a cycle that COMPLETED normally (no abort,
// err==nil) should keep its worktree for salvage based on its final verdict.
// A FAIL verdict (audit FAIL → retro → end) leaves the builder's work
// UNCOMMITTED in the worktree; the default exit-cleanup prune would discard
// it (ADR-0046 Layer 2 was built and lost twice this way, cycles 306/307 —
// inbox preserve-worktree-on-verdict-fail). A PASS/SHIPPED_VIA_BUILD cycle's
// work is already in main and a SKIPPED_UNKNOWN produced nothing, so those
// clean as before. Mirrors the abort-path and ship-failure preservation.
```

### `go/internal/core/ledger_runid_writers_guard_test.go:12` — above `var agentSubprocessWriters = map[string]string{`

```text
// ledger_runid_writers_guard_test.go — the DURABLE half of the cycle-1571 H1
// fix. Fixing the three unstamped writers closes today's hole; this guard is
// what stops the fourth from being added silently.
//
// PR #503 made run_id load-bearing: ship's binding lookup and the composition
// snapshot both refuse an auditor entry that does not carry THIS run's id. That
// turned every agent_subprocess writer into a participant in a ship-gate
// contract, but nothing enforced participation — the premise "every current
// recorder stamps run_id" was simply asserted, and was false for three of four
// writers.
//
// The scan is deliberately line-level: a line that ASSIGNS the kind (`Kind:` or
// `"kind":`) writes entries; a line that COMPARES it (`!=`, `==`) merely reads
// them, and readers owe nothing here.
//
// KNOWN LIMIT, stated because it already bit once: this guard proves the
// resolver is CALLED, never that its value reaches the emitted bytes. The first
// attempt at stamping cyclesimulator called it and still emitted nothing —
// jsonCompact drops any key outside its own allowlist — and this test was green
// throughout. Source scanning closes "a writer was added and nobody noticed";
// only a behavioural test over the real writer closes "the value is dropped on
// the way out". Every writer therefore also owes one: subagent/runid_stamp_test.go,
// cyclesimulator/runid_stamp_test.go, core/phase_bindings_fail_verdict_test.go.
```

### `go/internal/core/ledger_source_adversarial_cycle230_test.go:12` — above `func TestLedgerEntrySource_LargeSkipList_Amp(t *testing.T) {`

```text
// Cycle-230 test-amplification adversarial tests for task ledger-skip-source.
// Written from spec only (no implementation read) — anti-bias isolation.
//
// Coverage gaps addressed:
//   - Large skip list: ALL entries (not just first 2) must carry Source:"router"
//   - Non-"router" source value: round-trip must preserve custom sources without
//     hardcoding "router" in UnmarshalJSON
//   - Direct struct marshal: struct with Source set must produce "source" key in
//     JSON without going through the unmarshal→marshal path first
```

### `go/internal/core/ledger_source_test.go:12` — above `func TestLedgerEntrySource_FieldPresent(t *testing.T) {`

```text
// Cycle-230 task ledger-skip-source: phase_skipped ledger entries must carry a
// skip-source attribution (`Source string` on LedgerEntry, json tag
// "source,omitempty"; values psmas|router|content), and recordRoutingDecision
// must stamp Source:"router" on the phase_skipped entries it appends.
//
// These tests use reflection / JSON-level assertions deliberately so the core
// package still COMPILES at the RED baseline (no direct e.Source access) and
// each test fails on an assertion, not a build error — "fails for the right
// reason" per the TDD contract.
//
// DO NOT MODIFY (builder contract): add the field in ports.go (LedgerEntry +
// ledgerEntryWire + UnmarshalJSON) and stamp it in
// orchestrator.go:recordRoutingDecision.
```

### `go/internal/core/ledger_source_test.go:26` — above `func TestLedgerEntrySource_FieldPresent(t *testing.T) {`

```text
// TestLedgerEntrySource_FieldPresent: LedgerEntry must expose a Source string
// field tagged `json:"source,omitempty"` (omitempty keeps historical hash-chain
// bytes stable — absent on every pre-cycle-230 entry).
```

### `go/internal/core/legality_graph_test.go:3` — above `import (`

```text
// legality_graph_test.go — PA-DDK DDK-5 (ADR-0060 §1a). The legality graph
// (`allowed`) is now config-driven via config.legal_successors, and the
// load-time validator is the relocated trust anchor that gates it. These tests
// load the real registry via the kerneltest fixture and reference phases through
// structural accessors — never hardcoded names — so renaming a phase needs no
// test rewrite.
```

### `go/internal/core/legality_graph_test.go:97` — above `func TestOrchestrator_SafeConfigRunsNormally(t *testing.T) {`

```text
// TestOrchestrator_SafeConfigRunsNormally: the guard does not false-positive — an
// orchestrator built with the real reference config runs a full cycle.
//
// SpineFloor is dialed to shadow here because this test's fake runners write
// NO artifacts (the literal cycle-283 shape) and the R8.5-armed floor now
// correctly ABORTS that — which is the floor working, not the legality-graph
// validator false-positiving. The floor's own contract (enforce blocks /
// shadow proceeds / degraded fails open) is pinned by orchestrator_spinegate_test.go;
// this test pins ONLY that ValidateSafetyInvariants accepts the reference config.
```

### `go/internal/core/lost_landing_floor.go:3` — above `import (`

```text
// lost_landing_floor.go — a cycle whose landing was destroyed must not report PASS.
//
// Live incident (wave-20260822a-verify, 2026-08-22): two lanes raced one main.
// cycle-1536 won and landed; cycle-1535 rebased into a genuine non-derived
// conflict on a peer's eval file, routed to the debugger exactly as designed,
// and produced NO commit — then closed out with final_verdict PASS.
//
// Nothing at cycle close was asking the question. `finalizeOutcome` only
// reclassifies a SKIPPED verdict (cycle_outcome.go), so a PASS handed up by the
// audit floor survives untouched whether or not ship ever landed. The one
// existing no-ship notice keys on CycleOutcomeSkippedUnknown, which such a cycle
// never becomes.
//
// Why this is worth a floor rather than a log line: a lost landing that reports
// PASS makes a zero-ship wave read as a BUILDER-QUALITY problem, when it is
// really landing-queue contention. That is the first thing the zero-ship halt
// protocol tells an operator to rule out, and the misreading sends them at the
// agents instead of the queue. The loss was invisible here until someone diffed
// ship-error.json against ship-binding.json by hand, per cycle.
//
// The evidence is the cycle's OWN artifacts, deliberately NOT git HEAD movement:
// in fleet mode a sibling lane moves HEAD constantly, so a HEAD delta would have
// credited cycle-1535 with cycle-1536's commit — the precise confusion this
// exists to end.
//
// Detection is deliberately NARROW: ship ran and recorded an error, ship left no
// binding, and the cycle still claims a shipping verdict. A cycle that never
// reached ship has no landing to lose, and a cycle already reporting FAIL/WARN is
// telling the truth.
```

### `go/internal/core/lost_landing_floor.go:57` — above `if _, err := os.Stat(filepath.Join(workspace, "ship-binding.json")); err == nil {`

```text
// A binding is the ship phase's own proof of delivery. Present ⇒ landed,
// whatever transient errors it survived on the way (cycle-1536 recorded a
// GIT_FLEET_REBASE_NEEDED and landed anyway — flagging that would make every
// contended wave a wall of false alarms).
```

### `go/internal/core/lost_landing_floor_test.go:3` — above `import (`

```text
// lost_landing_floor_test.go — a cycle that lost its landing must not report PASS.
//
// Live incident (wave-20260822a-verify, 2026-08-22). Two lanes raced one main:
//
//	cycle-1536  ship-error GIT_FLEET_REBASE_NEEDED  +  ship-binding commit=adcbddb2  → LANDED
//	cycle-1535  ship-error GIT_FLEET_REBASE_NEEDED  +  no ship-binding at all        → LOST
//
// The recovery machinery behaved CORRECTLY for both: 1535's rebase hit a genuine
// non-derived conflict on .evolve/evals/pipeline-replay-contract-boundary.md and
// routed to the debugger, exactly as designed. What went wrong is downstream of
// that — cycle-1535 closed out with final_verdict PASS. Its audit passed, and
// nothing at cycle close ever asks whether the cycle's own ship actually landed:
// finalizeOutcome only reclassifies a SKIPPED verdict, so a PASS from audit
// stands whether or not ship succeeded.
//
// The cost is not cosmetic. A lost landing that reports PASS makes a zero-ship
// wave read as a builder-quality problem when it is really a landing race, which
// is the first thing the zero-ship halt protocol tells an operator to rule out.
//
// The fixtures are the REAL artifacts from both cycles, because the whole point
// is that the two are distinguishable ONLY by the presence of ship-binding.json.
```

### `go/internal/core/lost_landing_floor_test.go:55` — above `func TestDetectLostLanding_RealCycle1535IsNotAPass(t *testing.T) {`

```text
// THE headline regression: cycle-1535's real artifacts must not read as a PASS.
```

### `go/internal/core/mandatory_anchors_test.go:3` — above `import (`

```text
// mandatory_anchors_test.go — PA-BIG S6 (ADR-0058): the spine-anchor ORDER is
// derived from config (cfg.Order ∩ cfg.Mandatory ∩ artifact-anchors), not a
// package literal. The artifact map (which phases gate, and on what) stays
// hardcoded. Byte-identity matters here: SpineSatisfiedUpTo is the non-gameable
// ship floor (invariant #2).
```

### `go/internal/core/materialization_correction_delivery_test.go:3` — above `import (`

```text
// materialization_correction_delivery_test.go — cycle-1554 RED contract for
// `scout-eval-materialization-correction-delivery` (inbox
// pipeline-defect-pipeline-blocker, P0).
//
// evalgate_escalation_test.go (package core) already proves the CLI-escalation
// ladder using a hand-authored evalGateProbe reviewer that FAKES the gate's
// Approve/Reason/Remediation shape. That test never runs the REAL
// evalgate.materializationGate — it never stats a real
// .evolve/evals/<slug>.md file, never reads a real scout-report.md, and never
// verifies that a file the agent creates in response to the correction
// directive actually clears the real gate on re-review. Nothing in either the
// core or evalgate suite drives the production seam end-to-end: real
// scout-report.md (missing eval) -> real materializationGate rejection with
// its real remediation text -> correction re-dispatch -> agent creates the
// eval at the EXACT workspace path the remediation named -> real
// materializationGate re-review APPROVES. This file closes that gap (cycles
// 1540/1545: the remediation text was byte-perfect and the correction still
// burned its full budget without landing — "produced a remediation string
// without a proven consumer" per the scout hypothesis).
//
// External test package (core_test): wiring the REAL evalgate.NewReviewer
// (which itself imports core) alongside core would be an import cycle from an
// internal test file — see internal/core/evalgate_escalation_test.go's header
// for why that file stays a fake instead. test/fixtures supplies the
// canonical FakeStorage/FakeLedger/BuildRunners so this file needs no
// unexported core test helpers.
```

### `go/internal/core/materialization_correction_delivery_test.go:98` — above `body := "# Eval " + r.slug + "\n\n- [code] 'go test ./internal/widget/...'\n\n'''bash\ngo test ./internal/widget/...\n''…`

```text
// A complying scout writes the [code] grader the remediation now requires
// (cycle 1679), not only the legacy bash fence.
```

### `go/internal/core/mergerung2.go:1` — above `package core`

```text
// mergerung2.go — the merge ladder's RUNG 2: scoped merge review
// (cycle-941, fleet-scoped todo merge-rung2-scoped-merge-review;
// knowledge-base/research/merge-concurrency-2026, MergeBERT lineage).
//
// RUNG 0 (composition_carryforward.go) carries an audit verdict forward when a
// clean fleet rebase leaves the composed diff's patch-id UNCHANGED. When the
// patch-id DID change — real overlapping edits, not a trivial rebase — today's
// only fallback is RUNG 3, a full re-audit. RUNG 2 is the missing middle: it
// reviews ONLY the hunks that actually intersect between the audited change and
// the composed change and, if that overlap is compatible, composes directly;
// only genuine entanglement escalates to the full re-audit.
//
// The reviewer is an injected closure (Option seam, mirroring RUNG 0) so this
// pure core stays adapter- and LLM-agnostic. An LLM-assisted resolution the
// reviewer may return is SUGGESTION-GRADE: it is trusted only after re-entering
// RUNG 0 patch-id verification (ResolutionMatchesAudited), never on the
// reviewer's word.
```

### `go/internal/core/mergerung2_consolidation_test.go:1` — above `package core`

```text
// mergerung2_consolidation_test.go — RED contract for cycle-946's
// reconcile-rung2-duplicate-implementations task (fleet-scoped todo
// merge-rung2-scoped-merge-review, campaign merge-efficiency-2026-07).
//
// go/internal/core carries TWO unreconciled rung-2 implementations of the
// same concept ("which hunks intersect between an audited diff and a
// composed diff"):
//
//   - mergerung2.go's intersectingHunks/RunScopedMergeReview (cycle-941) —
//     the PRODUCTION-WIRED path: compares each hunk's OLD-side (pre-image)
//     line range and is the one recoverFromShipError actually dispatches to
//     (scopedMergeCarryForward, composition_carryforward.go).
//   - composition_scoped_review.go's IntersectingHunks (cycle-942) — built
//     and unit/ACS-tested (go/acs/cycle942/predicates_test.go) but never
//     wired into any production call site; it compares each hunk's NEW-side
//     (post-image) line range instead.
//
// Because the two compare DIFFERENT coordinate spaces, they can disagree on
// whether the same diff pair overlaps at all — a hunk pair whose old-side
// ranges intersect (dispatched for review on the real, wired path) can have
// disjoint new-side ranges (silently skipped as "no conflict" on the unwired
// path). This test pins that divergence as a failing (RED) contract: the
// Builder's consolidation must make both call sites agree on ONE overlap
// semantic (the production-wired old-side comparison), not merely delete
// one file and hope the ACS-pinned public API (IntersectingHunks,
// ScopedReviewVerdict, ScopedReviewMethod, ReverifyResolution — see
// go/acs/cycle942/predicates_test.go) keeps behaving as before.
//
// RED today: composition_scoped_review.IntersectingHunks reports the fixture
// pair as NON-overlapping while mergerung2's canonical intersectingHunks
// reports it as overlapping (see the Errorf message for the exact
// assertion). This is a genuine behavioral divergence, not a source-text
// grep — both implementations are invoked and asserted on their real output.
```

### `go/internal/core/mergerung2_test.go:1` — above `package core`

```text
// mergerung2_test.go — RED contract for the merge ladder RUNG 2 scoped
// merge review (cycle-941, fleet-scoped todo merge-rung2-scoped-merge-review;
// knowledge-base/research/merge-concurrency-2026, MergeBERT lineage).
//
// RUNG 0 (composition_carryforward.go) carries an audit verdict forward when a
// clean fleet rebase leaves the composed diff's patch-id unchanged. When the
// patch-id DID change (real overlapping edits, not a trivial rebase), today's
// only fallback is RUNG 3 — a full re-audit. RUNG 2 is the missing middle: it
// reviews ONLY the hunks that actually intersect between the audited change and
// the composed change and, if that overlap is compatible, composes directly;
// only genuine entanglement escalates to the full re-audit.
//
// This file is the pure-core RED contract (Task A merge-rung2-scoped-review-core
// + the Task B re-entry invariant and orchestrator wiring observability). It
// references symbols the Builder must create in go/internal/core/mergerung2.go
// and the Method field on CompositionVerdictInput; at authoring every reference
// is undefined, so the package fails to compile — the correct RED. Builder
// contract: implement the documented API to turn these GREEN; DO NOT modify
// this file.
//
// Documented API the Builder must implement (go/internal/core/mergerung2.go):
//
//	type ScopedMergeDisposition string
//	const ScopedMergeCompatible ScopedMergeDisposition = "compatible"
//	const ScopedMergeEntangled  ScopedMergeDisposition = "entangled"
//	type MergeHunk struct { File, Header, Body string }
//	type ScopedMergeReviewOutcome struct {
//	    Disposition    ScopedMergeDisposition
//	    ResolutionDiff []byte // optional LLM-assisted resolution (suggestion-grade)
//	}
//	type ScopedMergeReviewer func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome
//	type ScopedMergeInput struct { AuditedDiff, ComposedDiff []byte; AuditedSummary, ComposedSummary string }
//	type ScopedMergeResult struct {
//	    Disposition     ScopedMergeDisposition
//	    DispatchedHunks []MergeHunk
//	    Dispatched      bool
//	}
//	func RunScopedMergeReview(in ScopedMergeInput, review ScopedMergeReviewer) (ScopedMergeResult, error)
//	func ResolutionMatchesAudited(auditedPatchID string, resolutionDiff []byte) (bool, error)
//	// plus: CompositionVerdictInput.Method string (core mirror), and on
//	// *Orchestrator: WithScopedMergeReviewer(fn) Option + ScopedMergeReviewWired() bool.
//
// Contract details Builder must honor:
//   - RunScopedMergeReview parses both diffs into hunks and computes the hunks
//     whose file+line ranges intersect. A malformed diff (a `@@` hunk header
//     that does not parse as `@@ -old,n +new,m @@`) is fail-closed: it returns a
//     non-nil error, does NOT invoke the reviewer, and does NOT report
//     compatible.
//   - Empty intersection: the reviewer is NOT invoked, Dispatched is false,
//     Disposition is compatible (nothing entangled — no wasted review).
//   - Non-empty intersection: exactly the intersecting hunks (never the
//     audited-only or composed-only hunks) are dispatched; the reviewer's
//     Disposition is carried through verbatim.
//   - ResolutionMatchesAudited recomputes the resolution diff's OWN patch-id
//     (rung-0 verification) and returns true only if it equals auditedPatchID —
//     an LLM-assisted resolution is trusted by patch-id, never on the reviewer's
//     word (MergeBERT lineage). A malformed resolution fails closed (error).
```

### `go/internal/core/mint_catalog_publisher_test.go:1` — above `package core`

```text
// mint_catalog_publisher_test.go — cycle-1429 TDD contract, task
// `mint-catalog-live-refresh` (inbox pipeline-defect-infra-systemic, P0).
//
// THE DEFECT (cycle-1424 root cause, still open after #428/#429).
// The composition root binds the bridge's deliverable-contract resolver ONCE, at
// cycle start, over a SNAPSHOT of the phase catalog:
//
//	cmd/evolve/cmd_cycle.go:482
//	  br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))
//
// `catalog.Get` is a method value bound to that catalog VALUE, whose `byName`
// map is the pre-mint map. When the advisor mints a phase mid-cycle,
// registerMintedPhases (routing_dispatch.go:158-162) does
// `merged, _ := o.catalog.Merge(...); o.catalog = merged` — and Catalog.Merge
// returns a NEW Catalog over a NEW map (phasespec/discover.go:99-104). The
// orchestrator's own catalog therefore knows the minted phase, while the
// resolver the bridge injects prompts through is still reading the stale,
// pre-mint map. CatalogResolver.Resolve misses for the freshly-minted phase for
// the REST OF THAT CYCLE, so every contract-dependent consumer falls back to the
// unresolved-agent path. That is the cycle-1424 halt: `defect-disposition-ledger`
// was dispatched, the engine polled 600s for an artifact, and the phase never
// resolved a contract. #429 made that fallback SAFE (the footer now discloses the
// polled path); this test closes the miss itself so the fallback stops being
// load-bearing.
//
// THE CONTRACT (what Builder must implement).
//  1. `core.WithCatalogPublisher(fn func(phasespec.Catalog)) Option` — injects a
//     sink the orchestrator notifies whenever its live catalog CHANGES.
//  2. `registerMintedPhases` calls that sink AFTER the `o.catalog = merged`
//     splice, with the merged catalog, for every successfully-minted phase.
//  3. `(*Orchestrator).CatalogPublisherWired() bool` — the composition-root
//     reachability predicate (same idiom as CompositionFastPathWired), asserted
//     from cmd/evolve by TestWireOrchestrator_CatalogPublisherWired.
//
// RED today: WithCatalogPublisher / CatalogPublisherWired do not exist, so this
// file does not compile — the correct RED for a missing seam.
//
// Reachability probe (cycle-644 obligation): internal/core ALREADY imports both
// phasecontract (build_removal_check.go:33) and phasespec, so the pins below add
// no new import edge and cannot introduce an import cycle.
```

### `go/internal/core/mint_catalog_publisher_test.go:70` — above `func TestRegisterMintedPhases_PublishesCatalogToLiveResolver(t *testing.T) {`

```text
// TestRegisterMintedPhases_PublishesCatalogToLiveResolver is the cycle-1429
// acceptance for mint-catalog-live-refresh, reproducing the cycle-1424 shape
// EXACTLY: a resolver bound over the pre-mint catalog snapshot misses the minted
// phase; after the mint publishes, a resolver bound over the published catalog
// hits and yields the spec-derived contract.
//
// The two resolvers are the point. `stale` models today's cmd_cycle.go:482
// binding (a method value over the pre-mint catalog value) and must STILL miss —
// proving the publisher is what carries the new catalog across, not some
// accidental aliasing. `live` models the post-fix binding and must HIT.
```

### `go/internal/core/mint_catalog_publisher_test.go:95` — above `const minted = "defect-disposition-ledger"`

```text
// the cycle-1424 phase, by name
```

### `go/internal/core/mint_exemption.go:24` — above `func isActiveMintPhasePath(mints map[string]bool, p string) bool {`

```text
// isActiveMintPhasePath reports whether a main-tree write is a VERIFIED
// advisor mint's phase config — the third guard-only classifier beside
// isLegitimateMainTreePath and isScoutEvalMaterialization. In fleet mode both
// lanes diff the SAME shared tree with per-lane baselines, so lane A's mint of
// .evolve/phases/<name>/phase.json lands in lane B's post-phase diff and was
// charged to lane B as a deliverable leak (cycle-967 false-abort). The
// registrar records every minted name in the shared mintregistry BEFORE
// persisting its files; the guard exempts a leaked path IFF it is one of the
// EXACTLY TWO paths a mint writes — .evolve/phases/<name> (bare dir entry) or
// .evolve/phases/<name>/phase.json — for a registered, TTL-fresh, CONTENT-
// VERIFIED name (mints must already be filtered through verifiedActiveMints).
// A novel phase-config name, a companion payload under a registered name, and
// an unclamped spec all still fire the guard — the deliverable-leak pin
// (TestIsScoutEvalMaterialization's {scout, .evolve/phases/x/phase.json,
// false}) keeps governing smuggles.
```

### `go/internal/core/model_routing_envelope_wiring_test.go:3` — above `import (`

```text
// Cycle-976 RED wiring proofs for the model-tier-envelope guard.
//
// These are INTEGRATION tests through RunCycle (not the router unit boundary):
// they drive the REAL Orchestrator.profileForModelRouting seam so they fail
// against the current permanent nil-stub (cyclerun.go:711-713) and pass once
// Builder wires a real per-phase profile lookup. The router-level guard
// (router.ClampPlanModelRouting) is already unit-covered and correct; the defect
// under test is purely that the production DI seam feeding it real profiles
// returns nil for every phase, silently disabling floor AND ceiling clamps
// (including the documented "universal floor") for 100% of live cycles.
//
// Why RED now: profileForModelRouting returns nil → prof==nil in the guard →
// the whole envelope branch (model_routing_clamp.go:88-109) is skipped → the
// advisor-proposed out-of-envelope tier reaches dispatch verbatim. The
// assertions below want the CLAMPED tier, so they fail on the assertion (right
// reason), not on a compile error — every helper used here already exists in
// the core test package.
```

### `go/internal/core/model_routing_test.go:16` — above `func modelRoutingCfg(mr config.ModelRouting) config.RoutingConfig {`

```text
// modelRoutingCfg builds a DynamicLLM cfg at StageAdvisory with the given
// model-routing axis — the cycle-440 MR4 wiring gate (o.cfg.Stage >=
// StageAdvisory && o.cfg.Mode == ModeDynamicLLM && o.planner != nil) is what
// makes the whole-cycle plan (and, once wired, ClampPlanModelRouting) run at
// all.
```

### `go/internal/core/model_routing_test.go:211` — above `func TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault(t *testing.T) {`

```text
// TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault (I8): the two new
// wire fields are omitempty — a zero-value PhaseRequest (the entire
// pre-cycle-440 fleet) marshals with neither key present, so an unaware
// consumer (e.g. the subprocess phaseproto override path) sees no new shape.
```

### `go/internal/core/observer.go:3` — above `import "context"`

```text
// observer.go — cycle-122 Fix 3: phase-observer auto-spawn from the
// orchestrator's RunCycle (ADR-0030).
//
// Background: pre-v12, the bash dispatcher unconditionally background-
// spawned phase-observer.sh per phase (see ADR-0030 for the silent-
// regression history). The Go port preserved the observer code as a
// manual `evolve phase-observer` subcommand but never re-added the
// orchestrator-side auto-spawn. Cycle-122's tdd-phase hung 10 min
// before the bridge artifact-timeout fired — the observer would have
// emitted a stall_no_output INCIDENT well before that, giving operators
// forensic visibility into "the phase is stuck right now" rather than
// "the phase aborted 10 min ago."
//
// The interface here is intentionally tiny: presence is the contract,
// not shape. The noopObserver default is byte-identical to the pre-fix
// orchestrator behavior, so ObserverPolicy.Autospawn=false + an absent
// WithObserver option together reproduce the pre-ADR-0030 cycle exactly.
```

### `go/internal/core/observer.go:47` — above `type noopObserver struct{}`

```text
// noopObserver is the orchestrator's default when WithObserver was not
// used: Start does nothing, cancel does nothing — byte-identical to
// the pre-ADR-0030 cycle. Kept here (not a separate file) so the
// contract "nil/absent observer implies this exact behavior" lives
// next to the interface that defines it.
```

### `go/internal/core/observer_test.go:26` — above `func TestOrchestrator_NoopObserver_IsByteIdentical(t *testing.T) {`

```text
// TestOrchestrator_NoopObserver_IsByteIdentical is the ADR-0030
// pre-opt-in contract: when the operator doesn't pass WithObserver,
// the orchestrator runs every phase to completion exactly like the
// pre-fix cycle — no observer-related behavior observable.
```

### `go/internal/core/observer_test.go:127` — above `func TestNoopObserver_StartReturnsNonNilCancel(t *testing.T) {`

```text
// TestNoopObserver_StartReturnsNonNilCancel pins the noopObserver
// contract directly: even the noop must return a callable cancel
// (orchestrator code calls cancel() unconditionally per ADR-0030).
```

### `go/internal/core/optional_skip_details_test.go:3` — above `import (`

```text
// optional_skip_details_test.go — the skip's forensic surface must split by
// error class: a missing persona (cycle-1551, zero retries, no infra event)
// filed under "optional_infra_skip" at exit 0 would merge two failure classes
// the ledger has been burned by merging before.
```

### `go/internal/core/orchestrator.go:38` — above `var QuotaBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error`

```text
// QuotaBoundaryCheckpointer is a package-level hook to write a quota-likely
// checkpoint block when the dispatch seam detects all-families exit=85
// exhaustion (cycle-656). Set by the checkpoint package to avoid circular
// imports; it preserves completed phases + the worktree so `evolve loop
// --resume` re-enters at the deferred phase after the quota resets.
```

### `go/internal/core/orchestrator.go:55` — above `func optionalSkipDetails(p Phase, err error) (kind, msg string, diags []Diagnostic) {`

```text
// optionalInfraSkip reports whether a phase whose retries exhausted may
// record SKIPPED with a warning and advance instead of aborting the cycle (the Workstream-D
// intent documented on ErrArtifactTimeout; cycle-283). Four conditions, all
// required: the error is skippable-shaped per IsOptionalSkippableError (infra
// teardown — artifact timeout / transient bridge — OR a missing persona doc,
// cycle-1551; never integrity or logic failures), the phase is NOT
// configured-mandatory,
// the phase is catalog-Optional, and the phase sits outside the resolved ship
// floor — so the skip can never weaken `ship ⇒ build ∧ audit ∧ tdd`. The
// mandatory guard is generic and config-driven (the orchestrator reads
// cfg.Mandatory, not a hardcoded phase name): it subsumes the former ship
// special-case — ship is a mandatory anchor — while protecting any mandatory
// phase mis-marked Optional (the floor loop alone would miss ship, which is
// not in the floor set). Phase-agnostic flow per ADR-0035/0038.
// optionalSkipDetails names the ledger kind, operator message, and structured
// diagnostic for an ADMITTED optional-phase skip, split by error class so
// forensics never files a config defect under an infra key: a missing persona
// (cycle-1551) had zero retries and no infra event — calling it
// "optional_infra_skip" at exit 0 would merge two failure classes the ledger
// has already paid for merging once. The diagnostic rides the synthesized WARN
// response so the cause reaches audit/retro, not only stderr + ledger.
```

### `go/internal/core/orchestrator.go:105` — above `func (o *Orchestrator) postShipObserverSkip(p Phase, shipped bool) bool {`

```text
// postShipObserverSkip classifies a POST-ship best-effort observer failure as
// non-fatal (cycle-574, inbox memo-phase-tier-envelope). A RoleControl observer
// phase that runs after a healthy ship (memo, post-ship-monitor) must never turn
// an already-shipped cycle abnormal: its failure degrades to a WARN diagnostic
// and the cycle keeps its shipped/PASS outcome. This is a sibling to
// optionalInfraSkip — same floor/mandatory guards, but keyed on ship-having-
// -landed rather than an infra-shaped error, so it also covers a memo policy or
// logic error (the exact tier-envelope shape that reddened healthy cycles).
// True iff ALL hold: (1) ship already recorded PASS this cycle (shipped) — a
// failure BEFORE ship is never swallowed; (2) p is a catalog-Optional RoleControl
// observer and NOT ship itself; (3) p is not configured-mandatory and sits
// outside the resolved ship floor — the skip can never weaken the integrity floor.
```

### `go/internal/core/orchestrator.go:143` — above `func (o *Orchestrator) specFor(p Phase) (phasespec.PhaseSpec, bool) {`

```text
// specFor resolves a phase's descriptor, canonicalizing the name first
// (PhaseRetro→"retrospective") so the lookup cannot silently miss on the
// core↔router skew. The registry is the SSOT and wins; on a registry miss it
// falls to the builtinControlSpec seam (ADR-0058 §5) for control phases that have
// no registry home (debugger). A total miss yields (_, false), which keeps Next
// on its byte-identical literal path. It is the StateMachine's window onto
// config-driven transition resolution (ADR-0058).
```

### `go/internal/core/orchestrator.go:157` — above `func (o *Orchestrator) phaseArchetype(phase string) string {`

```text
// phaseArchetype resolves a phase's composition class (plan/build/evaluate/
// control) from the existing phasespec taxonomy — the registry spec's explicit
// archetype when present, else name inference. The single source the latency
// roll-up buckets by; see recordPhaseOutcome (ADR-0044 C1 chokepoint).
```

### `go/internal/core/orchestrator.go:168` — above `func builtinControlSpec(p Phase) (phasespec.PhaseSpec, bool) {`

```text
// builtinControlSpec is the control-phase metadata seam (ADR-0058 §5): the one
// place Go data describes a phase, justified because the control phases
// (debugger; start/end) are registered as runners in cmd_cycle.go and have no
// registry `phases[]` home. It supplies ONLY branch metadata — debugger's
// signal-driven successor — never an OnPass/OnFail edge, so Next stays literal
// for control phases. A registry entry of the same name overrides it (specFor
// precedence). Returns (_, false) for any phase the seam does not describe.
```

### `go/internal/core/orchestrator.go:193` — above `func (o *Orchestrator) successorStrategy(p Phase) string {`

```text
// successorStrategy resolves how phase p's successor is chosen — the
// branching_strategy declared on its descriptor (ADR-0058). On a catalog miss,
// or an entry that omits the field, it degrades to the literal phase-identity
// default (literalSuccessorStrategy), keeping the flow byte-identical when the
// catalog is unset. "Config selects, code constrains": this only routes the
// orchestrator among branches the state machine already deems legal — it never
// invents an edge.
```

### `go/internal/core/orchestrator.go:207` — above `func literalSuccessorStrategy(p Phase) string {`

```text
// literalSuccessorStrategy is the unconfigured backstop: the successor-selection
// strategy each phase used before ADR-0058 made it config-driven. Two phases are
// not verdict-driven: retrospective is history-driven (the failure-adapter
// consults cycle history) and debugger is signal-driven (its decision signal
// picks the successor). Every other phase is verdict-driven (the empty default).
```

### `go/internal/core/orchestrator.go:280` — above `outcome *outcome.Recorder`

```text
// unit 01 (ADR-0103): the C1 recording chokepoint
```

### `go/internal/core/orchestrator.go:281` — above `diag    *failurediag.Writer`

```text
// unit 02 (ADR-0103): the failure-diag sidecar + delivery classifier
```

### `go/internal/core/orchestrator.go:282` — above `carry   *carryover.Lifecycle`

```text
// unit 03 (ADR-0103): the carryover-todo lifecycle
```

### `go/internal/core/orchestrator.go:283` — above `learn   *failurelearning.Engine`

```text
// unit 03b (ADR-0103): the failure-learning engine (the recorder, the floor, the recurrence closure)
```

### `go/internal/core/orchestrator.go:357` — above `worktree WorktreeProvisioner`

```text
// worktree provisions/cleans the per-cycle source worktree (ADR-0027).
// Default gitWorktree (real git); injected in tests via
// WithWorktreeProvisioner so RunCycle runs without touching real git.
```

### `go/internal/core/orchestrator.go:371` — above `planner router.Planner`

```text
// planner produces the upfront whole-cycle plan (ADR-0024 §2). Optional:
// nil ⇒ no advisor plan ⇒ the kernel floor falls back to the configurable
// never-skip spine (fail-safe to static). Consulted once at cycle start,
// only at Stage>=Advisory; its output is clamped to the integrity floor
// before being threaded into every routing decision.
```

### `go/internal/core/orchestrator.go:396` — above `catalogPublisher func(phasespec.Catalog)`

```text
// catalogPublisher is notified with the LIVE catalog every time a mid-cycle
// mint changes it, so consumers that bound a resolver over the cycle-START
// catalog value (the bridge's deliverable-contract resolver, cmd_cycle.go)
// can re-bind. Without it the orchestrator knows the minted phase while the
// resolver keeps reading the pre-mint map for the rest of the cycle — the
// cycle-1424 naked-dispatch halt. Nil (default) ⇒ no-op, byte-identical
// legacy behavior. Set via WithCatalogPublisher.
```

### `go/internal/core/orchestrator.go:426` — above `continuationFor func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation`

```text
// continuationFor resolves the ADR-0076 slice C continuation binding for a
// cycle's scope — claimed (the inbox mover's processing claims) or, for a
// lane whose scope came from the wave planner, the pinned lane-scope todo
// ids handed in as scopeIDs. Nil (default) = continuations never adopt —
// byte-identical provisioning.
```

### `go/internal/core/orchestrator.go:437` — above `acsPredicates predicateLister`

```text
// acsPredicates lists the cycle's ACS predicate names for the Task Contract
// (ADR-0098); nil ⇒ listACSPredicates (real `go test -list`). Injectable so
// the wiring proof exercises the inventory without a Go toolchain.
```

### `go/internal/core/orchestrator.go:446` — above `failureCountFor func(id string) int`

```text
// failureCountFor reads an inbox item's durable failure_count (ADR-0076 D
// retry tier escalation). Wired at the composition root from
// inboxmover.ReadFailureCount; nil = escalation disabled (safe no-op).
```

### `go/internal/core/orchestrator.go:451` — above `failurePolicy policy.SystemFailurePolicy`

```text
// failurePolicy is the resolved system-failure DECISION policy (ADR-0072):
// the category→action map + Go-enforced floor. Resolved once from
// policy.json at the composition root; absent ⇒ compiled defaults.
```

### `go/internal/core/orchestrator.go:480` — above `observer Observer`

```text
// observer is the per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
// Start is called once before each runner.Run; the returned cancel runs
// once after. Nil ⇒ noopObserver default ⇒ byte-identical to the pre-
// ADR-0030 cycle. Set via WithObserver; cmd_cycle.go wires the real
// implementation when ObserverPolicy.Autospawn is enabled.
```

### `go/internal/core/orchestrator.go:487` — above `failureAdviser FailureAdviser`

```text
// failureAdviser is the ADR-0044 C3 LLM escalation tail consulted by
// adviseOnUnclassifiedFailure (failure_hook.go) — only at
// cfg.PhaseRecovery == StageEnforce, only for unclassified
// artifact-timeout panes. Nil (default) ⇒ hook inert. Set via
// WithFailureAdviser.
```

### `go/internal/core/orchestrator.go:494` — above `contractVerifier ContractVerifier`

```text
// contractVerifier is the ADR-0045 I2 breaker-neutral deliverable
// re-check used by the correction ladder's salvage rung. Nil (default)
// ⇒ the salvage rung gets zero budget and the ladder degrades to
// redispatch-only — exactly the pre-I2 correction loop.
```

### `go/internal/core/orchestrator.go:505` — above `signals       *signalcenter.Center`

```text
// signals is the ADR-0101 Signal Center the orchestrator listens to; nil is
// the Null Object (tests and the two pinned secondary roots). signalSummary
// is the current cycle's view, guarded by signalMu — the first production
// mutex in core: the Center holds no lock while delivering, observeSignal
// never emits, and no orchestrator path holds signalMu across an Emit.
```

### `go/internal/core/orchestrator.go:542` — above `func WithPlanner(p router.Planner) Option {`

```text
// WithPlanner injects the whole-cycle phase planner (ADR-0024 §2 hybrid
// cadence). A nil planner is ignored so the no-plan default stands; the
// orchestrator consults it only at Stage>=Advisory and always clamps its
// output to the integrity floor — "model proposes, kernel disposes".
```

### `go/internal/core/orchestrator.go:619` — above `func WithContinuationResolver(fn func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation) Opti…`

```text
// WithWorkflowConfig injects the resolved workflow policy.
// WithContinuationResolver injects the scope continuation lookup (ADR-0076
// slice C): claimed scopes first, then the cycle's pinned lane-scope todo ids.
// Nil is ignored — adoption stays off.
```

### `go/internal/core/orchestrator.go:631` — above `func (o *Orchestrator) ScopePathProbe(projectRoot, taskID string) (string, bool) {`

```text
// WithScopePathResolver injects the live-record path lookup for scoped task
// ids, so a lane's phases receive the ONE correct file instead of a bare name.
//
// Why a name is not enough (cycle-1548, soak-20260823a): auto-minted ids are
// deliberately stable per category — the dedup identity — so inbox/consumed/
// accumulates same-id namesakes forever (17 records for one id at the
// incident). An agent handed only the name name-searches the tree and finds
// whichever namesake matches first; every phase of cycle-1548 worked a record
// from a halt cured two weeks earlier. Nil is ignored — disclosure stays off.
// ScopePathProbe reports whether a scope-path resolver is wired and, when it
// is, what it resolves taskID to. A WIRING probe, not a workflow API: the
// composition root's registration of the resolver is exactly the layer unit
// tests of the resolver function cannot see (this week's nine NOT-WIRED
// mutation survivors are all this shape), and Orchestrator's fields are
// unexported by design — this is the narrow window that lets cmd/evolve pin
// its own wiring without widening anything else.
```

### `go/internal/core/orchestrator.go:673` — above `func WithFailureCountReader(fn func(id string) int) Option {`

```text
// WithFailureCountReader injects the item failure-count read seam (ADR-0076
// D). Nil is ignored, preserving any prior reader (the WithKB idiom).
```

### `go/internal/core/orchestrator.go:683` — above `func WithFailurePolicy(fp policy.SystemFailurePolicy) Option {`

```text
// WithFailurePolicy injects the resolved system-failure decision policy
// (ADR-0072). The zero-option default is the compiled DefaultSystemFailurePolicy.
```

### `go/internal/core/orchestrator.go:717` — above `func WithWorktreeBase(base string) Option {`

```text
// WithWorktreeBase injects the operator worktree-base override, resolved once
// from policy.json (worktree.base) at the composition root. Empty ⇒ no override
// (the gitWorktree default <root>/.evolve/worktrees stands). Replaces the former
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064). Mutually exclusive
// with WithWorktreeProvisioner (both set o.worktree); production uses only this
// one, tests use only the fake — never both.
```

### `go/internal/core/orchestrator.go:731` — above `func WithObserver(o Observer) Option {`

```text
// WithObserver injects a per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
// The orchestrator calls observer.Start(...) before each phase's runner.Run
// and the returned cancel after — running a background watcher that emits
// stall_no_output events to the workspace when the subagent's stdout-log
// stops growing. A nil observer (default) keeps the noopObserver default,
// which is byte-identical to the pre-ADR-0030 cycle.
//
// cmd_cycle.go wires the real implementation via
// observer.NewCoreAdapter when ObserverPolicy.Autospawn is enabled.
```

### `go/internal/core/orchestrator.go:764` — above `func WithModelCatalogLookup(fn func(cli, tier string) (string, bool)) Option {`

```text
// WithModelCatalogLookup injects the model resolvability check (cycle-440
// MR4a) consulted by router.ClampPlanModelRouting: (cli,tier)→(model,ok).
// Nil (default) skips the resolvability gate — the plan's guardrail validation
// (allowed_clis/model_tier_envelope) still applies.
//
// The composition root wires the manifest-backed resolver (cmd/evolve/
// model_tier_resolver.go, which documents why not Catalog.Lookup) so core
// stays a leaf and never imports bridge or modelcatalog (dependency
// inversion, mirroring catalogRefresh above). ModelCatalogLookupWired
// (failure_hook.go) lets that wiring be proven in a real test.
```

### `go/internal/core/orchestrator.go:802` — above `func WithContractVerifier(v ContractVerifier) Option {`

```text
// WithContractVerifier injects the breaker-neutral deliverable re-check the
// ADR-0045 I2 salvage rung verifies relocations with. Nil is ignored, leaving
// the redispatch-only ladder (byte-identical to the pre-I2 correction loop).
// cmd_cycle.go wires deliverable.NewVerifierWithCatalog beside the reviewer.
```

### `go/internal/core/orchestrator.go:832` — above `func (o *Orchestrator) HasRunner(p Phase) bool {`

```text
// NewOrchestrator wires the orchestrator with its dependencies. Routing stays
// off unless a WithRouting option supplies an enabled-stage config.
// HasRunner reports whether a PhaseRunner is registered for p. It is the
// composition-root's read seam: a phase the router can nominate but that has
// no runner is silently skipped by cyclerun_dispatch's missing-runner escape
// hatch (the cycle-563 memo-dispatch bug), so tests assert on this to prove the
// routing→dispatch handoff is actually wired, not just that Route() names it.
```

### `go/internal/core/orchestrator.go:863` — above `observer:                   noopObserver{},`

```text
// cycle-122 Fix 3 / ADR-0030: byte-identical default until WithObserver is used
```

### `go/internal/core/orchestrator.go:871` — above `o.outcome = o.wiredRecorder()`

```text
// Unit 01 (ADR-0103): the recorder reads the clock and the catalog LIVE (tests
// swap o.now after construction; mints change the catalog mid-cycle) and
// raises the orchestrator's phase.outcome through emitPhaseOutcome.
```

### `go/internal/core/orchestrator.go:878` — above `o.sm.WithCatalog(o.specFor).WithSpine(spinePhasesFrom(o.cfg.SpineOrder)).`

```text
// ADR-0058: hand the state machine its config-driven verdict-branch
// resolution now that the catalog (hence specFor) is settled by options.
// Without a catalog, specFor misses and Next stays on the literal table
// (byte-identical). PA-DDK DDK-3: the linear spine is now config-declared
// (cfg.SpineOrder); an empty order leaves the SM on the canonical literal.
```

### `go/internal/core/orchestrator.go:885` — above `o.safetyViolations = ValidateSafetyInvariants(o.sm, o.cfg, o.catalog)`

```text
// PA-DDK DDK-5 (ADR-0060 §1a): with the legality graph + gates + verdict
// branches all config-driven, the floor's only structural guarantee is the
// phase-agnostic validator. Compute its verdict once over the fully-wired SM;
// RunCycle/RunCycleFromPhase fail closed if it found a floor hole. A bare/empty
// config yields no violations, so default orchestrators are unaffected.
```

### `go/internal/core/orchestrator.go:965` — above `defer cr.releaseShipWindow()`

```text
// Cycle-778: no exit path (abort, chokepoint escape, panic-free error
// return) may leave the ship-window lease held — siblings would wait out
// the full TTL. Idempotent; the normal release happens in recordAndBranch.
```

### `go/internal/core/orchestrator.go:969` — above `defer func() { cr.abnormalEpilogue(retErr) }()`

```text
// Cycle-1048: no exit path may leave a started cycle without its evidence
// trail (dossier + digest + coherent state) — see cyclerun_epilogue.go.
// retErr (the named return) is the abort's cause at defer-execution time —
// the distinguisher that keeps three distinct aborts from sharing one
// Unexplained fingerprint (batch-19 cycle-1208 halt).
```

### `go/internal/core/orchestrator.go:1010` — above `if werr := interaction.WriteRollup(cr.cs.WorkspacePath); werr != nil {`

```text
// ADR-0045 I1: roll every per-phase interaction ledger (bridge
// subprocess + orchestrator producers alike) into
// interaction-summary.json. Best-effort, abort paths included —
// an interaction that isn't recorded with its outcome doesn't exist.
```

### `go/internal/core/orchestrator.go:1023` — above `if cr.cs.ActiveWorktree != "" {`

```text
// ADR-0048 Slice B (SHADOW): content-addressed audit-reuse probe. If this
// cycle's worktree content already matches a prior audited verdict, the
// tdd/build/audit pipeline COULD be skipped and the prior verdict carried
// forward. Observe-only — logs the would-reuse and changes nothing (mirrors
// the Slice A shadow precedent; the EVOLVE_VERDICT_CACHE enforce dial lands
// with the enforce stage, after soak observation per ADR-0046 discipline).
//
// Probe location is pre-loop ON PURPOSE: it targets the "fast re-land" case
// (the ADR's cycles 247-248 motivation) — a preserved/re-dispatched worktree
// that still carries content a prior cycle audited PASS. A FRESH cycle's
// worktree is a clean HEAD clone with no build changes yet, so it will not
// match (correctly — there is nothing to reuse before any work runs). A
// richer post-build probe (same-tree-already-audited within a normal cycle)
// is an enforce-stage decision, deliberately out of the shadow increment.
```

### `go/internal/core/orchestrator.go:1120` — above `if act, merr := cr.maybeRemediate(next, &dr); act == loopAbort {`

```text
// Graduated remediation (2026-07-21): a configured deterministic gate
// that FAILed gets one bounded builder fix + a same-gate re-run BEFORE
// the verdict is recorded. Nothing downstream is bypassed — the same
// gate must pass and audit/EGPS/ship floors run unchanged.
```

### `go/internal/core/orchestrator.go:1139` — above `if next == PhaseScout {`

```text
// WS2-S0 (ADR-0052): post-scout re-plan hook. Fires once per cycle after
// scout's handoff has been recorded (recordAndBranch above) and BEFORE the
// next selectNext — gated on the just-completed phase being scout. Firing
// here (post-record, pre-select) is what keeps the re-plan from widening the
// run-set or bypassing the spine gate. No-op until WS2-S3 wires the shadow
// RePlan behind EVOLVE_ROUTER_REPLAN.
```

### `go/internal/core/orchestrator.go:1146` — above `normalizeScoutGoalHash(cr.cs.WorkspacePath)`

```text
// Lane-scope reconciliation (cycle-640 pin; supersedes the old
// hard-abort gate). The scout is asked to echo the pinned goal_hash
// into its Decision Trace, but that echo is a FRAGILE signal — a
// deterministic LLM transcription flip false-aborted healthy cycles
// before triage (cycles 945/947/... — greedy decoding reproduces the
// same wrong digit, so retries never self-heal). The pinned
// lane-scope.json goal_hash is authoritative, so a divergence is
// machine-STAMPED into the report (triage proceeds on a coherent lane)
// with a WARN, not an abort. Fail-open on missing pin/report/hash.
```

### `go/internal/core/orchestrator.go:1158` — above `if next == PhaseTriage {`

```text
// ADR-0076 slice C: adoption is keyed to the ACTUAL claim — triage
// claims items into processing/cycle-N mid-cycle, so only AFTER the
// triage phase can the resolver see whether this cycle's scope
// carries preserved work (architect finding #1: resolving at
// provisioning time reads a dir that does not exist yet).
```

### `go/internal/core/orchestrator.go:1171` — above `if !cr.reachedPhaseEnd {`

```text
// ADR-0044 C1 chokepoint-escape guard: the bounded loop can exit by
// exhausting its iteration budget (a transition-table cycle) instead of
// reaching PhaseEnd. That exit recorded no terminal outcome, so
// cyclehealth.ClassifyOutcome would page the cycle FAILED_UNEXPLAINED — the
// alarm bucket (the cycle-492 escape). Record an explicit abort so the escape
// is FAILED_EXPLAINED and diagnosable: it names the phase the cursor stalled
// on. Runs BEFORE finalizeCycle so the recorded FAIL preserves the worktree
// for salvage.
```

### `go/internal/core/orchestrator.go:1192` — above `func WithDossierCommit(commit bool) Option {`

```text
// WithDossierCommit decides whether each cycle's closeout dossier is
// git-committed into the project root (the production default) or only
// written (the --simulate root: a no-LLM plumbing walk must never mutate the
// operator's repository — docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).
```

### `go/internal/core/orchestrator_advisory_veto_test.go:3` — above `import (`

```text
// Cycle-238 advisory-soak defect D1 regression (orchestrator layer): when
// enforceNext DECLINES the router's plan-honoring skip (because an anchor
// artifact is missing, so SpineSatisfiedUpTo rejects the proposed jump), the
// static fallback successor — nextInOrder for a user-phase current — must NOT
// run a phase the advisory plan vetoed. In cycle 238 this decline-fallback
// chain ran 10 catalog phases the 8-phase plan never scheduled (18 phases vs
// plan; see .evolve/runs/cycle-238.reset-*/routing-decision-{8..16}.json: the
// router kept proposing "audit" while vetoed catalog phases kept executing).
```

### `go/internal/core/orchestrator_advisory_veto_test.go:52` — above `plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{`

```text
// The advisor schedules phase-a and explicitly VETOES phase-b. fakeRunners
// write no handoff artifacts, so SpineSatisfiedUpTo(audit) stays false all
// cycle (scout/build anchors artifact-absent) — the exact decline condition
// that made enforceNext fall back to the static nextInOrder successor in
// cycle 238.
```

### `go/internal/core/orchestrator_allphases_worktree_test.go:1` — above `package core`

```text
// orchestrator_allphases_worktree_test.go — CB.1 contract (concurrency campaign W4):
// EVERY dispatched phase runs with cwd = the cycle worktree.
//
// Pre-CB.1, runsInWorktree scoped the worktree cwd to source writers (tdd,
// build, writes_source user phases) + audit. Everything else dispatched with
// Worktree="" → cwd = the MAIN repo root, so a read-only phase's stray write
// (or a guard misfire — the cycle-280 inserted-phase fatal) landed in the live
// tree. CB.1 inverts the dispatch default: the worktree is provisioned at
// cycle start, so every phase gets cwd=worktree and the main tree is touched
// by no phase subprocess at all (the integrator alone writes main, CD track).
//
// CRITICAL discriminator these tests keep honest: cwd is NOT write permission.
// The write axis (role-gate + tree-diff guard + normalize) keys off
// worktreePhase / WorktreePhase and minted writes_source — that axis must be
// BYTE-IDENTICAL before and after CB.1. Only the cwd axis widens.
```

### `go/internal/core/orchestrator_allphases_worktree_test.go:107` — above `func TestCB1_ResumePathCarriesWorktree(t *testing.T) {`

```text
// TestCB1_ResumePathCarriesWorktree: RunCycleFromPhase is a first-class
// dispatch surface — `evolve loop --resume` is the standard recovery path
// after ANY cycle failure — and it builds its PhaseRequest independently of
// the RunCycle loop. It must thread the persisted cs.ActiveWorktree into
// every resumed phase, or a resumed tdd/build runs cwd=main-tree: the exact
// cycle-280 class CB.1 closes, reopened only on resume (review BLOCK finding).
```

### `go/internal/core/orchestrator_auditleak_test.go:3` — above `package core`

```text
// orchestrator_auditleak_test.go — cycle-235 task `audit-phase-leak-recover` (RED).
//
// Inbox defect (2026-06-06T05-48-00Z-audit-leak-recover): `evolve acs suite`
// during AUDIT rebuilds go/evolve in the MAIN tree; the post-phase tree-diff
// guard (orchestrator.go ~1632) sees the binary as a newly-dirty main-tree
// path and aborts the whole cycle. recoverBuildLeak is gated on PhaseBuild,
// so the audit phase has no recovery path — a rebuilt binary kills the cycle.
//
// Contract encoded here (scout-report cycle-235, Task 2):
//   - when the ONLY newly-dirty main-tree paths after a guarded phase are
//     tracked build artifacts (buildArtifacts: go/evolve, go/bin/evolve),
//     the orchestrator discards the churn in the MAIN tree, re-checks, WARNs,
//     and CONTINUES the cycle (the binary is restored to committed content);
//   - any non-artifact leak still aborts the cycle via the tree-diff guard;
//   - the recovery path never reverts non-artifact files (operator work).
//
// RED note: this test compiles against existing API and fails at RUNTIME
// today — subtest binary_churn_recovered aborts with the tree-diff error.
// Builder makes it GREEN by adding the phase-agnostic binary discard before
// the `res.Error(...)` return in the tree-diff guard. The other subtests are
// pre-existing-GREEN regression pins (clean cycle + non-artifact abort) that
// must SURVIVE the fix.
```

### `go/internal/core/orchestrator_auditleak_test.go:166` — above `if got := auditLeakReadFile(t, filepath.Join(root, "go", "evolve")); got != auditLeakBinV1 {`

```text
// The churned binary must be restored to its committed content —
// continuing WITHOUT discarding would commit binary drift (cycle-153).
```

### `go/internal/core/orchestrator_backfill_path_test.go:8` — above `func TestBackfillArtifactPath_AllPhases(t *testing.T) {`

```text
// TestBackfillArtifactPath_AllPhases pins the phase→filename mapping that
// backfillArtifactPath must produce (cycle-187 AC-3/AC-4, Scout BA-1). The
// orchestrator passes this path to backfill.TryExtract; a wrong filename means
// a backfilled artifact lands where the next phase never reads it.
//
// RED at baseline for "retro" and "build-planner": the function's default
// branch yields "retro-report.md" / "build-planner-report.md", but the agents
// write "retrospective-report.md" / "build-plan.md". The other rows are
// regression guards so a future phase addition can't silently re-break the
// default-branch phases.
```

### `go/internal/core/orchestrator_contextfill_projection_test.go:1` — above `package core`

```text
// orchestrator_contextfill_projection_test.go — cycle-1271 RED tests for the
// contextfill → phase-timing wiring (task wire-contextfill-into-phasetiming-entry).
//
// These are the WIRING PROOF: they drive the real production chokepoint,
// (*Orchestrator).recordPhaseOutcome — the ADR-0044 C1 single writer of
// phase-timing.json, the exact seam token-telemetry S4 was wired into — and
// assert the derived fill ratio lands on the timing entry it emits. A test that
// called contextfill.FillRatio directly would pass on dead code and prove
// nothing; these fail until a production path actually derives the ratio.
//
// Tier provenance. PhaseOutcome carries no abstract tier field, but
// ResolvedModel already IS the tier the terminal attempt ran at
// (internal/phases/runner/runner.go:854-859 sets it from tieredRes.Tier,
// falling back to the concrete model id only in the empty-candidates edge case).
// So contextfill.WindowSizeForTier(out.ResolvedModel) is the honest lookup: a
// canonical tier yields the 200k window, anything else yields 0 →
// contextfill.ErrInvalidWindow → both fields stay zero. No tier is ever invented.
```

### `go/internal/core/orchestrator_faillearn_test.go:1` — above `package core_test`

```text
// orchestrator_faillearn_test.go — failure-floor Phase 2 (inbox
// retro-always-invariant, gap 1 / cycle-243 reproduction).
//
// Behavioral contract: when the LLM retro degrades during failure
// learning (retro runner error, e.g. bridge timeout exit=81, or a
// non-canonical verdict), the orchestrator must still produce durable
// learning artifacts deterministically — a retrospective-report.md in
// the cycle workspace and a failure-lesson YAML in
// .evolve/instincts/lessons/ — instead of only a stderr WARN.
//
// Shares the core_test harness (newRunners / newTestOrchestrator)
// defined in orchestrator_recovery_test.go.
```

### `go/internal/core/orchestrator_faillearn_test.go:26` — above `type staticVerdictRunner struct {`

```text
// staticVerdictRunner succeeds at the transport level but returns a
// fixed (possibly non-canonical) verdict — the cycle-243 "retro ran but
// produced garbage" shape.
```

### `go/internal/core/orchestrator_faillearn_test.go:139` — above `type reportingErrRunner struct{ name string }`

```text
// reportingErrRunner writes a report carrying a v2 failure-block sentinel,
// then fails — the "phase was healthy enough to self-report" shape
// (ADR-0039 §7 item 5).
```

### `go/internal/core/orchestrator_ghostphase_test.go:3` — above `import (`

```text
// orchestrator_ghostphase_test.go — cycle-265 incident RED tests: the routing
// surface (registry order + catalog) can know phases the DISPATCH surface
// cannot run. Live: registry-listed `memo` had no .evolve/phases config, so
// no specrunner was registered; after the post-ship optional phases the
// static order walked into it and `no runner registered for phase memo`
// killed a batch whose cycle had already PASSED and shipped.
//
// Kernel floor: a selected-but-unregistered OPTIONAL phase is skipped loudly
// (WARN; the order walk continues; it never appears in PhasesRun — nothing
// dispatched). A missing MANDATORY runner remains a fatal wiring bug.
```

### `go/internal/core/orchestrator_ghostphase_test.go:42` — above `runners[Phase("user-a")] = &fakeRunner{name: "user-a"}`

```text
// user-a IS dispatchable; ghost-phase is catalog/order-known but has NO
// runner — the cycle-265 memo shape.
```

### `go/internal/core/orchestrator_guard_test.go:16` — above `func TestIsLegitimateMainTreePath(t *testing.T) {`

```text
// orchestrator_guard_test.go — cycle-274 G task: inserted-phase tree-diff guard gap.
//
// Background (cycle-270 defect): the tree-diff guard was gated on
// `phaseWorktree != ""`, so non-worktree phases (scout, triage, retro, and
// advisor-inserted minted phases) ran with no snapshot — any untracked source
// file they wrote to the main tree slipped through to audit and potentially to
// ship. The fix (G-B): drop the worktree gate, snapshot every phase, filter
// legitimate `.evolve/` workspace writes via isLegitimateMainTreePath.
//
// Test map:
//   TestIsLegitimateMainTreePath     — R9: pure classifier unit test (no git, ≥2 sub-cases)
//   TestGuardCatchesInsertedPhaseLeak — R5+R6: cycle aborts when untracked source leak detected (≥2 sub-cases)
//   TestGuardIgnoresLegitimateWorkspaceWrite — R7: legitimate .evolve/ workspace write does NOT trip the guard
```

### `go/internal/core/orchestrator_guard_test.go:132` — above `name:      "untracked_source_file_after_scout",`

```text
// R6: an untracked source file (never in git) shows up after scout
// (a spine, non-worktree phase) — the original cycle-270 escape route.
```

### `go/internal/core/orchestrator_guard_test.go:176` — above `func TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase(t *testing.T) {`

```text
// TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase pins the CI regression
// from the cycle-274 salvage: during a WORKTREE phase (tdd/build), the
// orchestrator-side contract gate writes its own untracked runtime state
// (.evolve/contract-gate-breaker.json) into the main tree. That is not a
// phase escape — recoverBuildLeak already classifies it legitimate and skips
// it — so the tree-diff guard must apply the same classification instead of
// aborting the cycle (guard and recovery must agree on one vocabulary).
```

### `go/internal/core/orchestrator_guard_test.go:258` — above `workspacePath := ".evolve/runs/cycle-1/triage-report.md"`

```text
// Simulate triage writing its workspace report (.evolve/runs/cycle-1/triage-report.md).
// The guard sees this as a new path but isLegitimateMainTreePath returns true,
// so the cycle continues without a tree-diff abort.
```

### `go/internal/core/orchestrator_guard_test.go:321` — above `func TestGuardIgnoresScoutEvalMaterialization(t *testing.T) {`

```text
// TestGuardIgnoresScoutEvalMaterialization pins the soak-#6 cycle-319 defect:
// scout's contract (internal/evalgate/materialization.go) is to write the
// SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN
// tree, where Gate A reads them. A later cycle iterating the same coverage
// target re-materializes the same slug, MODIFYING the prior cycle's committed
// eval (318→319 ledger-seal-io-coverage) — a main-tree write the tree-diff
// guard wrongly flagged as a deliverable leak and aborted the cycle. Scout's
// eval materialization is its JOB, not a deliverable escape.
```

### `go/internal/core/orchestrator_guard_test.go:375` — above `func TestGuardRecoversCatalogWritesSourcePhaseLeak(t *testing.T) {`

```text
// --- cycle-533: fix-treediff-leak-recovery-catalog-predicate ---
//
// The leak-recovery gate at cyclerun_review.go:263 invokes recoverBuildLeak
// only when `WorktreePhase(next)` is true — the catalog-BLIND two-literal set
// {tdd, build}. Every other phase, INCLUDING advisor-minted / catalog phases the
// catalog marks writes_source:true (bug-reproduction, coverage-gate), gets ZERO
// recovery: any source file it leaks into the main tree goes straight to the
// tree-diff guard, which hard-aborts the cycle. That is the confirmed,
// repeating root cause of the 10 recorded "tree-diff guard: phase wrote to the
// main tree" aborts (cycles 390..529). The catalog-aware verdict already exists
// on the Orchestrator — (*Orchestrator).worktreePhase(next) — and `cr.o` (the
// *Orchestrator holding the catalog) is already in scope at this exact call
// site, so the fix is a one-line swap to the method form. No new field is
// needed here (contrast the role-gate half, which is control-plane-protected —
// see the cycle-533 test-report AC-Materialization section).
//
// Both cases use a REAL git repo + the real gitWorktree provisioner (mirroring
// TestTDDLeakRecover / TestOrchestrator_AuditLeakRecover) so the production
// recovery + tree-diff guard paths run end-to-end — not a stubbed classifier.
```

### `go/internal/core/orchestrator_guard_test.go:395` — above `func TestGuardRecoversCatalogWritesSourcePhaseLeak(t *testing.T) {`

```text
// TestGuardRecoversCatalogWritesSourcePhaseLeak is the cycle-533 RED proof. A
// spine phase that is NOT a WorktreePhase literal (scout) is marked a catalog
// source-writer via WithCatalog and leaks a tracked-file edit into the main
// tree. With the bug, WorktreePhase(scout)==false skips recovery and the
// tree-diff guard aborts the cycle. With the fix, o.worktreePhase(scout)==true
// (catalog), recoverBuildLeak relocates the leak into the worktree, main is
// restored to HEAD, and the cycle proceeds to ship.
```

### `go/internal/core/orchestrator_guard_test.go:444` — above `func TestGuardStillAbortsNonSourcePhaseLeak(t *testing.T) {`

```text
// TestGuardStillAbortsNonSourcePhaseLeak is the paired anti-over-broadening
// guard: the catalog-aware gate must NOT make recovery unconditional. A phase
// that is NOT a declared source-writer (audit — absent from the injected
// catalog, so o.worktreePhase(audit)==false) leaking a source file into the
// main tree must STILL hard-abort via the tree-diff guard. Shares the positive
// case's catalog (scout=source-writer) to prove the gate keys off the per-phase
// verdict, not a blanket allow. Passes before AND after the fix (a
// pre-existing-green regression pin; RED status noted in the cycle-533 report).
```

### `go/internal/core/orchestrator_hasrunner_test.go:5` — above `func TestOrchestrator_HasRunner(t *testing.T) {`

```text
// TestOrchestrator_HasRunner pins the composition-root read seam behind the
// cycle-563 memo-dispatch bug: a phase the router can nominate but that has
// no registered runner is silently skipped by dispatch's missing-runner
// escape hatch, so HasRunner is how tests prove a routing→dispatch handoff
// is actually wired — not just that Route() names the phase. CI's apicover
// -enforce flagged it UNCOVERED (no test named it).
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:1` — above `package core`

```text
// orchestrator_inserted_worktree_test.go — RED contract for the cycle-280 P0:
// advisor-INSERTED (minted) write-capable phases dispatched with Worktree=""
// because the mint template defaults writes_source:false, so worktreePhase /
// runsInWorktree returned false → phaseWorktree="" → the tree-diff guard fired
// cycle-fatal AND the abort cleanup deleted the uncommitted worktree (all
// builder output lost). Two HIGH inbox items (2026-06-10T19:35Z/19:36Z) and the
// scout-281 P0 carryover mandate this fix.
//
// The contract these tests pin (implementation-agnostic — they assert the
// observable runsInWorktree / Cleanup behaviour, not any field type):
//
//  1. An inserted phase the advisor mints WITHOUT explicitly opting out of
//     source writes DEFAULTS to write-capable, so it inherits the cycle
//     worktree (runsInWorktree == true). A minted phase that gets no worktree
//     is the exact cycle-280 fatal.
//  2. A minted phase that EXPLICITLY declares writes_source:false stays
//     read-only and gets NO worktree (the discriminator: the fix must not
//     blanket-grant worktrees to every mint, only default the unspecified
//     ones to write-capable). Distinguishing "unspecified" from "explicit
//     false" is what forces the real fix (a tri-state mint flag) rather than a
//     no-op.
//  3. On an ABNORMAL mid-cycle abort (phase-fatal / guard-abort), the worktree
//     is PRESERVED, never pruned — so uncommitted builder work survives for
//     recovery, the way the ship-failure path already preserves it.
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:32` — above `const writableMintPlanJSON = '[{"phase":"test-amplification","run":true,"justification":"amplify adversarial tests for t…`

```text
// writableMintPlanJSON is a realistic advisor plan that inserts a write-capable
// phase the SAME way cycle-280's failing advisor did: a mint block that does NOT
// set writes_source. Per the cycle-280 forensic this must still inherit the
// worktree. parsePhasePlan is the real advisor-output parser, so the test
// exercises the genuine mint→register→dispatch seam.
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:44` — above `func TestInsertedPhaseWritableInheritsWorktree(t *testing.T) {`

```text
// TestInsertedPhaseWritableInheritsWorktree: an advisor-minted phase that does
// not explicitly mark itself read-only must dispatch with cwd=worktree. RED
// today because the mint default is writes_source:false → runsInWorktree false →
// Worktree="" → tree-diff guard fatal (cycle-280).
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:60` — above `if !o.worktreePhase(Phase("test-amplification")) {`

```text
// Post-CB.1 the worktree CWD is universal (every phase dispatches with it —
// see TestCB1_EveryDispatchedPhaseCarriesWorktree), so the cycle-280 fatal
// class is closed structurally. What this test still pins is the WRITE axis:
// a mint that does not opt out of source writes must be write-capable, or
// the role-gate denies its edits and the phase stalls.
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:99` — above `type fatalBuildRunner struct{ err error }`

```text
// fatalBuildRunner always returns an error, modelling a non-recoverable
// mid-cycle phase abort (the class that includes the cycle-280 guard-fatal).
```

### `go/internal/core/orchestrator_inserted_worktree_test.go:108` — above `func TestAbortCleanupPreservesWorktreeDiff(t *testing.T) {`

```text
// TestAbortCleanupPreservesWorktreeDiff: when a cycle ends ABNORMALLY before
// ship (here a fatal build phase — the same abort class as the cycle-280
// guard-fatal), the worktree holding uncommitted builder work must be PRESERVED
// for recovery, never pruned by the exit cleanup. RED today: the abort-cleanup
// defer only preserves on ship failure, so a mid-cycle phase-fatal silently
// deletes the worktree and its uncommitted diff (cycle-280 data loss).
```

### `go/internal/core/orchestrator_insertedleak_test.go:3` — above `package core`

```text
// orchestrator_insertedleak_test.go — acceptance pin for inbox defect
// 2026-06-10T09-40-00Z-inserted-phase-treediff-guard-gap (cycle-270 replay).
//
// Cycle-270: the advisor-inserted bug-reproduction phase wrote
// go/internal/looppreflight/bug_reproduction_test.go into the MAIN tree and no
// tree-diff guard fired — the cycle proceeded to audit/ship as if clean. The
// guard then was snapshot-scoped to source-writing SPINE phases; inserted/
// minted phases were invisible to it.
//
// Production has since moved (7e0df0b5 one-classifier-for-all-phases +
// 02b778ef writes_source-keyed worktree dispatch): the pre-phase snapshot and
// post-phase check now run for EVERY dispatched phase. These tests pin the
// inbox item's acceptance at the genuine end-to-end seam — parsePhasePlan
// (real advisor-output parser) → ClampPlanToFloorWith → registerMintedPhases
// → dispatch loop → tree-diff guard — so the coverage can never silently
// regress to phase-identity scoping again:
//
//  1. A minted phase that writes a NEW source file into the MAIN tree (the
//     exact cycle-270 shape: untracked, so the porcelain — not diff-HEAD —
//     path must catch it) is AUTO-RECOVERED: leak-recovery (cycle-528/529)
//     relocates the leaked file into the phase's real worktree, stages it, and
//     restores the main tree clean, so the cycle completes and ships with the
//     work still visible to the auditor's `git diff HEAD`. This preserves
//     cycle-270's true guarantee — leaked work must never be invisible to audit
//     — via the newer non-aborting mechanism (the file lands in the worktree,
//     not silently in main). Recovery needs a REAL git worktree to `git add`
//     into: the earlier fake non-git t.TempDir() made that add fail, aborting
//     the cycle via the wrong path (cycle-535 CI-red root cause).
//  2. The same minted phase writing inside its provisioned worktree is clean:
//     the cycle continues to ship. The discriminator that keeps the guard
//     honest — isolation is the contract, not "minted phases always recover".
```

### `go/internal/core/orchestrator_insertedleak_test.go:51` — above `const insertedLeakPlanJSON = '[{"phase":"bug-reproduction","run":true,"justification":"reproduce the reported defect as …`

```text
// insertedLeakPlanJSON replays the cycle-270 advisor output shape: a single
// minted bug-reproduction phase inserted after build, NOT opting out of source
// writes (so it inherits the cycle worktree per 02b778ef). The mandatory spine
// is re-added by ClampPlanToFloorWith, exactly as in production.
```

### `go/internal/core/orchestrator_insertedleak_test.go:57` — above `const insertedLeakRelPath = "go/internal/looppreflight/bug_reproduction_test.go"`

```text
// insertedLeakRelPath is the file cycle-270's inserted phase leaked into the
// main tree (from .evolve/runs/cycle-270/bug-reproduction-report.md).
```

### `go/internal/core/orchestrator_insertedleak_test.go:78` — above `func initInsertedLeakRepo(t *testing.T) string {`

```text
// initInsertedLeakRepo creates a real git repo (production gitDirtyPaths runs
// real git against it) with one committed source file, so the leaked test file
// is a NEW untracked path — the cycle-270 shape the tracked-only diff-HEAD
// baseline used to miss.
```

### `go/internal/core/orchestrator_insertedleak_test.go:108` — above `type realLeakWorktree struct {`

```text
// realLeakWorktree is a WorktreeProvisioner that provisions a REAL `git worktree
// add` off the cycle's projectRoot, OUTSIDE the repo (so worktree writes can
// never appear in the main porcelain — the Test 2 discriminator's invariant).
// It replaces the former fakeWorktree{path: t.TempDir()}: that fake was a plain
// non-git dir, so recoverBuildLeak's `git add -f` of a relocated leak failed
// with "not a git repository" (rc=128), recovery reported failure, and the
// cycle aborted via the "recovery failed" path instead of auto-healing — the
// cycle-535 CI-red root cause. Cleanup is a deliberate no-op so a shipped
// cycle's worktree survives for post-run `git diff HEAD` inspection; t.TempDir
// handles the filesystem teardown. Mirrors buildleak_recover_test.go's
// realWorktree() production topology.
```

### `go/internal/core/orchestrator_insertedleak_test.go:136` — above `func insertedLeakOrchestrator(t *testing.T, planJSON string, onRun func(PhaseRequest)) (*Orchestrator, *realLeakWorktree…`

```text
// insertedLeakOrchestrator wires the full advisory-mint path: routing at
// Advisory+DynamicLLM, a fixedPlanner serving the parsed cycle-270-shaped
// plan, and the leakMinter as registrar. It provisions a REAL git worktree off
// the CycleRequest.ProjectRoot (via realLeakWorktree) so recoverBuildLeak has a
// genuine worktree to relocate + stage a leaked path into. The returned
// provisioner's .path is populated once the cycle provisions the worktree.
```

### `go/internal/core/orchestrator_insertedleak_test.go:161` — above `func TestInsertedPhaseMainTreeLeakRecovers(t *testing.T) {`

```text
// TestInsertedPhaseMainTreeLeakRecovers — inbox acceptance #1, on the newer
// leak-recovery contract (cycle-528/529): an advisor-inserted (minted) phase
// that leaks a NEW untracked source file into the MAIN tree is AUTO-RECOVERED
// at the phase boundary — recoverBuildLeak relocates the file into the phase's
// real worktree, stages it, and restores the main tree clean — so the cycle
// completes and ships with the work still visible to the auditor's
// `git diff HEAD`. This preserves cycle-270's guarantee (leaked work must never
// be invisible to audit) via the non-aborting mechanism. Renamed from
// ...Aborts: the guard no longer aborts this shape, it heals it, and a test
// named "Aborts" asserting "ships" would be a maintenance landmine.
```

### `go/internal/core/orchestrator_intent_required_test.go:3` — above `import (`

```text
// Cycle-238 advisory-soak defect D4, orchestrator call-site: the upfront-plan
// RouteInput (planIn) the orchestrator hands the advisor — and then passes to
// ClampPlanToFloorWith — must carry the cycle's IntentRequired bit. Without
// it the floor clamp cannot force intent into the plan, and the advisory
// override drops the operator's EVOLVE_REQUIRE_INTENT=1 gate (see
// floor_intent_test.go in internal/router for the clamp-side contract).
```

### `go/internal/core/orchestrator_optional_skip_test.go:1` — above `package core`

```text
// orchestrator_optional_skip_test.go — RED contract for the cycle-283 killer:
// an advisor-scheduled OPTIONAL phase that exhausts its retries on an
// INFRA-shaped error (bridge artifact timeout exit=81 / transient bridge
// failure) aborted the whole cycle via wrapCycleLevelError, so audit and ship
// never ran and completed spine work was discarded unshipped. The intended
// behavior has been documented on ErrArtifactTimeout since Workstream D
// (errors.go: "an OPTIONAL phase that hits this degrades to WARN+advance
// instead of aborting the whole cycle") but was never implemented.
//
// The contract these tests pin:
//
//  1. Optional phase + infra-shaped exhaustion → synthesized WARN, cycle
//     ADVANCES; audit and ship still run (the operator policy: work that
//     would pass review must reach review; review-PASS must ship).
//  2. A MANDATORY/floor phase (build) with the same infra exhaustion stays
//     cycle-fatal — the skip must never weaken the integrity floor.
//  3. An optional phase failing with a NON-infra error stays cycle-fatal —
//     only infrastructure weather qualifies, never integrity or logic
//     failures (tree-diff guard aborts, gate failures, generic errors).
```

### `go/internal/core/orchestrator_optional_skip_test.go:33` — above `func optionalSkipHarness(t *testing.T, optRunner PhaseRunner) (*Orchestrator, *fakeRunner, *fakeRunner) {`

```text
// optionalSkipHarness builds the advisory-routing orchestrator of the
// cycle-283 shape: spine runners all green, one catalog-Optional phase
// scheduled after build whose runner behavior the caller controls.
```

### `go/internal/core/orchestrator_optional_skip_test.go:74` — above `func TestOptionalPhaseInfraTimeoutSkipsAndCycleShips(t *testing.T) {`

```text
// TestOptionalPhaseInfraTimeoutSkipsAndCycleShips: the cycle-283 replay.
// amplify-tests (catalog-Optional, advisor-scheduled) times out on every
// attempt; the cycle must degrade it to WARN and still run audit + ship. RED
// today: wrapCycleLevelError aborts the cycle without consulting optionality,
// so audit/ship never run.
```

### `go/internal/core/orchestrator_optional_skip_test.go:165` — above `func TestMandatoryPhaseNeverInfraSkipped(t *testing.T) {`

```text
// TestMandatoryPhaseNeverInfraSkipped pins the generalized infra-skip guard:
// a configured-mandatory phase is NEVER infra-skipped, even if its catalog spec
// is (mis)marked Optional. This generalizes the former ship-only special case —
// the engine consults the mandatory SET (config), not a hardcoded phase name
// (phase-agnostic flow, ADR-0035/0038). RED until optionalInfraSkip reads
// isConfiguredMandatory: the old `if p == PhaseShip` guard protects only ship,
// so a mandatory-but-optional non-ship phase is wrongly skip-eligible.
```

### `go/internal/core/orchestrator_outcome_test.go:46` — above `o := &Orchestrator{}`

```text
// Cycle 1630: a sibling lane's landing moved HEAD, but this cycle never
// shipped. The label must not be inferred from anything but the latch.
```

### `go/internal/core/orchestrator_phaseboundary_test.go:1` — above `package core_test`

```text
// orchestrator_phaseboundary_test.go — cycle-234 task `phase-boundary-checkpoint` (RED).
//
// Invariant 3 (campaign retro cycles 215-231): a durable checkpoint must
// exist at EVERY phase boundary so `evolve loop --resume` can reconstruct a
// cycle after a kill at any point — not only at the quota wall (the only
// trigger before this cycle; three --resume attempts failed this campaign
// with "no live checkpoint").
//
// Behavioral contract under test: after each phase completes, the on-disk
// <projectRoot>/.evolve/cycle-state.json gains/updates an additive
// "checkpoint" block with reason "phase-complete" whose completedPhases
// include the just-completed phase. The probe runner reads the REAL file
// mid-cycle, so a checkpoint written only at cycle end cannot fake a pass.
//
// Shares the core_test harness (newRunners / newTestOrchestrator) defined in
// orchestrator_recovery_test.go.
```

### `go/internal/core/orchestrator_phaseoutcome_test.go:3` — above `package core`

```text
// orchestrator_phaseoutcome_test.go — ADR-0044 C1 (Slice 1) RED tests:
// single-source phase-outcome recording.
//
// cycle-262 (2026-06-09) located fork: the build phase's CLI fallback
// SUCCEEDED (codex exit 81 → claude exit 0, build-report.md PASS, the runner
// returned PASS/nil), then the post-phase tree-diff guard CORRECTLY aborted
// the cycle (the fallback builder wrote the tracked config
// .evolve/commit-prefix-scope.json to the MAIN tree, and recoverBuildLeak
// deliberately skips .evolve/ paths). The abort path — like EVERY abort path
// between runner.Run returning and the happy-path recording site
// (orchestrator.go ~2084) — returned without recording the phase outcome: no
// phase-timing.json entry, no <phase>-usage.json, no PhasesRun membership.
// Reality (build ran, burned tokens, PASSed) diverged from the record (build
// never happened) — the D1 divergence ADR-0044 C1 makes structurally
// impossible.
//
// Contract encoded here (the C1 chokepoint): EVERY terminal disposition of a
// phase dispatch — happy advance AND each abort return (exhausted retries,
// non-canonical verdict, review-gate reject, tree-guard abort, ledger append
// failure) — records the outcome exactly once: PhasesRun membership +
// phase-timing.json entry + <phase>-usage.json sidecar, carrying the phase's
// own canonical verdict (synthesizing FAIL when none exists — NEVER PASS)
// and, on aborts, a non-empty abort_reason. Cycle-level semantics are
// unchanged: a tree-guard abort still fails the cycle; recording reflects
// reality, it does not resurrect the cycle.
//
// RED note: these tests compile against existing API only (same approach as
// orchestrator_timing_test.go) and fail at RUNTIME today — the abort paths
// return before any recording happens.
```

### `go/internal/core/orchestrator_phaseoutcome_test.go:46` — above `type outcomeRunner struct {`

```text
// outcomeRunner PASSes its phase with a scripted cost/duration after running
// a side effect — models cycle-262's build: real work done (tokens burned,
// PASS report) with an optional main-tree leak that trips the tree-diff guard.
```

### `go/internal/core/orchestrator_phaseoutcome_test.go:71` — above `func initOutcomeRepo(t *testing.T) string {`

```text
// initOutcomeRepo creates a real git repo whose committed tree contains the
// tracked config file cycle-262's builder leaked (.evolve/commit-prefix-scope.json).
// The default gitDirtyPaths runs real git against it, so the tree-diff guard
// exercises its production code path — and recoverBuildLeak's deliberate
// ".evolve/ paths are never relocated" skip applies exactly as it did live.
```

### `go/internal/core/orchestrator_phaseoutcome_test.go:160` — above `func TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome(t *testing.T) {`

```text
// TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome pins the cycle-262
// CLASS: build PASSes with real cost, leaves the main tree dirty in a way
// recovery cannot repair, the tree-diff guard aborts the cycle (CORRECT),
// and the build outcome must STILL be recorded: PhasesRun membership, a
// phase-timing entry carrying the agent's own PASS + cost + duration + a
// non-empty abort_reason, and build-usage.json.
//
// Fixture note: the original fixture was 262's literal leak (a tracked
// .evolve/commit-prefix-scope.json edit) — that is now RECOVERABLE by design
// (the deliverable allowlist relocates it; pinned in
// buildleak_recover_test.go), so this test uses a STAGED RENAME of a tracked
// file, which no recovery branch handles — the canonical still-unrecoverable
// main-tree mutation.
```

### `go/internal/core/orchestrator_postship_observer_test.go:3` — above `import (`

```text
// orchestrator_postship_observer_test.go — RED contract for cycle-574 Task 1
// (fix-memo-phase-tier-envelope, inbox weight 0.95 critical) SECOND half.
//
// Context. Cycle-573 already closed the tier half of this task: the memo pin's
// model tier now satisfies its profile envelope (internal/policy
// memo_envelope_config_test.go — GREEN). But the inbox item asks for TWO fixes;
// the tier alignment was only the first. The second, still unimplemented:
//
//	"classify PASS-side post-ship observer phases (memo, post-ship-monitor) as
//	 non-fatal: a failure there downgrades to a WARN diagnostic on an
//	 already-shipped cycle, never a cycle-level failure."
//
// Today, a memo phase that fails AFTER a healthy ship (the exact envelope-error
// shape, or any other non-infra error) flows through dispatch's abort arm
// (cyclerun_dispatch.go: wrapCycleLevelError) and turns a shipped cycle
// abnormal — the worktree is preserved, wave accounting reports 0/N ok, retro
// fires on a healthy ship. optionalInfraSkip does NOT cover this: it degrades
// only INFRA-shaped errors (artifact timeout / transient bridge), never a
// policy/logic error, and it is blind to whether ship already succeeded.
//
// The contract these tests pin — a NEW orchestrator classifier
//
//	func (o *Orchestrator) postShipObserverSkip(p Phase, shipped bool) bool
//
// which returns true (degrade the failed phase to WARN + advance instead of
// aborting) iff ALL hold:
//
//  1. shipped == true — ship has already recorded a PASS this cycle. A failure
//     BEFORE ship is never swallowed (nothing shipped to protect yet).
//  2. p is a best-effort post-ship observer — a RoleControl observer phase
//     (memo / post-ship-monitor), NOT ship/build/audit/etc.
//  3. p is NOT configured-mandatory and NOT in the resolved ship floor — the
//     skip can never weaken the integrity floor (same guard optionalInfraSkip
//     uses).
//
// RED today: postShipObserverSkip is undefined, so package core's test build
// fails to compile — the intended RED signal before Builder implements the
// classifier and wires it into the dispatch abort path.
```

### `go/internal/core/orchestrator_recovery_test.go:239` — above `func TestRunCycle_ShipControlPlaneViolation_RebuildsThenShips(t *testing.T) {`

```text
// F37 (architecture review M3): a control-plane refusal (CONTROL_PLANE_VIOLATION,
// class precondition) recovers into BUILD — the phase that owns the diff —
// then audit re-binds and ship succeeds, through the real orchestrator. The
// generic precondition route would re-audit the same diff instead (the
// cycle-230 audit↔ship loop).
```

### `go/internal/core/orchestrator_spinegate_test.go:1` — above `package core`

```text
// orchestrator_spinegate_test.go — R5 (concurrency-factory plan): the spine
// gate fails CLOSED at EVOLVE_PHASE_RECOVERY=enforce.
//
// Cycle-283: build proceeded despite a missing mandatory-predecessor handoff
// — "[orchestrator] WARN spine not satisfied for next=build ... proceeding
// fail-open". The fail-open rationale was that Digest cannot distinguish a
// transient READ MISS from a genuine ABSENCE; that distinction now exists
// (RoutingSignals.DigestDegraded), so a clean absence can block.
//
// Contract pinned here:
//  1. enforce + clean absence → the cycle ABORTS (FAILED-EXPLAINED: typed
//     error naming the gate; worktree preserved) instead of running a phase
//     whose mandatory predecessor never delivered.
//  2. shadow (default) → byte-compatible with today: WARN + proceed. The
//     block ships dormant until the R8.5 dial flip.
//  3. enforce + DEGRADED digest (handoff unreadable for a non-absence
//     reason) → fail-open WARN: a transient read error must never false-
//     block a real cycle.
//  4. The waiver is the EXISTING config escape (R5.3): an anchor removed
//     from cfg.Mandatory is not required — no new machinery.
```

### `go/internal/core/orchestrator_spinegate_test.go:34` — above `func spineGateOrch(t *testing.T, recovery config.Stage, mandatory []string) (*Orchestrator, *fakeWorktree, *fakeStorage)…`

```text
// spineGateOrch builds an advisory-routing orchestrator whose fake runners
// write NO handoff artifacts — the cycle-283 shape: CompletedPhases says
// scout ran, but no handoff-scout.json (and, post-fallback, no
// scout-report.md) exists, so SpineSatisfiedUpTo(build) is false at the build
// transition. R8.5: the gate keys on the spine floor's OWN dial (SpineFloor),
// decoupled from the overloaded PhaseRecovery.
```

### `go/internal/core/orchestrator_test.go:765` — above `for _, p := range res.PhasesRun {`

```text
// Disposition contract (cycle-1046 verdict-path wiring): fixtures carry no
// disposition.json, so the gate prefixes its loud reason — the branch
// decision must still be carried (suffix), and the gate must be audible.
```

### `go/internal/core/orchestrator_test.go:867` — above `func TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers(t *testing.T) {`

```text
// A phase that hits a bridge ArtifactTimeout once then succeeds must be
// relaunched, and the cycle must complete normally (cycle-149 exit=81 at scout
// aborted the whole loop with no retry).
```

### `go/internal/core/orchestrator_timing_test.go:3` — above `import (`

```text
// RED tests for cycle-171 T1 (phase-timing-json + phase_retry ledger) and T2
// (structured-failure-diag). White-box (package core) so they reuse the existing
// fakeStorage / fakeLedger / buildRunners / wrapTimeout harness in
// orchestrator_test.go. They reference ONLY already-public symbols so the core
// test binary still COMPILES at the pre-implementation baseline — they fail at
// RUNTIME (file missing / no ledger entry), which is the correct RED signal and
// does not break sibling tests. Builder makes them GREEN by writing the
// phase-timing.json accumulator, the phase_retry ledger append, and the
// <phase>-failure-diag.json writer in orchestrator.go.
```

### `go/internal/core/orchestrator_triageleak_test.go:3` — above `package core`

```text
// orchestrator_triageleak_test.go — cycle-564 task
// decouple-leak-recovery-from-worktree-phase-gate (RED).
//
// Mirrors orchestrator_auditleak_test.go's harness, but proves the OTHER half
// of the fix: recovery must fire for a phase that is NOT a WorktreePhase
// (role-gate write-permission axis) at all — triage, scout, audit, and
// bug-reproduction all get an active cycle worktree (provisioned once at
// cycle start, before any phase runs) but today's recovery call site
// (cyclerun_review.go ~263) gates on cr.o.worktreePhase(next), which is false
// for triage. An untracked leak there has ZERO recovery path today and hard-
// aborts via the tree-diff guard — the exact recurring signature behind
// cycles 390/399/491/496/501/538/540/556 (9 recorded carryover failures).
```

### `go/internal/core/orchestrator_triageleak_test.go:69` — above `git("config", "user.name", "t")`

```text
// Identity must live IN the repo, not in the helper's env: RunCycle's own
// git children (the dossier-closeout commit) don't inherit these env vars,
// and CI's ubuntu runners have no ambient identity git can auto-detect —
// the commit fails there, leaving staged dossier files that broke the
// clean-tree assertion below (ubuntu-only red, 2026-07-06).
```

### `go/internal/core/orchestrator_workspace_test.go:47` — above `func TestArchivePollutedWorkspace_NonEmptyDirRenamed(t *testing.T) {`

```text
// TestArchivePollutedWorkspace_NonEmptyDirRenamed is the regression for
// cycle-108: a prior attempt's scout-report.md must be moved aside so
// the fresh phases run cleanly.
```

### `go/internal/core/orchestrator_worktree_preserve_test.go:1` — above `package core`

```text
// orchestrator_worktree_preserve_test.go — RED contract for ship-failure
// worktree preservation (ADR-0039 §8, operator-approved 2026-06-07).
//
// D10 incident (domain-campaign cycles 7/12): a ship abort BEFORE the
// worktree-commit step let the deferred cycle cleanup prune the worktree with
// uncommitted, substantively-PASS work inside — cycle 7's work was lost
// entirely. The rule under test: when the cycle ends with an unresolved
// ship failure, the worktree is PRESERVED for recovery; it is only cleaned
// up when the cycle's ship eventually succeeds (or the operator runs an
// explicit `evolve cycle reset`).
```

### `go/internal/core/orchestrator_worktree_preserve_test.go:86` — above `func TestPreserveOnVerdict(t *testing.T) {`

```text
// TestPreserveOnVerdict pins inbox preserve-worktree-on-verdict-fail: a cycle
// that COMPLETES with a FAIL verdict (audit FAIL → retro → end, err==nil)
// leaves the builder's work UNCOMMITTED in the worktree. Pruning it discards
// salvageable work (ADR-0046 Layer 2 was built and lost twice this way, cycles
// 306/307). Only a FAIL verdict warrants completion-time preservation; PASS
// (shipped to main) and other outcomes clean as before.
```

### `go/internal/core/phase.go:93` — above `Worktree string 'json:"worktree"'`

```text
// Worktree is the per-cycle git worktree — the SHIPPED-TREE root. A
// source-writing phase edits here, and an EGPS predicate's `go test`
// compiles here (acssuite runs predicates with cwd=Worktree while `.evolve/`
// still resolves to ProjectRoot via EVOLVE_PROJECT_ROOT — the intentional
// dual root, issue #9 + #12). Since CB.1 it is set for EVERY phase (cwd
// isolation is universal; write permission stays on the worktreePhase
// axis); empty only when provisioning failed — the degraded mode where
// phases run against ProjectRoot. Keep ProjectRoot (data) and Worktree
// (code) distinct: collapsing them reintroduces the cycle-190 "predicate
// ran against main" bug.
```

### `go/internal/core/phase.go:108` — above `WorktreeReadOnly bool 'json:"worktree_read_only,omitempty"'`

```text
// WorktreeReadOnly marks a phase that is not a declared source writer
// (registry writes_source=false — audit, adversarial-review, retro, …). The
// phase runner fences the worktree around such a dispatch: the tree the
// phase hands downstream is byte-identical to the tree it was given, and
// any write the agent made in between is reported and undone (the
// cycle-1603/1604/1605 class — an auditor's mutation probes rewrote the
// builder's material files and the sealed build explanation no longer
// matched the tree). Derived from the orchestrator's write-permission
// predicate at dispatch, never set by a phase.
```

### `go/internal/core/phase.go:138` — above `BuildPlan string 'json:"build_plan,omitempty"'`

```text
// BuildPlan is the build phase's upstream build-plan.md body, served via the
// typed envelope instead of an ad-hoc disk read inside the phase (ADR-0050
// Phase 3.7). Populated once at the dispatch seam, and only at
// EVOLVE_PHASE_IO>=advisory with the planner enabled; empty at off/shadow so
// the build phase reads disk exactly as before (byte-identical dispatch).
```

### `go/internal/core/phase.go:156` — above `Input phaseio.PhaseInput 'json:"-"'`

```text
// Input is the unified typed phase-I/O envelope (ADR-0050 Phase 3.10). It is
// assembled once at the dispatch seam and ONLY at EVOLVE_PHASE_IO>=enforce; the
// zero value at off/shadow/advisory keeps dispatch byte-identical (no phase
// consumes it until the enforce cutover migrates readers off the Context map).
// Excluded from JSON: PhaseInput seals its channels behind unexported fields so
// it is not wire-serializable — the subprocess override path keeps using
// Context, and in-core phases read this envelope when composing prompts before
// any subprocess launch.
```

### `go/internal/core/phase.go:170` — above `ComposePhases bool 'json:"compose_phases,omitempty"'`

```text
// ComposePhases signals that phases are being run via `evolve compose`
// (ad-hoc composition bypassing the state machine). The kernel guard
// downgrades from BLOCK to WARN when this field is true. Replaces the
// retired EVOLVE_COMPOSE_PHASES env signal (cycle-10 flag-reduction).
```

### `go/internal/core/phase.go:196` — above `ModelRoutingCLI  string 'json:"model_routing_cli,omitempty"'`

```text
// ModelRoutingCLI/ModelRoutingTier (cycle-440 MR4a/c) carry the whole-cycle
// plan's clamped {cli,tier} proposal for THIS phase — but ONLY when
// config.ModelRouting==ModelRoutingAuto (advisory computes and logs the
// same clamped proposal to phase-plan.json but leaves these empty; static
// never sets them). The runner applies them as a SOFT dispatch overlay
// (llmroute.ApplySoftOverlay): promote to chain primary without discarding
// the profile's fallback chain, so a benched/failing proposal still falls
// back via the ordinary cli-health chain. Empty on every pre-cycle-440
// dispatch and on any mode other than auto — omitempty keeps a zero-value
// PhaseRequest's JSON shape byte-identical.
```

### `go/internal/core/phase.go:208` — above `BudgetScale float64 'json:"budget_scale,omitempty"'`

```text
// BudgetScale (ADR-0076 slice A) is the difficulty multiplier for this
// dispatch's artifact budget, set for the BUILD phase from the cycle's
// digest-resolved size estimate. 0/1 = unscaled (byte-identical dispatch).
```

### `go/internal/core/phase.go:223` — above `BootMS      int64        'json:"boot_ms,omitempty"'`

```text
// BootMS is the cold REPL-boot latency carried up from the bridge
// (ADR-0043 A0) — the slice of DurationMS that was pure dispatch overhead
// (tmux new-session → prompt marker), not model think time. 0 = no cold boot.
```

### `go/internal/core/phase.go:246` — above `ModelSource   string 'json:"model_source,omitempty"'`

```text
// ModelSource + ResolvedModel (T3, cycle-463) record WHICH resolution path
// won this phase's dispatch — "profile" (neither pin nor advisor overlay),
// "pin" (an operator policy.json pin, which always wins), or "advisor" (the
// MR4c soft overlay applied) — plus the concrete resolved model/tier. Closes
// the P3 observability gap: per-phase model provenance was previously
// unrecorded, so a dormant advisor overlay had no artifact proving it.
```

### `go/internal/core/phase.go:264` — above `type PersonaProber interface {`

```text
// PersonaProber is implemented by phase runners that can answer, before any
// dispatch, whether their persona doc exists. A nil error means available; an
// error wrapping ErrAgentDocMissing means the doc is absent — the same
// sentinel the dispatch path raises, so the plan-time exclusion and the skip
// classifier agree on the cause (2026-09-09 token-waste root cause #2). Any
// other error is reported but does not exclude the phase: only a KNOWN
// absence is deterministic enough to remove a phase from the menu.
```

### `go/internal/core/phase_advisor.go:3` — above `import (`

```text
// phase_advisor.go — unit 04 (ADR-0103, design decomposition/04-advisor.md):
// core's seam onto the phase advisor. The brain lives in internal/core/advisor
// behind a leaf-owned Launcher port; this file keeps the host struct the
// composition root builds with its four options, projects the core Bridge
// onto that port ONCE, and holds the facades every old caller keeps its
// spelling through — the router ports, replay, the span/identity aliases,
// resume's plan parser, the judge's and the adjudicator's span scanner, the
// task-recall cap, the failure digest's atomic writer, the orchestrator's
// bench projection and the git reader the seam injects — plus the test
// facades the ACS-named and protected core tests call by name.
```

### `go/internal/core/phase_advisor.go:30` — above `type PhaseAdvisor struct {`

```text
// PhaseAdvisor is the bridge-backed DynamicLLM brain. It satisfies two router
// ports: router.Proposer (Propose — the per-transition "insert this optional
// phase?" advice) and router.Planner (Plan — the upfront whole-cycle run/skip
// plan, ADR-0024 §2). Both ask an LLM via the core.Bridge port given the
// objective digest. All output is ADVISORY: the pure router.Route() clamp pass
// re-validates it against the kernel floor (mandatory spine, TDD-pin,
// ship-needs-real-audit), so a hallucinated or malformed proposal can never
// weaken the ship guarantee. Any failure is returned as an error and the caller
// degrades cleanly to the deterministic static path — "model proposes, kernel
// disposes", fail-safe to the floor. Since ADR-0103 unit 04 it is the host of
// the advisor leaf: the options mutate the identity, the depth guard and the
// Center before the ONE construction.
```

### `go/internal/core/phase_advisor.go:44` — above `identity AgentIdentity`

```text
// ADR-0052 WS1-S1: the shared dispatch identity (cli/model/profile/persona/label)
```

### `go/internal/core/phase_advisor.go:77` — above `func WithDepthCheck(fn func(env map[string]string) bool) PhaseAdvisorOption {`

```text
// WithDepthCheck injects the recursion-depth guard (defense-in-depth, ADR-0052 §4.3).
// When fn returns true for the dispatch env, the launch errors before the bridge.
// Pass AdvisorDepthExceeded for production behavior; nil (the zero-field default) skips the check.
```

### `go/internal/core/phase_advisor.go:97` — above `func WithAdvisorSignals(c *signalcenter.Center) PhaseAdvisorOption {`

```text
// WithAdvisorSignals hands the root's Signal Center to the advisor (ADR-0103
// unit 04): the brain reports its faults as advisor.warning through it. The
// composition root builds the Center before the advisor in the same function,
// so this is honest construction-time DI — the ONE declared root spelling change.
```

### `go/internal/core/phase_advisor.go:203` — above `func (p *PhaseAdvisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {`

```text
// RePlan is the post-scout re-plan (ADR-0052 WS1-S3), called in shadow every
// cycle by the orchestrator under the cfg.RouterReplan dial.
```

### `go/internal/core/phase_advisor.go:217` — above `type AdvisorSpan = advisor.Span`

```text
// AdvisorSpan is the OTel-GenAI decision span (ADR-0052 WS3-S3) — the leaf's
// Span, kept under its core spelling for the routing explain command.
```

### `go/internal/core/phase_advisor_chainattempt_test.go:9` — above `func TestBridgeRequestOf_MarksTheAdvisorsAttempt(t *testing.T) {`

```text
// TestBridgeRequestOf_MarksTheAdvisorsAttempt — the advisor walks its own CLI
// chain (llmroute.Dispatch, cycle-435); each launch it hands the bridge is one
// attempt of that walk and must say so, or a chain-walking bridge handle would
// resolve and walk the chain a second time around it.
```

### `go/internal/core/phase_advisor_clihealth_test.go:3` — above `import (`

```text
// Slice-4 contract: the routing advisor SEES the environment (cycle-283 — the
// advisor kept planning codex-routed inserts all night while codex was
// quota-walled, because RouteInput carried zero CLI state). The orchestrator
// projects the cli-health store's active benches here; the prompt section
// they render into is the advisor leaf's (ADR-0103 unit 04).
```

### `go/internal/core/phase_advisor_guard_test.go:9` — above `func TestMintConfigsFrom_RejectsAdvisorRoleMint(t *testing.T) {`

```text
// TestMintConfigsFrom_RejectsAdvisorRoleMint pins the WS1-S2 recursion guard
// (ADR-0052 D1, primary defense): the advisor proposes phases for the executed
// spine, NEVER another router/advisor — a brain minting a brain is the one
// recursion the layering forbids. A mint whose name is a reserved control-plane
// identity (router/evolve-router/advisor/failure-advisor, case-insensitive) is
// dropped from the registered set with an observable reason; legitimate mints
// pass through untouched.
```

### `go/internal/core/phase_advisor_guard_test.go:40` — above `func TestAdvisorLaunch_DepthGuard(t *testing.T) {`

```text
// TestAdvisorLaunch_DepthGuard pins the WS1-S2 secondary recursion guard.
// EVOLVE_ADVISOR_DEPTH was retired in cycle-10 (flag-reduction campaign);
// AdvisorDepthExceeded is now dormant (always false). The primary guard
// (reservedAdvisorNames denylist, tested above) remains the live defense.
// This test verifies WithDepthCheck still compiles and that the dormant guard
// never blocks the advisor path.
```

### `go/internal/core/phase_advisor_identity_test.go:5` — above `func TestAdvisorDispatch_DefaultsAndOverrides(t *testing.T) {`

```text
// TestAdvisorDispatch_DefaultsAndOverrides pins the WS1-S1 AgentIdentity value
// object (ADR-0052): the default identity a fresh PhaseAdvisor dispatches under,
// and that the functional options populate that ONE identity (which advisorLaunch
// then reads to build BridgeRequest). It asserts the value object directly —
// TestPhaseAdvisor_DispatchWiringFlowsToBridge already covers the field→bridge
// flow — so the single-source identity both control-plane advisors now share is
// locked against drift.
```

### `go/internal/core/phase_advisor_mint_test.go:73` — above `func TestPhaseAdvisor_PlanMintCarriesSelectMetadata(t *testing.T) {`

```text
// TestPhaseAdvisor_PlanMintCarriesSelectMetadata proves the minter satisfies the
// catalog SELECT-metadata contract itself (cycle-1275): description/when_to_use
// supplied in the advisor's mint block land on the minted PhaseSpec, which is
// exactly what TestPhaseCatalog_OptionalPhasesHaveSelectMetadata reads. Before
// this, every minted phase reached the catalog metadata-less and the gate was
// satisfied by padding metadataAllowlist after the fact (#404, #406).
```

### `go/internal/core/phase_advisor_pins_test.go:3` — above `import (`

```text
// phase_advisor_pins_test.go — ADR-0103 unit 04 §6 step 1: the pre-move pins
// on the phase advisor, green on the pre-extraction code and each proven red
// against its named mutant before a line moved. They pin the seam the
// composition root, the orchestrator and the ledger read: the error texts
// cyclerun.go prints, the depth guard's refusal, capture-before-parse, the
// prompt/launch/capture goldens and the stderr + stream silence on the happy
// paths. After the move they run through core's seam (the one wired
// construction + the Bridge→Launcher projection); the leaf carries its own
// copies against its exported spellings, and the pure-render goldens (the
// routing/plan prompts, the capture artifacts) live only in the leaf.
```

### `go/internal/core/phase_advisor_replay_modelrouting_test.go:66` — above `func TestReplayPlanFromResponse_LegacyResponseByteIdenticalDispatch(t *testing.T) {`

```text
// TestReplayPlanFromResponse_LegacyResponseByteIdenticalDispatch (T4 AC5,
// EDGE): replaying a legacy plan response — the exact cycle-459 shape,
// {phase,run,justification} only — must yield ZERO model-routing clamps and
// ZERO dispatch overlay: the simulated dispatch chain and model equal the
// profile-static baseline exactly. No overlay, no rejection artifact
// entries, for either phase entry.
```

### `go/internal/core/phase_advisor_seam_test.go:3` — above `import (`

```text
// phase_advisor_seam_test.go — ADR-0103 unit 04 §6 step 4: the core seam —
// the ONE wired construction, the Bridge→Launcher projection, the facades the
// composition root, resume, the judge/adjudicator, the failure digest and the
// by-name tests keep, the git reader the seam injects, the Center reaching
// the brain through the option, and the unit-05 handoff pin.
```

### `go/internal/core/phase_advisor_test.go:19` — above `tokens     TokenUsage`

```text
// ADR-0103 unit 04: the span golden threads token usage
```

### `go/internal/core/phase_advisor_test.go:303` — above `func TestPhaseAdvisor_PlanPromptUsesAbsoluteArtifactPath(t *testing.T) {`

```text
// TestPhaseAdvisor_PlanPromptUsesAbsoluteArtifactPath pins the fix for the
// cycle-210 degradation: composePlanPrompt must instruct the agent to write the
// ABSOLUTE workspace artifact path (the same path advisorLaunch tells the bridge
// to watch), not a relative "routing-plan.json". Under claude-tmux the REPL cwd
// is NOT the workspace (it varies per cycle — repo root / worktree), so a relative
// write lands where the bridge never polls → 600s artifact-timeout → degrade to
// static. The absolute path makes the file land where the bridge watches,
// regardless of REPL cwd.
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:17` — above `func TestComposePlanPrompt_ElicitsTierAndCLI(t *testing.T) {`

```text
// TestComposePlanPrompt_ElicitsTierAndCLI (T1 AC1): the PRODUCTION persona
// path (composePlanPrompt with a non-empty persona) must show the SAME
// optional per-phase {cli,tier} schema example that today only lives in the
// legacy buildPlanPrompt fallback (#293 divergence — phase_advisor.go:484-489
// never ran when identity.Persona != ""). The example must attach cli/tier to
// an EXISTING phase entry (no "mint" block on that entry), proving the
// elicitation is single-sourced across both prompt-assembly paths rather than
// re-forked. RED today: composePlanPrompt's output is persona + cycle context
// only — it never calls the schema/example writer buildPlanPrompt uses.
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:34` — above `tierIdx := strings.Index(got, '"tier":"balanced"')`

```text
// The cli/tier examples must appear on an EXISTING-phase entry, not only
// inside a "mint" block — find the JSON object carrying "tier" and assert
// it has no "mint" key alongside it (a mint-only elicitation would leave
// existing-phase proposals undocumented, reproducing the #293 gap in a new
// shape).
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:54` — above `func TestComposePlanPrompt_RendersOperatorModelPolicy(t *testing.T) {`

```text
// TestComposePlanPrompt_RendersOperatorModelPolicy (T1 AC1): the persona-path
// prompt must carry the operator's model-tier policy guidance — deep for
// judgment-heavy phases, fast confined to mechanical-only phases — so the
// advisor never proposes fast for a phase that writes source or renders a
// verdict. RED today: this guidance exists nowhere in composePlanPrompt's
// output (buildPlanPrompt doesn't carry it either — it is genuinely new
// prose, not a #293-style migration).
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:165` — above `func TestParsePhasePlan_AbsentCLITierFieldsStayEmpty(t *testing.T) {`

```text
// TestParsePhasePlan_AbsentCLITierFieldsStayEmpty (T1 AC4, NEGATIVE — degrade
// path byte-identical): an advisor response shaped like the pre-change
// (cycle-459-era) wire format — {phase,run,justification} only, no cli/tier
// keys at all — must parse to entries whose CLI/Tier are empty, so nothing
// downstream ever applies a dispatch overlay for them (llmroute.
// ApplySoftOverlay is gated on non-empty CLI/Tier). This is the regression
// pin a gaming fake ("add the schema text but break absent-field parsing")
// must not be able to defeat. Expected pre-existing GREEN: parsePhasePlan
// already zero-values unset JSON fields; this test locks that fact in as
// part of the T1 contract.
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:190` — above `func TestSanitizeAdvisorTier_RejectsHighAndRawModel(t *testing.T) {`

```text
// TestSanitizeAdvisorTier_RejectsHighAndRawModel (T1 AC5, EDGE — tier
// vocabulary confinement): an advisor response entry proposing "high" or a
// raw model name must never propagate past sanitizeAdvisorTier — only the
// canonical tier vocabulary survives. Originally named
// TestSanitizeAdvisorTier_RejectsHighTopAndRawModel and asserted "top" was
// rejected too, back when fast/balanced/deep were the only three canonical
// tiers (phase_advisor.go:857-864 era). cycle-516 (task
// advisor-tier-vocab-add-top) intentionally widens the vocabulary to
// fast/balanced/deep/top — modelcatalog.CanonicalTiers already treats "top"
// as canonical — so "top" moves to the ACCEPTED table in
// TestSanitizeAdvisorTier (phase_advisor_tier_test.go). This test keeps
// confining every OTHER non-canonical string, so a future change still
// can't silently widen the vocabulary further than intended.
```

### `go/internal/core/phase_advisor_tier_elicitation_test.go:217` — above `func realRouterPersona(t *testing.T) (frontmatter map[string]any, body string) {`

```text
// --- cycle-476 T1: advisor-real-persona-liveness-golden ---
//
// The missing test class scout root-caused: EVERY other advisor-prompt test
// injects a STUB persona (WithPersona("PERSONA BODY")) and so is structurally
// blind to the SHIPPED agents/evolve-router.md, whose own existing-phase
// response-schema example (line 35) omits {cli,tier} and — appearing BEFORE and
// competing with the Go-appended {cli,tier} example (writePlanResponseSchema) —
// makes the composed prompt show two conflicting schemas. LLMs mimic the
// earliest/most-authoritative example, so the optional tier fields are emitted
// intermittently. These goldens load the REAL persona exactly as production does.
```

### `go/internal/core/phase_advisor_tier_test.go:1` — above `package core`

```text
// phase_advisor_tier_test.go — cycle-516 task `advisor-tier-vocab-add-top`
// (RED).
//
// modelcatalog/refresh.go added "top" to CanonicalTiers (fast/balanced/deep/
// top — "the frontier tier, default when a profile/advisor is silent"), but
// sanitizeAdvisorTier — the SOLE gate for advisor-emitted tiers
// (phase_advisor.go:919-925) — still hard-codes the old 3-tier switch and
// silently drops "top" to "" (loosening it to accept extra strings would
// violate the driver_agnostic_model_routing invariant, so this test also
// pins that garbage is still rejected).
```

### `go/internal/core/phase_bindings.go:30` — above `o.recordPhaseBinding(ctx, phase, in)`

```text
// FAIL included since cycle-1571 H3: without a binding, ship's lookup
// has nothing for this run and the FAIL verdict is invisible to the
// gate built to enforce it — the binding is how ship reads THIS run's
// report and returns the honest VERDICT_FAIL terminal. SKIPPED stays
// excluded (no audit ran, no artifact to bind).
```

### `go/internal/core/phase_bindings.go:74` — above `worktreeBase string`

```text
// worktreeBase is the worktree's own base commit (CycleState.WorktreeBaseSHA)
// — the ONLY correct operand for the Put-site fresh-base guard. projectRoot
// HEAD at audit time diverges under fleet concurrency (a sibling ship
// advances main mid-cycle to a commit the lane worktree may not contain),
// which either mismatches the operands or fails the resolution open and
// re-admits the shared fresh-base cache write ADR-0048 exists to prevent.
```

### `go/internal/core/phase_bindings.go:106` — above `func (o *Orchestrator) recordAuditBinding(ctx context.Context, cycle int, projectRoot, workspace, worktree, worktreeBase…`

```text
// recordAuditBinding writes the rich auditor ledger entry that ship's
// audit-binding (verify.go findLatestAudit / verifyAuditBinding) requires:
// role=auditor, kind=agent_subprocess, with git_head + tree_state_sha +
// artifact_path/sha256. Without it the Go orchestrator recorded audit only as
// kind:phase (no binding fields), so ship fell back to an ancient bash-era
// auditor entry and every cycle failed AUDIT_BINDING_HEAD_MOVED (root cause,
// 2026-05-29). tree_state_sha is sha256(`git diff HEAD`) — byte-identical to
// ship's computeTreeStateSHA so the bind matches. Best-effort: a failure WARNs
// and is swallowed; ship then fails loudly on the missing/stale binding rather
// than shipping unbound.
```

### `go/internal/core/phase_bindings.go:122` — above `worktreeTree := worktreeContentSHA(ctx, projectRoot, worktree)`

```text
// Worktree CHANGES tree: stage tracked changes and write a tree object = the
// tree ship will commit from the builder-declared index. This is what the auditor
// SHOULD bind (it audited the worktree's working changes); its persona binds
// HEAD^{tree} = the unchanged base, which can never equal the changes-commit
// tree → INTEGRITY_TREE_DRIFT every cycle (cycle-152). Ship prefers this
// over the auditor's comment. Best-effort: empty ⇒ ship falls back to the
// auditor's value. No commit is made (write-tree only); ship re-stages anyway.
```

### `go/internal/core/phase_bindings.go:170` — above `if !verdictcache.Reusable(verdict) {`

```text
// ADR-0048 Slice B: project this verdict into the content-addressed verdict
// cache, keyed by the SAME worktree tree SHA the binding records. The cache
// is a projection of the audit binding (single-source), not a second record.
// Best-effort + advisory: an empty key (no worktree content identity) or a
// write failure never blocks the cycle — a future lookup miss just costs a
// full re-run.
//
// The fresh-base guard is the SAME predicate the pre-loop shadow probe reads
// (verdictcache.ProbeEligible), fed the SAME base operand: the worktree's
// own base commit, never projectRoot HEAD at audit time (salvage-review
// HIGH-1 — a sibling ship advancing main mid-cycle diverged the operands
// and re-admitted the shared fresh-base write). An untouched worktree's
// tree identity is shared by every sibling lane at that base, so recording
// under it would contaminate their lookups.
//
// Write-side fail-CLOSED, deliberately asymmetric with the read side's
// fail-open: a skipped Lookup costs one shadow log line, but a poisoned
// Put sits in the shared store for every future consumer. No base
// identity ⇒ no cache write.
// Ledger-bound above, but only a REUSABLE verdict is cache-projected: the
// cache exists to let identical known-good trees skip a re-audit, so a FAIL
// (a rejection) and a SKIPPED (no audit ran) must never enter it. The
// vocabulary lives in verdictcache.Reusable, the same predicate the store's
// write guard and the RUNG 0 composition snapshot use, so the three sites
// cannot drift apart (the ProbeEligible precedent, ADR-0048 cycle-1488).
```

### `go/internal/core/phase_bindings.go:220` — above `func worktreeContentSHA(ctx context.Context, projectRoot, worktree string) string {`

```text
// worktreeContentSHA stages tracked worktree changes while retaining already
// staged new files, then writes a tree object (git write-tree) — the content
// identity of the cycle's declared changes. Unstaged untracked files are not
// adopted. It is the SINGLE source for both the audit binding's WorktreeTreeSHA
// (recordAuditBinding) and the ADR-0048 Slice B verdict-cache key, so the value
// recorded and the value looked up are computed identically. Best-effort:
// returns "" when worktree is empty or git fails (callers degrade — ship falls
// back to the auditor comment; the cache simply does not record/match).
```

### `go/internal/core/phase_bindings.go:265` — above `func (o *Orchestrator) recordBuildBinding(ctx context.Context, cycle int, projectRoot, workspace string) {`

```text
// recordBuildBinding writes the builder's provenance ledger entry — role=builder,
// kind=agent_subprocess — that BOTH the red-team predicate rt-001-ledger-role-
// completeness AND the auditor's Ledger-Verification check require as proof the
// builder actually ran. The orchestrator's per-phase entry is role="build" (the
// PHASE name), not "builder" (the AGENT name), and recent cycles no longer get a
// bridge-written per-agent entry — so a cycle that goes through FORMAL audit (vs the
// inline build-commit path that bypasses it) false-FAILed provenance with "no
// role:builder entry" even though the build ran (cycle-181 / issue #13). Mirrors
// recordAuditBinding (role=auditor); best-effort + loud WARN, never blocks the cycle.
```

### `go/internal/core/phase_bindings.go:346` — above `func normalizeWorktreeToBase(ctx context.Context, worktree, baseSHA string) {`

```text
// normalizeWorktreeToBase soft-resets the worktree to baseSHA so any commits a
// builder made during the build phase become PENDING changes again. The builder
// is instructed to `git add -A && git commit -m "… [worktree-build]"`
// (agents/evolve-builder.md:235) for crash-safety, but the auditor
// (agents/evolve-auditor.md:57: "Run `git diff HEAD`") and the orchestrator's
// audit-binding (recordAuditBinding: sha256(`git diff HEAD`)) both inspect the
// PENDING diff — which is empty after a commit. agy/Gemini followed the commit
// instruction literally and every cycle's work was discarded as "tree lacks the
// files". Resetting --soft to the cycle base re-exposes the work to `git diff
// HEAD` without changing the auditor prompt or the security binding. See
// docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md (Option C).
//
// Best-effort: any failure WARNs and leaves the worktree untouched (audit then
// inspects whatever state exists); it NEVER aborts the cycle. No-op when HEAD is
// already at baseSHA (the builder left changes uncommitted — the historical
// Claude-builder path), so opting in is byte-identical for non-committing builders.
```

### `go/internal/core/phase_bindings.go:394` — above `func (o *Orchestrator) normalizeBuildWorktree(ctx context.Context, completed Phase, cs CycleState, projectRoot string) {`

```text
// normalizeBuildWorktree applies two post-phase normalizations to the active
// worktree, shared by RunCycle and RunCycleFromPhase (resume). The whole
// function is a no-op when there is no active worktree.
//
//  1. Build-commit soft-reset (cycle-156): runs ONLY after PhaseBuild —
//     re-exposes a committing builder's work as pending for audit's
//     `git diff HEAD`. Base comes from the persisted CycleState.WorktreeBaseSHA.
//  2. gofmt -s normalize (cycle-352): runs after EVERY worktree phase, because
//     tdd, build, AND test-amplification all author .go that the audit gofmt
//     gate scans. Cheap no-op when the worktree is already clean.
```

### `go/internal/core/phase_bindings.go:411` — above `if completed == PhaseBuild {`

```text
// The build-commit soft-reset (cycle-156) is build-ONLY: it re-exposes a
// committing builder's work as pending for audit's `git diff HEAD`.
```

### `go/internal/core/phase_bindings.go:416` — above `normalizeBuildGofmt(cs.ActiveWorktree)`

```text
// The gofmt -s normalize runs after EVERY worktree phase, not just build:
// tdd, build, AND test-amplification all author .go, and the audit gofmt
// gate scans the whole worktree. Cycle 352: test-amplification left
// modeltier_amp_test.go dirty AFTER the build-only normalize, re-failing the
// gate. Cheap no-op when the worktree is already clean.
```

### `go/internal/core/phase_bindings.go:450` — above `func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {`

```text
// normalizeDerivedProjections regenerates each GENERATED projection whose
// source-of-truth this cycle changed (e.g. control-flags.md after a registry
// edit), in the build worktree, BEFORE the audit/docs gate inspects it. Like
// build-gofmt, regenerating a derived projection is deterministic work that must
// NOT depend on the LLM builder remembering: a flag cycle edits registry_table.go
// but the builder routinely leaves the control-flags.md projection stale
// (cycle-11 H1), which the docs/flags gate then correctly FAILs. This closes that
// class at the source — the gate stays the backstop. Best-effort; never aborts.
//
// Timing/integrity: this runs in the BUILD iteration of recordAndBranch (after
// emitPhaseBindings(PhaseBuild), which does NOT compute a tree SHA) and stages the
// regenerated file. The AUDIT iteration's emitPhaseBindings(PhaseAudit) then runs
// worktreeContentSHA (git add -u + write-tree), binding the already-staged
// regenerated projection — so committed_tree == audit_bound_tree holds (no
// CodeIntegrityTreeDrift).
```

### `go/internal/core/phase_bindings.go:493` — above `func ChangedWorktreePaths(ctx context.Context, worktree string) []string {`

```text
// ChangedWorktreePaths is the exported projection of changedWorktreePaths for
// consumers outside this package — today `evolve phase verify build`, which
// feeds the set to deliverable.VerifyBuildWithChangedPaths so the agent's
// self-check judges the SAME diff the host-side docs-floor reviewer does
// (build_floor_reviewer.go:136). A projection, not a second implementation:
// re-deriving the diff in internal/cli/phasecmd would put two answers to "what
// did this cycle change?" in the tree and let the gate and the self-check drift
// (the ADR-0034 no-drift invariant). Fail-open semantics are inherited — a path
// that is not a git repo yields no paths rather than an error.
```

### `go/internal/core/phase_bindings.go:506` — above `func normalizeBuildGofmt(worktree string) {`

```text
// normalizeBuildGofmt applies the deterministic `gofmt -w -s` normalization to
// the build worktree's Go module BEFORE the audit gofmt gate inspects it.
// Formatting is deterministic work and must not depend on the LLM builder
// remembering to run it: when the builder leaves a non-gofmt-s-clean file
// (comment alignment, etc.), the audit gate correctly FAILs the whole cycle
// (cycles 339-341, 350, 351). This closes that class at the source — the gate
// stays the backstop, but the builder's formatting lapses are normalized away
// first. Best-effort: a gofmt failure WARNs and lets the audit gate catch
// anything that slips through; it NEVER aborts the cycle. Scoped to the same
// module dir the audit gate scans (codequality.ModuleDir), so the two cannot
// disagree; the worktree is cut from CI-clean main, so only this cycle's
// changed files are ever dirty.
```

### `go/internal/core/phase_bindings.go:532`

```text
// porcelainDirtySet returns the set of paths `git status --porcelain` reports
// dirty in dir — tracked-modified AND untracked. Captured for the main tree at
// cycle start so recoverBuildLeak only touches paths the BUILD introduced, never
// the operator's pre-existing uncommitted work. (The tree-diff guard's
// `git diff --name-only HEAD` baseline is tracked-only and misses untracked, so
// it can't serve this purpose — see the cycle-160 incident.)
```

### `go/internal/core/phase_bindings_fail_verdict_test.go:3` — above `package core`

```text
// Cycle-1571 H3 producer half: a FAIL audit verdict emitted NO auditor ledger
// binding (phase_bindings.go guarded to PASS|WARN), so ship's findLatestAudit
// had nothing for this run and fell back to a FOREIGN run's entry — the FAIL
// verdict was the very thing that removed the gate's ability to see it. These
// pins flip the producer: FAIL records the same rich auditor binding (so ship
// reads THIS cycle's host rejection rather than another run's evidence),
// while the verdict-cache projection stays PASS|WARN-only (the cache exists to
// skip re-audits of known-good trees; caching FAIL would change its consumers'
// contract, and the WARN control below proves the guard is what's observed,
// not an environmental skip).
```

### `go/internal/core/phase_bindings_fail_verdict_test.go:26` — above `if err := os.WriteFile(filepath.Join(wt, "f.txt"), []byte("delta"), 0o644); err != nil {`

```text
// Dirty the worktree so the content tree differs from the base tree —
// ProbeEligible would be TRUE, so only the verdict guard can skip the Put.
// The delta MUST be a tracked modification: since cycle-1594's declared-
// content contract, worktreeContentSHA stages `git add -u`, so an untracked
// file is residue that keeps base identity — it would make this pin pass
// vacuously via the fresh-base guard instead of the verdict guard.
```

### `go/internal/core/phase_bindings_gofmt_test.go:42` — above `func TestNormalizeBuildWorktree_GofmtsAfterNonBuildPhase(t *testing.T) {`

```text
// Cycle 352: test-amplification authored a .go file AFTER the build-only
// normalize, re-failing the audit gofmt gate. The gofmt normalize must run
// after EVERY worktree phase (here a non-build phase), not just build.
```

### `go/internal/core/phase_bindings_graduation.go:3` — above `import (`

```text
// phase_bindings_graduation.go — deterministic build-entry graduation guard
// (inbox new-package-graduation-buildentry-gate, 3rd recurrence: cycles
// 575/587/652). A package NEW this cycle cannot be in go/.apicover-enforce yet,
// so the touched∩enforced apicover gate never inspects it — the recurring
// warnship_apicover_ci_gap blind spot. The audit-side half
// (apicoverNewPackageGraduationDefault) landed 2026-07-07; this is the
// build-entry half: the same predicate at the post-build seam, but
// abort-capable — unlike buildSelfCheck (WARN-only, NEVER aborts, see
// phase_bindings_selfcheck.go), an ungraduated new package FAILS the build
// phase with an explicit abort_reason, because graduation is a hard shipping
// obligation the builder itself must satisfy, not a diagnostic for audit to
// re-discover two attempts later.
```

### `go/internal/core/phase_bindings_graduation.go:65` — above `func packageHasProductionGoFiles(worktree, pkg string) bool {`

```text
// packageHasProductionGoFiles delegates to the SHARED graduation predicate —
// see ciparity.PackageDirHasProductionGoFiles for the test-only-package
// rationale (cycles 1223/1224/1228). One predicate, two seams, no disagreement.
```

### `go/internal/core/phase_bindings_graduation_selfservice_test.go:3` — above `import (`

```text
// phase_bindings_graduation_selfservice_test.go — RED contract for the
// SELF-SERVICE half of the build-entry graduation floor (inbox
// acs-apicover-enrollment-in-builder-brief, 0.94).
//
// Evidence: batch-21 HALTED at cycle-1218 on the identical-fingerprint ceiling
// because THREE lanes' build phases aborted on the same cause — a new internal
// package absent from go/.apicover-enforce. The floor's refusal was correct and
// already NAMED the offending packages, but the reason only gestured at the
// obligation ("add its pattern line and an apicover_named_test.go"), so each
// lane had to re-derive the doctrine: which file, which line, which path.
//
// The fix is a message that is SELF-SERVING, not merely correct: it must emit
// the EXACT two edits per package, so a builder that hits it can comply
// verbatim. Naming the class is not enough — the abort text IS the remediation
// interface (the builder never reads ADR-0069).
//
// Scope boundary: this changes the message only. The detection contract
// (TestBuildGraduationCheck) is unchanged and must keep passing.
```

### `go/internal/core/phase_bindings_graduation_selfservice_test.go:82` — above `func TestBuildGraduationCheck_MessageDistinguishesTheTwoApicoverGates(t *testing.T) {`

```text
// TestBuildGraduationCheck_MessageDistinguishesTheTwoApicoverGates — ADR-0069
// documents TWO apicover gates: the per-cycle ACS coverage gate and the
// repo-wide enforce list. ONLY the second needs the enrollment line, and a
// message that blurs them sends the builder to edit ACS predicates it is
// role-gated out of. The reason must say which gate it is.
```

### `go/internal/core/phase_bindings_graduation_test.go:3` — above `import (`

```text
// phase_bindings_graduation_test.go — cycle-675 RED contract for the
// build-entry new-package graduation guard (inbox
// new-package-graduation-buildentry-gate, 3rd recurrence: cycles 575/587/652).
//
// The audit-side half (apicoverNewPackageGraduationDefault, audit.go:248) landed
// 2026-07-07; this encodes the missing build-entry half: a deterministic
// post-build check that FAILS the build phase — explicit abort_reason, unlike
// buildSelfCheck's WARN-only contract — when a changed go/internal/<pkg> is new
// this cycle and absent from go/.apicover-enforce.
//
// Contract under test (Builder implements; tests must not be modified):
//
//	buildGraduationCheck(ctx context.Context, worktree string) string
//
// returns "" when nothing is ungraduated (or the check cannot apply: empty
// worktree, no enforce file — fail-open mirroring the audit default), else a
// non-empty abort reason naming each ungraduated package and the
// .apicover-enforce graduation obligation. Detection reuses
// ciparity.NewUngraduatedPackages over the worktree's changed set; a package
// whose directory no longer exists in the worktree (delete/rename) is NOT new
// and must never be flagged (AC3).
```

### `go/internal/core/phase_bindings_graduation_test.go:82` — above `name: "new-ungraduated-package-fails",`

```text
// AC1 positive: reproduces cycle-652 — a brand-new internal package
// with no .apicover-enforce entry must fail the build phase, and the
// reason must name the package AND the graduation obligation.
```

### `go/internal/core/phase_bindings_graduation_test.go:94` — above `name: "test-only-package-does-not-abort",`

```text
// cycle-1223/1224/1228 halt class (batch of 2026-08-02): the tdd
// phase RED-first mints a package containing ONLY a _test.go file.
// A test-only package has ZERO exported production symbols, so the
// repo-wide apicover gate this graduation protects cannot fire on
// it (CI's own enforce step: "apicover finds 0 exported symbols in
// the test-only acs packages and passes") — the obligation is
// vacuous, but the abort was fatal AND unreachable by any in-cycle
// correction, so the next cycle re-minted the same package and the
// identical-fingerprint breaker halted the batch at 3.
```

### `go/internal/core/phase_bindings_selfcheck.go:3` — above `import (`

```text
// phase_bindings_selfcheck.go — deterministic post-build self-check (the
// false-green backstop). A builder can delete an env read, break a pre-existing
// UNIT test in a changed package, "pass" its own ACS check (which does not run
// that package's unit tests), and hand off a green build-report — the regression
// then only surfaces at audit, two attempts later (cycle-2 / w1-config-singletons
// M1). Running the changed packages' unit tests here, deterministically, records
// ground-truth so the builder's self-report cannot lie and the audit/retro have
// the exact failing tests. UNIT tests only (no -tags integration): the
// env-dependent tmux/REPL integration tests are intentionally excluded so a real
// regression fails the check while a flaky live-launch test does not.
//
// Like build-gofmt and build-derived-regen this is deterministic work that must
// not depend on the LLM builder remembering; best-effort and NEVER aborts —
// audit stays the verdict authority (build's only legal successor is audit).
```

### `go/internal/core/phase_bindings_selfcheck_failloud_test.go:3` — above `import (`

```text
// phase_bindings_selfcheck_failloud_test.go — RED contract for the fable5
// deep-scan finding selfcheck-breaker-fail-loud (inbox weight 0.91, cycle-618
// scout), core.phase_bindings_selfcheck.go half.
//
// Context. writeBuildSelfCheckArtifact persists the build self-check's failing
// packages so the audit/toolchain gate can read exact ground truth. Today every
// I/O step silently swallows its error:
//
//	if err := os.MkdirAll(dir, 0o755); err != nil {
//	    return
//	}
//	data, err := json.MarshalIndent(fails, "", "  ")
//	if err != nil {
//	    return
//	}
//	_ = os.WriteFile(dst, append(data, '\n'), 0o644)
//
// A write failure (disk full, permission denied, path collision) leaves the
// gate reading a STALE or MISSING artifact with zero operator-visible signal —
// the exact silent-skip shape this repo's fail-loud rule (AGENTS.md Rule 12)
// forbids. The self-check itself must stay fail-OPEN (never abort build on a
// persistence error — audit is the backstop, per the file's own header
// comment), but the failure must be WARNed to stderr the same way the sibling
// WARN two lines above it already is.
//
// RED today: writeBuildSelfCheckArtifact discards the MkdirAll error with no
// stderr output, so the assertion below fails for the right reason (empty
// captured stderr) rather than a compile error — this task's fix is a
// behavioral (fail-loud) change, not a new API.
```

### `go/internal/core/phase_bindings_selfcheck_test.go:58` — above `wt := initGitWorktree(t)`

```text
// A prior failed attempt wrote the artifact; the retry's changed package now
// PASSES. The artifact must be cleared so the toolchain gate (which reads it)
// does not loop forever on a stale failure. Regression for the gate-hardening
// after relaunch cycle 12 shipped vet-failing code.
```

### `go/internal/core/phase_bindings_staging_scope_test.go:3` — above `import (`

```text
// phase_bindings_staging_scope_test.go — cycle-1594 RED contract for
// `gitignore-staging-sweep`.
//
// The defect (reproduced in .evolve/runs/cycle-1594/bug-reproduction-report.md):
// worktreeContentSHA stages the whole worktree with `git add -A` purely to
// compute a content identity. Any unrelated untracked file that happens to sit
// in the lane — a bug-reproduction reproducer, a regenerated coverage artifact,
// a minted phase stub, another agent's scratch file — is therefore adopted into
// the tree the audit binding records (phase_bindings.go:129) and into the
// ADR-0048 verdict-cache key. The binding then attests a tree the auditor never
// reviewed, and the cache is keyed on foreign content. This is the mechanism
// behind the wave-3 cycle-1572/1574 staging rejections amortised into the
// `gitignore-staging-sweep` inbox record.
//
// The contract these tests pin is a SELECTION boundary, deliberately expressed
// in observable git terms so the Builder keeps design freedom (no new exported
// symbol, parameter, or carrier is frozen here — the current signature
// `worktreeContentSHA(ctx, "", worktree)` stays valid):
//
//	declared     = content already in the lane's git index (tracked files, plus
//	               anything the builder explicitly `git add`ed) + tracked
//	               modifications on disk
//	NOT declared = untracked files nobody staged
//
// Adversarial axes:
//   - negative  : ExcludesUnrelatedUntrackedResidue / ResidueOnlyWorktreeKeepsBaseIdentity
//                 (residue must NOT ride) and CapturesUnstagedTrackedModification
//                 (the anti-degenerate guard — a fix that just returns HEAD^{tree}
//                 passes the two exclusion pins and dies here, because a binding
//                 identical to the base is exactly the INTEGRITY_TREE_DRIFT /
//                 fresh-base cache collision this identity exists to avoid).
//   - edge      : the residue-ONLY worktree, where no declared work exists at
//                 all and the empty selection must not silently adopt residue.
//   - semantic  : exclusion, retention, tracked-modification capture and
//                 production reachability are four distinct behaviors.
//
// Every pin drives real git against a real repository and asserts on the real
// tree object; none greps source.
```

### `go/internal/core/phase_bindings_staging_scope_test.go:111` — above `writeFile(t, filepath.Join(repo, "foreign-residue.txt"), "must not enter the audit binding\n")`

```text
// Residue: an unrelated untracked file. Not gitignored — that is the whole
// point; .gitignore already excludes the families it knows about, and the
// classes that burned cycles 1572/1574 were precisely the ones it did not.
```

### `go/internal/core/phase_bindings_staging_scope_test.go:150` — above `func TestWorktreeContentSHA_CapturesUnstagedTrackedModification(t *testing.T) {`

```text
// AC3 (regression guard / anti-degenerate). A modification to a TRACKED file
// that was never `git add`ed must still be captured. Without this, the cheapest
// way to pass AC1 and AC4 — return the base tree, or stage nothing at all —
// would look correct while making every binding identical to its base: the
// INTEGRITY_TREE_DRIFT class (cycle-152) and the verdict-cache fresh-base
// collision ADR-0048's ProbeEligible guard exists to prevent.
```

### `go/internal/core/phase_diagnostics_seal_test.go:16` — above `type diagnosticFailRunner struct {`

```text
// phase_diagnostics_seal_test.go — a phase's own FAIL reason must reach the seal.
//
// Live incidents (two-wave health batch, 2026-09-12/13): cycles 1634 and 1636
// both FAILed at triage because Classify's protected-surface admission check
// refused a top_n card naming a control-plane file. Classify returned the
// reason as an error-severity diagnostic; the C1 outcome record carried no
// field for it, so phase-timing.json held only `verdict: FAIL` and the seal
// wrote "phase triage: verdict FAIL with no recorded abort reason (phase-infra
// class)" — a deterministic, reasoned rejection paged as infrastructure, twice,
// with the actual reason persisted nowhere. Floor phases never hit this: their
// diagnostics ride a side channel (persistFloorFailReasons). Every other phase
// dropped them at the chokepoint.
```

### `go/internal/core/phase_diagnostics_seal_test.go:114` — above `func TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail(t *testing.T) {`

```text
// The C1 chokepoint is the ONE producer: phaseOutcomeFrom relays the phase's
// diagnostics, recordPhaseOutcome writes them to the record and emits ONE
// phase.outcome; the root's WARN-filtered stderr sink renders a reasoned FAIL
// in the one line format (ADR-0101 S1) — the chokepoint itself prints nothing.
// A PASS carrying warnings is recorded (the durable trail) and stays off stderr.
```

### `go/internal/core/phase_input_dispatch_test.go:23` — above `func TestDispatch_PhaseInput_ZeroValueBelowEnforce(t *testing.T) {`

```text
// TestDispatch_PhaseInput_ZeroValueBelowEnforce is the Slice-0 byte-identity
// proof (ADR-0050 Phase 3.10): at EVOLVE_PHASE_IO off/shadow/advisory the typed
// PhaseInput envelope is NEVER assembled onto the PhaseRequest, so every
// dispatched request carries the zero PhaseInput. The shadow stage still runs its
// comparison (3.4) — that is unchanged — but the dispatch *field* stays zero until
// enforce, which is what keeps the live loop byte-identical pre-cutover.
//
// The check uses reflect.DeepEqual against a struct literal (PhaseInput seals
// channels behind unexported fields, so it is not ==-comparable). This is valid
// because the guard lives in the CODE PATH, not the comparison: assemblePhaseIO
// returns early (before any NewPhaseInput call) below enforce, so the field is the
// literal zero value here — not a NewPhaseInput(empty) result that merely looks
// zero.
```

### `go/internal/core/phase_input_dispatch_test.go:147` — above `func TestPlanCycle_CarryoverSummary_NoRawWriterBelowEnforce(t *testing.T) {`

```text
// TestPlanCycle_CarryoverSummary_NoRawWriterBelowEnforce (cycle 1632, task
// tokenopt-handoff-digests): the cycle-1593 retry minted a raw
// ctxSnap["carryover_summary"] writer from state.CarryoverTodos and put the
// full uncapped backlog (218,480 measured bytes) into every below-enforce
// triage prompt — a path that carried ZERO carryover bytes before that diff
// (cycle-1593 audit H2/M1). This pins the true baseline: with a populated
// carryover backlog in state and no carryover_summary in the request Context,
// NO dispatched request carries the key at StageOff/StageShadow (and the typed
// PhaseInput stays zero, per the byte-identity proof above). The cap is a
// guard on what arrives; it is not a licence to create the bytes it bounds.
```

### `go/internal/core/phase_input_envelope.go:13` — above `func readUpstreamBuildPlan(stage config.Stage, phase Phase, phaseEnables map[string]string, workspace string) string {`

```text
// readUpstreamBuildPlan returns the build phase's upstream build-plan.md body to
// serve via the typed PhaseRequest.BuildPlan envelope (ADR-0050 Phase 3.7). It is
// the dispatch-seam relocation of the ad-hoc os.ReadFile the build phase did
// inside ComposePrompt: same file, read once at the seam so the phase no longer
// reaches to disk for an upstream artifact.
//
// It returns non-empty ONLY at EVOLVE_PHASE_IO>=advisory, for the build phase,
// with the build-planner enabled and a readable file. Every other case returns
// "" so the build phase falls back to its original disk read — byte-identical
// dispatch at off/shadow (the campaign's master no-regression invariant). The
// read is best-effort: a missing/unreadable file yields "" (the phase's own
// fallback then also reads nothing), never an error.
```

### `go/internal/core/phase_input_envelope.go:35` — above `func (cr *cycleRun) assemblePhaseIO(phase Phase, phaseWorktree string, phaseCtx map[string]string) phaseio.PhaseInput {`

```text
// assemblePhaseIO is the dispatch-seam phase-I/O hook (ADR-0050 Phase 3.4 shadow
// comparison + Phase 3.10 enforce input), invoked only at EVOLVE_PHASE_IO>=shadow.
// It owns the SINGLE router.Digest of the upstream and then:
//   - runs the shadow comparison (the typed-vs-legacy divergence tripwire); and
//   - at >=enforce, returns the authoritative typed PhaseInput the phase consumes
//     in place of the legacy Context map.
//
// Below enforce it returns the zero PhaseInput, so the dispatch field stays
// zero-valued — byte-identical to the pre-cutover loop. A digest failure is
// non-fatal: the shadow comparison is skipped (it needs the digest) and, at
// enforce, the input is still assembled from the phase context with an empty
// Upstream view so the envelope remains authoritative.
```

### `go/internal/core/phase_input_envelope_test.go:11` — above `func TestReadUpstreamBuildPlan(t *testing.T) {`

```text
// TestReadUpstreamBuildPlan pins the dispatch-seam population rule for ADR-0050
// Phase 3.7: the build phase's upstream build-plan body is served via the
// envelope ONLY at advisory+ with the planner enabled (via WorkflowPolicy.PhaseEnables)
// and a readable file; every other case returns "".
```

### `go/internal/core/phase_judge.go:13` — above `type JudgeVerdict struct {`

```text
// JudgeVerdict is the LLM-as-judge route-quality grade (ADR-0052 WS4-S3). It is
// advisory telemetry, NOT a trust boundary: Score is in [0,1] on a successful
// grade, or the sentinel -1 ("no opinion") when the judge could not produce a
// valid grade. A caller treats -1 as non-blocking.
```

### `go/internal/core/phase_judge.go:27` — above `type PlanJudge struct {`

```text
// PlanJudge is the optional LLM-as-judge that scores an emitted routing plan
// against the cycle goal (ADR-0052 WS4-S3), behind EVOLVE_ROUTING_JUDGE and
// strictly off the build path. It is deliberately NOT a router.Proposer or
// router.Planner — it only READS a plan and EMITS a score, so router.Select can
// never wire it as a routing brain. That, plus dispatching under the non-router
// "judge" agent label, is the structural recursion guard: a judge call has no
// path back into planning, so it needs no mint denylist. Per D2 the judge is
// the FAST/cheap tier (deep reasoning is reserved for the confidence-critical
// Plan/RePlan).
```

### `go/internal/core/phase_judge_test.go:37` — above `func TestRouteQualityJudge_ScoresPlanAgainstGoal(t *testing.T) {`

```text
// WS4-S3 (ADR-0052): GradePlan scores a routing plan against the goal and
// returns a typed verdict, dispatched as a NON-router agent on the fast tier.
```

### `go/internal/core/phase_judge_test.go:72` — above `if fb.gotReq.CLI != "claude-tmux" {`

```text
// D2: the judge is the FAST/cheap tier (deep is reserved for Plan/RePlan), and
// it uses the uniform artifact-completion contract — locks the dispatch shape
// against drift (a flip to opus would defeat D2; a wrong artifact path is the
// cycle-210 silent-timeout class the sibling advisor test pins).
```

### `go/internal/core/phase_timing_composition_test.go:103` — above `func TestRecorder_LiteralOrchestratorGetsTheNullObjectRecorderOnce(t *testing.T) {`

```text
// Unit 01 (ADR-0103): an Orchestrator assembled as a literal — the shape this
// package's tests keep — and a cycleRun with no orchestrator at all both get
// the Null-Object recorder on first use, once, so every path still has ONE
// writer; NewOrchestrator builds the wired recorder eagerly.
```

### `go/internal/core/phase_timing_composition_test.go:138` — above `func TestRecorder_SeesASignalCenterAppliedAfterConstruction(t *testing.T) {`

```text
// Unit 01 (ADR-0103), architecture review MEDIUM-1: the recorder reads the
// orchestrator's Center through an accessor, so WithSignalCenter applied
// after NewOrchestrator (the shape resume_lifecycle_test.go uses) still
// routes the recorder's own warnings — a snapshot at construction would
// have kept them silent for the orchestrator's whole life.
```

### `go/internal/core/phase_timing_single_writer_test.go:10` — above `func TestPhaseTimings_SingleWriter(t *testing.T) {`

```text
// TestPhaseTimings_SingleWriter is a source-scan guard, deliberately modelled
// on cycleoutcome's TestLaneScopeProjection_SingleWireShapeDeclaration — the
// guard that caught this diff's own SSOT violation.
//
// The defect it protects has now regressed twice. `phase-timing.json` must be
// composed and written through ONE path (cycleRun.flushPhaseTimings, which
// caches so the composition happens exactly once per cycle). A second raw
// caller re-appends this invocation's entries to a log that already contains
// them, so the durable log and the dossier built from it disagree — silently,
// and only on the resume path, which is exactly where nobody looks.
//
// Unit 01 (ADR-0103) moved the writer to the exported
// (*outcome.Recorder).WritePhaseTimings, which any package could construct
// and call, so the scan covers the WHOLE module (every non-test .go file
// outside the unit that defines it), not just this directory: Go's visibility
// no longer makes "one writer" structurally true, this guard does.
//
// If you are adding a legitimate writer: route it through flushPhaseTimings on
// the cycle's OWN cycleRun. If you truly need another, this test is the place
// to argue for it.
```

### `go/internal/core/phaseio_shadow.go:17` — above `type phaseIOMismatch struct {`

```text
// phaseIOMismatch records one field where the assembled typed Upstream view
// (phaseio.Handoffs) diverged from the legacy routing digest during the
// EVOLVE_PHASE_IO shadow stage (ADR-0050 Phase 3.4).
```

### `go/internal/core/phaseio_shadow.go:106` — above `func assembleCycleInputs(ctx map[string]string) phaseio.CycleInputs {`

```text
// assembleCycleInputs builds the typed CycleInputs from the legacy per-phase
// Context map (ctxSnap/phaseCtx), reading the SAME keys the phases read today
// (ADR-0050 Phase 3.5/3.6). Note challengeToken is camelCase (the live Context
// key), not the snake_case wire-JSON field name; carryover reads the legacy
// carryover_summary key (triage), not carryover.
```

### `go/internal/core/phaseio_shadow_test.go:163` — above `func TestScout_Strategy_FromCycleInputs_MatchesContext(t *testing.T) {`

```text
// ── ADR-0050 Phase 3.6: named per-reader equivalence anchors ──────────────────
// One named test per remaining Context reader (scout/triage/intent/ship/debugger),
// each pinning that the typed CycleInputs/ErrorContext getter reproduces the exact
// legacy req.Context key the live phase still reads — the per-field key-drift guard
// the soak relies on. Each test asserts ONLY its own field so a failure localizes
// to the reader it names (the comparator's drift behaviour, incl. clean and
// diverging paths, is proven once over ALL fields in
// TestCompareCycleInputsShadow_KeyDrift). The phase code keeps reading req.Context
// until the 3.10 enforce cutover; these prove the typed envelope is a faithful
// shadow of every reader before that cutover is permitted.
//
// These tests run unconditionally — they exercise the assembler directly. At
// EVOLVE_PHASE_IO=off the production shadow hook (emitPhaseIOShadow, gated at
// cyclerun_dispatch.go:112) is never called, but the unit tests remain always-on.
```

### `go/internal/core/phaseio_shadow_test.go:354` — above `func overCapCtxValue(lead byte) string {`

```text
// ---------------------------------------------------------------------------
// Cycle 1632 RED contract (tokenopt-handoff-digests): the shadow comparator
// must be CAP-AWARE but not cap-BLIND. NewCycleInputs now bounds the two
// free-text fields through phaseio.CapField, so comparing the raw legacy value
// against the typed getter byte-for-byte would flag every over-cap dispatch as
// a false mismatch AND leak the full uncapped text into the ledger message
// (cycle-1593 round-2 H1). The fix runs the legacy "want" through the SAME
// phaseio.CapField — which must still surface a typed value that genuinely
// differs. Frozen (doNotModifyTests).
// ---------------------------------------------------------------------------
```

### `go/internal/core/phaseoutputs_signal.go:3` — above `import (`

```text
// phaseoutputs_signal.go — the per-cycle phase-output survey, emitted into the
// unified signal stream (abnormal-events.jsonl) at cycle finalize.
//
// This lives in CORE, not the loop CLI, because the first monitored wave
// proved the placement wrong the other way: fleet-dispatched lanes never
// traverse cmd_loop's single-loop post-cycle block, so cycles 1452/1453
// completed with zero phase-outputs-surveyed events — the exact silent
// non-reporting the survey exists to end. The finalize defers in RunCycle and
// RunCycleFromPhase run for every cycle on every dispatch topology, abort
// paths included, so the emission cannot be skipped by how the cycle was
// launched. A cycle that aborts and later resumes emits once per finalize —
// two events on one stream, both true at their moment; consumers take the
// LAST event per cycle (documented on EventPhaseOutputsSurveyed).
//
// Thin adapter: reading goes through internal/phaseoutputs' shared loaders and
// every decision (gap, chain state, abnormality) is made in that pure layer.
```

### `go/internal/core/phaseoutputs_signal_test.go:3` — above `import (`

```text
// phaseoutputs_signal_test.go — the WIRING proof for the per-cycle
// phase-output survey signal. The first monitored wave (cycles 1452/1453)
// completed with ZERO phase-outputs-surveyed events because the emission
// lived on cmd_loop's single-loop path, which fleet lanes never traverse.
// This test drives a REAL RunCycle through the orchestrator and asserts the
// event reached the workspace's unified stream — so the emission can never
// again silently depend on how the cycle was dispatched.
```

### `go/internal/core/ports.go:58` — above `type (`

```text
// (Legacy speculative Observer interface removed in cycle-122 Fix 3 /
// ADR-0030 — it was scaffolding with zero callers. The live interface
// is in observer.go with the Start(ctx, phase, req)→cancel shape that
// the orchestrator actually wires from RunCycle.)
```

### `go/internal/core/ports.go:83` — above `type LedgerEntry struct {`

```text
// LedgerEntry is one .jsonl line in .evolve/ledger.jsonl.
//
// The cycle field has a custom unmarshaler that accepts int (canonical)
// or string (legacy manual entries, e.g. "manual-release-v10.16.0").
// On-disk bytes are never rewritten — doing so would cascade SHA256
// hash-chain breaks through every subsequent entry.
```

### `go/internal/core/ports.go:103` — above `WorktreeTreeSHA string   'json:"worktree_tree_sha,omitempty"'`

```text
// WorktreeTreeSHA is the git tree SHA of the per-cycle worktree's WORKING
// state (all changes staged) at audit time — the tree ship will commit.
// Written by the orchestrator's audit-binding entry so ship's pre/post-merge
// tree-drift check binds to the audited CHANGES, not the auditor's
// HEAD^{tree} (which is the unchanged base in the worktree flow, cycle-152).
```

### `go/internal/core/ports.go:205` — above `if !bytes.Equal(trimmed, []byte("null")) {`

```text
// JSON null ≡ absent (live pin: 15 "cycle":null inbox-lifecycle
// entries written 2026-07-22 are permanent append-only history —
// rejecting them hard-fails every full-ledger iteration forever).
```

### `go/internal/core/ports.go:265` — above `SecondaryArtifacts []string 'json:"secondary_artifacts,omitempty"'`

```text
// SecondaryArtifacts are additional deliverables the phase contract
// requires beyond ArtifactPath (absolute paths). The completion detector
// holds phase-complete until every one EXISTS (existence only — the
// settle window stays primary-only, respecting the cycle-1210/1212 race
// design); the artifact-timeout final poll still completes without them,
// and the phase gate then reports the absence loudly. Closes the
// single-artifact cutoff class (retro disposition.json since <=1382;
// audit defect-dispositions.json, cycles 1397-1429; plan Phase B).
```

### `go/internal/core/ports.go:274` — above `Completion string 'json:"completion,omitempty"'`

```text
// Completion selects the phase-completion contract (ADR-0027): "" /
// "artifact" = poll the artifact file (default); "stdout" = complete on
// REPL-idle for agents that print their answer and write no file (the
// router/advisor). Only the *-tmux drivers honor it; others ignore it.
```

### `go/internal/core/ports.go:297` — above `BudgetScale float64 'json:"budget_scale,omitempty"'`

```text
// BudgetScale scales the launch's artifact-wait budget (ADR-0076 slice A:
// difficulty-conditioned budgets — a large cycle's build gets a longer
// deadline). 0 or 1 = unscaled; <1 never shrinks. The engine applies it to
// the per-agent policy base (or the builtin when the map has no entry).
```

### `go/internal/core/ports.go:339` — above `SessionName string 'json:"session_name,omitempty"'`

```text
// SessionName, when non-empty, pins the tmux session to a deterministic,
// caller-controlled name (claude-tmux/*-tmux only; headless drivers ignore
// it). The swarm harness (ADR-0032) sets this and REGISTERS the name before
// calling Launch, so a worker cancelled mid-spawn can still be reaped by name
// (closing the orphan-on-cancel gap). A named session is preserved by the
// driver's own cleanup — the caller owns teardown.
```

### `go/internal/core/ports.go:356` — above `BootMS int64 'json:"boot_ms,omitempty"'`

```text
// BootMS is the cold-boot latency the tmux-REPL driver spent from
// tmux new-session to the REPL prompt marker appearing — pure dispatch
// overhead, paid before the prompt is delivered (ADR-0043 A0). 0 when no
// cold boot happened (a resumed/warm named session, or a headless driver).
```

### `go/internal/core/ports_ledger_test.go:26` — above `func TestLedgerEntry_Unmarshal_StringCycle(t *testing.T) {`

```text
// TestLedgerEntry_Unmarshal_StringCycle covers the legacy malformed
// entry at .evolve/ledger.jsonl line 1741 from the v10.16.0 manual
// release. cycle="manual-release-v10.16.0" must parse without error and
// land in CycleLabel; Cycle stays 0.
```

### `go/internal/core/ports_ledger_test.go:201` — above `func TestLedgerEntry_NullCycleAbsorbed(t *testing.T) {`

```text
// TestLedgerEntry_NullCycleAbsorbed — live corruption pin (2026-07-22): 15
// inbox-lifecycle promote entries carry "cycle":null, and append-only history
// cannot be rewritten — the defensive unmarshal must absorb null exactly like
// an absent field (Cycle=0, no label, no error), or every full-ledger
// iteration (evolve ledger verify, TestIter_RealLedger_NoStringCycleError)
// hard-fails forever at seq 80361.
```

### `go/internal/core/post_build_repin.go:13` — above `var postBuildRepinProvenanceFn = defaultPostBuildRepinProvenance`

```text
// post_build_repin.go — cycle 636, task ship-sha-repin-after-build.
//
// The cycle-514 boot healer auto-re-pins expected_ship_sha ONLY at boot; a
// legitimate within-version rebuild of go/bin/evolve between boots leaves a
// frozen pin that denied the terminal ship gate on 8 consecutive cycles
// (625->634, SELF_SHA_TAMPERED). This closes the class: the orchestrator re-pins
// immediately AFTER a successful build phase, reusing the SAME provenance-gated
// primitive the boot healer uses (phaseintegrity.RepinIfDrifted) so the two
// paths can never diverge.
```

### `go/internal/core/post_build_repin_test.go:3` — above `import (`

```text
// post_build_repin_test.go — RED tests (cycle 636, task ship-sha-repin-after-build).
//
// The cycle-514 boot healer auto-repins expected_ship_sha ONLY at boot; a
// legitimate within-version rebuild of go/bin/evolve between boots leaves a frozen
// pin that denied the ship gate on 8 consecutive cycles (625->634,
// SELF_SHA_TAMPERED, repair_outcome=declined). This closes the class: the orchestrator
// re-pins immediately AFTER a successful build phase, reusing the SAME
// provenance-gated primitive the boot healer uses (phaseintegrity.RepinIfDrifted).
//
// Contract the Builder implements (TDD-defined seam; mirrors cmd/evolve's proven
// shipRepinProvenanceFn boot-repin seam so the decision stays git-free/
// deterministic under test):
//
//	// postBuildRepinProvenanceFn resolves the running binary's build-commit + the
//	// provenance predicate authorizing a post-build auto-repin. Production:
//	// version.Commit() + a `git merge-base --is-ancestor <commit> HEAD` closure
//	// over projectRoot — identical to cmd/evolve's defaultShipRepinProvenance.
//	var postBuildRepinProvenanceFn = defaultPostBuildRepinProvenance
//	func defaultPostBuildRepinProvenance(projectRoot string) (commit string, prov phaseintegrity.ProvenanceVerified)
//
//	// repinShipSHAAfterBuild re-pins <projectRoot>/.evolve/state.json:expected_ship_sha
//	// to the freshly-built <projectRoot>/go/bin/evolve after a successful build, via
//	// phaseintegrity.RepinIfDrifted(statePath, binPath, commit, "", prov). NEVER
//	// operator-authorized (unattended). Fail-open: a refusal/error WARNs and returns
//	// a zero RepinResult; the ship gate stays the backstop. Wire it into
//	// recordAndBranch's `next == PhaseBuild` branch (see test-report.md WIR-1 checklist).
//	func repinShipSHAAfterBuild(projectRoot string) phaseintegrity.RepinResult
//
// RED now: repinShipSHAAfterBuild + postBuildRepinProvenanceFn undefined -> package
// core test build fails. Do NOT modify this file — implement the seam + wire it.
```

### `go/internal/core/prescription_carryover_test.go:3` — above `import (`

```text
// prescription_carryover_test.go — RED contract for cycle-1375 task
// `prescription-carryover-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91; scout-report.md Task 2, fleet lane
// audit-warn-prescriptions-unenforced).
//
// DEFECT BEING FIXED: audit's emitDefectLedger already tags a WARN-carried
// structured Prescription[] entry as an OPEN "PRESCRIPTION: <text>" row in
// <workspace>/defect-ledger.json (defect_ledger.go:134-198), and
// reconcileAgainstAncestor already blocks PASS on any unaccounted OPEN row —
// but ONLY when the current cycle is formally bound as a continuation of the
// ledger-holding cycle. An ordinary next-lane ship (the common case — triage
// picks lanes by content, not by ledger lineage) never reconciles, so the
// prescription is silently dropped at ship. cycle-1258's own prescription
// ("materialize .evolve/evals/artifact-ready-crosspoll-debounce.md, git add -f
// past .gitignore") is the live, still-unrepaired instance (Task 1, this cycle).
//
// FIX (Builder authors go/internal/core/prescription_carryover.go + wires the
// call site in finalizeCycle beside MergeWorkspaceCarryover, per
// carryover_merge.go:26-40's existing pattern): a cycle-terminal hook that, if
// <workspace>/defect-ledger.json exists, tolerant-decodes its entries, keeps
// only Status=="OPEN" rows whose Text carries the exact "PRESCRIPTION: " prefix
// emitDefectLedger already writes (defect_ledger.go:181), and merges one
// CarryoverTodo per surviving entry into state.CarryoverTodos (dedup by id via
// the existing mergeCarryoverTodos — idempotent on re-entry, same idiom as
// MergeWorkspaceCarryover). This makes every future WARN prescription reach the
// next scout through the already-mandatory carryoverTodos flow, regardless of
// continuation binding.
//
// These tests are authored by the TDD engineer and are RED now (they will not
// even compile until MergeWorkspacePrescriptionCarryover exists — a valid RED
// per the compile-failure rule). The Builder must make them GREEN by adding
// production code ONLY; it must NOT modify this file.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : TestRunCycle_MergesPrescriptionCarryoverIntoState — the
//     real terminal path (finalizeCycle) persists the prescription todo. This is
//     the load-bearing anti-no-op signal: a helper that exists but is never
//     wired into the terminal hook leaves F3's gap open and FAILS here.
//   - Negative        : TestMergeWorkspacePrescriptionCarryover_FixedAndDeferredEntriesAreNotCarriedOver —
//     a FIXED or DEFERRED prescription must NOT surface as a new carryover (it
//     is already resolved; re-surfacing it would nag forever).
//   - Semantic         : TestMergeWorkspacePrescriptionCarryover_NonPrescriptionOpenEntryIsIgnored —
//     an OPEN structured-defect row that is NOT tagged "PRESCRIPTION: " (i.e.
//     the ordinary reconcile-gate defect, already covered by
//     reconcileAgainstAncestor) must be left alone — this hook is scoped to the
//     PRESCRIPTION: subset only, never a blanket ledger-to-carryover mirror.
//   - Edge/OOD         : TestMergeWorkspacePrescriptionCarryover_AbsentLedgerIsNoOp,
//     TestMergeWorkspacePrescriptionCarryover_MalformedLedgerWarnsNotFails — a
//     missing or corrupt ledger must never abort the cycle-terminal hook.
//   - Semantic         : TestMergeWorkspacePrescriptionCarryover_DedupesById — a
//     second finalize over the same still-OPEN ledger must not duplicate the
//     carryover row (crash-resume / double-invocation idempotence).
```

### `go/internal/core/prune_superseded_orphans.go:1` — above `package core`

```text
// prune_superseded_orphans.go — housekeeping walker for stale orphan `cycle-*`
// branches (cycle 962, dependent on carryforward-real-cherrypick-filter).
//
// Over a fleet campaign the local ref namespace accumulates `cycle-*` branches
// whose work has already landed on the base under a different sha. Left alone
// they silently regrow the carry-forward candidate backlog. This walker reuses
// the Task-1 supersession screen (refSuperseded) to flag those functional
// duplicates and prune the stale refs — honoring
// verify_remote_pr_before_branch_delete: a branch with an open PR (or remote
// presence) is flagged but NEVER deleted. Distinct, not-yet-landed
// (different-goal) orphans are left untouched.
```

### `go/internal/core/prune_superseded_orphans_test.go:3` — above `import (`

```text
// prune_superseded_orphans_test.go — fast-tier coverage for the orphan-prune
// walker (cycle 962). Reuses the seamGit seam from
// carryforward_filter_test.go (same package). The assertions probe the two
// safety-critical intents: a superseded branch WITHOUT an open PR is deleted,
// but a superseded branch WITH an open PR is flagged-but-KEPT
// (verify_remote_pr_before_branch_delete) — never silently deleted.
```

### `go/internal/core/quota_defer_retro_skip_test.go:13` — above `func allFamiliesExhaustedRun(t *testing.T) (*fakeStorage, *fakeLedger, map[Phase]PhaseRunner, error) {`

```text
// RED contract for cycle-1585 task `quota-defer-short-circuits-retro`
// (instinct inst-L1582a), SUPERSEDED by the cycle-1587 fix for
// `pipeline-defect-pipeline-blocker-cycle1582`
// (.evolve/evals/pipeline-defect-pipeline-blocker-cycle1582.md). The
// all-families-quota-exhausted abort (cyclerun_dispatch.go:264-287) is a
// DEFERRED, resumable checkpoint, not a diagnosed phase failure: the
// quota-boundary checkpoint is already written and the loop exits rc=5 to be
// resumed after the quota resets.
//
// cycle-1585 only fixed the "no retro dispatch" half (recordFailureLearning
// still called recordFailedApproachState unconditionally before its
// ErrAllFamiliesExhausted short-circuit, so a FailedRecord + P0 carryover todo
// were still appended on every quota wall — the cycle-1582 dossier root
// cause: it queued a spurious `cycle-N-failed-scout` P0 todo that competed
// with real work every time the fleet hit quota). This RED contract closes
// the remaining half: the guard must short-circuit BEFORE
// recordFailedApproachState on the all-families-exhausted arm, not merely
// before the retro dispatch that follows it.
//
//	AC1 — retro runner is never called on the all-families-exhausted path
//	AC2 — CycleState.Phase / ActiveAgent are never mutated to "retro" there
//	AC3 — state.FailedAt / carryover-todo bookkeeping is NOT recorded for the
//	      all-families-exhausted arm (superseded from cycle-1585's "still
//	      recorded" expectation — a DEFERRED checkpoint is not a FAIL)
//	AC4 — a genuine non-quota failure still dispatches retro exactly once and
//	      still records FailedAt (unaffected by the guard)
//	AC5 — a multiply-wrapped sentinel is still matched (errors.Is, not ==),
//	      and skips bookkeeping too
//	AC6 — a single-family exit=85 attempt with a differently-shaped sibling
//	      (not the all-85 signature) is NOT the exhaustion arm at all, so it
//	      must learn exactly as before: unaffected by the guard
```

### `go/internal/core/quota_defer_retro_skip_test.go:76` — above `func TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro(t *testing.T) {`

```text
// AC1 + AC3: the counting fake proves the PRODUCTION dispatch chain never
// reaches the retro runner, AND — superseding cycle-1585's "bookkeeping still
// recorded" expectation — that a DEFERRED quota checkpoint records no
// FailedRecord and no P0 carryover todo either: it is not a diagnosed failure,
// so failure-learning must not fire at all on this arm (cycle-1582 dossier:
// the spurious `cycle-N-failed-scout` P0 todo competed with real work on every
// quota wall). NOT t.Parallel: swaps the package-level
// QuotaBoundaryCheckpointer hook.
```

### `go/internal/core/quota_defer_retro_skip_test.go:104` — above `func TestDispatch_AllFamiliesExhausted_NoFailureLearning(t *testing.T) {`

```text
// Regression reproducer for cycle-1582: all-family quota exhaustion is a
// DEFERRED checkpoint, not a diagnosed phase failure. The current tree skips
// retro but still creates failed-at and P0 carryover state before that guard.
```

### `go/internal/core/quota_defer_retro_skip_test.go:253` — above `func TestRecordFailureLearning_ShipExhausted_PreservesCoherenceCarrier(t *testing.T) {`

```text
// TestRecordFailureLearning_ShipExhausted_PreservesCoherenceCarrier ensures a
// deferred ship quota checkpoint remains out of failure learning while keeping
// its real dispatch error available to the ADR-0072 coherence floor.
```

### `go/internal/core/quota_exhaustion.go:3` — above `const abortReasonAllFamiliesExhausted = "all-families-exhausted"`

```text
// abortReasonAllFamiliesExhausted prefixes the C1 abort_reason recorded when a
// phase exhausts its retry budget with exit=85 (provider quota) on EVERY
// attempt. cyclehealth.ClassifyOutcome matches this prefix (via the C1 JSON
// record — the cross-package contract) to classify the cycle DEFERRED instead
// of FAILED_EXPLAINED, and the loop stops resumable (rc=5) instead of burning
// the next cycle into the same drained quota (cycle-656 D2).
```

### `go/internal/core/quota_exhaustion.go:11` — above `func allFamiliesQuotaExhausted(attemptExits []int) bool {`

```text
// allFamiliesQuotaExhausted reports whether an exhausted retry budget is the
// all-CLI-families quota-terminal case (cycle-656): every attempt's bridge
// exit code was 85. The bridge alternates families across attempts (cycle-393
// failover), so with PhaseMaxAttempts >= 2 an all-85 sequence means every
// family in the fallback chain was tried and is drained. A single attempt
// proves nothing about the chain, so len < 2 is never terminal — single-family
// 85 with a healthy sibling keeps the existing failover behavior.
//
// Since WS-876 (llmroute.DispatchTiered) the quota-terminal error feeding this
// classification is only reachable after the LOWEST tier in the plan's tier
// fallback chain (Plan.Tiers, floored at the phase's ModelTierEnvelope.Min /
// the universal "balanced" floor) is also exhausted — an all-85 sequence now
// means every family at every allowed tier is drained, not just at the
// initially resolved tier.
```

### `go/internal/core/quota_exhaustion_test.go:53` — above `func TestRunCycle_AllFamilies85_CheckpointsAndDefers(t *testing.T) {`

```text
// Regression for cycle-656 D2: every attempt exits 85 → the dispatch seam must
// write a quota-likely checkpoint, record the C1 abort reason + ledger entry,
// and abort with ErrAllFamiliesExhausted — not fail forward.
// NOT t.Parallel: it swaps the package-level QuotaBoundaryCheckpointer hook.
```

### `go/internal/core/quota_pause.go:13` — above `cr.emitQuotaPaused(next, phaseErr)`

```text
// ADR-0101 S2a: the pause is the quota.paused signal (WARN); the sink
// renders it — this seam is the one both dispatch roots reach.
```

### `go/internal/core/quota_pause.go:30` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt,`

```text
// ADR-0044 C1: record the abort reason with the DEFERRED
// prefix so cyclehealth classifies the cycle DEFERRED.
```

### `go/internal/core/recovery_test.go:3` — above `import (`

```text
// recovery_test.go — PA-DDK DDK-6 (ADR-0060): the recovery successor TARGETS are
// config-driven (retrospective in the registry, debugger in the control seam);
// the decision POLICY stays Go. Tests load config via the fixture and assert the
// resolver consults it, with the literal as the fallback.
```

### `go/internal/core/recurrence_generic_escalation_c662_test.go:3` — above `import (`

```text
// recurrence_generic_escalation_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index gap G3, consumer side. escalateRetroReason
// (decision_branch.go) upgrades a deterministic "proceed:" retro reason to an
// "adapt:"-with-escalation reason once a pattern recurs (count>=2). Post-backfill
// the corpus is dominated by generic noise (operator-reset x96, loop-fatal x62);
// escalation MUST consume NON-generic patterns only, or every FAIL cycle would
// be force-escalated by the noise floor.
//
// Builder contract: escalateRetroReason must return the reason unchanged when the
// pattern is generic (led.IsGenericPattern(pattern) == true), even at count>=2.
//
// Internal test (package core) — escalateRetroReason and recurrence.Entry fields
// are exercised directly. RED today: recurrence.Entry has no Generic field, so
// package core fails to compile.
```

### `go/internal/core/recurrence_wiring_c662_test.go:3` — above `import (`

```text
// recurrence_wiring_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index gap G1 (PRODUCTION WIRING). Cycle 661 landed
// recurrence.Ledger.RecordClosure with ZERO call sites, so nothing ever writes
// .evolve/recurrence-ledger.json and Count()==0 forever (the same
// consumed-without-landing disease as token-resolver-production-wiring).
//
// This test drives the EXPORTED orchestrator entry point (RunCycle) end-to-end
// through the real retro-closeout / failure-learning seam
// (writeDeterministicLearning → faillearn.WriteArtifacts), which is where the
// closure must also be recorded into the ledger. It exercises the SUT — a magic
// string cannot make RunCycle write a ledger file keyed by the failing cycle.
//
// Builder contract: wire RecordClosure at the deterministic retro-closeout seam
// (nil Escalator/Autofiler there — escalation apply stays boundary-only to avoid
// racing inboxmover.Claim), persisting to
// <root>/.evolve/recurrence-ledger.json via Load/Save under flock.
//
// RED today: RunCycle writes the lesson YAML but no recurrence-ledger.json, so
// recurrence.Load returns an empty ledger. GREEN once the closure is wired.
```

### `go/internal/core/remediation_rerun_review_test.go:3` — above `import (`

```text
// remediation_rerun_review_test.go — ADR-0100 §4: the remediated gate re-run's
// deliverable goes through the same review as the original dispatch.
//
// maybeRemediate re-dispatches the failed gate after the Builder's fix and
// overwrote dr.resp with the re-run's response; that response then reached
// recordAndBranch without ever meeting the reviewer, so a re-run that omitted
// a declared deliverable — or any contract check — was recorded on the
// strength of its verdict alone.
```

### `go/internal/core/repair_brief.go:3` — above `import (`

```text
// repair_brief.go — what a tdd/build re-dispatch inside an audit-repair round
// is TOLD (research proposal R2, docs/research/ship-rate-harness-reliability-
// 2026-09-02.md §4).
//
// Before this file the brief was audit-fail-reason.json alone: the coherence
// floor's deterministic gate strings ("EGPS: ship_eligible=false", "apicover
// -enforce flagged 2 line(s)"). The auditor's actual findings — the HIGH
// entries in audit-report.md with root cause and path:line evidence — never
// reached the builder. Cycle 1605's H1 (a new exported package with zero
// production callers) survived three rounds while the round-2 builder rewrote
// one sentence of the explanation document; cycle 1596's round-4 builder
// received one truncated defect. The literature is unambiguous that repair
// without the specific finding does not converge (Olausson et al.: feedback
// quality is the bottleneck; Self-Debug: execution-grounded feedback beats
// explanation-only by an order of magnitude).
//
// The brief now carries, in this order and inside the existing byte budget:
//  1. the gate reasons (unchanged — deterministic evidence outranks prose);
//  2. the auditor's findings of the round that just rejected, CRITICAL/HIGH
//     first, MEDIUM after, LOW omitted — parsed by the SAME grammar the
//     dashboard renders (reportdoc.Findings), so the operator and the agent
//     read one list;
//  3. the findings that PERSISTED from the previous round (matched by
//     reportdoc.FindingKey against the archived audit-report.round<N-1>.md) —
//     the "you were told this already" line that turns a blind retry into a
//     targeted one.
//
// The previous round's prompt is archived beside the audit archives
// (build-prompt.round<N>.txt) so what each round was actually told is
// forensically recoverable — gap G10.
```

### `go/internal/core/repair_brief_test.go:130` — above `func TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt(t *testing.T) {`

```text
// TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt is the
// wiring proof through RunCycle: the repair-round build is TOLD the auditor's
// H1 (not only gate strings) and the round-1 prompt is archived.
```

### `go/internal/core/repair_eligibility.go:5` — above `const legitRejection = "legit-rejection"`

```text
// repair_eligibility.go — the in-cycle repair BOOKKEEPING: the durable attempt
// counter, the grant primitive, and the findings-injection seam.
//
// Its former eligibility RULE is gone (ADR-0093). Retries are now decided at the
// audit chokepoint from the audit's own declared failure class and the ADR-0072
// policy table — see audit_fail_decision.go and retry_envelope.go. One retry
// authority, not two: the deleted rule was a second mechanism built beside a
// declarative policy that had always declared the same cap and that nothing read.
//
// What survives here is the machinery a retry still needs wherever it is decided:
// a bound that outlives a crash, one primitive that spends it, and the seam that
// hands the rebuilding agent the audit's own findings.
//
// The legitRejection vocabulary word below is retained for the CORROBORATION half
// of ADR-0092, which is still live in applyFailureDecisionFloor: an agent-authored
// floor claim contradicted by both the deterministic evidence and the agent's own
// disposition does not halt. That narrowing is unchanged; only its retry-granting
// half was removed.
```

### `go/internal/core/repair_eligibility.go:51` — above `const CtxKeyStandingAuditFindings = "standing_audit_findings"`

```text
// CtxKeyStandingAuditFindings carries the last audit's actionable findings
// into a tdd/build re-entry that follows a SHIP-error recovery: the audit
// passed (WARN) so no repair grant exists, yet the recovery rebuild is
// re-audited by the same rubric and every finding left unaddressed is named
// again as standing (cycle 1679, rounds 4→5). A rejection grant outranks it.
```

### `go/internal/core/repair_eligibility.go:130` — above `func retroRouted(cs CycleState) bool {`

```text
// retroRouted reports whether the phase about to be dispatched re-enters the
// cycle straight after a retrospective that followed an audit-fail DECLINE —
// the envelope refused a direct repair (unrecognised class, budget,
// system-level class) and the retro's floor gates then adjudicated a retry
// (cycle 1684). Both halves are required: a retro reached from a dispatch
// error or an exhausted correction ladder is not re-audited work owed the
// audit's findings (AuditDeclineReason is empty there), and a decline whose
// retro sealed the cycle never re-enters. Such a re-entry is re-audited by
// the same rubric, so it carries the standing findings exactly as a
// ship-error recovery does.
```

### `go/internal/core/repair_tier_escalation_test.go:3` — above `import (`

```text
// repair_tier_escalation_test.go — the producer half of `audit_retry_2plus`.
//
// builder.json and tdd-engineer.json have declared
// `model_tier_overrides.audit_retry_2plus: "deep"` since the override table
// landed, and the only tests guarding it checked that the VALUE is a canonical
// tier. Nothing produced the situation: cycles 1595–1605 re-dispatched every
// repair round at the identical tier (balanced, balanced, balanced) and ship
// probability by audit-round count ran 100 % → 50 % → 17 % → 0 %. The
// orchestrator now raises the tdd/build re-dispatch tier to the profile's
// declared override while CycleState.AuditRepairActive is set — the same
// persisted flag the repair brief derives from — through the same envelope
// clamp the ADR-0076 D floor uses.
```

### `go/internal/core/repair_tier_escalation_test.go:94` — above `func TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop(t *testing.T) {`

```text
// TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop is the wiring proof
// (I2 invariant): driven through RunCycle with the real dispatch seam, the
// FIRST build runs at the profile default (no overlay) and the build
// re-dispatched inside the repair round granted after audit round 1's FAIL
// carries ModelRoutingTier=deep — the declared override, live. A unit-green
// repairRoundTier that nothing calls would leave this red.
```

### `go/internal/core/repro_cycle1285_test.go:11` — above `func collidingDefects() []string {`

```text
// repro_cycle1285_core_test.go — executable reproduction of the cycle-1285
// adversarial review's F1 (HIGH), the finding that lives on the failure-floor
// side of the diff.
//
// It drives writeDeterministicLearning — the production seam the failure path
// calls (the three fallback tails of recordFailureLearning) — not
// faillearn.WriteArtifacts directly, because the collision is MINTED by the
// failure-learning engine (ADR-0103 unit 03b, internal/core/failurelearning):
// remediationItems derives the inbox id from remediationSlug(title), and
// remediationSlug stops at remediationSlugMaxRunes = 60.
//
// Chain: two defect lines sharing a 60-rune slug prefix → one id, two different
// titles → inbox.go:114-116 (the cycle-1282 DEF-4 fix) raises a hard error →
// writer.go:30-32 (the WithInbox ordering) returns BEFORE the retrospective and
// the lesson are written → the engine downgrades the whole thing to one
// FAILURELEARNING_FLOOR_WRITE_FAILED signal.
//
// Net effect: a failing cycle produces NO retrospective and NO lesson. That is
// the cycle-1255 state — a defect with no durable record — reached through the
// mechanism built to make it unreachable, and reached more completely, because
// 1255 at least had a report. No adversary is required; two real defects from
// one subsystem routinely share a 60-character prefix.
```

### `go/internal/core/reset.go:42` — above `var ErrCycleOwnedLive = errors.New("reset: cycle is owned by a live run (fresh lease) — refusing to seal without --force…`

```text
// ErrCycleOwnedLive is returned when the cycle's per-run lease is still FRESH
// (its owner is alive, heartbeating) and SealOptions.Force was not set. Sealing
// a running loop's cycle out from under it is the cycle-395 race this fence
// closes; a stale/missing lease (dead owner) seals freely. SealResult carries
// LeaseOwnerPID + LeaseHeartbeatAge so the caller can name the live owner.
```

### `go/internal/core/reset.go:75` — above `AutomatedRecovery bool`

```text
// AutomatedRecovery marks a seal driven by unattended boot self-heal
// (AutosealStaleMarker, triggered merely by a dead owner PID — trivially
// arrangeable by anything that can kill the owning process) rather than an
// explicit human `evolve cycle reset`. ADR-0081's in-band epoch anchor
// trusts a ledger line ONLY when its Role is exactly "operator" — a real
// human sign-off. Before this flag, both paths wrote Role:"operator"
// identically, so triggering the automated path was enough to mint a
// trust-anchor-eligible seal with no human involved at all (the
// unauthenticated-self-declared-role defect). Set true, the ledger entry
// carries a distinct role that Verify's epoch-anchor resolver does not
// recognise — the automated recovery still runs (clearing the role-gate
// block), it just never gains operator trust for chain verification.
```

### `go/internal/core/reset.go:293` — above `statePath := filepath.Join(opts.EvolveDir, "state.json")`

```text
// 3b+4. Mutate state.json under the SAME "<state.json>.lock" sidecar that
// storage.UpdateState holds for its whole RMW (flock.PathLock — the single
// documented single-writer contract). Both the failedApproaches record and
// the lastCycleNumber/batch RMW below were previously unlocked, so in fleet
// mode (2+ concurrent lanes are the live operating mode) a concurrent locked
// UpdateState could read stale state and clobber this seal's writes — the
// cycle-616 lost-update fix. flock is blocking and per-open-file-description;
// neither failurelog.Record nor readJSONMapFile/writeJSONMapFileAtomic locks
// internally, so wrapping them here is not re-entrant. ORDERING IS
// LOAD-BEARING: Record runs BEFORE the seal's own read-modify-write, so the
// seal's write (which re-reads the file, picking up the new entry) stays the
// final authority on lastCycleNumber / currentBatch / lastUpdated. A missing
// state.json is soft-skipped (preflight owns creating it; the seal's own
// write below creates it fresh).
// The lock sits on the RESOLVED path: a worktree's state.json is a link to
// canonical, and a sidecar beside the link is one no canonical-path writer
// ever contends on (cross-tree lost update).
```

### `go/internal/core/reset_autoseal_role_test.go:3` — above `import (`

```text
// reset_autoseal_role_test.go — cycle-1194 continuation (ADR-0081 audit
// defect): the in-band ledger seal's trust anchor must not be reachable via
// the unattended boot self-heal path. AutosealStaleMarker is triggered
// merely by a dead owner PID (trivially arrangeable — kill the owning
// process), so before SealOptions.AutomatedRecovery existed, that path wrote
// the identical Role:"operator" a genuine human `evolve cycle reset` writes.
// Ledger verify's epoch-anchor resolver (go/internal/adapters/ledger/anchor.go)
// trusts ANY ledger line with Role=="operator" — Role/CycleLabel are
// otherwise self-declared, unauthenticated fields — so the automated path was
// enough to mint a trust-anchor-eligible seal with no human sign-off at all.
```

### `go/internal/core/reset_concurrency_test.go:84` — above `func TestSealCycle_ConcurrentUpdateStateNoLostUpdate(t *testing.T) {`

```text
// TestSealCycle_ConcurrentUpdateStateNoLostUpdate is the cycle-616 regression
// for the fable5_deep_scan finding "statefile-rmw-flock-consolidation":
// SealCycle's state.json read-modify-write (reset.go's
// readJSONMapFile/writeJSONMapFileAtomic, step 4) holds NO flock, while
// storage.UpdateState (the documented single-writer contract in
// adapters/storage/updatestate.go) holds "<state.json>.lock" for its whole
// RMW. In fleet mode (2+ concurrent lanes are the live operating mode — see
// fleet_concurrency_respect_architecture memory) SealCycle can race a
// concurrent UpdateState caller and lose one side's write.
//
// The interleaving is forced deterministically via channels (no sleep-based
// race gambling), mirroring the existing
// TestWithPathLock_SerializesConcurrentRMW pattern in
// adapters/flock/withpath_test.go:
//  1. A goroutine starts storage.UpdateState, which locks state.json, reads
//     it (lastCycleNumber=41, version=18), then blocks mid-mutate on a
//     channel — the lock stays held the whole time (release is deferred
//     until mutate returns AND the merged write completes).
//  2. While UpdateState is paused holding the lock, a second goroutine runs
//     SealCycle (which today does its own unlocked read+write of the SAME
//     state.json) and is given a bounded window to run to completion.
//  3. UpdateState is then released to finish its own write.
//
// Without a shared lock, SealCycle's unlocked write completes first and is
// then clobbered by UpdateState's write, because UpdateState's in-memory
// state was read BEFORE SealCycle ran and does not reflect SealCycle's
// lastCycleNumber bump — the classic lost update. Once reset.go's RMW
// acquires the same flock.PathLock(statePath) storage.UpdateState uses,
// SealCycle blocks until UpdateState releases, then reads the
// POST-UpdateState state and its own write lands cleanly on top — both
// writers' changes survive.
```

### `go/internal/core/reset_concurrency_test.go:179` — above `func TestSealCycle_ManyConcurrentUpdateStateWritersNoLostUpdate(t *testing.T) {`

```text
// --- Test-amplification additions (cycle 616 black-box adversarial pass) ---
//
// The AC-Materialization contract for statefile-rmw-flock-consolidation
// requires "reset.go RMW uses the same flock.PathLock-protected accessor as
// statefile.go; concurrent-writer regression test passes; no duplicate RMW
// implementation remains." The RED test above pins the minimal 2-writer
// interleaving. The tests below amplify that same contract along two
// adversarial axes the RED test does not cover: (1) scaling from a single
// concurrent writer to many (the actual fleet operating mode is 2+ lanes,
// per the fleet_concurrency_respect_architecture memory, and the lock must
// hold under N-way contention, not just N=2), and (2) verifying the lock is
// released promptly on SealCycle's success path — a lock-leak/deadlock class
// of regression the original test never checks for, since it only observes
// state AFTER releasing its own paused writer.
```

### `go/internal/core/reset_faillearn_test.go:3` — above `import (`

```text
// reset_faillearn_test.go — failure-floor Phase 2 (inbox
// retro-always-invariant, gap 2 / cycle-244 reproduction): `evolve cycle
// reset` must LEARN, not just archive. The seal writes a deterministic
// retrospective into the sealed archive dir, a failure-lesson YAML into
// instincts/lessons/, and appends an operator-reset failedApproaches
// entry — with the failedApproaches append ordered BEFORE the seal's own
// final state.json write (which stays the canonical last writer of
// lastCycleNumber / currentBatch / lastUpdated).
```

### `go/internal/core/reset_pidfence_test.go:12` — above `func TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce(t *testing.T) {`

```text
// reset_pidfence_test.go — RED contract for cycle-554 workspace-hygiene-s1:
// SealCycle's liveness fence (reset.go F1) checks lease freshness only, so a
// crashed owner with a still-fresh heartbeat (2-6min post-crash window) blocks
// sealing and forces `evolve cycle reset --force` at every batch boundary
// (plan docs/plans/workspace-hygiene-2026-07.md §S1). SealOptions.PidAlive
// threads a liveness probe through the fence via runlease.OwnerLive:
// dead-pid+fresh-lease now seals WITHOUT --force, while a genuinely live
// owner (fresh lease, alive pid) still refuses — the invariant the fence
// exists for (cycle-395 race) must survive unweakened.
```

### `go/internal/core/reset_symlink_test.go:14` — above `func TestSealCycle_SymlinkedStateLocksCanonicalTarget(t *testing.T) {`

```text
// TestSealCycle_SymlinkedStateLocksCanonicalTarget is the cycle-1690 pin for
// the linkGuardDeps topology: a worktree's .evolve/state.json is a link to the
// canonical state file. SealCycle must take the "<canonical>.lock" sidecar
// every canonical-path writer (statemap.UpdateStateMap, storage.UpdateState)
// contends on — never a sidecar beside the link — and its failurelog.Record +
// RMW must write THROUGH the link, leaving it intact (the cycle-999 sever).
```

### `go/internal/core/reset_test.go:258` — above `func TestSealCycle_RegressionCycle395(t *testing.T) {`

```text
// TestSealCycle_RegressionCycle395 (F3) pins the exact incident: a sibling
// `evolve cycle reset` ran in the BETWEEN-CYCLES gap — the loop's per-cycle
// .evolve/.lock was NOT held at that instant — and the old code sealed the
// running loop's cycle out from under it. With the lease fence the FRESH
// heartbeat refuses the seal regardless of whether any flock is held: the
// per-run lease, not the coarse cycle lock, is the liveness SSOT. (Design note:
// we deliberately do NOT make .evolve/.lock batch-scoped — that would break
// shipped fleet concurrency; the heartbeat already spans the whole run.)
```

### `go/internal/core/reset_test.go:289` — above `func TestSealCycle_LeaseFencing(t *testing.T) {`

```text
// TestSealCycle_LeaseFencing (F1) — the load-bearing concurrency fix. SealCycle
// must consult the per-run .lease heartbeat (runlease) before sealing: a FRESH
// lease means a live owner, so sealing is refused (ErrCycleOwnedLive) unless
// --force; a STALE/missing/unparsable lease means the owner is gone, so the
// cycle auto-reclaims with no --force. This is the regression guard for the
// cycle-395 incident where a sibling `evolve cycle reset` sealed a RUNNING loop.
```

### `go/internal/core/resume.go:90` — above `if !errors.Is(err, ErrNoCheckpoint) || os.Getenv(ipcenv.CycleStateFileKey) != "" {`

```text
// Discovery fallback — the 2026-08-29 incident. Fleet lanes write their
// quota/escalation checkpoints through the SAME resolver, but with
// ipcenv.CycleStateFileKey set, so the checkpoint lands in the lane's
// per-run cycle-state file. A later host-global `evolve loop --resume`
// (fresh process, no override) resolved only the singleton and reported
// "no live checkpoint" while three live quota-likely checkpoints sat in
// .evolve/runs/cycle-158*/cycle-state.json — abandoning a lane that had
// already completed build and reached audit. The writer learned fleet
// isolation; the reader had not.
//
// Scope, deliberately narrow:
//   - only on ErrNoCheckpoint (a stale PRIMARY checkpoint is a real answer
//     about a real checkpoint — never scan past it);
//   - only when NO env override is set: inside a lane the override IS the
//     authority, and scanning siblings would let one lane resume another's
//     cycle.
```

### `go/internal/core/resume.go:415` — above `if o.reviewer == nil || resp.Verdict == VerdictSKIPPED {`

```text
// The skip set is the fresh loop's (cyclerun_correction.go): no reviewer,
// or a SKIPPED verdict. A version-0 (pre-explanation-contract) checkpoint
// is NOT a reason to skip — mandatoryExplanationReviewer already delegates
// on version 0 itself, so the former extra skip protected nothing it
// needed to and silently exempted every other reviewer (the contract gate,
// the declared-deliverables gate) for any resumed legacy cycle (ADR-0100 §4).
```

### `go/internal/core/resume_audit_binding_test.go:3` — above `package core`

```text
// Regression test for the cycle-294 resume incident (2026-06-12): ship's
// verifyAuditBinding reads the latest role=auditor kind=agent_subprocess
// ledger entry. RunCycle emits it after a shippable audit, but the resume
// path (RunCycleFromPhase) did not — so a resumed audit→ship always bound to
// a stale entry from an earlier cycle and failed AUDIT_BINDING_HEAD_MOVED.
```

### `go/internal/core/resume_coverage_test.go:108` — above `func TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted(t *testing.T) {`

```text
// TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted pins the resume-correctness
// fix: an advisor-inserted phase (e.g. "mutation-gate") is registered in o.runners
// at runtime via MintPhase but is NOT one of the 13 spine phases Phase.IsValid()
// recognizes. The cycle-295 checkpoint-preservation fix makes resumeFromPhase
// record such a phase, so RunCycleFromPhase must ACCEPT a startPhase found in
// o.runners (not reject it as "invalid resume phase"). Behavioral assertion: the
// inserted runner is actually dispatched (Run called) — proof the guard let it
// through. RED baseline: the guard rejects before the lock, so calls == 0.
```

### `go/internal/core/resume_dynamic_transition_test.go:3` — above `import (`

```text
// resume_dynamic_transition_test.go — RED regression suite for cycle-637 task
// resume-dynamic-phase-transition (inbox 2026-07-10T01-30-00Z, weight 0.93).
//
// Defect: `evolve loop --resume` replays the checkpointed advisor-inserted phase
// (e.g. bug-reproduction, fault-localization — real catalog phases the dynamic
// router splices onto bugfix cycles), the phase re-runs and PASSes, but the
// transition kernel consulted AFTER it (o.sm.Next in RunCycleFromPhase's dispatch
// loop) only knows the static spine. current.IsValid()==false for the inserted
// phase, so Next returns "core: invalid phase: <phase>", the resumed cycle dies,
// and — because that transition-error return escapes the ADR-0044 C1 recording
// chokepoint — it is paged FAILED_UNEXPLAINED with no abort_reason.
//
// Evidence: 2026-07-10 resume of cycle 635 (resumeFromPhase=bug-reproduction):
// "evolve loop: resume cycle 635: transition from bug-reproduction: core: invalid
// phase: bug-reproduction"; outcome FAILED_UNEXPLAINED "a terminal path escaped
// the C1 chokepoint".
//
// These tests are plain (untagged) package-core tests so they run in the default
// `go test ./internal/core/...` suite and under -race. They reuse the in-package
// fakes (fakeStorage/fakeLedger/fakeRunner/buildRunners) from orchestrator_test.go
// exactly as the existing resume coverage tests do. The cycle-637 ACS predicates
// (go/acs/cycle637) shell `go test -run` over each one.
```

### `go/internal/core/resume_dynamic_transition_test.go:40` — above `func writeRoutingPlan(t *testing.T, workspace string, entries []map[string]any) {`

```text
// writeRoutingPlan writes the advisor's whole-cycle plan to
// <workspace>/routing-plan.json in the SAME on-disk shape parsePhasePlan reads:
// a bare JSON array of {"phase","run"} entries. Mirrors the real artifact
// (.evolve/runs/cycle-N/routing-plan.json) the sealed cycle-635 run produced.
```

### `go/internal/core/resume_dynamic_transition_test.go:151` — above `func TestRunCycleFromPhase_TransitionFailureRecordsAbortReason(t *testing.T) {`

```text
// TestRunCycleFromPhase_TransitionFailureRecordsAbortReason — AC3 (predicate).
// The ADR-0044 C1 invariant on the resume dispatch loop: NO terminal path may
// escape the recording chokepoint. When the cursor cannot continue after a
// transition (here: the resolved successor has no registered runner), the
// failure must funnel through recordPhaseOutcome — writing a <phase>-usage.json
// sidecar carrying a non-empty abort_reason — so the outcome is FAILED_EXPLAINED,
// never the FAILED_UNEXPLAINED the cycle-635 resume produced.
//
// RED baseline: resume.go returns the transition/no-runner error bare, without
// recording an outcome for the stalled phase, so audit-usage.json is never
// written and cyclehealth pages the cycle FAILED_UNEXPLAINED.
```

### `go/internal/core/resume_execution.go:57` — above `var phaseTimings []phaseTimingEntry`

```text
// ADR-0044 C1 (deferred-to-C3 debt, now paid): the resume path was a
// SECOND recording boundary that wrote no timings/sidecars at all —
// resumed phases were invisible in phase-timing.json. Every terminal
// disposition below funnels through the same recordPhaseOutcome
// chokepoint RunCycle uses; the deferred writer flushes on abort too
// and APPEND-MERGES with the pre-crash entries (writePhaseTimings).
// Semantic note: PhasesRun now includes aborted-but-DISPATCHED phases on
// resume too (the chokepoint appends on every terminal path) — same
// what-actually-ran contract RunCycle adopted in Slice 1; consumers are
// printing/telemetry only (audited then).
```

### `go/internal/core/resume_execution.go:122` — above `noRunnerErr := fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)`

```text
// ADR-0044 C1 (cycle-637): a stranded successor on RESUME is a
// terminal disposition that must funnel through the recording
// chokepoint — the cycle-635 resume died FAILED_UNEXPLAINED precisely
// because this bare error escaped it. Record a synthetic outcome
// carrying the abort_reason so the outcome is FAILED_EXPLAINED.
```

### `go/internal/core/resume_execution.go:141` — above `resetFloorFailReason(&cs, next)`

```text
// Mirrors cyclerun_dispatch.go (resume-parity): a resumed audit
// re-dispatch supersedes any prior attempt's diagnosed-FAIL
// explanation — stale reasons must never mark a later FAIL as
// diagnosed to the ADR-0072 floor.
```

### `go/internal/core/resume_execution.go:146` — above `supersedePreviousAuditRound(&cs)`

```text
// Resume-parity for the round's verdict artifacts (cycle-1603):
// same supersession rule as cyclerun_dispatch.go — a resumed
// re-audit must not replay the previous round's verdict.
```

### `go/internal/core/resume_execution.go:183` — above `Worktree:                        cs.ActiveWorktree,`

```text
// CB.1: the resume path is a first-class dispatch surface and must
// thread the persisted worktree like the RunCycle loop does — a
// resumed phase with Worktree="" runs cwd=main-tree (cycle-280 class).
```

### `go/internal/core/resume_execution.go:288` — above `if cursor.current == PhaseAudit && resp.Verdict == VerdictFAIL {`

```text
// Resume-path parity for the audit-FAIL disposition (ADR-0093). Without
// this branch the resume surface falls through to sm.Next(audit, FAIL) =
// retro, so a resumed cycle could NEVER repair — and since retro is now
// terminal, the retry the policy table grants would be silently
// unreachable on exactly the surface that exists for recovery. The live
// loop's branch is cyclerun_record.go; both call the same primitive.
```

### `go/internal/core/resume_execution.go:308` — above `if o.successorStrategy(cursor.current) == phasespec.BranchingHistory {`

```text
// History-branch gate (ADR-0058): the branch-entry CONDITION is lockstep
// with recordAndBranch (both key on successorStrategy == history, which
// owns the degrade). The branch BODY differs by design — resume takes the
// deterministic decideAfterRetro, whereas the live loop additionally
// routes via decideAfterRetroRouted at advisory stage.
```

### `go/internal/core/resume_execution.go:337` — above `if gateErr := o.finalizeRetroCompletion(cs.WorkspacePath); gateErr != nil {`

```text
// S2 disposition gate: an absent or invalid disposition is
// surfaced loudly in RetroDecision, never silently recorded clean
// (the cycle-1046 gap). Without this, a resumed cycle reports a
// clean retro decision over a disposition nothing verified.
```

### `go/internal/core/resume_execution.go:346` — above `if sysFail != nil && result.SystemFailure == nil {`

```text
// ADR-0072 S4: the Go floor is non-bypassable on the resume path too —
// a floor category halts + escalates rather than looping as task-level.
```

### `go/internal/core/resume_execution.go:361` — above `}`

```text
// The debugger signal-branch gate (ADR-0058 S3) is intentionally NOT
// mirrored here. Per the ADR the debugger override is live-loop-only: of
// the two record/resume override sites, resume duplicates only the retro
// (history) override. A cycle resumed at debugger therefore terminates via
// Next(debugger,_)→end rather than re-running decideAfterDebugger — the
// unchanged pre-ADR behavior. Lifting that to resume is a separate slice,
// not an S3 byte-identity change.
```

### `go/internal/core/resume_execution.go:371` — above `timingOwner.ctx, timingOwner.cs, timingOwner.state = ctx, cs, state`

```text
// ADR-0044 C1, resume parity: the fresh path records an explicit abort
// here (orchestrator.go, recordChokepointEscape) precisely so the
// escape classifies FAILED_EXPLAINED instead of paging an operator
// with the FAILED_UNEXPLAINED alarm bucket — the cycle-492 escape.
// Fresh's "preserves the worktree for salvage" rationale carries over
// too: this return happens before completeCycle, so the closeout's
// cycleCompletedNormally stays false and RunCycleFromPhase's exit
// teardown (cycle_worktree_teardown.go) preserves the tree.
// Resume returned a bare error, reproducing on this path the exact
// defect the fresh path was fixed for. Call the SAME primitive rather
// than a second implementation: it records the phase outcome through
// the C1 chokepoint, feeds failure-learning, and marks the verdict.
//
// phaseTimings is handed to the owner and read back because
// recordPhaseOutcome appends through a pointer to the owner's own
// slice; without the round trip the deferred timing flush would write
// the pre-escape set and the abort would be invisible on disk.
// ctx is assigned deliberately and must NOT be "simplified" away. The
// owner is constructed with context.Background() so the timing defers
// survive cancellation, but recordChokepointEscape also feeds
// failure-learning, which DISPATCHES a retrospective agent and skips
// that model call only when its own ctx is already cancelled. Handing
// it Background would defeat that guard and spend a dispatch after an
// operator interrupt. The abnormal-epilogue defer is unaffected either
// way: it guards on the LOCAL ctx before it ever reads the owner.
//
// All four mutated fields are read back below, not just the two the
// escape appends to: recordFailureLearning also mutates cs (it stamps
// the retro phase and appends it to CompletedPhases), and the epilogue
// defer persists cs — so dropping the read-back would let that write
// clobber the retro completion it just recorded.
```

### `go/internal/core/resume_fleet_discovery_test.go:3` — above `import (`

```text
// resume_fleet_discovery_test.go — a fleet-written checkpoint must be findable
// by a host-global resume.
//
// THE INCIDENT (2026-08-29 → 08-31, cycles 1580-1582): all three lanes hit the
// all-families quota wall. Each one kept its promise — "checkpoint written —
// resume with `evolve loop --resume` after quota reset" — via
// QuotaBoundaryCheckpointer, which resolves through ResolveCycleStatePath and
// therefore honored the fleet lane's EVOLVE_CYCLE_STATE_FILE override: the
// checkpoint landed in the lane's PER-RUN cycle-state file
// (.evolve/runs/cycle-N/cycle-state.json). The lane teardown then unset the
// override. When the operator ran `evolve loop --resume` after the quota reset,
// the fresh process resolved the HOST-GLOBAL .evolve/cycle-state.json — absent —
// and reported "no live checkpoint" while THREE live quota-likely checkpoints
// sat on disk. Cycle-1580 had completed build and reached audit; all of that
// progress was abandoned and the next wave re-did the work from scratch.
//
// The writer learned fleet isolation (2026-07-03); the reader never did. This
// is also the root cause behind the long-standing operator note "fleet -resume
// broken (relaunch FRESH)" — it was never a checkpointing defect, it is a
// DISCOVERY defect.
```

### `go/internal/core/resume_fleet_discovery_test.go:54` — above `func TestLoadResumeState_DiscoversFleetPerRunCheckpoint(t *testing.T) {`

```text
// TestLoadResumeState_DiscoversFleetPerRunCheckpoint reproduces the incident:
// host-global cycle-state.json ABSENT, three per-run quota-likely checkpoints
// present. Resume must find the NEWEST one (by savedAt) instead of reporting
// "no live checkpoint" over live checkpoints.
```

### `go/internal/core/resume_fleet_discovery_test.go:65` — above `writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1580"), fleetCheckpointState(1580, "2026-08-29T02:10:00Z", "au…`

```text
// The three orphans, exactly as the incident left them.
```

### `go/internal/core/resume_fleet_discovery_test.go:135` — above `func TestLoadResumeState_StalePerRunCheckpointReportsStaleNotMissing(t *testing.T) {`

```text
// When per-run checkpoints exist but the newest is STALE (worktree gone), the
// error must be the honest ErrStaleCheckpoint — never "no live checkpoint",
// which is the lie the incident produced. An operator told "stale" knows to
// pass the override or reset; an operator told "nothing to resume" relaunches
// fresh and burns the preserved progress.
```

### `go/internal/core/resume_lifecycle_test.go:185` — above `if paused := eventsOfKind(*got, signalcenter.KindQuotaPaused); len(paused) != 1 || paused[0].Phase != "audit" || paused[…`

```text
// ADR-0101 S2a: the resume root reaches the same quota.paused producer
// (pauseForQuota), and a pause never seals the cycle.
```

### `go/internal/core/resume_normalize_test.go:3` — above `package core`

```text
// resume_normalize_test.go (integration tier — uses real git via gitInRepo).
// RED contract for the resume.go:278 TODO
// (cycle-156 parity): RunCycle soft-resets a committing builder's worktree
// commits back to the cycle base after PhaseBuild (normalizeWorktreeToBase)
// so audit + binding see PENDING changes; the crash-resume path
// (RunCycleFromPhase) lacked that normalize because the base SHA lived in a
// RunCycle-local variable. Fix: persist CycleState.WorktreeBaseSHA at
// worktree creation and run one shared post-build normalize in both loops.
//
// The persisted-base design introduces one risk the local variable never
// had: after a rebase recovery (soak 2026-06-12) the stored base may no
// longer be an ancestor of the worktree HEAD, and `reset --soft` to a
// non-ancestor would repoint the branch and stage a huge spurious diff.
// normalizeWorktreeToBase must therefore verify ancestry before resetting.
```

### `go/internal/core/resume_normalize_test.go:38` — above `func TestRunCycleFromPhase_NormalizesBuildWorktree(t *testing.T) {`

```text
// TestRunCycleFromPhase_NormalizesBuildWorktree: resuming FROM PhaseBuild
// with a committing builder must re-expose the builder's commit as pending
// changes (cycle-156 Option C), exactly like RunCycle does. The base SHA
// comes from the persisted CycleState.WorktreeBaseSHA.
```

### `go/internal/core/resume_parity_history_test.go:13` — above `func TestRunCycleFromPhase_HistoryBranchSurfacesDispositionGate(t *testing.T) {`

```text
// TestRunCycleFromPhase_HistoryBranchSurfacesDispositionGate pins the S2
// disposition gate on the resume path. The fresh history branch
// (cyclerun_record.go) runs finalizeRetroCompletion and PREPENDS its error to
// the retro reason, because "an absent/invalid disposition is surfaced loudly
// in RetroDecision, never silently recorded clean" — the cycle-1046 live gap.
// Resume's history branch mirrored the bookkeeping-regrade bound right beside
// it (with a comment calling out resume-path parity) but not this gate, so a
// resumed cycle records a clean retro decision over a disposition that was
// never verified.
```

### `go/internal/core/resume_parity_recording_test.go:1` — above `package core_test`

```text
// resume_parity_recording_test.go — resume-path parity regressions.
//
// The fresh cycle path records a terminal outcome at every way a cycle can
// end. The resume path, built as a parallel implementation, does not: three
// of its exits return a bare error where the fresh path first records the
// outcome and feeds failure-learning. The consequence is named in
// recordChokepointEscape's own doc comment — an unrecorded terminal exit
// classifies FAILED_UNEXPLAINED, "the alarm bucket (the cycle-492 escape)" —
// so a resumed cycle that dies this way pages an operator with no diagnosable
// reason, which is exactly the failure mode the fresh path was fixed for.
//
// Each test below mirrors an existing fresh-path test against
// RunCycleFromPhase. The compatibility table in
// docs/reports/2026-09-11-large-component-decomposition-plan.md lists the
// fresh/resume differences that ARE intentional (parallel evaluation,
// remediation, the debugger override, legacy contract versions); none of
// these three is on it, and the resume code documents the debugger omission
// inline while saying nothing about these.
```

### `go/internal/core/resume_policy.go:15` — above `if next == PhaseBuild {`

```text
// ADR-0076 D (deterministic escalation floor — applied AFTER the mode-
// gated projection and INDEPENDENT of it, review finding D1: the live
// registry runs model_routing=static and a policy floor must still fire):
// a retried scoped item raises the build dispatch tier to deep, clamped
// through the same envelope guardrail as the routing clamp.
```

### `go/internal/core/resume_policy.go:31` — above `if next == PhaseBuild {`

```text
// ADR-0076 slice A: the build dispatch carries the cycle's difficulty
// multiplier so the engine can stretch the artifact-wait deadline. Scale
// 1.0 is left unset — byte-identical legacy dispatch.
```

### `go/internal/core/resume_review_parity_test.go:3` — above `import (`

```text
// resume_review_parity_test.go — ADR-0100 §4: the resume loop's review skip
// set is identical to the fresh loop's.
//
// reviewResumedDeliverable skipped review entirely when the checkpoint's
// ExplanationDocumentationVersion was 0 ("legacy checkpoints retain their
// historical behavior"). The explanation reviewer already delegates on
// version 0 (mandatoryExplanationReviewer), so that skip protected nothing
// it needed to — and it silently exempted every OTHER reviewer (the contract
// gate, the declared-deliverables gate) for any resumed legacy cycle.
```

### `go/internal/core/resume_transition.go:10` — above `func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {`

```text
// resume_transition.go — cycle-637 (inbox resume-dynamic-phase-transition, 0.93).
// Rehydrates the transition kernel on `evolve loop --resume` so a resumed cycle
// can transition OUT of an advisor-inserted phase (bug-reproduction,
// fault-localization — real catalog phases the dynamic router splices onto
// bugfix cycles, none spine-valid). The static kernel (o.sm.Next) returns
// "core: invalid phase: <phase>" for them, so the resumed cycle-635 died and its
// bare error escaped the ADR-0044 C1 chokepoint as FAILED_UNEXPLAINED.
```

### `go/internal/core/resume_worktree_teardown_test.go:81` — above `func TestRunCycleFromPhase_PreservesAFailedCycleWorktree(t *testing.T) {`

```text
// TestRunCycleFromPhase_PreservesAFailedCycleWorktree is the narrowness guard,
// and it is the one that matters most: this worktree holds audited, possibly
// uncommitted work, and `evolve loop --resume` / `evolve cycle reset` reclaim
// the lane BY that path. A teardown that pruned unconditionally would trade a
// leak for the cycle-7 lost-work incident.
```

### `go/internal/core/retro_remediation_filter_test.go:12` — above `func remediationFixture(t *testing.T) (*Orchestrator, failureLearningRequest, string) {`

```text
// retro_remediation_filter_test.go — RED contract for cycle-1282 D5
// (.evolve/runs/cycle-1279/audit-report.md, MEDIUM).
//
// failure_learning.go:442-448 states: "Only SELF-REPORTED structured defects are
// filed — the synthesized summary echo (ev.Defects == []string{summary}) is a
// restatement of the failure, not an actionable item, and filing it would be
// inbox noise." The guard implementing that claim is `structured != nil` ALONE.
// But `structured != nil` does not imply `len(structured.Defects) > 0`:
// phasecontract.ReadFailureBlock returns a block whenever Class != ""
// (sentinel.go:148), and ev.Defects is overwritten only under
// `if len(structured.Defects) > 0` (failure_learning.go:432). A
// classed-but-defectless block therefore leaves ev.Defects == []string{summary}
// and files it as a kind:"bug", priority:"H" inbox item — a comment asserting a
// filter the code does not implement.
//
// faillearn.structuredDefects (faillearn.go:143-148) is the EXISTING, named
// rule for exactly this degenerate case and is not reused. The builder must
// route the call site through that one rule rather than restating it here
// (feedback_never_duplicate_centralize_via_design_patterns) — exporting it from
// faillearn requires a matching entry in that package's apicover_named test.
//
// These drive writeDeterministicLearning, the seam the production failure path
// calls at failure_learning.go:366/372; TestC662_RetroCloseoutRecordsClosureInLedger
// already pins that RunCycle reaches it, so this file grades the RULE while the
// reachability lock stays where it is.
```

### `go/internal/core/retro_verdict_semantics_test.go:3` — above `import (`

```text
// retro_verdict_semantics_test.go — RED contract for what a retro verdict MEANS.
//
// `decideAfterRetro` opened with:
//
//	// retro PASS → ship; no failureadapter consultation, no floor (nothing failed).
//	if retroVerdict == VerdictPASS {
//	    return o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseShip), nil, "retro-recovered: ship", nil
//	}
//
// and TestOrchestrator_RetroPASS_RoutesToShip (2026-05-23) pinned it deliberately,
// forcing audit=FAIL, retro=PASS and asserting the tail audit→retro→ship. So this
// is a considered behaviour being changed, not an oversight being tidied. Three
// independent facts say the consideration was wrong:
//
//  1. THE COMMENT'S PREMISE IS FALSE. Retro runs only when the previous verdict is
//     FAIL/WARN (retro.go: "previous verdict != FAIL/WARN → SKIPPED"), and the
//     state machine routes only audit-FAIL to retro. "Nothing failed" is never
//     true on this path.
//
//  2. RETRO CANNOT CHANGE THE TREE. Its persona is "READ-ONLY everywhere except
//     retrospective-report.md, handoff-retrospective.json, failure-decision.json,
//     and instincts/lessons/*.yaml". So the tree ship would commit is BYTE-IDENTICAL
//     to the one the auditor rejected. Nothing happened in between that could make
//     a rejected build shippable.
//
//  3. RETRO PASS ANSWERS A DIFFERENT QUESTION. retro.go computes it as
//     "retrospective non-empty AND a failure-lesson exists" — a DELIVERABLE
//     COMPLETENESS signal. Routing consumed it as "the cycle recovered". Those are
//     different questions, and a verdict is only meaningful with the question it
//     answers.
//
// Why it looked viable for three months: ship's audit binding used to be
// satisfiable by another cycle's PASS entry (the cross-run fail-open closed by
// #503 on 2026-08-26). Once ship began failing closed with
// CodeAuditBindingVerdictFail, this route's only possible outcome became an error.
// A path whose sole outcome is a guaranteed ShipError is not a designed path.
```

### `go/internal/core/retro_verdict_semantics_test.go:65` — above `func TestDecideAfterRetro_RetroPASSStillFacesTheFloor(t *testing.T) {`

```text
// The ladder must be REACHED, not merely not-ship. This pins that a retro PASS
// gets the same floor evaluation a retro FAIL does — otherwise a system failure
// could escape ADR-0072 simply by the post-mortem being well written.
```

### `go/internal/core/retro_verdict_semantics_test.go:85` — above `func TestDecideAfterRetro_RetroPASSReachesTheSameLadderAsFAIL(t *testing.T) {`

```text
// A retro PASS must reach the same disposition ladder a retro FAIL reaches. The
// original form of this test asserted it reached ADR-0092's retro-side repair
// branch; that branch is gone — retries are now decided at the AUDIT chokepoint
// from the audit's own declared class (audit_fail_decision.go). What still matters,
// and what this pins, is that a retro PASS is not short-circuited past the ladder.
```

### `go/internal/core/retro_verdict_semantics_test.go:123` — above `func TestDecideAfterRetroRouted_RetroPASSCannotDropTheFloorSignal(t *testing.T) {`

```text
// THE LIVE PATH, and the sharpest form of the defect. decideAfterRetroRouted had
// its own PASS early return:
//
//	detNext, extraEnv, detReason, sig := o.decideAfterRetro(...)
//	if retroVerdict == VerdictPASS {
//	    return detNext, extraEnv, detReason, nil // PASS recovers; not a failure branch
//	}
//
// It discards `sig`. So a DETERMINISTIC ADR-0072 floor candidate — the signal the
// codebase describes as "non-bypassable" and "a broken pipeline cannot dodge it" —
// was dropped whenever the retrospective happened to be well written. A system
// failure could escape the halt by writing a good post-mortem.
//
// Unreachable in practice only because the retro gate almost never returns PASS
// (the failure-lesson artifact mismatch). Fixing that gate without this would have
// made it live — the same masked-defect pair, one layer down.
```

### `go/internal/core/retry_adjudication_test.go:3` — above `import "testing"`

```text
// retry_adjudication_test.go — RED contract for the CLAMP: how an adjudicator's
// choice is bound by the deterministic envelope.
//
// The adjudicator is a deep-tier phase that reviews a failure architecturally and
// proposes a path. It exists because choosing BETWEEN legal options is judgment
// work (Core Agent Rule 5). It must never become another proxy-as-verdict, so two
// properties are pinned here:
//
//   - STRATEGY: adjudicated and deterministic paths are interchangeable — the
//     caller gets an action either way and never branches on which ran.
//   - NULL OBJECT: an absent, malformed, or out-of-vocabulary adjudication
//     degrades to the policy default. It is an ENHANCEMENT, never a precondition.
//     This is precisely the ADR-0092 defect designed out: there, the retry
//     depended on an agent artifact, and was measured reachable on 3 of 16
//     failures because the artifact was usually missing.
//
// The envelope is ORDERED by preference, so the default is simply Legal[0] — the
// most thorough legal action. No second table of defaults to drift.
```

### `go/internal/core/retry_adjudication_test.go:103` — above `func TestClampAdjudication_HaltingEnvelopeIsNotNegotiable(t *testing.T) {`

```text
// A halting envelope offers nothing, and no adjudication may resurrect it. This is
// the ADR-0072 boundary: the floor is not negotiable by an agent.
```

### `go/internal/core/retry_adjudicator_agent.go:12` — above `type bridgeRetryAdjudicator struct {`

```text
// retry_adjudicator_agent.go — the bridge-dispatched RetryAdjudicator.
//
// Built exactly like FailureAdvisor and PhaseAdvisor: bridge-dispatched,
// persona-injected, strict-JSON-parsed. It differs from them in one deliberate
// way, and that difference is the whole safety argument.
//
// FailureAdvisor returns an ERROR on any failure so the caller escalates. This
// adjudicator returns NIL instead, because nil is a working answer here: the
// deterministic policy has already decided what is legal, and clampAdjudication
// turns a nil proposal into the policy default. The agent can only ever narrow a
// choice among options Go already permitted — it can never grant a retry policy
// forbids, exceed MaxRetries, or overturn the ADR-0072 floor.
//
// That is why this is not another proxy-as-verdict. The failure mode of ADR-0092
// was an agent artifact being a PRECONDITION for a decision; here it is an
// enhancement to one that already works without it.
```

### `go/internal/core/retry_backoff_test.go:3` — above `import (`

```text
// retry_backoff_test.go — regression lock for composeCorrection's
// VERBATIM-INCLUSION property (cycle-1510 task
// `contract-correction-verbatim-output-fidelity`, carryover from the
// cycle-1508 audit FAIL defect M1).
//
// Honest framing, stated up front because it is the whole lesson of
// inst-L1508b: this property ALREADY HOLDS on the pre-change code
// (retry_backoff.go:12-17 concatenates the reason with `+`). These tests are
// therefore PRE-EXISTING GREEN by design, not RED. Cycle-1508 failed audit
// precisely for claiming a tautologically-green criterion as work "established"
// by a product change; the correct disposition is to say so and convert the
// unlocked property into a locked one. The value here is future-facing: any
// later refactor of composeCorrection that reformats, re-wraps, truncates,
// escapes, or normalizes the rejection reason now fails loudly instead of
// silently degrading the correction directive the phase re-dispatch depends on.
//
// Why verbatim matters. The reason string is deliverable.summarize()'s
// rendering, carrying "[code] message" tokens. contractViolationCodeRE parses
// those SAME tokens back out of the directive downstream
// (contract_escalation.go), and the re-dispatched agent is expected to read the
// literal violation text. Any lossy transform breaks both consumers at once.
```

### `go/internal/core/retry_envelope.go:11` — above `type retryAction string`

```text
// retry_envelope.go — what the DETERMINISTIC policy declares legal after an audit
// FAIL. A Specification: a pure predicate, no I/O, so the same answer comes out on
// the live, routed and resume paths and one table can pin every branch.
//
// It makes ADR-0072's category table the single retry authority. That table has
// always declared
//
//	CategoryCodeAuditFail: {Level: LevelTask, Action: ActionRetryWithFix,
//	                        FixType: "address-audit-findings", MaxRetries: 2}
//
// while `fp.Categories[...]` was read in exactly one place, only for Level —
// Action, MaxRetries and FixType were consumed nowhere. ADR-0092 then built a
// parallel knob and a disposition-prose eligibility rule beside it, arriving at
// the same cap of 2 independently. One authority now, not two.
//
// The envelope only says what is LEGAL. Choosing among legal actions is the
// adjudicator's job, and it can never widen this set.
```

### `go/internal/core/retry_envelope.go:168` — above `func clampAdjudication(env retryEnvelope, adj *adjudication) (retryAction, bool) {`

```text
// clampAdjudication binds a proposal to the envelope, returning the action to take
// and whether the proposal had to be overridden.
//
// NULL OBJECT: a nil, unjustified, or out-of-vocabulary proposal yields the policy
// default rather than "no decision". No agent artifact is load-bearing here — the
// ADR-0092 failure (retry reachable on 3 of 16 cycles because the artifact was
// usually missing) is designed out rather than mitigated.
//
// The clamp is one-directional by construction: an adjudicator may always choose a
// MORE conservative action within the envelope, and can never reach outside it.
```

### `go/internal/core/retry_envelope_join_test.go:11` — above `func TestPolicyCategoryFor_JoinsEveryKnownClassificationOrSaysNone(t *testing.T) {`

```text
// The audit is told the failurelog vocabulary (13 classes) while the retry
// table is keyed by the policy vocabulary (7 categories); before this join the
// envelope looked a failurelog spelling up in the policy table and declined
// "unrecognised class" for words the prompt itself recommended (architecture
// review of F19). One projection joins the two, and the decline reason says
// which of the three things happened.
```

### `go/internal/core/retry_envelope_test.go:3` — above `import (`

```text
// retry_envelope_test.go — RED contract for the retry ENVELOPE: what the
// deterministic policy declares LEGAL after an audit FAIL.
//
// This is a Specification: a pure predicate over (declared class, deterministic
// floor candidate, attempts, policy). Pure because the same answer must come out
// on the live, routed and resume paths, and because one table has to pin every
// branch — every I/O concern is inverted into the input struct (Dependency
// Inversion; the policy is passed, never read from disk in here).
//
// THE FINDING THIS ENCODES. ADR-0072's category table already declares:
//
//	CategoryCodeAuditFail: {Level: LevelTask, Action: ActionRetryWithFix,
//	                        FixType: "address-audit-findings", MaxRetries: 2}
//
// and `fp.Categories[...]` was read in exactly ONE place (failure_dossier.go),
// only for Level. Action, MaxRetries and FixType were consumed NOWHERE. ADR-0092
// then built a parallel `max_audit_repair_attempts` knob and a disposition-prose
// eligibility rule beside it — including the same cap of 2. This envelope makes
// the existing declarative policy the single retry authority and deletes the
// parallel one.
```

### `go/internal/core/retry_opts.go:3` — above `import (`

```text
// retry_opts.go — the ONE registry of phase-retry recovery hooks (cycle-1166,
// evaluate-batch-retry-parity).
//
// Two retry loops existed: the sequential dispatch loop (cyclerun_dispatch.go)
// and the evaluate-batch loop (evaluate_batch.go). They agreed only by hand, so
// each hook the sequential loop grew had to be re-remembered on the batch side —
// and twice was not: optionalInfraSkip and postShipObserverSkip shipped
// sequential-only, so an optional evaluate phase that exhausted infra retries
// aborted the whole batch instead of degrading. Fixing the two misses does not
// fix the CLASS; the next hook diverges the same way.
//
// retryOpts is that class fix: a Strategy value enumerating every recovery hook
// a retry loop may run, with a nil field meaning "this path does not run that
// hook" — divergence made explicit and inspectable instead of implicit and
// invisible. Both paths take their hooks from a constructor here, so a new hook
// is a new FIELD, and a field the batch constructor forgets is visible in one
// place rather than discoverable only by diffing two loops.
```

### `go/internal/core/retry_opts.go:136` — above `cr.preserveWorktree = true`

```text
// Preserve the worktree from the exit cleanup while a ship failure is
// unresolved (ADR-0039 §8 / D10) — cleared when a later ship succeeds.
```

### `go/internal/core/retry_opts.go:149` — above `cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts,`

```text
// ADR-0044 C1: the failed ship attempt ran and burned budget — record it
// before routing to recovery. A later successful ship records its own.
```

### `go/internal/core/retry_opts_parity_test.go:3` — above `import (`

```text
// retry_opts_parity_test.go — RED contract for cycle-1166 Task 1
// (evaluate-batch-retry-parity, inbox weight 0.87).
//
// State of the world when this file was authored. The inbox item's FIRST half
// ("dispatchRunnerWithRetry is missing optionalInfraSkip + postShipObserverSkip")
// has ALREADY landed: evaluate_batch.go:110 calls both predicates, and
// evaluate_batch_retry_parity_test.go is green. What has NOT landed is the
// item's stated FIX — "extract the shared retry core … retryOpts is a small
// Strategy struct carrying the optional hooks" — and its second acceptance
// criterion, the PARITY PIN:
//
//	"a table test enumerating retryOpts hooks asserts the main loop passes the
//	 full set (a new hook added to cyclerun without registering in retryOpts
//	 fails compilation or the table)"
//
// Two copies of the retry loop that merely happen to agree today is exactly the
// replicated-beliefs disease the item cites: the NEXT hook added to
// cyclerun_dispatch.go silently misses the batch path again. The pin below is
// the structural guard that makes that impossible.
//
// RED today: `retryOpts`, `retryPhaseRunner`, `mainDispatchRetryOpts` and
// `evaluateBatchRetryOpts` do not exist, so this file does not compile. That is
// the intended RED — a compile failure naming the missing Strategy struct.
//
// Contract Builder must satisfy (names are load-bearing — this file binds them):
//
//	type retryOpts struct {
//	    backfill             func(...) ...   // nil ⇒ hook disabled
//	    optionalInfraSkip    func(...) ...
//	    shipRecovery         func(...) ...
//	    postShipObserverSkip func(...) ...
//	}
//	func (cr *cycleRun) retryPhaseRunner(phase Phase, req PhaseRequest, opts retryOpts) (PhaseResponse, int, error)
//	func (cr *cycleRun) mainDispatchRetryOpts() retryOpts    // the sequential loop's hook set
//	func (cr *cycleRun) evaluateBatchRetryOpts() retryOpts   // the batch path's subset
//
// Field SIGNATURES are deliberately NOT pinned (Builder owns those); only the
// field NAME SET and each constructor's enabled/disabled hooks are.
```

### `go/internal/core/retry_tier_escalation.go:3` — above `import (`

```text
// retry_tier_escalation.go — ADR-0076 slice D: recurrence-driven tier
// escalation as a DETERMINISTIC DISPATCH FLOOR. An item that already failed
// routes its next BUILD to at least the deep tier: deep-tier audit reliably
// catches what balanced-tier build cannot finish on hard items, so retrying
// at the same tier re-fails identically (batches 6-8).
//
// Design (adversarial review 2026-07-23, findings D1/D2): applied at BUILD
// DISPATCH, independent of the model_routing mode gate — a policy-driven
// floor must not depend on advisory routing to take effect (the live registry
// runs static). The raise is clamped through the SAME envelope guardrail the
// routing clamp uses (a single-entry ClampPlanModelRouting pass — never a
// second clamp implementation), so a profile's envelope Max still wins.
// Raise-only: a proposal already at or above deep is never touched.
//
// ADR-0096 adds a second producer of the same raise (repairRoundTier); both
// share clampedRaise so the raise-only rule and the clamp live once.
```

### `go/internal/core/retry_tier_escalation.go:126` — above `func (o *Orchestrator) repairRoundTier(projectRoot string, phase Phase, cs CycleState, currentTier string) (string, bool…`

```text
// repairRoundTier decides the dispatch-time raise for a tdd/build re-entry
// INSIDE an audit-repair round (CycleState.AuditRepairActive — the same
// persisted flag the repair brief derives from, so the live loop and the
// crash-resume path cannot diverge). The target tier is the phase profile's
// declared model_tier_overrides[audit_retry_2plus]; no declaration ⇒ the rule
// is inert (config decides, not Go). Raise-only and envelope-clamped through
// clampedRaise, exactly as the ADR-0076 D floor.
//
// Why: repair rounds re-dispatched at the identical tier and effort do not
// converge — cycles 1595–1605 ran balanced/balanced/balanced and ship
// probability by audit-round count fell 100 % → 50 % → 17 % → 0 % (research:
// docs/research/ship-rate-harness-reliability-2026-09-02.md, R1). Effort
// follows the tier via the profile's effort_overrides at the bridge launch.
```

### `go/internal/core/retry_tier_escalation_test.go:3` — above `import (`

```text
// retry_tier_escalation_test.go — ADR-0076 slice D pins (adversarial-review
// amended design): the escalation is a deterministic DISPATCH floor —
// mode-independent, raise-only, clamped through the real envelope guardrail
// (single-entry ClampPlanModelRouting — never a second clamp), driven by the
// max failure_count across the cycle's scoped items (lane scope ∪ this
// cycle's processing claims).
```

### `go/internal/core/reviewer.go:54` — above `Demoted bool`

```text
// Demoted marks that the contract gate's own circuit breaker gave up and
// demoted enforce→advisory (deliverable.Reviewer's consecutive-block
// breaker). Without this flag a demotion is structurally identical to a
// compliant deliverable, so the orchestrator cannot report that a gate
// stopped being enforced — the batch-19/batch-21 blind spot (inbox
// contract-block-cli-escalation). Reason carries the last violation.
//
// Normally paired with Approve=true (the demotion IS the approval), but
// ChainReviewers carries it onto a rejection from a LATER gate in the chain
// too: the contract gate stopped enforcing whether or not topngate/triagecap
// went on to reject the same deliverable.
```

### `go/internal/core/reviewer.go:76` — above `Remediation string`

```text
// Remediation is an OPTIONAL, gate-authored instruction describing how to
// SATISFY this specific violation. Empty for every gate that does not know
// how to fix its own rejection — and empty means the correction directive is
// byte-identical to what it has always been.
//
// It exists because one generic directive cannot serve two different failure
// classes. The default text is written for "the contracted artifact exists
// but is malformed" and ends with "Do not change unrelated files"; for a
// violation whose remedy is to CREATE a missing sidecar artifact that clause
// forbids the fix. Gate A (evals-materialized) is that case, and it recovered
// 0 of 4 times in production (cycles 1471/1476/1504/1531, each "rejected
// after 2 correction(s)").
```

### `go/internal/core/reviewer.go:137` — above `type ContractVerification struct {`

```text
// ContractVerification is a breaker-neutral well-formedness verdict for one
// phase deliverable (ADR-0045 I2). ArtifactPath is the CONTRACTED destination
// — the only path the salvage rung may relocate to.
```

### `go/internal/core/reviewer.go:146` — above `type ContractVerifier interface {`

```text
// ContractVerifier re-checks a phase's deliverable WITHOUT touching the
// contract-gate circuit breaker. The I2 integrity rule (cycle-265 forensics):
// the correction ladder's intermediate rung re-checks (salvage's
// verify-after-move, live-fix's post-window re-verify) must never increment
// the GLOBAL breaker in deliverable/reviewer.go — a multi-rung repair attempt
// would otherwise count three blocks for one flaky deliverable and silently
// demote the contract gate batch-wide. Only the ladder's FINAL outcome goes
// through DeliverableReviewer.Review. The error follows deliverable.Verify's
// fail-open contract: err => ambiguity (unknown phase) => the caller skips
// the rung rather than acting blind.
```

### `go/internal/core/reviewer.go:259` — above `type contractGateSignals interface {`

```text
// contractGateSignals is the capability the composition-root wiring proof
// asks for beside declaredDeliverablesGate: the contract gate reports its
// decisions through the Signal Center (ADR-0101 S2b).
```

### `go/internal/core/reviewer_chain_test.go:20` — above `func TestChainReviewers_AllApprove(t *testing.T) {`

```text
// ChainReviewers composes multiple DeliverableReviewers (e.g. evalgate then the
// deliverable-contract gate). It approves only when ALL approve; the first
// rejection short-circuits with its reason. ADR-0034.
```

### `go/internal/core/routing_dispatch.go:26` — above `advanced := false`

```text
// Skip-advance: when the static successor is a phase the router has
// declined (dec.SkipPhases — e.g. an EnableOff optional like build-planner,
// or a plan veto), advance to the next NON-skipped phase so the spine-decline
// fallback below never lands on a vetoed phase (cycle-238 D1).
//
// GUARD (cycle-240 e2e regression): nextInOrder reads o.cfg.Order, which is
// EMPTY when no phase-registry.json is present (the e2e fixtures, and any
// repo without the registry). An empty/exhausted order makes nextInOrder
// return PhaseEnd, which would silently rewrite staticNext to "end" — turning
// "skip this optional phase" into "terminate the cycle before build/audit/
// ship". Only advance while the order can name a real successor; if it yields
// PhaseEnd we cannot trust it, so keep the original staticNext (the cand
// override + downstream gates still drive forward exactly as pre-regression).
```

### `go/internal/core/routing_dispatch.go:178` — above `if o.catalogPublisher != nil {`

```text
// Publish the LIVE catalog so consumers holding a resolver bound over the
// cycle-START catalog value re-bind (the bridge's deliverable-contract
// resolver, cmd_cycle.go). Catalog.Merge returns a NEW value over a NEW
// map, so without this the minted phase is invisible to contract injection
// for the rest of the cycle — cycle-1424's naked dispatch and its 600s
// artifact timeout. Fired only AFTER a mint fully succeeds: a rejected or
// colliding mint continues above and publishes nothing (no routable ghost).
```

### `go/internal/core/routing_dispatch.go:238` — above `func (o *Orchestrator) leakRecoverablePhase(p Phase) bool {`

```text
// leakRecoverablePhase reports whether next is eligible for main-tree leak
// recovery. It is the UNION of the fixed active-worktree set (LeakRecoverablePhase
// — triage/audit/scout/bug-reproduction/tdd/build) with worktreePhase, so a user
// phase that opts into source writes via its spec (writes_source) keeps the
// recovery it had before cycle-564 while the four non-source-writing built-ins
// gain it. Distinct from worktreePhase (write PERMISSION) by design.
```

### `go/internal/core/routing_dispatch.go:248` — above `func phaseFromRouter(s string) Phase {`

```text
// phaseFromRouter denormalizes a router phase string back to a core.Phase.
// The router speaks canonical "retrospective"/"end"; core uses "retro"/
// PhaseEnd. An unknown string yields "" so enforceNext declines it.
//
// The core↔registry vocabulary skew this bridges is a DECIDED permanent boundary
// (ADR-0060 §57), not debt: do NOT "unify" PhaseRetro's wire string — "retro" is
// the trust-kernel serialized identity in state.json/ledger (pinned by
// cyclestate.TestPhaseConstants), so the converter stays; the rename does not.
```

### `go/internal/core/routing_dispatch.go:270` — above `func canonicalCatalogName(p Phase) string {`

```text
// canonicalCatalogName maps a core.Phase to its catalog/registry key, bridging
// the core↔router vocabulary skew (PhaseRetro stringifies to "retro" but the
// registry names the phase "retrospective"). It is the inverse of
// phaseFromRouter's alias cases — used by specFor so a descriptor lookup cannot
// silently miss on the skew and fall through to a wrong edge (ADR-0058). The skew
// is a decided permanent boundary (ADR-0060 §57); this converter is the accepted
// solution, not a deferred unification.
```

### `go/internal/core/routing_dispatch.go:278` — above `return phasecontract.RegistryKey(string(p))`

```text
// ONE rule, owned by phasecontract (ADR-0100): the registry key for a
// core phase name. Keeping a second copy here is how an alias added to
// one side silently ungated a phase on the other.
```

### `go/internal/core/runlease_hook.go:12` — above `func leaseRefreshInterval() time.Duration { return runlease.DefaultTTL / 2 }`

```text
// runlease_hook.go — ADR-0049 G16: the per-run .lease PRODUCER.
//
// runlease (the SSOT for the heartbeat file) was already CONSUMED by gc's
// liveness check (a run dir with a fresh lease is LIVE, never collected), but
// nothing WROTE one — so every concurrent fleet run was leaseless and a gc pass
// could reap a sibling's run dir mid-cycle. RunCycle now writes the lease at
// cycle start and refreshes it on a heartbeat for the run's lifetime.
//
// The producer is per-cycle and in-process (not a central fleet pump): each
// cycle owns its own lease, so under `evolve fleet` each child process leases
// its own run dir with no shared writer. The heartbeat is a bare ticker — it
// only sleeps, so a legitimately-busy-but-alive think-heavy phase never stalls
// it (we deliberately do NOT couple liveness to phase progress, which would
// reintroduce the busy-pane false-negative class).
```

### `go/internal/core/runworkspace.go:39` — above `func RunIDFromWorkspace(workspace string) string {`

```text
// RunIDFromWorkspace resolves the run identity recorded in a run workspace's
// run.json mirror. It is the SINGLE resolver every out-of-process ledger writer
// uses to stamp run_id, so the identity ship's run-scoped binding lookup keys on
// has one derivation rather than one per writer.
//
// Cycle-1571 H1: PR #503 made run_id load-bearing at the ship gate (a binding
// lookup refuses an entry that is not THIS run's), on the premise that every
// recorder already stamped it. Three of the four agent_subprocess writers did
// not — they run in a separate process from the orchestrator, so the in-memory
// currentRunID that stampingLedger uses is simply unavailable to them. The run
// workspace they are already handed carries the id on disk.
//
// Fail-SOFT by design: an unresolvable id returns "" and the caller OMITS the
// field rather than stamping an empty identity. The fail-CLOSED half belongs to
// the consumer — ship refuses to bind an unstamped entry — and inventing or
// zero-filling an identity here would defeat exactly that.
```

### `go/internal/core/runworkspace.go:81` — above `const CycleStateFile = "cycle-state.json"`

```text
// CycleStateFile is the global per-cycle state file under .evolve/. The single
// home for the filename (was a string literal repeated across storage /
// checkpoint / inboxmover / resume / reset). Every read-modify-writer of this
// file serializes on the sidecar "<dir>/cycle-state.json.lock" via
// flock.WithPathLock (ADR-0049 G7) so concurrent fleet cycles never lose each
// other's update.
```

### `go/internal/core/runworkspace.go:89` — above `func RunWorkspacePath(projectRoot string, cycle int) string {`

```text
// RunWorkspacePath is core's single spelling of a cycle's run-workspace
// directory, <projectRoot>/.evolve/runs/cycle-<N> — a projection of
// paths.RunWorkspace, the layout's one home (ADR-0103 unit 09). Phase
// artifacts, the tmux session registry (CB.5) and the run.json guard mirror
// (CB.4) all live here.
```

### `go/internal/core/runworkspace_runid_test.go:9` — above `func TestRunIDFromWorkspace_ReadsRunJSON(t *testing.T) {`

```text
// runworkspace_runid_test.go — RunIDFromWorkspace is the single resolver every
// OUT-OF-PROCESS agent_subprocess ledger writer uses to stamp run_id.
//
// Cycle-1571 follow-up (H1). PR #503 removed ship's cross-run binding fallback
// on the stated premise that "every current recorder stamps run_id". Three of
// the four agent_subprocess writers did not: subagent/run.go's hand-built JSON
// literal, subagent/subagent.go's LedgerEntry, and cyclesimulator's map. Only
// core/phase_bindings.go was stamped, and only because it routes through the
// Orchestrator's stampingLedger. The premise was checked empirically against
// recent ledger rows — all written by the one writer that DOES stamp — instead
// of structurally against every writer. Consequence: an auditor entry written
// by `evolve subagent run` (the sanctioned manual re-audit) can never match a
// run-scoped lookup, so ship hard-stops AUDIT_BINDING_NO_AUDITOR where it
// previously bound and shipped.
//
// The run workspace is already on hand at every one of those call sites, and
// its run.json already carries run_id (cyclestate.CycleState.RunID, mirrored by
// adapters/storage.mirrorRunState).
```

### `go/internal/core/runworkspace_runid_test.go:31` — above `mustWriteRunState(t, ws, '{"cycle_id":1519,"phase":"aborted","run_id":"01M09657TDN6Q1VMJK1XKYR376"}')`

```text
// Shape copied from a real file: .evolve/runs/cycle-1519/run.json.
```

### `go/internal/core/runworkspace_unit09_test.go:11` — above `func TestRunWorkspacePath_ProjectsPathsLayout(t *testing.T) {`

```text
// TestRunWorkspacePath_ProjectsPathsLayout is the consumer pin (ADR-0103 unit
// 09): core's RunWorkspacePath is a projection of paths.RunWorkspace, the one
// home of the <root>/.evolve/runs/cycle-<N> layout — a divergence is red, not
// silent.
```

### `go/internal/core/s5b_rebase_test.go:65` — above `func TestRebaseCycleBranchOntoMain_CleanDisjoint_Succeeds(t *testing.T) {`

```text
// TestRebaseCycleBranchOntoMain_CleanDisjoint_Succeeds pins ADR-0049 S5b-2b: the
// advisor-partitioned (disjoint-file) case rebases cleanly so the cycle can
// re-audit + re-ship the merged tree.
```

### `go/internal/core/safety_invariants.go:11` — above `func ValidateSafetyInvariants(sm *StateMachine, cfg config.RoutingConfig, cat phasespec.Catalog) []string {`

```text
// ValidateSafetyInvariants is the phase-agnostic load-time trust anchor
// (ADR-0060). As the transition kernel becomes data-driven (PA-DDK), the
// legality graph and gates move into config; the floor's non-gameability can no
// longer rest on a hardcoded graph literal. This validator replaces it: it
// HARD-checks that the transition graph + config preserve the ship floor,
// quantified over the graph and config ROLES (mandatory anchors, verdict
// branches) — never phase-name literals — so an operator may rename any phase
// without weakening the floor. Returns human-readable violations; empty == safe.
//
// DDK-1 lands the two invariants checkable before the graph/gates are
// config-driven; DDK-4/DDK-5 extend it with the artifact-gate and
// path-dominance invariants as those fields go live, so each check lands BEFORE
// its corresponding config-flip and the floor is never unguarded.
```

### `go/internal/core/safety_invariants.go:27` — above `for _, name := range cat.Names() {`

```text
// I8 — branch-target legality: a phase's verdict-branch targets (on_pass /
// on_fail) must resolve to a known phase AND be a legal successor. Config may
// only SELECT among already-legal edges, never invent one (ADR-0058 §1, now
// enforced as a data check rather than by a hardcoded graph).
```

### `go/internal/core/safety_invariants.go:104` — above `evals := mandatoryEvaluators(cfg, cat)`

```text
// Floor-evaluator existence (PA-DDK DDK-5 hardening — the operative half of
// ADR-0060 §4 "F⊆M"): the ship floor is only real if SOME mandatory phase
// gates on a shippable verdict — a mandatory EVALUATOR must exist. Without it
// an operator can drop the evaluator from mandatory_phases and
// SpineSatisfiedUpTo admits ship with no verdict gate (the runtime anchor goes
// inert — see TestSpineSatisfiedUpTo_ConfigurableMandatoryWeakensGate). Phase-
// agnostic: quantified over mandatory phases' gates, never a phase name. The
// floor-gate check above forbids a NON-shippable mandatory gate; this forbids
// the ABSENCE of one (and a presence-only gate with empty verdict_in, which a
// FAIL verdict would otherwise slip through).
//
// Only judged when the catalog is AUTHORITATIVE over the floor — it describes
// at least one mandatory phase. The real composition root always passes the
// full registry catalog, and a production tamper (dropping the evaluator from
// mandatory) leaves the other mandatory phases described, so the check still
// fires. A synthetic/empty catalog (unit tests of orchestration mechanics)
// cannot describe the evaluator's gate, so judging it would false-positive.
//
// evals (the floor evaluator SET) is computed once here and reused by the
// graph-dominance check below: existence needs its size, dominance needs the set.
```

### `go/internal/core/safety_invariants.go:146` — above `if len(evals) > 0 && len(anchors) > 0 {`

```text
// Evaluator-dominance (ADR-0060 §4 I1/I2, the load-bearing half made a
// load-time check): every start→ship path must traverse a floor
// evaluator. The evaluator SET is identified by role (mandatory phases
// gating a shippable verdict) and the ship sink by role — the LAST
// mandatory anchor, which is the ship terminal by registry convention
// (mandatoryAnchorsFor preserves cfg.Order, so anchors are ordered and the
// terminal is last). If the sink is still reachable with the WHOLE
// evaluator set deleted from the graph, some path reaches ship without an
// evaluator — the verdict floor is bypassable at the graph level (e.g. a
// legal_successors edge straight to ship). Phase-agnostic; never a phase
// name. All-anchor dominance (a non-evaluator anchor like a triage step)
// stays runtime-backstopped by SpineSatisfiedUpTo — only the evaluator
// carries the verdict floor, so only it is proven here at load.
```

### `go/internal/core/safety_invariants_test.go:3` — above `import (`

```text
// safety_invariants_test.go — PA-DDK DDK-1/DDK-3 (ADR-0060). The validator is the
// relocated trust anchor. These tests LOAD the real phase configuration via the
// kerneltest fixture and reference phases through STRUCTURAL accessors
// (FirstAnchor/ShipTerminal/Evaluator), never by hardcoded name — so renaming a
// phase in the registry does not require rewriting any test. Adversarial cases
// are built by mutating the loaded config with fixture-derived names.
```

### `go/internal/core/safety_invariants_test.go:113` — above `func TestValidateSafetyInvariants_NoMandatoryEvaluatorRejected(t *testing.T) {`

```text
// TestValidateSafetyInvariants_NoMandatoryEvaluatorRejected (DDK-5 hardening,
// ADR-0060 §4 F⊆M): dropping the evaluator from mandatory_phases means no
// mandatory phase gates the verdict, so SpineSatisfiedUpTo would admit ship with
// no audit (the runtime anchor goes inert). The validator must reject it at load.
// The evaluator name comes from the loaded config, not a literal.
```

### `go/internal/core/safety_invariants_test.go:170` — above `func TestValidateSafetyInvariants_NoStartNodeRejected(t *testing.T) {`

```text
// TestValidateSafetyInvariants_NoStartNodeRejected: a transition graph in which
// every node has an incoming edge (a pure cycle, no in-degree-0 source) has no
// start node, so reachability cannot be rooted and the floor cannot be proven —
// the validator must reject it. The synthetic 2-cycle is injected via
// WithLegalGraph; the Phase constants here are arbitrary bare-graph vocabulary
// (ADR-0060 §"Kernel test vocabulary"), not flow identity.
```

### `go/internal/core/safety_invariants_test.go:188` — above `func TestValidateSafetyInvariants_EvaluatorBypassRejected(t *testing.T) {`

```text
// TestValidateSafetyInvariants_EvaluatorBypassRejected (ADR-0060 §4 I1/I2 —
// graph-dominance, load-bearing half): a legal_successors edge that lets a path
// reach the ship sink WITHOUT traversing the floor evaluator (here: the first
// anchor jumps straight to ship, bypassing the audit-class evaluator) is rejected
// at load. Evaluator and ship sink are identified by ROLE (mandatory
// shippable-verdict phase / last mandatory anchor), never by name — a rename does
// not weaken the check. This catches at LOAD what was previously only caught at
// runtime by SpineSatisfiedUpTo (ADR-0060 §"Implemented invariants vs. design intent").
```

### `go/internal/core/safety_invariants_test.go:208` — above `func TestValidateSafetyInvariants_TwoEvaluatorsCollectiveDominance(t *testing.T) {`

```text
// TestValidateSafetyInvariants_TwoEvaluatorsCollectiveDominance (ADR-0060 §4): when
// two evaluators each gate a DISJOINT path to ship, every start→ship path still
// crosses AN evaluator, so the floor holds and the config is safe. The dominance
// check must delete the evaluator SET as a whole — deleting only one evaluator
// would leave the other's path open and false-positive. This locks the
// collective-dominance semantic against a future one-at-a-time regression. The
// Phase constants are bare-graph vocabulary (ADR-0060 §"Kernel test vocabulary").
```

### `go/internal/core/scope_path_disclosure_test.go:3` — above `import (`

```text
// scope_path_disclosure_test.go — a lane's assigned ids reach its phases WITH
// the live record's path, never as bare names.
//
// cycle-1548 (soak-20260823a): the scope id resolved to 17 on-disk records —
// 1 live, 16 consumed namesakes — the prompt carried a bare string, and every
// phase worked a record from a halt cured two weeks earlier (PR #421). The
// orchestrator resolves paths through an INJECTED resolver (core cannot import
// inboxmover: inboxmover -> adapters/ledger -> core), the same composition-root
// seam WithContinuationResolver uses. Nil resolver = byte-identical Context.
```

### `go/internal/core/seams_test.go:11` — above `var (`

```text
// TestArchitectureSeams_FoundationsExist is a standing guard for ADR-0052
// advisor-maximization (WS0-S1). The design hooks a set of load-bearing seams;
// a concurrent refactor that renames or removes one must fail HERE, at the
// start of the advisor work, rather than midway through an implementation slice.
//
// The package-level references below are the real guard: each names a seam by
// its exact identity (receiver + method/func name via a method expression), so
// a rename or signature-incompatible move breaks the build. The single runtime
// check covers panetrust.redactSecrets, which is unexported and so cannot be
// referenced across the package boundary.
```

### `go/internal/core/seed_phase_e2e_test.go:25` — above `func TestSeedPhase_BugReproductionReachesAdvisorCatalog(t *testing.T) {`

```text
// TestSeedPhase_BugReproductionReachesAdvisorCatalog is the ADR-0038 end-to-end
// proof: a user phase living as pure config under .evolve/phases/ flows from
// the real merged catalog into an enriched advisor card, so the advisor can
// make an informed SELECT on bugfix cycles. (The original ADR-0038 seed was
// named reproduce-bug; the two-tier naming rule renamed it bug-reproduction.)
```

### `go/internal/core/ship_recovery.go:33` — above `recoverCode, recoverClass := se.Code, se.Class`

```text
// ADR-0049 S5b: a fleet ff-merge divergence (a peer cycle moved main) is
// recovered by rebasing the cycle branch onto the new main BEFORE the
// re-audit (the router routes this code to audit). A clean rebase replays
// this cycle's patches onto the peer's changes → re-audit re-binds the merged
// tree → re-ship fast-forwards. A conflict confined to GENERATED projections
// (e.g. control-flags.md) is auto-resolved by regenerating them from the
// merged source (rebaseWithDerivedRegen) — every flag cycle rewrites that
// projection, so the partition cannot separate them and a debugger round-trip
// would be pure waste. A conflict touching any NON-derived path is genuine
// overlapping work the partition should have kept apart — abort loud.
// The code/class used for the routing decision; a fleet rebase may reclassify
// a clean RebaseNeeded into a CONFLICT (G13a) that routes to the debugger.
```

### `go/internal/core/ship_recovery.go:47` — above `if cs.ActiveWorktree != "" {`

```text
// Deterministic, zero-LLM pre-screen (cycle-968) BEFORE the blind rebase
// replay: ClassifyFleetRebaseCandidate reuses the cycle-962 carry-forward
// filter to decide whether this candidate is already landed, cleanly
// mergeable, or a genuine conflict. A superseded (already-landed) candidate
// short-circuits here with NO wasted rebase + re-audit — the explicit fix
// for the 948 "PASS-but-unlanded duplicate" waste class. Clean/Conflict fall
// through to the existing rebaseCycleBranchOntoMain path unchanged (which
// itself replays a clean candidate and routes a real conflict to the
// debugger). A pre-screen git-infra error is non-fatal here: log it and let
// the rebase below run and report its own infra failure loudly.
```

### `go/internal/core/ship_recovery.go:99` — above `if o.compositionCarryForward(ctx, cycle, *cs, projectRoot) {`

```text
// A clean replay MAY carry the audit verdict forward without a
// full re-audit (RUNG 0, cycle-801): if the composed diff's
// patch-id still matches what the audit reviewed and every
// composed-tree gate is green, write a composition-verdict entry
// and reship directly. Any rejection falls through unchanged to
// the pre-existing route below (router routes RebaseNeeded to
// audit, re-binding the merged tree).
```

### `go/internal/core/ship_recovery.go:109` — above `if o.scopedMergeCarryForward(ctx, cycle, *cs, projectRoot) {`

```text
// RUNG 2 (cycle-941): a RUNG 0 miss means the composed patch-id
// drifted (real overlapping edits). Before the RUNG 3 full
// re-audit, review ONLY the intersecting hunks; a compatible,
// patch-id-verified overlap composes directly with a
// composition-verdict{method:"scoped-review"} and reships.
// Entangled (or a dark reviewer) falls through unchanged.
```

### `go/internal/core/ship_recovery.go:170` — above `var derivedArtifacts = map[string]derivedArtifactSpec{`

```text
// derivedArtifacts is the single classifier for GENERATED projections that a flag
// cycle edits indirectly (via the registry) and that must be regenerated from the
// merged source — NEVER hand-maintained by the LLM builder (cycle-11 H1). It feeds
// BOTH the post-build normalizer (normalizeDerivedProjections — deterministic regen
// before audit, like build-gofmt) and the fleet rebase recovery
// (rebaseWithDerivedRegen — auto-resolve a rebase conflict confined to these paths).
//
// control-flags.md is the flag registry's projection (cmd_flags.go: `evolve flags
// generate` splices flagregistry.RenderIndex() into a marker region). Every
// flag-reduction cycle rewrites its whole marker region, so the projection drifts
// unless regenerated; regeneration IS the projection re-run (single-source-with-
// projection: no second renderer).
//
// Keep in sync with TestDerivedArtifacts_MapIntegrity (each key is a real on-disk
// file carrying a GENERATED marker; each ssotPrefix is a real source path).
```

### `go/internal/core/ship_recovery.go:258` — above `func rebaseCycleBranchOntoMain(ctx context.Context, projectRoot, worktree string) (ok bool, conflict bool) {`

```text
// rebaseCycleBranchOntoMain rebases the cycle's worktree branch onto the current
// main so a fleet cycle whose ff-merge diverged (a peer moved main) can re-audit
// + re-ship the merged tree (ADR-0049 S5b). Returns ok=true on a clean replay OR
// when every conflict was confined to derived projections that were regenerated
// from the merged source: the re-audit re-binds the regenerated tree, so the
// ship-time tree-SHA binding (ship/gitops.go) still holds — integrity-safe. A
// conflict touching any NON-derived path (genuine overlapping work, incl. the
// SSOT itself) returns conflict=true → the debugger. Infra failures and failed
// regenerations return (false,false). The in-progress rebase is always aborted on
// a non-ok return so the worktree is left clean. An empty worktree returns
// (false,false) — a degraded run never rebases.
```

### `go/internal/core/ship_recovery_budget.go:12` — above `func isContentionShipCode(code ShipErrorCode) bool {`

```text
// Width-scaled contention recovery (cycle-765, cycle-759 incident): with N
// fleet lanes racing one main, P(HEAD moves during your audit→ship window)
// grows with N, but the recovery budget was a constant maxRecoveryDepth=2 —
// at width 3+ that guarantees a steady abort rate that looks like "loop
// failure" while being pure landing-queue contention. Contention-class codes
// get a budget that scales with fleet width; everything else keeps the
// constant budget so width can never inflate retries for genuine failures.
```

### `go/internal/core/ship_recovery_composition_test.go:1` — above `package core_test`

```text
// ship_recovery_composition_test.go — RED contract for wiring the RUNG 0
// composition-verdict writer into the live fleet-rebase recovery path
// (cycle 801, inbox weight 0.98, campaign merge-efficiency-2026-07).
//
// Ship's trivial-rebase carry-forward reader (internal/phases/ship/
// composition.go) and the ledger's kernel-recomputable writer
// (internal/adapters/ledger/composition.go) are both fully built and unit-
// tested (cycle-786), but nothing in production code ever calls
// ledger.WriteCompositionVerdict — the one call site that could produce a
// composition-verdict entry for a CodeGitFleetRebaseNeeded recovery
// (recoverFromShipError, ship_recovery.go:45) always falls through to a
// full re-audit (router.Recover routes GIT_FLEET_REBASE_NEEDED → "audit"
// unconditionally, router/recovery.go:110). Note internal/core cannot
// import internal/adapters/ledger directly (ledger already imports core —
// an import cycle), so the fast path must be wired the same way core
// already wires catalogRefresh/modelCatalogLookup/directivesProvider:
// exported Option-injected closures the composition root (cmd/evolve)
// binds to the real ledger adapter; core itself stays adapter-agnostic.
//
// This file pins the OBSERVABLE, black-box contract (package core_test,
// driven only through the public core.NewOrchestrator/RunCycle API — it
// does not prescribe recoverFromShipError's internal signature, only the
// new exported seams and the routing behavior Builder must make true):
//
//  1. Two new exported types:
//     - core.CompositionAuditSnapshot{LaneAuditRef, AuditedBase string;
//     Diff []byte; PatchID string} — what the bound audit reviewed
//     for this lane BEFORE a peer moved main.
//     - core.CompositionVerdictInput — mirrors
//     ledger.CompositionVerdictInput field-for-field so the composition
//     root's injected writer closure can translate 1:1 into a real
//     ledger.WriteCompositionVerdict call.
//  2. Three new exported Options, each nil by default (⇒ the composition
//     fast path never fires; recovery behaves exactly as it does today):
//     - core.WithCompositionSnapshot(func(ctx, worktree, runID string)
//     (core.CompositionAuditSnapshot, error)) — captures the lane's
//     pre-rebase audited state.
//     - core.WithCompositionGateRunner(func(ctx, worktree string)
//     map[string]string) — runs the full native composed-tree gate set
//     (ciparity.RequiredComposedGates) on the rebased tree.
//     - core.WithCompositionVerdictWriter(func(ledgerPath string,
//     in core.CompositionVerdictInput) error) — persists the entry.
//  3. On a CLEAN fleet rebase for CodeGitFleetRebaseNeeded (the existing
//     rebaseCycleBranchOntoMain ok==true branch), when all three seams are
//     wired: the composed diff's recomputed patch-id must match the
//     snapshot's PatchID (drift → fall back, unchanged — same semantic-
//     drift guard ship's own tryTrivialRebaseCarryForward already
//     enforces on read) AND every ciparity.RequiredComposedGates entry
//     must be "pass" (ciparity.MissingComposedGates nil) before the writer
//     is ever called. Only when the writer succeeds does recovery route
//     straight back to ship WITHOUT re-running audit. Any rejection
//     (missing seam, patch-id drift, red gate, writer error) falls through
//     to the pre-existing full re-audit route unchanged — the fast path
//     can only narrow, never widen, what ships.
//
// RED today via this file's compile dependency on the not-yet-defined
// exported symbols above (same convention as ship_recovery_width_test.go's
// white-box RED).
```

### `go/internal/core/ship_recovery_composition_test.go:331` — above `if fx.audit.calls != 2 {`

```text
// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
// re-audit = 2 total for one fallback (see the note in the sibling
// success test above and TestOrchestrator_RecoveryDepthBudget).
```

### `go/internal/core/ship_recovery_composition_test.go:376` — above `if fx.audit.calls != 2 {`

```text
// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
// re-audit = 2 total (see note above).
```

### `go/internal/core/ship_recovery_composition_test.go:414` — above `if fx.audit.calls != 2 {`

```text
// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
// re-audit = 2 total (see note above).
```

### `go/internal/core/ship_recovery_width_test.go:3` — above `import (`

```text
// ship_recovery_width_test.go — RED contract for width-scaled-binding-retry
// (cycle 765, inbox weight 0.93, cycle-759 incident: AUDIT_BINDING_HEAD_MOVED
// exhausted a FIXED budget of 2 recoveries and aborted a clean cycle under
// normal width-3 landing-queue contention).
//
// Contract encoded here (Builder implements; DO NOT modify these tests):
//
//  1. The fleet supervisor advertises lane width via the SSOT IPC env key
//     ipcenv.FleetWidthKey ("EVOLVE_FLEET_WIDTH"), read from CycleRequest.Env
//     (never os.Getenv — fleet siblings must not leak into each other).
//  2. shipRecoveryBudget(code ShipErrorCode, fleetWidth int) int is the pure
//     budget classifier: contention-class codes (every AUDIT_BINDING_* code
//     and GIT_FLEET_REBASE_NEEDED) get max(2, fleetWidth+1); every other
//     code keeps the constant maxRecoveryDepth. Absent/garbage/non-positive
//     width resolves to 1 (solo), i.e. the constant budget.
//  3. Between contention re-audit attempts the orchestrator sleeps a JITTERED
//     positive backoff through the existing backoffSleep seam (so TestMain's
//     no-op keeps the suite fast and siblings don't re-collide in lockstep).
//     Jitter must be drawn at millisecond granularity or finer so distinct
//     attempts virtually never sleep identical durations.
//
// White-box (package core): reuses the internal fakes from orchestrator_test.go
// and the backoffSleep seam, which the external core_test package cannot reach.
```

### `go/internal/core/ship_recovery_width_test.go:71` — above `func persistentContentionShip() *widthShipStub {`

```text
// persistentContentionShip builds a ship runner that always fails with the
// cycle-759 contention error (a sibling landed during the audit→ship gap).
```

### `go/internal/core/shiperror_manifestgate_test.go:9` — above `func TestCodeManifestGate_DualRegistered(t *testing.T) {`

```text
// TestCodeManifestGate_DualRegistered asserts the dedicated manifest-gate code
// is re-exported through core exactly like every other ship code (the
// CodeCommitPrefixGate dual-registration pattern). Consumers import core, not
// shiperr, so a code that exists only in shiperr is unreachable from the ship
// phase and the debugger-routing layer (cycle-1064).
```

### `go/internal/core/shipped_via_build_own_ship_test.go:8` — above `func TestRunCycle_EmptyTriageCommitmentSurvivesSiblingLanding(t *testing.T) {`

```text
// shipped_via_build_own_ship_test.go — a cycle that shipped nothing must not be
// labelled SHIPPED_VIA_BUILD because a SIBLING lane moved main.
//
// Live incident (two-wave health batch, 2026-09-12): cycle 1630 ran scout and
// triage, triage honestly committed `top_n: []` (its inbox claim had failed),
// and the host ended the cycle `triage-empty-commitment`. Between cycle start
// and closeout a sibling lane landed on main, so the pre/post HEAD probes
// differed — and finalizeOutcome relabelled the SKIPPED no-work verdict as
// SHIPPED_VIA_BUILD (loop-20260912-healthcheck.log:197). Three consumers then
// misread the cycle: IsTriageNoWorkResult no longer held (the shortened ledger
// floor was refused), the throughput recorder credited a cycle that committed
// nothing, and the dossier recorded PASS. No persona instructs an inline ship
// and lane commits carry no per-cycle trailer, so HEAD movement can never prove
// THIS cycle shipped; the only evidence is the cycle's own ship latch
// (CycleState.Shipped, set by latchShippedState on both dispatch roots).
//
// The fixture is cycle 1630's exact shape: the composed RunCycle path (not a
// hand-built result), a real triage-decision.json with an empty commitment, and
// a HEAD probe that answers differently at cycle start and at closeout.
```

### `go/internal/core/shipped_via_build_own_ship_test.go:40` — above `heads := []string{"a2e60952-cycle-start", "b496a8dc-sibling-landed"}`

```text
// A sibling lane lands on main mid-cycle: the probe at cycle start and the
// probe at closeout return different SHAs, exactly as cycle 1630 saw.
```

### `go/internal/core/shipped_via_build_own_ship_test.go:69` — above `func TestCompleteCycle_ForwardsTheShipLatchToTheOutcomeLabel(t *testing.T) {`

```text
// The closeout is the one consumer of the latch: completeCycle hands the
// persisted CycleState (with its Shipped flag) to finalizeCycle. No composed path in the default
// pipeline yields a SKIPPED final verdict AFTER a ship PASS (the floor guard
// declines post-ship non-floor verdicts, and an abort returns before closeout),
// so a closeout that silently dropped the latch would leave every RunCycle test
// green. This pins the seam directly, in the one shape that can reach it: a
// configured ship-floor override whose post-ship floor phase SKIPPED. The
// negative row is the cycle-1630 twin at the same seam.
```

### `go/internal/core/shipwindow_wiring.go:18` — above `func (cr *cycleRun) acquireShipWindow(next Phase) {`

```text
// acquireShipWindow serializes the audit→ship critical section across lanes
// (cycle-778 ship-window-lease): taken immediately BEFORE the audit-binding
// HEAD snapshot (emitPhaseBindings → recordAuditBinding) and held through
// ship's push, so a sibling landing on main inside that window queues instead
// of forcing this lane into a deep-tier re-audit. Deliberately NOT held
// across the audit agent itself — only binding-snapshot→push. A re-audit
// while already holding releases first (FIFO fairness to waiting siblings +
// a fresh heartbeat on the new hold). Acquisition failure/timeout WARNs and
// proceeds unleased.
```

### `go/internal/core/shipwindow_wiring_test.go:13` — above `func TestShipWindowWiring_AuditAcquiresNonAuditReleases(t *testing.T) {`

```text
// Cycle-778 regression: the ship-window lease is acquired exactly at the
// audit-phase binding boundary, held across it, and freed by the first
// non-audit completion and by releaseShipWindow's idempotent re-entry.
```

### `go/internal/core/shipwindow_wiring_test.go:50` — above `func TestShipWindowWiring_FailOpenWhenSiblingHolds(t *testing.T) {`

```text
// Cycle-778 fail-open contract: a sibling holding the window must delay this
// lane at most shipWindowAcquireTimeout, after which the lane proceeds
// UNLEASED (pre-lease behavior) instead of wedging the loop.
```

### `go/internal/core/signal.go:3` — above `import (`

```text
// signal.go — ADR-0101 S1: the orchestrator is a registered LISTENER of the
// Signal Center, and the ADR-0044 C1 chokepoint (recordPhaseOutcome) is the
// first PRODUCER. Listeners observe, never decide: the summary kept here is
// reporting evidence; verdicts, transitions and halts stay with the floors.
//
// Lock order: the Center holds no lock while delivering, so observeSignal
// takes only signalMu; no orchestrator path holds signalMu across an Emit,
// and observeSignal never emits (no feedback loops).
```

### `go/internal/core/signal.go:27` — above `CodeAuditRepairDeclined signalcenter.Code = "ORCHESTRATOR_AUDIT_REPAIR_DECLINED"`

```text
// CodeAuditRepairDeclined / CodeAuditRepairGranted: decideAfterAuditFail's
// verdict on an audit FAIL — the decision that sent cycle 1684 through a
// full retrospective before a retry was invisible when its invented class
// declined the direct grant.
```

### `go/internal/core/signal.go:111` — above `const CodeGateCorrection signalcenter.Code = "ORCHESTRATOR_GATE_CORRECTION"`

```text
// CodeGateCorrection is the correction ladder's code (ADR-0101 S2b): one
// gate.corrected INFO per rung the orchestrator runs after a gate rejection.
```

### `go/internal/core/signal_cycle.go:3` — above `import (`

```text
// signal_cycle.go — ADR-0101 S2a producers. Each is an Adapter from a typed
// value the pipeline already owns (CycleResult, SystemFailureSignal, ShipError,
// the quota sentinel) to ONE Event at the chokepoint that owns the fact:
// cycle.sealed + system.failure at completeCycle (the closeout both dispatch
// roots share), ship.error at recordShipError, quota.paused at the
// quota pause (pauseForQuota, both roots); the abnormal epilogue seals the
// aborted cycle. Producers observe; every decision stays where it was.
```

### `go/internal/core/signal_cycle.go:74` — above `func (o *Orchestrator) emitShipError(cycle int, cs CycleState, se *ShipError, artifactPath string) {`

```text
// emitShipError projects a recorded ShipError verbatim: module ship, the
// SHIP_<code> projection, INCIDENT for the integrity class (§5.3), the
// ship-error.json path when it was written, and the Debug keys the triage
// whitelist names (shiperr.SignalDebugKeys — the landing step, the git exit
// code, the branches, the repair outcome; ADR-0103 unit 07) when non-empty.
```

### `go/internal/core/signal_cycle_test.go:3` — above `import (`

```text
// signal_cycle_test.go — ADR-0101 S2a producers: cycle.sealed and
// system.failure at completeCycle (both roots) and, for an abnormal exit,
// from the epilogue; ship.error at recordShipError; quota.paused at
// pauseForQuota (the seam both roots reach). Each is an Adapter from a typed
// value the pipeline already owns to one Event — no new state, no decision.
```

### `go/internal/core/signal_refusal_codes_test.go:3` — above `import (`

```text
// signal_refusal_codes_test.go — the C1 chokepoint's phase.outcome WARN carries
// the phase's error-severity diagnostic CODES as fields.diagnostic_codes, so
// the console line and the durable stream name the class of a FAIL beside its
// prose ("… diagnostic_codes=TRIAGE_PROTECTED_SURFACE"); an uncoded FAIL
// carries no such field (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```

### `go/internal/core/signal_ship_debug_test.go:10` — above `func TestEmitShipError_ProjectsSignalDebugKeys(t *testing.T) {`

```text
// ADR-0103 unit 07 — the ONE ship.error producer projects shiperr.SignalDebugKeys
// from the ShipError's Debug map into Event.Fields when non-empty (the triage
// whitelist: step, git_rc, worktree, branch, cycle_branch, repair_outcome) and
// never any other Debug key; a Debug without them leaves the fields as before.
```

### `go/internal/core/signal_test.go:3` — above `import (`

```text
// signal_test.go — ADR-0101 S1: the orchestrator is a registered listener of the
// Signal Center and the ADR-0044 C1 chokepoint (recordPhaseOutcome) is its
// first producer. Every terminal phase disposition, on both dispatch roots,
// becomes exactly one phase.outcome (or phase.aborted) event; a green cycle
// emits no WARN; a reasoned FAIL (cycle 1636's shape) is a WARN that names the
// phase, its code and its own reason — rendered by the stderr sink in the ONE
// line format, no longer hand-written at the chokepoint.
```

### `go/internal/core/size_budget.go:3` — above `import (`

```text
// size_budget.go — ADR-0076 slice A: cycle-size → budget scaling. The size
// signal is the EXISTING cycle_size_estimate vocabulary (scout/triage emit
// it; router.Digest parses it) — no new agent contract. Consumers: the build
// correction limit (cyclerun_review) and the build launch's artifact budget
// (PhaseRequest.BudgetScale → bridge engine). Absent/unknown size = 1.0 =
// byte-identical legacy behavior.
```

### `go/internal/core/size_budget_test.go:3` — above `import (`

```text
// size_budget_test.go — ADR-0076 slice A consumption pins: the cycle-size
// multiplier scales the correction limit (clamped at the policy ceiling) and
// the build launch's artifact budget via PhaseRequest.BudgetScale. Absent /
// unknown size = 1.0 = byte-identical legacy behavior.
```

### `go/internal/core/size_budget_wiring_test.go:13` — above `func sizeBudgetRun(t *testing.T, reportBody string, completed []string) *cycleRun {`

```text
// size_budget_wiring_test.go — ADR-0076 slice A consumption: the cycle's size
// signal (triage-report.md via router.Digest — the LIVE artifact path, not the
// extinct handoff JSON) drives the build phase's budget scale and correction
// limit. Absent/unknown size is pinned to the exact legacy behavior.
```

### `go/internal/core/solution_floor.go:10` — above `const CtxKeyDeliverableKindDefault = "deliverable_kind_default"`

```text
// solution_floor.go — ADR-0099 slice 2: the document deliverable's deterministic
// handoff floor. A `document` cycle delivers <root>/<slug>/ (candidate options +
// recommendation + assumptions-and-evidence); the ONE engine that judges its
// shape (internal/solutioncheck) is projected here as a BuildFloorCheckFn so the
// E2 correction ladder fixes a malformed deliverable in-phase, exactly as the Go
// floors do for code. The kind and the bound slugs come from the kernel's own
// reads — the triage-authoritative report header and the triage decision —
// never from the builder's report. SolutionViolations is the single
// classification + collection every projection (floor, audit gate) calls.
```

### `go/internal/core/solution_floor.go:89` — above `func (o *Orchestrator) seedDispatchContext(ctx context.Context, base map[string]string, next Phase, cs CycleState, proje…`

```text
// seedDispatchContext is the ONE place both dispatch surfaces (the live loop
// and resume) enrich a phase's context from the cycle's records: the Task
// Contract (ADR-0098) and the project's default deliverable kind (ADR-0099).
```

### `go/internal/core/solution_floor_test.go:42` — above `func TestSolutionFloorChecks(t *testing.T) {`

```text
// TestSolutionFloorChecks — ADR-0099 slice 2: the build handoff floor judges a
// document cycle's solutions/<slug>/ deterministically (the ONE engine,
// internal/solutioncheck) and stays silent for code cycles. The kind and the
// bound slugs come from the kernel's own reads (report headers + triage
// decision), never from the builder's report.
```

### `go/internal/core/spine_failopen_telemetry_test.go:3` — above `import (`

```text
// spine_failopen_telemetry_test.go — RED contract for cycle-1166 Task 3
// (spine-failopen-telemetry, inbox weight 0.85), core half.
//
// A width-3 batch on 2026-07-13 emitted 76 occurrences of
//
//	[orchestrator] WARN spine not satisfied for next=<phase> (a mandatory
//	predecessor's handoff artifact is missing); proceeding fail-open
//	(would-block at enforce)
//
// …to stderr and nowhere else (cyclerun_select.go:151). No counter, no dossier
// field, no threshold: an epidemic with no dashboard. This item is
// measurement-first — make it visible, THEN decide whether the fix is
// artifact-retry, driver-bounce recovery, or an enforce flip.
//
// The WARN today names the phase being entered but NOT which predecessor's
// artifact is missing, so "with phase AND artifact" (the AC's words) needs the
// spine gate to REPORT its unsatisfied anchor rather than just returning false.
// That is the core-side contract below; the dossier/rollup half lives in
// internal/dossier/spine_failopen_rollup_test.go.
//
// RED today: UnsatisfiedSpineAnchor, cyclestate.SpineFailOpen and
// CycleResult.SpineFailOpens do not exist — this file does not compile.
//
// Contract Builder must satisfy:
//
//	func (sm *StateMachine) UnsatisfiedSpineAnchor(target Phase, sig router.RoutingSignals,
//	    cfg config.RoutingConfig) (Phase, bool)   // first unsatisfied predecessor anchor
//	type cyclestate.SpineFailOpen struct{ Phase, MissingArtifact, Reason string }
//	CycleResult.SpineFailOpens []SpineFailOpen
//	func (cr *cycleRun) recordSpineFailOpen(next Phase, missingArtifact, reason string)
//
// …and the fail-open branch at cyclerun_select.go:151 must call
// recordSpineFailOpen alongside the existing stderr WARN.
```

### `go/internal/core/spine_order_test.go:3` — above `import (`

```text
// spine_order_test.go — PA-BIG S5 (ADR-0058): the linear transition spine is a
// DATA table (spineOrder), walked by spineNext, instead of a per-phase switch in
// Next. spineOrder is a config-INDEPENDENT trust anchor (like the legality
// graph): config SELECTS among legal edges (on_pass/on_fail), it cannot move the
// spine. The full Next() byte-identity is proven by the transition oracle
// (TestTransitionKernelOracle_Next); this pins the SSOT table directly.
```

### `go/internal/core/spine_replay_local_test.go:3` — above `import (`

```text
// spine_replay_local_test.go — env-gated OPERATOR replay harness (skipped in
// CI/normal runs). Replays a project's REAL .evolve/runs history through
// router.Digest + StateMachine.SpineSatisfiedUpTo to answer empirically:
// would any historical cycle trip the spine floor at enforce? This is the
// soak-evidence tool behind the R8.5 flip (2026-07-16: 536 dirs replayed,
// 0 would-block on every cycle shape since ~cycle-480 after the scout/audit
// digest fallbacks; the only misses were pre-convention dirs, cycles
// 361-479). Re-run it before widening the floor (new anchors, new gates):
//
//	EVOLVE_SPINE_REPLAY_DIR=<project-root> go test -v \
//	    -run TestSpineReplay_LocalHistory ./internal/core/
```

### `go/internal/core/stale_marker_autoseal.go:13` — above `func markerShouldAutoseal(ownerPID int, hasPID bool, alive func(int) bool) bool {`

```text
// stale_marker_autoseal.go — auto-seal a stranded cycle-state marker whose owner
// process is dead, at loop boot (cycle 507, task wire-boot-recovery-functions).
// A crashed cycle strands a role-gated marker (e.g. phase=retro) whose role-gate
// then BLOCKS operator/inbox writes, gating even the recovery actions behind a
// manual `evolve cycle reset --force`. Boot must auto-seal such a marker when its
// owner PID is dead — REUSING the same SealCycle(Force) path (no duplicated seal
// logic, per never_duplicate_centralize_via_design_patterns).
```

### `go/internal/core/stale_marker_autoseal_test.go:3` — above `import (`

```text
// stale_marker_autoseal_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for auto-sealing a stranded cycle-state marker
// whose owner process is dead, at loop boot. Function-level behavior contract
// for the recovery primitive the Builder (re)implements in
// stale_marker_autoseal.go.
//
// Root cause (scout-report.md Key Finding 5): a crashed cycle strands a
// role-gated cycle-state marker (e.g. phase=retro) whose role-gate then BLOCKS
// operator/inbox writes, gating even the recovery actions behind a manual
// `evolve cycle reset --force`. Boot must auto-seal such a marker when its owner
// PID is dead — reusing the SAME SealCycle(Force) path (no duplicated sealing
// logic, per never_duplicate_centralize_via_design_patterns).
//
// References markerShouldAutoseal / AutosealStaleMarker, which the Builder
// implements. RED now (undefined symbols → core test package fails to compile).
// Do NOT modify this file — implement the production seam.
```

### `go/internal/core/statemachine.go:29` — above `allowed map[Phase]map[Phase]bool`

```text
// allowed[from] is the set of legal `to` phases. As of PA-DDK DDK-5 this
// graph is config-DRIVEN (registry config.legal_successors, injected via
// WithLegalGraph); the literal default below is the byte-identical fallback
// when no config supplies it. The trust anchor is no longer the literal — it
// is ValidateSafetyInvariants, the phase-agnostic load-time validator the
// composition root HARD-fails on (ADR-0060 §1a). Config can declare any graph;
// the validator rejects any graph that could ship without the floor.
```

### `go/internal/core/statemachine.go:48` — above `var spineOrder = []Phase{`

```text
// spineOrder is the canonical linear successor sequence — the mandatory-default
// spine the state machine walks for any phase that is not a verdict branch
// (audit), a control sentinel (retro/debugger), or the intent-independent start
// edge. spineNext walks it so the LINEAR transition is a data lookup, not a
// per-phase switch (ADR-0058 S5). Like the legality graph (§1) it is a config-
// INDEPENDENT trust anchor: config SELECTS among already-legal edges
// (on_pass/on_fail), it can never move the spine itself. NB: this is NOT
// cfg.Order — cfg.Order interleaves optional insertions (spec-verify, tester, …)
// the static spine skips; reproducing the literal spine needs its own SSOT.
//
// audit appears so build→audit resolves, but its OWN successor is never taken via
// spineNext: Next intercepts audit in the explicit switch (the verdict branch)
// before the spine walk. end is the terminal waypoint for ship→end.
```

### `go/internal/core/statemachine.go:156` — above `func (sm *StateMachine) WithCatalog(specFor func(Phase) (phasespec.PhaseSpec, bool)) *StateMachine {`

```text
// WithCatalog gives the StateMachine config-driven transition resolution: a
// phase whose descriptor declares on_pass/on_fail resolves its verdict branch
// from config instead of a hardcoded phase-name case (ADR-0058). When unset
// (bare unit-test SMs) or when a phase declares no on_pass/on_fail, Next
// degrades to the exact literal table — so the kernel stays byte-identical for
// catalog-less orchestrators and a registry missing the fields.
```

### `go/internal/core/statemachine.go:187` — above `PhaseAudit: {PhaseShip: true, PhaseRetro: true, PhaseTDD: true, PhaseBuild: true},`

```text
// PhaseTDD/PhaseBuild are the audit-FAIL RE-ENTRY edges: a task-level
// rejection re-enters the dev cycle in the same cycle rather than tearing
// it down (see audit_fail_decision.go). They are legal ONLY through
// decideAfterAuditFail, which computes the deterministic policy envelope
// first — the ADR-0072 floor is evaluated before either edge can be taken.
```

### `go/internal/core/statemachine.go:275` — above `if sm.specFor != nil {`

```text
// Config-driven verdict branch (ADR-0058): a phase whose descriptor declares
// on_pass/on_fail resolves its successor from the verdict via config, not a
// hardcoded phase-name case. Targets are denormalized through phaseFromRouter
// (registry vocab → core.Phase). The legality graph still gates the chosen
// edge downstream, so config can only pick an already-legal successor. Absent
// a catalog (bare SM) or the fields, control falls through to the literal
// table below — byte-identical, as the transition oracle proves.
```

### `go/internal/core/statemachine.go:324` — above `if next, ok := sm.spineNext(current); ok {`

```text
// Linear spine (ADR-0058 S5): the successor is the next entry in the canonical
// spine table — a data walk, byte-identical to the former per-phase switch.
```

### `go/internal/core/statemachine.go:332` — above `func mandatoryAnchorsFor(cfg config.RoutingConfig) []Phase {`

```text
// mandatoryAnchorsFor is the spine-anchor ORDER, derived ENTIRELY from config
// (ADR-0058 S6): the mandatory phases in the configured order. No phase is a Go
// literal here — an operator sets the spine anchors purely by editing the
// registry's phase order / mandatory_phases. effectiveOrder falls back to
// cfg.Mandatory when no registry order is loaded, so the floor always has anchors.
```

### `go/internal/core/statemachine.go:384` — above `func (sm *StateMachine) UnsatisfiedSpineAnchor(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) (Phase…`

```text
// UnsatisfiedSpineAnchor is SpineSatisfiedUpTo's REPORTER: it returns the FIRST
// mandatory predecessor anchor of target whose handoff artifact is missing, and
// whether such an anchor exists. It is the exact complement of the gate — both
// walk the same anchor list through the same gateSatisfied check, so a reporter
// that names an anchor the gate would have accepted (or stays silent where the
// gate blocks) is impossible by construction.
//
// The fail-open WARN and its cycle-1166 telemetry need the CAUSE, not just the
// fact: "ship proceeded with build's report missing" groups by cause, while
// "ship proceeded" does not. The FIRST unsatisfied anchor is the right one to
// report — a later anchor is usually missing only BECAUSE the earlier one is.
```

### `go/internal/core/statemachine.go:415` — above `func (sm *StateMachine) gateSatisfied(anchor Phase, sig router.RoutingSignals) bool {`

```text
// gateSatisfied reports whether anchor's artifact floor holds against the digest
// (PA-DDK DDK-4). When the catalog declares the anchor's gate THRESHOLDS
// (requires_present / verdict_in), those config values decide; otherwise the
// literal anchorArtifactPresent map is the byte-identical fallback. The digest
// (which signal proves the handoff) stays trusted Go — only the thresholds are
// config (ADR-0060).
```

### `go/internal/core/statemachine.go:437` — above `func digestSignalFor(anchor Phase, sig router.RoutingSignals) (present bool, verdict string) {`

```text
// digestSignalFor reads an anchor's (present, verdict) from the trusted on-disk
// signal digest. This phase→signal mapping is the VERIFICATION layer, kept in Go
// by design (ADR-0060): config sets the gate thresholds, code reads the
// objective artifacts. An anchor with no digest slot is treated as present.
```

### `go/internal/core/successor_strategy_test.go:3` — above `import (`

```text
// successor_strategy_test.go — PA-BIG S2 (ADR-0058): the retro history-branch
// gate is config-driven. recordAndBranch/resume enter the retro
// failure-adapter branch when the phase's branching_strategy is "history",
// degrading to the literal phase-identity default (retro→history) when the
// catalog is unset or the field is absent — byte-identical to the pre-S2 flow.
```

### `go/internal/core/successor_strategy_test.go:141` — above `func resumeFromRetro(t *testing.T, opts ...Option) (CycleResult, error) {`

```text
// resumeFromRetro builds a fake-backed orchestrator (optionally with a catalog)
// and resumes a cycle starting at retro, exercising resume.go's history-branch
// gate — the lockstep twin of recordAndBranch. ADR-0058 requires the retro
// branch to be byte-identity-covered on the resume path too, not just the live
// loop.
```

### `go/internal/core/system_failure.go:14` — above `type floorFailReason struct {`

```text
// floorFailReason is the FORENSIC workspace artifact recording WHY a floor
// phase's verdict was recorded FAIL when the phase's own report said otherwise
// — the error-severity diagnostics of the runner-side downgrade (audit's
// CI-parity gates: integration tier, go vet, apicover, EGPS…). state.json
// truncates the defect string; this file is the untruncated on-disk "why" for
// retros/operators (one grep instead of a forensic dig).
//
// TRUST BOUNDARY (go-review HIGH): the workspace is agent-writable — the audit
// agent already writes audit-report.md there — so this file is NEVER read by
// the ADR-0072 coherence floor. The floor's authoritative signal is
// CycleState.AuditFailReasons, set in orchestrator memory at the same
// chokepoint; a file dropped by any workspace writer cannot talk the floor out
// of halting.
```

### `go/internal/core/system_failure.go:115` — above `func (o *Orchestrator) detectVerdictIncoherence(ctx context.Context, cs CycleState, finalVerdict string) (sig *SystemFai…`

```text
// detectVerdictIncoherence is the ADR-0072 Go floor for the verdict-incoherence
// category: the deterministic, non-negotiable check that catches a pipeline
// forging a verdict. It reads the cycle's own on-disk phase artifacts
// (audit-report evolve-verdict + acs-verdict.json) and, if the recorded verdict
// is FAIL/WARN while both artifacts are green, returns a system-failure signal.
//
// This floor fires regardless of orchestrator judgment or strict_audit: a
// broken pipeline cannot be talked out of halting. It is gated on the
// failure_policy IsFloor(verdict-incoherence) predicate, so an operator can
// never narrow it below the compiled floor.
//
// Scope (deliberate for the deterministic slice): it fires ONLY on a recorded
// FAIL/WARN with green artifacts — the exact cycle 862→899 forgery signature.
// A RED artifact is coherent (a genuine task-code failure → never-stop). The
// "silent no-ship" (CycleOutcomeSkippedUnknown) case is intentionally NOT
// hard-halted here — a benign no-op cycle can also produce it, so its
// disambiguation is left to the orchestrator's judgment layer. The other floor
// category, infra-systemic (all CLI families exhausted), is enforced by the
// pre-existing resumable quota-pause path (cmd_loop) — NOT this function; the
// two floor categories have distinct, deliberate detection sites.
//
// A DIAGNOSED downgrade is NOT forgery: cs.AuditFailReasons carries the
// error-severity gate diagnostics behind the FAIL, set in ORCHESTRATOR MEMORY
// at the verdict-record chokepoint (recordFloorVerdictFailure) and cleared on
// every audit re-dispatch — the runner's own CI-parity gate overrode a
// narrative PASS, a coherent task-level outcome that routes retro→continue.
// Halting on it was the cycles-930/931/932 batch-killer: the whole loop stopped
// for a flaky integration tier while the diagnostics naming the cause sat in
// the response. The signal is deliberately NOT the workspace reason file
// (agent-writable — see floorFailReason's trust boundary): only the
// orchestrator's own in-process record can mark a FAIL as explained.
//
// A clean-exit-late-write RACE is NOT forgery either: the bridge can declare a
// phase's clean exit before Claude Code finishes its post-turn async writes, so
// the runner records FAIL while a VALID audit-report is still landing (the
// ~3s settle window < the observed 60-90s dribble, cycles 930/931/932/cycle-3).
// When the on-disk audit-report passes the FULL deliverable.Verify chain
// (challenge-token + required sections + ADR-0039 failure-context — NOT the
// cheap ParseVerdictSentinel read ReadCycleVerdicts uses), the contradiction is
// a benign timing race → reconcile the recorded verdict to PASS, no halt
// (returned as the second result). A PASS-sentinel-tagged but MALFORMED report
// yields Verify OK==false → still a forged verdict → halt — the anti-laundering
// boundary the inbox explicitly requires be preserved.
//
// The FULL Verify runs through the SAME injected ContractVerifier the correction
// ladder's salvage rung uses (WithContractVerifier → deliverable.NewVerifierWith
// CatalogStage; the breaker-neutral re-check, so a coherence probe never trips
// the contract-gate breaker). Injection is required because core cannot import
// deliverable (deliverable imports core). A nil verifier (unconfigured
// composition) leaves DeliverableValid=false → the pre-fix conservative halt,
// never a launder — the self-heal is purely additive, gated on a verifier being
// present. Verify's fail-OPEN err (infra ambiguity) also leaves it false.
```

### `go/internal/core/system_failure.go:198` — above `func hasSubstantiveFailReasons(cs CycleState) bool {`

```text
// hasSubstantiveFailReasons is the ONE spelling of "the recorded negative
// verdict is DIAGNOSED": orchestrator memory (never agent-writable artifacts)
// holds a persisted floor-fail reason for audit or ship. It feeds
// coherence.VerdictInputs.SubstantiveError at both coherence surfaces (the
// live floor here and the dossier builder) and contradicts a prose
// verdict-incoherence claim in applyFailureDecisionFloor — three projections
// of one belief (cycle-1603: two of them agreeing and the third missing is
// exactly how a diagnosed downgrade halted a batch as "forgery").
```

### `go/internal/core/system_failure_test.go:13` — above `func writeVerdicts(t *testing.T, dir, audit, acs string) {`

```text
// ADR-0072 S3: the Go floor. A recorded-negative cycle whose on-disk artifacts
// are green is verdict-incoherence (the pipeline forged the verdict) → HALT.
// A recorded-negative with a RED artifact is a genuine task failure → nil.
```

### `go/internal/core/system_failure_test.go:167` — above `func TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt(t *testing.T) {`

```text
// TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt — the cycle-930/931/932
// false-HALT regression. The audit agent writes a PASS report + green ACS, but a
// runner-side CI-parity gate (the integration tier) legitimately downgrades the
// verdict to FAIL; the record chokepoint stamps the reasons into ORCHESTRATOR
// MEMORY (cs.AuditFailReasons). That FAIL is DIAGNOSED — a coherent task-level
// outcome (retro + continue), NOT a forged verdict — so the floor must NOT halt.
// Before this fix, detectVerdictIncoherence never populated SubstantiveError, so
// every diagnosed gate-downgrade with green artifacts halted the whole batch.
```

### `go/internal/core/system_failure_test.go:233` — above `func TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt(t *testing.T) {`

```text
// TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt — cycle-1329
// (pipeline-defect-pipeline-blocker, scout Task 1). The audit + ACS both
// recorded PASS (161/161 green), but the SHIP phase legitimately rejected the
// cycle post-audit (REPO_CONTRACT_GATE: "repo-contract scanner pack RED ...
// pushing would red main"). That is a real, explained ship-phase failure — a
// coherent task-level outcome (retro + continue) — NOT a forged verdict.
// Before this fix, SubstantiveError was computed from cs.AuditFailReasons
// alone, so this exact shape (green audit + green ACS + recorded FAIL) was
// indistinguishable from cycles 862→899's genuine forgery and halted the
// batch (3x identical-fingerprint recurrence, ship|unknown|76d0f4fca190).
```

### `go/internal/core/system_failure_test.go:246` — above `writeVerdicts(t, dir, "PASS", "PASS")`

```text
// audit-report PASS + acs-verdict.json PASS, exactly cycle-1329's shape
```

### `go/internal/core/system_failure_test.go:259` — above `func TestDetectVerdictIncoherence_AuditPhaseBehaviorUnchangedByShipField(t *testing.T) {`

```text
// TestDetectVerdictIncoherence_AuditPhaseBehaviorUnchangedByShipField —
// regression guard: adding the ship-phase carrier must not disturb the
// existing audit-phase-only signal. AuditFailReasons alone (ShipFailReasons
// empty/nil) still suppresses the halt exactly as the cycles-930/931/932 fix
// already covers (TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt);
// this test additionally pins that an EMPTY ShipFailReasons never itself
// contributes a false explanation when AuditFailReasons is also empty.
```

### `go/internal/core/task_contract.go:3` — above `import (`

```text
// task_contract.go — the harness-owned Task Contract block (ADR-0098, research
// proposal R4). The acceptance criteria a cycle is graded against live on the
// inbox item; before this file they reached the builder only by way of the
// scout's and triage's prose (two LLM hops from the source), and the ACS
// predicates the tdd phase wrote reached the builder only if it grepped for
// them. Both are now projected DETERMINISTICALLY into the tdd, build and audit
// prompts under one heading: the acceptance projected from the item file the
// lane is bound to (the same file triage and the auditor read), and — for the
// build, which runs after tdd — the predicate names `go test -list` reports
// for go/acs/cycle<N>. Nothing here is authored by an agent; a missing or
// unreadable input is a loud line in the block, never a silent omission.
```

### `go/internal/core/task_contract.go:78` — above `if hasSpec {`

```text
// No Go predicate suite for a document deliverable: the deterministic
// floor is the solution contract (ADR-0099 slice 2), self-checkable
// with `evolve solution check`. Without a registry contract there is
// no floor to name — composeTaskContract already said so per item.
```

### `go/internal/core/task_contract.go:155` — above `func ContractTaskIDs(workspace string) []string {`

```text
// ContractTaskIDs is the ONE id set the Task Contract binds a cycle to — the
// lane pin when present (LaneScopeIDs), else the triage decision's top_n
// (BoundTaskIDs), minus the decision's deferrals — the projection every
// consumer that reasons about "the committed members" reads: the Task
// Contract itself (taskItemRefs, parity-tested) and the TDD->Build scope gate
// (cycle-1620 salvage). Never triage-report.md's markdown ## top_n: that is
// prose in triage's working-id namespace, where decomposition sub-ids are the
// documented norm.
```

### `go/internal/core/task_contract_test.go:338` — above `func TestContractTaskIDs(t *testing.T) {`

```text
// TestContractTaskIDs — cycle-1620 salvage (architecture CRITICAL 1): the
// TDD->Build scope gate must bind to the SAME id set the Task Contract handed
// TDD — the lane pin when present, else the triage decision's top_n, minus the
// decision's deferrals — never to triage-report.md's markdown ## top_n, which
// is prose in triage's working-id namespace (decomposition sub-ids are the
// documented norm). ContractTaskIDs is that one projection; taskItemRefs
// derives its ids from the same readers, proven by parity below.
```

### `go/internal/core/throughput_hook_test.go:48` — above `func TestIsShippingVerdict_WholeOutcomeVocabulary(t *testing.T) {`

```text
// TestIsShippingVerdict_WholeOutcomeVocabulary walks every label the
// ADR-0079 outcome vocabulary can put in CycleResult.FinalVerdict. The
// allowlist shape is load-bearing: SKIPPED_UNKNOWN once fell through a
// denylist-shaped breaker, so an unrecognised label must classify as
// NON-shipping rather than defaulting to "shipped".
```

### `go/internal/core/transition_oracle_test.go:5` — above `type oracleCell struct {`

```text
// transition_oracle_test.go — the byte-identity ORACLE for the transition kernel
// (ADR-0058). It FREEZES the current behavior of NewStateMachine().Next and
// CanTransition, captured from HEAD (main abf787ff, 2026-06-21), so every
// subsequent slice that makes the kernel config-driven can prove it changed
// NOTHING observable. This is the trust anchor for the phase-agnostic-flow
// refactor: it MUST stay byte-identical green across S0..S7. A diff here is a
// behavior change in the integrity-floor transition logic and must be
// deliberate, reviewed, and re-frozen — never silently.
```

### `go/internal/core/transition_oracle_test.go:68` — above `PhaseAudit:    {PhaseShip, PhaseRetro, PhaseTDD, PhaseBuild},`

```text
// DELIBERATE WIDENING (retry + retro redesign, 2026-08-28). PhaseTDD and
// PhaseBuild are the audit-FAIL re-entry edges: a TASK-level rejection — per
// the ADR-0072 failure_policy category table, which has always declared
// code-audit-fail as {task, retry-with-fix, MaxRetries: 2} — re-enters the dev
// cycle in the SAME cycle instead of tearing it down. Reachable only through
// decideAfterAuditFail, which evaluates the deterministic floor FIRST, so a
// floor category halts before either edge is offered. This anchor is
// config-independent on purpose: widening it must be an explicit edit like
// this one, never a side effect of a config change.
```

### `go/internal/core/treediff_crosslane_mint_test.go:16` — above `func writeMintSpec(t *testing.T, root, name, body string) {`

```text
// --- cycle-967: fleet cross-lane mint false-abort ---
// (fix spec: treediff-967-crosslane-mint-fix-spec, Variant A2)
//
// Both fleet lanes run `evolve cycle run` against the SHARED project root.
// The advisor mint (phaseregistrar.Registrar) persists a minted phase's
// config to the shared .evolve/phases/<name>/phase.json, while the per-phase
// tree-diff guard diffs that same shared tree against a PER-LANE baseline —
// so lane A's mint landing during lane B's phase is charged to lane B.
// Cycle-967's PASS scout was aborted on concurrent lane-970's
// .evolve/phases/gate-wiring-proof/phase.json mint.
//
// Fix: the registrar records minted names in the shared mintregistry BEFORE
// persisting files; the guard exempts a leaked .evolve/phases/<name> path IFF
// <name> is a registered, fresh mint. An UNREGISTERED phase-config write must
// still abort — the deliverable-leak loophole TestIsScoutEvalMaterialization
// pins against stays closed (TestGuardStillAbortsUnregisteredPhaseConfigLeak
// holds both before and after the fix).
```

### `go/internal/core/treediff_crosslane_mint_test.go:67` — above `func TestGuardExemptsConcurrentLaneMintedPhaseConfig(t *testing.T) {`

```text
// TestGuardExemptsConcurrentLaneMintedPhaseConfig is the cycle-967 RED proof:
// a concurrent lane's REGISTERED mint appearing in this lane's post-scout
// diff must not abort the cycle. The on-disk spec is present in the registrar's
// normalized form — the guard verifies content (clamp parity), not just the
// registry name.
```

### `go/internal/core/triage_termination_claimable_test.go:9` — above `func TestHasClaimableInboxWork_ConsoleOwnedKindsAreNotClaimable(t *testing.T) {`

```text
// hasClaimableInboxWork decides whether an empty triage commitment is the
// planner's fault (claimable work existed) or the honest state of the queue.
// Research F25 (wave 6, cycle 1688): pipeline-* items are console-owned by
// the ADR-0074 classifier, so a queue holding only those is NOT claimable
// work — a lane cannot claim them, and blaming triage for leaving them would
// route a cycle to the claim-failed termination for work no lane may do.
```

### `go/internal/core/truncate_goal_projection_test.go:11` — above `func TestTruncateGoal_ProjectsTheAdvisorCap(t *testing.T) {`

```text
// ADR-0103 unit 04: truncateGoal and maxGoalTextChars are CONSUMERS of the
// advisor's cap — the judge's goal section and the task-recall digest render
// the advisor's bound with textcap's rule, so neither core facade can silently
// re-implement either.
```

### `go/internal/core/unavailable_phases.go:12` — above `func (o *Orchestrator) unavailablePhaseReason(name string) error {`

```text
// unavailable_phases.go — 2026-09-09 token-waste root cause #2: the advisor
// kept selecting optional phases whose persona doc does not exist; every
// selection cost a dispatch, a recorded skip and (before deterministic
// learning) a retrospective agent. The absence is deterministically known
// before the plan is made, so the plan never offers it: core probes every
// catalog-Optional, non-floor runner (PersonaProber) and hands the absent
// ones to the router as environmental context (RouteInput.UnavailablePhases).
```

### `go/internal/core/unavailable_phases_test.go:25` — above `func TestAdvisorPlanInput_ListsPhasesWithMissingPersona(t *testing.T) {`

```text
// TestAdvisorPlanInput_ListsPhasesWithMissingPersona — 2026-09-09 token-waste
// root cause #2: the advisor kept selecting optional phases whose persona doc
// does not exist; each selection cost a dispatch, a skip and (before the
// deterministic-learning fix) a retrospective agent. The orchestrator asks
// every OPTIONAL runner for its persona at plan time and hands the absent
// ones to the router as environmental context, so the advisor never sees them
// as selectable and the floor clamp drops them if proposed anyway. Mandatory
// and floor phases are not probed: their absence must stay a loud dispatch
// failure, never a silent exclusion.
```

### `go/internal/core/unexplained_failures_test.go:3` — above `import (`

```text
// Cycle-1044/1045/1047 (batch-6 live-fire): three DIFFERENT failures each
// wrote no failure-reason artifact, collapsed to the identical empty-evidence
// fingerprint "|unknown|e30d…", and tripped the identical-fingerprint breaker
// rule with a wrong diagnosis. Two contracts pinned here:
//  (1) every retro path supplies fallback evidence, so distinct failures get
//      DISTINCT fingerprints (the F8 principle: a failure mode must emit its
//      reason into an artifact);
//  (2) the breaker names the degenerate empty-evidence case honestly as an
//      unexplained-failures diagnosability halt, never "identical defects".
```

### `go/internal/core/verdict_cache_hook_apicover_test.go:9` — above `func TestWithVerdictCacheLookupHook_AppliesObserver(t *testing.T) {`

```text
// TestWithVerdictCacheLookupHook_AppliesObserver names AND executes
// WithVerdictCacheLookupHook in the DEFAULT (untagged) build. The option's only
// other users are `integration`-tagged tests, so the repo-wide apicover gate
// (ADR-0069) scored it FALSE-GREEN — named by a test but 0% executed. This test
// applies the returned Option to an Orchestrator and drives the installed hook,
// pinning that the option actually reaches o.verdictCacheLookupHook, the field
// the pre-loop shadow probe calls (orchestrator.go).
```

### `go/internal/core/verdict_cache_probe_wiring_test.go:15` — above `func TestVerdictCacheProbeEligibilityWiring(t *testing.T) {`

```text
// TestVerdictCacheProbeEligibilityWiring is the cycle-1488 reachability proof for
// the shared base-tree eligibility predicate.
//
// The fresh-base collision guard was born as an inline comparison duplicated
// at two call sites (orchestrator.go's pre-loop shadow probe and
// phase_bindings.go's audit-binding Put); both now route through the shared
// predicate. This test pins the contract that the ORCHESTRATOR's decision is
// derived from verdictcache.ProbeEligible — the single source a future
// enforce-stage lookup must reuse — by running the real
// RunCycle path and asserting its observed skip/match decision agrees with the
// predicate's verdict for the same (base tree, candidate tree) pair.
//
// A duplicated inline comparison that drifts from the predicate fails here; so
// does a predicate that is never reached from production (the oracle disagrees).
```

### `go/internal/core/verdict_cache_put_base_test.go:5` — above `import (`

```text
// verdict_cache_put_base_test.go — RED contract for the salvage-review HIGH-1
// (salvage/verdict-cache-land): the Put-site fresh-base guard must compare the
// audited worktree against the WORKTREE'S OWN base commit (CycleState.
// WorktreeBaseSHA), never against projectRoot HEAD at audit time. Under fleet
// concurrency a sibling ship advances main mid-cycle; resolving the base from
// the advanced HEAD (a commit the lane's worktree may not even contain) either
// diverges the operands or fails open — and an UNCHANGED worktree's shared
// fresh-base identity gets WRITTEN into the verdict cache, the exact entry
// class ADR-0048's guard exists to prevent on the read side.
```

### `go/internal/core/verdict_conflict_flow_test.go:8` — above `func TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons(t *testing.T) {`

```text
// verdict_conflict_flow_test.go — the CONSUMER half of the cycle-1124
// verdict-conflict wiring proof (producer half: internal/phases/audit/
// audit_verdict_conflict_test.go).
//
// The cycle-1124 fix deliberately adds NO new plumbing: it emits the conflict
// record as an ERROR-severity diagnostic so the existing chain carries it —
// cyclestate.ErrorMessages → CycleState.AuditFailReasons (the ADR-0072 coherence
// floor's only authoritative source) → <phase>-fail-reason.json (forensics) →
// failure dossier SubstantiveError/FailReasons (failure_dossier.go:86).
//
// That "no new plumbing" claim is load-bearing, so it is pinned here rather
// than assumed. This half is expected to be pre-existing GREEN (it locks the
// contract the producer relies on); the audit-package half is the RED one.
```

### `go/internal/core/verdict_conflict_flow_test.go:77` — above `func TestVerdictConflict_FingerprintIsStableAcrossTheNarrativeAlphabet(t *testing.T) {`

```text
// TestVerdictConflict_FingerprintIsStableAcrossTheNarrativeAlphabet — cycle-1127
// audit finding C1, as the property the breaker actually needs. IsVerdict bounds
// the narrative to four values; three of them (PASS/WARN/SKIPPED) reach the
// conflict record. If each yields its own fingerprint, one recurring poisoned
// gate lands in three buckets of one against IdenticalFingerprintCeiling=3 and
// the batch halt never fires — the 862-899 storm shape the breaker exists to
// stop. Bounded is not stable; this pins stable.
```

### `go/internal/core/verdict_distinguisher_test.go:3` — above `import (`

```text
// verdict_distinguisher_test.go — cycle-1054/1060 pin: two DIFFERENT tasks'
// agent-graded audit FAILs shared one fingerprint because the verdict-path
// fallback reason was a constant string — three would falsely trip the
// identical-fingerprint breaker rule. The fallback must fold in per-failure
// content that is STABLE across recurrences of the same defect (task ids,
// report defect head) but differs across different defects. Cycle numbers are
// deliberately excluded — they would make every fingerprint unique and blind
// the breaker to real repeats.
```

### `go/internal/core/workspace_guard.go:24` — above `pollution := 0`

```text
// lane-scope.json is provisioned by the fleet supervisor BEFORE the cycle
// runs (cycle-640 lane pin) — pre-phase by design, not pollution.
// minimal: a genuinely polluted dir is archived whole, pin included; the
// env-snapshot fallback re-materializes the pin for fleet lanes.
```

### `go/internal/core/worktree.go:15` — above `type WorktreeProvisioner interface {`

```text
// worktree.go — per-cycle git worktree provisioning for the Go orchestrator.
//
// Source-writing phases (tdd, build) run in an isolated per-cycle worktree so a
// failed or buggy cycle never mutates the live working tree. The bash
// run-cycle.sh provisioned these; the v11 Go port dropped it, which left
// cs.ActiveWorktree empty and the role-gate's only source-write allowance
// (phase==build && ActiveWorktree!="") permanently unsatisfiable — i.e. no
// phase could write code. This restores provisioning behind an injected seam
// so RunCycle stays unit-testable without real git. See ADR-0027.
```

### `go/internal/core/worktree.go:41` — above `baseOverride string`

```text
// baseOverride is the operator override for the per-cycle worktree base,
// resolved once from policy.json (worktree.base) and injected via
// WithWorktreeBase. Empty ⇒ the built-in <root>/.evolve/worktrees default.
// Replaces the former EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
```

### `go/internal/core/worktree.go:78` — above `if _, cerr := ensureCleanWorktree(context.Background(), wt, projectRoot, cycle); cerr != nil {`

```text
// Clean-HEAD assertion (cycle-653 / cycle-584 gate): a reused
// worktree may carry a prior failed attempt's uncommitted dirt,
// which ship would bind into this cycle's tree. Quarantine the
// dirt (preserved for salvage) and reset to HEAD; fail loudly
// rather than hand a dirty worktree to the cycle.
```

### `go/internal/core/worktree.go:105` — above `_, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.AddWorktreeWithRetry(`

```text
// Transient-contention retry (cycles 1221/1231/1232/1234/1240): N lanes of
// one repo provision concurrently, and `git worktree add` takes repo-level
// locks in the SHARED .git (the plane itself is a linked worktree), so a
// collision returns rc=255 with nothing on stderr beyond "Preparing
// worktree". One transient collision used to kill the lane's whole cycle:
// ActiveWorktree stayed empty, CB.2 fail-fasted every dispatch exit=10,
// three identical fingerprints halted the batch — twice in one day, once
// with zero console git activity (lane-vs-lane, not operator-vs-lane).
// Bounded backoff'd retry treats contention as what it is; a PERSISTENT
// failure still fails loudly with the same error after the last attempt —
// the downstream alarm chain is correct and must stay armed (the refuted
// PR #400 is the record of what happens when the alarm is silenced
// instead). verifylock is the precedent for cross-lane git serialization.
```

### `go/internal/core/worktree.go:187` — above `func integrationHead(ctx context.Context, git gitexec.Git, remote string) (string, error) {`

```text
// integrationHead resolves the ref a fresh lane must base on: the INTEGRATION
// HEAD its landing targets (2026-09-09 token-waste root cause #3 — a lane
// based on origin/main while the local main sat AHEAD by unpushed dossier
// closeouts forced a rebase and a second Build/Audit for twelve dossier
// files). Off the default branch, or with the local main current or BEHIND,
// the remote tip is the authority (the wave boundary fast-forwards a behind
// main). A local main strictly AHEAD is the authority: its unpublished
// landings ride the next push. Diverged histories are refused loudly — no
// base choice avoids a rebase then, so the plane must be reconciled before
// any lane spends a phase.
```

### `go/internal/core/worktree.go:234` — above `func (g gitWorktree) CreateFrom(projectRoot string, cycle int, startRef string) (string, error) {`

```text
// CreateFrom provisions the cycle worktree seeded from startRef instead of
// HEAD (ADR-0076 slice C adoption: the ref is a prior attempt's salvage
// snapshot, so the new cycle resumes committed work through the STANDARD
// provisioning path — no dirty-state adoption, no clean-HEAD bypass). The
// reuse/validation semantics of Create do not apply: a continuation always
// targets a NEW cycle number, so an existing directory is a stale collision
// and is recreated.
```

### `go/internal/core/worktree.go:383` — above `func LeakRecoverablePhase(p Phase) bool {`

```text
// LeakRecoverablePhase reports whether a phase runs with an active cycle
// worktree and therefore must be ELIGIBLE FOR LEAK RECOVERY — a DISTINCT axis
// from WorktreePhase (role-gate write-permission). The worktree is provisioned
// once at cycle start for every phase, so triage, audit, scout, and
// bug-reproduction all get one even though they are not source-writers; an
// unexpected write from any of them into the main tree must be RELOCATED into
// the worktree, not hard-abort the cycle via the tree-diff guard. Gating
// recovery on WorktreePhase (tdd/build only) is exactly the cycle-564 gap
// behind 9 recorded tree-diff-leak failures (390/399/491/496/501/538/540/556)
// spanning precisely these non-source-writing phases. This is a SEPARATE
// predicate — widening WorktreePhase in place would be a write-permission
// escalation masquerading as a recovery fix.
```

### `go/internal/core/worktree_base.go:3` — above `import (`

```text
// worktree_base.go — a reused salvage snapshot must never become the cycle's
// normalization base.
//
// gitWorktree.Create reuses a valid same-cycle worktree and returns it untouched
// (worktree.go:75-90), after which RunCycle records that worktree's current HEAD
// verbatim as cs.WorktreeBaseSHA (cyclerun.go:496-497). When the reused HEAD is
// an ADR-0076 salvage snapshot — the commit stampContinuationManifest makes to
// preserve a failed attempt's work (continuation_stamp.go:44-45) — the base and
// the preserved work are the SAME commit. normalizeWorktreeToBase then soft-
// resets to the snapshot, the salvaged diff reads as empty, and the very work
// salvage exists to protect normalizes away to nothing.
//
// The fix is deliberately narrow: only a snapshot HEAD is walked back, and only
// to its FIRST non-snapshot ancestor. An unconditional walk-back would re-base
// every ordinary lane, which is a strictly worse defect than the one being
// fixed, so ordinary reuse must capture HEAD verbatim and is pinned by test.
```

### `go/internal/core/worktree_base.go:43` — above `func resolveWorktreeBaseSHA(ctx context.Context, worktree string) (string, error) {`

```text
// resolveWorktreeBaseSHA returns the SHA to record as the cycle's
// WorktreeBaseSHA for worktree: its HEAD, unless HEAD is a salvage snapshot, in
// which case the first ancestor that is not one.
//
// Error discipline, case by case:
//   - HEAD itself unreadable → error (nothing sane to record).
//   - HEAD readable but its ANCESTRY is not → the captured HEAD is returned
//     VERBATIM with a WARN. The walk exists only to detect salvage snapshots,
//     and a snapshot can only be identified by reading subjects; when they
//     cannot be read, recording HEAD verbatim is exactly what the pre-guard
//     code did, whereas an error here would leave the base EMPTY at the call
//     site — which the pre-guard code itself documented as the worse outcome
//     (an empty base disables the cycle-156 normalize). Pinned by
//     TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim and, one
//     level up, by TestVerdictCacheCollisionRegression, whose missing-base
//     scenario is CONSTRUCTED by stubbing `rev-parse HEAD` — capture must
//     therefore go through `rev-parse HEAD`, never be inferred from the walk.
//   - HEAD is a snapshot with no non-snapshot ancestor within the bound →
//     errUnresolvableSnapshotBase (loud abort): both fallbacks reproduce the
//     defect this guard exists to fix.
```

### `go/internal/core/worktree_base_test.go:3` — above `import (`

```text
// worktree_base_test.go — cycle-1544, the reuse → base-capture seam.
//
// A reused worktree's HEAD can be an ADR-0076 salvage snapshot. Recording that
// SHA as WorktreeBaseSHA makes base and preserved work the SAME commit, so
// normalizeWorktreeToBase soft-resets onto the snapshot and the salvaged diff
// reads as empty — the work salvage exists to protect normalizes away to
// nothing. The guard walks back to the first non-snapshot ancestor, and ONLY
// for a snapshot HEAD: an unconditional walk-back would re-base every ordinary
// lane, which is the wider defect the second test bounds.
//
// These run against a REAL git repo, because the whole seam is a git ancestry
// question and a faked runner would prove only that the fake agrees with itself.
```

### `go/internal/core/worktree_base_test.go:261` — above `func TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim(t *testing.T) {`

```text
// TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim — the fail-OPEN
// branch: HEAD resolves but its ancestry cannot be read (here: HEAD is a
// grafted/garbage ref the log walk rejects). The guard can only justify walking
// when it can positively identify a salvage snapshot; unreadable subjects mean
// "record verbatim, WARN", never "record empty" — an empty base disables
// normalization, the outcome the pre-guard code itself called worse. This is
// also the seam TestVerdictCacheCollisionRegression's missing-base scenario
// constructs by stubbing rev-parse HEAD: capture must go through rev-parse and
// survive a failed walk, or that regression's scenario silently stops existing
// (which is exactly how PR #486's first CI run went red).
```

### `go/internal/core/worktree_baseconfig_test.go:8` — above `func TestGitWorktree_BaseFromConfigNotEnv(t *testing.T) {`

```text
// TestGitWorktree_BaseFromConfigNotEnv locks the flag-reduction change (ADR-0064):
// the per-cycle worktree base comes from the injected override (policy.json
// worktree.base, threaded via core.WithWorktreeBase) — NOT the EVOLVE_WORKTREE_BASE
// env var, which is removed. A set env var must be ignored; the struct field is
// the single source.
```

### `go/internal/core/worktree_clean.go:11` — above `func quarantineDir(projectRoot string, cycle int) string {`

```text
// worktree_clean.go — clean-HEAD assertion for REUSED per-cycle worktrees.
//
// Cycle-653 incident (fix of record = the cycle-584 lesson's prescribed gate,
// never landed until now): a reused worktree carried a prior failed attempt's
// uncommitted orphan RED test; ship binds the whole `git diff HEAD` tree, so
// the inherited dirt fails (or ships) the cycle regardless of any phase's
// notion of scope — cycle 653 would have PASSed in isolation. Family:
// cycle-24, -93, -365, -584, -645, -653.
//
// Policy (single mechanism, applied only on the gitWorktree.Create REUSE
// branch — a fresh `worktree add ... HEAD` is clean by construction, and the
// resume path (RunCycleFromPhase) never calls Create, so preserved mid-cycle
// work is untouched): dirty paths are MOVED to a per-cycle quarantine dir for
// salvage (never deleted), then the worktree is hard-reset to HEAD. Fail-loud:
// any quarantine/reset failure aborts provisioning rather than handing a
// dirty worktree to the cycle.
// minimal: the inbox item's alternative "cut a fresh worktree instead" mode is
// deliberately not implemented — quarantine+reset satisfies every acceptance
// criterion, and a config knob selecting between two equivalent outcomes would
// be flag sprawl (no-feature-flags rule). Upgrade path: a policy.json
// worktree.dirty block if a second behavior is ever genuinely needed.
```

### `go/internal/core/worktree_clean_test.go:12` — above `func initTestRepo(t *testing.T) string {`

```text
// Regression tests for the cycle-653 dirty-worktree-reuse incident: a reused
// per-cycle worktree inherited a prior failed attempt's uncommitted orphan RED
// test, ship bound the whole tree, and a would-PASS cycle failed. The
// cycle-584 lesson's prescribed gate (clean-HEAD provisioning with quarantine,
// never silent deletion) is ensureCleanWorktree, called on the
// gitWorktree.Create reuse branch.
```

### `go/internal/core/worktree_clean_test.go:45` — above `orphan := filepath.Join(wt, "go", "internal", "echo", "veto_red_test.go")`

```text
// Cycle-653 reproduction: the reuse candidate carries a prior attempt's
// uncommitted orphan RED test (untracked) AND a tracked-file modification.
```

### `go/internal/core/worktree_collision_integration_test.go:15` — above `func TestGitWorktree_ConcurrentSiblingsNoBranchCollision(t *testing.T) {`

```text
// TestGitWorktree_ConcurrentSiblingsNoBranchCollision reproduces the multi-stream
// failure: several `evolve loop` runs, each in its OWN worktree of the SAME repo,
// every one provisioning cycle 1. git worktree branch names — and, under a shared
// EVOLVE_WORKTREE_BASE, the directory path — are GLOBAL to one object store, so a
// bare `cycle-1` branch/dir from the first run made the second run's
// `git worktree add -B cycle-1` fail ("'cycle-1' is already used by worktree …");
// that run then fell back to the main tree and FAILED on the tree-diff guard.
// After the runscope fix each cycle name embeds the per-root lane, so sibling
// roots get distinct branches AND distinct dirs and both provision cleanly.
```

### `go/internal/core/worktree_fixture_integration_test.go:10` — above `func detachedWorktree(t *testing.T, repo string) string {`

```text
// detachedWorktree provisions what production provisions: a detached git
// worktree of repo at HEAD, in its own temp dir. Since 2026-09-14 a cycle
// never mutates its project root (inPlaceWorktree), so a fixture that hands
// the repository to the orchestrator as its own worktree exercises the
// refusal, not the mutating path it was written to pin.
```

### `go/internal/core/worktree_lanebase_test.go:11` — above `type laneBaseFake struct {`

```text
// worktree_lanebase_test.go — RED tests for cycle-1196 task
// `lane-base-fetch-origin-main` (todo id
// loop-must-base-lanes-on-origin-main-not-stale-local).
//
// gitWorktree.Create bases every new lane branch on the LOCAL HEAD of
// projectRoot (`git worktree add -B <branch> <wt> HEAD`, worktree.go:97) with
// zero fetch/origin interaction anywhere in the file. In a multi-lane fleet the
// local checkout drifts behind origin/main as sibling lanes land work, so each
// new lane silently forks from a stale tip — re-introducing already-fixed
// defects and inflating ship-time merge conflicts.
//
// Contract these tests pin:
//  1. origin exists  → fetch the remote tip in projectRoot BEFORE `worktree
//     add`, and cut the branch from the FETCHED ref (origin/main or
//     FETCH_HEAD), never the literal local "HEAD".
//  2. no origin      → no fetch, explicit local-HEAD fallback still succeeds
//     (isolated/local-only repos and test fixtures must not break).
//  3. fetch fails    → fail loudly: return a wrapped error and do NOT fall back
//     to the stale local tip (no `worktree add` at all).
//  4. reuse path     → an existing valid worktree is still reused with no fetch
//     and no add (regression guard on worktree.go:74-90).
//
// Uses the package gitRunner seam (git_seam_test.go / worktree_branchdelete_test.go
// style) — package core cannot import test/fixtures.FakeExec (import cycle).
```

### `go/internal/core/worktree_normalize_test.go:14` — above `func gitInRepo(t *testing.T, dir string, args ...string) string {`

```text
// worktree_normalize_test.go — Option C for the cycle-156 incident
// (docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md).
//
// A builder is instructed to `git commit ... [worktree-build]` (evolve-builder.md:235),
// but the auditor + binding inspect `git diff HEAD`, which is EMPTY after a commit.
// normalizeWorktreeToBase soft-resets the builder's commits back to the cycle base so
// the work becomes PENDING changes again — the state both the auditor and the binding
// assume. These tests pin that contract.
```

### `go/internal/core/worktree_provision_cause_test.go:3` — above `import (`

```text
// worktree_provision_cause_test.go — cycle-1474 RED contract for
// `worktree-provisioning-cause-fingerprint`.
//
// cyclerun.go:462-464 prints the provisioning error to stderr and deliberately
// continues with an empty ActiveWorktree; the role-gate then denies every
// source phase. That fail-fast is CORRECT and is not what this task touches.
// What is missing is the CAUSE: the git error never reaches the cycle's
// failure-reason / failure-digest surfaces, so the recorded identity is the
// downstream phase refusal ("scout|infra-error|…") and the real, recurring
// provisioning failure is invisible to the identical-fingerprint breaker and to
// anyone reading the digest afterwards.
//
// These tests drive the REAL RunCycle path (o.newCycleRun provisions through
// the injected WorktreeProvisioner) and read the REAL assembler
// (AssembleFailureDigest) over the cycle's own workspace — no reimplementation
// of either. They deliberately do NOT pin a field name or a file name: any
// existing fail-reason/digest seam that carries the cause satisfies them.
```

### `go/internal/core/worktree_provision_cause_test.go:36` — above `st := &fakeStorage{state: State{LastCycleNumber: 1473}}`

```text
// cycle 1474
```

### `go/internal/core/worktree_provision_cause_test.go:145` — above `func TestWorktreeProvisionFailure_EmptyStderrCauseStillIdentifies(t *testing.T) {`

```text
// TestWorktreeProvisionFailure_EmptyStderrCauseStillIdentifies is the edge
// axis. The live incident shape is rc=255 with NOTHING on stderr, so the cause
// text can be near-empty: the identity must still be non-empty, content-bearing
// and stable, never degrading back to the unexplained class the whole task
// exists to remove.
```

### `go/internal/core/worktree_retry_consolidate_test.go:3` — above `import (`

```text
// worktree_retry_consolidate_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`, adoption site #1:
// gitWorktree.CreateFrom.
//
// CreateFrom is the ADR-0076 continuation-seeding path — the one that
// provisions the worktree for a cycle RESUMING salvaged work — and it issues a
// bare, unretried `git worktree add` (worktree.go:208). A transient lock
// collision there costs the continuation its cycle in exactly the way PR #401
// fixed for Create, with the added insult that the salvaged work is what is
// being dropped on the floor.
//
// Fixtures (initRetryRepo, failingAddRunner) and the sleep no-op init() are
// shared with worktree_retry_test.go — PR #401's file, which must stay green
// and unmodified: "existing tests staying green" is part of this task's
// acceptance criteria, so the consolidation may not break core's
// worktreeAddAttempts / worktreeAddRetrySleep identifiers.
```

### `go/internal/core/worktree_retry_test.go:3` — above `import (`

```text
// worktree_retry_test.go — the cycles-1221/1231/1232/1234/1240 class: N lanes
// of one repo provision concurrently, `git worktree add` takes repo-level
// locks in the SHARED .git (the plane itself is a linked worktree), and a
// transient collision returns rc=255 with nothing but "Preparing worktree" on
// stderr. One collision killed the lane's whole cycle: ActiveWorktree stayed
// empty, CB.2 fail-fasted every dispatch exit=10, three identical fingerprints
// halted the batch — twice in one day, once with ZERO console git activity,
// proving lane-vs-lane contention. The alarm chain downstream is CORRECT; the
// defect is Create treating a transient lock failure as permanent.
```

### `go/internal/core/worktree_retry_test.go:25` — above `func init() { worktreeAddRetrySleep = func(time.Duration) {} }`

```text
// The retry backoff is REAL time.Sleep in production. Without this init, every
// core test whose fixture makes worktree provisioning fail (the scenario
// engine does so by design, dozens of times) pays the full 2s+4s ladder —
// which ballooned the package from ~52s to >600s and timed out the commit-gate
// twice while looking exactly like host contention (the same misattribution
// the incident itself invited). Mirrors runner's settleSleep test-init no-op.
// Tests that COUNT sleeps install their own recorder and restore this no-op.
```

### `go/internal/core/worktree_retry_test.go:56` — above `func failingAddRunner(failures *int, attempts *int) sysexec.RunFunc {`

```text
// failingAddRunner fails the first `worktree add` invocations with the live
// incident's exact shape (rc=255, "Preparing worktree" noise on stderr) and
// delegates everything else — and later attempts — to the real runner.
```

### `go/internal/core/worktree_retryable_test.go:3` — above `import (`

```text
// worktree_retryable_test.go — cycle-1270 blocker (B-4).
//
// gitexec owns the loop and the classifier; core is the CALLER that must
// actually supply it. A classifier that exists in gitexec but is never passed
// leaves the 198s cmd/evolve tax exactly where it was — the "seam whose only
// caller is a test" shape the house rules ban — so these pin core's own
// WorktreeAddRetry value, which is what both gitWorktree.Create and CreateFrom
// hand to the loop.
```

### `go/internal/core/worktree_retryable_test.go:32` — above `if !r.Retryable(255, "Preparing worktree (new branch 'cycle-lane-1')\n") {`

```text
// The live incident shape from PR #401: rc=255, nothing on stderr beyond
// "Preparing worktree". Speeding up permanent failures must not re-break
// the collision absorber — that regression costs a lane its whole cycle.
```

### `go/internal/core/worktree_startref_test.go:55` — above `func TestLaneStartRef_IntegrationHeadAuthority(t *testing.T) {`

```text
// TestLaneStartRef_IntegrationHeadAuthority — 2026-09-09 token-waste root cause
// #3: a fresh lane based on origin/main while the landing branch (the local
// main, AHEAD by unpushed dossier closeouts) sat elsewhere, so Ship rebased and
// re-dispatched Build/Audit for a diff of twelve dossier files. The lane base
// is the INTEGRATION HEAD the landing targets: origin/main when the local main
// is current or behind (the boundary fast-forwards it), the local main when it
// is strictly ahead (its unpublished landings ride the next push), and a
// loud refusal when the two have diverged — no base choice avoids a rebase
// then, so the plane must be reconciled before any spend.
```

### `go/internal/core/worktree_test.go:48` — above `func TestGitWorktree_RelativeBaseRefused(t *testing.T) {`

```text
// TestGitWorktree_RelativeBaseRefused: a RELATIVE EVOLVE_WORKTREE_BASE must
// be refused with an "absolute" error BEFORE any MkdirAll/git runs, so a
// relative base can never silently create worktree dirs under the cwd.
// Mirrors the swarm/provision.go addWorktree guard added in cycle 294.
//
// RED today: gitWorktree.Create has no IsAbs guard — it MkdirAll's the
// relative base and then `git -C <root> worktree add` fails with a *git*
// message that does NOT mention "absolute", so the discriminating
// assertion fails.
```

### `go/internal/core/worktree_test.go:60` — above `g := gitWorktree{baseOverride: relBase}`

```text
// base override (policy.json worktree.base) injected via the struct field —
// the EVOLVE_WORKTREE_BASE env read was removed (flag-reduction, ADR-0064).
```

### `go/internal/core/write_carryover_todos_length_test.go:3` — above `import (`

```text
// write_carryover_todos_length_test.go — cycle-488 RED tests for
// cap-carryover-todo-render-length (Task 2). White-box (`package core`) because
// writeCarryoverTodos is unexported.
//
// writeCarryoverTodos (phase_advisor.go) is the SOLE injection site for
// carryoverTodos into any agent prompt. It caps the COUNT at
// maxCarryoverTodosInPrompt (20) but not the per-item byte length, so an
// oversized stored Action — including the 54 entries already on disk today,
// which Task 1's creation-time fix cannot retroactively shrink — still renders
// in full. This is defense-in-depth: a render-time per-item cap protects the
// router prompt immediately and guards any future creation path.
//
// These tests MUST fail before the Builder adds the per-item render cap (the
// first is RED; the empty-omit and omitted-trailer tests are regression pins
// that are GREEN today and must stay GREEN through the edit).
```

### `go/internal/core/writeaxis_test.go:3` — above `import (`

```text
// writeaxis_test.go — PA-DDK DDK-7b (ADR-0060). The source-write axis (which
// phases write into the cycle worktree) is now declared in the registry via
// `writes_source`, with the WorktreePhase literal as the catalog-less floor for
// the role-gate. This test pins that the two AGREE for every core phase — so the
// config declaration and the security-boundary literal can never silently drift.
// Rename-proof: it iterates whatever the loaded registry contains; no phase name
// is hardcoded.
```

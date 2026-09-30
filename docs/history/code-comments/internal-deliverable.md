# Comment history: `internal/deliverable`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/deliverable/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED:
//
//	const CodeMissingArtifact / CodeStrayInWorktree / CodeInvalidJSON /
//	      CodeMissingKey — each is the code Verify returns on the corresponding
//	      well-formedness failure. We drive Verify against a fixture that triggers
//	      each one and assert the returned violation carries that exact code.
//	func  NewVerifier — the builtin-only core.ContractVerifier constructor;
//	      exercised via VerifyDeliverable on a builtin phase (resolves) and a
//	      user phase (fails to resolve → the documented fail-open error).
//	func  NewVerifierWithCatalog — the catalog-aware constructor; exercised via
//	      VerifyDeliverable on a user phase the builtin verifier cannot resolve.
//
// (NewVerifier / NewVerifierWithCatalog return core.ContractVerifier, whose only
// method is VerifyDeliverable — so naming the constructor and invoking that
// method exercises the whole symbol, not a no-op reference.)
```

### `go/internal/deliverable/apicover_named_test.go:165` — above `func TestSummarizeBadVerdictBaseline_NamesAndExercises(t *testing.T) {`

```text
// --- salvage baseline reporter (cycle-1407) ---------------------------------
//
//	const BadVerdictBaselineFile — the sidecar basename shared by the writer
//	      (recordBadVerdictBaseline) and every reader; exercised by asserting the
//	      writer actually creates a file of that name.
//	type  BaselineSummary / func SummarizeBadVerdictBaseline — the fold over
//	      that sidecar; exercised on a real two-record baseline and asserted on
//	      Total/Recoverable/Rate/ByPattern.
```

### `go/internal/deliverable/architecture_docs.go:12` — above `const CodeMissingArchitectureDocs = "missing_architecture_docs"`

```text
// CodeMissingArchitectureDocs: the build's diff is architecture-class (a policy
// or config vocabulary surface, a new internal package, a phase spec, or a
// trust-kernel source file) and carries no documentation delta. Closed
// vocabulary, snake_case like its siblings above; consumed by the CLI, the
// reviewer gate and the auditor checklist. Design: ADR-0077.
```

### `go/internal/deliverable/architecture_docs.go:19` — above `func ArchitectureDocsViolations(changed []string) []Violation {`

```text
// ArchitectureDocsViolations is the pure classifier over a build diff's
// changed-path set: one CodeMissingArchitectureDocs violation iff the set is
// architecture-class AND carries no docs delta, nil otherwise. Deterministic —
// no LLM judgement, no git shell-out, no filesystem access — so the agent's
// `evolve phase verify` self-check and the host-side gate reach the same verdict
// from the same input (the ADR-0034 no-drift invariant).
//
// The classification itself lives in internal/docsfloor, the single source of
// truth shared with the WARN-level build-handoff floor: this package owns the
// violation vocabulary, that one owns what "architecture" means.
//
// Fail-open: a diff that is not architecture-class never yields the violation,
// so bugfix, test-only and docs-only cycles are byte-identical to before.
```

### `go/internal/deliverable/architecture_docs.go:47` — above `func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error) {`

```text
// VerifyBuildWithChangedPaths is Verify("build", roots) with the documentation
// floor applied to the build's changed-path set: the well-formedness violations
// and the docs-floor violations in one Result, OK recomputed over both.
//
// ADDITIVE by construction — the floor never replaces or masks a well-formedness
// check, so a missing build-report still reports CodeMissingArtifact alongside
// it. Callers with no diff source keep using Verify, which has nothing to
// inspect and therefore never emits the floor violation (the fail-open half of
// the ADR-0034 contract).
//
// Return contract is Verify's: err != nil ⇒ ambiguity, fail OPEN; err == nil
// with !OK ⇒ confirmed violation, fail CLOSED.
```

### `go/internal/deliverable/architecture_docs_stage_test.go:12` — above `func stageBuildWorkspace(t *testing.T, body string) string {`

```text
// Cycle-1150: VerifyBuildWithChangedPathsStage is the resolver+stage-aware form
// the CLI self-check needs. The CLI resolves through the merged phase catalog at
// the configured EVOLVE_PHASE_IO stage; if reaching the docs floor forced it
// down to VerifyBuildWithChangedPaths' built-in/StageOff defaults, the wiring
// would silently WEAKEN the build contract it already enforces. These tests pin
// both halves: the floor still fires, and the stage-gated check is not lost.
```

### `go/internal/deliverable/architecture_docs_test.go:10` — above `const archFloorBuildReport = "# Build Report\n\n## Changes\n- go/internal/policy/policy.go\n\nVerdict: PASS\n"`

```text
// Cycle-1144 RED contract for the docs floor on architecture-class changes
// (inbox item cycle-docs-floor-architecture-changes).
//
// The gap: a cycle can add a policy vocabulary key, a new internal package, or a
// new phase spec and ship with ZERO documentation delta — `fleet.landing`
// (policy.go:1043-1208) is the live proof: real resolved-config semantics,
// no entry in docs/architecture/control-flags.md or
// docs/operations/runtime-reference.md. `deliverable.Verify` is the ADR-0034
// seam where the `evolve phase verify` self-check and the host-side reviewer
// gate run byte-identical logic, so the floor belongs here — enforced once,
// for both callers.
//
// Contract under test (to be implemented by Builder):
//
//	const CodeMissingArchitectureDocs = "missing_architecture_docs"
//	func ArchitectureDocsViolations(changed []string) []Violation
//	func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error)
//
// ArchitectureDocsViolations is a PURE classifier over the build diff's changed
// path set (deterministic — no LLM judgement, no git shell-out), returning one
// CodeMissingArchitectureDocs violation iff the set is architecture-class AND
// carries no docs delta. VerifyBuildWithChangedPaths is the wiring: the same
// well-formedness result Verify("build", …) produces, with the docs-floor
// violations appended and OK recomputed. Fail-open is the default — a diff that
// is not architecture-class never yields the violation, so pure bugfix, test
// and docs cycles are byte-identical to today.
```

### `go/internal/deliverable/architecture_docs_test.go:156` — above `func TestVerify_ArchitectureClassRequiresDocsDelta(t *testing.T) {`

```text
// --- AC5: WIRING PROOF — the classifier reaches a Verify result -----------
//
// A classifier nobody calls is a no-op. This is the gate-wiring proof: the
// verdict must surface through the ADR-0034 verify seam, on a build-report
// that is otherwise perfectly well-formed.
```

### `go/internal/deliverable/architecture_docs_test.go:264` — above `ws := t.TempDir()`

```text
// Verify("build", …) has no diff to inspect, so it must never emit the
// docs-floor violation — callers with no diff source keep today's exact
// behaviour (the fail-open half of the ADR-0034 contract).
```

### `go/internal/deliverable/catalogaware.go:21` — above `func VerifyCatalogAware(phase string, roots phasecontract.Roots) (Result, error) {`

```text
// VerifyCatalogAware runs the well-formedness checks resolving the phase's
// contract through the project's merged catalog (built-in registry +
// .evolve/phases user specs), locating the project from roots.EvolveDir.
// PRECONDITION: roots.EvolveDir must be <projectRoot>/.evolve (the shape
// every production constructor builds — paths.go, verifier.go rootsFor, the
// runner reconcile site); any other shape silently resolves the wrong
// project and degrades. Missing EvolveDir or an unloadable catalog degrades
// to built-in-only resolution WITH a stderr WARN — built-in phases always
// verify, a catalog glitch never hard-fails the check, but a degrade on the
// reconcile path can flip a user phase's outcome and must be visible.
//
// PhaseIO (ADR-0050 §3.10): as of Slice 1 this reconcile-on-timeout rung is
// stage-aware via VerifyCatalogAwareStage, so it honors the SAME stage-gated
// failure-context requirement as the host gate (NewReviewerWithCatalogStage) and
// the salvage rung (NewVerifierWithCatalogStage). VerifyCatalogAware is retained
// as the byte-identical back-compat wrapper (StageOff) for callers that pass no
// stage.
```

### `go/internal/deliverable/catalogaware.go:42` — above `func VerifyCatalogAwareStage(phase string, roots phasecontract.Roots, phaseIO config.Stage) (Result, error) {`

```text
// VerifyCatalogAwareStage is VerifyCatalogAware threaded with the EVOLVE_PHASE_IO
// rollout stage (ADR-0050 §3.10). The stage flows through to VerifyWithStage at
// every resolution branch (catalog-resolved, builtin fallback, and the degraded
// catalog-load path); at StageOff it is byte-identical to the pre-3.10 path, and
// at >=StageEnforce a build/scout/triage FAIL-without-block artifact reconciled on
// a timeout race is now caught here too (the reconcile rung reaches the same
// verdict as the host gate, not just deferring to it).
```

### `go/internal/deliverable/catalogaware_phaseio_test.go:10` — above `func TestVerifyCatalogAwareStage_FailWithoutBlock_BlocksAtEnforce(t *testing.T) {`

```text
// ADR-0050 Phase 3.10 Slice 1: the reconcile-on-timeout rung (VerifyCatalogAware)
// is the THIRD deliverable-verify path. 3.8 threaded the host gate and the salvage
// rung with cfg.PhaseIO but deliberately left this one at StageOff (the TODO(3.10)).
// Slice 1 makes it stage-aware via VerifyCatalogAwareStage, so at enforce it honors
// the SAME failure-context requirement the host gate does. These reuse the 3.8
// phaseioFailFixtures / failReport / writeFile / hasCode helpers (same package).
// With roots.EvolveDir == "" the catalog-aware path takes the BuiltinResolver
// branch, so the gate behaviour matches VerifyWithStage for built-in phases.
```

### `go/internal/deliverable/challenge_token_test.go:3` — above `import (`

```text
// challenge_token_test.go — cycle-269 incident RED tests: the challenge-token
// protocol (scout mints <workspace>/challenge-token.txt + embeds it in
// scout-report.md; downstream reports must echo it as proof-of-read) was
// enforced ONLY at audit — unrecoverable: a perfect EGPS-green build FAILed
// the whole cycle over a missing echo. The bash→Go migration had also dropped
// the prompt-side injection (resolved-prompt.txt: zero mentions), so fallback
// builders never even saw the instruction. This moves the invariant to the
// machine-checkable boundary where the EXISTING correction loop (PR #60) can
// re-dispatch with the exact fix BEFORE audit: contracts opt in via
// Contract.RequireChallengeToken (the RequireFailureContext precedent).
```

### `go/internal/deliverable/declared_deliverables_e2e_test.go:3` — above `import (`

```text
// declared_deliverables_e2e_test.go — ADR-0100, the proof at the public seam.
//
// A REAL cycle (production storage + ledger, the catalog-aware contract
// reviewer at enforce) whose builder writes a contract-valid build-report.md
// and NO handoff-build.json — a secondary the fixture catalog declares the
// agent owes, the way the registry declares triage-decision.json. Before ADR-0100 the cycle proceeded
// to audit. Now: the gate rejects, the ladder re-dispatches with the file
// named in the directive, and either the correction lands (the cycle ships)
// or the ladder exhausts and the cycle ends FAILED_EXPLAINED naming the file.
// Both entrypoints, because the resume loop is a separate implementation.
```

### `go/internal/deliverable/declared_deliverables_e2e_test.go:160` — above `func TestDeclaredDeliverables_Resume_MissingHandoff_IsCorrectedThenFails(t *testing.T) {`

```text
// TestDeclaredDeliverables_Resume_MissingHandoff_IsCorrectedThenFails is the
// resume twin: RunCycleFromPhase is a separate loop, and this session's
// resume-parity fixes (#568, #571) are why the twin is not optional.
```

### `go/internal/deliverable/declared_effects_e2e_test.go:3` — above `import (`

```text
// declared_effects_e2e_test.go — ADR-0100 slice 2, the proof at the public
// seam.
//
// A REAL cycle (production storage + ledger, the catalog-aware contract
// reviewer at enforce) whose triage writes a contract-valid triage-report.md
// and triage-decision.json committing to inbox item "x" — and never claims
// it: the item stays pending at the plane's inbox root, exactly the 1631
// shape (the agent "claimed" a worktree copy) and the 1623 shape (the claim
// was denied and the spine ran anyway). Before this slice the cycle proceeded
// to tdd. Now: the gate rejects, the ladder re-dispatches with the item and
// the command named in the directive, and either the claim lands (the cycle
// ships) or the ladder exhausts and the cycle ends FAILED_EXPLAINED naming
// the effect. An empty commitment owes no claim and still ends as triage
// no-work.
```

### `go/internal/deliverable/declared_effects_e2e_test.go:183` — above `func TestDeclaredEffects_EmptyCommitment_OwesNoClaim(t *testing.T) {`

```text
// An explicit empty commitment owes the declared-effects gate NO claim (no
// inbox-claim correction is issued, triage runs once) — and since the
// cycle-1623 P1 (inbox 2026-09-12T10-00-00Z-triage-empty-commitment-still-
// dispatches-spine), an empty commitment beside a still-CLAIMABLE inbox item is
// no longer credited as planned no-work: the host stops at triage with the
// named claim-failed reason and no implementation phase. The legitimate
// no-work disposition needs an inbox with nothing claimable (second case).
```

### `go/internal/deliverable/deliverable.go:1` — above `package deliverable`

```text
// Package deliverable is the shared verifier for phase-agent deliverables. The
// `evolve phase verify` self-check (cmd_phase_verify.go) and the host-side
// contract gate (reviewer.go) both call Verify so the agent's pre-finish check
// and the harness's post-phase gate run BYTE-IDENTICAL logic — they can never
// drift. Design: ADR-0034.
//
// Scope: WELL-FORMEDNESS ONLY (does the deliverable exist at the contracted
// path, in the right shape, with the required sections/keys and a parseable
// verdict). Semantic correctness — "is the report's content right" — is the
// auditor's LLM-judged job. A Verify PASS must never be read as a semantic PASS
// (the validation-vs-guardrail split; anti-Goodhart).
//
// Fail-open / fail-closed contract, encoded in the return signature:
//
//	err != nil       → ambiguity / infrastructure fault (unknown phase) → caller fails OPEN
//	err == nil, !OK  → confirmed agent violation                        → caller fails CLOSED
//	err == nil, OK   → well-formed
```

### `go/internal/deliverable/deliverable.go:82` — above `CodeMissingChallengeToken = "missing_challenge_token"`

```text
// CodeMissingChallengeToken: a RequireChallengeToken contract's report
// does not echo the minted <workspace>/challenge-token.txt token
// (proof-of-read, cycle-269). Checked here — the correctable boundary —
// so the PR-#60 correction loop re-dispatches with the exact token
// BEFORE the audit backstop. Fail-open when no token was minted.
```

### `go/internal/deliverable/deliverable.go:92` — above `CodeFailureContextMissing = "failure_context_missing"`

```text
// CodeFailureContextMissing: a sentinel-declared FAIL/WARN lacks the
// ADR-0039 structured failure block. (snake_case to match this closed
// vocabulary; ADR prose spells it with hyphens.)
```

### `go/internal/deliverable/deliverable.go:96` — above `CodeFailureClassUnknown = "failure_class_unknown"`

```text
// CodeFailureClassUnknown: the failure block's class is outside the
// failurelog vocabulary. The class drives the retry envelope — an unknown
// class declines the direct repair grant (cycle 1684's
// "superseded-predicate-contradiction" cost a full retrospective before a
// retry that carried none of the audit's findings) — so it is validated
// at this boundary and the correction hands the agent the vocabulary.
```

### `go/internal/deliverable/deliverable.go:103` — above `CodeMissingSecondary   = "missing_secondary"`

```text
// ADR-0100 — an AGENT-OWED secondary output (registry outputs.agent_owed)
// is absent, blank, or unparseable. The code stays one word per class so
// the correction ladder's same-defect identity recognizes a repeat; the
// message names the file, and that message is the correction the agent
// is re-dispatched with.
```

### `go/internal/deliverable/deliverable.go:111` — above `CodeMissingEffect = "missing_effect"`

```text
// ADR-0100 slice 2 — declared effects (effects.go). missing_effect: a
// committed inbox item the phase was instructed to claim is not under this
// cycle's processing/ dir; unbound_effect: the registry names an effect no
// deterministic check binds (a registry defect, not an agent one).
```

### `go/internal/deliverable/deliverable.go:138` — above `func VerifyWithStage(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver, phaseIO config.Stage) (Re…`

```text
// VerifyWithStage is VerifyWith threaded with the EVOLVE_PHASE_IO rollout stage
// (ADR-0050 §3.8). The stage gates only the additive RequireFailureContextPhaseIO
// check for build/scout/triage (fires at StageEnforce); every other check is
// stage-independent, so VerifyWithStage(..., StageOff) == the pre-3.8 VerifyWith.
```

### `go/internal/deliverable/deliverable.go:158` — above `if err := verifySecondaries(&res, c, roots); err != nil {`

```text
// ADR-0100: the agent-owed secondaries are judged after the primary so a
// correction can name everything the phase still owes in one directive.
```

### `go/internal/deliverable/deliverable.go:163` — above `if err := verifyEffects(&res, c, roots); err != nil {`

```text
// ADR-0100 slice 2: then the declared effects, so one directive names
// every output AND effect the phase still owes.
```

### `go/internal/deliverable/deliverable.go:214` — above `const (`

```text
// Write-in-flight grace window (cycle-1212). A phase agent's final deliverable
// write is not atomic with respect to the verify call that follows it: the
// self-check and the host contract gate can both observe ENOENT (create not yet
// visible) or a zero-length file (bytes not yet flushed) for a deliverable that
// IS being written. A single unretried read cannot tell that from "never
// written", and both surface as a CONFIRMED violation — a false FAIL that fails
// CLOSED. So absence/emptiness is treated as provisional for a bounded window.
//
// The window must be long enough to cover a lagging write and short enough that
// a genuinely missing deliverable is still reported promptly (a retry that waits
// minutes is its own outage). Deliberately NOT configurable: this is an I/O
// robustness constant, not a phase setting — no flag, no dial.
//
// LAYERING (review HIGH on the first cut): the host runner already re-probes
// Verify up to 16x at 200ms for missing/empty/MALFORMED artifacts
// (runner.go verifyReconcileDeliverable), so on that path this window nests
// inside the outer retry and a genuinely-absent artifact's confirmation cost
// is ~16x(probe+500ms) ≈ 11s worst-case — accepted: it is paid once, only on
// a phase that produced nothing, and is far cheaper than the false FAIL it
// prevents. This grace layer EXISTS for the callers with NO outer retry (the
// CLI self-check, `evolve phase verify`). Partial-but-non-blank content is
// deliberately NOT retried here — mid-write truncation is closed at the
// SOURCE by the bridge artifact-ready cross-poll debounce (completion.go):
// the wait loop reports a deliverable finished only after artifactStableTicks
// consecutive poll ticks observe an UNCHANGED (size, mtime) key, so a file
// still being appended to never reaches this reader half-written. mtime is in
// the key because a size-only window is blind to an equal-length fix-up Edit,
// and that same window gates the DESTRUCTIVE relocation of a non-canonical
// fallback rather than following it (copying first would snapshot a partial
// file to the canonical path and remove the source the agent still holds
// open). One path there completes without a closed window — the wait loop's
// final post-cancel poll — and it is restricted to rename-only
// canonicalization, which relinks an inode and so cannot truncate or delete;
// stating the exception is the point, because the unqualified version of this
// sentence was audited as a claim the code refuted (cycle-1256 D2). What is
// malformed-but-present stays the runner's reconcile territory.
```

### `go/internal/deliverable/deliverable.go:317` — above `if c.RequireFailureContext || (c.RequireFailureContextPhaseIO && phaseIO >= config.StageEnforce) {`

```text
// ADR-0039 §7 / ADR-0050 §3.8: a sentinel-declared FAIL/WARN must carry the
// structured failure block. RequireFailureContext (audit) enforces this
// unconditionally; RequireFailureContextPhaseIO (build/scout/triage) enforces
// it only once the PhaseIO rollout reaches enforce — off/shadow/advisory stay
// byte-identical, so a phase that has not yet adopted the sentinel cannot be
// false-blocked before the cutover. Applies ONLY to sentinel verdicts —
// legacy prose-only artifacts stay legal forever. The message is the
// correction directive (re-dispatched verbatim).
```

### `go/internal/deliverable/deliverable.go:348` — above `if c.RequireChallengeToken {`

```text
// Cycle-269: the challenge-token echo (proof the agent read the upstream
// report) was audit-only — a perfect EGPS-green build FAILed the whole
// cycle, unrecoverably, over a missing echo. Enforce at THIS boundary so
// the correction loop fixes it pre-audit. The minted token lives in the
// workspace; absent/empty file ⇒ nothing to echo ⇒ silent (fail-open).
```

### `go/internal/deliverable/deliverable.go:364` — above `func checkStray(res *Result, c phasecontract.Contract, roots phasecontract.Roots) {`

```text
// checkStray flags a deliverable the agent wrote into the worktree root instead
// of the workspace — the exact failure the recoverBuildLeak fixes
// (cb604d6/f96537c) chased reactively. Only meaningful for workspace-target
// contracts with a distinct worktree.
```

### `go/internal/deliverable/deliverable.go:433` — above `}`

```text
// A sentinel with an out-of-vocabulary verdict is not a valid declaration;
// fall through to the prose scan rather than trusting it. ADR-0050 §3.10
// Slice 5: below enforce that fall-through reaches the prose scan; at
// enforce the prose scan is gated off, so an out-of-vocab sentinel resolves
// to false (CodeBadVerdict) with no prose rescue.
```

### `go/internal/deliverable/deliverable.go:439` — above `if phaseIO < config.StageEnforce {`

```text
// ADR-0050 §3.10 Slice 5: the prose substring scan is the legacy fallback for
// older templates; at enforce the sentinel is mandatory, so gate it off
// (>= StageEnforce). Below enforce it stays active — byte-identical.
```

### `go/internal/deliverable/deliverable_phaseio_test.go:9` — above `func TestVerdictPresent_EnforceRequiresSentinel(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 5: verdictPresent's prose substring scan is the legacy
// fallback for older templates; at enforce the evolve-verdict sentinel is
// mandatory. Below enforce the prose scan stays active (byte-identical).
```

### `go/internal/deliverable/deliverable_test.go:11` — above `func writeFile(t *testing.T, dir, name, content string) {`

```text
// Layer 3 of the deliverable-contract feature (ADR-0034): the shared verifier
// both the `evolve phase verify` self-check AND the host-side contract gate
// call. The fail-open/fail-closed contract is encoded in the return signature:
//
//	err != nil          → ambiguity/infra (unknown phase, unreadable dir) → caller fails OPEN
//	err == nil, !OK     → confirmed agent violation                      → caller fails CLOSED
//	err == nil, OK      → well-formed deliverable
//
// Verify checks WELL-FORMEDNESS ONLY (location, sections, verdict parseable,
// JSON keys). Semantic correctness stays the auditor's job (anti-Goodhart).
```

### `go/internal/deliverable/deliverable_test.go:306` — above `func TestVerify_AuditFailWithoutFailureBlock_Violation(t *testing.T) {`

```text
// --- ADR-0039 §7: failure-context conditionality ---
```

### `go/internal/deliverable/dispatched_artifact_test.go:3` — above `import (`

```text
// dispatched_artifact_test.go — pin for the intent-delta contract-path skew
// fix (inbox intent-delta-contract-path-skew 0.88): the runner threads the
// EXACT dispatched artifact path through Roots.DispatchedArtifact, so Verify
// judges the file the phase was ASKED to write — intent in DELTA mode
// dispatches intent-delta.md while its registry contract names intent.md,
// and Verify previously judged the wrong file (found by PR #389's
// single-read work, deliberately not papered over there).
```

### `go/internal/deliverable/effects.go:13` — above `type effectCheck func(res *Result, roots phasecontract.Roots) error`

```text
// effects.go — ADR-0100 slice 2: a declared EFFECT is verified at the phase
// boundary exactly as a declared output is.
//
// A phase's persona can be instructed to do something outside its workspace
// that later phases and sibling lanes depend on. Triage's inbox claim is the
// one that exists today: `evolve inbox-mover claim` moves the item into
// processing/cycle-N/ so no other lane's triage can select it. Batch cycles
// 1630 (claim refused by the sandbox) and 1631 (the worktree's tracked COPY
// of the inbox was claimed, the plane's item stayed dispatchable) showed the
// report and the decision looking complete while the effect had not happened,
// and nothing judged it — the spine ran on an unclaimed commitment.
//
// The registry declares effects by name; effectChecks binds each name to one
// deterministic check (a registry lookup, not a strategy hierarchy — one
// entry today). A declared name with no binding is a registry defect and is
// reported as such: re-dispatching an agent cannot bind a check.
```

### `go/internal/deliverable/effects_test.go:3` — above `import (`

```text
// effects_test.go — ADR-0100 slice 2: a declared EFFECT is verified at the
// phase boundary exactly as a declared output is.
//
// Triage's persona claims every inbox item it ingests (`evolve inbox-mover
// claim`, Step 0a.4) so that no sibling lane can select the same item. Two
// batch cycles showed the claim silently not happening while the report and
// decision looked complete: 1631 claimed the worktree's tracked COPY of the
// inbox (the plane's item stayed dispatchable) and 1630's claim was refused by
// the sandbox. Nothing judged the effect, so the spine ran on an unclaimed
// commitment. The codes below are stable so the ladder's same-defect identity
// recognizes a repeat and escalates instead of re-dispatching blindly.
```

### `go/internal/deliverable/failure_class_test.go:11` — above `func failureExemplarFor(class string) *phasecontract.FailureBlock {`

```text
// Cycle 1684 (2026-09-15): the audit declared failure class
// "superseded-predicate-contradiction" — a class no policy category knows —
// the gate verified the deliverable, and the retry envelope declined the
// repair round on "unrecognised class" with no log line and no signal: a
// one-phase builder fix (retire the superseded predicate) went through a
// full retrospective first and re-entered tdd/build with none of the audit's
// findings in its briefs. The class drives the envelope, so the gate
// validates it at the boundary and the correction hands the auditor the
// vocabulary.
```

### `go/internal/deliverable/failure_context_phaseio_test.go:10` — above `var phaseioFailFixtures = map[string]struct {`

```text
// Phase 3.8 (ADR-0050): the structured-failure-block requirement — enforced
// today only for audit via the unconditional Contract.RequireFailureContext —
// is generalized to build/scout/triage, but gated on the EVOLVE_PHASE_IO dial
// so it is byte-identical (dormant) until enforce. A non-audit phase that
// self-reports a FAIL/WARN verdict sentinel WITHOUT a structured failure block
// is a violation ONLY at PhaseIO>=enforce; at off/shadow/advisory it does not
// fire. PASS sentinels and legacy prose-only artifacts stay legal forever.
```

### `go/internal/deliverable/grace_test.go:3` — above `import (`

```text
// grace_test.go — the write-in-flight grace window (cycle-1212 salvage,
// review BLOCK: the grace path crosses every phase of every cycle and shipped
// untested in the first cut). All timing goes through the graceSleep seam —
// no real-time waits.
```

### `go/internal/deliverable/repaired_content_test.go:10` — above `func TestRepairedVerdictContent_RepairsTheSoleRecoverableBadVerdictInMemory(t *testing.T) {`

```text
// Cycle 1685 (2026-09-15): the auditor wrote its verdict as fenced JSON
// without the sentinel wrapper. The gate salvaged the sole bad_verdict,
// persisted the repaired report and approved it — but the runner had already
// classified the UNREPAIRED bytes ("no parseable verdict → FAIL"), so a
// red_count=0 cycle sealed FAIL with no failure class and no repair round.
// The runner must classify the bytes the gate will approve: the pure repair
// is one function both consumers share.
```

### `go/internal/deliverable/reportsize.go:12` — above `const CodeHandoffBudgetExceeded = "handoff_budget_exceeded"`

```text
// reportsize.go — cycle-565 Slice S1 of report-size-contracts-jit-artifacts: a
// per-artifact token/size budget on the never-evict "## Handoff Summary" section
// (phasecontract.HandoffSummary). The section's PRESENCE rides the normal
// contract gate (CodeMissingSection); its SIZE rides a separate shadow-first
// rollout dial so a miscalibrated budget can be observed before it can ever
// block a cycle.
```

### `go/internal/deliverable/reportsize.go:74` — above `func VerifyWithReportSize(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver, phaseIO, reportSizeG…`

```text
// VerifyWithReportSize is VerifyWithStage threaded with the report-size gate's
// own rollout stage (cycle-565 Slice S1) — exactly as VerifyWithStage was added
// as a new layer over VerifyWith rather than changing an existing signature, so
// no existing call site (cmd_phase_verify.go, reviewer.go, verifier.go,
// catalogaware.go) churns. The reportSizeGate dial is INDEPENDENT of the
// ContractGate stage: dormant at off/shadow (byte-identical Violations to
// VerifyWithStage), blocking only at enforce.
```

### `go/internal/deliverable/reportsize.go:86` — above `if reportSizeGate < config.StageAdvisory {`

```text
// Off/shadow: fully dormant. Leave Violations byte-identical to
// VerifyWithStage so wiring the layer in cannot change existing behavior for
// any cycle that has not opted the gate above shadow. Advisory is the WARN
// rung (cycle-646): it RECORDS CodeHandoffBudgetExceeded so the host-side
// Reviewer can log a would-block warning, but the Reviewer keeps it
// non-blocking until reportSizeGate==enforce — the size dial's own staged
// rollout, independent of the ContractGate stage.
```

### `go/internal/deliverable/reportsize_test.go:8` — above `func TestEstimateTokens(t *testing.T) {`

```text
// reportsize_test.go — RED contract for cycle-565 Slice S1 of
// report-size-contracts-jit-artifacts: a per-artifact token/size budget check
// on the never-evict "## Handoff Summary" section (phasecontract.HandoffSummary
// — see handoffsummary_test.go in go/internal/phasecontract). No tokenizer
// dependency exists in this repo (go.mod has none), so EstimateTokens uses the
// common ~4-chars-per-token heuristic already implicit in this codebase's other
// byte-length budgets (e.g. core.salvageMaxBytes) rather than inventing a real
// tokenizer.
//
// RED today: EstimateTokens, HandoffSectionContent, CheckHandoffBudget, and
// CodeHandoffBudgetExceeded do not exist (compile failure).
```

### `go/internal/deliverable/reportsize_warn_test.go:13` — above `func TestVerifyWithReportSize_AdvisoryRecordsWarnViolation(t *testing.T) {`

```text
// reportsize_warn_test.go — cycle-646 slice of report-handoff-size-contract-scout.
//
// cycle-565 Slice S1 shipped CheckHandoffBudget/VerifyWithReportSize/the
// Reviewer wiring, but left "shadow" and "advisory" BEHAVIORALLY IDENTICAL:
// VerifyWithReportSize returns immediately (no res.add) for any
// reportSizeGate < StageEnforce, so an oversized Handoff Summary produces
// exactly zero observable signal — not even the reviewer's own "would-block"
// log line — until the operator flips the gate straight to enforce. That is
// not the staged WARN-mode rollout the source spec
// (.evolve/inbox/processed/cycle-565/bb0f4815-...json) and this cycle's
// scout-report Task 2 describe ("checked by the contract gate in WARN mode
// (not enforce)").
//
// This slice narrows "advisory" into the missing WARN rung: it must RECORD
// the CodeHandoffBudgetExceeded violation (so the existing Reviewer.Review
// shadow/advisory branch logs "would-block" via r.logf) while still
// Approve==true (non-blocking). "shadow" stays fully silent/dormant exactly
// as TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes already pins —
// this file does not touch that contract.
//
// RED today: VerifyWithReportSize's early-return threshold is
// `reportSizeGate < config.StageEnforce`, so StageAdvisory is silent — no
// CodeHandoffBudgetExceeded violation is recorded and the Reviewer's
// "would-block" log line never fires for an oversized handoff section.
```

### `go/internal/deliverable/reportsize_warn_test.go:54` — above `func TestVerifyWithReportSize_ShadowStaysSilent_Negative(t *testing.T) {`

```text
// TestVerifyWithReportSize_ShadowStaysSilent_Negative is the scope-boundary
// negative test: "shadow" is the fully-dormant rung (cycle-565 contract,
// pinned in TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes) and
// must NOT gain a violation from this slice — only "advisory" becomes WARN.
// A naive "advisory OR shadow both warn" implementation fails this.
```

### `go/internal/deliverable/reviewer.go:20` — above `const defaultBreakerThreshold = 3`

```text
// reviewer.go — Layer 4 of the deliverable contract (ADR-0034): the host-side
// gate. It runs the SAME Verify the `evolve phase verify` self-check runs, wired
// behind core.DeliverableReviewer at the orchestrator's per-phase seam (composed
// after evalgate via core.ChainReviewers).
//
// Posture (matches the validated June-2026 fail-safe guidance):
//   - Ambiguity / infra fault (unknown phase, unreadable dir) → fail OPEN.
//   - Confirmed well-formedness violation → fail CLOSED at StageEnforce.
//   - StageShadow → log-only (every violation approved).
//   - Circuit breaker: the breaker trips on CONTRACT/QUALITY violations (not
//     process exit codes); after N consecutive blocks it demotes enforce→
//     advisory and emits an escalation line, so a miscalibrated gate cannot
//     halt the autonomous loop. A clean cycle resets it (half-open).
```

### `go/internal/deliverable/reviewer.go:40` — above `phaseIO config.Stage`

```text
// EVOLVE_PHASE_IO rollout stage; gates the RequireFailureContextPhaseIO check (ADR-0050 §3.8). Default StageOff → byte-identical.
```

### `go/internal/deliverable/reviewer.go:41` — above `reportSizeGate         config.Stage`

```text
// reportSizeGate gates the Handoff Summary token-budget check (cycle-565
// Slice S1), independent of the ContractGate stage: blocks only at
// StageEnforce. reportSizeBudgetTokens is the budget it enforces. Zero-value
// (StageOff/0) ⇒ byte-identical to pre-S1 behavior.
```

### `go/internal/deliverable/reviewer.go:51` — above `signals *gatesignal.Reporter`

```text
// signals is the gate's Signal Center producer (ADR-0101 S2b): every
// decision Review reaches is one gate.passed / gate.rejected event. The
// Null Object until WithSignals installs the root's Center.
```

### `go/internal/deliverable/reviewer.go:81` — above `func NewReviewerWithCatalogStage(stage config.Stage, cat phasespec.Catalog, phaseIO config.Stage, opts ...Option) core.D…`

```text
// NewReviewerWithCatalogStage is NewReviewerWithCatalog threaded with the
// EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8). The stage gates only the
// additive RequireFailureContextPhaseIO check for build/scout/triage (blocks at
// StageEnforce, and only when the ContractGate stage is also enforce); every
// other gate behavior is unchanged, so passing StageOff equals the legacy
// constructor.
```

### `go/internal/deliverable/reviewer.go:91` — above `func NewReviewerWithCatalogStageReportSize(stage config.Stage, cat phasespec.Catalog, phaseIO, reportSizeGate config.Sta…`

```text
// NewReviewerWithCatalogStageReportSize is NewReviewerWithCatalogStage plus the
// report-size gate (cycle-565 Slice S1). reportSizeGate gates the Handoff
// Summary token-budget check — INDEPENDENT of the ContractGate stage, blocking
// only at StageEnforce; budgetTokens is the budget it enforces. Zero-value
// (StageOff/0) ⇒ byte-identical to NewReviewerWithCatalogStage, so wiring it in
// changes nothing until the report-size gate is deliberately promoted.
```

### `go/internal/deliverable/reviewer.go:129` — above `func WithSignals(c *signalcenter.Center) Option {`

```text
// WithSignals hands the gate the Signal Center it reports every decision
// through (ADR-0101 S2b). A nil Center keeps the Null Object.
```

### `go/internal/deliverable/reviewer.go:163` — above `func (r *Reviewer) VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (Result, err…`

```text
// VerifyForClassification is the gate's own verification offered to the
// runner's verdict engine, so there is ONE verifier: the bytes the engine
// classifies are the bytes this Reviewer will approve. At enforce a sole
// recoverable bad_verdict is salvaged, persisted and reported HERE (before
// classification), and the repaired, OK result is returned; Review then
// meets a clean file. Below enforce nothing is persisted (the gate would only
// would-block), so the verified bytes come back as they are. Cycle 1685
// (2026-09-15): with two verifiers the engine classified the unrepaired
// bytes as "no parseable verdict → FAIL" while the gate approved the
// repaired file, and a red_count=0 cycle sealed FAIL with no failure class.
```

### `go/internal/deliverable/reviewer.go:235` — above `if r.stage != config.StageEnforce {`

```text
// EFFECTS live below the dial; the DECISION above it (cycle-1442 audit
// H3). Computing "would this have salvaged" is precisely what a shadow
// soak is for, but the block inherited its position from the
// unconditional observability record above and so also rewrote the
// judged artifact, appended the telemetry sidecar and touched the
// breaker while the gate reported itself disabled — a soak run then
// measures a system its own "disabled" gate already mutated.
```

### `go/internal/deliverable/reviewer.go:253` — above `if r.persistSalvage(check, in.Phase, roots, res, salvaged) {`

```text
// Write back the bytes the salvage re-verify actually approved. This
// Reviewer is salvage's only production caller and it consumes the
// salvaged Result and nothing else, so the artifact on disk is the ONLY
// channel by which the NEXT phase to read ArtifactPath can observe what
// the gate approved; leaving the malformed original there let a FAIL
// sentinel the strict parse could not read be re-resolved to PASS by a
// downstream prose scan (cycle-1441 audit H1, HIGH).
//
// Fail CLOSED. If the approved bytes cannot be persisted we do not
// approve on them — control falls through to the ordinary block path,
// which is exactly the behaviour that predated the salvage stage. This
// is the one place the gate must not fail open: failing open here
// reinstates the defect (approval over bytes nobody downstream will
// ever see) instead of merely declining a recovery.
```

### `go/internal/deliverable/reviewer.go:274` — above `if r.reportSizeGate < config.StageEnforce && res.onlyViolation(CodeHandoffBudgetExceeded) {`

```text
// Report-size handoff-budget is warn-only below its own enforce dial
// (cycle-646): at advisory VerifyWithReportSize records the violation so we
// log a would-block WARN here, but the size gate must never block a cycle
// until reportSizeGate==enforce — independent of the ContractGate stage. If
// the ONLY reason to block is that warn-only size violation, approve. Any
// co-occurring real contract violation still falls through to the block path.
```

### `go/internal/deliverable/reviewer.go:393` — above `func persistSalvagedArtifact(path, judged, content string) error {`

```text
// persistSalvagedArtifact writes the salvage-approved bytes over the artifact
// the gate judged, so the file a downstream phase re-reads is the file the gate
// actually approved. Before this existed the gate approved `content` while
// leaving the malformed original on disk — the two diverged silently and a FAIL
// sentinel could be re-resolved to PASS by a prose scan downstream (cycle-1441
// audit H1, HIGH).
//
// An empty path is the contract's "declares no file" discriminator
// (deliverable.go:55, ship/NoArtifact): there is no artifact to reconcile, so
// this is a no-op success rather than an error. The write goes through
// internal/atomicwrite so a reader concurrent with the gate can never observe a
// half-written report — the repo's single implementation of that, not a local
// copy.
```

### `go/internal/deliverable/reviewer.go:410` — above `current, err := os.ReadFile(path)`

```text
// Re-read and compare against the bytes the decision was computed over.
// atomicwrite is an unconditional rename, so without this a still-live
// agent that rewrote its report after the gate's read would have its
// CORRECTED verdict silently replaced by the repaired STALE bytes — with
// Approve=true (cycle-1442 adversarial F1).
//
// STATED GUARANTEE, deliberately narrower than compare-and-swap
// (go-reviewer HIGH): this refuses when the file changed BEFORE this read.
// It is not atomic with the rename below, so a write landing inside that
// microsecond window is still clobbered. Closing that would need a lock the
// agent side does not take, which buys a far smaller window than it costs;
// what matters is that the comment does not claim a guarantee the code does
// not provide. Refusing is free: the caller fails closed and the ordinary
// block path runs, which is what
// happened before salvage existed.
```

### `go/internal/deliverable/reviewer_breaker_failloud_test.go:3` — above `import (`

```text
// reviewer_breaker_failloud_test.go — RED contract for the fable5 deep-scan
// finding selfcheck-breaker-fail-loud (inbox weight 0.91, cycle-618 scout),
// deliverable.reviewer.go circuit-breaker-persistence half.
//
// Context. writeBreaker persists the contract-gate's consecutive-block
// counter so it survives the per-cycle orchestrator reconstruction. Today the
// write/rename errors are discarded outright:
//
//	func writeBreaker(path string, n int) {
//	    ...
//	    data, _ := json.Marshal(breakerState{Consecutive: n})
//	    tmp := path + ".tmp"
//	    if os.WriteFile(tmp, data, 0o644) == nil {
//	        _ = os.Rename(tmp, path) // atomic
//	    }
//	}
//
// A write failure here silently resets the breaker's effective state to
// "never persisted" — readBreaker treats a missing/unreadable file as zero
// (fail-open by omission), so a genuinely tripped breaker can lose its count
// across a restart with zero operator-visible signal. This does not change the
// documented fail-open posture (the breaker still degrades enforce→advisory
// rather than aborting) — it only makes the persistence failure WARN-visible,
// matching the sibling `[contract-gate]` WARN convention already used
// elsewhere in this file (see logf calls in the enforce/shadow branches).
//
// RED today: writeBreaker discards the WriteFile error with no stderr output,
// so the assertion below fails for the right reason (empty captured stderr)
// rather than a compile error — this is a behavioral (fail-loud) fix, not a
// new API.
```

### `go/internal/deliverable/reviewer_demotion_test.go:3` — above `import (`

```text
// reviewer_demotion_test.go — the contract gate must ANNOUNCE its own
// demotion (inbox contract-block-cli-escalation, P1 0.95).
//
// Before this, a circuit-open returned ReviewResult{Approve:true} — structurally
// indistinguishable from a deliverable that actually satisfied its contract. The
// orchestrator therefore could not tell "the gate passed you" from "the gate
// gave up on you", which is why the batch-19 and batch-21 demotions were
// invisible outside one stderr line.
```

### `go/internal/deliverable/reviewer_explanation_section_test.go:27` — above `func TestReviewer_ExplanationSectionRequiredOnlyWhileContractActive(t *testing.T) {`

```text
// TestReviewer_ExplanationSectionRequiredOnlyWhileContractActive — cycles 1601
// and 1603 died on "audit-report.md is missing ## Explanation Documentation"
// as a terminal FAIL. The section is a contract violation the correction
// ladder re-dispatches — but only when the cycle's explanation contract is
// active; a cycle without it is never asked for the section.
```

### `go/internal/deliverable/reviewer_reportsize_ctor_test.go:13` — above `func TestNewReviewerWithCatalogStageReportSize_ThreadsGate(t *testing.T) {`

```text
// reviewer_reportsize_ctor_test.go — the catalog-aware constructor
// NewReviewerWithCatalogStageReportSize (cycle-565 Slice S1) actually threads
// the report-size gate + budget onto the Reviewer, mirroring
// TestNewReviewerWithCatalogStage_ThreadsPhaseIO for the phaseIO dial: an
// oversized handoff section is blocked at reportSizeGate=enforce and approved
// (dormant) at off — proving the two new params are wired, not dropped. This is
// the production wiring the cmd_cycle.go call site uses, so it must be exercised
// through the public constructor (apicover named-coverage, cycle-542 lesson).
```

### `go/internal/deliverable/reviewer_salvage_surface_test.go:14` — above `func TestReviewerReview_SalvageSurfacesSummaryLine(t *testing.T) {`

```text
// reviewer_salvage_surface_test.go — caller proof for SalvageSummaryLine.
//
// cycle-1392 audit LOW dd17d798e155571ecd91be63e14050ab6: the renderer was
// exported, documented in README §8 as "surfaced", and called from nothing but
// tests. A seam whose only caller is a test is dead code, and a doc that
// promises it is an overclaim. This test pins the OPPOSITE: the line reaches an
// operator through the real production entry point, `Reviewer.Review` — not by
// calling the renderer directly (that is TestSalvageSummaryLine_Surfaces...'s
// job) but by driving a salvageable deliverable through the gate and asserting
// the gate's own log stream carried the summary.
```

### `go/internal/deliverable/reviewer_signals_test.go:3` — above `import (`

```text
// reviewer_signals_test.go — ADR-0101 S2b: every decision Reviewer.Review
// reaches is reported through the Signal Center (gatesignal.Reporter), so the
// orchestrator's listener and the triage reader see "checked → advanced" or
// "checked → rejected: <file>" for every phase boundary. RED first.
```

### `go/internal/deliverable/reviewer_signals_test.go:192` — above `type ownedResolver struct{ owed, effects []string }`

```text
// ownedResolver overlays ADR-0100's declared secondaries and effects onto a
// built-in contract, the way CatalogResolver overlays the registry's.
```

### `go/internal/deliverable/reviewer_spec_test.go:67` — above `func TestNewReviewerWithCatalogStage_ThreadsPhaseIO(t *testing.T) {`

```text
// Phase 3.8 (ADR-0050): the catalog-aware ...Stage constructors actually thread
// the EVOLVE_PHASE_IO dial onto the gate/verifier. A build report that
// self-reports FAIL without a structured failure block is blocked at enforce and
// approved (dormant) at off — proving the phaseIO param is wired, not dropped.
```

### `go/internal/deliverable/reviewer_spec_test.go:142` — above `func TestReviewerWithCatalog_FailsOpenWhenNoContractResolves(t *testing.T) {`

```text
// TestReviewerWithCatalog_FailsOpenWhenNoContractResolves — test-plan P0 #5:
// a native/no-output phase that resolves to NO contract (CatalogResolver
// miss) is AMBIGUITY at the gate even at StageEnforce: approve (fail open),
// and the consecutive-block breaker must not move. This is the
// "[contract-gate] ship: ambiguity, failing open" line from the 2026-06-12
// soak — pinned so the fail-open never silently becomes a block (or a
// breaker leak) for contract-less phases.
```

### `go/internal/deliverable/reviewer_test.go:13` — above `func reviewInput(phase, workspace, projectRoot string) core.ReviewInput {`

```text
// Layer 4 (ADR-0034): the host-side contract gate. Same verifier as the agent
// self-check, wired behind core.DeliverableReviewer. Fail-open on ambiguity,
// fail-closed on confirmed violation at enforce, with a circuit breaker that
// demotes enforce→advisory after N consecutive blocks so a miscalibrated gate
// cannot brick the loop.
```

### `go/internal/deliverable/reviewer_test.go:37` — above `func TestReviewer_FailureContextPhaseIO_BlocksOnlyAtBothEnforce(t *testing.T) {`

```text
// Phase 3.8 (ADR-0050): the generalized failure-context check blocks at the
// gate ONLY when BOTH ContractGate==enforce AND PhaseIO==enforce. A build report
// that self-reports FAIL without a structured failure block is blocked there,
// and approved (dormant) at every lower PhaseIO stage even while ContractGate
// enforces — so the rollout cannot false-block before the cutover.
```

### `go/internal/deliverable/salvage_extract.go:3` — above `import (`

```text
// salvage_extract.go — the SECOND deliverable of the schema-aligned salvage
// layer (docs/research/deliverable-alignment-2026-08/README.md, portfolio item
// `schema-aligned-salvage-layer`): EXTRACTION, gated on cycle-1389's measured
// 9% (15/167) recoverable-malformed baseline (bad-verdict-baseline.jsonl,
// salvage_instrument.go's own header). That measurement cleared the item's
// "instrumentation before extraction" precondition (README §6/§7); this file
// is the extraction/coercion stage the inbox item's `fix` text describes.
//
// Per README §3.3 (BAML SAP citation): lenient, LOGGED, bounded coercion of
// bytes already present in Result.Content — reformatting only, never
// inventing a field value — with a hard refusal on genuine ambiguity (the
// inbox item's own "fail only on genuine ambiguity" constraint). A REFUSED
// salvage never touches the Result; an APPROVED one returns the repaired bytes
// it verified, so the approval and the bytes it was granted over can never
// diverge (cycle-1441 audit H1).
```

### `go/internal/deliverable/salvage_extract.go:62` — above `func verdictCandidates(content string, stringAware bool) []verdictSpan {`

```text
// verdictCandidates locates every verdict-bearing candidate ANYWHERE in the
// content, counting OBJECTS rather than the containers they sit in.
//
// The regex-only predecessor counted one candidate per FENCE and used
// verdictObjRE (`\{[^{}]*"verdict"\s*:[^{}]*\}`), whose character classes
// cannot cross a brace. Two candidates sharing one fence therefore counted as
// one, and a verdict object carrying a nested block — every ADR-0039 FAIL,
// which nests a structured `failure` object — was invisible entirely, so a
// stray PASS beside a substantive FAIL read as unambiguous and was laundered
// into an approval (cycle-1399 audit dfa28f113d8269306a5fe304b8091bbb2, HIGH).
// Counting balanced objects fixes both halves with one primitive: nesting is
// tracked by depth, and containers stop mattering.
//
// stringAware selects whether a brace inside a JSON string literal moves the
// depth. Both readings are run (see candidateCount): string-awareness is right
// for JSON, but a markdown deliverable is PROSE with JSON in it, and prose
// quoting cannot be trusted to be balanced.
//
// The scan fails CLOSED on truncation: an unterminated object carrying a
// verdict key is still returned as a candidate, so it makes the content
// ambiguous rather than disappearing from the count.
```

### `go/internal/deliverable/salvage_extract.go:149` — above `func candidateCount(content string) int {`

```text
// candidateCount reports how many independent verdict-bearing candidates the
// content carries. SalvageVerdict's ambiguity rule is "more than one candidate
// ANYWHERE in the content" (the inbox item's explicit hard constraint), so a
// sentinel payload coexisting with a stray bare object, two objects in one
// fence, and two objects in two fences are all equally ambiguous.
//
// It takes the LARGER of the two readings, because a counter that can only
// UNDERCOUNT implements "refuse when the attacker permits", not "refuse on
// ambiguity". The string-aware reading alone was exactly that: one unpaired `"`
// in prose left the scan believing every later byte was inside a string, so
// BOTH candidates of a two-candidate report vanished and `> 1` never fired —
// while ClassifyBadVerdict's step 2 computes parity FENCE-LOCALLY
// (verdictObjSpan runs over the fence body alone) and still qualified the
// stray fenced PASS, laundering a phase's own malformed FAIL into an approval
// (cycle-1424 audit d4982b388c4982275303ee68529b9313d, CRITICAL, reproduced
// through Reviewer.Review). The brace-only reading cannot be silenced by
// quoting; the string-aware reading still catches the mirror case where a
// string CONTAINS a stray brace and the brace-only reading would merge two
// objects into one. Neither reading dominates, so ambiguity is what EITHER can
// see — the same fail-closed asymmetry the classifier is built on:
// over-counting costs a salvage, under-counting costs the gate.
//
// Both of those readings are nevertheless STATEFUL scans, and cycle 1432's
// probe showed each has its own one-character silencer in ordinary prose: an
// unpaired `"` desynchronises the string-aware reading (aware=0), and a single
// unmatched `{` opens a depth the brace-only reading never closes, absorbing
// every later object into one truncation-fallback span (raw=1). With one of
// each in prose, `max(0, 1) == 1` is not `> 1`, the refusal never fires, and a
// fenced decoy PASS beside the phase's own displaced FAIL is laundered into an
// approval (cycle-1424 CRITICAL d4982b388c4982275303ee68529b9313d, still open
// through cycle 1428's quote-only fix; re-probed at the gate seam by
// go/acs/cycle1432/predicates_test.go TestC1432_001/002).
//
// verdictKeyCount is the third reading, and the answer to "a reading that
// cannot be desynchronised by an unbalanced delimiter": it carries NO scan
// state at all, so no earlier byte can change what a later one means. It
// counts the `"verdict":` keys themselves — one per verdict-bearing payload,
// whatever containers, nesting or stray delimiters surround them — which is
// the property the two structural scans trade away for span offsets. It is
// only ever consulted through the max, so like the brace-only reading it can
// add refusals and never remove one; a stray `"verdict":` mentioned in prose
// costs a salvage, which is the side of the asymmetry this gate is built on.
```

### `go/internal/deliverable/salvage_extract.go:206` — above `func verdictKeyCount(content string) int {`

```text
// verdictKeyCount is candidateCount's stateless reading: the number of
// `"verdict":` keys in the content. Deliberately not brace- or string-aware —
// that awareness is exactly the scan state an unbalanced delimiter hijacks.
//
// It counts the key the way the RE-VERIFY pass reads it, not the way the byte
// literal spells it. encoding/json matches struct fields case-INSENSITIVELY
// and decodes `\uXXXX` escapes, so `"Verdict":` and `"verdict":` are
// verdict keys to the decoder while a byte-literal count sees none of them —
// a decoy the ambiguity guard cannot see but the decoder can act on
// (cycle-1432 audit d4fa6591dcd07c365884c64925a8e3dbe, CRITICAL C1). Counting
// is only ever consulted through candidateCount's max, so widening it can add
// refusals and never remove one: an obfuscated key in prose costs a salvage,
// which is the side of the asymmetry this gate is built on.
```

### `go/internal/deliverable/salvage_extract.go:279` — above `func RepairedVerdictContent(res Result) (string, bool) {`

```text
// RepairedVerdictContent returns res.Content with its SOLE recoverable
// bad_verdict repaired to the canonical sentinel line — the pure half of the
// salvage, shared by the two consumers that must agree on the same bytes: the
// Reviewer (which re-verifies, persists the repaired artifact and reports the
// salvage) and the runner's verdict engine (which classifies). Cycle 1685
// (2026-09-15): the engine classified the unrepaired bytes ("no parseable
// verdict → FAIL") while the gate approved the repaired file, and a
// red_count=0 cycle sealed FAIL with no failure class. Nothing is written
// here.
```

### `go/internal/deliverable/salvage_extract.go:289` — above `if res.OK || !res.onlyViolation(CodeBadVerdict) {`

```text
// SOLE violation, never membership. hasCode ("is a bad_verdict in there
// anywhere") let a bad_verdict co-occurring with missing_section or
// missing_challenge_token be salvaged wholesale — OK forced true and ALL
// violations erased, including the cycle-269 anti-forgery proof-of-read
// check. That is a report-forgery bypass on the gate's own decision seam
// (cycle-1392 audit CRITICAL-1, probe-confirmed). Salvage repairs the
// VERDICT and nothing else, so it may only ever act when the verdict is
// the one and only thing wrong (the cycle-1397 negative predicate package
// has since been retired; the sole-violation cases live in this package's
// salvage tests and TestRepairedVerdictContent_…).
```

### `go/internal/deliverable/salvage_extract.go:329` — above `var check Result`

```text
// Re-verify the REPAIRED bytes instead of hand-setting OK=true. Flipping
// OK from the classification alone skipped every content check the strict
// parse never reached — most sharply RequireFailureContext, which turned a
// malformed FAIL sentinel with no ADR-0039 failure block into an approval
// (cycle-1392 audit MEDIUM-3). The caller's OWN roots are used, not an
// empty set: with empty roots the roots-dependent checks (challenge-token
// echo, stray-in-worktree) resolve their paths against the process CWD, so
// a challenge-token.txt planted in whatever directory the loop happens to
// run from could fail the re-verify and deny an otherwise valid salvage
// (cycle-1399 audit d8b22040, LOW — fail-closed, but attacker-influenced).
// Re-verifying under the same roots the original Verify used removes the
// CWD dependency entirely, and cannot newly block anything: those checks
// already ran over these same bytes upstream, and any violation they raised
// makes the result non-sole — refused above, before repair.
```

### `go/internal/deliverable/salvage_extract.go:352` — above `salvaged.Content = repaired`

```text
// Carry the bytes the re-verify above actually approved. Returning the
// ORIGINAL Content with only OK/Violations flipped made the gate report
// OK=true over a byte stream that was never the byte stream it verified:
// every consumer of Result.Content — and, via the Reviewer's write-back,
// every downstream phase re-reading ArtifactPath — still saw the malformed
// original, so a FAIL sentinel the strict parse could not read could be
// re-resolved to PASS by a prose scan further down the pipeline
// (cycle-1441 audit H1, HIGH). Approved bytes and returned bytes are now
// the same bytes, by construction.
```

### `go/internal/deliverable/salvage_extract.go:365` — above `func repairVerdict(content string, cls BadVerdictClassification) (string, bool) {`

```text
// repairVerdict rebuilds the deliverable's bytes with the single recovered
// verdict payload re-emitted, in place, as a canonical evolve-verdict sentinel
// comment. It is REFORMATTING only (README §3.3): the payload's own bytes are
// carried across verbatim except for the trailing commas JSON forbids, and no
// field value is invented. The result is a CANDIDATE for re-verification: it
// becomes Result.Content only after that re-verify passes, never before.
//
// It repairs the span cls QUALIFIED and nothing else. It deliberately performs
// no search of its own: its predecessor re-derived a span per pattern — the
// trailing-comma branch took `sentinelPayloadRE.FindStringSubmatch(content)`,
// the FIRST match, with no string-literal check the classifier had already
// applied — so a report quoting a decoy sentinel got CLASSIFIED on its genuine
// malformed FAIL and REPAIRED on the decoy PASS, and the gate approved
// (cycle-1406 audit CRITICAL-1). Adding a second quoting check here would have
// closed that instance and left two independently-drifting notions of an
// admissible span; taking the offsets instead means "repair a span the
// classifier rejected" has no expressible form.
//
// Returns ok=false when the qualified payload cannot be recovered into valid
// JSON (or the span does not address these bytes), which is itself a refusal:
// an unrepairable candidate never reaches the re-verify pass.
```

### `go/internal/deliverable/salvage_extract.go:470` — above `continue`

```text
// A record a killed writer truncated mid-line. Skipped by the
// breakdown, and — since the headline is derived from this same
// slice — skipped by the total too. The predecessor computed
// `total := len(lines)` over the raw split while the breakdown
// skipped blanks and truncations, so the two numbers came from
// different populations and the coercion count operators read
// overstated itself (cycle-1399 audit dae44191, MEDIUM).
```

### `go/internal/deliverable/salvage_extract.go:486` — above `scoped := make([]appliedRec, 0, len(recs))`

```text
// Scope to THIS run. The sidecar is a repo-level, never-rotated file, so
// its all-time total was being logged by reviewer.go as the current phase's
// figure — a real number with a false frame, which an operator reads as
// "this cycle coerced N verdicts" (cycle-1399 audit d8b22040, LOW). Records
// this run appended carry its id; when NONE do (a legacy sidecar written
// before the tag existed) the whole file is reported rather than nothing,
// so an older checkout degrades to the previous behaviour instead of going
// silent.
```

### `go/internal/deliverable/salvage_extract.go:534` — above `func operatorPatternLabel(pattern string) string {`

```text
// operatorPatternLabel decides what an untrusted sidecar `pattern` value may
// call itself in the single-line `[contract-gate]` operator log.
//
// Its predecessor (sanitizeLogField) checked the value against a CHARACTER
// CLASS `^[A-Za-z0-9_.-]+$` and Go-quoted anything else. That closed the
// control-character half of cycle-1399 audit dae44191 (a newline forged a
// second operator line) but not the other half: `salvage-applied.jsonl` is a
// repo-level file any phase can append to, and a forged value that is merely
// alphanumeric — `circuit-open-notice` — passed the class untouched and
// rendered as a breakdown entry indistinguishable from a real one, fabricating
// gate state in the one stream an operator judges the gate's health from
// (carryover todo-salvage-summary-line-rejects-untrusted-sidecar-text, HIGH).
//
// Membership in the closed SalvagePattern set is the check the shape actually
// warrants: the renderer mints these strings itself (recordSalvageApplied), so
// anything else is by definition not ours. Known constants render bare and
// legible; everything else folds into one `unknown` bucket — which subsumes the
// character-class guard (no control character can survive a closed allowlist)
// while still COUNTING the record, so a bogus pattern shows up as evidence
// rather than vanishing.
```

### `go/internal/deliverable/salvage_extract_test.go:23` — above `const unpairedQuoteAmbiguityBypass = "## Verdict\n\n" +`

```text
// unpairedQuoteAmbiguityBypass reproduces cycle-1424 audit defect
// d4982b388c4982275303ee68529b9313d (CRITICAL) through the production seam.
//
// One unpaired `"` in prose is the whole exploit. It leaves the string-aware
// scan believing every byte after it is inside a string literal, so BOTH
// verdict objects below become invisible to candidateCount and the
// `candidateCount > 1` ambiguity guard reads a two-candidate report as
// unambiguous. Classification is unaffected, because step 2 computes quote
// parity FENCE-LOCALLY (verdictObjSpan runs over the fence body alone) and
// still qualifies the fenced PASS. Salvage therefore repairs the stray PASS
// while the report's own genuine — and malformed — FAIL stays unparseable, and
// ParseVerdictSentinelFull reads the repaired PASS: a report whose intended
// verdict is FAIL reaches Approve=true.
```

### `go/internal/deliverable/salvage_extract_test.go:133` — above `if got.Content == res.Content {`

```text
// CONTRACT CHANGE (cycle-1441 audit H1, HIGH): this assertion used to demand
// Content come back byte-identical to the malformed input. That is precisely
// the defect — salvage re-verified `repaired` and then returned the ORIGINAL,
// so the gate reported OK=true over bytes it had never approved. An approved
// salvage now returns the bytes it verified; the "changes nothing" invariant
// survives intact where it belongs, on the REFUSAL path
// (TestSalvageVerdict_RefusesGenuinelyAbsent, below).
```

### `go/internal/deliverable/salvage_extract_test.go:148` — above `func TestSalvageVerdict_RecoversSentinelTrailingComma(t *testing.T) {`

```text
// TestSalvageVerdict_RecoversSentinelTrailingComma covers the OTHER recoverable
// sentinel shape: a canonical evolve-verdict comment whose payload is JSON with
// a trailing comma. It drives repairVerdict's SalvagePatternTrailingComma branch
// (salvage_extract.go:376-379), which shipped at zero executions because every
// existing test drives the fenced-json path (cycle-1441 audit H2, HIGH) — in a
// transform with cycle-1406/cycle-1399 CRITICAL history for mis-repaired spans.
```

### `go/internal/deliverable/salvage_hardening_test.go:3` — above `import (`

```text
// salvage_hardening_test.go — the acceptance criteria four consecutive lane
// audits demanded of the salvage stage and no attempt was ever scoped to close
// (cycles 1432/1434/1441/1442). Each test here IS one finding, written RED
// before its fix:
//
//	H3 (1442) stage dial not honored ...... TestReviewerReview_ShadowStage_NoSideEffects
//	H1 (1442) fail-closed arm untested .... TestReviewerReview_PersistFailure_FailsClosed
//	M2 (1441) breaker cleared by salvage .. TestReviewerReview_SalvageIsBreakerNeutral
//	F1 (1442 adversarial) TOCTOU .......... TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate
//	M2 (1442) torn line bricks the report . TestCountSalvageApplied_TolerantOfTornLines
//	H2 (1441) residue: json.Valid guard ... TestSalvageVerdict_RefusesUnrepairablePayload
//
// The unifying invariant the auditors kept restating: a salvage may only ever
// COST a recovery, never buy one — every refusal path must be executed by a
// test, and no effect may escape the enforce dial.
```

### `go/internal/deliverable/salvage_hardening_test.go:49` — above `func TestReviewerReview_ShadowStage_NoSideEffects(t *testing.T) {`

```text
// TestReviewerReview_ShadowStage_NoSideEffects — cycle-1442 audit H3.
//
// The salvage block was placed above the stage dial (it inherited the position
// of the observability record, which is unconditional BY DESIGN). At
// shadow/advisory the gate therefore rewrote the judged artifact on disk,
// appended the telemetry sidecar and touched the breaker — while reporting
// itself "disabled". A shadow soak run to decide whether salvage is safe then
// measures a system the disabled gate has already mutated.
//
// Effects, not the decision, are what the dial governs: computing "would this
// have salvaged" in shadow is the whole point of shadow, so this test pins the
// three EFFECTS byte-identical, not the absence of the computation.
```

### `go/internal/deliverable/salvage_hardening_test.go:106` — above `func TestReviewerReview_PersistFailure_FailsClosed(t *testing.T) {`

```text
// TestReviewerReview_PersistFailure_FailsClosed — cycle-1442 audit H1.
//
// The build report called this "the one place the gate must not fail open" and
// the branch shipped with zero executions (profile `reviewer.go:154.85,156.4
// 1 0`). If the approved bytes cannot be persisted, approving on them
// reinstates the very defect persistence closed: an approval over bytes no
// downstream reader will ever see.
```

### `go/internal/deliverable/salvage_hardening_test.go:146` — above `func TestReviewerReview_SalvageIsBreakerNeutral(t *testing.T) {`

```text
// TestReviewerReview_SalvageIsBreakerNeutral — cycle-1441 audit M2(b).
//
// The repo rule is that salvage rungs are breaker-NEUTRAL. The salvage approve
// path called resetBreaker, so a phase emitting recoverable-malformed reports
// held the consecutive-block counter at zero forever: neither the second-block
// escalation ladder nor the third-block breaker could ever fire, and a
// persistently malformed producer became invisible to both.
```

### `go/internal/deliverable/salvage_hardening_test.go:177` — above `func TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate(t *testing.T) {`

```text
// TestPersistSalvagedArtifact_RefusesWhenFileChangedUnderGate — cycle-1442
// adversarial-review F1 (raised, never adjudicated).
//
// The gate reads the artifact once, decides over those bytes, then writes the
// repair. atomicwrite is an unconditional rename, not a compare-and-swap, so a
// still-live agent that rewrites its report in the window between the read and
// the write has its CORRECTED verdict silently replaced by the repaired stale
// bytes — with Approve=true. The write must be conditional on the file still
// holding the bytes the decision was computed over.
```

### `go/internal/deliverable/salvage_hardening_test.go:219` — above `func TestCountSalvageApplied_TolerantOfTornLines(t *testing.T) {`

```text
// TestCountSalvageApplied_TolerantOfTornLines — cycle-1442 audit M2.
//
// The sidecar is append-only and unauthenticated: a torn line is an ordinary
// crash artifact, not an attack. The in-process summary tolerates one by
// design; the CLI counter hard-errored on it and `evolve salvage report` then
// exited 1 with NO output, discarding the already-computed baseline section.
// Two consumers of one file disagreeing on robustness is the defect — and the
// tolerant reading must still be HONEST about what it skipped, or a forged
// torn line becomes a way to hide records.
```

### `go/internal/deliverable/salvage_hardening_test.go:253` — above `func TestSalvageVerdict_RefusesUnrepairablePayload(t *testing.T) {`

```text
// TestSalvageVerdict_RefusesUnrepairablePayload — cycle-1441 audit H2 residue.
//
// repairVerdict's last guard refuses to emit a sentinel whose payload is not
// valid JSON (json.Valid, salvage_extract.go). It shipped at zero executions:
// the classifier's own recoverable shapes normally repair to valid JSON, so no
// existing fixture reaches it. A guard nothing executes is a guard nobody knows
// still works.
```

### `go/internal/deliverable/salvage_hardening_test.go:280` — above `func TestRepairVerdict_RefusesSpanNotAddressingContent(t *testing.T) {`

```text
// TestRepairVerdict_RefusesSpanNotAddressingContent — the other refusal arm of
// the same guard (theme T2: every failure arm executed, not just the happy
// one). The offsets are qualified against the content they were computed from;
// a classification carried to DIFFERENT bytes addresses nothing, and repairing
// on it is precisely the classifier↔repairer divergence the offset threading
// exists to make impossible (cycle-1406 CRITICAL-1).
```

### `go/internal/deliverable/salvage_instrument.go:91` — above `type BadVerdictClassification struct {`

```text
// BadVerdictClassification is the classifier's read of one bad_verdict
// deliverable. Reason is always non-empty for a recoverable classification:
// a silent classification is not observability — the baseline record has to
// say WHY a future salvage stage would have recovered this report.
//
// span and payload carry the QUALIFIED BYTE OFFSETS into the exact content this
// classification was computed from, and are meaningful only when Recoverable.
// They exist because the repairer used to re-derive its own span from the same
// content with a bare first-match regex and no quoting check, so classifier and
// repairer could qualify DIFFERENT spans: a report quoting a decoy `PASS`
// sentinel in prose was classified on its own genuine malformed `FAIL` but
// REPAIRED on the decoy, and since ParseVerdictSentinelFull takes the LAST
// parseable sentinel, a phase turned its own FAIL into an APPROVAL with one
// stray `"` (cycle-1406 audit CRITICAL-1). Handing the offsets to repairVerdict
// makes the divergence structurally impossible rather than guarded by a second
// quoting check that can drift out of step with this one.
//
//	span    — the half-open range to REPLACE with a canonical sentinel line
//	          (the whole sentinel comment / the whole fence / the bare object).
//	payload — the half-open range of the JSON object's own bytes inside it.
```

### `go/internal/deliverable/salvage_instrument.go:135` — above `func ownSentinelPayload(content string) (body string, span, payload verdictSpan, ok bool) {`

```text
// ownSentinelPayload selects the payload of the report's OWN verdict sentinel
// and returns the document with every ECHOED sentinel span excised.
//
// Two rules, both load-bearing, and neither sufficient alone (cycle-1407):
//
//   - Quote-awareness. A sentinel span delimited by a backtick is prose quoting
//     the sentinel SHAPE — a contract example, or another phase's verdict the
//     auditor pasted while describing it. Keying off such a span is the
//     cycle-641 lesson verbatim ("classifiers MUST exclude any span that is a
//     verbatim echo of injected prompt/instruction text"), and it is exactly
//     how the cycle-1298 corpus buried a real FAIL behind five quoted decoys.
//   - Tail anchoring. Among the spans that survive, the LAST wins: a producer
//     emits its real verdict at the tail, while examples accumulate above it.
//     Same selection rule as phasecontract.ParseVerdictSentinelFull, reached
//     independently here because this classifier must see the payloads that
//     parser REJECTED (it cannot reuse a parser that only returns valid ones).
//
// Last-match-wins alone is not decoy immunity — a decoy quoted BELOW the real
// sentinel would win — and quote-awareness alone is not enough either, since a
// document's own sentinel is routinely preceded by unquoted-looking examples.
//
// minimal: "quoted" means CONTAINED IN A CLOSED inline-code span, computed by
// pairing backtick runs (below). Upgrade path if a blockquoted (`> `) echo is
// ever observed: extend inlineCodeSpans' notion of a delimiter — not a new
// parser.
//
// Echoed spans are BLANKED rather than deleted so every offset into body is
// also an offset into content: the repairer acts on the span this classifier
// qualified (BadVerdictClassification.span), and a body rebuilt by deletion
// would silently shift every one of those offsets.
```

### `go/internal/deliverable/salvage_instrument.go:200` — above `func sentinelInClosedSpan(quoted [][2]int, start, end int) bool {`

```text
// sentinelInClosedSpan reports whether the sentinel span [start,end) lies
// entirely inside one of the CLOSED inline-code spans — the signature of a
// sentinel being DISCUSSED rather than emitted. (Named to stay clear of the
// TestNoQuotedEchoRegression symbol tripwire: the historical helper of the
// old name proved adjacency, not containment, and the ban on that name is
// deliberately kept armed — the cross-lane merge of cycles 1438/1439 landed
// this correct containment logic under the banned name and red'd main.)
//
// Containment, not adjacency (cycle-1407 finding F1): a lone backtick that
// never closes is ordinary prose punctuation, and reading it as a delimiter
// excised reports' own genuine verdicts, inflating the not-recoverable count
// this instrumentation exists to measure honestly.
```

### `go/internal/deliverable/salvage_instrument_test.go:3` — above `import (`

```text
// salvage_instrument_test.go — names AND exercises every exported symbol added
// by the salvage-instrumentation layer (export-naming floor, ADR-0069):
//
//	type  SalvagePattern, BadVerdictClassification
//	const SalvagePatternNone / SalvagePatternFencedJSON /
//	      SalvagePatternTrailingComma / SalvagePatternDisplaced
//	func  ClassifyBadVerdict
//
// The ACS predicates (go/acs/cycle1389) drive the same symbols through the real
// VerifyWithStage/Reviewer path; this suite covers the classifier's PRECEDENCE
// and negative axes, which the acceptance criteria do not reach.
```

### `go/internal/deliverable/salvage_instrument_test.go:144` — above `const strayBacktickPreamble = "The auditor noted a stray ' tick in the transcript and moved on.\n\n"`

```text
// strayBacktickPreamble is the historical poison: a lone, never-closed backtick
// sitting in prose ahead of the report's own verdict shape. The cycle-1406/1407
// defect (`isQuotedEcho`, since removed) read backtick *adjacency* as proof of a
// quoted echo without requiring the backtick run to close, so this preamble
// alone flipped a recoverable verdict to "genuinely absent, not recoverable" —
// poisoning the very baseline this layer exists to measure honestly.
```

### `go/internal/deliverable/salvage_instrument_test.go:301` — above `const decoyCorpusPath = "../phasecontract/testdata/cycle1298-quoted-decoys.md"`

```text
// decoyCorpusPath is the ONE canonical cycle-1298 adversarial-review report:
// five sentinel decoys quoted into prose plus the report's own tail sentinel.
// It is read from phasecontract's testdata rather than re-typed, so this suite
// and phasecontract's sentinel_tailanchor_test.go stay bound to the same bytes
// (single-source-of-truth — a copied excerpt would drift silently).
```

### `go/internal/deliverable/salvage_instrument_test.go:319` — above `func TestClassifyBadVerdict_QuotedDecoyCorpus(t *testing.T) {`

```text
// TestClassifyBadVerdict_QuotedDecoyCorpus is the durable regression case for
// decoy immunity (carryover todo-schema-aligned-salvage-layer-decoy-fixture).
//
// The classifier must key off the report's OWN verdict sentinel, never off a
// sentinel the report merely QUOTES while discussing one — the cycle-641 lesson
// ("classifiers MUST exclude any span that is a verbatim echo of injected
// prompt/instruction text"), and the exact bypass this corpus was landed to
// document. Before cycle-1407 the classifier took the FIRST sentinel-shaped
// span in the document, which in this corpus is a quoted decoy, so it never
// reached the real tail sentinel at all.
//
// Both directions are pinned, because each guards against the fix for the
// other: "first wins" fails the middle case, and a naive "last wins" fails the
// third. Only genuine quote-awareness plus tail anchoring passes all three.
```

### `go/internal/deliverable/salvage_instrument_test.go:376` — above `func TestClassifyBadVerdict_UnmatchedBacktickFalsePositive(t *testing.T) {`

```text
// TestClassifyBadVerdict_UnmatchedBacktickFalsePositive is the RED reproduction
// of adversarial-review finding F1 (cycle-1407 adversarial-review-report.md).
//
// isQuotedEcho (salvage_instrument.go) treats backtick ADJACENCY alone as proof
// a sentinel span is a quoted echo — it never checks that the backtick run
// actually closes. A single stray, unmatched backtick immediately before a
// report's OWN tail sentinel is therefore indistinguishable from real inline
// code, and the genuine (malformed-but-recoverable) sentinel is excised as if
// it were a decoy. That is the opposite failure mode from the corpus above:
// there the classifier must ignore a real quote; here it must NOT ignore a
// real sentinel merely because one unmatched backtick sits next to it.
//
// This directly widens the error bars on the recoverable-malformed rate the
// extraction stage (schema-aligned-salvage-layer) is gated on — see F1's
// "Impact" note — so it is pinned as its own case rather than folded into the
// decoy-corpus table above, which only covers BALANCED inline-code echoes.
```

### `go/internal/deliverable/salvage_keycase_test.go:5` — above `func TestCandidateCount_CountsDecoderVisibleVerdictKeys(t *testing.T) {`

```text
// salvage_keycase_test.go — regression for the ambiguity guard's blind spot on
// verdict keys the RE-VERIFY pass reads but the byte-literal count did not:
// encoding/json matches struct fields case-insensitively and decodes \uXXXX
// escapes, so `"Verdict":` / `"VERDICT":` / `"verdict":` are all verdict
// keys to the decoder. Counting them with a case-sensitive byte literal made a
// decoy invisible to the guard while remaining actionable by the decoder
// (cycle-1432 audit d4fa6591dcd07c365884c64925a8e3dbe, CRITICAL C1).
//
// The assertion is on candidateCount — the guard's own decision input — and it
// is directional: every case here must count MORE than one candidate, because
// candidateCount is consulted only as `> 1 ⇒ refuse`. Widening a count can add
// refusals and never remove one.
```

### `go/internal/deliverable/salvage_report.go:3` — above `import (`

```text
// salvage_report.go — the FIRST READER of the baseline sidecar
// salvage_instrument.go has been writing since cycle-1389.
//
// The rank-2 portfolio item `schema-aligned-salvage-layer`
// (docs/research/deliverable-alignment-2026-08/README.md §7) gates its
// extraction/coercion stage on a MEASURED recoverable-malformed rate. The
// instrumentation half landed and has appended a record per bad_verdict block
// ever since — but nothing ever read those records back, so the gate was
// blocked on a number no code computed and §7's baseline had to be produced by
// hand. This file computes it: a pure fold over the JSONL, surfaced by
// `evolve salvage report` (go/cmd/evolve/cmd_salvage.go).
//
// Still measurement, not extraction: nothing here coerces a verdict, and the
// summarizer performs no I/O of its own — it reads whatever io.Reader the
// caller opened.
```

### `go/internal/deliverable/salvage_report.go:71` — above `func CountSalvageApplied(r io.Reader) (saved int, malformed int, err error) {`

```text
// CountSalvageApplied folds the salvage-applied JSONL into the number of
// coercions the extraction stage actually performed — every run, not just this
// process's (run-scoping is SalvageSummaryLine's job, which answers the
// different question "what did THIS cycle salvage"). Pure: no filesystem
// access, no mutation of its input.
//
// Records of a foreign event_type are skipped: the sidecar is a repo-level file
// any emitter may append to, and counting a foreign line would inflate the one
// number an operator reads as "the gate coerced this many verdicts". Blank
// lines are not records.
//
// TRUST POSTURE (cycle-1442 audit M1/M2). This sidecar is append-only and
// UNAUTHENTICATED, so both of its failure directions are handled explicitly and
// neither is silent:
//
//   - Unreadable line. Returned as the second value (malformed), never fatal.
//     A torn line is an ordinary crash artifact — the in-process summary has
//     always tolerated one — and hard-erroring here discarded the entire
//     already-computed operator report (exit 1, no output) over one bad byte.
//     Reporting the skip count is what keeps tolerance honest: silently
//     dropping lines would make a deliberately torn record a way to HIDE
//     salvages.
//   - Inflated count. Foreign event types do not count, and the number this
//     returns is advisory telemetry, never a gate input — nothing in the
//     decision path reads it.
```

### `go/internal/deliverable/salvage_report_saved_test.go:38` — above `func TestCountSalvageApplied_EmptyAndTorn(t *testing.T) {`

```text
// TestCountSalvageApplied_EmptyAndTorn — an empty sidecar is 0 (the normal
// never-salvaged state); a torn append is REPORTED, not fatal.
//
// INVERTED, declared loudly (cycle-1442 audit M2). This test previously
// asserted a torn line must be "a loud error", and that literal reading is
// what the auditor tabled as the defect: `evolve salvage report` exited 1 with
// NO output over one crash-torn byte, discarding the entire already-computed
// baseline section — while the in-process summary tolerated the very same
// shape by design. Two consumers of one unauthenticated file disagreeing on
// robustness was the finding. Loudness is preserved where it belongs: the
// skipped count is RETURNED and the CLI prints a WARN naming it, so tolerance
// can never quietly hide records.
```

### `go/internal/deliverable/secondaries.go:13` — above `func verifySecondaries(res *Result, c phasecontract.Contract, roots phasecontract.Roots) error {`

```text
// secondaries.go — ADR-0100: every AGENT-OWED declared output is verified,
// not only outputs.files[0].
//
// The registry has always declared a phase's full output list; FromSpec
// projected only the first entry into the contract, so handoff-build.json,
// handoff-scout.json, triage-decision.json and carryover-todos.json were
// waited for by the bridge, read by the router and by committedset, and
// judged by nobody. A phase could omit them and the cycle continued — the
// shape behind batch cycle 1623 (2026-09-11) and the empty triage decisions
// of 1630/1631 (2026-09-12).
//
// Deliberately narrow: existence, non-emptiness, and parseability for JSON /
// NDJSON. Markdown SHAPE stays the primary contract's business (Sections,
// Verdicts) — restating it here would be the duplicated belief the campaign
// forbids. Harness-produced secondaries (acs-verdict.json, written by
// acsrunner) are declared in the registry under harness_produced and never
// reach this function: re-dispatching an agent cannot make a harness write.
```

### `go/internal/deliverable/secondaries.go:85` — above `func (r *Reviewer) VerifiesDeclaredDeliverables() bool { return r.stage != config.StageOff }`

```text
// VerifiesDeclaredDeliverables marks the production contract gate as the
// ADR-0100 declared-deliverables gate for core's composition-root wiring
// proof (core.DeclaredDeliverablesGateWired). True whenever the gate is not
// switched off: at shadow/advisory it still verifies and logs would-block.
```

### `go/internal/deliverable/secondaries_test.go:3` — above `import (`

```text
// secondaries_test.go — ADR-0100: the contract gate verifies every AGENT-OWED
// declared output, not only outputs.files[0].
//
// Before this, FromSpec projected only Files[0] into the contract, so
// handoff-build.json, handoff-scout.json, triage-decision.json and
// carryover-todos.json were declared in the registry, waited for by the
// bridge's completion detector, and never judged by anyone: a phase could
// omit them and the cycle continued. The codes below are stable so the
// correction ladder's same-defect identity (contractBlocksShareIdentity)
// recognizes a repeat and escalates instead of re-dispatching blindly.
```

### `go/internal/deliverable/verdict_trailingdata_test.go:9` — above `func TestVerdictPresent_Enforce_ToleratesTrailingBraceInSentinel(t *testing.T) {`

```text
// Gate-level pin for the cycle-1478 halt shape: at contract-gate ENFORCE the
// prose fallback is off (ADR-0050 §3.10 Slice 5), so the sentinel parse is the
// only road to a verdict — a sentinel with a stray trailing brace inside the
// comment must therefore satisfy verdictPresent, or a one-byte slip becomes
// CodeBadVerdict -> three blocks (two correction re-dispatches, the second a
// salvage retry) -> circuit-open -> ADR-0072 halt (batch-20260815c,
// cycle-1478).
```

### `go/internal/deliverable/verifier.go:3` — above `import (`

```text
// verifier.go — the breaker-neutral core.ContractVerifier implementation
// (ADR-0045 I2 integrity rule). The correction ladder's intermediate rung
// re-checks (salvage's verify-after-move) run the SAME VerifyWith the gate
// runs, but never touch contract-gate-breaker.json — a multi-rung repair of
// one flaky deliverable must not count as three consecutive blocks and
// silently demote the gate batch-wide (cycle-265 forensics: two breakers,
// two scopes, do not conflate). Only the ladder's FINAL outcome goes through
// Reviewer.Review.
```

### `go/internal/deliverable/verifier.go:26` — above `phaseIO  config.Stage`

```text
// EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8); default StageOff → byte-identical to pre-3.8.
```

### `go/internal/deliverable/verifier.go:65` — above `Cycle: in.Cycle,`

```text
// Declared effects (ADR-0100 slice 2) are judged against this cycle's
// lifecycle state (processing/cycle-N/), so the gate names the cycle.
```

### `go/internal/deliverable/verifier_test.go:3` — above `import (`

```text
// verifier_test.go — ADR-0045 I2: the rung re-check is BREAKER-NEUTRAL
// (§8 TestLadder_RungRechecksAreBreakerNeutral). White-box (package
// deliverable) to drive the Reviewer's breakerPath override beside the
// Verifier on identical inputs.
```

### `go/internal/deliverable/verify_for_classification_test.go:17` — above `func TestVerifyForClassification_SalvagesPersistsAndReportsOnce(t *testing.T) {`

```text
// Cycle 1685 (2026-09-15): the runner's verdict engine verified with the plain
// verifier and classified the UNREPAIRED bytes ("no parseable verdict →
// FAIL") while this Reviewer later salvaged, persisted and approved the
// repaired file — two verifiers, one salvage, a red_count=0 cycle sealed FAIL.
// VerifyForClassification is the Reviewer's own verify+salvage offered to the
// engine: the bytes it returns are the bytes it persisted and will approve,
// and the salvage is reported exactly once.
```

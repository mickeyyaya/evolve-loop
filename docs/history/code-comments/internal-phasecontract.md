# Comment history: `internal/phasecontract`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phasecontract/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED. Every assertion
// drives a real producer/consumer (never a bare reference):
//
//	const FooterMarker         — the prefix RenderContractFooter emits; we render
//	                             a footer and assert it leads with the marker.
//	const SentinelSchemaVersion — the v1 payload version RenderVerdictSentinel
//	                             stamps; we round-trip a no-failure sentinel and
//	                             assert the parsed SchemaVersion equals it.
//	const TargetEvolveDir      — the WriteTarget that routes ArtifactPath to the
//	                             .evolve dir; asserted via the orchestrator
//	                             contract (its sole evolve_dir consumer).
//	func  SynthesizesContract  — invoked across llm / native-no-outputs /
//	                             native-with-outputs specs; the predicate's three
//	                             real branches.
//	type  CatalogResolver      — bound to the Resolver interface (satisfaction)
//	                             and exercised via Resolve over a spec lookup.
//	type  VerdictSentinel      — produced by RenderVerdictSentinelWithFailure and
//	                             materialized by ParseVerdictSentinelFull; we
//	                             assert every field of the parsed value.
//	vars  Audit/Build/Intent/Scout/Triage — the builtin phase Report values; each
//	                             is wired into its registry Contract.Sections, so
//	                             we assert For(phase).Sections == <Var>.Sections
//	                             (the registry is the real consumer) plus the
//	                             var's own load-bearing Phase field.
```

### `go/internal/phasecontract/apicover_named_test.go:86` — above `func TestSynthesizesContract_Branches(t *testing.T) {`

```text
// TestSynthesizesContract_Branches — exercises the predicate's three real
// outcomes: an llm phase synthesizes; a native/command phase with no declared
// outputs does NOT (the unsatisfiable-ship-contract guard, cycle-281); a native
// phase that declares outputs.files DOES.
```

### `go/internal/phasecontract/apicover_named_test.go:182` — above `func TestBuiltinReportVars_WiredIntoRegistry(t *testing.T) {`

```text
// TestBuiltinReportVars_WiredIntoRegistry — Audit/Build/Intent/Scout/Triage are
// the builtin phase Report values; the registry contract for each phase wires its
// Sections from the matching var (contract_registry.go: Sections: alwaysOn(Build.Sections),
// etc.). Asserting For(phase).Sections equals alwaysOn(var.Sections) proves the
// var is the live single source consumed by the registry — and pins each var's
// Phase field as load-bearing. The always-on registry set is the DECLARED set
// minus stagedSections (cycle-565 S1: HandoffSummary is declared on the Report
// var but its enforcement rolls out through the report-size gate, so it is
// deliberately absent from the always-on enforced set — see contract_registry.go
// stagedSections/alwaysOn).
```

### `go/internal/phasecontract/challenge_token.go:9` — above `func ChallengeToken(workspace string) (string, bool) {`

```text
// ChallengeToken reads <workspace>/challenge-token.txt — the per-cycle
// anti-gaming token the orchestrator mints at cycle start (core/cyclerun.go)
// and the bridge reuses-or-mints (bridge/driver_common.go, launch_modes.go) —
// trimmed; only a non-empty token counts. It sits beside
// Contract.RequireChallengeToken, the rule that makes a report echo it. It is
// the reader the phase runner's prompt preparation (the proof-of-read block)
// and the verdict engine's ACS floor (the echo check) share (ADR-0103 unit
// 11). deliverable.Verify's echo violation, bridge/completion.go's git-evidence
// detector, bridge/report.go's ArtifactRef and driver_common.go's
// read-or-mint still spell the read inline — report.go keeps the
// file-present-but-empty distinction this signature folds — so consolidating
// them is a separate edit (unit 11 follow-up F1b), not a claim this comment
// makes.
```

### `go/internal/phasecontract/challenge_token_test.go:9` — above `func TestChallengeToken_TrimsAndRejectsEmpty(t *testing.T) {`

```text
// TestChallengeToken_TrimsAndRejectsEmpty — ChallengeToken is the reader of
// <workspace>/challenge-token.txt the runner's prompt preparation and the
// verdict engine's ACS floor share (ADR-0103 unit 11, review fold F1): trimmed,
// and only a non-empty token counts. Moved verbatim from the verdict leaf's
// test 31a. Kills `TrimSpace dropped`, `empty token accepted`.
```

### `go/internal/phasecontract/contract.go:1` — above `package phasecontract`

```text
// Package phasecontract is the SINGLE SOURCE OF TRUTH for the report section
// headings each phase's verdict classifier requires.
//
// Both sides of the producer→consumer contract read from here:
//
//   - the CONSUMER: each phase's Go classifier (build/scout/tdd/audit/intent/
//     triage) tests an agent report against these headings to derive a verdict;
//   - the PRODUCER-SIDE ALARM: the contract test (contract_test.go) asserts the
//     agent .md template/reference still DECLARES each canonical heading, so a
//     template edit that renames a section fails CI instead of silently
//     false-FAILing a valid report at cycle time.
//
// This closes the failure class behind cycle-192: the classifiers grepped for
// headings the templates no longer emitted, valid reports were classified FAIL,
// and a build false-FAIL tripped the auditor's report-vs-telemetry cross-check
// → no ship. The 64b2d95 fix widened the per-phase regexes into tolerant
// allow-lists but left the heading strings duplicated in 6 Go files with no
// alarm; this package centralizes them and adds the alarm.
//
// Match semantics that the *declarative* phasespec.ClassifyRules cannot express
// (build's OR-of-headings, scout/triage's heading-plus-≥1-item, tdd's
// OR-within-AND, audit's verdict-token extraction) stay in each phase's
// classifier — they are STABLE logic that does not drift. Only the heading
// STRINGS, which DO drift against the templates, live here.
```

### `go/internal/phasecontract/contract.go:64` — above `func (r Report) Complete(content string) bool {`

```text
// Complete reports whether every always-on required section is present in
// content. Sections whose enforcement is staged behind a dedicated gate
// (stagedSections — HandoffSummary rolls out via the report-size gate, cycle-565
// S1) are skipped here, so the always-on completeness check every classifier
// runs stays byte-identical until that gate graduates. A Report with no always-on
// sections is trivially complete. Callers handle the empty-artifact case
// separately (it is a distinct FAIL reason).
```

### `go/internal/phasecontract/contract.go:83` — above `var HandoffSummary = Section{Canonical: "## Handoff Summary", Accepted: []string{"## Handoff Summary"}}`

```text
// HandoffSummary is the never-evict summary section (cycle-565 Slice S1 of
// report-size-contracts-jit-artifacts): a canonical region carrying the
// decisions, acceptance criteria, open questions, and verdicts a downstream
// phase must always see, so it can be separately size-budgeted (see
// deliverable.CheckHandoffBudget) while the rest of a report becomes evictable
// detail. Required on the build/scout/audit contracts only this slice — tdd/
// intent/triage stay untouched (S2/S3 territory). No legacy Accepted variants:
// it is a new heading, so the canonical string is the only accepted form.
```

### `go/internal/phasecontract/contract.go:93` — above `var Build = Report{`

```text
// The six built-in phase report contracts. Heading strings and producer files
// were verified against agents/*.md at v16.2.0 (see contract_test.go, which
// fails if a producer stops declaring a canonical heading).
```

### `go/internal/phasecontract/contract.go:134` — above `var ExplanationDocumentation = Section{Canonical: "## Explanation Documentation", Accepted: []string{"## Explanation Doc…`

```text
// ExplanationDocumentation is the audit report section the explanation-
// documentation contract (explanationdocs, contract v1) requires while it is
// active for the cycle. It is CONDITIONAL — listed under
// Contract.ExplanationSections, not Sections — so an audit in a cycle without
// the contract is not asked for it, and an audit that omits it while the
// contract is active is a deliverable-contract violation the correction
// ladder re-dispatches (cycles 1601/1603 were terminal FAILs instead).
```

### `go/internal/phasecontract/contract_from_spec.go:35` — above `RequireFailureContext: len(verdicts) > 0 && spec.Classify != nil && spec.Classify.RequireFailureContext,`

```text
// Opt-in, and only meaningful for verdict-emitting phases (ADR-0039 §7).
```

### `go/internal/phasecontract/contract_from_spec.go:37` — above `AgentOwedFiles: spec.Outputs.AgentOwed,`

```text
// ADR-0100: projected from the declaration only.
```

### `go/internal/phasecontract/contract_from_spec.go:43` — above `func overlayDeclared(c Contract, spec phasespec.PhaseSpec) Contract {`

```text
// overlayDeclared copies the registry-only fields (ADR-0100) from a spec onto
// a contract that came from the built-in table. Built-ins never declare owed
// secondaries or effects — the registry is their single source — so this is a
// pure projection, never a merge of two beliefs.
```

### `go/internal/phasecontract/contract_from_spec.go:53` — above `func SynthesizesContract(spec phasespec.PhaseSpec) bool {`

```text
// SynthesizesContract reports whether a spec yields a meaningful derived
// contract: only "llm"-kind phases (an agent actually writes the artifact) or
// specs that explicitly declare outputs.files. Native/command executors with
// no declared outputs get NO synthesized contract — inventing <name>-report.md
// for a deterministic executor produced the unsatisfiable ship contract that
// tripped the enforce gate 3× and forced a breaker demotion every shipping
// cycle (cycle-281). This predicate is the single home of the rule; Resolve
// and any projection (inventory, lint) must consult it before FromSpec.
```

### `go/internal/phasecontract/contract_from_spec.go:114` — above `func verdictsFromSpec(spec phasespec.PhaseSpec) []string {`

```text
// verdictsFromSpec returns the standard verdict vocabulary ONLY when the spec
// opts in: an Evaluate-archetype phase that declares classify.verdict_on_pass.
// Any other phase returns nil — never auto-attaching a verdict gate the agent
// does not actually emit (the cycle-192 false-FAIL class).
```

### `go/internal/phasecontract/contract_from_spec_test.go:169` — above `func TestFromSpec_RequireFailureContext(t *testing.T) {`

```text
// ADR-0039 §7: classify.require_failure_context opts a verdict-emitting user
// phase into the failure-signal contract; it is inert without a verdict (the
// gate must never invent a verdict requirement — the cycle-192 class).
```

### `go/internal/phasecontract/contract_registry.go:5` — above `type Kind int`

```text
// This file extends the phasecontract SSOT (see contract.go) from "report
// section headings" to a full per-protocol Contract: WHERE the deliverable is
// written, WHAT kind it is, and the well-formedness rules. It is consumed by
// the shared go/internal/deliverable package (the `evolve phase verify`
// self-check AND the host-side contract gate run the SAME checks against this
// registry), and by the bridge prompt-injection that tells each agent its exact
// output path. Design: ADR-0034.
```

### `go/internal/phasecontract/contract_registry.go:70` — above `Cycle int`

```text
// Cycle is the verifying cycle number. Declared EFFECTS (ADR-0100 slice 2)
// are judged against per-cycle lifecycle state — the inbox claim lives in
// <EvolveDir>/inbox/processing/cycle-<Cycle>/ — so every production
// verifier (gate, runner, self-check) carries it; 0 means unknown and a
// declared effect then fails OPEN with an error rather than deciding blind.
```

### `go/internal/phasecontract/contract_registry.go:108` — above `RequireFailureContext bool`

```text
// RequireFailureContext makes a FAIL/WARN verdict sentinel without a
// structured failure block a violation (ADR-0039 §7) — the correction
// loop then re-dispatches with the exact fix. Applies only to
// sentinel-declared verdicts: legacy prose-only artifacts stay legal
// forever. Set for built-ins that extract a verdict; user phases opt in
// via classify.require_failure_context.
```

### `go/internal/phasecontract/contract_registry.go:115` — above `RequireFailureContextPhaseIO bool`

```text
// RequireFailureContextPhaseIO is the PhaseIO-gated generalization of
// RequireFailureContext to phases that classify on section presence and emit
// no verdict today (build/scout/triage). The SAME FAIL/WARN-sentinel-without-
// failure-block check applies, but ONLY at EVOLVE_PHASE_IO>=enforce
// (ADR-0050 §3.8): off/shadow/advisory stay byte-identical, so a phase that
// has not yet adopted the structured sentinel cannot be false-blocked before
// the cutover. Distinct from RequireFailureContext (unconditional, audit) so
// audit's existing enforcement is untouched.
```

### `go/internal/phasecontract/contract_registry.go:124` — above `RequireChallengeToken bool`

```text
// RequireChallengeToken makes a report that fails to echo the minted
// <workspace>/challenge-token.txt token a violation (cycle-269: the
// proof-of-read protocol was audit-enforced only — unrecoverable — and
// the bash→Go migration had dropped the prompt-side injection entirely).
// The runner injects the token block at dispatch; the deliverable gate
// checks the echo so the correction loop re-dispatches with the exact
// fix BEFORE audit. Fail-open when no token was minted. scout (the
// minter) must never set this — echoing yourself is circular.
```

### `go/internal/phasecontract/contract_registry.go:142` — above `AgentOwedFiles []string`

```text
// AgentOwedFiles and Effects are projected from the registry's declaration
// ONLY (spec.Outputs.AgentOwed, spec.Effects) — never from a built-in
// literal — so the persona's instructions, the sandbox grant, and the
// declared-deliverables gate (ADR-0100) all read one word. A built-in
// contract receives them as an overlay in CatalogResolver.Resolve.
```

### `go/internal/phasecontract/contract_registry.go:275` — above `var stagedSections = map[string]bool{HandoffSummary.Canonical: true}`

```text
// stagedSections are declared contract sections whose ENFORCEMENT rolls out
// through a dedicated gate rather than the always-on contract gate — so the
// always-on gate stays byte-identical until that gate graduates (the same
// off→shadow→enforce staging every other gate in this repo uses). HandoffSummary
// (cycle-565 Slice S1) is declared on the build/scout/audit Report contracts —
// producers MUST document it (contract_test.go) and each report SHOULD carry it —
// but its presence/size is observed via the report-size gate
// (deliverable.VerifyWithReportSize), which defaults to shadow per the S1
// "shadow/warn first" spec. Keeping it out of the always-on enforced set means
// the new section cannot false-block a report before the report-size gate is
// deliberately promoted, and keeps it decoupled from unrelated contract checks
// (challenge-token, failure-context, circuit-breaker).
```

### `go/internal/phasecontract/contract_registry.go:335` — above `func ArtifactName(phase string) string {`

```text
// ArtifactName returns the deliverable filename a phase writes, resolved from
// the registry — the SSOT for artifact names — so no consumer has to re-declare
// the literal. Before cycle-1145 five packages (evalgate, topngate,
// phases/scout, router, cyclesimulator) each carried their own copy of
// "scout-report.md"; a rename in the registry silently left them behind.
//
// Returns "" when the phase is not registered, and "" for a NoArtifact phase
// (ship), whose result is a pushed commit rather than a file — callers that
// need a fallback filename should test for the empty string, exactly as
// core.backfillArtifactPath does.
```

### `go/internal/phasecontract/contract_registry.go:353` — above `func ArtifactFilename(phase string) string {`

```text
// ArtifactFilename returns the deliverable filename for a phase, falling back
// to the "<phase>-report.md" convention when the registry has no answer —
// either the phase is unregistered (user/inserted phases) or it is NoArtifact.
//
// This is the SSOT form of the fallback every call site was hand-rolling:
// core.backfillArtifactPath, core/routing_dispatch and three more sites each
// re-declared `phase + "-report.md"` beside their own [For] lookup, which is
// how the retro-phase path mismatch survived cycle-1145's backfill (the
// registry moved, the literals did not). Callers that must DISTINGUISH "no
// registered artifact" from "conventional name" keep using [ArtifactName],
// whose empty return carries that distinction.
```

### `go/internal/phasecontract/contract_registry_test.go:10` — above `func TestFor_CoversAllEightAgents(t *testing.T) {`

```text
// Layer 1 of the deliverable-contract feature (ADR-0034): the Contract registry
// is the single source of truth for WHERE each agent writes its deliverable,
// WHAT kind it is (markdown report vs JSON artifact), and the well-formedness
// rules. These RED tests pin the contract before the implementation exists.
```

### `go/internal/phasecontract/contract_test.go:56` — above `func TestProducersDeclareCanonical(t *testing.T) {`

```text
// TestProducersDeclareCanonical is the drift alarm: every phase contract's
// canonical heading must still be declared by the union of its producer agent
// templates. When a template author renames a section, this fails at CI instead
// of silently false-FAILing a valid report at cycle time (cycle-192). To fix a
// failure, update BOTH the producer template AND the Section.Canonical/Accepted
// in contract.go together — that is the single-source discipline this enforces.
```

### `go/internal/phasecontract/declared_outputs_projection_test.go:3` — above `import (`

```text
// declared_outputs_projection_test.go — ADR-0100 projection pins over the REAL
// registry. Reads docs/architecture/phase-registry.json, so run with -count=1.
```

### `go/internal/phasecontract/handoffsummary_test.go:5` — above `func TestHandoffSummarySection_Canonical(t *testing.T) {`

```text
// handoffsummary_test.go — RED contract for cycle-565 Slice S1 of
// report-size-contracts-jit-artifacts (this fleet lane's sole triage-committed
// top_n task; see triage-report.md). The contract-gate gains a canonical,
// never-evict "Handoff Summary" section (decisions, acceptance criteria, open
// questions, verdicts) for the report families the triage decision names —
// build, scout, audit — so its size can be separately budgeted
// (see reportsize_test.go in go/internal/deliverable) without disturbing the
// existing per-phase Sections the classifiers already require.
//
// RED today: phasecontract.HandoffSummary does not exist (compile failure).
```

### `go/internal/phasecontract/properties_test.go:11` — above `func TestProperty_RenderBlockListsEveryEnforcedRequirement(t *testing.T) {`

```text
// properties_test.go — cross-leg invariants swept over EVERY built-in
// contract (test-plan P0 #1/#2, 2026-06-12 contract-pipeline review).
// Property loops fail automatically when a new phase violates the
// invariant; hand-picked examples don't.
```

### `go/internal/phasecontract/properties_test.go:43` — above `if !strings.Contains(block, "evolve phase verify "+c.Phase) {`

```text
// Every contract-bearing phase is told to self-check (ADR-0034).
```

### `go/internal/phasecontract/render.go:11` — above `const FooterMarker = "DELIVERABLE PATH:"`

```text
// Rendering of the Deliverable Contract into the prompt (ADR-0034, Layer 2).
// Split into two pieces for prompt-cache safety AND instruction recency:
//
//   - RenderContractBlock: the INVARIANT instruction block. Identical across
//     cycles for a given phase, so it stays in the cacheable prompt prefix
//     (injected alongside the rules/policy blocks). Carries NO absolute path.
//   - RenderContractFooter: the VOLATILE one-line path declaration, appended as
//     the LAST line of the prompt. The per-cycle path therefore never pollutes
//     the cache prefix, and lands where recency bias makes the model most likely
//     to obey it.
//
// Why this fixes the bug: today the agent must infer its output path (read the
// workspace from cycle context, recall the filename from prose, join them). The
// footer states the exact absolute path; the block tells it to use exactly that
// path, emit the verdict sentinel, and self-check with `evolve phase verify`
// before finishing.
```

### `go/internal/phasecontract/render.go:40` — above `func RenderContractBlockStage(c Contract, includePhaseIO bool) string {`

```text
// RenderContractBlockStage renders the contract block, optionally adding the
// PhaseIO self-report-failure instruction (ADR-0050 §3.8b). includePhaseIO is
// set by the dispatch path when EVOLVE_PHASE_IO>=advisory; it instructs
// build/scout/triage — phases that emit no verdict by default — to self-report a
// FAIL/WARN via a sentinel carrying a structured failure block. A false value is
// byte-identical to the pre-3.8b block, so production (off) prompts never change.
```

### `go/internal/phasecontract/render.go:233` — above `func exemplarClass(phase string) string {`

```text
// exemplarClass is the failure class the prompt's exemplar shows: a word from
// the failurelog vocabulary, never a synthesized "code-<phase>-fail" — the
// gate (failure_class_unknown) would refuse the block's own example for any
// verdict phase whose synthesized name is not a class (architecture review of
// F19). The audit's rejection is code-audit-fail; every other phase's
// self-reported failure is the build class it interrupts.
```

### `go/internal/phasecontract/render_tail_test.go:73` — above `_, parsed := ParseVerdictSentinelFull(tail)`

```text
// Writer/detector no-drift, with the cycle-603 guard's polarity respected:
// the failure-bearing EXEMPLAR carries literal placeholder tokens, so the
// production detector must REJECT it — that is exactly what stops a
// prompt example captured from scrollback being read as a real verdict.
// The bare template (non-failure-context phases) must parse.
```

### `go/internal/phasecontract/render_test.go:8` — above `func TestRenderContractBlock_Markdown_NoPath(t *testing.T) {`

```text
// Layer 2 (ADR-0034): the Deliverable Contract is rendered into the prompt in
// two pieces — an INVARIANT instruction block (stable cache prefix, no path) and
// a VOLATILE path footer (last line; recency-optimal AND it keeps the per-cycle
// path out of the cacheable prefix).
```

### `go/internal/phasecontract/render_test.go:36` — above `func TestRenderContractBlockStage_BuildFailureInstructionGated(t *testing.T) {`

```text
// Phase 3.8b (ADR-0050): build/scout/triage emit no verdict today. When the
// PhaseIO rollout activates the instruction (includePhaseIOFailureContext=true,
// i.e. EVOLVE_PHASE_IO>=advisory), their contract block gains a self-report-
// failure instruction: on a self-reported FAIL/WARN, emit a sentinel carrying a
// structured failure block. When false (the default), the block is byte-identical
// to the pre-3.8b RenderContractBlock — production (off) prompts never change.
```

### `go/internal/phasecontract/render_test.go:137` — above `func TestRenderContractBlock_FailureContextInstruction(t *testing.T) {`

```text
// ADR-0039 §7: a RequireFailureContext contract teaches the failure-block
// emission in the SAME injected block that teaches the sentinel — one
// instruction surface for every persona/CLI, no per-agent prose copies.
```

### `go/internal/phasecontract/render_test.go:157` — above `func TestRenderContract_StatesTheAgentOwedFilesAndEffectsTheGateVerifies(t *testing.T) {`

```text
// ADR-0100 declared the agent-owed secondaries and the effects the gate
// verifies; the operator's rule is that the gate's pass criteria reach the
// agent as input. Before this test they lived only in persona prose — a
// registry change would have moved the gate without moving the prompt.
```

### `go/internal/phasecontract/render_test.go:207` — above `func TestRenderContractBlock_Audit_NamesTheFailureClassVocabulary(t *testing.T) {`

```text
// The failure class drives the retry envelope (an unknown class declines the
// repair round — cycle 1684), so the contract block names the vocabulary the
// gate accepts instead of leaving the auditor to invent one.
```

### `go/internal/phasecontract/required_ssot_test.go:5` — above `func TestRequiredRoles_DerivesFromRegistryAgentNames(t *testing.T) {`

```text
// required_ssot_test.go pins the "what counts as a complete cycle" SSOT that
// cyclehealth, redteamcheck and ledgerverify now read instead of each carrying
// their own `[]string{"scout", "builder", "auditor"}` literal (cycle-1140,
// phasecontract-role-artifact-ssot). The accessors must DERIVE from the
// registry — a hand-typed return here would recreate the very drift they exist
// to remove.
```

### `go/internal/phasecontract/required_ssot_test.go:103` — above `func TestArtifactName_ResolvesFromRegistryAndSkipsNoArtifact(t *testing.T) {`

```text
// TestArtifactName_ResolvesFromRegistryAndSkipsNoArtifact exercises the
// accessor the cycle-1145 backfill routed five packages through (evalgate,
// topngate, phases/scout, router, cyclesimulator). It must return the registry
// value — not a re-typed literal — for real deliverable phases, and the empty
// string for the two cases callers have to branch on: an unregistered phase and
// a NoArtifact phase ("ship", whose result is a pushed commit, not a file).
```

### `go/internal/phasecontract/resolver.go:44` — above `if r.lookup != nil {`

```text
// ADR-0100: a built-in contract still takes its declared owed
// secondaries and effects from the registry entry, looked up by the
// same canonical key the catalog uses (retro → retrospective).
```

### `go/internal/phasecontract/resolver_native_skip_test.go:3` — above `import (`

```text
// resolver_native_skip_test.go — RED contract for the cycle-281 ship-contract
// noise: ship is a pure NATIVE executor (no LLM agent writes markdown), yet the
// spec-derived fallback invented a `ship-report.md` contract for it, so every
// shipping cycle hit 3 enforce-stage [missing_artifact] BLOCKs and survived
// only because the circuit breaker demoted enforce→advisory. The operator
// policy this pins: audit-PASS ⇒ ship must complete WITHOUT depending on a
// safety valve.
//
// Rule (single home: SynthesizesContract): a derived contract exists only when
// an LLM agent actually writes the artifact (kind "llm", the default) OR the
// spec explicitly declares outputs.files. Native/command executors with no
// declared outputs resolve to NO contract — the gate skips them, exactly like
// any other phase the resolver misses.
```

### `go/internal/phasecontract/resolver_native_skip_test.go:34` — above `func TestResolveNativeExecutorWithoutOutputsHasNoContract(t *testing.T) {`

```text
// TestResolveNativeExecutorWithoutOutputsHasNoContract: the SYNTHESIS-skip rule
// (cycle-281). A native-kind spec with no outputs.files must NOT get a
// convention-invented `<name>-report.md` contract — a deterministic executor
// has no agent to write it, and the enforce gate would block a phase that can
// never satisfy it. Example uses a non-built-in native phase: ship — the
// original cycle-281 case — now resolves to its EXPLICIT built-in NoArtifact
// contract (TestFor_Ship_NoArtifactContract). That is a strict improvement over
// fail-open: it satisfies the same operator policy (audit-PASS ⇒ ship completes
// without depending on a safety valve) by affirmatively knowing ship has no
// file deliverable, rather than failing open on ambiguity.
```

### `go/internal/phasecontract/round_archive.go:10` — above `func RoundArchiveFilename(name string, round int) string {`

```text
// RoundArchiveFilename is the single home of the round-archive naming rule for
// a phase artifact that is regenerated across in-cycle repair rounds:
// `audit-report.md` retired after round 2 becomes `audit-report.round2.md`,
// `acs-verdict.json` becomes `acs-verdict.round2.json`. The writer
// (core.retireSupersededAuditArtifacts) and every reader of the archives (the
// dashboard's repair-round history, the repair-brief seed's "persisted from the
// previous round" set) derive the name here, so a rename cannot orphan one
// side — the cycle-1145 class the registry exists to prevent.
```

### `go/internal/phasecontract/round_archive.go:23` — above `func ParseRoundArchive(filename, liveName string) (round int, ok bool) {`

```text
// ParseRoundArchive is the inverse of RoundArchiveFilename: given a file name
// and the live artifact it may archive, it returns the round index. Readers
// list a workspace and parse rather than probing round 1, 2, 3 … in sequence,
// because the writer does not guarantee contiguous indices — an audit dispatch
// that died before writing its report archives nothing at its index while the
// dispatch counter still advances.
```

### `go/internal/phasecontract/sentinel.go:11` — above `const SentinelSchemaVersion = 1`

```text
// The machine-readable verdict sentinel (ADR-0034, Layer 5). Producers emit one
// line of the form:
//
//	<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->
//
// Classifiers parse the sentinel FIRST and fall back to legacy regex-on-prose
// (strangler fig). This removes the verdict-drift failure class — a deterministic
// machine token can't be mis-shaped by the prose heading the way "## Verdict"
// vs "**Verdict:**" did (cycle-148).
```

### `go/internal/phasecontract/sentinel.go:25` — above `const SentinelSchemaVersionFailure = 2`

```text
// SentinelSchemaVersionFailure is the version emitted when a failure block is
// present (ADR-0039 §7). Version 1 lines (no failure block) stay legal FOREVER
// — for PASS verdicts and for every artifact written before v2.
```

### `go/internal/phasecontract/sentinel.go:30` — above `type FailureBlock struct {`

```text
// FailureBlock is the structured failure context a FAIL/WARN verdict may carry
// (ADR-0039 §7): the producing agent's own classification, defect list, and
// machine-readable evidence pointers. Crash-class failures cannot self-report —
// the deterministic floor synthesizes those; this block is for phases healthy
// enough to describe their own failure.
```

### `go/internal/phasecontract/sentinel.go:39` — above `Prescription []string 'json:"prescription,omitempty"'`

```text
// Prescription carries a WARN's named remediation as structured content
// (F3, docs/operations/batch-integrity-review-2026-08-04.md:215-238):
// distinct from Defects because a prescription describes a fix for a
// foreseen risk, not something itself wrong. Kept as a separate field
// (not merged into Defects) so downstream consumers can tag entries
// distinguishably rather than losing that distinction.
```

### `go/internal/phasecontract/sentinel.go:60` — above `var placeholderRE = regexp.MustCompile('^\s*<[^<>]+>\s*$')`

```text
// placeholderRE matches a failure-block entry that is wholly an angle-bracket
// placeholder token (e.g. `<one line per defect>`, `<artifact path>`). Such a
// value is never genuine agent output — only the Deliverable Contract's own
// printed example echoed into captured scrollback (cycle-603). A real defect
// string is prose and never wholly bracketed, so this is a precise guard with
// no false positives against real failure blocks.
```

### `go/internal/phasecontract/sentinel.go:84` — above `func ParseVerdictSentinelFull(content string) (VerdictSentinel, bool) {`

```text
// ParseVerdictSentinelFull returns the complete sentinel payload and whether a
// well-formed one was found. v1 and v2 lines both parse (an absent failure
// block is legal); a missing/malformed/verdict-less sentinel yields ok=false
// so the caller falls back to its legacy parser (tolerant reader).
//
// Selection is TAIL-ANCHORED: candidates are walked from the END of the
// document and the LAST valid one wins. A producer emits its real verdict at
// the tail, while prose earlier in the same report routinely QUOTES the
// sentinel shape (contract examples, review commentary). First-match selection
// let those decoys win — or, when a decoy's JSON was elided, blanked the read
// entirely (cycle-1298: five quoted decoys buried a real verdict=FAIL and
// circuit-opened the contract gate). Invalid candidates are skipped, not fatal,
// so a placeholder echo trailing the real sentinel no longer destroys it.
// Single-sentinel documents — the overwhelmingly common case — are unchanged.
```

### `go/internal/phasecontract/sentinel.go:108` — above `func parseSentinelPayload(payload string) (VerdictSentinel, bool) {`

```text
// parseSentinelPayload validates ONE candidate payload. A candidate counts only
// when its LEADING complete JSON value decodes, carries a verdict, and is not a
// prompt-echoed contract example: a sentinel whose failure block still holds
// literal placeholder tokens can only be the Deliverable Contract's own printed
// example captured from scrollback, never a real agent verdict (cycle-603).
//
// Leading-value decode, not whole-string Unmarshal: sentinelRE's non-greedy
// capture ends at the first '}' directly followed by '-->' — for real payloads
// that is the closing brace of the outermost object plus any stray bytes before
// it — so an agent that emits one stray trailing brace hands us "valid JSON +
// '}'". Whole-string Unmarshal rejected that one byte, and at contract-gate
// enforce (prose fallback off, ADR-0050 §3.10) the miss blocked three times
// against unchanged bytes (two correction re-dispatches, the second a salvage
// retry), opened the gate circuit, and halted the batch as
// verdict-incoherence while prose, sentinel, and acs-verdict.json all agreed
// (cycle-1478). Tolerance is bounded by the comment capture itself; a leading
// value that is not a verdict-bearing object is still rejected. The guard CODE
// is unchanged, though the candidate SET widens: a comment whose payload was
// previously rejected for trailing bytes can now win tail-anchored selection —
// acceptable because the artifact author already controls the verdict outright.
```

### `go/internal/phasecontract/sentinel_prescription_test.go:5` — above `func TestSentinel_Prescription(t *testing.T) {`

```text
// sentinel_prescription_test.go — RED contract for cycle-1327's
// `audit-warn-prescription-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91): a WARN audit can prescribe a remediation in prose with no
// machine-readable trace, so the fix is never enforced and silently vanishes
// (cycle-1258 lesson). This file pins the wire-schema half: FailureBlock gains
// a `prescription` channel, distinct from `defects`, that round-trips through
// the sentinel exactly like every other field ADR-0039 §7 added.
```

### `go/internal/phasecontract/sentinel_tailanchor_test.go:11` — above `const tailAnchorFixture = "testdata/cycle1298-quoted-decoys.md"`

```text
// Cycle-1299 RED contract — tail-anchored verdict-sentinel selection.
//
// ParseVerdictSentinelFull used FindStringSubmatch, which returns the FIRST
// structural match anywhere in the document. A report whose PROSE quotes
// example `<!-- evolve-verdict: {...} -->` syntax therefore beat the real
// sentinel at the tail: a quoted decoy that merely unmarshals silently won,
// and a quoted decoy with elided/unparseable JSON blanked the whole read
// (unmarshal failure returns ok=false rather than trying the next candidate).
// Both shapes are live in the cycle-1298 adversarial-review report, which fired
// [bad_verdict] x3 and circuit-opened the contract gate enforce→advisory.
//
// The contract these tests pin: walk candidates from the END and return the
// LAST one that unmarshals cleanly, carries a non-empty Verdict, and is not a
// placeholder echo. Single-sentinel documents (the overwhelmingly common case)
// are unaffected — a one-element candidate list behaves exactly as before.
```

### `go/internal/phasecontract/sentinel_tailanchor_test.go:96` — above `func TestSentinelTailAnchor_LonePlaceholderEchoStillNotOK(t *testing.T) {`

```text
// Cycle-603 preservation — a document whose ONLY sentinel is a contract-example
// placeholder echo must still decline. Tail-anchoring must not weaken this.
```

### `go/internal/phasecontract/sentinel_tailanchor_test.go:133` — above `func TestSentinelTailAnchor_LiveCycle1298Fixture(t *testing.T) {`

```text
// AC4 — the LIVE cycle-1298 report (5 quoted decoys in prose + the real
// sentinel at the tail), parsed through the same function every gate caller
// uses. This is the wiring proof against a real captured artifact, not a
// synthetic string.
```

### `go/internal/phasecontract/sentinel_test.go:9` — above `func TestParseVerdictSentinel_Basic(t *testing.T) {`

```text
// Layer 5 (ADR-0034): a machine-readable verdict sentinel removes the verdict-
// drift class. Classifiers read the sentinel FIRST, then fall back to the legacy
// regex-on-prose (strangler fig — the old path stays as fallback).
```

### `go/internal/phasecontract/sentinel_test.go:43` — above `func TestParseVerdictSentinelFull_V2RoundTrip(t *testing.T) {`

```text
// --- schema_version 2: optional failure block (ADR-0039 §7) ---
```

### `go/internal/phasecontract/sentinel_test.go:115` — above `func TestParseVerdictSentinelFull_RejectsPlaceholderEcho(t *testing.T) {`

```text
// TestParseVerdictSentinelFull_RejectsPlaceholderEcho — cycle-603: a captured
// scrollback can contain the Deliverable Contract's own printed FAIL-example
// sentinel, still carrying literal placeholder tokens in its failure block
// (never genuine agent output). That must be rejected (ok=false) so it can
// never win verdict classification — even a scrollback-sourced parse.
```

### `go/internal/phasecontract/sentinel_trailingdata_test.go:9` — above `func TestParseVerdictSentinelFull_Cycle1478RealArtifact(t *testing.T) {`

```text
// Cycle-1478 (batch-20260815c): the audit agent emitted a sentinel whose JSON
// payload was valid for its first complete value, followed by ONE stray '}'.
// sentinelRE's non-greedy capture extends to the last '}' before '-->', so the
// capture carried the stray byte and the whole-string json.Unmarshal rejected
// it ("invalid character '}' after top-level value"). At contract-gate enforce
// the prose fallback is gated off (ADR-0050 §3.10 Slice 5), so a one-byte
// formatting slip blocked three times (two correction re-dispatches, the
// second a salvage retry), opened the contract-gate circuit, and ended in an
// ADR-0072 verdict-incoherence HALT — while prose, sentinel, and
// acs-verdict.json all agreed on WARN.
//
// The fix: parse the LEADING complete JSON value of the captured payload and
// tolerate trailing bytes INSIDE the comment-bounded capture. Every existing
// guard is unchanged: tail-anchored candidate selection, the placeholder-echo
// rejection, the verdict-vocabulary check, and rejection of captures whose
// leading value is not a verdict-bearing object.
```

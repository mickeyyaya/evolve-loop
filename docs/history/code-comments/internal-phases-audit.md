# Comment history: `internal/phases/audit`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/audit/apicover_named_test.go:15` — above `func TestNewDefaultWithStage_NamedPhase(t *testing.T) {`

```text
// TestNewDefaultWithStage_NamedPhase names the concrete audit.Phase type
// (New/NewDefault return *Phase but the type is never named in a test) and
// exercises NewDefaultWithStage — the composition-root seam that threads the
// EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050 §3.10 Slice 5).
```

### `go/internal/phases/audit/apicover_named_test.go:54` — above `func TestNewDefaultWithStageCompact_NamedPhase(t *testing.T) {`

```text
// TestNewDefaultWithStageCompact_NamedPhase names + exercises
// NewDefaultWithStageCompact — the compact-prompts seam added in cycle 413
// (workflow.compact_prompts) that shipped without an apicover naming test and
// reddened main CI (apicover -enforce: "UNCOVERED (no test names it)"). Beyond
// the name gate, it pins the constructor's reason to exist: compact=true threads
// prompts.StripOnDemandSections into the dispatch path so the on-demand reference
// tail never reaches the model, while compact=false leaves the body intact.
```

### `go/internal/phases/audit/apicover_named_test.go:72` — above `const body = "# Auditor body\n\n## Reference Index (Layer 3, on-demand)\n\n- tail-only reference content\n"`

```text
// Contract 2: compact=true strips the on-demand reference tail from the
// DISPATCHED prompt; compact=false leaves it. Driven end-to-end through the
// fake bridge, which captures the request the adapter would materialize. The
// production "## Reference Index (Layer 3, on-demand)" heading form is used so
// this also guards the cycle-413 prefix-match fix at the dispatch boundary.
```

### `go/internal/phases/audit/apicover_named_test.go:124` — above `func TestWithContractVerifier_NamedOptionReachesTheEngine(t *testing.T) {`

```text
// TestWithContractVerifier_NamedOptionReachesTheEngine names the F22 export:
// the Option stores the gate's verifier accessor on the Config, and a Phase
// built from it reports the wiring the same way Signals does — the engine
// classifies through the gate's own verify+salvage, not a second verifier.
```

### `go/internal/phases/audit/audit.go:1` — above `package audit`

```text
// Package audit implements the EGPS gate phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// audit-specific variation points.
//
// Audit is the EGPS gate: PASS requires BOTH a parseable PASS verdict
// in audit-report.md AND red_count == 0 in acs-verdict.json.
// policy.json workflow.strict_audit additionally promotes WARN to FAIL.
//
// Verdict mapping:
//   - empty artifact / no parseable verdict declaration → FAIL
//   - acs-verdict.json missing or unparseable → FAIL + error diag
//   - acs-verdict.json red_count > 0 → FAIL + EGPS diag
//   - WARN + workflow.strict_audit → FAIL
//   - otherwise → whatever verdict the audit-report.md declares (PASS/WARN/FAIL/SKIPPED)
//
// The verdict declaration is recognized in several agent-produced shapes —
// canonical "## Verdict\n**PASS**" AND single-line variants like
// "**Verdict: PASS**" or "Verdict: PASS". Prose formatting varies by CLI, so
// the gate must not hinge on one exact shape: a genuine PASS written as
// "**Verdict: PASS**" with red_count==0 must not be mis-graded FAIL (the
// cycle-148 silent-no-ship bug). When the verdict is unparseable but the EGPS
// suite is green, a loud diagnostic is emitted (never a silent FAIL).
//
// Default model is "opus" for adversarial cross-family diversity from
// the build phase's Sonnet.
```

### `go/internal/phases/audit/audit.go:80` — above `type hooks struct {`

```text
// The regex-on-prose verdict fallback (canonical "## Verdict\n**PASS**" and
// the colon-bearing inline forms) is single-homed in reportdoc.Verdict since
// ADR-0095: the dashboard's round history and the repair-brief seed read the
// SAME grammar, so what the gate scores and what the operator/rebuilder sees
// cannot disagree. reportdoc scans visible lines only (fenced, indented and
// HTML-commented content stripped), so an embedded template can no longer
// declare a verdict here either.
```

### `go/internal/phases/audit/audit.go:98` — above `gofmtCheck func(req core.PhaseRequest) ([]string, error)`

```text
// gofmtCheck reports the worktree's .go files that are not gofmt -s clean.
// It is the CI-parity gate that stops a cycle shipping a gofmt regression
// to main (cycles 339-341 shipped CI-red because the cycle-scoped audit
// never ran gofmt over the generated go/acs/cycle<N>/*.go files). nil = no
// gofmt gate (legacy/tests). The registry default wires gofmtCheckDefault.
```

### `go/internal/phases/audit/audit.go:107` — above `skillsDriftCheck func(req core.PhaseRequest) ([]string, error)`

```text
// skillsDriftCheck reports the worktree's SKILL.md files whose generated
// phase-facts region has drifted from its SSOTs (profiles/registry/
// phasecontract). A cycle that edits .evolve/profiles/*.json without
// regenerating would FAIL the CI TestSkills_NoDrift gate (cycle 339), so the
// drift must FAIL audit. nil = no skills gate. NewDefault wires
// skillsDriftCheckDefault (in-process skillcheck.Check — no subprocess).
```

### `go/internal/phases/audit/audit.go:139` — above `phaseIO config.Stage`

```text
// phaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
// §3.10 Slice 5). At >= StageEnforce the evolve-verdict sentinel is mandatory —
// the legacy prose/regex fallbacks are gated off. Zero value (StageOff) keeps
// every path active, byte-identical.
```

### `go/internal/phases/audit/audit.go:144` — above `ledger *defectledger.Ledger`

```text
// ledger is the unit-09 defect ledger (ADR-0103): New wires it with the
// root's Center; nil (a hooks{} literal) resolves to the Null Object
// through defectLedger() — same gate, no Center.
```

### `go/internal/phases/audit/audit.go:153` — above `func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {`

```text
// SecondaryArtifacts (runner.SecondaryArtifactsProvider): on a continuation
// cycle — the workspace carries the adopter-written continuation manifest —
// the audit contract also requires defect-dispositions.json (ADR-0074 /
// cycle-1285 lineage accounting). Declaring it holds session teardown until
// the auditor writes it, closing the write-one-artifact-and-die class that
// failed cycles 1397-1429. Non-continuation audits return nil: byte-identical
// legacy behavior.
```

### `go/internal/phases/audit/audit.go:179` — above `fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)`

```text
// Harness-owned (ADR-0098): the SAME acceptance words and predicate
// inventory the builder was handed — the grader reads what the builder
// read, so the block is an authority, not the builder's claim.
```

### `go/internal/phases/audit/audit.go:184` — above `b.WriteString(inheritedDefectsPromptBlockVia(h.defectLedger(), req))`

```text
// Continuations are TOLD their inherited OPEN defect ids (2026-08-10
// investigation: auditors were graded against ids they were never shown).
```

### `go/internal/phases/audit/audit.go:191` — above `func chainExamplePromptBlock() string {`

```text
// chainExamplePromptBlock SHOWS the auditor the reasoning-chain shape
// (ADR-0088) instead of only describing it.
//
// Measured, not assumed: the first shadow wave dispatched three audits with
// byte-identical prompts that all carried the persona's chain instruction —
// delivery worked, compaction stripped nothing — and one of the three emitted a
// chain. The persona describes the format in prose and cannot show it, because
// the combined line budget has five lines of headroom and the example is nine.
//
// Injecting it here costs no budget and closes the drift hole a review raised
// as a BLOCK: auditchain.ChainBlockExample is now the persona's illustration,
// the parser's own round-trip fixture, and the dispatched text — one constant,
// three legs (ADR-0084 I2). The parser is tail-anchored, so an auditor that
// echoes this block above its real one has the echo ignored.
```

### `go/internal/phases/audit/audit.go:237` — above `if stage < config.StageEnforce {`

```text
// ADR-0050 §3.10 Slice 5: the regex-on-prose fallbacks serve reports written
// against older templates; at enforce the sentinel above is mandatory, so gate
// them off (>= StageEnforce). Below enforce they stay active — byte-identical.
```

### `go/internal/phases/audit/audit.go:248` — above `func readACSVerdict(path string) (redCount int, redIDs, phantomBindings []string, shipEligible *bool, err error) {`

```text
// readACSVerdict reads the EGPS gate fields from acs-verdict.json. shipEligible
// is a *bool so the caller can distinguish "field absent" (nil — legacy verdicts
// written before ship_eligible existed) from an explicit false (do-not-ship). A
// read/parse error is returned so the missing/malformed-file FAIL floor holds.
// redIDs are the ac_ids of the red results (empty for legacy verdicts without a
// results array) — the diagnostic embeds them so the failure-digest fingerprint
// carries the DEFECT's identity: batch-12 (2026-07-27) halted on the
// identical-fingerprint breaker because three DIFFERENT red predicates all
// produced the byte-identical bare "red_count=1" reason (the cycle-1054/1060
// constant-message collision class, at the gate-block).
```

### `go/internal/phases/audit/audit.go:360` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/audit/audit.go:381` — above `CheckSolution func(req core.PhaseRequest) ([]string, error)`

```text
// CheckSolution, when set, reports a document cycle's solution-contract
// violations (internal/solutioncheck over solutions/<slug>/ for every bound
// task); any violation FAILs the audit — the same deterministic-gate shape
// as gofmt (ADR-0099 slice 2). nil = no gate. NewDefault wires
// solutionCheckDefault, which is silent for code cycles.
```

### `go/internal/phases/audit/audit.go:415` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
// §3.10 Slice 5). Zero value (StageOff) = byte-identical (prose fallbacks active).
```

### `go/internal/phases/audit/audit.go:422` — above `Signals func() *signalcenter.Center`

```text
// Signals is the accessor of the root's Signal Center the defect ledger
// (ADR-0103 unit 09) and the CI-parity gates (unit 14) report through, read
// at every use. nil = the Null Object: the registry root (evolve phase
// audit), New(Config{}) and the tests emit nothing; the loop root passes
// WithSignals.
```

### `go/internal/phases/audit/audit.go:433` — above `func WithContractVerifier(fn func() runner.ContractVerifier) Option {`

```text
// WithSignals installs the Signal Center accessor the defect ledger and the
// CI-parity gates report through (the loop root's Center; deliverable.WithSignals
// precedent).
// WithContractVerifier hands the audit runner the deliverables gate's
// verifier accessor (one verifier for gate and engine, research F22).
```

### `go/internal/phases/audit/audit.go:448` — above `signals func() *signalcenter.Center`

```text
// unit 14 (ADR-0103): the CI-parity gates' Center accessor
```

### `go/internal/phases/audit/audit.go:494` — above `func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {`

```text
// NewDefault builds the audit phase with production defaults — notably
// GenerateVerdict and host evidence capture wired together: every Audit runs
// the real suite and seals its complete result before the ledger binds it.
// BOTH the registry init() and the loop's runner map (go/cmd/evolve/cmd_cycle.go)
// MUST construct audit via this single seam so the generator can never again be
// wired in one phase-construction path but dormant in the other — the
// dual-source divergence that left the loop force-FAILing on a missing verdict
// every cycle (cycle-147). New(Config) stays for tests that pin explicit
// (nil or fake) generators.
```

### `go/internal/phases/audit/audit.go:507` — above `func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {`

```text
// NewDefaultWithStage is NewDefault plus the EVOLVE_PHASE_IO stage (ADR-0050 §3.10
// Slice 5). The composition root (cmd_cycle.go) passes cfg.PhaseIO so the audit
// verdict extraction enforces the sentinel at >= StageEnforce. NewDefault stays as
// the StageOff (byte-identical) convenience for the registry init() and tests.
```

### `go/internal/phases/audit/audit.go:522` — above `func NewDefaultWithStageCompactSpec(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool, spec *config.…`

```text
// NewDefaultWithStageCompactSpec is NewDefaultWithStageCompact plus the
// registry's document deliverable contract (ADR-0099 slice 2), which the
// composition root resolves ONCE and hands to both the build floor and this
// audit gate. nil ⇒ no document contract ⇒ no solution gate. The variadic
// options (WithSignals) let the composition root hand the Signal Center in
// without touching the other callers.
```

### `go/internal/phases/audit/audit.go:544` — above `ciParity{signals: cfg.Signals}.wire(&cfg)`

```text
// The CI-parity gates (ADR-0103 unit 14) are wired through the seam's
// per-Phase adapter so the loop root's Center reaches them — filling only
// the hooks no Option set, so an Option over Config is honoured in full;
// the five *Default facades stay Center-less for the by-name tests.
```

### `go/internal/phases/audit/audit.go:566` — above `hostActive, beltErr := explanationdocs.CrossCheckActivation(binding)`

```text
// The activation belt (single home: explanationdocs, shared with ship —
// architecture review 2026-09-01). Before this, a dropped/zero
// ContractVersion silently disabled the whole gate: Verify resolved
// inactive and audit returned nil. Now a zero version against an ACTIVE
// host activation, or a live version with no host activation, fails the
// audit loudly; a nil return below means the host AGREES nothing applies.
```

### `go/internal/phases/audit/audit.go:643` — above `v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})`

```text
// Probe quarantine runs in Classify before host execution, including
// configurations with an injected generator.
// Discover predicate FILES from the worktree (Root), but resolve `.evolve/`
// runtime data (history, baselines, current build-report) to the MAIN project
// root via EVOLVE_PROJECT_ROOT — those live in main, not the worktree, so a
// suite run from the worktree (issue #9 audit-cwd=worktree) would else false-RED
// every regression predicate that reads .evolve/ (issue #12, cycle-177).
```

### `go/internal/phases/audit/audit.go:717` — above `func recordChainShadow(artifact string, req core.PhaseRequest, narrative, shipped string, overrodeBy []string) {`

```text
// recordChainShadow writes the ADR-0088 chain-versus-narrative comparison into
// the phase workspace. Best-effort and silent on failure by design: a shadow
// measurement must never influence, delay, or brick the decision it is
// measuring — the same posture the bad_verdict instrumentation ships with.
//
// The evidence set is what is actually ON DISK in the workspace, not what the
// prompt claimed: the question the record answers is whether the judge COULD
// have walked the chain, and a file the dispatch mentioned but never produced
// would make that answer a lie.
```

### `go/internal/phases/audit/audit_ciparity_test.go:69` — above `func TestNewDefault_WiresCIParityGates(t *testing.T) {`

```text
// NewDefault must wire the REAL CI-parity gates (cycle-147 lesson: a seam wired
// in one construction path but dormant in the other is the bug). Behavioral: a
// worktree whose go/ module has a real `go vet` defect (Printf verb/arg
// mismatch), EGPS green pre-staged, so the only possible FAIL is the real go-vet
// gate NewDefault wires.
```

### `go/internal/phases/audit/audit_egps_red_identity_test.go:13` — above `func writeACSVerdictReds(t *testing.T, ws string, redIDs ...string) {`

```text
// audit_egps_red_identity_test.go — regression lock for the 2026-07-27
// batch-12 false breaker trip: the EGPS gate-block diagnostic said only
// "EGPS: red_count=1 (cycle ships only when red_count==0)" with NO predicate
// identity, so three DIFFERENT red predicates (cycles 1107/1115/1116 — three
// distinct whole-suite meta-predicates flaking under width-2 contention)
// produced byte-identical audit-fail reasons → one failure fingerprint
// (audit|gate-block|048c5b1ca3fb) → the identical-fingerprint pipeline
// breaker halted the batch on what were three distinct honest failures.
// Same class as the cycle-1054/1060 verdict-path collision: a constant
// failure message blinds the breaker's identity premise.
//
// Contract: the EGPS red_count diagnostic embeds the red predicates' ac_ids
// (capped — the message must stay one line), so distinct red predicates yield
// distinct fingerprints while a genuine recurrence (same predicate red again)
// still collides exactly.
```

### `go/internal/phases/audit/audit_egps_red_identity_test.go:97` — above `a1 := egpsRedDiagnostic(t, "cycle841/TestC841_Amplify_CLIOutput_Memo_ResolvesToClaudeTmux")`

```text
// The TWO-PART live convention (no index group): TestC<cycle>_<Name> —
// real ids from .evolve/runs/cycle-841 and cycle-1000 acs-verdicts. The
// first normalizer required C\d+_\d+_ and left "C841_" (a cycle number)
// embedded — adversarial-review catch.
```

### `go/internal/phases/audit/audit_egps_red_identity_test.go:112` — above `n1 := egpsRedDiagnostic(t, "cycle416/TestC416_NEG_MarkerlessBody_CompactionIsNoOp")`

```text
// NEG-prefixed sibling shape (cycle-416 era) keeps its semantic tail.
```

### `go/internal/phases/audit/audit_gofmt_test.go:23` — above `func TestRun_GofmtDirty_FAILsAudit(t *testing.T) {`

```text
// A cycle whose worktree has a gofmt-dirty Go file must FAIL audit — even when
// the EGPS suite is green and the report declares PASS. This is the gate that
// would have caught cycles 339-341's "ships green locally, red in CI gofmt"
// class (the generated go/acs/cycle<N>/*.go predicate files).
```

### `go/internal/phases/audit/audit_gofmt_test.go:87` — above `func TestNewDefault_WiresGofmtCheck(t *testing.T) {`

```text
// NewDefault must wire the REAL gofmt check (parity with the cycle-147 lesson:
// a seam wired in one construction path but dormant in the other is the bug).
// Behavioral: a worktree with a gofmt-dirty go/ file, EGPS green pre-staged, so
// the only possible FAIL cause is the real gofmt gate NewDefault wires.
```

### `go/internal/phases/audit/audit_integration_test.go:41` — above `func TestNewDefault_WiresVerdictGenerator(t *testing.T) {`

```text
// TestNewDefault_WiresVerdictGenerator pins the cycle-147 fix: the audit phase
// constructed via NewDefault (the single seam now used by BOTH the registry
// init and the loop's runner map in cmd_cycle.go) must wire the REAL
// generateACSVerdict, so a missing acs-verdict.json is auto-generated host-side
// from the on-disk predicate suite — not force-FAILed. This exercises the real
// generateACSVerdict+acssuite path (unlike the fake-generator TestRun_Missing*
// tests) and would have failed against the pre-fix cmd_cycle.go wiring, which
// left GenerateVerdict nil.
```

### `go/internal/phases/audit/audit_integration_test.go:82` — above `vb, readErr := os.ReadFile(filepath.Join(ws, "acs-verdict.json"))`

```text
// The verdict file must now exist at the canonical path (it did NOT before
// the wiring fix — that was the cycle-147 forced-FAIL).
```

### `go/internal/phases/audit/audit_phaseio_test.go:10` — above `const auditSentinelPASS = "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->"`

```text
// ADR-0050 §3.10 Slice 5: at enforce the machine-readable evolve-verdict sentinel
// is mandatory — the legacy prose/regex verdict fallbacks are gated off. Below
// enforce (off/shadow/advisory) every path stays active, byte-identical.
```

### `go/internal/phases/audit/audit_report_length_test.go:13` — above `const sizeMarker = "audit-report.md size"`

```text
// audit_report_length_test.go — RED contract for the cycle-1522 fleet-scoped
// task `cap-audit-report-length` (scout-report.md ## Selected Tasks, Task 1).
//
// The defect: audit-report.md has no upper bound on total size. The ## Issues
// table grows one row per finding with no cap, the report is re-read in full at
// ship time (go/internal/phases/ship/audit.go:83, which also SHA-binds it), and
// the next cycle's handoff carries prior audit context — so an oversized report
// compounds token cost on every downstream read. defect_ledger.go:56-61 already
// bounds the *ledger* (defectLedgerMaxEntries / defectTextMaxRunes) with the
// exact idiom this task extends to the *report*: overflow is RECORDED, never
// silently dropped.
//
// The contract pinned here, in three load-bearing parts:
//
//  1. A package const `auditReportMaxBytes` exists and carries a sane budget
//     (this file references it directly, so RED is a compile failure until
//     Builder declares it).
//  2. Classify emits EXACTLY ONE warning-severity diagnostic when the artifact
//     exceeds the cap, naming both the actual size and the cap.
//  3. The check is DIAGNOSTIC-ONLY. It must never flip the verdict (an
//     oversized but green report still classifies exactly as its verdict says)
//     and must never mutate the on-disk artifact — ship SHA-binds those bytes
//     (ship/audit.go:83), so a truncating cap would break the ship-time
//     integrity check.
//
// Severity is WIRING, not taste: cyclestate.ErrorMessages keys off
// Severity=="error" to build AuditFailReasons, so an error-severity size
// diagnostic would convert a merely-verbose report into a dossier-visible
// failure. The size warning must be Severity=="warning".
```

### `go/internal/phases/audit/audit_skillsdrift_test.go:15` — above `func TestRun_SkillsDrift_FAILsAudit(t *testing.T) {`

```text
// A cycle whose worktree drifted a SKILL.md (e.g. edited .evolve/profiles/*.json
// without regenerating the phase-facts region) must FAIL audit — the gate that
// would have caught cycle 339's SKILL.md drift before it shipped CI-red on
// TestSkills_NoDrift.
```

### `go/internal/phases/audit/audit_solution_gate_test.go:13` — above `func TestRun_SolutionContractViolation_FAILsAudit(t *testing.T) {`

```text
// A document cycle whose solutions/<slug>/ fails the deterministic contract
// (internal/solutioncheck) must FAIL audit even when the narrative says PASS
// and EGPS is green — the same single-exit gate shape as gofmt (ADR-0099 slice 2).
```

### `go/internal/phases/audit/audit_test.go:475` — above `proj := t.TempDir()`

```text
// Strict mode is now sourced from .evolve/policy.json (workflow.strict_audit),
// not an env dial — replaces the EVOLVE_STRICT_AUDIT read (flag-reduction, ADR-0064).
```

### `go/internal/phases/audit/audit_test.go:597` — above `func TestRun_MissingACSVerdict_GeneratedThenPASS(t *testing.T) {`

```text
// cycle-138/139 fix: when acs-verdict.json is ABSENT, the audit phase
// generates it (via the injected GenerateVerdict seam → acssuite in prod)
// before reading red_count, so a clean autonomous cycle reaches PASS→ship
// instead of being forced to FAIL on the missing file. The generator
// stand-in here writes a red_count==0 verdict, mimicking a green suite.
```

### `go/internal/phases/audit/audit_test.go:785` — above `func TestExtractAuditVerdict_Formats(t *testing.T) {`

```text
// --- verdict-format robustness (cycle-148 mis-grade fix) ---
```

### `go/internal/phases/audit/audit_test.go:823` — above `func TestRun_InlineVerdictFormat_PASS(t *testing.T) {`

```text
// Regression for cycle-148: a genuine PASS written inline as "**Verdict: PASS**"
// with red_count==0 must grade PASS and route to ship — not be mis-graded FAIL.
```

### `go/internal/phases/audit/audit_test.go:871` — above `func TestValidateExplanationReview_ReadsTheSectionAsAuditorsWriteIt(t *testing.T) {`

```text
// TestValidateExplanationReview_ReadsTheSectionAsAuditorsWriteIt — the
// FORMAT tolerance that cycles 1604/1605/1606 needed (several Evidence lines,
// a line range, citations under another field name, backticked values) while
// the substance rule (every reference cited at a line) is unchanged.
```

### `go/internal/phases/audit/audit_test.go:940` — above `func TestClassify_PathOnlyCitationsKeepThePassVerdictAndRecordTheAdvisory(t *testing.T) {`

```text
// The cycle-1638/1640 shape (2026-09-13): the auditor's narrative is PASS and
// its review reasons about the document, but its Evidence cites the material
// paths without a literal path:line. ADR-0102: the reasoning is the gate, the
// citation form is advisory — the verdict stands and the advisory rides the
// record so the shape can still be improved without burning the cycle.
```

### `go/internal/phases/audit/audit_verdict_conflict_egps_facts_test.go:10` — above `func errorMessages(diags []core.Diagnostic) []string {`

```text
// audit_verdict_conflict_egps_facts_test.go — the cycle-1130 increment for the
// inbox item `verdict-coherence-auditor-vs-egps`.
//
// The sibling suites (audit_verdict_conflict_test.go, ..._gates_test.go,
// ..._narrative_test.go) already pin that a `verdict-conflict:` record EXISTS,
// carries the narrative verdict verbatim, is distinguishable per gate reason,
// and stays silent on every coherent case. What none of them pin is the fact
// the scout report's verifiableBy actually asks for: that ONE Classify call
// hands the operator BOTH halves of the forensic pair —
//
//	(a) the auditor's own declared verdict, and
//	(b) the gate's red identity facts (red_count and the normalized red_ids),
//
// both at Severity=="error", so both ride cyclestate.ErrorMessages →
// AuditFailReasons → <phase>-fail-reason.json → the dossier's SubstantiveError.
//
// Scope note (the one ambiguity in the AC, resolved deliberately): the AC reads
// "the returned []core.Diagnostic contains a message matching both PASS and the
// red_count/red_ids facts". Read strictly that demands a SINGLE message holding
// both; the shipped implementation instead splits them across two
// error-severity diagnostics returned from the same call ("Gate detail is in
// the error diagnostics beside this one"). These tests pin the SLICE-level
// reading, because the operator-visible outcome the item was filed for — a
// dossier that shows the disagreement next to the evidence — is satisfied
// either way, and both diagnostics travel the same error-severity chain as one
// unit. A future refactor that keeps the conflict record but drops the gate
// facts (or demotes either to warning) breaks the pair and fails here, which is
// the regression this file exists to catch.
```

### `go/internal/phases/audit/audit_verdict_conflict_egps_facts_test.go:62` — above `func TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether(t *testing.T) {`

```text
// TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether — AC-1. A
// narrative PASS over a red predicate suite must leave the operator holding the
// pair: what the auditor said, and which predicate the gate actually tripped
// on. Either half alone is what cycles 1107/1116/1117 already had, and it was
// not enough to tell a genuine defect from a poisoned predicate.
```

### `go/internal/phases/audit/audit_verdict_conflict_egps_facts_test.go:75` — above `for _, want := range []string{`

```text
// The identity token is the CYCLE-NORMALIZED tail, not the raw ac_id:
// egpsRedIDCycleTokens (audit.go) strips the cycle group and index on
// purpose, so three occurrences of the same predicate across cycles collide
// into one failure fingerprint instead of minting a fresh one per retry
// (bc2e3236, the batch-12 breaker false-trip). Asserting the raw
// "cycle1130/TestC1130_007_ProbeIsolation" here would pin the exact behavior
// that commit removed on purpose; what the AC actually needs is that the
// operator can tell WHICH predicate tripped, and the normalized tail carries
// that.
```

### `go/internal/phases/audit/audit_verdict_conflict_gates_test.go:11` — above `func offenders(names ...string) func(core.PhaseRequest) ([]string, error) {`

```text
// audit_verdict_conflict_gates_test.go — RED contract for the cycle-1127
// continuation of `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, 4th recurrence of the
// cycle-87 / cycle-352 / cycle-456 family).
//
// What is ALREADY done (cycle-1124 salvage, HEAD 33596bb0): the three EGPS
// override branches (acs-verdict.json unreadable, red_count>0,
// ship_eligible=false) record the disagreement — see
// audit_verdict_conflict_test.go, all GREEN at HEAD.
//
// What is STILL OPEN and is what this file pins: AC-1 names FIVE more gates
// that force `verdict = core.VerdictFAIL` in hooks.Classify —
//
//	audit.go  gofmt gate                       (h.gofmtCheck)
//	audit.go  skills-drift gate                (h.skillsDriftCheck)
//	audit.go  applyCIGate x5                   (goVet, acsDurable,
//	                                            integrationTier,
//	                                            apicoverEnforce,
//	                                            apicoverNewPkgGraduation)
//
// — and NONE of them records the auditor's narrative verdict before clobbering
// it. The operator-facing consequence is identical to the EGPS case the salvage
// already closed: a FAIL dossier whose SubstantiveError says only "gofmt: 3
// file(s) are not gofmt -s clean" cannot be told apart from one where the
// auditor itself independently found the cycle broken. Half a fix is a fix that
// still loses the signal on 5 of 8 gates.
//
// Contract pinned here (an extension of the salvaged contract, NOT a rewrite —
// every existing TestVerdictConflict_* case must stay green):
//
//  1. EVERY gate that forces FAIL over a found, non-FAIL narrative emits the
//     error-severity `verdict-conflict:` record naming that narrative verdict.
//  2. Exactly ONE record per Classify call, no matter how many gates fired —
//     the record is a statement about the call, not about each gate. This is
//     what makes a post-gate single-exit emission the natural implementation.
//  3. The fail-OPEN paths stay silent: a gate that could not RUN emits its
//     existing warning and does not force FAIL, so there is no conflict.
//  4. AC-4: the returned verdict is byte-identical to today's behaviour in
//     every case. The record is additive; it never softens a gate.
//
// Out of scope, deliberately unpinned: the policy.json workflow.strict_audit
// WARN→FAIL promotion. AC-1 does not name it, and it is a policy decision on a
// narrative the auditor already declined to pass, not a mechanical gate
// disagreeing with a clean read. Whether the implementation happens to cover it
// is left free; no test here asserts either way.
```

### `go/internal/phases/audit/audit_verdict_conflict_narrative_test.go:10` — above `func sentinelReport(verdict string) string {`

```text
// audit_verdict_conflict_narrative_test.go — regression contract for the
// cycle-1124 audit finding C1 (blocking): the conflict record interpolated the
// auditor's narrative verdict with NO enum check.
//
// `narrative` originates in extractAuditVerdict → phasecontract.ParseVerdictSentinel,
// and ParseVerdictSentinelFull rejects only the empty string — it never
// constrains the value to PASS/WARN/FAIL/SKIPPED. audit-report.md is
// LLM-authored content in an agent-writable workspace, so an arbitrary string
// (including one carrying newlines) could reach an ERROR-severity diagnostic —
// which is exactly the diagnostic cyclestate.ErrorMessages lifts into
// CycleState.AuditFailReasons → <phase>-fail-reason.json → the failure
// dossier's FailReasons → the sha256 fingerprint (failure_digest.go) → the
// identical-fingerprint blocker breaker (blocker_breaker.go).
//
// Two consequences the tests below pin:
//
//	C1a — a per-attempt-varying narrative ("PASS (2 caveats)", routine LLM
//	      output) yields a different fingerprint every retry for the SAME
//	      defect, so the runaway-loop halt never fires. This is the very
//	      invariant egpsRedIDCycleTokens strips cycle tokens to protect.
//	C1b — a "\n"-bearing sentinel verdict renders as MULTIPLE reason lines in
//	      the operator-facing dossier and in retro/failure-adapter prompts, so
//	      one FailReasons entry can forge a second, authoritative-looking line.
//
// The regex path is NOT the interesting one (it can only match a canonical
// verdict). Every case here therefore probes the SENTINEL path, where
// verdictFound==true for a value that was never a verdict.
```

### `go/internal/phases/audit/audit_verdict_conflict_narrative_test.go:51` — above `junk := []struct{ name, narrative string }{`

```text
// Each case carries a SHORT name: t.Run's name feeds t.TempDir()'s
// directory component, and the 40xPASS narrative used as its own subtest
// name overflowed the 255-byte filename limit on CI's Go 1.23
// ("mkdir: file name too long" — main RED 2026-07-27). Newer local
// toolchains truncate TempDir names, so the per-cycle gate never saw it.
```

### `go/internal/phases/audit/audit_verdict_conflict_narrative_test.go:97` — above `func TestVerdictConflict_RecordVariesOnlyInTheNarrativeToken(t *testing.T) {`

```text
// TestVerdictConflict_RecordVariesOnlyInTheNarrativeToken — C1a stated as the
// property the blocker breaker actually needs, over the ACCEPTED alphabet.
//
// The previous version of this test compared sentinelReport("PASS-r1") against
// ("PASS-r2"); both are REJECTED by the core.IsVerdict guard it meant to
// exercise, so both sides were the empty set and it passed for the wrong reason
// (cycle-1127 audit finding C2 — a green assertion that probed nothing).
//
// The real risk is the three values that ARE accepted. They must reach the
// operator verbatim (that is the whole point of the record), so the records
// cannot be byte-identical; what must hold is that `narrative=<verdict>` is the
// ONE token they differ in. That is precisely the contract
// core.normalizeReasonForFingerprint relies on to fold three attempts at one
// defect back into one fingerprint (pinned end-to-end by
// TestVerdictConflict_FingerprintIsStableAcrossTheNarrativeAlphabet in
// internal/core). A second varying token added here — a timestamp, a cycle
// number, a retry counter — would silently re-open C1, and fails here.
```

### `go/internal/phases/audit/audit_verdict_conflict_test.go:13` — above `const conflictMarker = "verdict-conflict"`

```text
// audit_verdict_conflict_test.go — RED contract for the cycle-1124 inbox item
// `verdict-coherence-auditor-vs-egps` (weight 0.92, 4th recorded instance of the
// family: cycle-87 / cycle-352 / cycle-456).
//
// The defect: hooks.Classify extracts the auditor's OWN narrative verdict, then
// unconditionally overwrites it with core.VerdictFAIL at each of three EGPS
// gate branches (acs-verdict.json unreadable, red_count>0, ship_eligible=false)
// WITHOUT ever recording what the narrative said. The override is correct — the
// deterministic gate must outrank prose (cycles 339-341) — but the DISAGREEMENT
// is silently discarded. Downstream (cyclestate.ErrorMessages → AuditFailReasons →
// <phase>-fail-reason.json → failure dossier SubstantiveError) therefore only
// ever sees the gate's own message, so an operator reading a dossier cannot
// distinguish a genuine defect from a POISONED predicate the auditor itself
// flagged as clean. The connected `audit-probe-tree-isolation` item is the live
// case: cycles 1116 (auditor PASS) / 1107 (WARN) / 1117 ("Not FAIL") were all
// EGPS-forced FAIL on predicates later proven poisoned by the auditor's own
// untracked probe tests — three conflicts that left no record anywhere.
//
// Contract pinned here:
//  1. When the narrative verdict was FOUND and is NOT FAIL, each of the three
//     override branches emits an ERROR-severity `verdict-conflict:` diagnostic
//     naming the narrative verdict and the gate reason.
//  2. Error severity is the WIRING: cyclestate.ErrorMessages (cyclestate/result.go (ErrorMessages))
//     keys off Severity=="error", so an error-severity diagnostic reaches
//     AuditFailReasons/the dossier with zero new plumbing. A warning-severity
//     conflict record would be silently dropped by that same function.
//  3. No noise on the COHERENT case: narrative already FAIL, narrative
//     unparseable, or the gate green ⇒ no conflict diagnostic at all.
//  4. The override itself is untouched: every conflicting case still returns
//     core.VerdictFAIL. The record is additive, never a softening of the gate.
//
// Structural constraint (why the fix lives here and not in the auditor's
// prompt): acs-verdict.json is written AFTER audit-report.md (measured 1115:
// 00:15:09 vs 00:13:56; 1117: 01:38:45 vs 01:37:36), so the auditor cannot
// reconcile against a file that does not yet exist. Classify runs after both.
```

### `go/internal/phases/audit/audit_verdict_conflict_test.go:154` — above `func TestVerdictConflict_BranchesAreDistinguishable(t *testing.T) {`

```text
// TestVerdictConflict_BranchesAreDistinguishable — the failure-fingerprint
// lesson (audit_egps_red_identity_test.go, batch-12 breaker false-trip): a
// constant conflict message blinds the identical-fingerprint breaker's identity
// premise. Distinct gate reasons must yield distinct conflict messages.
```

### `go/internal/phases/audit/bookkeeping_reason_singlesource_test.go:3` — above `import (`

```text
// bookkeeping_reason_singlesource_test.go — producer↔classifier binding for
// the bookkeeping-regrade micro-cycle (ADR-0084 I2 spirit: the reader and the
// writer of a machine-graded string must be pinned against each other).
//
// core.BookkeepingRegradeEligible classifies CycleState.AuditFailReasons by
// prefix. The reasons are minted HERE (defect_ledger.go, closure_claim.go,
// audit.go's verdict-conflict record). This test feeds REAL minted
// diagnostics through the core matchers, so a prefix drift on either side —
// a reworded "defect ledger:" mint, a reanchored matcher — reds it instead
// of silently disarming the regrade (the class that made the eval
// quality-gate vacuous, #426).
```

### `go/internal/phases/audit/chain_example_prompt_test.go:3` — above `import (`

```text
// chain_example_prompt_test.go — the auditor must be SHOWN the shape, not only
// told about it.
//
// MEASURED, not assumed. The first shadow wave dispatched three audits whose
// prompts were byte-identical 337-line files, each carrying the chain
// instruction — so delivery worked and compaction stripped nothing (the failure
// that cost 40 cycles in August). Yet only ONE of the three emitted a chain.
// The gap is compliance, and the obvious cause is that the instruction
// describes a format in prose while never showing it: the literal
// `ChainBlockExample` lives in Go and reached no prompt, because the persona
// line budget (<751 combined) has 5 lines of headroom and the example is 9.
//
// Injecting it at dispatch costs no budget AND closes the drift hole the
// earlier review raised as a BLOCK: one constant is now the persona's example,
// the parser's fixture, and the dispatched text — three legs, one source
// (ADR-0084 I2).
```

### `go/internal/phases/audit/chain_shadow_test.go:3` — above `import (`

```text
// chain_shadow_test.go — the audit phase consuming the reasoning chain, in
// SHADOW (ADR-0088 rollout).
//
// Shadow means: the chain is parsed, concluded against the evidence the phase
// was actually given, and RECORDED beside the cycle — and the phase's verdict
// is byte-identical to what it would have been without any of it. That is the
// whole point of the stage: a wave produces the comparison data that says
// whether the chain agrees with the narrative verdict, and where it does not,
// WHICH LINK the narrative was silent about. Enforcing before that data exists
// would be the same mistake as every gate this repo has had to walk back.
```

### `go/internal/phases/audit/changedpkgs_git_test.go:10` — above `func gitInAudit(t *testing.T, dir string, args ...string) {`

```text
// changedpkgs_git_test.go — RED contract for cycle-573 Task 2, the integration
// half. changedPackagesForAudit is the audit phase's changed-package locator; it
// gates apicover. Today it reads an extinct handoff-build.json and returns nil
// (fail-open) when absent, so the apicover gate never fires on a real cycle.
// After the fix it derives the set from git (changedpkgs.FromGit), so a cycle
// that changed a package is detected even with NO handoff file present.
//
// RED today: with no handoff file, changedPackagesForAudit returns nil, so this
// assertion (non-empty, includes the changed package) fails. GREEN once the
// locator is git-derived.
```

### `go/internal/phases/audit/changedpkgs_git_test.go:54` — above `writeAuditFile(t, root, "go/internal/foo/foo.go", "package foo\n\nfunc New() {}\n")`

```text
// The cycle's change: a new package, uncommitted, and deliberately NO
// handoff-build.json / handoff-builder.json in .evolve/runs/cycle-573.
```

### `go/internal/phases/audit/changelog_closure_test.go:10` — above `func closureReport(body string) string {`

```text
// changelog_closure_test.go — RED contract for cycle-1285 Task 2
// (`changelog-closure-cite-gate`; inbox item `continuation-defect-ledger`
// clause (3), batch-integrity-review-2026-08-04.md:123).
//
// The defect this pins: the 1255 → 1268 → 1270 → 1272 chain closed a named
// CRITICAL by ASSERTION. A bookkeeping line reading "verified closed" was
// enough — nothing anywhere required that claim to point at the per-defect
// disposition record that would let a reader check it. defect_ledger.go now
// mints that record (`defect-dispositions.json` / `defect-ledger.json`); this
// contract makes citing it mandatory whenever a report claims a prior cycle's
// defect is closed.
//
// Two levels, both required:
//
//   - closureClaimOffenders(text) — the content rule, line-scoped.
//   - hooks.Classify — the WIRING. A gate reachable only from a unit test is
//     dead code; every acceptance case below reaches the rule through the real
//     audit verdict seam, the same seam reconcileContinuationDefects hangs off
//     (audit.go:311).
//
// Detection rule pinned by this contract (case-insensitive, per LINE):
//
//	claim   := line contains "verified closed"
//	           OR (line contains "closed" AND line references cycle-<digits>)
//	cited   := THE SAME line contains "defect-dispositions.json"
//	           or "defect-ledger.json"
//	offender := claim AND NOT cited
//
// Line-scoped deliberately: a single mention of the artifact elsewhere in a
// long CHANGELOG must not vouch for every closure claim in the file. That
// whole-document reading is the loophole, not the feature.
```

### `go/internal/phases/audit/ciparity.go:14` — above `var runCmd sysexec.RunFunc = sysexec.DefaultRunner`

```text
// ciparity.go — the unit-14 seam (ADR-0103) between the audit phase and the
// CI-parity gates, which live in internal/phases/audit/ciparitygate: the
// deterministic gates that stop a cycle shipping green-locally / red-in-CI.
// This file keeps the host's two package-var seams, the change-set locator,
// the ONE construction of the gates and the five Strangler facades the
// by-name tests and ACS predicates keep. The gates are wired ONLY through
// NewDefaultWithStageCompactSpec (production); New(Config{}) leaves them nil
// so the audit package's own `go test` never recursively forks the go
// toolchain. They run in the phase-runner process (not the sandboxed auditor
// LLM), so the subprocess is unrestricted.
```

### `go/internal/phases/audit/ciparity.go:121` — above `func changedPackagesForAudit(projectRoot string, cycle int) ([]string, bool) {`

```text
// changedPackagesForAudit locates this cycle's changed-package set and reports
// whether it is derivable. It prefers the build handoff when present (same
// locator the EGPS suite uses; a handoff yielding >=1 pkg is derivable), then
// falls back to a deterministic git derivation (changedpkgs.FromGitChecked vs
// HEAD). The handoff has been extinct since ~cycle 215, so the git fallback is
// what keeps the apicover gate live. The derivable flag closes the last
// fail-open hole: previously the git fallback returned nil identically whether
// the tree was git-clean (nothing changed) or the set was underivable (git
// failed), letting an underivable cycle ship with a silent PASS (cycle-581
// D1/D2, standing memory warnship_apicover_ci_gap). Injected into the gates as
// their change-set Strategy: git and the handoff layout never enter the leaf.
```

### `go/internal/phases/audit/ciparity_caveat.go:3` — above `import (`

```text
// ciparity_caveat.go — the integration-tier gate reports what it MEASURED, not
// what it guesses CI would do.
//
// The gate runs `go test -tags integration` on the cycle host and used to
// conclude "CI's integration-tier test step would FAIL". That inference is only
// valid when the host and CI execute the SAME set of tests, and they do not: the
// real-tmux tier is guarded by requireTmux, which t.Skip()s when tmux is absent
// from PATH. GitHub runners have no tmux. So every requireTmux-guarded test runs
// here and skips there, and a local offender in that set corresponds to no CI
// failure at all.
//
// cycle-1543 was blocked on exactly that: 13 offenders, all
// TestRealTmux_Interactive_*, all exit=80 (REPL BOOT timeout) — host contention
// from the wave's own concurrent agent tmux sessions, not defects. Measured with
// the wave stopped: 7/7 PASS in 17.2s, versus 3.6x-7.7x slower and failing under
// load. Meanwhile main's go job ran the same tier in CI and passed.
//
// The discriminator is DERIVED from the same predicate requireTmux uses rather
// than a list of test files, so the caveat stays true if the guarded set changes
// — a hardcoded file list would rot into a second falsehood.
```

### `go/internal/phases/audit/ciparity_caveat_test.go:3` — above `import (`

```text
// ciparity_caveat_test.go — a gate may not assert a CI outcome it cannot know.
//
// cycle-1543 (wave-20260822b-verify) was blocked by the integration-tier gate
// with: "the integration tier reported 13 offender(s) — CI's integration-tier
// test step would FAIL (e.g. TestFleetSoak)". That claim is FALSE, and provably
// so: all 13 offenders were TestRealTmux_Interactive_*, every one guarded by
// requireTmux, which t.Skip()s when tmux is absent from PATH. GitHub runners
// have no tmux, so those tests SKIP in CI — main's go job on 444815a4 ran
// `go test -race -tags integration` and passed with all of them in the tree.
//
// The failures were host contention, measured: 7/7 PASS in 17.2s with no wave
// running, versus 3.6x-7.7x slower and exit=80 (REPL BOOT timeout) while the
// wave held concurrent agent tmux sessions.
//
// A gate that blocks real work citing an impossible CI failure teaches
// operators to bypass gates. The caveat is DERIVED from the same predicate
// requireTmux uses — does this host have tmux — so it stays true if the guarded
// test set ever changes, rather than encoding today's file names.
```

### `go/internal/phases/audit/ciparity_caveat_test.go:140` — above `if strings.Contains(msg, "CI's integration-tier test step would FAIL") {`

```text
// The falsehood that blocked cycle-1543 must be gone from the LIVE message.
```

### `go/internal/phases/audit/ciparity_derivability_test.go:3` — above `import (`

```text
// ciparity_derivability_test.go — RED contract for cycle-582's
// changedpkgs-derivability-failloud task (scout-report.md Task 1; cycle-581
// audit D1/D2, unshipped).
//
// TODAY: changedPackagesForAudit returns nil identically whether the tree is
// git-clean (nothing changed) or the changed-set is genuinely underivable
// (git error, no repo, fleet index-lock race). Both apicoverEnforceChangedDefault
// and apicoverNewPackageGraduationDefault treat a nil changed-set as "nothing to
// enforce" and silently no-op (nil, nil) — a fail-open PASS on the very cycle
// that most needs the gate.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this file fails to compile today; that compile failure IS the RED
// evidence):
//
//   - changedPackagesForAudit gains a second return value, derivable bool
//     (true via the handoff path, or via changedpkgs.FromGitChecked's own
//     derivable flag).
//   - apicoverEnforceChangedDefault and apicoverNewPackageGraduationDefault
//     each return a single actionable offender (FAIL, not WARN — err stays
//     nil) when the module dir exists, an .apicover-enforce list is present,
//     and the changed-set is underivable — instead of falling through the
//     empty-intersection/empty-ungraduated no-op path.
//   - A genuinely clean, git-derivable tree remains (nil, nil) — the fix must
//     not turn every cycle into a FAIL.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/negative pair per gate: *_UnderivableChangedSet_FailsLoud
//     (must FAIL) vs *_CleanGitTree_StaysNoOp (must NOT FAIL) — the paired
//     test is the strongest guard against a naive "always FAIL" or "never
//     FAIL" implementation.
```

### `go/internal/phases/audit/ciparity_derivability_test.go:99` — above `func TestApicoverEnforceChangedDefault_UnderivableChangedSet_FailsLoud(t *testing.T) {`

```text
// TestApicoverEnforceChangedDefault_UnderivableChangedSet_FailsLoud: an
// underivable changed-set on a cycle with a real .apicover-enforce list must
// FAIL loud (non-empty offenders, err==nil) instead of the current silent
// (nil, nil) no-op — closing the fail-open class from cycle-581 D1/D2.
```

### `go/internal/phases/audit/ciparity_integration_test.go:14` — above `func TestRun_IntegrationTierGate_Offenders_FAILsAudit(t *testing.T) {`

```text
// TDD RED (cycle-806, task ciparity-integration-tier).
//
// Config.CheckIntegrationTier and integrationTierCheckDefault do not yet exist
// → compile RED until Builder adds the integration-tier CI-parity gate. Root
// cause it closes: the `go` workflow's `-tags integration` job (TestFleetSoak)
// went red while the per-cycle audit stayed green, because ciparity ran
// go vet / acs-durable / apicover but NEVER the integration tier — the
// warnship_apicover_ci_gap disease, one tier up (per-cycle proof ⊊ repo CI).
```

### `go/internal/phases/audit/ciparity_integration_test.go:64` — above `func TestNewDefault_WiresIntegrationTierGate(t *testing.T) {`

```text
// AC3.3 (membership / anti-drift pin) — NewDefault must WIRE the real
// integration-tier gate (cycle-147 dormant-seam lesson), and that gate must
// actually build the test binary under `-tags integration`. Proof that does NOT
// couple to the exact -run pattern: a fixture cmd/tool package (not env-exclusive — membership is the record table integrationTierEnvExclusive, the single authority) with an
// integration-tagged test file that FAILS TO COMPILE only under that tag. Under
// `-tags integration` the whole test binary fails to build → non-zero exit →
// offenders, regardless of any -run filter (go compiles the binary before -run
// selection). Strip the tag from the gate command and the file is excluded, the
// package compiles clean, and this test fails — so it pins the tag membership.
```

### `go/internal/phases/audit/ciparity_integration_test.go:115` — above `func writeRaceFixtureWorktree(t *testing.T) string {`

```text
// TDD RED (cycle-809, task ciparity-integration-tier-race-parity).
//
// Root cause it closes: integrationTierCheckDefault runs
// `go test -count=1 -tags integration <pkgs>` (ciparity.go:205) but the CI step
// it claims to mirror runs `go test -race -count=1 -tags integration ...`
// (.github/workflows/go.yml:59). `-race` is present in CI, absent from the gate.
// A genuine data race in a touched package therefore passes this cycle's audit
// clean and then goes CI-red on the exact `-tags integration` step this gate was
// built to pre-empt — the warnship_apicover_ci_gap disease (per-cycle proof ⊊
// repo CI), recurring one flag short of parity.
```

### `go/internal/phases/audit/ciparity_integration_test.go:181` — above `func TestIntegrationTierGate_Race(t *testing.T) {`

```text
// AC1 (behavioral, headline RED) — the integration-tier gate must catch a REAL
// data race, which is only possible when its `go test` command carries `-race`.
// Currently the gate runs without `-race`, so the int-counter race passes → the
// gate reports zero offenders → this test FAILs (RED). After Builder adds `-race`
// the race detector fires → non-zero exit → offenders → GREEN. This proves the
// flag by effect, never by string presence (cycle-85 anti-gaming rule).
```

### `go/internal/phases/audit/ciparity_newexport_test.go:3` — above `import (`

```text
// ciparity_newexport_test.go — RED contract for cycle-1331's
// percycle-audit-apicover-newexport-parity task (scout finding 4). The
// existing two-gate split (apicoverEnforceChangedDefault: touched∩enforced;
// apicoverNewPackageGraduationDefault: new-package blind spot) has never had a
// regression test proving the specific edge: a new EXPORTED symbol landing in
// an EXISTING enforced package via a brand-new file (not a new package, and
// not an edit to an already-tracked file). This is the untested case flagged
// in scout-report.md Finding 4 — the per-cycle gate must catch it exactly as
// CI's whole-repo `apicover -enforce` would, since the new file is recorded
// under the handoff's `files_new` bucket rather than `files_modified`.
//
// changedpkgs.ChangedPackages folds BOTH files_new and files_modified into the
// same changed-package set (changedpkgs.go:85-86), so the hypothesis is that
// no code change is required — this test exists to make that parity a durable,
// provable guard rather than an assumption (scout Hypothesis 2).
```

### `go/internal/phases/audit/ciparity_newpkg_test.go:3` — above `import (`

```text
// ciparity_newpkg_test.go — RED contract for cycle-547's
// apicover-new-package-graduation-gate task, wiring half (ciparity.go's
// NewUngraduatedPackages pure function is tested directly in
// internal/ciparity/newpkg_test.go; this file pins the audit-phase gate that
// consumes it).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence):
//
//   - Config gains a new CI-parity hook field, CheckApicoverNewPkgGraduation
//     func(req core.PhaseRequest) ([]string, error), wired through Run
//     exactly like CheckGoVet/CheckACSDurable/CheckApicoverEnforce (offenders
//     -> FAIL; infra error -> fail-open WARN).
//   - apicoverNewPackageGraduationDefault(req) is the real implementation:
//     reads the cycle's changed packages + .apicover-enforce (mirrors
//     apicoverEnforceChangedDefault's own resolution), calls
//     ciparity.NewUngraduatedPackages, and returns an actionable offender line
//     per ungraduated package when non-empty.
//   - NewDefaultWithStageCompact wires CheckApicoverNewPkgGraduation:
//     apicoverNewPackageGraduationDefault alongside the other three CI-parity
//     gates.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestApicoverNewPkgGraduation_OffendersFailAudit (mirrors
//     TestRun_CIParityGate_Offenders_FAILsAudit's exact pattern for the new
//     hook)
//   - Negative : TestApicoverNewPkgGraduation_NoUngraduatedPackages_NoOp (an
//     already-graduated changed package must not FAIL)
//   - Edge     : TestApicoverNewPkgGraduationDefault_CmdChangeNotFlagged (a
//     go/cmd/... only change must be a no-op — the AC's explicit exclusion)
```

### `go/internal/phases/audit/ciparity_newpkg_test.go:117` — above `func TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix(t *testing.T) {`

```text
// TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix —
// cycle-1329 AC1 (audit-warn-prescription-gate): the audit offender line for
// an ungraduated package must carry the SAME copy-pasteable prescription the
// build-entry seam already emits (graduationPrescription /
// phase_bindings_graduation.go:81), not just the terse "add it + an
// apicover_named_test.go" sentence the offender line has today. This is the
// exact fixture from TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged
// (new ungraduated go/internal/brandnew), re-asserted against the OFFENDER
// STRING CONTENT rather than merely its non-emptiness — the assertion this
// task adds. Today's offender string is the terse sentence, so this is RED
// until the audit seam is wired to the relocated ciparity prescription
// helper (Beyond-the-Ask / Research→Implementation Map hypothesis 1).
```

### `go/internal/phases/audit/ciparity_premove_pins_test.go:3` — above `import (`

```text
// ciparity_premove_pins_test.go — ADR-0103 unit 14, step 1: the order and
// byte-identity invariants no test pinned before the CI-parity gates moved
// into internal/phases/audit/ciparitygate. Every pin here was GREEN on the
// pre-extraction code (8e8f080f) and proven RED against its named mutant
// before the move; they run through the kept facades, so the leaf must keep
// them green byte-for-byte. The goldens they read were captured on the same
// tree (ciparitygate/testdata/*.golden.*). The pins that named moved symbols
// (the module guard before derivation, the lock root, the attempt-1 write
// before the lock wait, the two stderr lines) moved with them into the leaf
// (scope, lock, stream and stderr tests) and were deleted here.
```

### `go/internal/phases/audit/ciparity_premove_pins_test.go:29` — above `func premoveGolden(t *testing.T, name string) map[string]string {`

```text
// premoveGolden reads one `key<TAB>quoted` golden file captured on 8e8f080f.
```

### `go/internal/phases/audit/ciparity_premove_pins_test.go:125` — above `func TestIntegrationTierLog_MatchesTheGoldenBytes(t *testing.T) {`

```text
// Pin 6 (M7, M8): integration-tier.log is byte-identical to the golden
// captured on 8e8f080f for the red-then-green run.
```

### `go/internal/phases/audit/ciparity_premove_pins_test.go:150` — above `func TestChangedSetUnderivable_SeverityAsymmetryBytes(t *testing.T) {`

```text
// Pin 7 (M9, M10): the cycle-581 severity asymmetry, byte-exact — the three
// whole-repo gates WARN with the one underivable text; the two apicover gates
// hard-FAIL with their own D1/D2 sentence.
```

### `go/internal/phases/audit/ciparity_seam_test.go:3` — above `import (`

```text
// ciparity_seam_test.go — ADR-0103 unit 14: the host seam (ciparity.go) that
// builds the CI-parity gates per call from the package-var seams, projects
// the request, keeps the five Strangler facades, and threads the Signal
// Center from the composition root through Config.Signals / WithSignals.
```

### `go/internal/phases/audit/ciparity_touchedgo_derivability_test.go:3` — above `import (`

```text
// ciparity_touchedgo_derivability_test.go — RED contract for the inbox defect
// `cycletouchedgo-derivability-silent-skip`.
//
// TODAY cycleTouchedGo computed `pkgs, _ := changedPackagesForAudit(...); return
// len(pkgs) > 0` — it DISCARDED the derivable bool. changedpkgs.FromGitChecked
// only ever returns (nil,false) on git failure, so !derivable implies
// len(pkgs)==0 implies cycleTouchedGo==false. When git diff fails (the concrete
// trigger the changedpkgs doc names: a concurrent-fleet .git/index.lock race)
// all three whole-repo gates — go vet, acs-durable, integration-tier — short-
// circuited to (nil,nil): a SILENT skip, neither a WARN nor a whole-repo run.
//
// That is the cycle-581 D1/D2 fail-open conflation (git-clean vs underivable)
// reintroduced one call-frame UP from where apicover was hardened against it.
// A gate that cannot determine its input must not resolve to "nothing to check".
//
// FIX CONTRACT:
//   - the touched∧derivable decision has ONE owner (changedScopeForGate); no
//     gate re-derives the change-set independently;
//   - underivable ⇒ (nil, error) so applyCIGate surfaces a WARN diagnostic
//     ("gate skipped, CI backstops") — WARN not FAIL, since a transient index
//     lock must not hard-block a shippable cycle;
//   - a genuinely Go-untouched but DERIVABLE cycle still no-ops silently — the
//     paired negative that stops a naive "always WARN" implementation;
//   - a worktree with no Go module at all stays silent (nothing to check), so a
//     docs-only / synthetic-fixture cycle gains no spurious WARN.
```

### `go/internal/phases/audit/ciparity_unit_test.go:208` — above `func TestApicoverEnforceChanged_MeasurementError_Fails(t *testing.T) {`

```text
// TestApicoverEnforceChanged_MeasurementError_Fails: when apicover.Run itself
// errors (a touched package won't parse → code 2), the gate must FAIL
// (offenders, nil) — the same bucket the old bin/apicover exit-2 fell into — NOT
// silently downgrade to a WARN (nil, err). In-process there is no exec-start
// failure mode, so any measurement error is a real gate failure (cf. the
// underivable-changed-set hard-FAIL, cycle-581 D1).
```

### `go/internal/phases/audit/ciparity_unit_test.go:226` — above `func seqRunFunc(t *testing.T, script []struct {`

```text
// --- integration-tier flake-absorb (post-v22.4.2 false-RED class) ----------
//
// Post-release audit of the verification batch proved 3 audit-FAILs (cycles
// 943/950/955) were tier false-REDs: every named test PASSES in isolation in
// the failed cycles' own preserved worktrees. Two mechanisms, two remedies:
//   - env-inheritance: the gate subprocess inherited the lane's full
//     environment (sysexec nil-env → os.Environ()) while CI runs clean — a
//     CI-parity bug; the tier now ALWAYS runs with a scrubbed allowlist env;
//   - fleet contention: -race integration tests starve under live lanes; on
//     red the tier retakes ONCE under a cross-lane exclusive lock — a green
//     retake is a flake (absorbed → WARN), a red retake is genuine (FAIL).
```

### `go/internal/phases/audit/ciparity_unit_test.go:260` — above `func killedAtDeadline(fn sysexec.RunFunc) sysexec.RunFunc {`

```text
// killedAtDeadline makes a scripted runner faithful to a process the ctx
// deadline KILLED: it returns only once ctx is done — a real SIGKILL follows
// the deadline, never precedes it — so the 1 ns budgets below reach the
// deadline arms deterministically. Without it context.WithTimeout(…, 1ns) may
// arm a timer instead of expiring synchronously and an instant fake is
// observed before ctx.Err() is set (the leaf saw 2/40 such runs, 2026-09-14).
```

### `go/internal/phases/audit/classification.go:53` — above `advisories, reviewErr := validateExplanationReview(artifact, req)`

```text
// ADR-0102 (2026-09-13): the review's shape is advisory — it rides the
// record as warnings; only a missing reasoning or a missing delivery
// still forces the verdict.
```

### `go/internal/phases/audit/closure_claim.go:11` — above `var closureCycleRef = regexp.MustCompile('cycle[- ]?(\d+)')`

```text
// closure_claim.go — the closure-citation gate (cycle-1285 Task 2; inbox item
// `continuation-defect-ledger` clause (3), batch-integrity-review-2026-08-04.md:123).
//
// The 1255 → 1268 → 1270 → 1272 chain closed a named CRITICAL by ASSERTION: a
// bookkeeping line reading "verified closed" was the entire proof, and nothing
// required it to point at a record a reader could check. defect_ledger.go now
// MINTS that record (defect-ledger.json / defect-dispositions.json); this gate
// makes citing it mandatory whenever a report claims a prior cycle's defect is
// closed. Without it the ledger is a filing cabinet nobody is obliged to open.
//
// LINE-scoped, deliberately. A whole-document reading — "the file mentions
// defect-dispositions.json somewhere, so every closure claim in it is cited" —
// is the loophole, not the feature: one incidental mention would vouch for
// twenty unevidenced claims.
```

### `go/internal/phases/audit/closure_claim.go:26` — above `var closureCycleRef = regexp.MustCompile('cycle[- ]?(\d+)')`

```text
// closureCycleRef matches a cycle reference in prose ("cycle-1272", "cycle
// 1255"), with the number captured. Serves two callers: the weak rung below
// (MatchString — does this line reference ANY cycle) and closureLineCycleRefs
// (the captured numbers themselves, for the lineage-scoped demotion in
// audit.go) — one pattern, so a future tune to what counts as a cycle
// reference cannot update one caller's notion of it and not the other's.
```

### `go/internal/phases/audit/closure_claim.go:46` — above `var (`

```text
// The closure-claim token matchers (cycle-1431 lesson — see
// closureClaimOffenders and the stripQuotedSpans design record): word-bounded
// so "disclosed"/"foreclosed" never match; the negation/openness guards apply
// to the WEAK rung only. The negation vocabulary is deliberately small
// ("hasn't/won't/cannot be closed" still flag) — grow it from firings, not
// speculation.
```

### `go/internal/phases/audit/closure_claim.go:59` — above `closureClosedTokenRE = regexp.MustCompile('(?:^|[^-\w])closed\b')`

```text
// `(?:^|[^-\w])` instead of `\b`: a hyphen IS a word boundary, so plain
// \bclosed\b matched inside "fail-closed" — a state adjective, not a
// closure claim (cycle-1493 infra-systemic halt; the same compound in
// cycle-1486's "fail-closed by construction" prose fired twice more).
// Letter-prefixed compounds ("disclosed") stay excluded because a letter
// is rejected by [^-\w] just as it was by \b.
```

### `go/internal/phases/audit/closure_claim.go:88` — above `strong := closureClaimRE.MatchString(lower)`

```text
// Two rungs (cycle-1431 lesson — see the matcher var block): the
// STRONG rung ("verified closed") is never guard-suppressed — an
// appended "…still open" clause must not become a one-token bypass of
// the citation demand; only the WEAK rung (bare "closed" + cycle-ref)
// accepts the negation/openness guards, whose whole job is the
// disclosed/"still open" false-RED class.
```

### `go/internal/phases/audit/closure_claim.go:95` — above `weak := closureClosedTokenRE.MatchString(lower) && closureCycleRef.MatchString(stripPathTokens(lower)) &&`

```text
// The weak rung's cycle reference must come from PROSE, not from a
// path: `.evolve/runs/cycle-1493/…` is a citation locator, and it was
// the ONLY cycle token on the line that force-FAILed cycle-1493's
// narrative-green audit. Tokens containing '/' are dropped for this
// one check; the closed-token, negation, and citation checks keep the
// full line (a real prose claim next to a path still flags — pinned).
```

### `go/internal/phases/audit/closure_claim.go:120` — above `func stripQuotedSpans(line string) string {`

```text
// stripQuotedSpans removes text between matched quotation marks so the gate
// matches an ASSERTION of closure rather than the mere presence of the phrase
// (cycle-1285 F5).
//
// The canonical inherited defect text in this repo literally contains the words
// "verified closed" — batch-integrity-review-2026-08-04.md reports the 1255-D1
// CRITICAL as having been «narrowed to 'verified closed'». Substring matching
// therefore FAILED the auditor who correctly reported that defect as still
// open, which is worse than useless: the cheapest way out is to append the
// literal token "defect-dispositions.json" to the line, which satisfies the
// gate and adds no evidence at all. A gate whose remedy is a one-token
// appeasement becomes noise and then gets deleted.
//
// Quoting is the signal because it is what the honest report actually does:
// quoting someone else's closure claim is reporting, asserting one unquoted is
// claiming. Explicit negation markers ("still open", "not closed") were
// originally REJECTED as a second signal — they would hand the gate a bypass
// strictly cheaper than the citation it demands, since appending "not closed"
// is one token and evidences nothing. Cycle-1431 (with prior firings
// 1339/1371/1428) revised that posture for the WEAK rung only: four P0
// false-RED batch halts on honest refutations outweigh a one-rung leak that
// the per-id dispositions gate still backstops, so bare-"closed"+cycle-ref
// lines accept the negation/openness guards. The STRONG rung ("verified
// closed") keeps the original rejection in full — no guard suppresses it —
// so the one-token bypass remains closed where the claim is unambiguous.
// Quoting cannot be used the same way: a claim wrapped in quotes reads as
// someone else's.
//
// Backticks are NOT delimiters. In markdown a code span is how a real citation
// is written (`defect-dispositions.json`), so stripping them would erase the
// evidence and manufacture offenders.
//
// An unmatched delimiter strips nothing: a line with one apostrophe must not
// swallow its own tail. A `'` is a delimiter only when it is not word-internal,
// which keeps ordinary possessives ("the gate's record") out of the pairing.
```

### `go/internal/phases/audit/closure_claim.go:179` — above `func stripPathTokens(line string) string {`

```text
// stripPathTokens drops whitespace-delimited tokens containing '/' — file
// paths and locators. Used ONLY for the weak rung's cycle-reference check: a
// cycle number inside an evidence path is where a citation points, not a
// prose claim about that cycle (cycle-1493). Accepted misses, documented in
// the compound test: a markdown-link ref ("[cycle-1272](docs/x.md)") and a
// dual-ref token ("cycle-1272/cycle-1273") also strip — both are weak-rung
// shapes an author could already evade by omitting the ref outright, and the
// strong rung + citation demand still stand on such lines.
```

### `go/internal/phases/audit/closure_claim_boundary_test.go:3` — above `import (`

```text
// closure_claim_boundary_test.go — RED contract for the cycle-1431 verify-wave
// halt (auto-filed P0; prior firings 1339/1371/1428): closureClaimOffenders
// used an unbounded strings.Contains, so the substring "closed" inside
// "disclosed" — on a line that literally ended "still open" — tripped the
// gate, force-FAILed a narrative-PASS audit, and halted the batch as
// infra-systemic. Two false-positive classes close here: (1) substring
// matches ("disclosed", "foreclosed"); (2) negated/openness-asserting lines
// ("is NOT closed", "still open") — a report SAYING a defect remains open is
// the opposite of a closure claim.
```

### `go/internal/phases/audit/closure_claim_boundary_test.go:21` — above `"The minted-path fix (cycle-1424) is disclosed in the footer; the underlying defect is still open.",`

```text
// The live cycle-1431 shape: "closed" only inside "disclosed", line asserts openness.
```

### `go/internal/phases/audit/closure_claim_compound_test.go:3` — above `import (`

```text
// closure_claim_compound_test.go — RED contract for the cycle-1493 infra-systemic
// halt (batch-20260816c): two more weak-rung false-positive classes, both live-fired.
// (1) HYPHEN COMPOUNDS: `\bclosed\b` matches inside "fail-closed" — the hyphen is a
// word boundary, so the cycle-1431 fix's "disclosed/foreclosed never match" guarantee
// does not extend to hyphenated adjectives. (2) PATH-SHAPED CYCLE REFS: the weak rung's
// cycle-reference requirement was satisfied by the report's OWN evidence path
// (`.evolve/runs/cycle-1493/coverage-gate-report.md:32-36`) — a citation locator, not a
// prose claim about a prior cycle. Line 36 of the live audit-report asserted the
// inherited defect was "reproduced, not fixed" (the OPPOSITE of closure), carried
// "closed" only inside two "fail-closed" tokens and a cycle ref only inside its
// evidence path — and still force-FAILed a narrative-green audit. Cycle-1486's two
// closure flags ("fail-closed by construction" prose) were the same class.
```

### `go/internal/phases/audit/closure_claim_compound_test.go:21` — above `const cycle1493Line36 = "| H3 | HIGH | Inherited defect 'd8e3cdca…' is reproduced, not fixed: the coverage gate FAILs at…`

```text
// The live cycle-1493 audit-report.md line 36, byte-shape preserved (trimmed of the
// table's trailing spaces): every signal on it is a false one.
```

### `go/internal/phases/audit/closure_claim_demotion_test.go:3` — above `import (`

```text
// closure_claim_demotion_test.go — RED contract for the cycle-1502
// verdict-incoherence halt (batch-20260817a): the closure-citation gate forced
// FAIL over ONE summary line lacking a same-line citation, on a report whose
// per-id defect-dispositions covered EVERY inherited defect and whose
// continuation defect-ledger reconcile had verified that accounting (acs
// 165/0, 8/8 predicates, all 4 ids dispositioned). The reconcile's verified
// machine record is strictly stronger evidence than the line citation the
// prose gate demands — so when the reconcile RAN against a lineage and
// accounted every defect, prose closure-misses demote to warning diagnostics
// instead of verdict-forcing FAIL. Every other path is unchanged: a blocked
// reconcile still forces, and a NON-continuation cycle (no lineage, no
// dispositions — the original 1255 laundering shape) still forces.
```

### `go/internal/phases/audit/closure_claim_demotion_test.go:25` — above `const demotionClosureReport = "# Audit Report\n\n## Findings\n\n" +`

```text
// The cycle-1502 line-139 shape: a WARN summary asserting closure, no
// citation on that line.
```

### `go/internal/phases/audit/closure_docs_test.go:10` — above `var closureGovernedDocs = []string{`

```text
// closure_docs_test.go — cycle-1287 RED contract for the closure-citation gate's
// own paperwork (Task 2, batch-integrity-review-doc-closure-crossref).
//
// The gate in closure_claim.go exists because the 1255 → 1272 chain closed a
// CRITICAL with the words "verified closed" and no record. The two documents
// that narrate that gate must therefore SATISFY it — a doc that announces the
// rule while breaking it is the exact pattern the inbox item names. This is a
// self-check, not prose review: it runs the production `closureClaimOffenders`
// over the real committed files.
```

### `go/internal/phases/audit/closure_docs_test.go:51` — above `func TestC1287_DocsPassClosureCitationGate(t *testing.T) {`

```text
// TestC1287_DocsPassClosureCitationGate is the cycle-1287 crux for Task 2: every
// closure claim in the two governed documents must name the per-defect
// disposition record on its own line. RED until the "Not closed here" section
// and the F1 accounting lines are rewritten as cited closure records.
```

### `go/internal/phases/audit/defect_ledger.go:16` — above `const (`

```text
// defect_ledger.go — the anti-laundering ledger
// (batch-integrity-review-2026-08-04.md F1(i)).
//
// A named CRITICAL defect survived the 1255 → 1268-salvage → 1270 → 1272 chain
// by being individually honest at every step but collectively erased: each
// continuation narrowed, renamed, or declared-already-fixed the defect, and no
// code anywhere required a continuation to reconcile against the ORIGINAL
// rejecting audit's machine-readable defects[].
//
// Two mechanisms, both hanging off hooks.Classify (the audit verdict seam):
//
//  1. EMIT — a rejecting audit persists <workspace>/defect-ledger.json, one
//     addressable OPEN entry per structured defect, text verbatim.
//  2. RECONCILE — a continuation cycle loads its ancestor's ledger and may NOT
//     emit PASS while any inherited OPEN entry is unaccounted for. The
//     disposition is written back into THIS cycle's ledger, so it is visible in
//     the audit's own artifact rather than inferable from a diff a human must
//     run. Entries transition; they are never deleted. A ledger that shrinks is
//     a ledger that launders.
//
// Degrade posture, deliberately asymmetric: a cycle that is not a continuation
// (no manifest) or whose ancestor left no ledger is a clean no-op — the
// overwhelming majority of cycles, and nothing to reconcile against. But a
// MISSING disposition artifact on a real continuation is the defect itself, not
// an environment gap, so it blocks (unlike probe_quarantine's missing-worktree
// case, which correctly degrades open).
//
// Since ADR-0103 unit 09 the ledger's schema, writer, readers and the gate
// live in internal/core/defectledger; this file is the audit package's SEAM:
// the vocabulary projected, the ledger's ONE wired construction, the
// request/rejection projections, the citation resolver the gate takes as a
// Strategy, the three production spellings and the Null-Object facades the
// by-name tests keep. Design: docs/architecture/decomposition/09-defectledger.md.
```

### `go/internal/phases/audit/defect_ledger.go:176` — above `func evidenceResolves(evidence string, req core.PhaseRequest) (bool, string) {`

```text
// evidenceResolves reports whether a closure claim's evidence names a file that
// actually EXISTS, plus the operator-facing reason when it does not. Validating
// evidence for non-emptiness alone accepts `evidence:"x"` and closes a CRITICAL
// on a string nobody can follow — the unverifiable closure claim the batch
// integrity review indicts.
//
// Deliberately permissive about SHAPE, strict about WHAT IT NAMES: auditors
// cite "path:line" and "path:line:col" as often as a bare path, and rejecting a
// legitimate citation shape would block every future continuation.
//
// Existence alone was the cycle-1282 DEF-2 hole: `os.Stat` under either root,
// plus a raw-absolute-path branch, meant `/etc/hosts` and the attacker's own
// `defect-dispositions.json` each closed a CRITICAL. Four rules now hold:
//
//  1. RELATIVE only. An absolute path names something outside the repo's
//     accounting; `/etc/hosts` exists on every host and proves nothing.
//  2. NO ESCAPE. After Clean, a leading ".." leaves the root — the workspace
//     sits three levels down, so traversal is reachable, not theoretical.
//  3. PROJECT ROOT or this lane's WORKTREE, never the workspace. A citation is
//     resolved under the project root first and, failing that, under
//     req.Worktree — an unmerged lane's fix is only ever in its own worktree
//     (cycle-1340; the 1320→1330 deadlock). The workspace is still barred: it is
//     this cycle's own agent-authored ephemera; citing it is the graded party
//     vouching for itself. Real workspace artifacts remain citable by their
//     path FROM the root (".evolve/runs/cycle-N/audit-report.md"), which is
//     also what makes the citation followable by a reader who has only the repo.
//  4. NOT THE GATE'S OWN RECORD. defect-dispositions.json / defect-ledger.json /
//     continuation-manifest.json are the mechanism's own bookkeeping; a claim
//     that cites them cites itself.
//
// Symlinks are rejected with Lstat rather than followed: a symlink planted in
// the tree resolves rule 2 away.
//
// minimal: ceiling is "a real, in-repo, non-self file exists". It does NOT
// prove the file is in this cycle's diff or is related to the defect text.
// Upgrade path: resolve against the changed set (`git diff --name-only
// <manifest.base_sha>` in the worktree) once the audit hook carries the diff.
// A multi-citation value (the array shape above, joined) resolves only when
// EVERY citation resolves: "one of these files exists" would let a real cite
// carry an invented one past the gate.
```

### `go/internal/phases/audit/defect_ledger.go:223` — above `if !citeShaped(f) {`

```text
// A ';'-joined fragment that is not cite-shaped is a prose ANNOTATION
// ("…; verified live: `go test` -> PASS") — cycles 1393/1415 rejected
// whole real claims on such fragments, accreting ledger entries faster
// than they closed. Annotations are ignored, never graded; every
// cite-SHAPED fragment must still resolve, and at least one is
// mandatory — prose alone stays inadmissible.
```

### `go/internal/phases/audit/defect_ledger.go:280` — above `if strings.HasSuffix(path, ")") {`

```text
// Drop ONE trailing parenthetical annotation ("path:114-129 (helperName
// now cycle-scoped)") before locator stripping: two independent chains
// decorated otherwise-valid cites this way (cycles 1356/1360) and ground
// on "resolves to no file" every round, ACCRETING ledger entries faster
// than they closed. The annotation is dropped, never resolved; every
// rejection below still applies to the stripped path — an
// annotation-only cite (" (…)" with nothing before it, LastIndex 0) and
// a bare "(…)" (no " (" separator) fall through unchanged and fail the
// path checks as before.
```

### `go/internal/phases/audit/defect_ledger.go:294` — above `for i := 0; i < 2; i++ {`

```text
// Strip at most a ":line" and a ":col" suffix; anything else is part of the
// path (a Windows drive letter is not reachable here — these are repo paths).
// A ":line-line" RANGE counts as one locator: it is the house citation style
// in build and audit reports, and leaving it glued to the path made every
// range citation unresolvable under EVERY root (cycle-1340, defect
// ddda7857a — a real file, present in both roots, rejected anyway).
```

### `go/internal/phases/audit/defect_ledger.go:315` — above `base := filepath.Base(clean)`

```text
// Case-INSENSITIVE (cycle-1285 F3). The rejection below and the os.Lstat
// two lines down must agree on what "the same file" means, and on the
// stated platform (darwin/APFS) Lstat resolves "Defect-Ledger.json" to
// defect-ledger.json while an exact-string switch does not. That gap let
// the gate's OWN record close every inherited defect. Comparing with
// EqualFold is strictly conservative: on a case-sensitive volume it can
// only reject a differently-cased name that was never going to be a
// legitimate citation anyway.
```

### `go/internal/phases/audit/defect_ledger.go:332` — above `roots := []string{req.ProjectRoot}`

```text
// Two roots, project root FIRST. A continuation lane's fix lives in the
// lane's own worktree and reaches the project root only when the lane
// merges — which is exactly what this gate blocks when the citation
// misses. Cycles 1320→1323→1325→1330 each cited a real, worktree-resident
// file and each was rejected identically: the gate demanded evidence it
// structurally prevented from existing. The worktree is a FALLBACK, not a
// replacement, and it is reached only after rules 1-4 above have already
// run — so a self-citation or an escape is refused under either root.
```

### `go/internal/phases/audit/defect_ledger_adversarial_test.go:13` — above `func passingReport() string {`

```text
// defect_ledger_adversarial_test.go — the cycle-1282 audit's PoCs promoted from
// a throwaway `-overlay` into the tree. That audit's finding was not that the
// code was sloppy; it was that every committed suite was green because none of
// them constructed adversarial input. These do. They are regression locks: each
// FAILED on the pre-fix tree and names the defect it pins.
```

### `go/internal/phases/audit/defect_ledger_adversarial_test.go:25` — above `func TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition(t *testing.T) {`

```text
// TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition — cycle-1282 DEF-1
// (CRITICAL). The graded phase agent may write inside `.evolve/runs/cycle-N/`,
// so reading disposition state out of THIS cycle's own ledger let one file
// write close three inherited CRITICALs with no disposition artifact at all —
// and, because the merge keyed on ID alone, substitute the planted row's TEXT
// for the inherited defect's under the real id.
```

### `go/internal/phases/audit/defect_ledger_adversarial_test.go:65` — above `func TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect(t *testing.T) {`

```text
// TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect — cycle-1282 DEF-2
// (HIGH). evidenceResolves only os.Stat'd the citation under either root, with
// a raw-absolute branch, so `/etc/hosts` and the attacker's own disposition
// file each closed a CRITICAL. Existence is necessary, not sufficient.
```

### `go/internal/phases/audit/defect_ledger_adversarial_test.go:111` — above `func TestAdversarial_ShadowedIDIsLoudAndBlocking(t *testing.T) {`

```text
// TestAdversarial_ShadowedIDIsLoudAndBlocking — cycle-1282 DEF-3 (MEDIUM). A
// 4-byte defectID is ~2^32 from a chosen second preimage, and the merge index
// resolved a duplicated id to the LAST row, shadowing the inherited entry. The
// id is now 16 bytes, the FIRST row wins the index, and a text mismatch on an
// inherited id blocks instead of silently rewriting the record.
```

### `go/internal/phases/audit/defect_ledger_adversarial_test.go:155` — above `func TestEmitDefectLedger_CapsUnboundedDefects(t *testing.T) {`

```text
// TestEmitDefectLedger_CapsUnboundedDefects — cycle-1282 DEF-6 (LOW). The
// verdict sentinel is agent-authored; neither the defect count nor the per-line
// length was bounded. The cap must RECORD the overflow, never erase it.
```

### `go/internal/phases/audit/defect_ledger_annotation_prompt_test.go:3` — above `import (`

```text
// defect_ledger_annotation_prompt_test.go — RED contract for the two halves of
// inbox disposition-skeleton-preseed + evidence-cite-annotation-tolerance
// (2026-08-10 investigation; agents A/B: continuations 0/11 with evidence
// rejections and MISSING dispositions as the top killers).
//
// Half 1 — annotation tolerance: real chains authored evidence like
// "path.go:12-34; verified live: `go test ./...` -> PASS" and the whole claim
// was rejected because splitEvidence ANDs EVERY ';'-fragment as a citation
// (cycles 1393/1415). A prose fragment is an annotation, not a cite; a
// cite-SHAPED fragment must still resolve (a typoed path may never degrade
// into "prose"), and at least one cite-shaped fragment is still mandatory.
//
// Half 2 — continuations are TOLD their inherited ids: the audit prompt for a
// continuation workspace now carries the ancestor's OPEN defect ids + texts
// and the disposition duty, composed deterministically from the same records
// the gate grades against (no LLM tokens; ~200 tokens per continuation audit).
```

### `go/internal/phases/audit/defect_ledger_annotation_prompt_test.go:50` — above `{"cite-plus-prose", "docs/x.md:3; verified live: 'go test ./...' -> PASS", true},`

```text
// The cycle-1393/1415 class: real cite + prose annotation.
```

### `go/internal/phases/audit/defect_ledger_apicover_named_test.go:10` — above `func TestNewDefaultWithStageCompactSpec_AcceptsOptions(t *testing.T) {`

```text
// Test 44 — the tail constructor accepts functional options (ADR-0103 unit
// 09): WithSignals reaches Config.Signals; zero options is today's phase. The
// two exports Option and WithSignals are named here for the apicover gate.
```

### `go/internal/phases/audit/defect_ledger_cite_annotation_test.go:11` — above `func TestClassify_AnnotatedRangeCiteCloses(t *testing.T) {`

```text
// defect_ledger_cite_annotation_test.go — the decorated-cite long-tail
// (2026-08-06, evidence-cite-annotation-tolerance).
//
// Two independent chains decorated otherwise-VALID path:range cites with a
// trailing parenthetical annotation and ground on "resolves to no file":
// cycle-1356 "go/internal/phases/triage/triage.go:114-129
// (carryforwardCandidatesTimestamp...)" and cycle-~1360
// "go/internal/core/runlease_hook.go:56-73 (stale lease)". The annotation is
// reasonable agent output, not gaming — but evidenceResolves strips only
// numeric :suffixes, so the whole decorated string stats as a nonexistent
// path and the agent, believing its cite correct, re-decorates every round
// (the accretion grind). Tolerance: ONE trailing " (…)" group is DROPPED
// before resolution. Every anti-gaming rejection must survive: the stripped
// path still has to be a real, repo-relative, non-self-vouching regular file.
```

### `go/internal/phases/audit/defect_ledger_doc_example_test.go:14` — above `var dispositionExampleFence = regexp.MustCompile("(?s)'''json\\s*\\n(.*?)'''")`

```text
// defect_ledger_doc_example_test.go — RED contract for cycle-1403 Task 2
// `disposition-schema-literal-example` (scout-report.md Task 2).
//
// agents/evolve-auditor.md tells the auditor to write
// `{"dispositions":[{"id","status","evidence","reason"}]}` — a list of FIELD
// NAMES, not a document. It is not valid JSON and shows no legal value for any
// field, so the authoring agent must invent the shape; cycles 1397/1399/1400
// each invented a different wrong one. Task 2 replaces it with a filled literal
// example and keeps it identical to the one already in
// docs/architecture/continuation-defect-ledger.md.
//
// These predicates are NOT source greps for a magic string (the cycle-85 ban).
// They EXTRACT the documented example and run it through the production reader,
// readDispositions — the same function the gate calls — so a doc example that
// the gate would reject fails here. The cross-document case then compares the
// two examples as parsed JSON, not as text, so reformatting one is fine and
// drifting one is not.
```

### `go/internal/phases/audit/defect_ledger_doc_example_test.go:115` — above `func TestAuditorPromptAndArchDocDispositionExamplesAgree(t *testing.T) {`

```text
// TestAuditorPromptAndArchDocDispositionExamplesAgree — AC9, the doc-sync half
// (`always_full_documentation` house rule; cycle-1342 landed prompt and
// architecture doc together for exactly this reason). Compared as PARSED JSON,
// so reflowing or re-indenting one document is free and drifting its content is
// not.
```

### `go/internal/phases/audit/defect_ledger_evidence_edge_test.go:3` — above `import (`

```text
// defect_ledger_evidence_edge_test.go — edge-case pins for the cycle-1403
// tolerant-evidence fix (#422), added after the 2026-08-09 zero-ship batch
// postmortem (docs/incidents/2026-08-09-zero-ship-batch.md). The base suite
// (defect_ledger_evidence_shape_test.go) pins string/array/empty/object
// shapes; these cases close the corners the adversarial review left
// UNVERIFIED or noted as untested:
//   - mixed-type array (["cite", 42]) — encoding/json rejects mid-decode;
//     must fail CLOSED, never PASS, never crash.
//   - null evidence on FIXED — "evidence": null decodes to the zero value;
//     must be treated as no evidence.
//   - whitespace-only string — trim must not admit "   " as a citation.
//   - literal "; " inside ONE string — the join token doubles as a split
//     token, so a semicolon-joined pair behaves exactly like the array form:
//     both halves must resolve (stricter-never-looser, pinned both ways).
```

### `go/internal/phases/audit/defect_ledger_evidence_shape_test.go:11` — above `const evidenceUnparseableMarker = "is unparseable"`

```text
// defect_ledger_evidence_shape_test.go — RED contract for cycle-1403 Task 1
// `disposition-evidence-tolerant-unmarshal` (scout-report.md Task 1).
//
// The live failure. Cycle-1399's auditor wrote a defect-dispositions.json whose
// `evidence` was a JSON ARRAY of citations. `defectDispositionDoc.Evidence` is
// typed `string` (defect_ledger.go:89), so encoding/json rejected the whole
// document — `json: cannot unmarshal array into Go struct field
// .dispositions.evidence of type string` — and readDispositions blocked the
// cycle on "unparseable". The auditor had done the work and cited it; the gate
// could not read the claim. #419 (`fdc9c3e3`) tolerated a *decorated* cite
// string and is orthogonal: it never touches the JSON type.
//
// Contract shape. Every case reaches its subject through the REAL production
// seam, hooks{}.Classify — the audit verdict path — never readDispositions
// directly: a decoder that parses an array while the gate still blocks would be
// a fix nobody can use.
//
// NOTE TO BUILDER — join-and-forget is NOT a fix. scout-report Task 1 suggested
// joining array elements with "; ". A joined "a.go:1; b.go:2" is not a path, so
// evidenceResolves (defect_ledger.go:267) rejects it and the cycle blocks
// anyway — the operator-visible behaviour would be unchanged. AC2 below is
// stated at the verdict, not at the decoder, precisely so a cosmetic join
// cannot satisfy it: an array of RESOLVABLE cites must produce PASS. Whether
// you resolve each element or teach evidenceResolves to split is your call.
//
// Adversarial diversity (skills/adversarial-testing §6):
//   - regression  — the string shape that works today must keep working.
//   - new/positive — array of resolvable cites now PASSes (AC2, the crux).
//   - negative    — array of UNRESOLVABLE cites must still block, and must not
//     be reported as "unparseable": tolerance may not become a bypass.
//   - edge        — an empty array on a FIXED claim is "no evidence", still a
//     block.
//   - negative    — a shape that is neither string nor array (an object) must
//     still be rejected outright; no silent degrade to "" (cycle-1285 F2).
```

### `go/internal/phases/audit/defect_ledger_evidence_shape_test.go:77` — above `func TestClassify_DispositionEvidenceArrayShapeAccepted(t *testing.T) {`

```text
// TestClassify_DispositionEvidenceArrayShapeAccepted — AC2, THE CRUX, and the
// exact cycle-1399 reproduction. `evidence` is a JSON array of two citations,
// both resolving to real files. The gate must read the file (no "unparseable")
// and honour the closure (PASS). RED today: encoding/json refuses the document
// before any resolution logic runs.
```

### `go/internal/phases/audit/defect_ledger_evidence_shape_test.go:153` — above `func TestClassify_DispositionEvidenceObjectShapeStillBlocks(t *testing.T) {`

```text
// TestClassify_DispositionEvidenceObjectShapeStillBlocks — NEGATIVE. Neither
// string nor array-of-strings: an object. This must keep hitting the
// unparseable path and BLOCK. Silently degrading an unrecognised shape to ""
// is the cycle-1285 F2 posture violation ("degrading open there would hand the
// gate its cheapest bypass", defect_ledger.go:653-655).
```

### `go/internal/phases/audit/defect_ledger_hardening_test.go:13` — above `func evidenceFile(t *testing.T, root, rel string) string {`

```text
// defect_ledger_hardening_test.go — RED contract for cycle-1282, the
// CONTINUATION of cycle-1279 (`continuation-defect-ledger`). The mechanism
// landed in 1279; its own audit then rejected it with seven defects (D1–D7,
// .evolve/runs/cycle-1279/audit-report.md). This file encodes D1, D2, D3, D4
// and D6 — the five that live in this package — as executable criteria.
//
// Every assertion reaches its subject through the REAL production seam
// (`hooks.Classify`, audit.go:311 reconcile / :394 emit), never by calling an
// unexported helper directly: a predicate that passes on a helper passes on
// dead code, which is the failure mode this whole cycle exists to close.
//
// Contract summary the builder inherits:
//
//	D1  reconcile MERGES the current workspace ledger with the ancestor's; an
//	    entry present after one Classify is present after the next. Ids are
//	    derived from defect CONTENT, never from a position counter, so a
//	    re-mint cannot bind an old id to new text.
//	D2  a manifest-named continuation whose ancestor left NO ledger is
//	    DIAGNOSED (the `rm` that silently disarmed the gate becomes visible).
//	D3  a FIXED claim's evidence must RESOLVE to a real file (under ProjectRoot
//	    or the workspace); `evidence:"x"` is an unaccounted defect, not a
//	    closure.
//	D4  every disposition switch arm plus the non-OPEN carry-forward is
//	    exercised by a table case (the headline rule had zero coverage).
//	D6  emit fires on FAIL *and* WARN — a WARN-shipped cycle carrying
//	    structured defects must not leave the next continuation nothing to
//	    inherit.
```

### `go/internal/phases/audit/defect_ledger_hardening_test.go:96` — above `hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})`

```text
// Attempt 2 is the ordinary retry that now grades clean. reconcile runs
// (audit.go:311) and emit does not (nothing to emit on a PASS), so the
// truncate-write is unmasked: this cycle's own recorded defect is erased
// with no adversary at all, and the operator's record of what cycle-1270
// itself got wrong is gone.
```

### `go/internal/phases/audit/defect_ledger_hardening_test.go:225` — above `func TestClassify_ResolvableEvidenceClosesADefect(t *testing.T) {`

```text
// TestClassify_ResolvableEvidenceClosesADefect — the POSITIVE half of D3.
// Evidence that names a real file (in "path:line" form, and as a phase
// artifact) must still close the defect: a rule that rejected every claim would
// block every continuation forever.
//
// cycle-1282 DEF-2 narrowed the rule to PROJECT-ROOT resolution only. A phase
// artifact is still perfectly citable — by its path FROM the root, which is
// also what makes the citation followable by a reader who has only the repo.
// What is gone is the bare workspace-relative form, because the workspace is
// this cycle's own agent-authored ephemera and citing it is self-vouching.
```

### `go/internal/phases/audit/defect_ledger_hardening_test.go:258` — above `realEvidence := "go/internal/core/fleet.go"`

```text
// cycle-1282 DEF-2: closure evidence resolves under the PROJECT ROOT only.
```

### `go/internal/phases/audit/defect_ledger_prescription_test.go:15` — above `const prescriptionTagPrefix = carryover.PrescriptionPrefix`

```text
// defect_ledger_prescription_test.go — RED contract for cycle-1327's
// `audit-warn-prescription-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91).
//
// Reuse, not a parallel mechanism: emitDefectLedger already fires on WARN
// (audit.go:395) and reconcileAgainstAncestor is already generic over "an OPEN
// entry with an id and text" (defect_ledger.go:322-367). This file pins the
// ONE missing step — emitDefectLedger must also source
// Failure.Prescription — plus proves the existing reconcile/evidence gates
// apply unmodified to a prescription-sourced entry, and that an ordinary,
// prescription-less WARN is byte-for-byte unchanged (the regression guard
// against widening the ledger trigger into every narrative WARN).
//
// Prescription-sourced text carries a "PRESCRIPTION: " prefix (scout report
// Hypothesis 2) so an operator reading defect-ledger.json can distinguish "what
// was wrong" (a defect) from "a foreseen risk's named fix" (a prescription)
// without a second ledger or a schema-breaking Kind field.
```

### `go/internal/phases/audit/defect_ledger_prescription_test.go:35` — above `func warnReportWithPrescription(prescriptions ...string) string {`

```text
// warnReportWithPrescription renders an audit-report.md whose evolve-verdict
// sentinel is WARN, carries the given prescription strings and zero defects —
// the exact cycle-1258 shape (a foreseen risk, not a defect) that
// emitDefectLedger currently drops on the floor.
```

### `go/internal/phases/audit/defect_ledger_schema_inline_test.go:11` — above `var dispositionSchemaTokens = []string{"dispositions", "id", "status", "evidence", "reason"}`

```text
// defect_ledger_schema_inline_test.go — RED contract for cycle-1403 Task 3
// `disposition-parse-error-surfaced-inline` (scout-report.md Task 3).
//
// Today a rejected defect-dispositions.json yields the raw encoding/json error
// ("cannot unmarshal number into Go struct field …") and nothing else. The
// agent that must re-author the file on the next dispatch does not read Go, so
// the diagnostic names the failure without naming the remedy. Task 3 makes the
// rejection self-sufficient: the message carries the literal schema the file
// was supposed to match.
//
// Both cases drive hooks{}.Classify, the production verdict seam.
//
// Adversarial diversity: positive (the unparseable branch gains the schema) and
// negative (the MISSING branch is a DIFFERENT operator action — author the file
// — and must not be relabelled as a parse failure by this change).
```

### `go/internal/phases/audit/defect_ledger_schema_singlesource_test.go:11` — above `func TestDispositionSchemaExampleMatchesDocumentedExample(t *testing.T) {`

```text
// defect_ledger_schema_singlesource_test.go — the third leg of the doc-sync
// contract (cycle-1403). AC9 already holds agents/evolve-auditor.md and
// docs/architecture/continuation-defect-ledger.md to each other; this holds the
// GO constant echoed inline on rejection (dispositionSchemaExample) to the same
// document. Without it the two docs could stay in lockstep while the message an
// agent actually reads at the moment of failure drifted away from both — which
// is the failure mode this cycle exists to close, one level down.
```

### `go/internal/phases/audit/defect_ledger_seam_test.go:3` — above `import (`

```text
// defect_ledger_seam_test.go — ADR-0103 unit 09: the audit package's seam onto
// internal/core/defectledger — the Null-Object facades carry the REAL lane-scope
// reader and resolver, Config.Signals reaches the ledger, ONE construction
// site, the production spellings, and the ordered signal stream a blocked
// continuation leaves.
```

### `go/internal/phases/audit/defect_ledger_seam_test.go:41` — above `func TestDefectLedgerSeam_NullFacadeKeepsTheRegistryFallback(t *testing.T) {`

```text
// Test 40 — the free facade runs on a Null-Object ledger built with the REAL
// core.LaneScopeIDs: the cycle-1285 F2 fixture (registry + lane-scope pin,
// manifest deleted) still blocks with the registry finding. A source scan
// pins that the one wired construction spells the real collaborators.
```

### `go/internal/phases/audit/defect_ledger_test.go:14` — above `const (`

```text
// defect_ledger_test.go — RED contract for cycle-1279 Tasks 1 and 2
// (`continuation-defect-ledger-emit`, `continuation-audit-disposition-diff`;
// batch-integrity-review-2026-08-04.md F1 solution bullet i).
//
// The defect this pins: a named CRITICAL defect survived the
// 1255 → 1268-salvage → 1270 → 1272 chain by being individually honest at
// every step but collectively erased — each continuation narrowed, renamed, or
// declared-already-fixed the defect, and NO code anywhere required a
// continuation to reconcile against the ORIGINAL rejecting audit's
// machine-readable `defects[]`.
//
// Two mechanisms are pinned, both through the REAL production seam
// (`hooks.Classify` — the audit phase's verdict path, the same entry
// quarantineProbesForRequest hangs off at audit.go:169). A helper called
// directly would pass on dead code; every assertion below reaches its subject
// from Classify.
//
//  1. EMIT: a rejecting audit persists `<workspace>/defect-ledger.json`, one
//     addressable entry per structured defect, status OPEN.
//  2. DIFF: a continuation cycle's audit loads the ancestor's ledger and may
//     NOT emit PASS while any entry is unaccounted for; the disposition is
//     visible in the audit's own written-back ledger, never merely inferable.
//
// Wire schema pinned by this contract:
//
//	defect-ledger.json      {"origin_cycle":N,"entries":[{"id","text","status","evidence","reason"}]}
//	defect-dispositions.json {"dispositions":[{"id","status","evidence","reason"}]}
//
// status ∈ {OPEN, FIXED, DEFERRED}. Entries are never deleted — status
// transitions only (that is the anti-laundering property: a renamed or
// narrowed defect cannot make its ledger row disappear).
```

### `go/internal/phases/audit/defect_ledger_test.go:276` — above `writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{`

```text
// cycle-1282 D3: a closure claim's evidence must RESOLVE to a real file, so
// the fixture now materializes the artifacts it cites (evidenceFile lives in
// defect_ledger_hardening_test.go). This strengthens the fixture; the
// assertions below are unchanged.
```

### `go/internal/phases/audit/defect_ledger_unit09_pins_test.go:3` — above `import (`

```text
// defect_ledger_unit09_pins_test.go — ADR-0103 unit 09, step 1: the pre-move
// pins on the host's order and the gate's hidden couplings, written and green
// on 8e8f080f BEFORE the ledger moved into internal/core/defectledger, each
// proven red against its named mutant (design §6 tests 1-5). Every case drives
// the production seam hooks.Classify.
```

### `go/internal/phases/audit/defect_ledger_worktree_evidence_test.go:12` — above `func worktreeContinuationFixture(t *testing.T, ancestorCycle, thisCycle int, openDefects []string) (string, string, core…`

```text
// defect_ledger_worktree_evidence_test.go — RED contract for cycle-1340
// `defect-ledger-worktree-evidence-fallback` (the sole top_n card).
//
// The defect this pins (scout Finding 1): evidenceResolves
// (defect_ledger.go:253-300) resolves every closure citation with a single
// os.Lstat under req.ProjectRoot. A continuation LANE's own fix lives in its
// own still-open worktree and reaches the project root only when the lane
// merges — which is precisely what this gate blocks. Cycles 1320 → 1323 →
// 1325 → 1330 each cited the same two real, worktree-resident files and each
// was rejected with the identical "resolves to no file under the project
// root" message: the gate demands evidence it structurally prevents from
// existing. P0, cycles_unpicked=5+.
//
// The fix (Task 1): when the project-root Lstat misses AND req.Worktree != "",
// retry under req.Worktree — the SHIPPED-TREE root already threaded to every
// phase (core/phase.go:81-91). Every existing rejection stays: absolute paths,
// escapes, non-regular files, and the self-citation guard (rule 4) must reject
// worktree-resident citations exactly as they reject project-root ones, or the
// fallback reopens the self-vouching hole cycle-1285 F3 closed.
//
// Every assertion below reaches its subject through the REAL production seam,
// hooks{}.Classify — the audit phase's verdict path. evidenceResolves is
// unexported and calling it directly would pass on dead code.
//
// Adversarial diversity: positive (worktree-only evidence closes a defect),
// negative (absent from BOTH roots still blocks; self-citation still rejected
// under the new root), edge (empty Worktree unchanged; escape/absolute still
// rejected with a worktree set), semantic (four distinct gate behaviors, not
// one restated).
```

### `go/internal/phases/audit/defect_ledger_worktree_evidence_test.go:54` — above `func TestClassify_WorktreeResidentEvidenceClosesADefect(t *testing.T) {`

```text
// TestClassify_WorktreeResidentEvidenceClosesADefect — POSITIVE, the P0 repro.
// The lane cites go/cmd/evolve/cmd_loop_chain_boundaryrefresh_shortsha_test.go
// (cycle-1323's actual citation). The file exists in the lane's worktree and
// NOT under the project root, because the merge that would put it there is
// what this gate is blocking. Today: rejected, cycle cannot PASS, forever.
```

### `go/internal/phases/audit/defect_ledger_worktree_evidence_test.go:115` — above `func TestClassify_WorktreeSelfCitationStillRejected(t *testing.T) {`

```text
// TestClassify_WorktreeSelfCitationStillRejected — NEGATIVE, the hole the
// fallback could reopen. The gate's own bookkeeping is rejected by basename
// (rule 4, EqualFold, cycle-1285 F3). A lane may not evade that by planting
// defect-ledger.json in its worktree instead of the project root. The graded
// agent WRITES its own worktree, so this is the cheapest bypass of the fix.
```

### `go/internal/phases/audit/defect_ledger_worktree_evidence_test.go:164` — above `func TestClassify_LineRangeCitationResolves(t *testing.T) {`

```text
// TestClassify_LineRangeCitationResolves — the SECOND live instance of the
// same deadlock, found while reproducing it. Defect ddda7857a (inherited by
// this lane from 1325) cites "go/cmd/evolve/cmd_loop_chain.go:570-588": a real
// file, present under BOTH roots, rejected anyway. The suffix stripper takes
// only ":<digits>", so a ":<line>-<line>" RANGE stays glued to the path and no
// Lstat can ever succeed. The worktree fallback alone does not close the P0 —
// this citation misses under both roots for a reason the fallback cannot fix.
//
// Ranges are the house citation style (build/audit reports cite them
// everywhere), so this is the common case, not an exotic one.
```

### `go/internal/phases/audit/disposition_preflight_test.go:11` — above `const (`

```text
// disposition_preflight_test.go — RED contract for cycle-1342 Task 3
// `disposition-completeness-preflight` (scout-report.md Finding 4).
//
// Today, an ancestor id with no matching claim in defect-dispositions.json
// surfaces ONLY as a per-id fallthrough inside reconcileAgainstAncestor's
// switch — `unaccounted = append(unaccounted, a.ID+" (no disposition)")` —
// mixed in among every other per-id branch (FIXED-but-unresolvable,
// DEFERRED-without-reason, unknown status). That blocks PASS correctly, but
// there is no STRUCTURAL signal that names the disposition FILE itself as
// absent or short of the inherited-id set — a future auditor reading
// diagnostics sees N unrelated-looking per-id gripes, never "the file you
// were supposed to write covers 0 of 2 ids". Finding 4 calls this a
// pre-flight gap: nothing fails loudly, BY NAME, on the artifact's
// completeness as a whole before grading proceeds id-by-id.
//
// Every assertion below reaches its subject through the REAL production
// seam, hooks{}.Classify — the audit phase's verdict path. A helper called
// directly would pass on dead code.
//
// Adversarial diversity: negative (file entirely missing), negative (file
// present but short), edge/anti-no-op (a complete file and a non-continuation
// cycle must trip NEITHER new message — a pre-flight that always fires
// proves nothing).
```

### `go/internal/phases/audit/disposition_seed_singlesource_test.go:3` — above `import (`

```text
// disposition_seed_singlesource_test.go — pins core's disposition-skeleton
// preseed against THIS package's gate (ADR-0084 I2: writer and reader of a
// machine-graded artifact bind against each other). core cannot import audit,
// so its seeder re-reads the ledger wire shape; this test feeds one real
// document through both sides and proves:
//  1. same OPEN id set: every OPEN ancestor entry gets exactly one seeded row;
//  2. honest gate semantics on an UNTOUCHED skeleton: the preflight sees the
//     file as present and covering (never MISSING/INCOMPLETE), while the
//     per-id reconcile still blocks every seeded id by name — a seed the
//     auditor ignores can never launder a defect.
```

### `go/internal/phases/audit/egps_phantom_test.go:24` — above `const phantomVerdictJSON = '{`

```text
// The REAL cycle-1546 shape, as the new producer writes it.
```

### `go/internal/phases/audit/explanation_review_gate.go:14` — above `func validateExplanationReview(report string, req core.PhaseRequest) (advisories []string, err error) {`

```text
// validateExplanationReview applies audit's policy around the shared review
// contract (explanationdocs.ValidateReviewedHandoff). Since 2026-09-13
// (ADR-0102, operator decision) the reviewer's reasoning is the gate and the
// section's shape is advisory: the returned advisories ride the phase record
// as warnings and never touch the verdict. The error — the only blocking
// outcome — is reserved for a missing reasoning (the Evidence floor; a
// missing or duplicated review section is no review text at all), a missing
// Build delivery reviewed as anything but FAIL, and host-side defects in the
// handoff itself. Before this, cycles 1638 and 1640 (2026-09-13) were burned
// by a PASS narrative overridden on citation form alone.
```

### `go/internal/phases/audit/graduation_registration_test.go:5` — above `import (`

```text
// graduation_registration_test.go — cycle-675 AC2 (Task 2,
// build-entry-graduation-guard-audit-regression): the audit-side graduation
// gate (apicoverNewPackageGraduationDefault) landed 2026-07-07 wired via
// NewDefaultWithStageCompact (audit.go:419), but no test binds the PRODUCTION
// constructor to the gate actually firing — the existing coverage exercises
// the default function directly or injects a fake hook via New(Config). This
// inbox item is a 3rd recurrence caused by seams silently disagreeing on
// scope (cycle-652 retro), so the regression bar is: constructed exactly as
// production constructs it, an ungraduated new package FAILs the audit, and
// an enrolled one PASSes. If the CheckApicoverNewPkgGraduation wiring is ever
// dropped from NewDefaultWithStageCompact, the first arm goes green-PASS and
// this test fails.
```

### `go/internal/phases/audit/graduation_registration_test.go:44` — above `if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {`

```text
// Materialize the production file the handoff claims — the gate now skips
// test-only/absent package dirs (vacuous obligation; cycles 1223/1224/1228).
```

### `go/internal/phases/audit/integration_tier_orchestration_test.go:3` — above `import (`

```text
// integration_tier_orchestration_test.go — cycle-1554 RED contract for
// `integration-tier-contention-retake-accountability` (inbox
// pipeline-defect-pipeline-blocker, P0).
//
// ciparity_unit_test.go already proves CheckIntegrationTier itself returns
// (nil, flake-error) on red-then-green and (offenders, nil) on red-then-red.
// That is necessary but not sufficient: nothing in the suite wires the
// PRODUCTION integrationTierCheckDefault (the function NewDefaultWithStageCompact
// actually installs at audit.go:816/862, as h.integrationTierCheck) through the
// real hooks.Classify orchestration and asserts on the AUDIT VERDICT the gate
// produces. audit_verdict_conflict_gates_test.go exercises Classify's
// override/no-override wiring, but only via hand-authored offenders(...)/
// cannotRun(...) stand-ins for h.integrationTierCheck — never the real
// red-then-green-retake logic. A regression that broke the seam between
// CheckIntegrationTier's return shape and applyCIGate's (cerr!=nil ⇒ WARN,
// offenders>0 ⇒ FAIL) branching — e.g. a future refactor that stopped mapping
// the flake error into a could-not-run WARN — would pass every existing test
// in this package while silently turning every contention flake into a false
// audit FAIL (or laundering a genuine red-then-red into a WARN). These two
// tests close that gap: same subprocess-level fixtures as ciparity_unit_test.go,
// but driven through the real Classify orchestration.
```

### `go/internal/phases/audit/integration_tier_orchestration_test.go:135` — above `func TestAuditOrchestration_IntegrationTier_DeadlineKill_MarkerFreeDegradesToWarn(t *testing.T) {`

```text
// TestAuditOrchestration_IntegrationTier_DeadlineKill_MarkerFreeDegradesToWarn —
// a retake killed by its budget with NO recognizable verdict in the truncated
// output is not a judgment: it must degrade to the fail-open WARN, never a
// red-twice FAIL (2026-09-01: the re-widened tier scope makes the deadline a
// live path, and integrationTierTimeout became a var to make this testable —
// the same rationale as apicoverTimeout).
```

### `go/internal/phases/audit/probe_quarantine.go:18` — above `const auditProbesDir = "audit-probes"`

```text
// probe_quarantine.go — the observer must not perturb the observed system.
//
// The adversarial auditor legitimately authors probe tests to refute a build,
// but writing them INTO the package under test poisons the EGPS run: any
// predicate that shells `go test` over that package inherits the probe's
// engineered failure as a builder regression (cycles 1115/1117 — auditor
// verdict PASS/"Not FAIL" beside a red gate). The probes were then discarded
// with the worktree, so the auditor's own finding was unrecoverable.
//
// New-since-dispatch *_test.go files (the audit-prompt artifact's mtime is
// the dispatch anchor — engine.go writes it at dispatch; builder files
// predate it) are PRESERVED under <workspace>/audit-probes/ and removed from
// the tree, one loud log line each. Tracked files are never touched, and
// go/acs/ is exempt (deleting the cycle's predicate package would nuke the
// gate itself — an auditor edit there is a different violation with a
// different guard). The sanctioned probe idiom remains `go test -overlay`
// (cycle-1106), which never writes the tree.
//
// Known limitation (adversarial review H1): the anchor is AUDIT dispatch, so
// a probe the adversarial-review phase left behind earlier is classified as
// builder work. That phase deletes its probes per its own persona; the
// residual risk is documented there rather than guessed at here — a wrong
// guess would quarantine bug-reproduction's legitimate repro tests.
```

### `go/internal/phases/audit/probe_quarantine_test.go:3` — above `import (`

```text
// probe_quarantine_test.go — the observer must not perturb the observed
// system. The adversarial auditor legitimately writes probe tests to try to
// refute the build (cycles 1115/1117: TestZZAudit_QuoteSpillSuppressed,
// TestZZAuditProbe_BulletBannerSuppressed) — but it wrote them INTO the
// package under test, so the EGPS run inherited the probes' engineered
// failures as if the BUILDER had regressed a sibling: auditor-graded PASS
// alongside a red gate, a false cycle FAIL. The probes were then discarded
// with the worktree, leaving the finding unrecoverable.
//
// quarantineAuditProbes runs after the auditor agent and before acssuite.Run:
// untracked *_test.go files created SINCE the audit agent was dispatched
// (mtime after the audit-prompt artifact — builder files predate it) are
// PRESERVED into the run dir and REMOVED from the tree, loudly. The safe
// probe idiom remains `go test -overlay` (cycle-1106's PoCA/PoCB), which
// never touches the tree at all.
```

### `go/internal/phases/audit/repro_cycle1285_test.go:13` — above `const reproScopeID = "continuation-defect-ledger"`

```text
// repro_cycle1285_test.go — executable reproduction of the cycle-1285
// adversarial review's findings that live in this package (F2 HIGH, F3 MEDIUM,
// F5 MEDIUM). Every case drives the production seam `hooks.Classify`, never a
// helper: the committed suites are green precisely because none of them
// constructs these inputs.
//
// Shared shape of the two F2 cases: the whole reconcile gate's ARMING condition
// is `continuation.ReadManifest(req.Workspace)` — a plain 0644 file in the
// directory the graded phase agent writes. The gate is well defended against a
// forged disposition and undefended against never being asked to run.
//
// These tests are RED on the tree that produced them. They are not a style
// preference: each asserts the posture defect_ledger.go states for its OTHER
// inputs. A deleted ancestor ledger is already a loud warning because "one `rm`
// outside the workspace would otherwise disarm the whole gate in silence"
// (defect_ledger.go:336-344), and an unparseable defect-dispositions.json
// already BLOCKS because "degrading open there would hand the gate its cheapest
// bypass" (:458-460). The manifest — the input that decides whether the gate
// runs at all, and the one the agent writes most directly — gets neither.
```

### `go/internal/phases/audit/repro_cycle1285_test.go:33` — above `const reproScopeID = "continuation-defect-ledger"`

```text
// reproContinuationFixture is continuationFixture with two changes that matter
// to the arming question:
//
//  1. the current workspace sits at its REAL production location,
//     <root>/.evolve/runs/cycle-<n>, rather than an unrelated temp dir; and
//  2. the same lineage is additionally witnessed OUT OF BAND, in the root-owned
//     continuation registry (continuation.RegistryPath —
//     .evolve/continuation-registry.json, the scope-id-keyed map ADR-0076 slice
//     C G2 already writes at the preserve decision). That file is outside every
//     per-cycle workspace, so it still names this lane's ancestor after the
//     workspace manifest is deleted.
//
// The registry is what makes these cases a DEFECT rather than an over-strict
// test: "this cycle is a continuation of cycle-N" remains knowable from a
// non-workspace source, so a silent no-op is a choice the code makes, not a
// limit the environment imposes.
// reproScopeID is the lane's pinned todo id. It is the registry KEY and the
// lane-scope.json entry alike — one identity, two records, which is precisely
// the property the F2 fix depends on.
```

### `go/internal/phases/audit/repro_cycle1285_test.go:87` — above `writeJSON(t, filepath.Join(ws, core.LaneScopeFile), map[string]any{`

```text
// cycle-1287, adopting this reproducer into the tree: the fixture gained
// lane-scope.json, which it did not carry when it was written in the
// cycle-1285 workspace. This is an ADDITION of a production artifact, not a
// relaxation — core.materializeLaneScope pins this file into every run
// workspace BEFORE any phase runs, so a real graded audit always sees it,
// and the registry is keyed by exactly these todo ids. Without it the
// fixture asked the gate to recover a lineage from a root-owned map with no
// key to look under; arming on ANY registry entry instead would block every
// ordinary cycle in a project where any lane had ever preserved work, which
// is a worse defect than the one being fixed. Every assertion below is
// unchanged from the cycle-1285 original.
```

### `go/internal/phases/audit/repro_cycle1285_test.go:107` — above `func TestRepro1285_F2_ManifestDeletionSilentlyDisarmsTheReconcileGate(t *testing.T) {`

```text
// TestRepro1285_F2_ManifestDeletionSilentlyDisarmsTheReconcileGate — `rm` on one
// 0644 file in the graded agent's own workspace retires every inherited OPEN
// defect. reconcileContinuationDefects reads the manifest, gets
// (zero, false, nil), and returns (nil, false): PASS, and not one diagnostic.
//
// This is the same trust class as cycle-1282's DEF-1, which that audit graded
// CRITICAL — and strictly cheaper, because DEF-1 needed a well-formed planted
// ledger while this needs a delete. The root-owned registry written by the
// fixture still names cycle-1255 as this lane's ancestor, so the disarm is
// detectable; the code simply does not look.
```

### `go/internal/phases/audit/repro_cycle1285_test.go:210` — above `func TestRepro1285_F5_QuotedDefectTextIsNotAClosureClaim(t *testing.T) {`

```text
// TestRepro1285_F5_QuotedDefectTextIsNotAClosureClaim — closureClaimOffenders
// substring-matches "verified closed" per line with no notion of quoting or
// negation. The canonical inherited defect text in this repo literally contains
// the phrase (docs/operations/batch-integrity-review-2026-08-04.md: "the 1255-D1
// stale-worktree CRITICAL narrowed to 'verified closed'"), so an auditor who
// correctly reports that defect as STILL OPEN is blocked for accuracy.
//
// The second-order damage is worse than the availability hit: the cheapest way
// out is to append the literal string "defect-dispositions.json" to the line,
// which satisfies the gate and adds no evidence at all. A gate whose remedy is a
// one-token appeasement becomes noise, then gets disabled.
```

### `go/internal/phases/audit/solution_gate.go:8` — above `func solutionGate(spec config.DeliverableKindSpec) func(req core.PhaseRequest) ([]string, error) {`

```text
// solutionGate is the production solution-contract gate (ADR-0099 slice 2):
// for a document cycle it reports every contract violation the ONE engine
// finds over the bound tasks' deliverables — the same core.SolutionViolations
// the build floor runs, over the same registry spec the composition root hands
// both of them, so the two surfaces cannot disagree. Code cycles report
// nothing. The gate never loads config itself: the root owns that policy.
```

### `go/internal/phases/audit/tia_wiring_test.go:3` — above `import (`

```text
// tia_wiring_test.go — the REACHABILITY proof for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`).
//
// A seam whose only caller is a test is dead code. internal/regressiontia can
// be perfectly unit-tested and still never run in production, which is exactly
// the defect this cycle fixes: changedpkgs.ImporterClosure shipped GREEN in
// cycle-1253 with ZERO callers, so the reverse-dependency widening that would
// have caught the cycle-1250 router/routingtest miss never executed once.
//
// These tests therefore drive generateACSVerdict — the real audit-phase
// function that runs the EGPS suite (audit.go:638, calling acssuite.Run at
// :651) — and never call regressiontia directly. The shadow decision must be
// emitted from THAT path or not at all.
//
// Root is a bare temp dir with no go.mod, so acssuite's Go lane is a fast
// no-op (hasGoACSTree false → zero predicates → generateACSVerdict returns
// early without writing a verdict). The TIA emission must happen BEFORE that
// early return: the evidence is about which packages the cycle touched, not
// about whether the suite found predicates.
//
// RED today: internal/regressiontia does not exist and nothing in audit.go
// emits the artifact, so this file fails to COMPILE — a hard non-zero exit,
// never a silent pass.
```

### `go/internal/phases/audit/verdict_foreign_root_test.go:56` — above `candidates, err := filepath.Glob(filepath.Join(ws, "acs-verdict.candidate.*.json"))`

```text
// The foreign artifact is EVIDENCE — preserved, not clobbered (the
// incident class was "the misdiagnosis was invisible from the file").
```

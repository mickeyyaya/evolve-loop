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
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

// auditReportMaxBytes bounds the SIZE of audit-report.md, the same way
// defect_ledger.go's defectLedgerMaxEntries/defectTextMaxRunes bound the
// ledger: bound it, RECORD the overflow, never silently drop. The report is
// re-read in full at ship time and SHA-bound there
// (internal/phases/ship/audit.go), and the next cycle's handoff carries the
// prior audit forward, so an unbounded ## Issues table compounds token cost on
// every downstream read.
//
// Two properties are load-bearing. The overflow diagnostic is severity
// "warning", NEVER "error": core's errorSeverityMessages keys off
// Severity=="error" to build AuditFailReasons, so an error here would convert a
// merely verbose report into a dossier-visible failure. And the check never
// touches the file on disk — ship SHA-binds those exact bytes, so a truncating
// cap would break the integrity check it was meant to protect.
//
// 32KiB is ~1.5x the largest audit-report.md observed across 256 recorded runs
// (max 22,035 bytes, p90 15,777): high enough that a normal report never trips
// it, low enough to catch a runaway table.
const auditReportMaxBytes = 32 * 1024

// The regex-on-prose verdict fallback (canonical "## Verdict\n**PASS**" and
// the colon-bearing inline forms) is single-homed in reportdoc.Verdict since
// ADR-0095: the dashboard's round history and the repair-brief seed read the
// SAME grammar, so what the gate scores and what the operator/rebuilder sees
// cannot disagree. reportdoc scans visible lines only (fenced, indented and
// HTML-commented content stripped), so an embedded template can no longer
// declare a verdict here either.

// hooks carries injected verification seams. Production always executes the
// predicate generator and seals its evidence. A nil seam supports isolated tests.
type hooks struct {
	genVerdict        func(req core.PhaseRequest) error
	predicateEvidence func(core.PhaseRequest) (func() error, error)
	// explanationCheck independently verifies the Build-explanation handoff
	// after the auditor has finished. A failure overrides the native Audit
	// response without masquerading as an EGPS predicate. nil keeps legacy unit
	// fixtures unchanged.
	explanationCheck func(req core.PhaseRequest) error
	// gofmtCheck reports the worktree's .go files that are not gofmt -s clean.
	// It is the CI-parity gate that stops a cycle shipping a gofmt regression
	// to main (cycles 339-341 shipped CI-red because the cycle-scoped audit
	// never ran gofmt over the generated go/acs/cycle<N>/*.go files). nil = no
	// gofmt gate (legacy/tests). The registry default wires gofmtCheckDefault.
	gofmtCheck func(req core.PhaseRequest) ([]string, error)
	// solutionCheck reports document-cycle solution-contract violations
	// (ADR-0099 slice 2); nil = no gate.
	solutionCheck func(req core.PhaseRequest) ([]string, error)
	// skillsDriftCheck reports the worktree's SKILL.md files whose generated
	// phase-facts region has drifted from its SSOTs (profiles/registry/
	// phasecontract). A cycle that edits .evolve/profiles/*.json without
	// regenerating would FAIL the CI TestSkills_NoDrift gate (cycle 339), so the
	// drift must FAIL audit. nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault (in-process skillcheck.Check — no subprocess).
	skillsDriftCheck func(req core.PhaseRequest) ([]string, error)
	// goVetCheck / acsDurableCheck / apicoverEnforceCheck are the CI-parity
	// gates: each runs the EXACT whole-repo CI command (go vet ./..., -tags acs
	// acs-durable, apicover -enforce over touched-enforced packages) against the
	// cycle worktree and FAILs audit on offenders — closing the "per-cycle proof
	// ≠ repo-wide CI gate" gap (import cycles, flagregistry/flag-ceiling, unnamed
	// exports). nil = no gate (tests). NewDefaultWithStageCompact wires the
	// *Default impls (subprocess); each fails OPEN (warning) if it cannot run.
	goVetCheck           func(req core.PhaseRequest) ([]string, error)
	acsDurableCheck      func(req core.PhaseRequest) ([]string, error)
	apicoverEnforceCheck func(req core.PhaseRequest) ([]string, error)
	// integrationTierCheck runs the `-tags integration` test tier (go.yml's
	// "test … incl. integration tier" step) against the cycle worktree. It
	// closes the parity hole one tier up from goVetCheck: the fleet soak
	// (TestFleetSoak) went red in CI while the per-cycle audit stayed green
	// because ciparity ran vet/acs-durable/apicover but never the integration
	// tier. nil = no gate. NewDefaultWithStageCompact wires the *Default impl.
	integrationTierCheck func(req core.PhaseRequest) ([]string, error)
	// apicoverNewPkgGraduationCheck reports changed go/internal/<pkg> packages
	// that are new this cycle and absent from .apicover-enforce — the blind spot
	// IntersectEnforced silently drops (a new package cannot yet be in the
	// enforce list, so the touched∩enforced scoping never inspects it). Any such
	// ungraduated package FAILs audit fail-fast, closing the recurring
	// warnship_apicover_ci_gap. nil = no gate. NewDefaultWithStageCompact wires
	// apicoverNewPackageGraduationDefault.
	apicoverNewPkgGraduationCheck func(req core.PhaseRequest) ([]string, error)
	// phaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). At >= StageEnforce the evolve-verdict sentinel is mandatory —
	// the legacy prose/regex fallbacks are gated off. Zero value (StageOff) keeps
	// every path active, byte-identical.
	phaseIO config.Stage
}

func (hooks) PhaseName() string       { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string { return "evolve-auditor" }

// SecondaryArtifacts (runner.SecondaryArtifactsProvider): on a continuation
// cycle — the workspace carries the adopter-written continuation manifest —
// the audit contract also requires defect-dispositions.json (ADR-0074 /
// cycle-1285 lineage accounting). Declaring it holds session teardown until
// the auditor writes it, closing the write-one-artifact-and-die class that
// failed cycles 1397-1429. Non-continuation audits return nil: byte-identical
// legacy behavior.
func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {
	if _, err := os.Stat(filepath.Join(req.Workspace, "continuation-manifest.json")); err != nil {
		return nil
	}
	return []string{filepath.Join(req.Workspace, "defect-dispositions.json")}
}

func (hooks) ArtifactFilename(_ core.PhaseRequest) string {
	return phasecontract.ArtifactFilename(string(core.PhaseAudit))
}
func (hooks) DefaultModel() string { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	if contract := req.Context[core.CtxKeyTaskContract]; contract != "" {
		// Harness-owned (ADR-0098): the SAME acceptance words and predicate
		// inventory the builder was handed — the grader reads what the builder
		// read, so the block is an authority, not the builder's claim.
		fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)
	}
	// Continuations are TOLD their inherited OPEN defect ids (2026-08-10
	// investigation: auditors were graded against ids they were never shown).
	b.WriteString(inheritedDefectsPromptBlock(req))
	b.WriteString(chainExamplePromptBlock())
	return b.String()
}

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
func chainExamplePromptBlock() string {
	return "\n## Reasoning chain — emit exactly this shape (ADR-0088)\n\n" +
		"Your verdict is the CONCLUSION of this chain. Replace every status and finding with your own;\n" +
		"cite something a third party can open. A link you could not check is `unverifiable`, never\n" +
		"`coherent`. A link you omit is treated as decisive against the cycle.\n\n" +
		auditchain.ChainBlockExample + "\n"
}

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	classification := newAuditClassification(h, artifact, req)
	classification.prepareEvidence()
	classification.executeRepositoryGates()
	classification.reconcileContinuation()
	classification.finalize()

	return classification.verdict, classification.diagnostics, string(core.PhaseShip)
}

// extractAuditVerdict returns the declared verdict word and whether a
// parseable verdict declaration was found. It tries the canonical
// "## Verdict\n**PASS**" heading first, then common single-line variants. The
// found bool lets the caller distinguish "no verdict declared" (a format miss
// worth a loud diagnostic) from an explicit "FAIL". A real FAIL/WARN/SKIPPED
// declaration is captured verbatim, so broadening the accepted FORMATS never
// turns a real non-PASS verdict into a PASS.
func extractAuditVerdict(content string, stage config.Stage) (string, bool) {
	// Layer-5 strangler: the machine-readable sentinel wins when present; the
	// legacy regex-on-prose remains the fallback for reports written against the
	// older templates.
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
	// ADR-0050 §3.10 Slice 5: the regex-on-prose fallbacks serve reports written
	// against older templates; at enforce the sentinel above is mandatory, so gate
	// them off (>= StageEnforce). Below enforce they stay active — byte-identical.
	if stage < config.StageEnforce {
		if v := reportdoc.Verdict(content); v != "" {
			return v, true
		}
	}
	return "", false
}

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
func readACSVerdict(path string) (redCount int, redIDs, phantomBindings []string, shipEligible *bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, nil, nil, fmt.Errorf("read: %w", err)
	}
	var v struct {
		RedCount     int   `json:"red_count"`
		ShipEligible *bool `json:"ship_eligible"`
		Results      []struct {
			ACID            string   `json:"ac_id"`
			Result          string   `json:"result"`
			PhantomBindings []string `json:"phantom_bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return 0, nil, nil, nil, fmt.Errorf("parse: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range v.Results {
		if r.Result != "red" {
			continue
		}
		if r.ACID != "" {
			redIDs = append(redIDs, r.ACID)
		}
		// Phantom names are surfaced so the gate-block can carry the cure
		// (phantom_binding.go in acssuite). Deduped across results: two
		// predicates demanding the same renamed test is one repoint.
		for _, pb := range r.PhantomBindings {
			if pb != "" && !seen[pb] {
				seen[pb] = true
				phantomBindings = append(phantomBindings, pb)
			}
		}
	}
	return v.RedCount, redIDs, phantomBindings, v.ShipEligible, nil
}

// egpsRedIDCycleTokens strips the cycle-numbered chrome from an ACS ac_id:
// "cycle1115/TestC1115_003_BridgeAndRecoveryStayGreen" →
// "BridgeAndRecoveryStayGreen". The SEMANTIC name is the defect's stable
// identity; the cycle-numbered prefix changes on every retry, and embedding it
// would make a real cross-cycle recurrence never collide — blinding the
// identical-fingerprint breaker (the same "never cycle numbers" rule
// verdictFailDistinguisher documents). Both live naming conventions are
// covered by stripping the C-group and the index group INDEPENDENTLY
// (adversarial review caught `C\d+_\d+_` missing the two-part
// "TestC841_Amplify_…"/"TestC416_NEG_…" shape, which left C841_ — a cycle
// number — embedded): three-part TestC<cycle>_<index>_<Name> and two-part
// TestC<cycle>_<Name>. A name that merely starts with 'C'+letters
// ("TestCarryforward_…") is untouched — C\d+ requires digits.
var egpsRedIDCycleTokens = regexp.MustCompile(`^(?:cycle\d+/)?(?:Test)?(?:C\d+_)?(?:\d+_)?`)

// egpsRedMessage renders the one-line EGPS gate-block diagnostic. The red
// predicates' cycle-normalized semantic names (acssuite results order, at most
// maxNamedReds named) make the message — and therefore the failure
// fingerprint — distinct per defect, while the same defect red again on a
// retry cycle (new cycle-numbered ac_id, same semantic name) still collides
// exactly, so the breaker keeps catching real recurrences.
func egpsRedMessage(redCount int, redIDs, phantomBindings []string) string {
	const maxNamedReds = 5
	if len(redIDs) == 0 {
		return fmt.Sprintf("EGPS: red_count=%d (cycle ships only when red_count==0)", redCount) + phantomBindingClause(phantomBindings)
	}
	shown := make([]string, 0, len(redIDs))
	for _, id := range redIDs {
		if n := egpsRedIDCycleTokens.ReplaceAllString(id, ""); n != "" {
			shown = append(shown, n)
		} else {
			shown = append(shown, id) // degenerate id — keep verbatim over dropping
		}
	}
	suffix := ""
	if len(shown) > maxNamedReds {
		suffix = fmt.Sprintf(" +%d more", len(shown)-maxNamedReds)
		shown = shown[:maxNamedReds]
	}
	return fmt.Sprintf("EGPS: red_count=%d [%s%s] (cycle ships only when red_count==0)",
		redCount, strings.Join(shown, " "), suffix) + phantomBindingClause(phantomBindings)
}

// phantomBindingClause renders the actionable half of a phantom-binding red, or
// "" when there are none — the phantom-free message stays byte-identical
// (pinned), so every existing fingerprint and log grep is untouched.
//
// The clause states diagnosis, cure, AND the anti-gaming boundary in the
// directive itself: the reader of this line is usually an agent (retro, a
// continuation builder, a correction round), and an agent told only "test
// missing" reliably reaches for the cheapest green — deleting the predicate —
// which is the exact gaming vector red-on-missing exists to block. The
// 1539-1546 streak burned five cycles on a red this one line would have cured.
func phantomBindingClause(phantomBindings []string) string {
	if len(phantomBindings) == 0 {
		return ""
	}
	return fmt.Sprintf("; PHANTOM binding(s) [%s]: the bound test name does not resolve in its target package (renamed or never created) — repoint the predicate's binding to the real test name or restore the name; do NOT delete the predicate",
		strings.Join(phantomBindings, " "))
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// GenerateVerdict, when set, produces <workspace>/acs-verdict.json from
	// the cycle's ACS predicates on every classification. Candidates never
	// suppress execution. A nil generator supports isolated phase tests.
	// The registry default wires generateACSVerdict (runs acssuite).
	GenerateVerdict func(req core.PhaseRequest) error
	// BeginPredicateEvidence captures the host tree before verification and
	// returns its final evidence sealer. Production always injects this hook.
	BeginPredicateEvidence func(core.PhaseRequest) (func() error, error)
	// CheckExplanation binds explanationdocs.Verify into the native Audit
	// override. nil disables only this injected seam (tests); production defaults
	// always wire verifyExplanationDocumentation.
	CheckExplanation func(req core.PhaseRequest) error
	// CheckGofmt, when set, reports the worktree .go files that are not
	// gofmt -s clean; any offender FAILs the audit (CI-parity gate). nil = no
	// gofmt gate. NewDefault wires gofmtCheckDefault.
	CheckGofmt func(req core.PhaseRequest) ([]string, error)
	// CheckSolution, when set, reports a document cycle's solution-contract
	// violations (internal/solutioncheck over solutions/<slug>/ for every bound
	// task); any violation FAILs the audit — the same deterministic-gate shape
	// as gofmt (ADR-0099 slice 2). nil = no gate. NewDefault wires
	// solutionCheckDefault, which is silent for code cycles.
	CheckSolution func(req core.PhaseRequest) ([]string, error)
	// SolutionSpec is the registry's document deliverable contract, handed in
	// by the composition root (the SAME spec the build floor runs). When set
	// and CheckSolution is nil, New wires the production gate over it; nil ⇒
	// the registry declares no document contract ⇒ no gate.
	SolutionSpec *config.DeliverableKindSpec
	// CheckSkillsDrift, when set, reports the worktree SKILL.md files whose
	// phase-facts region drifted from its SSOTs; any drift FAILs the audit
	// (CI TestSkills_NoDrift parity). nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault.
	CheckSkillsDrift func(req core.PhaseRequest) ([]string, error)
	// CheckGoVet / CheckACSDurable / CheckApicoverEnforce are the CI-parity gates
	// (whole-repo go vet ./..., -tags acs acs-durable, apicover -enforce over
	// touched packages); any offender FAILs the audit. nil = no gate.
	// NewDefaultWithStageCompact wires the *Default impls.
	CheckGoVet           func(req core.PhaseRequest) ([]string, error)
	CheckACSDurable      func(req core.PhaseRequest) ([]string, error)
	CheckApicoverEnforce func(req core.PhaseRequest) ([]string, error)
	// CheckIntegrationTier runs the `-tags integration` test tier over the cycle
	// worktree; any offender FAILs the audit (the tier that let TestFleetSoak go
	// CI-red under a green per-cycle audit). nil = no gate.
	// NewDefaultWithStageCompact wires integrationTierCheckDefault.
	CheckIntegrationTier func(req core.PhaseRequest) ([]string, error)
	// CheckApicoverNewPkgGraduation is the new-package graduation gate: it FAILs
	// audit when a changed go/internal/<pkg> is new this cycle and absent from
	// .apicover-enforce (the blind spot apicover -enforce's touched∩enforced
	// scoping drops). nil = no gate. NewDefaultWithStageCompact wires
	// apicoverNewPackageGraduationDefault.
	CheckApicoverNewPkgGraduation func(req core.PhaseRequest) ([]string, error)
	// PhaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). Zero value (StageOff) = byte-identical (prose fallbacks active).
	PhaseIO config.Stage
	// CompactPrompts strips the on-demand reference tail from the disk-loaded agent
	// doc before dispatch. Value flows from workflow.compact_prompts (policy.json);
	// never set to a bare literal here (standing rule: phase-settings-from-config).
	CompactPrompts bool
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	if c.CheckSolution == nil && c.SolutionSpec != nil {
		c.CheckSolution = solutionGate(*c.SolutionSpec)
	}
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks: hooks{
				genVerdict:                    c.GenerateVerdict,
				predicateEvidence:             c.BeginPredicateEvidence,
				explanationCheck:              c.CheckExplanation,
				gofmtCheck:                    c.CheckGofmt,
				solutionCheck:                 c.CheckSolution,
				skillsDriftCheck:              c.CheckSkillsDrift,
				goVetCheck:                    c.CheckGoVet,
				acsDurableCheck:               c.CheckACSDurable,
				integrationTierCheck:          c.CheckIntegrationTier,
				apicoverEnforceCheck:          c.CheckApicoverEnforce,
				apicoverNewPkgGraduationCheck: c.CheckApicoverNewPkgGraduation,
				phaseIO:                       c.PhaseIO,
			},
			Bridge:         c.Bridge,
			Prompts:        c.Prompts,
			NowFn:          c.NowFn,
			CompactPrompts: c.CompactPrompts,
		}),
	}
}

// NewDefault builds the audit phase with production defaults — notably
// GenerateVerdict and host evidence capture wired together: every Audit runs
// the real suite and seals its complete result before the ledger binds it.
// BOTH the registry init() and the loop's runner map (go/cmd/evolve/cmd_cycle.go)
// MUST construct audit via this single seam so the generator can never again be
// wired in one phase-construction path but dormant in the other — the
// dual-source divergence that left the loop force-FAILing on a missing verdict
// every cycle (cycle-147). New(Config) stays for tests that pin explicit
// (nil or fake) generators.
func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {
	return NewDefaultWithStage(br, prm, config.StageOff)
}

// NewDefaultWithStage is NewDefault plus the EVOLVE_PHASE_IO stage (ADR-0050 §3.10
// Slice 5). The composition root (cmd_cycle.go) passes cfg.PhaseIO so the audit
// verdict extraction enforces the sentinel at >= StageEnforce. NewDefault stays as
// the StageOff (byte-identical) convenience for the registry init() and tests.
func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {
	return NewDefaultWithStageCompact(br, prm, stage, false)
}

// NewDefaultWithStageCompact is NewDefaultWithStage plus the compact-prompts flag
// (workflow.compact_prompts). Called from cmd_cycle.go with wfCfg.CompactPrompts so
// the reference tail is stripped before dispatch when the policy default is on.
func NewDefaultWithStageCompact(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool) *Phase {
	return NewDefaultWithStageCompactSpec(br, prm, stage, compact, nil)
}

// NewDefaultWithStageCompactSpec is NewDefaultWithStageCompact plus the
// registry's document deliverable contract (ADR-0099 slice 2), which the
// composition root resolves ONCE and hands to both the build floor and this
// audit gate. nil ⇒ no document contract ⇒ no solution gate.
func NewDefaultWithStageCompactSpec(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool, spec *config.DeliverableKindSpec) *Phase {
	return New(Config{
		SolutionSpec:                  spec,
		Bridge:                        br,
		Prompts:                       prm,
		GenerateVerdict:               generateACSVerdict,
		BeginPredicateEvidence:        beginPredicateEvidence,
		CheckExplanation:              verifyExplanationDocumentation,
		CheckGofmt:                    gofmtCheckDefault,
		CheckSkillsDrift:              skillsDriftCheckDefault,
		CheckGoVet:                    goVetCheckDefault,
		CheckACSDurable:               acsDurableCheckDefault,
		CheckIntegrationTier:          integrationTierCheckDefault,
		CheckApicoverEnforce:          apicoverEnforceChangedDefault,
		CheckApicoverNewPkgGraduation: apicoverNewPackageGraduationDefault,
		PhaseIO:                       stage,
		CompactPrompts:                compact,
	})
}

// verifyExplanationDocumentation is a native host gate, deliberately separate
// from acs-verdict.json: ACS results are execution-grounded Go predicates, and
// this structural verifier must not impersonate one. Classify turns its error
// into a named deterministic Audit override; Ship independently re-verifies.
func verifyExplanationDocumentation(req core.PhaseRequest) error {
	binding := explanationdocs.CycleBinding{
		ProjectRoot:     req.ProjectRoot,
		Worktree:        req.Worktree,
		Workspace:       req.Workspace,
		BaseSHA:         req.WorktreeBaseSHA,
		Cycle:           req.Cycle,
		RunID:           req.RunID,
		ContractVersion: req.ExplanationDocumentationVersion,
	}
	// The activation belt (single home: explanationdocs, shared with ship —
	// architecture review 2026-09-01). Before this, a dropped/zero
	// ContractVersion silently disabled the whole gate: Verify resolved
	// inactive and audit returned nil. Now a zero version against an ACTIVE
	// host activation, or a live version with no host activation, fails the
	// audit loudly; a nil return below means the host AGREES nothing applies.
	hostActive, beltErr := explanationdocs.CrossCheckActivation(binding)
	if beltErr != nil {
		return beltErr
	}
	if !hostActive {
		return nil
	}
	verified, active, verifyErr := explanationdocs.Verify(context.Background(), binding)
	if verifyErr != nil {
		return verifyErr
	}
	if !active {
		return nil
	}
	if !explanationdocs.SameView(req.BuildExplanation, verified) {
		return fmt.Errorf("typed Build explanation handoff does not match the verified host snapshot")
	}
	return nil
}

// skillsDriftCheckDefault is the production SKILL.md-drift gate: it runs the
// same projection check the CI TestSkills_NoDrift gate runs (skillcheck.Check)
// against the cycle worktree (where the builder's profile/SSOT edits live),
// in-process — no subprocess, so no fork-bomb under `go test`. The worktree is
// preferred; ProjectRoot is the fallback; an empty root is a no-op.
func skillsDriftCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	return skillcheck.Check(root)
}

// gofmtCheckDefault is the production gofmt CI-parity gate: it lists the .go
// files under the cycle worktree's go/ module that are not gofmt -s clean,
// matching CI's `gofmt -d -s .`. The worktree (where the builder wrote this
// cycle's changes) is preferred; ProjectRoot is the fallback. When no go/
// module exists the gate is a no-op (returns nil) rather than an error.
func gofmtCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	// ModuleDir is the single source for "where the .go files live", shared with
	// the post-build gofmt normalizer (normalizeBuildGofmt) so the gate and the
	// normalizer can never disagree on which tree to verify vs. format.
	return codequality.UnformattedGoFiles(codequality.ModuleDir(root))
}

// generateACSVerdict runs the ACS predicate suite for req.Cycle and writes
// <workspace>/acs-verdict.json. It runs the predicates discovered under the
// cycle's worktree (where this cycle's acs/cycle-N/*.sh live), falling back
// to the project root. When the suite discovers ZERO predicates it writes
// NOTHING — the audit's missing-file FAIL floor then holds, so a cycle with
// no predicates cannot auto-pass. EvolveDir is derived from the workspace
// (<evolveDir>/runs/cycle-N), matching where audit reads the verdict.
func generateACSVerdict(req core.PhaseRequest) error {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	// Regression test-impact evidence, BEFORE the suite runs. It is about which
	// packages this cycle touched, not about what the suite found, so it must
	// not sit behind the zero-predicate early return below.
	emitTIADecision(req, root)
	// Probe quarantine runs in Classify before host execution, including
	// configurations with an injected generator.
	// Discover predicate FILES from the worktree (Root), but resolve `.evolve/`
	// runtime data (history, baselines, current build-report) to the MAIN project
	// root via EVOLVE_PROJECT_ROOT — those live in main, not the worktree, so a
	// suite run from the worktree (issue #9 audit-cwd=worktree) would else false-RED
	// every regression predicate that reads .evolve/ (issue #12, cycle-177).
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return fmt.Errorf("acssuite run: %w", err)
	}
	if v.PredicateSuite.Total == 0 {
		// No predicates → leave the file absent so the EGPS floor fails the
		// cycle rather than auto-passing it on an empty suite.
		return nil
	}
	// evolveDir = parent of runs/, i.e. dirname(dirname(workspace)).
	evolveDir := filepath.Dir(filepath.Dir(req.Workspace))
	if _, err := acssuite.WriteVerdict(evolveDir, v); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

// emitTIADecision is the production caller for regression test-impact
// selection: it resolves the staged rollout from .evolve/policy.json, computes
// the decision over this cycle's changed packages, and drops it in the cycle
// workspace as evidence.
//
// Two bounds hold it inside the observability lane. It NEVER changes what
// acssuite runs — at the live "off" default (the checked-in policy.json has no
// regression_tia block) it returns before touching git, so this path is
// byte-identical to its pre-change self. And a failure to write the evidence is
// deliberately swallowed: shadow TIA runs on the path that grades every cycle,
// so a broken evidence sink must degrade quietly rather than turn a healthy
// audit into an error. Observability may never gate the gate.
func emitTIADecision(req core.PhaseRequest, root string) {
	stage := policy.RegressionTIAStageFor(req.ProjectRoot)
	if stage == "off" {
		return
	}
	// An underivable changed set (no repo, git error, concurrent-fleet index
	// lock) must not be read as "nothing changed" — that is the direction that
	// hides a regression class — so it degrades to an empty scope, which the
	// selector treats as UNKNOWN impact and skips nothing for.
	changed, derivable := changedpkgs.FromGitChecked(root, "HEAD")
	if !derivable {
		changed = nil
	}
	d := regressiontia.Compute(stage, root, codequality.ModuleDir(root), changed)
	if d.Stage == "" {
		return
	}
	_, _ = regressiontia.Emit(req.Workspace, d)
}

// verdictConflictMessage renders the verdict-conflict record — the one place
// its format lives. Consumers key on the prefix: core.normalizeReasonForFingerprint
// stabilizes the `narrative=<verdict>` token, and the bookkeeping-regrade
// classifier (core.BookkeepingConflictAuditReason) matches the record's head;
// bookkeeping_reason_singlesource_test.go pins this producer to that matcher.
func verdictConflictMessage(narrative string, overrodeBy []string) string {
	return fmt.Sprintf("verdict-conflict: auditor narrative=%s but %d deterministic gate(s) forced FAIL [%s] — "+
		"the gate outranks the narrative (ship policy unchanged); both readings are recorded so the "+
		"disagreement is weighable. Gate detail is in the error diagnostics beside this one.",
		narrative, len(overrodeBy), strings.Join(overrodeBy, ", "))
}

func init() {
	registry.Register(string(core.PhaseAudit), func(req core.PhaseRequest) core.PhaseRunner {
		return NewDefault(bridge.NewDefault(req.ProjectRoot), prompts.NewForProject(req.ProjectRoot))
	})
}

// recordChainShadow writes the ADR-0088 chain-versus-narrative comparison into
// the phase workspace. Best-effort and silent on failure by design: a shadow
// measurement must never influence, delay, or brick the decision it is
// measuring — the same posture the bad_verdict instrumentation ships with.
//
// The evidence set is what is actually ON DISK in the workspace, not what the
// prompt claimed: the question the record answers is whether the judge COULD
// have walked the chain, and a file the dispatch mentioned but never produced
// would make that answer a lie.
func recordChainShadow(artifact string, req core.PhaseRequest, narrative, shipped string, overrodeBy []string) {
	if req.Workspace == "" {
		return
	}
	var given []string
	for _, a := range auditchain.RequiredEvidence(string(core.PhaseAudit)) {
		if _, err := os.Stat(filepath.Join(req.Workspace, a)); err == nil {
			given = append(given, a)
		}
	}
	rec := auditchain.Shadow(req.Cycle, string(core.PhaseAudit), artifact, narrative, given)
	rec.ShippedVerdict = shipped
	rec.OverrodeBy = overrodeBy
	_ = atomicwrite.JSON(filepath.Join(req.Workspace, auditchain.ShadowRecordFile), rec)
}

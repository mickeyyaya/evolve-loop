// Package audit implements the EGPS gate phase: PASS requires a parseable
// verdict in audit-report.md and red_count == 0 in acs-verdict.json.
// See docs/architecture/packages/internal-phases-audit.md.
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
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

// auditReportMaxBytes bounds audit-report.md the way defect_ledger.go bounds
// the ledger: bound it, record the overflow, never silently drop.
const auditReportMaxBytes = 32 * 1024

// hooks carries injected verification seams; a nil seam supports isolated tests.
type hooks struct {
	genVerdict        func(req core.PhaseRequest) error
	predicateEvidence func(core.PhaseRequest) (func() error, error)
	// explanationCheck overrides the native Audit response on a failed
	// Build-explanation handoff, without masquerading as an EGPS predicate.
	explanationCheck func(req core.PhaseRequest) error
	// gofmtCheck reports the worktree's .go files that are not gofmt -s clean.
	// nil = no gofmt gate. The registry default wires gofmtCheckDefault.
	gofmtCheck func(req core.PhaseRequest) ([]string, error)
	// solutionCheck reports document-cycle solution-contract violations
	// (ADR-0099 slice 2); nil = no gate.
	solutionCheck func(req core.PhaseRequest) ([]string, error)
	// skillsDriftCheck reports the worktree's SKILL.md files whose generated
	// phase-facts region has drifted from its SSOTs. nil = no skills gate.
	// NewDefault wires skillsDriftCheckDefault (in-process skillcheck.Check).
	skillsDriftCheck func(req core.PhaseRequest) ([]string, error)
	// goVetCheck / acsDurableCheck / apicoverEnforceCheck are the CI-parity
	// gates: each runs the whole-repo CI command against the cycle worktree and
	// FAILs audit on offenders. nil = no gate (tests). NewDefaultWithStageCompact
	// wires the *Default impls.
	goVetCheck           func(req core.PhaseRequest) ([]string, error)
	acsDurableCheck      func(req core.PhaseRequest) ([]string, error)
	apicoverEnforceCheck func(req core.PhaseRequest) ([]string, error)
	// integrationTierCheck runs the `-tags integration` test tier against the
	// cycle worktree. nil = no gate. NewDefaultWithStageCompact wires the
	// *Default impl.
	integrationTierCheck func(req core.PhaseRequest) ([]string, error)
	// apicoverNewPkgGraduationCheck reports changed go/internal/<pkg> packages
	// that are new this cycle and absent from .apicover-enforce. nil = no gate.
	// NewDefaultWithStageCompact wires apicoverNewPackageGraduationDefault.
	apicoverNewPkgGraduationCheck func(req core.PhaseRequest) ([]string, error)
	// phaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction. At
	// >= StageEnforce the evolve-verdict sentinel is mandatory and the prose
	// fallbacks are gated off. Zero value (StageOff) keeps every path active.
	phaseIO config.Stage
	// ledger is the defect ledger; nil (a hooks{} literal) resolves to the
	// Null Object through defectLedger().
	ledger *defectledger.Ledger
}

func (hooks) PhaseName() string       { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string { return "evolve-auditor" }

// SecondaryArtifacts (runner.SecondaryArtifactsProvider): on a continuation
// cycle — the workspace carries the adopter-written continuation manifest —
// the audit contract also requires defect-dispositions.json. Declaring it
// holds session teardown until the auditor writes it. Non-continuation
// audits return nil.
// See ADR-0074.
func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {
	if _, err := os.Stat(filepath.Join(req.Workspace, continuation.ManifestName)); err != nil {
		return nil
	}
	return []string{filepath.Join(req.Workspace, defectDispositionFile)}
}

func (hooks) ArtifactFilename(_ core.PhaseRequest) string {
	return phasecontract.ArtifactFilename(string(core.PhaseAudit))
}
func (hooks) DefaultModel() string { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (h hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	if contract := req.Context[core.CtxKeyTaskContract]; contract != "" {
		// The same contract the builder was handed, so the grader reads an
		// authority rather than the builder's own claim.
		// See ADR-0098.
		fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)
	}
	// Continuations are told their inherited OPEN defect ids, so the auditor
	// is graded only on ids it was actually shown.
	b.WriteString(inheritedDefectsPromptBlockVia(h.defectLedger(), req))
	b.WriteString(chainExamplePromptBlock())
	return b.String()
}

// chainExamplePromptBlock shows the auditor the reasoning-chain shape instead
// of only describing it in prose, whose line budget cannot fit the example.
// See ADR-0088, ADR-0084.
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
// parseable declaration was found, distinguishing "no verdict declared" from
// an explicit FAIL/WARN/SKIPPED.
func extractAuditVerdict(content string, stage config.Stage) (string, bool) {
	// The machine-readable sentinel wins when present; the regex-on-prose
	// fallback serves reports written against older templates.
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
	// At >= StageEnforce the sentinel is mandatory and the fallback is gated
	// off; below it, the fallback stays active.
	// See ADR-0050.
	if stage < config.StageEnforce {
		if v := reportdoc.Verdict(content); v != "" {
			return v, true
		}
	}
	return "", false
}

type acsGateReading struct {
	redCount        int
	redIDs          []string
	phantomBindings []string
	harnessReds     []string
	shipEligible    *bool
}

// readACSVerdict reads the EGPS gate fields from acs-verdict.json. shipEligible
// is a *bool so the caller can distinguish an absent field from an explicit
// false. redIDs name the red predicates so the failure-digest fingerprint
// carries the defect's identity rather than a generic count.
func readACSVerdict(path string) (acsGateReading, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return acsGateReading{}, fmt.Errorf("read: %w", err)
	}
	var v struct {
		RedCount     int   `json:"red_count"`
		ShipEligible *bool `json:"ship_eligible"`
		Results      []struct {
			ACID            string   `json:"ac_id"`
			Result          string   `json:"result"`
			EvidenceExcerpt string   `json:"evidence_excerpt"`
			PhantomBindings []string `json:"phantom_bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return acsGateReading{}, fmt.Errorf("parse: %w", err)
	}
	g := acsGateReading{redCount: v.RedCount, shipEligible: v.ShipEligible}
	seen := map[string]bool{}
	for _, r := range v.Results {
		if r.Result != "red" {
			continue
		}
		if r.ACID != "" {
			g.redIDs = append(g.redIDs, r.ACID)
		}
		if strings.HasPrefix(r.ACID, acsverdict.SyntheticRedPrefix) {
			g.harnessReds = append(g.harnessReds, r.ACID+": "+r.EvidenceExcerpt)
		}
		// Phantom names are surfaced, deduped, so the gate-block can carry the cure.
		for _, pb := range r.PhantomBindings {
			if pb != "" && !seen[pb] {
				seen[pb] = true
				g.phantomBindings = append(g.phantomBindings, pb)
			}
		}
	}
	return g, nil
}

// egpsRedIDCycleTokens strips the cycle-numbered chrome from an ACS ac_id,
// e.g. "cycle1115/TestC1115_003_BridgeAndRecoveryStayGreen" →
// "BridgeAndRecoveryStayGreen", so the same defect red on a later cycle
// collides on its stable semantic name rather than a new cycle-numbered id.
var egpsRedIDCycleTokens = regexp.MustCompile(`^(?:cycle\d+/)?(?:Test)?(?:C\d+_)?(?:\d+_)?`)

// egpsRedMessage renders the one-line EGPS gate-block diagnostic, naming the
// cycle-normalized red predicates (at most maxNamedReds) so the failure
// fingerprint is distinct per defect while still colliding on a recurrence.
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

// phantomBindingClause renders the actionable half of a phantom-binding red,
// or "" when there are none, so the phantom-free message stays byte-identical.
// It states diagnosis, cure and the anti-gaming boundary together, because an
// agent told only "test missing" reliably reaches for the cheapest green:
// deleting the predicate.
func phantomBindingClause(phantomBindings []string) string {
	if len(phantomBindings) == 0 {
		return ""
	}
	return fmt.Sprintf("; PHANTOM binding(s) [%s]: the bound test name does not resolve in its target package (renamed or never created) — repoint the predicate's binding to the real test name or restore the name; do NOT delete the predicate",
		strings.Join(phantomBindings, " "))
}

func harnessRedClause(harnessReds []string) string {
	if len(harnessReds) == 0 {
		return ""
	}
	clause := fmt.Sprintf("; %d red(s) are the harness's own: predicates that could not run, first %s", len(harnessReds), harnessReds[0])
	if len(harnessReds) > 1 {
		clause += fmt.Sprintf(" (+%d more in acs-verdict.json)", len(harnessReds)-1)
	}
	return clause
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	// ContractVerifier hands the verdict engine the deliverables gate's own
	// verifier, so gate and engine agree on one verification. nil = the
	// catalog-aware default.
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
	// GenerateVerdict, when set, produces <workspace>/acs-verdict.json from
	// the cycle's ACS predicates on every classification. A nil generator
	// supports isolated phase tests. The registry default wires
	// generateACSVerdict (runs acssuite).
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
	// violations; any violation FAILs the audit, the same deterministic-gate
	// shape as gofmt. nil = no gate.
	// See ADR-0099.
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
	// CheckGoVet / CheckACSDurable / CheckApicoverEnforce are the CI-parity
	// gates: each runs the whole-repo CI command over touched packages; any
	// offender FAILs the audit. nil = no gate. NewDefaultWithStageCompact wires
	// the *Default impls.
	CheckGoVet           func(req core.PhaseRequest) ([]string, error)
	CheckACSDurable      func(req core.PhaseRequest) ([]string, error)
	CheckApicoverEnforce func(req core.PhaseRequest) ([]string, error)
	// CheckIntegrationTier runs the `-tags integration` test tier over the cycle
	// worktree; any offender FAILs the audit. nil = no gate.
	// NewDefaultWithStageCompact wires integrationTierCheckDefault.
	CheckIntegrationTier func(req core.PhaseRequest) ([]string, error)
	// CheckApicoverNewPkgGraduation is the new-package graduation gate: it FAILs
	// audit when a changed go/internal/<pkg> is new this cycle and absent from
	// .apicover-enforce. nil = no gate. NewDefaultWithStageCompact wires
	// apicoverNewPackageGraduationDefault.
	CheckApicoverNewPkgGraduation func(req core.PhaseRequest) ([]string, error)
	// PhaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction. Zero
	// value (StageOff) keeps every path active.
	// See ADR-0050.
	PhaseIO config.Stage
	// CompactPrompts strips the on-demand reference tail from the disk-loaded
	// agent doc before dispatch. Value flows from workflow.compact_prompts
	// (policy.json).
	CompactPrompts bool
	// Signals is the accessor of the root's Signal Center the defect ledger and
	// the CI-parity gates report through, read at every use.
	// See ADR-0103.
	// nil = the Null Object: the registry root (evolve phase
	// audit), New(Config{}) and the tests emit nothing; the loop root passes
	// WithSignals.
	Signals func() *signalcenter.Center
}

// Option configures the production Config before the phase is built.
type Option func(*Config)

// WithSignals installs the Signal Center accessor the defect ledger and the
// CI-parity gates report through.
// WithContractVerifier hands the audit runner the deliverables gate's
// verifier accessor: one verifier for gate and engine.
func WithContractVerifier(fn func() runner.ContractVerifier) Option {
	return func(c *Config) { c.ContractVerifier = fn }
}

// WithHostEffects hands the audit runner the host's effect performer.
func WithHostEffects(fn func() core.HostEffects) Option {
	return func(c *Config) { c.HostEffects = fn }
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(cfg *Config) { cfg.Signals = c }
}

type Phase struct {
	*runner.BaseRunner
	signals func() *signalcenter.Center // the CI-parity gates' Center accessor
}

// SignalsWired reports whether the CI-parity gates of this phase reach a
// Signal Center — the roots' wiring proof, through the leaf.
func (p *Phase) SignalsWired() bool {
	return ciParity{signals: p.signals}.wiredGates().SignalsWired()
}

func New(c Config) *Phase {
	return &Phase{
		signals: c.Signals,
		BaseRunner: runner.New(runner.Options{
			Hooks:            newHooks(c),
			Bridge:           c.Bridge,
			ContractVerifier: c.ContractVerifier,
			HostEffects:      c.HostEffects,
			Prompts:          c.Prompts,
			NowFn:            c.NowFn,
			CompactPrompts:   c.CompactPrompts,
		}),
	}
}

// newHooks is the one hooks literal: the injected seams from Config and the
// defect ledger wired to Config's Center.
func newHooks(c Config) hooks {
	if c.CheckSolution == nil && c.SolutionSpec != nil {
		c.CheckSolution = solutionGate(*c.SolutionSpec)
	}
	return hooks{
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
		ledger:                        wiredDefectLedger(c.Signals),
	}
}

// NewDefault builds the audit phase with production defaults: GenerateVerdict
// and host evidence capture wired together, so every Audit runs the real
// suite and seals its complete result before the ledger binds it. The
// registry init() and the loop's runner map must both construct audit through
// this one seam. New(Config) stays for tests that pin explicit generators.
func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {
	return NewDefaultWithStage(br, prm, config.StageOff)
}

// NewDefaultWithStage is NewDefault plus the EVOLVE_PHASE_IO stage. NewDefault
// stays as the StageOff convenience for the registry init() and tests.
func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {
	return NewDefaultWithStageCompact(br, prm, stage, false)
}

// NewDefaultWithStageCompact is NewDefaultWithStage plus the compact-prompts
// flag (workflow.compact_prompts).
func NewDefaultWithStageCompact(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool) *Phase {
	return NewDefaultWithStageCompactSpec(br, prm, stage, compact, nil)
}

// NewDefaultWithStageCompactSpec is NewDefaultWithStageCompact plus the
// registry's document deliverable contract, which the composition root
// resolves once and hands to both the build floor and this audit gate. nil
// means no document contract and no solution gate.
func NewDefaultWithStageCompactSpec(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool, spec *config.DeliverableKindSpec, opts ...Option) *Phase {
	cfg := Config{
		SolutionSpec:           spec,
		Bridge:                 br,
		Prompts:                prm,
		GenerateVerdict:        generateACSVerdict,
		BeginPredicateEvidence: beginPredicateEvidence,
		CheckExplanation:       verifyExplanationDocumentation,
		CheckGofmt:             gofmtCheckDefault,
		CheckSkillsDrift:       skillsDriftCheckDefault,
		PhaseIO:                stage,
		CompactPrompts:         compact,
	}
	for _, o := range opts {
		o(&cfg)
	}
	// Wires the CI-parity gates through the seam's per-Phase adapter, filling
	// only the hooks no Option set, so an Option over Config is honoured in full.
	ciParity{signals: cfg.Signals}.wire(&cfg)
	return New(cfg)
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
	// A zero ContractVersion against an active host activation, or a live
	// version with no host activation, fails the audit loudly; a nil return
	// below means the host agrees nothing applies.
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

// skillsDriftCheckDefault is the production SKILL.md-drift gate: it runs
// skillcheck.Check in-process, never a subprocess, against the cycle
// worktree, falling back to ProjectRoot; an empty root is a no-op.
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
// matching CI's `gofmt -d -s .`. The worktree is preferred, ProjectRoot is the
// fallback, and a missing go/ module is a no-op rather than an error.
func gofmtCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	// ModuleDir is shared with the post-build gofmt normalizer, so the gate and
	// the normalizer never disagree on which tree to verify vs. format.
	return codequality.UnformattedGoFiles(codequality.ModuleDir(root))
}

func generateACSVerdict(req core.PhaseRequest) error {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	emitTIADecision(req, root)
	// Predicate files are discovered under the worktree, but `.evolve/` runtime
	// data resolves to the main project root: it lives in main, not the
	// worktree, and a suite run from the worktree would otherwise false-RED
	// every regression predicate that reads `.evolve/`.
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return fmt.Errorf("acssuite run: %w", err)
	}
	// evolveDir = parent of runs/, i.e. dirname(dirname(workspace)).
	evolveDir := filepath.Dir(filepath.Dir(req.Workspace))
	if _, err := acssuite.WriteVerdict(evolveDir, v); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

// emitTIADecision resolves the staged rollout from .evolve/policy.json,
// computes the regression test-impact decision over this cycle's changed
// packages, and drops it in the cycle workspace as evidence. It never changes
// what acssuite runs, and a failure to write the evidence is deliberately
// swallowed: observability may never gate the gate.
func emitTIADecision(req core.PhaseRequest, root string) {
	stage := policy.RegressionTIAStageFor(req.ProjectRoot)
	if stage == "off" {
		return
	}
	// An underivable changed set must not be read as "nothing changed", which
	// would hide a regression class; it degrades to an empty scope, which the
	// selector treats as unknown impact and skips nothing for.
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

func verdictConflictMessage(narrative string, overrodeBy []string) string {
	return fmt.Sprintf("verdict-conflict: auditor narrative=%s but %d deterministic gate(s) forced FAIL [%s] — "+
		"the gate outranks the narrative (ship policy unchanged); both readings are recorded so the "+
		"disagreement is weighable. Gate detail is in the error diagnostics beside this one.",
		narrative, len(overrodeBy), strings.Join(overrodeBy, ", "))
}

func init() {
	registry.Register(string(core.PhaseAudit), func(req core.PhaseRequest) core.PhaseRunner {
		return NewDefault(bridge.NewDefault(req.ProjectRoot, nil), prompts.NewForProject(req.ProjectRoot))
	})
}

// recordChainShadow writes the chain-versus-narrative comparison into the
// phase workspace. It is best-effort and silent on failure: a shadow
// measurement must never influence, delay, or brick the decision it is
// measuring.
// See ADR-0088.
//
// The evidence set is what is actually on disk in the workspace, not what the
// prompt claimed, so a file the dispatch mentioned but never produced does not
// count as evidence the judge could have used.
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

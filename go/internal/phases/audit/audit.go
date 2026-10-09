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
)

const auditReportMaxBytes = 32 * 1024

type hooks struct {
	genVerdict                    func(req core.PhaseRequest) error
	predicateEvidence             func(core.PhaseRequest) (func() error, error)
	explanationCheck              func(req core.PhaseRequest) error
	gofmtCheck                    func(req core.PhaseRequest) ([]string, error)
	solutionCheck                 func(req core.PhaseRequest) ([]string, error)
	skillsDriftCheck              func(req core.PhaseRequest) ([]string, error)
	goVetCheck                    func(req core.PhaseRequest) ([]string, error)
	acsDurableCheck               func(req core.PhaseRequest) ([]string, error)
	apicoverEnforceCheck          func(req core.PhaseRequest) ([]string, error)
	integrationTierCheck          func(req core.PhaseRequest) ([]string, error)
	apicoverNewPkgGraduationCheck func(req core.PhaseRequest) ([]string, error)
	phaseIO                       config.Stage
	ledger                        *defectledger.Ledger
}

func (hooks) PhaseName() string       { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string { return "evolve-auditor" }

func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {
	if _, err := os.Stat(filepath.Join(req.Workspace, continuation.ManifestName)); err != nil {
		return nil
	}
	return []string{filepath.Join(req.Workspace, defectDispositionFile)}
}

func (hooks) ArtifactFilename(_ core.PhaseRequest) string {
	return phasecontract.ArtifactFilename(string(core.PhaseAudit))
}
func (hooks) DefaultModel() string { return "opus" }

func (h hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	if contract := req.Context[core.CtxKeyTaskContract]; contract != "" {
		fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)
	}
	b.WriteString(inheritedDefectsPromptBlockVia(h.defectLedger(), req))
	b.WriteString(chainExamplePromptBlock())
	return b.String()
}

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

func extractAuditVerdict(content string, stage config.Stage) (string, bool) {
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
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
		for _, pb := range r.PhantomBindings {
			if pb != "" && !seen[pb] {
				seen[pb] = true
				g.phantomBindings = append(g.phantomBindings, pb)
			}
		}
	}
	return g, nil
}

var egpsRedIDCycleTokens = regexp.MustCompile(`^(?:cycle\d+/)?(?:Test)?(?:C\d+_)?(?:\d+_)?`)

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
			shown = append(shown, id)
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
	clause := fmt.Sprintf("; %d%s, first %s", len(harnessReds), core.HarnessRedClauseMarker, harnessReds[0])
	if len(harnessReds) > 1 {
		clause += fmt.Sprintf(" (+%d more in acs-verdict.json)", len(harnessReds)-1)
	}
	return clause
}

type Config struct {
	Bridge                        core.Bridge
	Prompts                       *prompts.Loader
	ContractVerifier              func() runner.ContractVerifier
	HostEffects                   func() core.HostEffects
	NowFn                         func() time.Time
	GenerateVerdict               func(req core.PhaseRequest) error
	BeginPredicateEvidence        func(core.PhaseRequest) (func() error, error)
	CheckExplanation              func(req core.PhaseRequest) error
	CheckGofmt                    func(req core.PhaseRequest) ([]string, error)
	CheckSolution                 func(req core.PhaseRequest) ([]string, error)
	SolutionSpec                  *config.DeliverableKindSpec
	CheckSkillsDrift              func(req core.PhaseRequest) ([]string, error)
	CheckGoVet                    func(req core.PhaseRequest) ([]string, error)
	CheckACSDurable               func(req core.PhaseRequest) ([]string, error)
	CheckApicoverEnforce          func(req core.PhaseRequest) ([]string, error)
	CheckIntegrationTier          func(req core.PhaseRequest) ([]string, error)
	CheckApicoverNewPkgGraduation func(req core.PhaseRequest) ([]string, error)
	PhaseIO                       config.Stage
	CompactPrompts                bool
	Signals                       func() *signalcenter.Center
}

type Option func(*Config)

func WithContractVerifier(fn func() runner.ContractVerifier) Option {
	return func(c *Config) { c.ContractVerifier = fn }
}

func WithHostEffects(fn func() core.HostEffects) Option {
	return func(c *Config) { c.HostEffects = fn }
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(cfg *Config) { cfg.Signals = c }
}

type Phase struct {
	*runner.BaseRunner
	signals func() *signalcenter.Center
}

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

func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {
	return NewDefaultWithStage(br, prm, config.StageOff)
}

func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {
	return NewDefaultWithStageCompact(br, prm, stage, false)
}

func NewDefaultWithStageCompact(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool) *Phase {
	return NewDefaultWithStageCompactSpec(br, prm, stage, compact, nil)
}

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
	ciParity{signals: cfg.Signals}.wire(&cfg)
	return New(cfg)
}

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

func gofmtCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	return codequality.UnformattedGoFiles(codequality.ModuleDir(root))
}

func generateACSVerdict(req core.PhaseRequest) error {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	emitTIADecision(req, root)
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return fmt.Errorf("acssuite run: %w", err)
	}
	evolveDir := filepath.Dir(filepath.Dir(req.Workspace))
	if _, err := acssuite.WriteVerdict(evolveDir, v); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

func emitTIADecision(req core.PhaseRequest, root string) {
	stage := policy.RegressionTIAStageFor(req.ProjectRoot)
	if stage == "off" {
		return
	}
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

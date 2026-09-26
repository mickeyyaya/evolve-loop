package deliverable

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// defaultBreakerThreshold consecutive enforce blocks demote the gate to advisory, so a miscalibrated gate cannot halt the loop.
const defaultBreakerThreshold = 3

// Reviewer is the host-side contract gate behind core.DeliverableReviewer.
type Reviewer struct {
	stage   config.Stage
	phaseIO config.Stage // EVOLVE_PHASE_IO stage; gates the RequireFailureContextPhaseIO check
	// reportSizeGate gates the Handoff Summary budget check independently of stage; it blocks only at enforce.
	reportSizeGate         config.Stage
	reportSizeBudgetTokens int
	threshold              int
	breakerPath            string // test override; "" derives the counter file under .evolve
	logf                   func(format string, args ...any)
	resolver               phasecontract.Resolver
	// signals reports every decision Review reaches; a Null Object until WithSignals.
	signals *gatesignal.Reporter
}

// breakerFile is the default persistent counter location.
const breakerFile = "contract-gate-breaker.json"

// NewReviewer builds the contract gate for stage, resolving only built-in contracts.
func NewReviewer(stage config.Stage, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.BuiltinResolver{}, config.StageOff, opts...)
}

// NewReviewerStage is NewReviewer with the PhaseIO stage, returning the concrete *Reviewer a runner takes as its ContractVerifier.
func NewReviewerStage(stage, phaseIO config.Stage, opts ...Option) *Reviewer {
	return newReviewer(stage, phasecontract.BuiltinResolver{}, phaseIO, opts...)
}

// NewReviewerWithCatalog builds the gate resolving built-ins first, then spec-derived contracts for the catalog's user and minted phases.
func NewReviewerWithCatalog(stage config.Stage, cat phasespec.Catalog, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), config.StageOff, opts...)
}

// NewReviewerWithCatalogStage is NewReviewerWithCatalog with the EVOLVE_PHASE_IO stage threaded.
func NewReviewerWithCatalogStage(stage config.Stage, cat phasespec.Catalog, phaseIO config.Stage, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO, opts...)
}

// NewReviewerWithCatalogStageReportSize adds the report-size gate, whose own stage blocks only at enforce.
func NewReviewerWithCatalogStageReportSize(stage config.Stage, cat phasespec.Catalog, phaseIO, reportSizeGate config.Stage, budgetTokens int, opts ...Option) *Reviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO, append([]Option{withReportSize(reportSizeGate, budgetTokens)}, opts...)...)
}

// withReportSize is an Option so every setting applies at one point and a caller's later option is never clobbered.
func withReportSize(gate config.Stage, budgetTokens int) Option {
	return func(r *Reviewer) {
		r.reportSizeGate = gate
		r.reportSizeBudgetTokens = budgetTokens
	}
}

func newReviewer(stage config.Stage, resolver phasecontract.Resolver, phaseIO config.Stage, opts ...Option) *Reviewer {
	r := &Reviewer{
		stage:     stage,
		phaseIO:   phaseIO,
		threshold: defaultBreakerThreshold,
		logf:      func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
		resolver:  resolver,
		signals:   gatesignal.New(nil),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Option configures a Reviewer at construction (functional options).
type Option func(*Reviewer)

// WithSignals reports every gate decision through c; a nil Center keeps the Null Object.
func WithSignals(c *signalcenter.Center) Option {
	return func(r *Reviewer) {
		if c != nil {
			r.signals = gatesignal.New(c)
		}
	}
}

// SignalsWired reports whether a Center was injected; it is the composition root's wiring proof.
func (r *Reviewer) SignalsWired() bool { return r.signals.Wired() }

// persistSalvage is the one effect of a salvage (persist, record, report), shared by Review and
// VerifyForClassification so a salvage is persisted and reported exactly once.
func (r *Reviewer) persistSalvage(check gatesignal.Check, phase string, roots phasecontract.Roots, res, salvaged Result) bool {
	if err := persistSalvagedArtifact(res.ArtifactPath, res.Content, salvaged.Content); err != nil {
		r.logf("[contract-gate] %s: salvage recovered the verdict but the repaired artifact could not be persisted; refusing the salvage: %v", phase, err)
		return false
	}
	pattern := ClassifyBadVerdict(res.Content).Pattern
	recordSalvageApplied(roots, phase, pattern, r.logf)
	r.signals.Salvaged(check, filepath.Base(res.ArtifactPath), string(pattern))
	if line := SalvageSummaryLine(roots.EvolveDir); line != "" {
		r.logf("[contract-gate] %s: %s", phase, line)
	}
	return true
}

// VerifyForClassification hands the verdict engine the gate's own verification, salvaging at enforce, so both judge the same bytes.
func (r *Reviewer) VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (Result, error) {
	res, err := VerifyWithReportSize(phase, roots, r.resolver, r.phaseIO, r.reportSizeGate, r.reportSizeBudgetTokens)
	if err != nil || res.OK || r.stage != config.StageEnforce {
		return res, err
	}
	// Only a persisted salvage is recorded here; Review records an unsalvageable block once, not once per settle probe.
	salvaged, applied := salvageVerdictWith(res, r.resolver, roots, r.phaseIO)
	if !applied {
		return res, nil
	}
	if !r.persistSalvage(check, phase, roots, res, salvaged) {
		return res, nil // a refused CAS re-probes; nothing recorded until a salvage lands
	}
	recordBadVerdictBaseline(roots, phase, res, r.logf)
	return salvaged, nil
}

// Review adjudicates one finished phase's deliverable.
func (r *Reviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if r.stage == config.StageOff {
		return core.ReviewResult{Approve: true}
	}
	check := gatesignal.Check{Cycle: in.Cycle, RunID: in.RunID, Phase: in.Phase}
	roots := rootsFor(in)
	res, err := VerifyWithReportSize(in.Phase, roots, r.resolver, r.phaseIO, r.reportSizeGate, r.reportSizeBudgetTokens)
	if err != nil {
		// The gate's own uncertainty fails open and leaves the breaker alone.
		r.logf("[contract-gate] %s: ambiguity, failing open: %v", in.Phase, err)
		r.signals.FailOpen(check, err)
		return core.ReviewResult{Approve: true}
	}
	bp := r.breakerPath
	if bp == "" {
		bp = filepath.Join(roots.EvolveDir, breakerFile)
	}
	if res.OK {
		resetBreaker(bp)
		r.signals.Verified(check, r.verifiedSet(res))
		return core.ReviewResult{Approve: true}
	}

	// Observability only: it never reads or writes a decision.
	recordBadVerdictBaseline(roots, in.Phase, res, r.logf)

	if salvaged, applied := salvageVerdictWith(res, r.resolver, roots, r.phaseIO); applied {
		// Below enforce a salvage is only reported: a soak must not rewrite the artifact, telemetry or breaker.
		if r.stage != config.StageEnforce {
			reason := summarize(in.Phase, res)
			r.logf("[contract-gate] %s: %s (stage=%s, would-block; would salvage the verdict — artifact, telemetry and breaker left untouched)",
				in.Phase, reason, r.stage)
			r.signals.WouldBlock(check, reason, codesOf(res), r.stage.String(), true)
			return core.ReviewResult{Approve: true}
		}
		// Fail closed: downstream phases re-read the file, so approving bytes that were not persisted reinstates the defect.
		if r.persistSalvage(check, in.Phase, roots, res, salvaged) {
			return core.ReviewResult{Approve: true}
		}
	}

	reason := summarize(in.Phase, res)

	// The size gate is warn-only below its own enforce; a co-occurring real violation still blocks.
	if r.reportSizeGate < config.StageEnforce && res.onlyViolation(CodeHandoffBudgetExceeded) {
		r.logf("[contract-gate] %s: %s (reportSizeGate=%s, would-block, WARN)", in.Phase, reason, r.reportSizeGate)
		r.signals.WouldBlock(check, reason, codesOf(res), r.reportSizeGate.String(), false)
		return core.ReviewResult{Approve: true}
	}

	if r.stage != config.StageEnforce {
		r.logf("[contract-gate] %s: %s (stage=%s, would-block)", in.Phase, reason, r.stage)
		r.signals.WouldBlock(check, reason, codesOf(res), r.stage.String(), false)
		return core.ReviewResult{Approve: true}
	}

	n := incrBreaker(bp)
	if n >= r.threshold {
		r.logf("[contract-gate] CIRCUIT OPEN: %d consecutive contract blocks — demoting enforce→advisory so the loop is not bricked. Inspect policy.gates.contract_gate / the failing phase %q. Last reason: %s", n, in.Phase, reason)
		// Demoted, never a bare approval: an approval the gate did not earn must not look like one.
		r.signals.Demoted(check, reason, n)
		return core.ReviewResult{Approve: true, Demoted: true, Reason: reason, Blocks: n}
	}
	r.logf("[contract-gate] %s: %s (stage=enforce, BLOCK %d/%d)", in.Phase, reason, n, r.threshold)
	r.signals.Rejected(check, reason, codesOf(res), n, r.threshold)
	// Blocks is this breaker's count, so the ladder escalates off the counter that opens the circuit.
	return core.ReviewResult{Approve: false, Reason: reason, Blocks: n}
}

// summarize renders the violations into one actionable rejection reason.
func summarize(phase string, res Result) string {
	parts := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		parts = append(parts, fmt.Sprintf("[%s] %s", v.Code, v.Message))
	}
	return fmt.Sprintf("%s deliverable failed contract: %s", phase, strings.Join(parts, "; "))
}

// codesOf projects a Result's violation codes for the signal's fields.
func codesOf(res Result) []string {
	codes := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		codes = append(codes, v.Code)
	}
	return codes
}

// verifiedSet names what a clean verification checked: the primary's basename and size, and Result.Owed and Result.Effects.
func (r *Reviewer) verifiedSet(res Result) gatesignal.Verified {
	v := gatesignal.Verified{Bytes: len(res.Content), Owed: res.Owed, Effects: res.Effects, Stage: r.stage.String()}
	if res.ArtifactPath != "" {
		v.Artifact = filepath.Base(res.ArtifactPath)
	}
	return v
}

// breakerState persists the consecutive-block count under .evolve, because the orchestrator is rebuilt every cycle.
type breakerState struct {
	Consecutive int `json:"consecutive"`
}

func readBreaker(path string) int {
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var s breakerState
	if json.Unmarshal(data, &s) != nil {
		return 0
	}
	return s.Consecutive
}

func writeBreaker(path string, n int) {
	if path == "" {
		return
	}
	data, _ := json.Marshal(breakerState{Consecutive: n})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[contract-gate] WARN could not persist breaker state %s: %v\n", path, err)
		return
	}
	if err := os.Rename(tmp, path); err != nil { // atomic
		fmt.Fprintf(os.Stderr, "[contract-gate] WARN could not commit breaker state %s: %v\n", path, err)
	}
}

func incrBreaker(path string) int {
	n := readBreaker(path) + 1
	writeBreaker(path, n)
	return n
}

func resetBreaker(path string) {
	if readBreaker(path) != 0 {
		writeBreaker(path, 0)
	}
}

// persistSalvagedArtifact atomically writes the approved bytes over the judged artifact, so downstream phases
// re-read what the gate approved; an empty path (NoArtifact) is a no-op.
func persistSalvagedArtifact(path, judged, content string) error {
	if path == "" {
		return nil
	}
	// Refuse if the file changed since the gate read it, so a live agent's newer report is never replaced by a
	// repair of the older one. Not atomic with the rename: a write inside that window is still clobbered.
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("re-read before write-back: %w", err)
	}
	if string(current) != judged {
		return fmt.Errorf("artifact changed under the gate between verify and write-back (judged %d bytes, on disk %d) — refusing to overwrite a newer report with a repair of the older one", len(judged), len(current))
	}
	return atomicwrite.Bytes(path, []byte(content))
}

// PlainVerifier is the verdict engine's verifier when the gate is off: catalog-aware verify with no salvage.
type PlainVerifier struct{ PhaseIO config.Stage }

// VerifyForClassification verifies without salvaging.
func (v PlainVerifier) VerifyForClassification(_ gatesignal.Check, phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyCatalogAwareStage(phase, roots, v.PhaseIO)
}

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

const defaultBreakerThreshold = 3

type Reviewer struct {
	stage                  config.Stage
	phaseIO                config.Stage
	reportSizeGate         config.Stage
	reportSizeBudgetTokens int
	threshold              int
	breakerPath            string
	logf                   func(format string, args ...any)
	resolver               phasecontract.Resolver
	signals                *gatesignal.Reporter
}

const breakerFile = "contract-gate-breaker.json"

func NewReviewer(stage config.Stage, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.BuiltinResolver{}, config.StageOff, opts...)
}

func NewReviewerStage(stage, phaseIO config.Stage, opts ...Option) *Reviewer {
	return newReviewer(stage, phasecontract.BuiltinResolver{}, phaseIO, opts...)
}

func NewReviewerWithCatalog(stage config.Stage, cat phasespec.Catalog, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), config.StageOff, opts...)
}

func NewReviewerWithCatalogStage(stage config.Stage, cat phasespec.Catalog, phaseIO config.Stage, opts ...Option) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO, opts...)
}

func NewReviewerWithCatalogStageReportSize(stage config.Stage, cat phasespec.Catalog, phaseIO, reportSizeGate config.Stage, budgetTokens int, opts ...Option) *Reviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO, append([]Option{withReportSize(reportSizeGate, budgetTokens)}, opts...)...)
}

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

type Option func(*Reviewer)

func WithSignals(c *signalcenter.Center) Option {
	return func(r *Reviewer) {
		if c != nil {
			r.signals = gatesignal.New(c)
		}
	}
}

func (r *Reviewer) SignalsWired() bool { return r.signals.Wired() }

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

func (r *Reviewer) VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (Result, error) {
	res, err := VerifyWithReportSize(phase, roots, r.resolver, r.phaseIO, r.reportSizeGate, r.reportSizeBudgetTokens)
	if err != nil || res.OK || r.stage != config.StageEnforce {
		return res, err
	}
	salvaged, applied := salvageVerdictWith(res, r.resolver, roots, r.phaseIO)
	if !applied {
		return res, nil
	}
	if !r.persistSalvage(check, phase, roots, res, salvaged) {
		return res, nil
	}
	recordBadVerdictBaseline(roots, phase, res, r.logf)
	return salvaged, nil
}

func (r *Reviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if r.stage == config.StageOff {
		return core.ReviewResult{Approve: true}
	}
	check := gatesignal.Check{Cycle: in.Cycle, RunID: in.RunID, Phase: in.Phase}
	roots := RootsFor(in)
	res, err := VerifyWithReportSize(in.Phase, roots, r.resolver, r.phaseIO, r.reportSizeGate, r.reportSizeBudgetTokens)
	if err != nil {
		r.logf("[contract-gate] %s: ambiguity, failing open: %v", in.Phase, err)
		r.signals.FailOpen(check, err)
		return core.ReviewResult{Approve: true}
	}
	bp := r.breakerPath
	if bp == "" {
		bp = filepath.Join(roots.EvolveDir, breakerFile)
	}
	if res.OK {
		if !in.BreakerExempt {
			resetBreaker(bp)
		}
		r.signals.Verified(check, r.verifiedSet(res))
		return core.ReviewResult{Approve: true}
	}

	recordBadVerdictBaseline(roots, in.Phase, res, r.logf)

	if salvaged, applied := salvageVerdictWith(res, r.resolver, roots, r.phaseIO); applied {
		if r.stage != config.StageEnforce {
			reason := summarize(in.Phase, res)
			r.logf("[contract-gate] %s: %s (stage=%s, would-block; would salvage the verdict — artifact, telemetry and breaker left untouched)",
				in.Phase, reason, r.stage)
			r.signals.WouldBlock(check, reason, codesOf(res), r.stage.String(), true)
			return core.ReviewResult{Approve: true}
		}
		if r.persistSalvage(check, in.Phase, roots, res, salvaged) {
			return core.ReviewResult{Approve: true}
		}
	}

	return r.violationResult(check, in, res, bp)
}

func (r *Reviewer) violationResult(check gatesignal.Check, in core.ReviewInput, res Result, bp string) core.ReviewResult {
	reason := summarize(in.Phase, res)

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

	if in.BreakerExempt {
		r.logf("[contract-gate] %s: %s (stage=enforce, BLOCK, breaker-exempt: the circuit never counts this phase)", in.Phase, reason)
		r.signals.Rejected(check, reason, codesOf(res), 0, r.threshold)
		return core.ReviewResult{Approve: false, Reason: reason, DeliverableAbsent: res.deliverableAbsent()}
	}
	n := incrBreaker(bp)
	if n >= r.threshold {
		r.logf("[contract-gate] CIRCUIT OPEN: %d consecutive contract blocks — demoting enforce→advisory so the loop is not bricked. Inspect policy.gates.contract_gate / the failing phase %q. Last reason: %s", n, in.Phase, reason)
		r.signals.Demoted(check, reason, n)
		return core.ReviewResult{Approve: true, Demoted: true, Reason: reason, Blocks: n}
	}
	r.logf("[contract-gate] %s: %s (stage=enforce, BLOCK %d/%d)", in.Phase, reason, n, r.threshold)
	r.signals.Rejected(check, reason, codesOf(res), n, r.threshold)
	return core.ReviewResult{Approve: false, Reason: reason, Blocks: n, DeliverableAbsent: res.deliverableAbsent()}
}

func summarize(phase string, res Result) string {
	parts := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		parts = append(parts, fmt.Sprintf("[%s] %s", v.Code, v.Message))
	}
	return fmt.Sprintf("%s deliverable failed contract: %s", phase, strings.Join(parts, "; "))
}

func codesOf(res Result) []string {
	codes := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		codes = append(codes, v.Code)
	}
	return codes
}

func (r *Reviewer) verifiedSet(res Result) gatesignal.Verified {
	v := gatesignal.Verified{Bytes: len(res.Content), Owed: res.Owed, Effects: res.Effects, Stage: r.stage.String()}
	if res.ArtifactPath != "" {
		v.Artifact = filepath.Base(res.ArtifactPath)
	}
	return v
}

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
	if err := os.Rename(tmp, path); err != nil {
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

func persistSalvagedArtifact(path, judged, content string) error {
	if path == "" {
		return nil
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("re-read before write-back: %w", err)
	}
	if string(current) != judged {
		return fmt.Errorf("artifact changed under the gate between verify and write-back (judged %d bytes, on disk %d) — refusing to overwrite a newer report with a repair of the older one", len(judged), len(current))
	}
	return atomicwrite.Bytes(path, []byte(content))
}

type PlainVerifier struct{ PhaseIO config.Stage }

func (v PlainVerifier) VerifyForClassification(_ gatesignal.Check, phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyCatalogAwareStage(phase, roots, v.PhaseIO)
}

package deliverable

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

const defaultBreakerThreshold = 3

// Reviewer is the deliverable-contract gate. Construct with NewReviewer; tests
// override breakerPath/threshold/logf directly.
type Reviewer struct {
	stage       config.Stage
	phaseIO     config.Stage // EVOLVE_PHASE_IO rollout stage; gates the RequireFailureContextPhaseIO check (ADR-0050 §3.8). Default StageOff → byte-identical.
	threshold   int
	breakerPath string // override for the consecutive-block counter file (tests); "" → derive under .evolve
	logf        func(format string, args ...any)
	resolver    phasecontract.Resolver // built-in only by default; catalog-aware via NewReviewerWithCatalog
}

// breakerFile is the default persistent counter location.
const breakerFile = "contract-gate-breaker.json"

// NewReviewer builds the contract gate for a stage, resolving only built-in
// contracts. Callers wire it via core.WithReviewer (chained after evalgate)
// only when stage != StageOff.
func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.BuiltinResolver{}, config.StageOff)
}

// NewReviewerWithCatalog builds the contract gate resolving built-in contracts
// first and falling back to spec-derived contracts (FromSpec) for the catalog's
// user/minted phases. PhaseIO defaults to StageOff (byte-identical); production
// wires the real dial via NewReviewerWithCatalogStage.
func NewReviewerWithCatalog(stage config.Stage, cat phasespec.Catalog) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), config.StageOff)
}

// NewReviewerWithCatalogStage is NewReviewerWithCatalog threaded with the
// EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8). The stage gates only the
// additive RequireFailureContextPhaseIO check for build/scout/triage (blocks at
// StageEnforce, and only when the ContractGate stage is also enforce); every
// other gate behavior is unchanged, so passing StageOff equals the legacy
// constructor.
func NewReviewerWithCatalogStage(stage config.Stage, cat phasespec.Catalog, phaseIO config.Stage) core.DeliverableReviewer {
	return newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO)
}

func newReviewer(stage config.Stage, resolver phasecontract.Resolver, phaseIO config.Stage) *Reviewer {
	return &Reviewer{
		stage:     stage,
		phaseIO:   phaseIO,
		threshold: defaultBreakerThreshold,
		logf:      func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
		resolver:  resolver,
	}
}

// Review adjudicates one finished phase's deliverable.
func (r *Reviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if r.stage == config.StageOff {
		return core.ReviewResult{Approve: true}
	}
	roots := rootsFor(in)
	// r.resolver is always set by newReviewer (the single construction point):
	// BuiltinResolver for NewReviewer, a CatalogResolver for
	// NewReviewerWithCatalog. No nil guard needed.
	res, err := VerifyWithStage(in.Phase, roots, r.resolver, r.phaseIO)
	if err != nil {
		// Ambiguity / infra — fail OPEN (never brick the loop on the gate's own
		// inability to decide). Does not touch the breaker.
		r.logf("[contract-gate] %s: ambiguity, failing open: %v", in.Phase, err)
		return core.ReviewResult{Approve: true}
	}
	bp := r.breakerPath
	if bp == "" {
		bp = filepath.Join(roots.EvolveDir, breakerFile)
	}
	if res.OK {
		resetBreaker(bp)
		return core.ReviewResult{Approve: true}
	}

	reason := summarize(in.Phase, res)
	if r.stage != config.StageEnforce {
		// Shadow/advisory: log the would-block and approve.
		r.logf("[contract-gate] %s: %s (stage=%s, would-block)", in.Phase, reason, r.stage)
		return core.ReviewResult{Approve: true}
	}

	// Enforce: count the block; the breaker demotes to advisory at threshold.
	n := incrBreaker(bp)
	if n >= r.threshold {
		r.logf("[contract-gate] CIRCUIT OPEN: %d consecutive contract blocks — demoting enforce→advisory so the loop is not bricked. Inspect EVOLVE_CONTRACT_GATE / the failing phase %q. Last reason: %s", n, in.Phase, reason)
		return core.ReviewResult{Approve: true}
	}
	r.logf("[contract-gate] %s: %s (stage=enforce, BLOCK %d/%d)", in.Phase, reason, n, r.threshold)
	return core.ReviewResult{Approve: false, Reason: reason}
}

// summarize renders the violations into one actionable rejection reason.
func summarize(phase string, res Result) string {
	parts := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		parts = append(parts, fmt.Sprintf("[%s] %s", v.Code, v.Message))
	}
	return fmt.Sprintf("%s deliverable failed contract: %s", phase, strings.Join(parts, "; "))
}

// --- circuit breaker persistence ---
//
// The consecutive-block count is persisted so it survives the per-cycle
// reconstruction of the orchestrator in `evolve loop`. A tiny JSON file under
// .evolve keeps the state crash-safe and inspectable.

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
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path) // atomic
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

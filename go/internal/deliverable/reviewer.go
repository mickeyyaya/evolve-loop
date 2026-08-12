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
	stage   config.Stage
	phaseIO config.Stage // EVOLVE_PHASE_IO rollout stage; gates the RequireFailureContextPhaseIO check (ADR-0050 §3.8). Default StageOff → byte-identical.
	// reportSizeGate gates the Handoff Summary token-budget check (cycle-565
	// Slice S1), independent of the ContractGate stage: blocks only at
	// StageEnforce. reportSizeBudgetTokens is the budget it enforces. Zero-value
	// (StageOff/0) ⇒ byte-identical to pre-S1 behavior.
	reportSizeGate         config.Stage
	reportSizeBudgetTokens int
	threshold              int
	breakerPath            string // override for the consecutive-block counter file (tests); "" → derive under .evolve
	logf                   func(format string, args ...any)
	resolver               phasecontract.Resolver // built-in only by default; catalog-aware via NewReviewerWithCatalog
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

// NewReviewerWithCatalogStageReportSize is NewReviewerWithCatalogStage plus the
// report-size gate (cycle-565 Slice S1). reportSizeGate gates the Handoff
// Summary token-budget check — INDEPENDENT of the ContractGate stage, blocking
// only at StageEnforce; budgetTokens is the budget it enforces. Zero-value
// (StageOff/0) ⇒ byte-identical to NewReviewerWithCatalogStage, so wiring it in
// changes nothing until the report-size gate is deliberately promoted.
func NewReviewerWithCatalogStageReportSize(stage config.Stage, cat phasespec.Catalog, phaseIO, reportSizeGate config.Stage, budgetTokens int) core.DeliverableReviewer {
	r := newReviewer(stage, phasecontract.NewCatalogResolver(cat.Get), phaseIO)
	r.reportSizeGate = reportSizeGate
	r.reportSizeBudgetTokens = budgetTokens
	return r
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
	res, err := VerifyWithReportSize(in.Phase, roots, r.resolver, r.phaseIO, r.reportSizeGate, r.reportSizeBudgetTokens)
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

	// Observability-only, strictly after the OK branch and strictly before any
	// decision is computed: record what a future salvage stage WOULD have
	// recovered from this bad_verdict. Never reads a decision, never writes one.
	recordBadVerdictBaseline(roots, in.Phase, res, r.logf)

	// Extraction/coercion stage (schema-aligned salvage layer, second
	// deliverable): when the bad_verdict is the SOLE violation, is genuinely
	// recoverable and unambiguous, AND the repaired bytes re-verify clean,
	// approve via the salvaged Result instead of falling through to block —
	// same fail-safe posture as the instrumentation above (never invents a
	// value, refuses on ambiguity). Logged separately from the unconditional
	// baseline record, additive not a replacement.
	if salvaged, applied := salvageVerdictWith(res, r.resolver, roots, r.phaseIO); applied {
		// EFFECTS live below the dial; the DECISION above it (cycle-1442 audit
		// H3). Computing "would this have salvaged" is precisely what a shadow
		// soak is for, but the block inherited its position from the
		// unconditional observability record above and so also rewrote the
		// judged artifact, appended the telemetry sidecar and touched the
		// breaker while the gate reported itself disabled — a soak run then
		// measures a system its own "disabled" gate already mutated.
		if r.stage != config.StageEnforce {
			// Carries the contract reason too: the ordinary shadow path below
			// logs summarize(), and an operator tailing the gate at advisory
			// must not lose the rejection reason for exactly the subset of
			// reports salvage happens to touch (diff-review LOW).
			r.logf("[contract-gate] %s: %s (stage=%s, would-block; would salvage the verdict — artifact, telemetry and breaker left untouched)",
				in.Phase, summarize(in.Phase, res), r.stage)
			return core.ReviewResult{Approve: true}
		}
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
		if err := persistSalvagedArtifact(res.ArtifactPath, res.Content, salvaged.Content); err != nil {
			r.logf("[contract-gate] %s: salvage recovered the verdict but the repaired artifact could not be persisted; refusing the salvage: %v", in.Phase, err)
		} else {
			recordSalvageApplied(roots, in.Phase, ClassifyBadVerdict(res.Content).Pattern, r.logf)
			// Surfaced, not just logged to a sidecar nobody reads: README §8 promises
			// operators that every coercion is "logged + surfaced", and a salvage
			// silently approving a phase is exactly the false-confidence failure the
			// research memo (§3.3) is written against. Rendered AFTER the sidecar
			// append so the running total includes this salvage, and read back FROM
			// that sidecar so the count is single-sourced, never a second counter
			// (cycle-1392 audit LOW dd17d798e155571ecd91be63e14050ab6). Empty at zero
			// records or an unreadable/absent sidecar — no zero-noise, and the gate's
			// decision never depends on it.
			if line := SalvageSummaryLine(roots.EvolveDir); line != "" {
				r.logf("[contract-gate] %s: %s", in.Phase, line)
			}
			// Breaker-NEUTRAL, not breaker-clearing (cycle-1441 audit M2b; the
			// repo rule for every salvage rung). Resetting here pinned the
			// consecutive-block counter at zero for any phase that kept
			// emitting recoverable-malformed reports, so neither the
			// second-block escalation ladder nor the third-block breaker could
			// ever fire on a persistently malformed producer. Leave the count
			// exactly as the gate found it: a salvage is neither a block nor a
			// clean pass.
			return core.ReviewResult{Approve: true}
		}
	}

	reason := summarize(in.Phase, res)

	// Report-size handoff-budget is warn-only below its own enforce dial
	// (cycle-646): at advisory VerifyWithReportSize records the violation so we
	// log a would-block WARN here, but the size gate must never block a cycle
	// until reportSizeGate==enforce — independent of the ContractGate stage. If
	// the ONLY reason to block is that warn-only size violation, approve. Any
	// co-occurring real contract violation still falls through to the block path.
	if r.reportSizeGate < config.StageEnforce && res.onlyViolation(CodeHandoffBudgetExceeded) {
		r.logf("[contract-gate] %s: %s (reportSizeGate=%s, would-block, WARN)", in.Phase, reason, r.reportSizeGate)
		return core.ReviewResult{Approve: true}
	}

	if r.stage != config.StageEnforce {
		// Shadow/advisory: log the would-block and approve.
		r.logf("[contract-gate] %s: %s (stage=%s, would-block)", in.Phase, reason, r.stage)
		return core.ReviewResult{Approve: true}
	}

	// Enforce: count the block; the breaker demotes to advisory at threshold.
	n := incrBreaker(bp)
	if n >= r.threshold {
		r.logf("[contract-gate] CIRCUIT OPEN: %d consecutive contract blocks — demoting enforce→advisory so the loop is not bricked. Inspect policy.gates.contract_gate / the failing phase %q. Last reason: %s", n, in.Phase, reason)
		// Demoted (never a bare Approve): the orchestrator turns this into a
		// loud, phase+CLI-named WARN, a cycle-visible ledger entry and a staged
		// escalation intent. An approval the gate did not earn must not look
		// like one (inbox contract-block-cli-escalation).
		return core.ReviewResult{Approve: true, Demoted: true, Reason: reason, Blocks: n}
	}
	r.logf("[contract-gate] %s: %s (stage=enforce, BLOCK %d/%d)", in.Phase, reason, n, r.threshold)
	// Blocks reports THIS breaker's consecutive count so the correction ladder
	// escalates its CLI off the same counter that will open the circuit, instead
	// of re-deriving a correction ordinal that desyncs from it.
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
func persistSalvagedArtifact(path, judged, content string) error {
	if path == "" {
		return nil
	}
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
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("re-read before write-back: %w", err)
	}
	if string(current) != judged {
		return fmt.Errorf("artifact changed under the gate between verify and write-back (judged %d bytes, on disk %d) — refusing to overwrite a newer report with a repair of the older one", len(judged), len(current))
	}
	return atomicwrite.Bytes(path, []byte(content))
}

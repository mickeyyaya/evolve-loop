package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type phaseReconciliation struct {
	reconciled              bool
	verifiedResult          deliverable.Result
	acsFloorRescued         bool
	acsFloorOverriddenCodes string
}

// reconcileDeliverable interprets bridge teardown separately from the agent's
// deliverable verdict. It returns an early response only when the phase cannot
// safely continue to normal classification.
func (b *BaseRunner) reconcileDeliverable(
	ctx context.Context,
	req core.PhaseRequest,
	prep phasePreparation,
	dispatch phaseDispatchResult,
) (phaseReconciliation, *core.PhaseResponse, error) {
	phase := prep.phase
	artifactPath := prep.artifactPath
	preDispatch := prep.preDispatch
	hadPreDispatch := prep.hadPreDispatch
	bres := dispatch.bridgeResponse
	bridgeErr := dispatch.bridgeErr
	durationMS := dispatch.durationMS

	// reconciled is set when a bridge INFRA teardown (timeout OR transient) is
	// overridden by a well-formed deliverable on disk: control then FALLS THROUGH
	// to the same artifact-read + Classify path the happy case uses (so audit's
	// EGPS gate still applies — reconciliation can never ship a green-looking
	// report whose predicates are red). See the reconcile block below.
	reconciled := false
	acsFloorRescued := false
	acsFloorOverriddenCodes := "" // teardown Verify codes the ACS floor overrode (surfaced on the response)
	// reconciledRes is the reconcile probe's Verify result, carried out of this
	// block so the classify step below consumes the bytes that Verify READ
	// (Result.Content) instead of re-reading the path — see the verdict-source
	// comment on the single-read invariant.
	var reconciledRes deliverable.Result
	if bridgeErr != nil {
		// A bridge INFRA teardown — an artifact-wait timeout (exit 81) OR a
		// transient failure (exit 80/85/86: quota, liveness-exhaustion) — ends the
		// SESSION, but is not a verdict: the agent may have written its contracted
		// deliverable before the teardown (cycle-254/255 timeout false-FAIL;
		// cycle-835 quota false-FAIL — a complete PASS audit discarded because the
		// deep-tier auditor hit exit=85 at the tail while agy+codex were walled).
		// Reconcile against the deliverable: if it is on disk and well-formed, trust
		// its verdict (via Classify) instead of synthesizing FAIL. Reconciliation
		// can only UPGRADE toward the agent's real verdict, never downgrade a real one.
		if core.IsInfraTeardownError(bridgeErr) {
			roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree, DispatchedArtifact: artifactPath, ExplanationDocumentationVersion: req.ExplanationDocumentationVersion}
			if req.ProjectRoot != "" {
				// EvolveDir completes the roots (orchestrator-target
				// deliverables) AND locates the merged catalog for the
				// catalog-aware default.
				roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
			}
			// CANCELLATION-IMMUNE ladder (WithoutCancel, deliberate): on THIS path a
			// cancelled ctx is frequently the CAUSE of the teardown, not a reason to stop
			// waiting — the tmux driver, on ctx.Err(), takes one final completion poll and
			// otherwise exits ExitArtifactTimeout, "laundering a finished session into a
			// timeout … the runner's settle-retry was the only thing standing between that
			// mislabel and a false FAIL" (driver_tmux_repl.go). Honoring cancellation here
			// would re-open the cycles-824/825 false-FAIL class the ladder exists for, and
			// it would buy nothing in the standing fleet mode (a lane's `evolve cycle run`
			// passes context.Background()). The ctx-aware bail lives on the clean-exit path
			// below, where the agent exited 0 and nothing more is coming.
			res, verr := b.verifyReconcileDeliverable(context.WithoutCancel(ctx), phase, roots)
			// The stale-leftover gate (cycle-1550): a deliverable byte-identical
			// to the pre-dispatch snapshot is the PRIOR attempt's report — well-
			// formed, cycle-scoped challenge token and all — and reconciling it
			// would re-grade a stale verdict as this session's work. Refuse BOTH
			// reconcile doors (the well-formed fall-through and the ACS floor)
			// and let the optional/fatal arms handle it as untrustworthy, with
			// the cause on record.
			staleLeftover := hadPreDispatch && artifactUnchangedSince(artifactPath, preDispatch)
			if staleLeftover && verr == nil && res.OK {
				verr = fmt.Errorf("deliverable at %s is byte-identical to the pre-dispatch leftover (a prior attempt's report) — reconcile refused (cycle-1550)", artifactPath)
				res = deliverable.Result{}
			}
			switch {
			case verr == nil && res.OK:
				// Deliverable survived the timeout — fall through to Classify.
				reconciled = true
				reconciledRes = res
			case b.optional:
				// Optional-phase soft-fail (Workstream D / cycle-120): no
				// trustworthy deliverable, but an optional phase's successor is
				// verdict-unconditional, so degrade to WARN and let the cycle
				// advance instead of aborting.
				msg := fmt.Sprintf("optional phase %q degraded: no trustworthy deliverable after a bridge infra teardown (%v); cycle continues", phase, bridgeErr)
				if verr != nil {
					msg = fmt.Sprintf("%s [deliverable unverifiable: %v]", msg, verr)
				}
				resp := core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictWARN,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics: []core.Diagnostic{{
						Severity: "warning",
						Message:  msg,
					}},
				}
				return phaseReconciliation{}, &resp, nil
			default:
				// ACS DETERMINISTIC FLOOR (verdict-incoherence family: cycles
				// 603/921/924/931/3). The teardown-time deliverable.Verify can return
				// not-OK on a report that standalone-verifies OK — a session-lifecycle
				// artifact (the agent wrote a valid report, then idled without the
				// `evolve phase verify` handshake until ctx-cancel), NOT a defect. Before
				// discarding a possibly-shippable cycle as FAIL, consult the NON-LLM ground
				// truth a stall cannot corrupt: the acssuite verdict. When it is PASS AND
				// the report carries THIS cycle's challenge token with a PASS sentinel
				// (anti-gaming: a forged/stale report fails the token), the phase genuinely
				// passed — reconcile to the agent's own report via the same Classify path
				// the clean exit uses. This is exactly the (audit==PASS && acs==PASS)
				// condition the ADR-0072 coherence floor flags as incoherent, prevented at
				// the source instead of halting after the fact.
				// The verified snapshot is BOTH the rescue's evidence and (below) the
				// classified content. When the probe produced no bytes — an infra read
				// fault returns an empty Result, and an artifact that appeared after the
				// last probe verifies absent — fall back to ONE disk read and adopt those
				// bytes into the snapshot, so the rescue keeps the liveness it had before
				// the single-read change AND the rescue and the classification still judge
				// the same bytes (a rescue on content Classify never sees would FAIL anyway).
				// Setting ArtifactPath alongside Content keeps the snapshot
				// self-consistent (these bytes, and where they came from) so the
				// classify step below reuses them instead of reading a third time.
				// It does widen the field from "the path Verify judged" to "the
				// path these bytes came from" — safe here because audit has no
				// dispatch/contract path skew, and scoped to this rescue branch.
				if res.Content == "" {
					if data, readErr := os.ReadFile(artifactPath); readErr == nil {
						res.ArtifactPath, res.Content = artifactPath, string(data)
					}
				}
				if !staleLeftover && acsFloorRescues(phase, req.Workspace, res.Content) {
					reconciled = true
					reconciledRes = res
					acsFloorRescued = true
					acsFloorOverriddenCodes = forensicCodes(res.Violations)
					log.Diag().Infof("[runner] ACS-FLOOR phase=%s: teardown Verify not-OK (codes=[%s]) but the acssuite verdict is ship-eligible and the report carries this cycle's challenge token with a PASS sentinel — reconciled to the deterministic verdict\n", phase, acsFloorOverriddenCodes)
					break
				}
				// Mandatory phase, no trustworthy deliverable (absent/malformed/
				// unverifiable): hard-fail as before, enriched with the
				// well-formedness violation when we have one.
				msg := bridgeErr.Error()
				if verr == nil && len(res.Violations) > 0 {
					msg = fmt.Sprintf("%s; deliverable not trustworthy: %s", msg, res.Violations[0].Message)
				} else if verr != nil {
					// Includes the stale-leftover refusal (cycle-1550) — the
					// forensic dig must see WHY reconcile declined, not only
					// that the bridge timed out.
					msg = fmt.Sprintf("%s; deliverable not trustworthy: %v", msg, verr)
				}
				// [VERDICT-FORENSIC] The retro's #1 ask (cycle-3): a teardown default-FAIL
				// today records NO reasoning, so a false-FAIL is a 30-minute forensic dig.
				// Log the roots passed to verifyFn, the violation codes, verr, and the
				// on-disk deterministic state so the next recurrence is one grep.
				log.Diag().Infof("[VERDICT-FORENSIC] teardown-FAIL phase=%s roots{ws=%s wt=%s evolve=%s} verr=%v codes=[%s] report{%s} acs{%s}\n",
					phase, roots.Workspace, roots.Worktree, roots.EvolveDir, verr, forensicCodes(res.Violations),
					forensicSnapshot(artifactPath, 160), forensicSnapshot(filepath.Join(req.Workspace, "acs-verdict.json"), 200))
				resp := core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictFAIL,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
				}
				return phaseReconciliation{}, &resp, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
			}
		} else {
			// A substantive bridge error (launch/boot/safety/cost — NEITHER infra
			// sentinel) means the process failed in a way that makes any on-disk
			// output untrustworthy: no shippable work was produced — hard-fail,
			// optional or not, without consulting the deliverable.
			resp := core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				CostUSD:      bres.CostUSD,
				Tokens:       bres.Tokens,
				DurationMS:   durationMS,
				BootMS:       bres.BootMS,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
			}
			return phaseReconciliation{}, &resp, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
		}
	}

	return phaseReconciliation{
		reconciled:              reconciled,
		verifiedResult:          reconciledRes,
		acsFloorRescued:         acsFloorRescued,
		acsFloorOverriddenCodes: acsFloorOverriddenCodes,
	}, nil, nil
}

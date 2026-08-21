package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
)

// reviewAndGuard runs the per-phase deliverable review gate + correction ladder,
// the ship-preserve clear, the worktree-leak recovery, and the post-phase
// tree-diff guard (extracted behavior-preserving from RunCycle). dr is a pointer
// because a successful correction re-dispatch UPDATES dr.resp (and dr.phaseReq's
// CorrectionDirective), which recordAndBranch then consumes.
//
// Returns loopAbort + error on a review reject (after corrections), a correction
// dispatch failure, a worktree-leak recovery failure, or a real tree-diff leak;
// loopNext otherwise.
func (cr *cycleRun) reviewAndGuard(next Phase, dr *dispatchResult) (loopAction, error) {
	// Workstream E2: per-phase deliverable review gate. Runs ONLY for
	// non-SKIPPED verdicts (a SKIPPED phase produced no deliverable to
	// review) and BEFORE the tree-diff guard + ledger append, so a reject
	// aborts the cycle without recording the phase as a success. The
	// default reviewer is noopReviewer (every phase approved) so opt-out
	// is byte-identical to pre-E2. On reject the correction loop below
	// re-dispatches up to the configured correction limit before aborting.
	if cr.o.reviewer != nil && dr.resp.Verdict != VerdictSKIPPED {
		rin := ReviewInput{
			Phase:           string(next),
			WorktreeBaseSHA: cr.cs.WorktreeBaseSHA,
			Response:        dr.resp,
			Workspace:       cr.cs.WorkspacePath,
			Worktree:        dr.phaseWorktree,
			ProjectRoot:     cr.req.ProjectRoot,
		}
		// baseRoutingCLI is the routing CLI this phase was DISPATCHED with (the
		// advisor overlay under model_routing=auto; empty otherwise). The
		// contract-block escalation below temporarily overrides it for a
		// re-dispatch and this value is restored when the ladder ends, so the
		// phase's own routing is never mutated (scoping constraint 1 in
		// contract_escalation.go).
		baseRoutingCLI := dr.phaseReq.ModelRoutingCLI
		// escalated latches once a contract-block escalation has re-pointed a
		// re-dispatch, so the demotion WARN can state truthfully whether
		// escalation was tried (it is a no-op for a phase whose whole chain is
		// one CLI family).
		escalated := false
		// salvageRetried latches once the no-escalation-target remedy (scoping
		// constraint 5) has turned a correction into a structured re-prompt, so
		// the demotion record can distinguish "a remedy was tried and failed"
		// from "no remedy was possible". Disjoint from escalated by construction:
		// the remedy fires only where contractEscalationCLI found no target.
		salvageRetried := false
		// prevBlockIdentity is the defect identity (violation-code set +
		// normalized reason) of the block that
		// triggered the previous correction on THIS ladder ("" ⇒ none seen yet,
		// e.g. a breaker left hot by an earlier cycle). It gates the escalation
		// trigger on defect identity, not just block count. The zero value
		// means no block seen yet.
		var prevBlockIdentity contractBlockIdentity
		rr := cr.reviewDeliverable(next, rin, contractDispatch{cli: baseRoutingCLI})
		// Contract-correction retry: on a deliverable-contract reject,
		// re-dispatch the phase with the violation injected as a
		// "## Correction" directive (bounded by policy, default 2). This re-runs
		// runner.Run directly (no
		// bridge-timeout retry on corrections — see the design's scope note).
		maxCorrections := cr.correctionLimitFor(next, cr.retryConfig.ContractCorrectionRetries)
		// ADR-0045 I1: a correction re-dispatch is an interaction — every
		// rung of ONE correction decision shares a DecisionID, and each
		// re-dispatch records an outcome resolved by its verdict + the
		// re-review. The I2 ladder's salvage/live-fix rungs will join
		// this same decision when they ship.
		irec := interaction.NewRecorder(cr.cs.WorkspacePath)
		decisionID := ""
		if !rr.Approve && (maxCorrections > 0 || cr.o.contractVerifier != nil) {
			decisionID = fmt.Sprintf("%s-c%d-%d", next, cr.cycle, cr.o.now().UnixNano())
		}
		// ADR-0045 I2: graduated correction ladder. The DECISION is the
		// pure interaction.NextCorrection CoR (salvage → live_fix →
		// redispatch, cheapest first); EXECUTION is stage-gated here.
		// Salvage gets budget only when a breaker-neutral verifier is
		// wired. Rung 2 (live_fix) is decision-complete but
		// execution-dormant at v1: the orchestrator does not yet request
		// named sessions, so NamedREPL is hard-false until the session
		// request + reaper plumbing lands (the C1→C3 deferred-unification
		// precedent; see interaction/correction.go).
		rungBudget := map[string]int{
			interaction.RungSalvage:    1,
			interaction.RungLiveFix:    1,
			interaction.RungRedispatch: maxCorrections,
		}
		if cr.o.contractVerifier == nil {
			rungBudget[interaction.RungSalvage] = 0
		}
		corr := 0
		salvagedFromInvalid := "" // found-but-invalid origin → rung-3 kernel evidence
		for !rr.Approve {
			act := interaction.NextCorrection(interaction.CorrectionInput{
				Phase:      string(next),
				Workspace:  cr.cs.WorkspacePath,
				Worktree:   dr.phaseWorktree,
				Violation:  rr.Reason,
				NamedREPL:  false, // v1: no named-session request plumbing yet
				Busy:       false,
				DecisionID: decisionID,
				RungBudget: rungBudget,
			})
			if act.Rung == "" {
				break // ladder exhausted → abort below, exactly as today
			}
			rungBudget[act.Rung]-- // every iteration spends budget: the loop is finite
			if act.Rung == interaction.RungSalvage {
				salvEv := interaction.Event{
					Kind:       interaction.KindSalvage,
					Phase:      string(next),
					Cycle:      cr.cycle,
					Trigger:    "contract_reject",
					Rung:       interaction.RungSalvage,
					DecisionID: decisionID,
					Payload:    rr.Reason,
				}
				if cr.o.cfg.PhaseRecovery != config.StageEnforce {
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: would-salvage misplaced deliverable (%s; EVOLVE_PHASE_RECOVERY=%s)\n", next, act.Reason, cr.o.cfg.PhaseRecovery)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultWouldAct})
					continue
				}
				salvStart := cr.o.now()
				sr := cr.o.salvageDeliverable(cr.ctx, rin)
				switch {
				case sr.Relocated && sr.Verified:
					// Never-upgrades-verdict: the relocated artifact faces
					// the SAME gate — the breaker-touching FINAL outcome.
					rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
					res := interaction.ResultRejectedAgain
					if rr.Approve {
						res = interaction.ResultAccepted
					}
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvaged %s → contracted path (gate approve=%v)\n", next, sr.From, rr.Approve)
					irec.Record(interaction.Outcome{Event: salvEv, Result: res, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				case sr.Relocated:
					salvagedFromInvalid = sr.From
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvage relocated %s but the destination failed verification — falling through\n", next, sr.From)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultFoundButInvalid, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				default:
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: nothing to salvage (%s)\n", next, sr.Reason)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultNotFound, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				}
				continue
			}
			if act.Rung == interaction.RungLiveFix {
				// Unreachable at v1 (NamedREPL hard-false). Termination
				// invariant for when the named-session plumbing lands:
				// the decrement above already spent this rung's budget,
				// and live_fix never mutates rr — so the `continue` is
				// safe under `for !rr.Approve` and the ladder stays
				// finite (every iteration spends budget or breaks).
				continue
			}
			corr++
			fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract violation (correction %d/%d) — re-dispatching with correction: %s\n",
				next, corr, maxCorrections, rr.Reason)
			// CLI escalation (inbox contract-block-cli-escalation): from the
			// correction answering the SECOND consecutive block, re-dispatch on
			// a different CLI FAMILY instead of asking the CLI that just
			// mis-formatted the deliverable twice to try a third time. Scoped to
			// THIS re-dispatch only — the profile and the phase's own routing are
			// untouched, and the soft overlay keeps the original chain behind the
			// escalated primary. No target (already on the universal family with
			// no other family configured) ⇒ the correction becomes a structured
			// re-prompt on the SAME routing instead (scoping constraint 5) — the
			// ladder no longer falls through unremedied there.
			// The count alone is not the signature: the block must also be the
			// SAME defect as the one that triggered the previous correction
			// (contractBlocksShareIdentity, scoping constraint 4) — two different
			// violations are two honest defects, not an incapable CLI.
			salvageRetry := false
			if rr.Blocks >= contractEscalateAtBlock && contractBlocksShareIdentity(prevBlockIdentity, rr.Reason) {
				failedCLI := cr.contractDispatchCLI(next, dr.phaseReq.ModelRoutingCLI)
				esc, ok := cr.contractEscalationCLI(next, dr.phaseReq.ModelRoutingCLI)
				switch {
				case ok && esc != dr.phaseReq.ModelRoutingCLI:
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: CLI ESCALATION — %d consecutive contract blocks on cli=%s; correction %d re-dispatches on cli=%s (contract blocks never trigger cli_fallback, which is how a mis-formatting CLI used to demote the gate). The phase's primary routing is unchanged.\n",
						next, rr.Blocks, failedCLI, corr, esc)
					dr.phaseReq.ModelRoutingCLI = esc
					escalated = true
				case !ok:
					// The two remedies are disjoint: this arm is reached only when
					// there is genuinely no other family to escalate to, so one
					// block's round-2 budget is never spent on both.
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: CONTRACT SALVAGE RETRY — %d consecutive contract blocks on cli=%s and no other CLI family is available to escalate to; correction %d re-dispatches on the SAME cli with a structured re-prompt carrying the validator output verbatim. Breaker-neutral: no extra dispatch, no extra correction, no routing change.\n",
						next, rr.Blocks, failedCLI, corr)
					salvageRetry = true
					salvageRetried = true
				}
			}
			// Carry THIS block's identity to the next iteration: identity is
			// compared against the immediately preceding block, matching the
			// blocker breaker's own consecutive-identical-fingerprint semantics.
			prevBlockIdentity = newContractBlockIdentity(rr.Reason)
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "contract_correction",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN contract_correction ledger append: %v\n", lerr)
			}
			// Payload carries the violation that TRIGGERED this dispatch;
			// a rejected_again outcome's NEW violation appears as the next
			// iteration's payload (trigger semantics, not result semantics).
			corrEv := interaction.Event{
				Kind:       interaction.KindCorrectionRedispatch,
				Phase:      string(next),
				Cycle:      cr.cycle,
				Trigger:    "contract_reject",
				Rung:       "redispatch",
				DecisionID: decisionID,
				Payload:    rr.Reason,
			}
			corrStart := cr.o.now()
			recordCorrection := func(res string) {
				irec.Record(interaction.Outcome{
					Event:     corrEv,
					Result:    res,
					LatencyMS: cr.o.now().Sub(corrStart).Milliseconds(),
					CostUSD:   dr.resp.CostUSD,
				})
			}
			directive := composeCorrection(rr.Reason, rr.Remediation)
			if salvageRetry {
				directive = composeContractSalvageRetry(rr.Reason, rr.Remediation)
			}
			if cr.o.cfg.PhaseRecovery == config.StageEnforce {
				// Evidence-enriched re-dispatch (I2 rung 3): kernel-verified
				// facts only — never agent self-assessment. Shadow keeps
				// today's directive byte-identical.
				if digest := kernelEvidenceDigest(dr.phaseWorktree, salvagedFromInvalid); digest != "" {
					directive += "\n\n" + digest
				}
				// Consume-once: the found-but-invalid note describes what
				// rung 1 discovered for the IMMEDIATE next dispatch; after
				// that agent has rewritten the artifact, repeating the old
				// path would be stale, not kernel-verified.
				salvagedFromInvalid = ""
			}
			dr.phaseReq.CorrectionDirective = directive
			obsCancel := cr.o.observer.Start(cr.ctx, string(next), dr.phaseReq)
			var rerr error
			dr.resp, rerr = dr.runner.Run(cr.ctx, dr.phaseReq)
			if obsCancel != nil {
				obsCancel()
			}
			if rerr != nil {
				recordCorrection(interaction.ResultDispatchFailed)
				phaseErr := fmt.Errorf("phase %q correction %d dispatch failed: %w", next, corr, rerr)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			// A correction re-dispatch must produce a canonical verdict to be
			// evaluable, same invariant the outer attempt loop enforces before
			// breaking. Corrections deliberately skip the bridge-timeout retry
			// ladder (scope note), so a non-canonical result here aborts rather
			// than retrying.
			if !IsVerdict(dr.resp.Verdict) {
				recordCorrection(interaction.ResultNonCanonicalVerdict)
				phaseErr := fmt.Errorf("phase %q correction %d produced a non-canonical verdict %q", next, corr, dr.resp.Verdict)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			// rin.Response is refreshed for reviewer consistency; the deliverable
			// reviewer reads the filesystem (workspace/worktree), not this field.
			rin.Response = dr.resp
			rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
			if rr.Approve {
				recordCorrection(interaction.ResultAccepted)
			} else {
				recordCorrection(interaction.ResultRejectedAgain)
			}
		}
		// Defensive: phaseReq is fresh per phase iteration, but never let the
		// directive — or a contract-block CLI escalation — outlive the loop. The
		// escalation is scoped to the failing re-dispatch by construction; the
		// restore is what keeps it from becoming a phase-wide reroute if a later
		// consumer of dr.phaseReq is ever added.
		dr.phaseReq.CorrectionDirective = ""
		dr.phaseReq.ModelRoutingCLI = baseRoutingCLI
		if !rr.Approve {
			if maxCorrections == 0 {
				// Byte-identical to the pre-feature abort message.
				phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
				// ADR-0044 C1: the phase ran and produced its own verdict;
				// the reject is recorded as the abort reason, not a rewrite.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, 1)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s", next, maxCorrections, rr.Reason)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
			cr.recordFailureLearning(next, phaseErr, maxCorrections)
			return loopAbort, wrapCycleLevelError(next, phaseErr)
		}
	}

	if next == PhaseShip && dr.resp.Verdict == VerdictPASS {
		// Ship landed AND survived the deliverable review gate above — the
		// worktree is merged, normal exit cleanup applies. Deliberately
		// AFTER the review gate: a review-rejected ship abort must still
		// preserve the worktree for triage (ADR-0039 §8 / D10).
		cr.preserveWorktree = false
		// Latch ship success so a subsequent post-ship observer failure
		// (memo / post-ship-monitor) degrades to WARN rather than turning a
		// shipped cycle abnormal — read by postShipObserverSkip on the dispatch
		// abort path (cycle-574 memo-phase-tier-envelope).
		cr.shipped = true
	}

	// Workstream B: post-phase tree-diff check. Runs BEFORE the ledger
	// append so a leak aborts the cycle without recording the phase as a
	// success. Snapshot failures (pre OR post) degrade silently — the
	// guard is belt-and-suspenders to the OS sandbox, so a transient git
	// read error must never cause a false abort.
	// Cycle-160 fix (Option A): a non-Claude builder (agy/codex in tmux) is
	// not bound by the Claude-only role-gate, and the OS sandbox is off on
	// nested-macOS, so it can write build output to the MAIN tree instead of
	// its worktree. Relocate any such leak into the worktree (staged, so audit
	// sees it) and restore main BEFORE the tree-diff guard runs. Runs
	// unconditionally after build (no-op when clean) because the guard's
	// `git diff --name-only HEAD` baseline is tracked-only and misses
	// pure-untracked leaks. On recovery FAILURE we abort explicitly — the
	// tree-diff guard only backstops tracked leaks, so a failed recovery
	// of an untracked leak must not slip past into audit.
	if cr.o.leakRecoverablePhase(next) && cr.cs.ActiveWorktree != "" {
		if !recoverBuildLeak(cr.ctx, cr.req.ProjectRoot, cr.cs.ActiveWorktree, cr.mainDirtyBaseline, cr.o.worktreePhase(next)) {
			phaseErr := fmt.Errorf("phase %s: worktree-leak recovery failed (main tree left unsafe for audit)", next)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
			cr.recordFailureLearning(next, phaseErr, 1)
			return loopAbort, phaseErr
		}
	}

	if dr.treeGuard != nil && !dr.snapshotFailed {
		res := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
		if res.SnapshotMissed {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff post-phase snapshot failed for %s (sandbox guard degraded; not aborting)\n", next)
		} else if !res.OK() {
			// Attempt phase-agnostic binary churn discard for build artifacts
			var relBin string
			if execPath, err := os.Executable(); err == nil {
				if rel, err := filepath.Rel(cr.req.ProjectRoot, execPath); err == nil && !strings.HasPrefix(rel, "..") {
					relBin = filepath.ToSlash(rel)
				}
			}

			// Always discard "go/evolve" and relBin (if set)
			_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, "go/evolve")
			if relBin != "" && relBin != "go/evolve" {
				if isGitignored(cr.ctx, cr.req.ProjectRoot, relBin) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN: relBin path %q is gitignored; skipping discardMainLeak to prevent checkout error\n", relBin)
				} else {
					_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, relBin)
				}
			}

			// Re-snapshot and check again
			res2 := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
			if res2.OK() {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: discarded binary rebuild churn in phase %s; continuing\n", next)
			} else {
				// Filter the leaked set through isLegitimateMainTreePath for EVERY
				// phase — the same classification recoverBuildLeak applies (R9: one
				// shared vocabulary). Non-worktree phases need it for their
				// .evolve/ workspace writes (R7); worktree phases need it because
				// orchestrator-side gates write their own untracked runtime state
				// (.evolve/contract-gate-breaker.json) into the main tree mid-phase
				// — recovery skips those by design, so a strict guard here turned
				// every contract-gate trip into a false cycle abort (the cycle-274
				// salvage CI regression). PLUS a guard-only second classifier,
				// isScoutEvalMaterialization: scout writes its selected evals to the
				// main tree by contract (materialization.go), which recoverBuildLeak
				// never sees (scout is not a WorktreePhase) so it lives only here
				// (soak-#6 cycle 318→319). PLUS a third guard-only classifier,
				// isActiveMintPhasePath: in fleet mode a CONCURRENT lane's advisor
				// mint persists .evolve/phases/<name>/phase.json into the SHARED
				// tree this lane diffs, charging the mint to an innocent phase
				// (cycle-967 false-abort). The registrar records minted names in
				// the shared mintregistry before persisting, so a registered,
				// TTL-fresh name is mint infrastructure, not a leak; an
				// UNREGISTERED phase-config write still aborts. A registry read
				// error only disables the exemption (guard stays armed — the
				// fail-safe direction). Real escapes stay armed: source files and
				// non-scout/non-eval deliverable paths classify as leaks, and
				// porcelainDirtySet emits both rename sides so a deliverable renamed
				// to a .evolve/evals/ look-alike still aborts via its source path.
				leaked := res2.Leaked
				mints, mintErr := mintregistry.ActiveNames(mintregistry.Path(cr.req.ProjectRoot), time.Now())
				if mintErr != nil {
					// ABNORMAL, not WARN: a corrupt registry is either damage or a
					// deliberate availability attack (a lane-wide exemption outage
					// reproduces the cycle-967 false-abort). Quarantine bounds the
					// outage to this one check; the guard stays armed either way.
					fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL tree-diff: mint registry unreadable (%v); mint exemption disabled for this check\n", mintErr)
					if _, qErr := mintregistry.QuarantineCorrupt(mintregistry.Path(cr.req.ProjectRoot)); qErr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: mint registry quarantine failed: %v\n", qErr)
					}
				} else {
					mints = verifiedActiveMints(cr.req.ProjectRoot, mints)
				}
				realLeaks, waived := filterRealLeaks(next, leaked, mints, cr.consoleLeased, os.Stderr)
				if len(realLeaks) == 0 {
					if waived > 0 {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: phase %s continued only because %d leaked path(s) were console-leased (ADR-0080 S4) — not a clean phase\n", next, waived)
					} else {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: phase %s wrote only legitimate main-tree paths (.evolve/ workspace); continuing\n", next)
					}
					leaked = nil
				} else {
					leaked = realLeaks
				}
				if len(leaked) > 0 {
					phaseErr := fmt.Errorf("tree-diff guard: phase %q wrote to the main tree outside its worktree %q — leaked paths: %v",
						string(next), dr.phaseWorktree, leaked)
					// ADR-0044 C1 — THE cycle-262 path: the build ran, PASSed,
					// and burned tokens before the guard caught its main-tree
					// leak. The abort is correct; erasing the outcome was not.
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
					cr.recordFailureLearning(next, phaseErr, 1)
					// After abort, check if go/bin/evolve is absent
					evolveBinPath := filepath.Join(cr.req.ProjectRoot, "go/bin/evolve")
					if _, err := os.Stat(evolveBinPath); os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL: go/bin/evolve absent after cycle abort — trust-kernel guards degraded\n")
					}
					return loopAbort, phaseErr
				}
			}
		}
	}

	return loopNext, nil
}

// filterRealLeaks applies the tree-diff guard's classifier chain to the
// leaked set: workspace legitimacy, scout eval materialization, registered
// TTL-fresh mints, and — LAST, loud (ADR-0080 S4) — the cycle-start-adopted
// console lease, which waives EXACT paths only and prints one WARN per
// waiver so a leased leak can never masquerade as a clean phase.
func filterRealLeaks(next Phase, leaked []string, mints, leased map[string]bool, warn io.Writer) (real []string, waived int) {
	for _, p := range leaked {
		if isLegitimateMainTreePath(p) || isScoutEvalMaterialization(next, p) || isActiveMintPhasePath(mints, p) {
			continue
		}
		if leased[p] {
			waived++
			fmt.Fprintf(warn, "[orchestrator] WARN tree-diff: leaked path %q WAIVED by the cycle-start console lease (ADR-0080 S4) — operator-leased, not a clean phase\n", p)
			continue
		}
		real = append(real, p)
	}
	return real, waived
}

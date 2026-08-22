package core

// contract_escalation.go — CONTRACT-BLOCK CLI ESCALATION (inbox
// contract-block-cli-escalation, P1 weight 0.95).
//
// The defect this closes, confirmed live twice: the correction ladder in
// reviewAndGuard re-dispatches the SAME profile CLI after a deliverable-contract
// block. The profile's cli_fallback chain fires only on infra exit codes
// {80,81,85,124,127} — never on a contract violation — so a CLI that
// systematically mis-formats a deliverable burns every correction and the
// contract-gate breaker opens, demoting enforce→advisory for the rest of the run.
// Batch-19 (cycles 1171/1172, adversarial-review) and batch-21 (cycle-1215,
// triage) both ended that way: a FORMAT-compliance failure silently WEAKENED a
// gate. The correct escape hatch is CLI escalation, not gate demotion.
//
// Three deliberate scoping constraints:
//
//  1. ESCALATE THE RE-DISPATCH, NOT THE PHASE (from the inbox item). The
//     non-compliance lives on the rare failure path (triage's v1 FAIL sentinel,
//     adversarial-review's section headings) while the same CLI ships the common
//     path fine. Rerouting the phase would change 99% of dispatches to fix 1%. So
//     the escalation is applied to PhaseRequest.ModelRoutingCLI on the
//     re-dispatch only — a SOFT overlay (llmroute.ApplySoftOverlay) that promotes
//     the target to chain primary while keeping the profile's own chain behind it
//     — and is reverted when the ladder ends. The profile on disk is never touched.
//
//  2. NOT ON THE FIRST BLOCK. One malformed turn is a bad turn, not a CLI
//     verdict. Escalation starts at the CONTRACT GATE'S OWN second consecutive
//     block (ReviewResult.Blocks, reported by the breaker that will open the
//     circuit — never a locally re-counted correction ordinal, which desyncs from
//     the breaker whenever a prior cycle left the count hot or the salvage rung
//     consumed a block). That is still before the third strike, so the circuit
//     stays the last resort.
//
//  3. ONLY A CONTRACT BLOCK ESCALATES. Blocks==0 means the rejecting reviewer in
//     the chain keeps no contract-block counter (evalgate / topngate / triagecap /
//     the build floor). Those are task-binding or capacity rejections, not
//     format-compliance failures, so a different CLI is not the remedy and the
//     ladder behaves exactly as before.
//
//  4. ONLY AN IDENTICAL BLOCK ESCALATES (cycle-1289). ReviewResult.Blocks counts
//     blocks, not defects: two genuinely DIFFERENT contract violations on one
//     phase (block 1 misses a section heading, block 2 misses the verdict
//     sentinel) are two honest defects, not one incapable-CLI signature, and
//     round 2's budget should not buy a different CLI family for them. The
//     trigger is therefore gated on failure IDENTITY as well as count — see
//     contractBlocksShareIdentity.
//
//  5. A TRIGGER WITH NO TARGET STILL GETS A REMEDY (cycle-1300, from this item's
//     LIVE EVIDENCE note of 2026-08-05). When a phase's whole dispatch chain is
//     one CLI family, contractEscalationCLI returns ok=false and — before this
//     cycle — the ladder did nothing at all: the same incapable CLI got the same
//     plain directive a third time and the breaker opened, so the ratchet failed
//     OPEN purely because there was nowhere to escalate to. The TOP-FAMILY
//     remedy is a structured re-prompt (composeContractSalvageRetry) on the
//     re-dispatch the ladder was already performing. It is disjoint from
//     escalation — a phase WITH a target escalates and does not re-prompt, so one
//     block's budget is never spent twice — and it is BREAKER-NEUTRAL: no extra
//     dispatch, no extra correction, no change to ModelRoutingCLI (there is no
//     other family: that is the whole premise), so the circuit still opens on the
//     third strike as the last resort.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// contractEscalateAtBlock is the contract gate's consecutive-block count at
// which a correction re-dispatch escalates its CLI. Two: block 1's correction
// retries the same CLI (one bad turn is not a CLI verdict), block 2's correction
// escalates, and block 3 — the breaker's default threshold — stays the last resort.
const contractEscalateAtBlock = 2

// universalContractFallbackCLI is the escalation target for a phase whose chain
// offers no other CLI family: the same universal default llmroute resolves when
// nothing names a CLI (llmroute.resolvePrimary). Pinned against that resolver by
// TestUniversalContractFallbackMatchesLLMRouteDefault so the two cannot drift.
const universalContractFallbackCLI = "claude-tmux"

// contractSalvageRetryDirectiveHeading marks a correction directive as a
// STRUCTURED RE-PROMPT rather than the plain rejection framing composeCorrection
// emits — the TOP-FAMILY remedy for the case constraint 5 below describes.
const contractSalvageRetryDirectiveHeading = "## Contract Salvage Retry — verbatim validator output"

// composeContractSalvageRetry is the remedy for a contract block that WOULD have
// escalated but has nowhere to escalate to (scoping constraint 5). It enriches
// the correction the ladder was already going to re-dispatch: same CLI, same
// round, same budget — only the directive changes, from a paraphrasable
// rejection notice into an explicit diagnosis carrying the validator's output
// VERBATIM under a distinct heading.
//
// Breaker-neutral by construction: it adds no dispatch and consumes no extra
// correction, so ReviewResult.Blocks — the breaker's own counter — is untouched
// by the remedy itself and the circuit still opens on the third strike as the
// last resort. The repair-economics rationale (arXiv:2306.09896, cited by the
// inbox item) is to spend the harder round's budget on the DIAGNOSIS when buying
// a different CLI family is not on the menu.
// remediation, when the triggering gate supplied one, is threaded through
// composeCorrection and changes the closing clause below (see salvageClosing).
func composeContractSalvageRetry(reason, remediation string) string {
	return composeCorrection(reason, remediation) + "\n\n" + contractSalvageRetryDirectiveHeading + "\n\n" +
		"This is the second consecutive block reporting the SAME defect, and no other CLI family is " +
		"available to escalate to — this is the last correction before the contract gate's circuit " +
		"breaker opens and the gate stops enforcing for the rest of this run.\n\n" +
		"The contract validator's output, verbatim:\n\n" + reason + "\n\n" +
		"Do not re-summarize it. Take each bracketed [violation_code] above in turn, state the exact " +
		"section heading or file path that code refers to, then re-emit the whole deliverable at the " +
		"contracted path with that specific defect closed. " + salvageClosing(remediation)
}

// salvageClosing picks the salvage rung's final clause. The default forbids
// collateral edits; when the gate supplied a remedy that clause must not fire,
// because a remediation exists only for violations whose fix is to CREATE
// something.
//
// REACHABILITY, stated honestly: this branch is currently DEAD in production and
// is forward-looking parity, not the fix. The salvage rung only fires at
// rr.Blocks >= contractEscalateAtBlock, and Blocks is set solely by
// internal/deliverable, while Remediation is set solely by internal/evalgate —
// no gate can satisfy both today. The 0-for-4 Gate A failures were "rejected
// after 2 correction(s)" via maxCorrections exhaustion in the ORDINARY
// correction loop, which goes through composeCorrection — that is the call that
// actually closes the gap. This exists so the two directive paths cannot drift
// if a future gate ever sets both.
func salvageClosing(remediation string) string {
	if remediation != "" {
		return "Change nothing else beyond what the remedy above requires."
	}
	return "Do not change unrelated files."
}

// ledgerKindContractGateDemoted is the ledger Kind recorded when the contract
// gate's breaker opens. The demotion used to be one stderr line and therefore
// invisible in the cycle record; as a ledger entry it is bound to the cycle's
// audit chain like every other abnormal event.
const ledgerKindContractGateDemoted = "contract_gate_demoted"

// contractDispatch describes the dispatch the deliverable under review came from:
// cli is the routing override in force ("" ⇒ resolve the profile/env default) and
// escalated records whether a contract-block escalation put it there — so the
// demotion WARN can state truthfully whether escalation was tried.
//
// salvageRetried is the same fact for the OTHER remedy: a demotion where a
// structured re-prompt was tried and failed is a different diagnosis from one
// where no remedy was possible at all, and an escalated=false entry alone cannot
// separate them (which is the exact ambiguity the live-evidence note reports).
type contractDispatch struct {
	cli            string
	escalated      bool
	salvageRetried bool
}

// contractEscalationProfile resolves the profile governing a phase, plus the
// AGENT name whose EVOLVE_<AGENT>_CLI env key the dispatch resolver reads.
//
// Two lookups, in precedence order: the built-in phase→agent table, then the
// `<phase>.json` convention every MINTED/user phase follows. The second is
// load-bearing, not defensive: phaseAgentName covers only the 10 built-in spine
// phases, so adversarial-review — the phase in this fix's own batch-19 evidence,
// which has a real .evolve/profiles/adversarial-review.json — resolves to nil
// through the built-in table alone and could never have escalated.
func (cr *cycleRun) contractEscalationProfile(phase Phase) (*profiles.Profile, string) {
	loader := profiles.NewFromDir(filepath.Join(cr.req.ProjectRoot, ".evolve", "profiles"))
	if loader == nil {
		return nil, string(phase)
	}
	for _, agent := range []string{phaseAgentName[string(phase)], string(phase)} {
		if agent == "" {
			continue
		}
		if prof, err := loader.Get(agent); err == nil {
			return &prof, agent
		}
	}
	return nil, string(phase)
}

// contractDispatchCLI names the CLI a phase's deliverable was actually produced
// by: the routing override when one is in force, else the primary the dispatch
// resolver would pick. It goes through llmroute.Resolve rather than reading
// profile.CLI because EVOLVE_<AGENT>_CLI / EVOLVE_CLI outrank the profile — a
// WARN (or an escalation family test) computed from profile.CLI would name a CLI
// that never ran.
func (cr *cycleRun) contractDispatchCLI(phase Phase, override string) string {
	if override != "" {
		return override
	}
	prof, agent := cr.contractEscalationProfile(phase)
	if plan := llmroute.Resolve(agent, string(phase), "", cr.envSnap, prof, nil, nil); len(plan.Candidates) > 0 {
		return plan.Candidates[0]
	}
	return universalContractFallbackCLI
}

// contractEscalationCLI picks the CLI a contract-blocked re-dispatch escalates
// to: the first candidate in the phase's resolved dispatch chain belonging to a
// DIFFERENT family than the one that just failed, else the universal claude
// fallback. dispatchedCLI is the CLI the blocks are attributable to.
//
// The family test is what makes this an escalation rather than a shuffle — a
// same-family sibling driver runs the same model through the same prompt renderer
// and would reproduce the identical format violation. A phase whose whole chain
// is one family, and that family is the universal fallback's, therefore has no
// target and returns ok=false: the ladder then behaves exactly as before.
func (cr *cycleRun) contractEscalationCLI(phase Phase, dispatchedCLI string) (string, bool) {
	prof, agent := cr.contractEscalationProfile(phase)
	plan := llmroute.Resolve(agent, string(phase), "", cr.envSnap, prof, nil, nil)
	failed := llmroute.Family(cr.contractDispatchCLI(phase, dispatchedCLI))
	for _, c := range plan.Candidates {
		if c == "" || llmroute.Family(c) == failed {
			continue
		}
		if cr.escalationAllowed(phase, c, prof) {
			return c, true
		}
	}
	if llmroute.Family(universalContractFallbackCLI) != failed &&
		cr.escalationAllowed(phase, universalContractFallbackCLI, prof) {
		return universalContractFallbackCLI, true
	}
	return "", false
}

// contractBlocksShareIdentity reports whether the contract block now on the
// ladder is the SAME defect as the block that triggered the previous correction,
// which is the second half of the escalation trigger (scoping constraint 4).
//
// Identity is the block's VIOLATION-CODE SET, not its rendered text (cycle-1291,
// repairing the cycle-1289 audit defect). The reason under comparison is
// deliverable.summarize() — a "; "-joined rendering of EVERY violation on the
// block as "[code] message" — so a whole-string compare reads a PARTIALLY
// REPAIRED defect set as a different defect: block 1 reports
// {missing_section, missing_verdict}, the correction closes one, block 2 reports
// {missing_verdict} alone, the two strings differ, and the escalation is
// suppressed exactly where the incapable-CLI signature is strongest (the CLI
// demonstrably cannot close the remaining violation). Superset regressions and
// re-ordered/re-worded renderings of ONE set fail the same way.
//
// deliverable.Violation.Code is the stable identity primitive, untouched by
// prose rewording or violation order, so two blocks are the SAME defect exactly
// when their code sets INTERSECT — which covers subset, superset and equal, and
// still separates the disjoint sets constraint 4 exists to keep apart. The codes
// reach here as plain data parsed out of the rendered reason: internal/deliverable
// imports internal/core (reviewer.go, verifier.go) and core imports deliverable
// nowhere, so a []deliverable.Violation field on ReviewResult would be an import
// cycle.
//
// FAIL-SAFE: not every reason on this path is a summarize() rendering. When
// EITHER block yields no code, identity falls back to failure_digest.go's
// normalizeReasonForFingerprint — the blocker breaker's own primitive, which
// projects a reason onto its defect identity by dropping identity-noise tokens
// (go-test durations, narrative verdicts). Reading "no codes on either side" as
// "∅ ∩ ∅ ⇒ different defect" would silently delete the ladder for every
// non-summarize reason shape.
//
// The rule is "prior reason known AND differing ⇒ suppress", NOT "equal ⇒
// escalate". The difference is the hot-breaker edge: the contract-gate breaker is
// process-global, so a cycle that aborted mid-ladder leaves it hot and the next
// phase can arrive at Blocks >= contractEscalateAtBlock on its ladder's FIRST
// block — where no prior block exists to compare. Requiring equality there would
// silently delete the escape hatch that constraint 2 deliberately keeps open, so
// the zero-value prev (no block observed yet) reports true.
func contractBlocksShareIdentity(prev contractBlockIdentity, reason string) bool {
	if !prev.observed {
		return true
	}
	cur := newContractBlockIdentity(reason)
	if len(prev.codes) > 0 && len(cur.codes) > 0 {
		for code := range cur.codes {
			if _, ok := prev.codes[code]; ok {
				return true
			}
		}
		return false
	}
	return cur.normalized == prev.normalized
}

// contractBlockIdentity is ONE contract block's defect identity, computed once
// per block by the caller and carried forward to the next iteration: the set of
// violation codes the block reported, plus the fingerprint-normalized reason the
// code-less fail-safe compares. observed distinguishes "a prior block reported
// no codes and an empty reason" from "no prior block at all" (the hot-breaker
// edge) — a distinction the previous empty-string sentinel could not make.
type contractBlockIdentity struct {
	observed   bool
	normalized string
	codes      map[string]struct{}
}

// contractViolationCodeRE matches the "[code]" tokens deliverable.summarize()
// emits. The class is deliberately narrow — code-shaped tokens only, no spaces —
// so bracketed prose inside a violation MESSAGE cannot masquerade as a code and
// fabricate an intersection between two unrelated defects.
var contractViolationCodeRE = regexp.MustCompile(`\[([A-Za-z0-9_.:-]+)\]`)

// contractArtifactDetermined reports whether repairing the deliverable
// violation named by code NECESSARILY rewrites the bytes of the watched
// artifact. It is the precondition a hash-equality check needs before it may
// read "the artifact hash did not move" as "the agent did no new work": that
// inference is sound only for violations whose ONLY repair is an edit to the
// artifact itself.
//
// The counter-example the classifier exists for is deliverable.CodeStrayInWorktree
// (deliverable.go:306), which is repaired by DELETING a stray copy elsewhere in
// the worktree — the watched artifact is left byte-identical, so an unchanged
// hash there is not evidence of inaction and a short-circuit reading it that way
// would turn a repairable contract block into a deterministic ladder abort
// (inst-L1508a).
//
// ALLOWLIST, not denylist, and therefore fail-closed: a code this function has
// never heard of is one whose repair mechanics are unknown, and the only safe
// answer for an unknown repair mechanic is false — "do not skip the re-verify".
// False negatives cost one redundant verification; a false positive silently
// drops a real repair.
//
// Deliberately NARROWER than the criterion in one respect, stated here so the
// comment is true as written: deliverable.CodeInvalidJSON and
// deliverable.CodeFailureContextMissing also repair by editing the artifact, and
// this cycle's contract (triage top_n; the cycle-1510 eval's vocabulary-drift
// grader) pins the allowlist to six. Widening to those two is queued as
// follow-up work rather than taken here — the omission errs in the fail-closed
// direction, so it cannot cause the unsound inference above. See the Amendments
// section of the cycle-1510 build report.
//
// The codes arrive as plain strings, never deliverable.Violation values:
// internal/deliverable imports internal/core (reviewer.go, verifier.go), so the
// reverse import would be a cycle — the same constraint documented for
// contractViolationCodeRE above. That makes drift between the two vocabularies
// invisible to the compiler, which is why it is asserted in a test instead
// (TestContractArtifactDetermined_CodesMatchDeliverableVocabulary).
//
// NO PRODUCTION CALLER YET, by design: the hash short-circuit this classifies
// for is not scheduled, and building the short-circuit alongside it would be
// scope creep (Core Rule 2). The exercised caller is the test above.
func contractArtifactDetermined(code string) bool {
	switch code {
	case "missing_artifact", // the artifact does not exist; repair writes it
		"empty_artifact",          // zero bytes; repair fills it
		"missing_section",         // a required heading is absent; repair adds it
		"missing_challenge_token", // the token is not echoed; repair echoes it
		"bad_verdict",             // the verdict sentinel is wrong; repair rewrites it
		"missing_key":             // a required JSON key is absent; repair adds it
		return true
	}
	return false
}

// newContractBlockIdentity projects one block's rejection reason onto its defect
// identity. Both projections are computed eagerly: which one the comparison uses
// depends on the OTHER block, so neither can be deferred.
func newContractBlockIdentity(reason string) contractBlockIdentity {
	id := contractBlockIdentity{observed: true, normalized: normalizeReasonForFingerprint(reason)}
	for _, m := range contractViolationCodeRE.FindAllStringSubmatch(reason, -1) {
		if id.codes == nil {
			id.codes = make(map[string]struct{})
		}
		id.codes[m[1]] = struct{}{}
	}
	return id
}

// escalationAllowed keeps the escalation inside the guardrails that bound the
// sanctioned ModelRoutingCLI writer. policy.ValidatePin is the SINGLE validator
// for a profile's allowed_clis, and the routing projection in cyclerun_dispatch
// only ever writes values that passed it (via router.ClampPlanModelRouting) — so
// escalating past it would make this the one path that can route a phase to a
// CLI family its operator forbade (e.g. tester allowed_clis=["claude"]).
func (cr *cycleRun) escalationAllowed(phase Phase, cli string, prof *profiles.Profile) bool {
	if err := policy.ValidatePin(string(phase), policy.Pin{CLI: cli}, prof); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract-escalation candidate cli=%s refused by the profile guardrails: %v\n", phase, cli, err)
		return false
	}
	return true
}

// formatContractGateDemotionWarn renders the operator-facing line for a contract
// gate that demoted itself. It names the PHASE, the CLI the blocks are
// attributable to, WHICH remedy actually ran, and the last violation — the
// batch-19 line named none of the four, which is why the same class recurred
// twice before anyone noticed.
//
// It takes the whole contractDispatch rather than a bare escalated bool because
// there are now two remedies to report and "did NOT run" is a line an operator
// trusts and acts on: it must be false only when it is false.
func formatContractGateDemotionWarn(phase, cli string, d contractDispatch, reason string) string {
	tried := "CLI escalation did NOT run for this phase (no other CLI family available in its chain, or the block count opened the circuit first)"
	switch {
	case d.escalated:
		tried = "CLI escalation already ran and the escalated CLI failed the contract too"
	case d.salvageRetried:
		tried = "no other CLI family was available to escalate to, so a structured re-prompt salvage retry attempted the repair on the same CLI — and that CLI failed the contract too"
	}
	return fmt.Sprintf("[orchestrator] WARN CONTRACT GATE DEMOTED: phase %s on cli=%s tripped the contract-gate circuit breaker — the gate is now advisory (enforce→advisory) for the rest of this run, so later phases ship UNGATED. %s. Last violation: %s",
		phase, cli, tried, reason)
}

// noteContractGateDemotion makes one gate demotion impossible to miss: a loud
// stderr WARN naming the phase + CLI, a cycle-visible ledger entry carrying the
// CLI and the violation, and a staged autofile intent so the class becomes
// queued work.
//
// The intent is STAGED (dispositionrouter), never written straight into
// .evolve/inbox: a mid-flight inbox write races inboxmover.Claim's os.Rename and
// can resurrect a claimed item into double work across fleet lanes.
// recurrence.ApplyBoundary — invoked at the loop's per-iteration boundary once no
// lane is in flight — is the only sanctioned inbox writer.
//
// Best-effort by construction: neither a ledger nor a staging failure may change
// the cycle's outcome (the gate has already decided to approve).
func (cr *cycleRun) noteContractGateDemotion(phase Phase, d contractDispatch, blocks int, reason string) {
	cli := cr.contractDispatchCLI(phase, d.cli)
	warn := formatContractGateDemotionWarn(string(phase), cli, d, reason)
	fmt.Fprintln(os.Stderr, warn)
	if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:    cr.o.now().UTC().Format(time.RFC3339),
		Cycle: cr.cycle,
		Role:  string(phase),
		Kind:  ledgerKindContractGateDemoted,
		// Action carries the decision verb + evidence so the demotion survives
		// beyond transient stderr (a Kind/Role-only entry is the content-free
		// fingerprint shape that blinded the breaker diagnostics in cycle-1117).
		//
		// salvage_attempted rides ALONGSIDE escalated= (never instead of it): the
		// two remedies are disjoint, so recurrence analytics reading this entry
		// can tell "no remedy was possible" from "a structured re-prompt was
		// tried and the CLI still could not comply".
		Action:   fmt.Sprintf("demote enforce->advisory: cli=%s escalated=%v salvage_attempted=%v blocks=%d: %s", cli, d.escalated, d.salvageRetried, blocks, reason),
		ExitCode: 0,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s ledger append: %v\n", ledgerKindContractGateDemoted, lerr)
	}
	// Weight comes from policy, never a literal here
	// (feedback_phase_settings_from_config_not_code): the repo's one knob for an
	// auto-filed inbox item's weight. A load failure still stages the intent at
	// the compiled safe default rather than dropping the escalation.
	pol, perr := policy.Load(filepath.Join(cr.req.ProjectRoot, ".evolve", "policy.json"))
	if perr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s: policy load for autofile weight: %v (using compiled default)\n", ledgerKindContractGateDemoted, perr)
	}
	if _, serr := dispositionrouter.StageIntent(
		filepath.Join(cr.req.ProjectRoot, ".evolve", "escalations"),
		dispositionrouter.Intent{
			Action: dispositionrouter.ActionAutofile,
			Route:  dispositionrouter.RouteConsole,
			ItemID: "contract-gate-demoted-" + string(phase),
			// Pattern is the recurrence identity AND the filed item's title stem
			// (recurrence.applyIntent renders "recurring failure <pattern> (<n>
			// occurrences)"), so it must read as a defect, not as a bare label.
			Pattern:    fmt.Sprintf("contract gate demoted enforce->advisory on phase %s (cli=%s)", phase, cli),
			Cycle:      cr.cycle,
			Recurrence: blocks,
			Weight:     pol.RetroAutofileDefaultWeight(),
			Reason:     warn,
		}); serr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s: stage escalation intent: %v\n", ledgerKindContractGateDemoted, serr)
	}
}

// reviewDeliverable is the ONE place reviewAndGuard consults the review gate. It
// forwards to the injected reviewer and, when the contract gate reports it
// demoted itself, records that demotion against the dispatch the reviewed
// deliverable came from. A demotion is reported even when a LATER gate in the
// chain rejected the same deliverable — the gate still stopped enforcing.
func (cr *cycleRun) reviewDeliverable(phase Phase, in ReviewInput, d contractDispatch) ReviewResult {
	rr := cr.o.reviewer.Review(cr.ctx, in)
	if rr.Demoted {
		cr.noteContractGateDemotion(phase, d, rr.Blocks, rr.Reason)
	}
	return rr
}

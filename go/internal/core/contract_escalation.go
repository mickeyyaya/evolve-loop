package core

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
// which a correction re-dispatch escalates its CLI, one strike before the
// breaker's own threshold opens the circuit.
const contractEscalateAtBlock = 2

// universalContractFallbackCLI is the escalation target for a phase whose chain
// offers no other CLI family: the same universal default llmroute resolves when
// nothing names a CLI (llmroute.resolvePrimary). Pinned against that resolver by
// TestUniversalContractFallbackMatchesLLMRouteDefault so the two cannot drift.
const universalContractFallbackCLI = "claude-tmux"

// contractSalvageRetryDirectiveHeading marks a correction directive as a
// structured re-prompt rather than the plain rejection framing
// composeCorrection emits.
const contractSalvageRetryDirectiveHeading = "## Contract Salvage Retry — verbatim validator output"

// composeContractSalvageRetry is the remedy for a contract block that would
// escalate but has no other CLI family to escalate to. It enriches the
// correction the ladder was already going to re-dispatch — same CLI, same
// round, same budget — only the directive changes, from a paraphrasable
// rejection notice into an explicit diagnosis carrying the validator's output
// verbatim under a distinct heading.
//
// Breaker-neutral by construction: it adds no dispatch and consumes no extra
// correction, so ReviewResult.Blocks — the breaker's own counter — is untouched
// by the remedy itself and the circuit still opens on the third strike as the
// last resort. remediation, when the triggering gate supplied one, is threaded
// through composeCorrection and changes the closing clause below (see
// salvageClosing).
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
func salvageClosing(remediation string) string {
	if remediation != "" {
		return "Change nothing else beyond what the remedy above requires."
	}
	return "Do not change unrelated files."
}

// ledgerKindContractGateDemoted is the ledger Kind recorded when the contract
// gate's breaker opens, binding the demotion to the cycle's audit chain like
// any other abnormal event.
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
// load-bearing, not defensive: phaseAgentName covers only the built-in spine
// phases, so a minted phase with a real .evolve/profiles/<phase>.json would
// otherwise resolve to nil through the built-in table alone and could never
// escalate.
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
// ladder is the same defect as the block that triggered the previous
// correction — the second half of the escalation trigger.
//
// Identity is the block's violation-code SET, not its rendered text: a
// whole-string compare would read a partially repaired violation set as a
// different defect, exactly where the incapable-CLI signature is strongest.
// Two blocks are the same defect when their code sets intersect, which covers
// subset, superset and equal while still separating genuinely disjoint sets.
// The codes reach here as plain data parsed out of the rendered reason,
// because internal/deliverable imports internal/core and the reverse would be
// an import cycle.
//
// When either block yields no code, identity falls back to
// failure_digest.go's normalizeReasonForFingerprint (the blocker breaker's own
// primitive), because reading "no codes on either side" as "different defect"
// would silently disable the ladder for every non-summarize reason shape.
//
// The rule is "prior reason known AND differing ⇒ suppress", never "equal ⇒
// escalate": the contract-gate breaker is process-global, so a cycle that
// aborts mid-ladder leaves it hot and the next phase's ladder can start at
// block 2 with no prior block to compare — the zero-value prev (nothing
// observed) reports true, keeping that escape hatch open.
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
// The counter-example is deliverable.CodeStrayInWorktree, which is repaired by
// deleting a stray copy elsewhere in the worktree — the watched artifact is
// left byte-identical, so an unchanged hash there is not evidence of inaction.
//
// Allowlist, not denylist, and therefore fail-closed: a code this function has
// never heard of has unknown repair mechanics, and the only safe answer is
// false. A false negative costs one redundant verification; a false positive
// would silently drop a real repair.
//
// Deliberately narrower than the criterion in one respect: CodeInvalidJSON and
// CodeFailureContextMissing also repair by editing the artifact but are not
// yet in the allowlist; widening it is follow-up work, and the omission errs
// in the fail-closed direction.
//
// The codes arrive as plain strings, never deliverable.Violation values,
// because internal/deliverable imports internal/core and the reverse would be
// a cycle; drift between the two vocabularies is therefore asserted in
// TestContractArtifactDetermined_CodesMatchDeliverableVocabulary rather than
// caught by the compiler.
//
// No production caller yet: the hash short-circuit this classifies for is not
// scheduled, so building it alongside this classifier would be scope creep.
// The test above is the only exerciser.
func contractArtifactDetermined(code string) bool {
	switch code {
	case "missing_artifact", // the artifact does not exist; repair writes it
		"empty_artifact",          // zero bytes; repair fills it
		"missing_section",         // a required heading is absent; repair adds it
		"missing_challenge_token", // the token is not echoed; repair echoes it
		"bad_verdict",             // the verdict sentinel is wrong; repair rewrites it
		"failure_class_unknown",   // the failure block's class is outside the vocabulary; repair rewrites it
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

// formatContractGateDemotionWarn renders the operator-facing line for a
// contract gate that demoted itself. It names the phase, the CLI the blocks
// are attributable to, which remedy actually ran, and the last violation, so
// an operator can tell a real capacity gap from a CLI that failed twice.
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
		// beyond transient stderr; a Kind/Role-only entry would be a
		// content-free fingerprint shape that blinds breaker diagnostics.
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
	rr := cr.o.performEffectsAndReview(cr.ctx, in)
	if rr.Demoted {
		cr.noteContractGateDemotion(phase, d, rr.Blocks, rr.Reason)
	}
	return rr
}

package core

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const contractEscalateAtBlock = 2

const universalContractFallbackCLI = "claude-tmux"

const contractSalvageRetryDirectiveHeading = "## Contract Salvage Retry — verbatim validator output"

func composeContractSalvageRetry(round int, reason, remediation string) string {
	return composeCorrection(round, reason, remediation) + "\n\n" + contractSalvageRetryDirectiveHeading + "\n\n" +
		"This is the second consecutive block reporting the SAME defect, and no other CLI family is " +
		"available to escalate to — this is the last correction before the contract gate's circuit " +
		"breaker opens and the gate stops enforcing for the rest of this run.\n\n" +
		"The contract validator's output, verbatim:\n\n" + reason + "\n\n" +
		"Do not re-summarize it. Take each bracketed [violation_code] above in turn, state the exact " +
		"section heading or file path that code refers to, then re-emit the whole deliverable at the " +
		"contracted path with that specific defect closed. " + salvageClosing(remediation)
}

func salvageClosing(remediation string) string {
	if remediation != "" {
		return "Change nothing else beyond what the remedy above requires."
	}
	return "Do not change unrelated files."
}

const ledgerKindContractGateDemoted = "contract_gate_demoted"

type contractDispatch struct {
	cli            string
	escalated      bool
	salvageRetried bool
}

func (cr *cycleRun) contractEscalationProfile(phase Phase) (*profiles.Profile, string) {
	return phaseProfile(cr.req.ProjectRoot, string(phase))
}

func phaseProfile(projectRoot, phase string) (*profiles.Profile, string) {
	loader := profiles.NewFromDir(filepath.Join(projectRoot, ".evolve", "profiles"))
	for _, agent := range []string{phaseAgentName[phase], phase} {
		if agent == "" {
			continue
		}
		if prof, err := loader.Get(agent); err == nil {
			return &prof, agent
		}
	}
	return nil, phase
}

func (cr *cycleRun) contractDispatchCLI(phase Phase, override string) string {
	if override != "" {
		return override
	}
	_, walk, routed := cr.routingWalk(phase)
	if !routed {
		return universalContractFallbackCLI
	}
	return walk.Candidates[0]
}

func (cr *cycleRun) contractEscalationCLI(phase Phase, dispatchedCLI string) (string, bool) {
	d, walk, routed := cr.routingWalk(phase)
	if !routed {
		return "", false
	}
	failed := llmroute.Family(cmp.Or(dispatchedCLI, walk.Candidates[0]))
	candidates := slices.Clone(walk.Candidates)
	if d.Legacy() {
		candidates = append(candidates, universalContractFallbackCLI)
	}
	for _, c := range candidates {
		if c != "" && llmroute.Family(c) != failed && escalationAllowed(phase, c, d) {
			return c, true
		}
	}
	return "", false
}

func (cr *cycleRun) routingWalk(phase Phase) (cliroute.Decision, llmroute.Plan, bool) {
	d, routed := cr.routingDecision(phase)
	if !routed {
		return d, llmroute.Plan{}, false
	}
	walk, err := d.Walk()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract escalation has no route: %v\n", phase, err)
		return d, llmroute.Plan{}, false
	}
	return d, walk, true
}

func (cr *cycleRun) routingDecision(phase Phase) (cliroute.Decision, bool) {
	prof, agent := cr.contractEscalationProfile(phase)
	router, err := cr.cliRouter(agent, prof)
	d := cliroute.Decision{}
	if err == nil {
		d, err = router.Resolve(cliroute.Request{Agent: agent, Phase: string(phase), ProjectRoot: cr.req.ProjectRoot, Env: cr.envSnap, Cycle: cr.cycle})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract escalation has no route: %v\n", phase, err)
		return cliroute.Decision{}, false
	}
	return d, true
}

func (cr *cycleRun) cliRouter(agent string, prof *profiles.Profile) (*cliroute.Router, error) {
	if cr.o != nil && cr.o.cliRouter != nil {
		return cr.o.cliRouter, nil
	}
	return cliroute.NewLegacyLaunchRouter(cr.req.ProjectRoot, cliroute.SingleProfile{Agent: agent, Profile: prof})
}

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

type contractBlockIdentity struct {
	observed   bool
	normalized string
	codes      map[string]struct{}
}

var contractViolationCodeRE = regexp.MustCompile(`\[([A-Za-z0-9_.:-]+)\]`)

func contractArtifactDetermined(code string) bool {
	switch code {
	case "missing_artifact",
		"empty_artifact",
		"missing_section",
		"missing_challenge_token",
		"bad_verdict",
		"failure_class_unknown",
		"missing_key":
		return true
	}
	return false
}

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

func escalationAllowed(phase Phase, cli string, d cliroute.Decision) bool {
	if !d.Allows(cli) {
		fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract-escalation candidate cli=%s refused: outside the allowed set %v\n", phase, cli, d.Allowed)
		return false
	}
	return true
}

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

func (cr *cycleRun) noteContractGateDemotion(phase Phase, d contractDispatch, blocks int, reason string) {
	cli := cr.contractDispatchCLI(phase, d.cli)
	warn := formatContractGateDemotionWarn(string(phase), cli, d, reason)
	fmt.Fprintln(os.Stderr, warn)
	if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:       cr.o.now().UTC().Format(time.RFC3339),
		Cycle:    cr.cycle,
		Role:     string(phase),
		Kind:     ledgerKindContractGateDemoted,
		Action:   fmt.Sprintf("demote enforce->advisory: cli=%s escalated=%v salvage_attempted=%v blocks=%d: %s", cli, d.escalated, d.salvageRetried, blocks, reason),
		ExitCode: 0,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s ledger append: %v\n", ledgerKindContractGateDemoted, lerr)
	}
	pol, perr := policy.Load(filepath.Join(cr.req.ProjectRoot, ".evolve", "policy.json"))
	if perr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s: policy load for autofile weight: %v (using compiled default)\n", ledgerKindContractGateDemoted, perr)
	}
	if _, serr := dispositionrouter.StageIntent(
		filepath.Join(cr.req.ProjectRoot, ".evolve", "escalations"),
		ContractGateDemotion{Phase: phase, CLI: cli, Cycle: cr.cycle, Blocks: blocks, Weight: pol.RetroAutofileDefaultWeight(), Reason: warn}.Intent()); serr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s: stage escalation intent: %v\n", ledgerKindContractGateDemoted, serr)
	}
}

type ContractGateDemotion struct {
	Phase  Phase
	CLI    string
	Cycle  int
	Blocks int
	Weight float64
	Reason string
}

func (d ContractGateDemotion) Intent() dispositionrouter.Intent {
	return dispositionrouter.Intent{
		Action:        dispositionrouter.ActionAutofile,
		Route:         dispositionrouter.RouteConsole,
		ItemID:        "contract-gate-demoted-" + string(d.Phase),
		Pattern:       fmt.Sprintf("contract gate demoted enforce->advisory on phase %s (cli=%s)", d.Phase, d.CLI),
		Cycle:         d.Cycle,
		Recurrence:    d.Blocks,
		Weight:        d.Weight,
		Reason:        d.Reason,
		PriorityClass: inboxbatch.ClassCorrectness,
	}
}

func (cr *cycleRun) reviewDeliverable(phase Phase, in ReviewInput, d contractDispatch) ReviewResult {
	rr := cr.o.performEffectsAndReview(cr.ctx, in)
	if rr.Demoted {
		cr.noteContractGateDemotion(phase, d, rr.Blocks, rr.Reason)
	}
	return rr
}

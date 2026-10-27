package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const orchestratorProfile = phasecontract.FlowSeatSelfhealOrchestrator

var proposeChannels = []string{"errors", "loop", "inbox"}

type proposeReport struct {
	Stage          string              `json:"stage"`
	ValidatorStage string              `json:"validator_stage"`
	Proposals      []selfheal.Proposal `json:"proposals"`
}

type proposalDecider struct {
	on        bool
	warnings  []string
	validator selfheal.Validator
	decisions []selfheal.Decision
}

func runAgentCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return agentcmd.New(agentcmd.Deps{InstallRouter: rootRouterInstaller, Bridge: agentRunBridge, Signals: agentRunSignals, VerbSchema: coreVerbSchema})(args, stdin, stdout, stderr)
}

func runSelfhealPropose(env selfhealEnv, w codeEnvelopeWriter, root string, pol policy.Policy) int {
	report, warnings, err := proposeDiagnoses(env, root, pol, w.stderr)
	if err != nil {
		return w.fail(exitIO, "io", "evolve selfheal: "+err.Error())
	}
	for _, p := range report.Proposals {
		if p.State != selfheal.ProposalProposed {
			warnings = append(warnings, p.IncidentID+" "+p.State+": "+p.Error)
		}
	}
	for _, msg := range warnings {
		w.warnings = append(w.warnings, msg)
		fmt.Fprintln(w.stderr, "evolve selfheal: WARN: "+msg)
	}
	printProposals(w.stdout, w.json, report)
	return w.ok(report)
}

type proposePass struct {
	root      string
	pol       policy.Policy
	incidents []selfheal.Incident
	since     time.Time
}

func proposeDiagnoses(env selfhealEnv, root string, pol policy.Policy, stderr io.Writer) (proposeReport, []string, error) {
	cfg := pol.HealingOrchestrator()
	report := proposeReport{Stage: cfg.Stage, ValidatorStage: pol.HealingValidator().Stage, Proposals: []selfheal.Proposal{}}
	if cfg.Stage == policy.HealingStageOff {
		return report, nil, nil
	}
	evolveDir := paths.EvolveDirOf(root)
	incidents, err := selfheal.LoadIncidents(selfheal.Dir(evolveDir))
	if err != nil {
		return report, nil, err
	}
	done, err := selfheal.LoadProposals(selfheal.ProposalsDir(evolveDir))
	if err != nil {
		return report, nil, err
	}
	now := env.now()
	pending := selfheal.Pending(incidents, done, selfheal.PassLimits{Max: cfg.MaxPerPass, RetryAt: now.Add(-hours(cfg.RetryAfterHours))})
	if len(pending) == 0 {
		return report, nil, nil
	}
	pass := proposePass{root: root, pol: pol, incidents: incidents, since: now.Add(-hours(pol.HealingDetector().WindowHours))}
	signals, err := selfheal.LoadSignals(paths.EventChannelsRoot(evolveDir), proposeChannels, pass.since)
	if err != nil {
		return report, nil, err
	}
	decider, err := newProposalDecider(env, pass)
	if err != nil {
		return report, nil, err
	}
	var proposals []selfheal.Proposal
	for _, inc := range pending {
		proposals = append(proposals, decider.decide(diagnoseIncident(env, root, inc, signals, cfg.Stage, now, stderr), inc, now))
	}
	if err := decider.appendLedger(evolveDir); err != nil {
		return report, nil, err
	}
	for _, p := range proposals {
		if err := selfheal.WriteProposal(selfheal.ProposalsDir(evolveDir), p); err != nil {
			return report, nil, err
		}
		report.Proposals = append(report.Proposals, p)
	}
	decider.publish(env, root, stderr)
	return report, decider.warnings, nil
}

func hours(n int) time.Duration { return time.Duration(n) * time.Hour }

func newProposalDecider(env selfhealEnv, pass proposePass) (*proposalDecider, error) {
	cfg := pass.pol.HealingValidator()
	if cfg.Stage == policy.HealingStageOff {
		return &proposalDecider{}, nil
	}
	evolveDir := paths.EvolveDirOf(pass.root)
	history, err := selfheal.LoadHistory(evolveDir)
	if err != nil {
		return nil, err
	}
	inv, warnings, err := decisionInventory(env, pass)
	if err != nil {
		return nil, err
	}
	limits := selfheal.Limits{Window: hours(cfg.WindowHours), Repairs: map[string]int{selfheal.TierPipeline: cfg.T1Repairs, selfheal.TierItem: cfg.T2Repairs},
		BreakerFailures: cfg.BreakerFailures, BreakerOpen: hours(cfg.BreakerOpenHours)}
	return &proposalDecider{on: true, warnings: warnings, validator: selfheal.Validator{Inventory: inv, History: history, Limits: limits, Stage: cfg.Stage}}, nil
}

func decisionInventory(env selfhealEnv, pass proposePass) (selfheal.Inventory, []string, error) {
	evolveDir := paths.EvolveDirOf(pass.root)
	items, err := inboxfile.PendingIDs(filepath.Join(evolveDir, "inbox"))
	if errors.Is(err, fs.ErrNotExist) {
		items, err = map[string]bool{}, nil
	}
	if err != nil {
		return selfheal.Inventory{}, nil, fmt.Errorf("inbox: %w", err)
	}
	var warnings []string
	landings, err := selfheal.LoadLandings(shipjournal.Path(evolveDir), pass.since)
	if err != nil {
		warnings = append(warnings, "no landing of the loop is known, so each revert-last-landing is refused: "+err.Error())
	}
	mainRed := slices.ContainsFunc(pass.incidents, func(inc selfheal.Incident) bool {
		return inc.State == selfheal.StateOpen && inc.Class == selfheal.ClassMainRed
	})
	fileExists := func(path string) bool {
		_, err := os.Stat(filepath.Join(pass.root, path))
		return err == nil
	}
	return selfheal.Inventory{Items: items, Families: env.families(), Landings: landings, MainRed: mainRed, FileExists: fileExists}, warnings, nil
}

func (d *proposalDecider) decide(p selfheal.Proposal, inc selfheal.Incident, now time.Time) selfheal.Proposal {
	var decision selfheal.Decision
	switch {
	case !d.on || p.State == selfheal.ProposalFailed:
		return p
	case p.State == selfheal.ProposalRefused:
		decision = d.validator.Refuse(inc, errors.New(p.Error), now)
	default:
		decision = d.validator.Decide(inc, *p.Action, now)
	}
	d.validator = d.validator.Record(decision)
	d.decisions = append(d.decisions, decision)
	return p.WithDecision(decision)
}

func (d *proposalDecider) appendLedger(evolveDir string) error {
	if len(d.decisions) == 0 {
		return nil
	}
	return selfheal.AppendDecisions(selfheal.DecisionsPath(evolveDir), d.decisions)
}

func (d *proposalDecider) publish(env selfhealEnv, root string, stderr io.Writer) {
	if len(d.decisions) == 0 {
		return
	}
	center, closeCenter := env.newCenter(root, paths.EvolveDirOf(root), stderr)
	defer closeCenter()
	for _, decision := range d.decisions {
		for _, e := range decision.Events() {
			center.Emit(e)
		}
	}
}

func diagnoseIncident(env selfhealEnv, root string, inc selfheal.Incident, signals []signalcenter.Event, stage string, now time.Time, stderr io.Writer) selfheal.Proposal {
	idx, err := selfheal.WriteEvidence(root, inc, signals)
	if err != nil {
		return selfheal.NewProposal(inc, stage, "", now).Failed(err)
	}
	run, err := runOrchestratorAgent(env, root, inc, stderr)
	p := selfheal.NewProposal(inc, stage, run.RunLabel, now)
	if err != nil {
		return p.Failed(err)
	}
	d, err := phasecontract.ParseDiagnosis([]byte(run.Report), inc.ID, selfheal.EntryIDs(idx))
	if err != nil {
		return p.Refused(err)
	}
	return p.WithDiagnosis(d)
}

func runOrchestratorAgent(env selfhealEnv, root string, inc selfheal.Incident, stderr io.Writer) (agentcmd.Envelope, error) {
	return runAgentSeat(env.agent, []string{orchestratorProfile, "--json", "--project-root", root}, orchestratorInput(root, inc), stderr)
}

func orchestratorInput(root string, inc selfheal.Incident) string {
	ws := selfheal.EvidenceWorkspace(paths.EvolveDirOf(root), inc.ID)
	return fmt.Sprintf("Diagnose the incident %s (class %s, tier %s, severity %s): %s\n\n"+
		"Its evidence workspace is %s. List the evidence index with `evolve context index --workspace %s --json`. "+
		"Open the required entry first with `evolve context open i1 --workspace %s --json`.\n\n"+
		"Write diagnosis.json with the schema %s and the incident_id %s, as your instructions say.\n",
		inc.ID, inc.Class, inc.Tier, inc.Severity, inc.Summary, ws, ws, ws, phasecontract.DiagnosisSchema, inc.ID)
}

func printProposals(stdout io.Writer, asJSON bool, r proposeReport) {
	if asJSON {
		return
	}
	fmt.Fprintf(stdout, "selfheal propose: stage %s, %d proposal(s)\n", r.Stage, len(r.Proposals))
	for _, p := range r.Proposals {
		action := p.Error
		if p.Action != nil {
			action = p.Action.Kind
		}
		if p.Decision != nil {
			action += " (" + strings.TrimSuffix(p.Decision.Verdict+" "+p.Decision.Precondition, " ") + ")"
		}
		fmt.Fprintf(stdout, "%s %s %s: %s\n", p.IncidentID, p.Tier, p.State, action)
	}
}

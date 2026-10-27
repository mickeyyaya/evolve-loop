package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const orchestratorProfile = phasecontract.FlowSeatSelfhealOrchestrator

var proposeChannels = []string{"errors", "loop", "inbox"}

type proposeReport struct {
	Stage     string              `json:"stage"`
	Proposals []selfheal.Proposal `json:"proposals"`
}

func runAgentCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return agentcmd.New(agentcmd.Deps{InstallRouter: rootRouterInstaller, Bridge: agentRunBridge, Signals: agentRunSignals})(args, stdin, stdout, stderr)
}

func runSelfhealPropose(env selfhealEnv, w codeEnvelopeWriter, root string, pol policy.Policy) int {
	report, err := proposeDiagnoses(env, root, pol, w.stderr)
	if err != nil {
		return w.fail(exitIO, "io", "evolve selfheal: "+err.Error())
	}
	for _, p := range report.Proposals {
		if p.State != selfheal.ProposalProposed {
			msg := p.IncidentID + " " + p.State + ": " + p.Error
			w.warnings = append(w.warnings, msg)
			fmt.Fprintln(w.stderr, "evolve selfheal: WARN: "+msg)
		}
	}
	printProposals(w.stdout, w.json, report)
	return w.ok(report)
}

func proposeDiagnoses(env selfhealEnv, root string, pol policy.Policy, stderr io.Writer) (proposeReport, error) {
	cfg := pol.HealingOrchestrator()
	report := proposeReport{Stage: cfg.Stage, Proposals: []selfheal.Proposal{}}
	if cfg.Stage == policy.HealingStageOff {
		return report, nil
	}
	evolveDir := paths.EvolveDirOf(root)
	incidents, err := selfheal.LoadIncidents(selfheal.Dir(evolveDir))
	if err != nil {
		return report, err
	}
	done, err := selfheal.LoadProposals(selfheal.ProposalsDir(evolveDir))
	if err != nil {
		return report, err
	}
	pending := selfheal.Pending(incidents, done, cfg.MaxPerPass)
	if len(pending) == 0 {
		return report, nil
	}
	now := env.now()
	since := now.Add(-time.Duration(pol.HealingDetector().WindowHours) * time.Hour)
	signals, err := selfheal.LoadSignals(paths.EventChannelsRoot(evolveDir), proposeChannels, since)
	if err != nil {
		return report, err
	}
	for _, inc := range pending {
		p := diagnoseIncident(env, root, inc, signals, cfg.Stage, now, stderr)
		if err := selfheal.WriteProposal(selfheal.ProposalsDir(evolveDir), p); err != nil {
			return report, err
		}
		report.Proposals = append(report.Proposals, p)
	}
	return report, nil
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
	var stdout bytes.Buffer
	rc := env.agent([]string{"run", orchestratorProfile, "--json", "--project-root", root}, strings.NewReader(orchestratorInput(root, inc)), &stdout, stderr)
	var run agentcmd.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &run); err != nil {
		return run, fmt.Errorf("evolve agent run %s: exit %d with no JSON envelope: %w", orchestratorProfile, rc, err)
	}
	if rc != 0 {
		return run, fmt.Errorf("evolve agent run %s: exit %d: %s", orchestratorProfile, rc, run.Error)
	}
	return run, nil
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
		fmt.Fprintf(stdout, "%s %s %s: %s\n", p.IncidentID, p.Tier, p.State, action)
	}
}

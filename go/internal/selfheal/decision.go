package selfheal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeProposalRefused signalcenter.Code = "HEALING_PROPOSAL_REFUSED"
	CodeEscalated       signalcenter.Code = "HEALING_ESCALATED"

	decisionsFile = "decisions.jsonl"
	repairsFile   = "repairs.jsonl"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeProposalRefused, "the catalog validator refused a proposal of the orchestrator agent; fields incident, class, tier, action, verdict, precondition, reason, stage (llm-isolation.md §19, §19.2, §24)")
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeEscalated, "the catalog validator sent an incident to the operator: a second refusal, two failed repairs, an open breaker or a halt-and-escalate action; the same fields as HEALING_PROPOSAL_REFUSED (llm-isolation.md §19.1, §19.2, §24)")
}

func (d Decision) Events() []signalcenter.Event {
	var out []signalcenter.Event
	switch {
	case d.Verdict == VerdictRefused:
		out = append(out, d.event(signalcenter.SeverityWarn, CodeProposalRefused))
	case !d.Escalated:
		out = append(out, d.event(signalcenter.SeverityInfo, ""))
	}
	if d.Escalated {
		out = append(out, d.event(signalcenter.SeverityIncident, CodeEscalated))
	}
	return out
}

func (d Decision) event(severity signalcenter.Severity, code signalcenter.Code) signalcenter.Event {
	action := ""
	if d.Action != nil {
		action = d.Action.Kind
	}
	return signalcenter.Event{
		Module: signalcenter.ModuleHealing, Kind: signalcenter.KindSelfhealDecision, Origin: "selfheal.Validator",
		Severity: severity, Code: code,
		Reason: fmt.Sprintf("proposal for incident %s %s: %s %s", d.IncidentID, d.Verdict, action, d.Reason),
		Fields: map[string]string{
			"incident": d.IncidentID, "class": string(d.Class), "tier": d.Tier, "action": action, "verdict": d.Verdict,
			"precondition": d.Precondition, "reason": d.Reason, "stage": d.Stage,
		},
	}
}

func (p Proposal) WithDecision(d Decision) Proposal {
	p.Decision = &d
	return p
}

func (p Proposal) Dispatchable() (phasecontract.DiagnosisAction, error) {
	if p.Decision == nil || p.Decision.Verdict != VerdictAccepted || p.Decision.Action == nil {
		return phasecontract.DiagnosisAction{}, fmt.Errorf("%w for the incident %s", ErrNoAcceptedProposal, p.IncidentID)
	}
	return *p.Decision.Action, nil
}

func DecisionsPath(evolveDir string) string { return filepath.Join(Dir(evolveDir), decisionsFile) }

func RepairsPath(evolveDir string) string { return filepath.Join(Dir(evolveDir), repairsFile) }

func AppendDecisions(path string, ds []Decision) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, d := range ds {
		if err := enc.Encode(d); err != nil {
			return fmt.Errorf("decisions: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("decisions: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, err = f.Write(buf.Bytes())
		err = errors.Join(err, f.Close())
	}
	if err != nil {
		return fmt.Errorf("decisions: %w", err)
	}
	return nil
}

func LoadHistory(evolveDir string) (History, error) {
	decisions, err := readJSONL[Decision](DecisionsPath(evolveDir))
	if err != nil {
		return History{}, err
	}
	repairs, err := readJSONL[Repair](RepairsPath(evolveDir))
	if err != nil {
		return History{}, err
	}
	return History{Decisions: decisions, Repairs: repairs}, nil
}

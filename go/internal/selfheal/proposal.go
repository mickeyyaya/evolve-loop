package selfheal

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	ProposalSchema   = "evolve.healing.proposal/1"
	ProposalProposed = "proposed"
	ProposalRefused  = "refused"
	ProposalFailed   = "failed"
	proposalsDirName = "proposals"
)

type Proposal struct {
	Schema             string                         `json:"schema"`
	IncidentID         string                         `json:"incident_id"`
	Fingerprint        string                         `json:"fingerprint"`
	Class              Class                          `json:"class"`
	Tier               string                         `json:"tier"`
	Stage              string                         `json:"stage"`
	State              string                         `json:"state"`
	RunLabel           string                         `json:"run_label,omitempty"`
	Diagnosis          *phasecontract.Diagnosis       `json:"diagnosis,omitempty"`
	Action             *phasecontract.DiagnosisAction `json:"action,omitempty"`
	Decision           *Decision                      `json:"decision,omitempty"`
	Error              string                         `json:"error,omitempty"`
	IncidentRecurrence int                            `json:"incident_recurrence"`
	IncidentLastSeenAt time.Time                      `json:"incident_last_seen_at"`
	CreatedAt          time.Time                      `json:"created_at"`
}

func ProposalsDir(evolveDir string) string { return filepath.Join(Dir(evolveDir), proposalsDirName) }

func NewProposal(inc Incident, stage, runLabel string, at time.Time) Proposal {
	return Proposal{Schema: ProposalSchema, IncidentID: inc.ID, Fingerprint: inc.Fingerprint, Class: inc.Class, Tier: inc.Tier,
		Stage: stage, RunLabel: runLabel, IncidentRecurrence: inc.Recurrence, IncidentLastSeenAt: inc.LastSeenAt, CreatedAt: at}
}

func (p Proposal) WithDiagnosis(d phasecontract.Diagnosis) Proposal {
	action := proposedAction(d)
	p.State, p.Diagnosis, p.Action = ProposalProposed, &d, &action
	return p
}

func proposedAction(d phasecontract.Diagnosis) phasecontract.DiagnosisAction {
	if d.Confidence != phasecontract.ConfidenceLow {
		return d.Action
	}
	return phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate, Params: phasecontract.DiagnosisParams{Reason: "a low-confidence diagnosis goes to the operator: " + d.Hypothesis}}
}

func (p Proposal) Refused(err error) Proposal {
	p.State, p.Error = ProposalRefused, err.Error()
	return p
}

func (p Proposal) Failed(err error) Proposal {
	p.State, p.Error = ProposalFailed, err.Error()
	return p
}

func (p Proposal) current(inc Incident, retryAt time.Time) bool {
	if p.State == ProposalFailed {
		return p.CreatedAt.After(retryAt)
	}
	return p.IncidentRecurrence == inc.Recurrence && p.IncidentLastSeenAt.Equal(inc.LastSeenAt)
}

type PassLimits struct {
	Max     int
	RetryAt time.Time
}

func Pending(incidents []Incident, done map[string]Proposal, limits PassLimits) []Incident {
	var out []Incident
	for _, inc := range incidents {
		if p, ok := done[inc.ID]; inc.State == StateOpen && (!ok || !p.current(inc, limits.RetryAt)) {
			out = append(out, inc)
		}
	}
	slices.SortStableFunc(out, func(a, b Incident) int {
		return cmp.Or(cmp.Compare(b.Severity.Level(), a.Severity.Level()), strings.Compare(a.ID, b.ID))
	})
	return out[:min(limits.Max, len(out))]
}

func WriteProposal(dir string, p Proposal) error {
	if err := atomicwrite.JSON(filepath.Join(dir, p.IncidentID+".json"), p); err != nil {
		return fmt.Errorf("proposal %s: %w", p.IncidentID, err)
	}
	return nil
}

func LoadProposals(dir string) (map[string]Proposal, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Proposal{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("proposals: %w", err)
	}
	out := map[string]Proposal{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("proposal: %w", err)
		}
		var p Proposal
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("proposal %s: %w", e.Name(), err)
		}
		out[p.IncidentID] = p
	}
	return out, nil
}

package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func auditArtifactName() string {
	if c, ok := phasecontract.For("audit"); ok && c.ArtifactName != "" {
		return c.ArtifactName
	}
	return "the audit report"
}

func timingRecords(workspace string, live []phasetiming.Entry) ([]PhaseRecord, *phasetiming.Summary, bool) {
	entries := live
	if len(entries) == 0 {
		read, err := phasetiming.Read(workspace)
		if err != nil || len(read) == 0 {
			return nil, nil, false
		}
		entries = read
	}
	records := make([]PhaseRecord, 0, len(entries))
	for _, e := range entries {
		records = append(records, PhaseRecord{
			Name:          e.Phase,
			Verdict:       normalizeVerdict(e.Verdict),
			DurationMS:    e.DurationMS,
			StartedAt:     e.StartedAt,
			EndedAt:       e.EndedAt,
			Archetype:     e.Archetype,
			ModelSource:   e.ModelSource,
			ResolvedModel: e.ResolvedModel,
			Tokens:        e.Tokens,
		})
	}
	summary := phasetiming.Rollup(entries)
	return records, &summary, true
}

func normalizeVerdict(v string) string {
	switch v {
	case VerdictPass, VerdictWarn, VerdictFail:
		return v
	default:
		return VerdictWarn
	}
}

type BuildOpts struct {
	WorkspacePath      string
	Goal               string
	RunID              string
	FinalVerdict       string
	SystemFailure      *cyclestate.SystemFailureSignal
	SkippedPhases      []cyclestate.SkippedPhase
	VerdictsNotAdopted []cyclestate.VerdictNotAdopted
	SpineFailOpens     []cyclestate.SpineFailOpen
	PhaseTimings       []phasetiming.Entry
}

func Build(cycle int, opts BuildOpts) (*Dossier, error) {
	if err := validateBuildInputs(cycle, opts); err != nil {
		return nil, err
	}
	verdict, err := resolveBuildVerdict(opts.FinalVerdict)
	if err != nil {
		return nil, err
	}
	d := &Dossier{
		SchemaVersion:              CurrentSchemaVersion,
		Cycle:                      cycle,
		Goal:                       opts.Goal,
		RunID:                      opts.RunID,
		FinalVerdict:               verdict,
		SystemFailure:              opts.SystemFailure,
		Phases:                     []PhaseRecord{evidenceUnavailablePhase()},
		SkippedPhases:              opts.SkippedPhases,
		PhasesRunVerdictNotAdopted: opts.VerdictsNotAdopted,
		SpineFailOpens:             opts.SpineFailOpens,
	}
	if records, summary, ok := timingRecords(opts.WorkspacePath, opts.PhaseTimings); ok {
		d.Phases = records
		d.Timing = summary
	}
	if commit, tree, ok := shippedCommit(opts.WorkspacePath); ok {
		d.CommitSHA, d.TreeSHA = commit, tree
	}
	if tasks, ok := committedTasks(opts.WorkspacePath); ok {
		d.Tasks = &tasks
	}
	if rec, ok := ciWatchRecord(opts.WorkspacePath); ok {
		d.CIWatch = rec
	}
	d.RecalledLessons = readRecalledLessons(opts.WorkspacePath)
	if verdict == VerdictFail {
		recordAuditFailure(d, cycle, opts.WorkspacePath)
	}
	return d, nil
}

func validateBuildInputs(cycle int, opts BuildOpts) error {
	if cycle <= 0 {
		return fmt.Errorf("dossier: Build: cycle must be >= 1, got %d", cycle)
	}
	if strings.TrimSpace(opts.WorkspacePath) == "" {
		return fmt.Errorf("dossier: Build: WorkspacePath must not be blank")
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("dossier: Build: Goal must not be blank")
	}
	return nil
}

func recordAuditFailure(d *Dossier, cycle int, workspace string) {
	d.Defects = []Defect{{
		ID:       "audit-fail",
		Severity: "HIGH",
		Summary:  fmt.Sprintf("cycle did not pass audit; see %s + acs-verdict.json", auditArtifactName()),
		Fix:      "address the audit findings recorded for this cycle",
	}}
	d.Carryover = []Carryover{{
		ID:       "address-audit-findings",
		Action:   fmt.Sprintf("resolve the audit findings that failed cycle %d", cycle),
		Priority: "high",
	}}
	if rec, ok := failureRecord(workspace, cycle); ok {
		d.Failure = rec
	}
}

func resolveBuildVerdict(v string) (string, error) {
	switch v {
	case "":
		return VerdictPass, nil
	case VerdictPass, VerdictWarn, VerdictFail:
		return v, nil
	default:
		return "", fmt.Errorf("dossier: Build: FinalVerdict %q must be empty|PASS|WARN|FAIL", v)
	}
}

func evidenceUnavailablePhase() PhaseRecord {
	return PhaseRecord{
		Name:    "evidence-unavailable",
		Verdict: VerdictWarn,
		KeyFindings: "per-phase evidence unavailable at dossier build (no live timings; " +
			phasetiming.FileName + " absent or empty) — this record is DEGRADED, not a one-phase cycle",
	}
}

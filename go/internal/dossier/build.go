package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// auditArtifactName derives the audit report filename from the phasecontract
// registry; a re-typed literal would go stale when the registry renames it.
func auditArtifactName() string {
	if c, ok := phasecontract.For("audit"); ok && c.ArtifactName != "" {
		return c.ArtifactName
	}
	return "the audit report"
}

// timingRecords projects live timings, else the workspace phase-timing.json,
// into per-phase records and the roll-up. ok is false when neither has entries.
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

// normalizeVerdict maps an unknown or blank timing verdict to WARN, so Validate
// cannot reject a record over a legacy or hand-edited log entry.
func normalizeVerdict(v string) string {
	switch v {
	case VerdictPass, VerdictWarn, VerdictFail:
		return v
	default:
		return VerdictWarn
	}
}

// BuildOpts configures a Build call.
type BuildOpts struct {
	WorkspacePath string
	// LedgerPath is reserved for a future ledger walk; no caller sets it and
	// Build does not read it.
	LedgerPath string
	Goal       string
	RunID      string
	// FinalVerdict is the cycle's real outcome. Empty means PASS; FAIL makes
	// Build synthesize a defect and carryover that point at the audit artifacts.
	FinalVerdict       string
	SystemFailure      *cyclestate.SystemFailureSignal
	SkippedPhases      []cyclestate.SkippedPhase
	VerdictsNotAdopted []cyclestate.VerdictNotAdopted
	SpineFailOpens     []cyclestate.SpineFailOpen
	// PhaseTimings is the caller's composed timing set. It is passed in because
	// the on-disk log lands after the dossier on the normal path; empty falls
	// back to reading the workspace log.
	PhaseTimings []phasetiming.Entry
}

// Build assembles the dossier for cycle from opts and the workspace artifacts.
// With no per-phase evidence it records one evidence-unavailable WARN phase
// rather than invent a phase verdict.
func Build(cycle int, opts BuildOpts) (*Dossier, error) {
	if cycle <= 0 {
		return nil, fmt.Errorf("dossier: Build: cycle must be >= 1, got %d", cycle)
	}
	if strings.TrimSpace(opts.WorkspacePath) == "" {
		return nil, fmt.Errorf("dossier: Build: WorkspacePath must not be blank")
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return nil, fmt.Errorf("dossier: Build: Goal must not be blank")
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
	// Validate requires a FAIL to carry a defect and a carryover. These point at
	// the audit artifacts rather than invent specifics.
	if verdict == VerdictFail {
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
		if rec, ok := failureRecord(opts.WorkspacePath, cycle); ok {
			d.Failure = rec
		}
	}
	return d, nil
}

// resolveBuildVerdict defaults an empty verdict to PASS and rejects anything
// outside PASS|WARN|FAIL rather than silently defaulting it.
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

// evidenceUnavailablePhase marks a record with no per-phase evidence. WARN
// stays inside the validated vocabulary; the cycle's FinalVerdict is untouched.
func evidenceUnavailablePhase() PhaseRecord {
	return PhaseRecord{
		Name:    "evidence-unavailable",
		Verdict: VerdictWarn,
		KeyFindings: "per-phase evidence unavailable at dossier build (no live timings; " +
			phasetiming.FileName + " absent or empty) — this record is DEGRADED, not a one-phase cycle",
	}
}

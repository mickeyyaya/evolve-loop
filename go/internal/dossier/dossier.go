// Package dossier builds, validates, writes and reads the committed per-cycle
// record, knowledge-base/cycles/cycle-N.{json,md}.
// See docs/architecture/packages/internal-dossier.md.
package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// Verdict values a cycle (or phase) can carry.
const (
	VerdictPass = "PASS"
	VerdictWarn = "WARN"
	VerdictFail = "FAIL"

	// CurrentSchemaVersion is stamped on every new dossier; the unversioned
	// legacy corpus reads as zero.
	CurrentSchemaVersion = 2
)

// Dossier is the aggregated, committed record of one cycle.
type Dossier struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	Cycle         int    `json:"cycle"`
	RunID         string `json:"run_id,omitempty"`
	Goal          string `json:"goal"`
	FinalVerdict  string `json:"final_verdict"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	TreeSHA       string `json:"tree_sha,omitempty"`
	// Tasks is the triage commitment. It is a pointer so that nil (no decision
	// read, field omitted) stays distinct from an explicit empty commitment ([]).
	Tasks   *[]string     `json:"tasks,omitempty"`
	Phases  []PhaseRecord `json:"phases"`
	Defects []Defect      `json:"defects,omitempty"`
	// Failure is nil on non-FAIL cycles and whenever the failure artifacts
	// yielded no content.
	Failure       *FailureRecord                  `json:"failure,omitempty"`
	SystemFailure *cyclestate.SystemFailureSignal `json:"system_failure,omitempty"`
	Decisions     []string                        `json:"decisions,omitempty"`
	Lessons       []Lesson                        `json:"lessons,omitempty"`
	Carryover     []Carryover                     `json:"carryover,omitempty"`
	// SkippedPhases lists only phases that did not run. A phase that ran but
	// whose verdict was declined belongs in PhasesRunVerdictNotAdopted.
	SkippedPhases []cyclestate.SkippedPhase `json:"skipped_phases,omitempty"`
	// PhasesRunVerdictNotAdopted lists non-floor phases that ran and returned
	// non-PASS after the floor verdict was set, so their verdict was declined.
	PhasesRunVerdictNotAdopted []cyclestate.VerdictNotAdopted `json:"phases_run_verdict_not_adopted,omitempty"`
	SpineFailOpens             []cyclestate.SpineFailOpen     `json:"spine_fail_opens,omitempty"`
	StartedAt                  string                         `json:"started_at,omitempty"`
	EndedAt                    string                         `json:"ended_at,omitempty"`
	Timing                     *phasetiming.Summary           `json:"timing,omitempty"`
	CIWatch                    *CIWatchRecord                 `json:"ci_watch,omitempty"`
}

// PhaseRecord is one phase's outcome within the cycle.
type PhaseRecord struct {
	Name        string         `json:"name"`
	Verdict     string         `json:"verdict"`
	KeyFindings string         `json:"key_findings,omitempty"`
	ArtifactSHA string         `json:"artifact_sha,omitempty"`
	Signals     map[string]any `json:"signals,omitempty"`
	DurationMS  int64          `json:"duration_ms,omitempty"`
	StartedAt   string         `json:"started_at,omitempty"`
	EndedAt     string         `json:"ended_at,omitempty"`
	Archetype   string         `json:"archetype,omitempty"`
	// ModelSource is "profile", "pin" or "advisor"; it and ResolvedModel are
	// empty when the timing log predates model provenance.
	ModelSource   string                `json:"model_source,omitempty"`
	ResolvedModel string                `json:"resolved_model,omitempty"`
	Tokens        cyclestate.TokenUsage `json:"tokens,omitempty"`
}

// Defect is one audit finding, recorded so a failed cycle says why it failed.
type Defect struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Fix      string `json:"fix,omitempty"`
}

// Lesson is a narrative learning folded in from the retrospective/memo.
type Lesson struct {
	ID               string `json:"id"`
	Pattern          string `json:"pattern"`
	PreventiveAction string `json:"preventive_action,omitempty"`
}

// Carryover is a work item for the next cycle (the compounding work-list).
type Carryover struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	Priority string `json:"priority,omitempty"`
}

func validVerdict(v string) bool {
	return v == VerdictPass || v == VerdictWarn || v == VerdictFail
}

// Validate requires a cycle, a goal, valid verdicts and at least one phase, and
// for a FAIL at least one defect (why) and one carryover (the fix work).
func (d *Dossier) Validate() error {
	if d.Cycle <= 0 {
		return fmt.Errorf("dossier: cycle must be >= 1")
	}
	if strings.TrimSpace(d.Goal) == "" {
		return fmt.Errorf("dossier: cycle %d: goal is empty", d.Cycle)
	}
	if !validVerdict(d.FinalVerdict) {
		return fmt.Errorf("dossier: cycle %d: final_verdict %q must be PASS|WARN|FAIL", d.Cycle, d.FinalVerdict)
	}
	if len(d.Phases) == 0 {
		return fmt.Errorf("dossier: cycle %d: no phases recorded", d.Cycle)
	}
	for i, p := range d.Phases {
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("dossier: cycle %d: phase[%d] has empty name", d.Cycle, i)
		}
		if !validVerdict(p.Verdict) {
			return fmt.Errorf("dossier: cycle %d: phase %q verdict %q must be PASS|WARN|FAIL", d.Cycle, p.Name, p.Verdict)
		}
	}
	for i, df := range d.Defects {
		if strings.TrimSpace(df.ID) == "" || strings.TrimSpace(df.Summary) == "" {
			return fmt.Errorf("dossier: cycle %d: defect[%d] needs id + summary", d.Cycle, i)
		}
	}
	for i, c := range d.Carryover {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Action) == "" {
			return fmt.Errorf("dossier: cycle %d: carryover[%d] needs id + action", d.Cycle, i)
		}
	}
	if d.FinalVerdict == VerdictFail {
		if len(d.Defects) == 0 {
			return fmt.Errorf("dossier: cycle %d: FAIL verdict must record >=1 defect (why it failed)", d.Cycle)
		}
		if len(d.Carryover) == 0 {
			return fmt.Errorf("dossier: cycle %d: FAIL verdict must record >=1 carryover (the fix work)", d.Cycle)
		}
	}
	return nil
}

// HasCommitment reports whether a triage commitment was recorded, even an empty one.
func (d *Dossier) HasCommitment() bool { return d != nil && d.Tasks != nil }

// CommitmentLine renders the committed task ids for the markdown record; an
// explicit empty commitment renders as a stated fact, not a blank.
func (d *Dossier) CommitmentLine() string {
	if !d.HasCommitment() {
		return ""
	}
	ids := *d.Tasks
	if len(ids) == 0 {
		return "_(nothing — this cycle committed to no task)_"
	}
	return "`" + strings.Join(ids, "`, `") + "`"
}

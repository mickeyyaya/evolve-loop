// Package dossier is the durable, structured, cross-loop history/experience
// record for an evolve-loop cycle (ADR-0055). Today every cycle's structured
// data (phase reports, ledger, lessons, carryover) lives in gitignored runtime
// (.evolve/) and is lost across sessions/branches/loops. The Dossier aggregates
// it into ONE committed artifact (knowledge-base/cycles/cycle-N.json) that the
// next cycle's Scout — and any other session or loop — reads as the source of
// truth, so the project learns from experience including FAILED verdicts.
//
// The Go struct + Validate() are the SSOT (deterministic; the project does not
// use a JSON-Schema-v2020-12 validator). schemas/cycle-dossier.schema.json is the
// committed human/cross-tool reference, kept in sync by a drift test.
package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// Verdict values a cycle (or phase) can carry.
const (
	VerdictPass = "PASS"
	VerdictWarn = "WARN"
	VerdictFail = "FAIL"
)

// Dossier is the aggregated, committed record of one cycle.
type Dossier struct {
	Cycle        int           `json:"cycle"`
	RunID        string        `json:"run_id,omitempty"`
	Goal         string        `json:"goal"`
	FinalVerdict string        `json:"final_verdict"`
	CommitSHA    string        `json:"commit_sha,omitempty"`
	TreeSHA      string        `json:"tree_sha,omitempty"`
	Phases       []PhaseRecord `json:"phases"`
	Defects      []Defect      `json:"defects,omitempty"`
	Decisions    []string      `json:"decisions,omitempty"`
	Lessons      []Lesson      `json:"lessons,omitempty"`
	Carryover    []Carryover   `json:"carryover,omitempty"`
	StartedAt    string        `json:"started_at,omitempty"`
	EndedAt      string        `json:"ended_at,omitempty"`
	// Timing is the cycle-level latency roll-up (where the wall-clock went),
	// ingested from phase-timing.json. Nil when the cycle wrote no timing log.
	Timing *phasetiming.Summary `json:"timing,omitempty"`
}

// PhaseRecord is one phase's outcome within the cycle (mirrors a ledger entry +
// its handoff report). The timing fields (omitempty) carry the durable per-phase
// latency evidence ingested from phase-timing.json.
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
}

// Defect is one audit finding (the H1/H2 taxonomy) — preserved so a failed cycle
// records WHY it failed and how to fix it.
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

// Validate is the deterministic trust boundary: a dossier is well-formed only if
// it identifies the cycle + goal, carries a valid final verdict and at least one
// phase, and — crucially — a FAILED cycle records BOTH why it failed (>=1 defect)
// AND the work to fix it (>=1 carryover), so no failure's experience is lost.
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

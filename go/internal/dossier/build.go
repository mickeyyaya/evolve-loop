package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// timingRecords reads phase-timing.json from the cycle workspace and projects it
// into per-phase dossier records plus the cycle-level roll-up. Returns ok=false
// when no usable log exists, so Build keeps its always-valid stub.
func timingRecords(workspace string) ([]PhaseRecord, *phasetiming.Summary, bool) {
	entries, err := phasetiming.Read(workspace)
	if err != nil || len(entries) == 0 {
		return nil, nil, false
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

// normalizeVerdict maps a timing verdict onto the canonical dossier vocabulary,
// defaulting an unknown/blank value to WARN so Validate cannot reject a record
// for a legacy or hand-edited log entry.
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
	// WorkspacePath is the cycle workspace directory (contains *-report.md files).
	WorkspacePath string
	// LedgerPath is the path to ledger.jsonl (for phase record extraction).
	LedgerPath string
	// Goal is the cycle goal text.
	Goal string
	// RunID is the cycle run ULID (CA.2).
	RunID string
	// FinalVerdict is the cycle's REAL outcome (PASS|WARN|FAIL). Empty defaults
	// to PASS (back-compat with the always-PASS skeleton). A FAIL value makes
	// Build synthesize a minimal defect + carryover pointing at the audit
	// artifacts, so the dossier records WHY the cycle failed and still satisfies
	// Validate — the producer never fabricates a PASS for a failed cycle.
	FinalVerdict string
}

// Build assembles a Dossier for the given cycle. It validates the cycle number,
// then constructs a Dossier from BuildOpts. When no LedgerPath is provided (or
// the file is absent), Build synthesises a "cycle-recorded" phase so the returned
// Dossier is always valid (Validate passes when FinalVerdict is PASS). Callers
// that have a real ledger should set BuildOpts.LedgerPath.
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
		Cycle:        cycle,
		Goal:         opts.Goal,
		RunID:        opts.RunID,
		FinalVerdict: verdict,
		Phases: []PhaseRecord{
			{
				Name:        "cycle-recorded",
				Verdict:     verdict,
				KeyFindings: "cycle completed; ledger walk deferred to future slice",
			},
		},
	}
	// Ingest the per-phase timing log when present: real per-phase records +
	// the cycle-level roll-up replace the stub, so the committed dossier carries
	// the durable latency evidence. Absent/empty log ⇒ the stub stands (the
	// always-valid back-compat skeleton).
	if records, summary, ok := timingRecords(opts.WorkspacePath); ok {
		d.Phases = records
		d.Timing = summary
	}
	// Ingest the post-push CI-watch verdict when the workspace recorded one
	// (ci-watch-verdict.json). Absent artifact ⇒ nil — never fabricated.
	if rec, ok := ciWatchRecord(opts.WorkspacePath); ok {
		d.CIWatch = rec
	}
	// A FAIL cycle must record BOTH why it failed and the fix work (Validate
	// enforces >=1 defect + >=1 carryover). Without a ledger walk we synthesize a
	// minimal, truthful pair that points at the audit artifacts rather than
	// inventing specifics — the future ledger-walk slice replaces these.
	if verdict == VerdictFail {
		d.Defects = []Defect{{
			ID:       "audit-fail",
			Severity: "HIGH",
			Summary:  "cycle did not pass audit; see audit-report.md + acs-verdict.json",
			Fix:      "address the audit findings recorded for this cycle",
		}}
		d.Carryover = []Carryover{{
			ID:       "address-audit-findings",
			Action:   fmt.Sprintf("resolve the audit findings that failed cycle %d", cycle),
			Priority: "high",
		}}
	}
	return d, nil
}

// resolveBuildVerdict maps an optional BuildOpts.FinalVerdict to a valid verdict:
// empty defaults to PASS (back-compat); a known verdict passes through; anything
// else errors loudly rather than silently defaulting. Pure.
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

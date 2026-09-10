package dossier

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// auditArtifactName is the audit deliverable's filename, DERIVED from the
// phasecontract registry (the SSOT for report filenames) rather than re-typed.
// Cycle-1141: the registry exists precisely so this vocabulary is declared once
// — a synthesized defect that points at a stale filename sends the next cycle
// looking for a file that is no longer written.
func auditArtifactName() string {
	if c, ok := phasecontract.For("audit"); ok && c.ArtifactName != "" {
		return c.ArtifactName
	}
	return "the audit report"
}

// timingRecords reads phase-timing.json from the cycle workspace and projects it
// into per-phase dossier records plus the cycle-level roll-up. Returns ok=false
// when no usable log exists, so Build keeps its always-valid stub.
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
	// NOTE: no production caller sets it — the phase records come from
	// PhaseTimings (or the workspace log). Kept for the ledger-walk slice that
	// ADR-0055 describes; until that lands, absence of a ledger is NOT what
	// makes a record degrade.
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
	// SkippedPhases are phases that genuinely did NOT run, with the skip cause.
	// Surfaced verbatim.
	SkippedPhases []cyclestate.SkippedPhase
	// VerdictsNotAdopted are non-floor phases that RAN whose non-PASS outcome was
	// declined rather than allowed to clobber the floor-derived FinalVerdict
	// (cycle-802). Surfaced verbatim so the dossier records the degrade, never
	// dropping it — and never as a "skipped phase" (dossier-retro-skipped-mislabel).
	VerdictsNotAdopted []cyclestate.VerdictNotAdopted
	// SpineFailOpens are the cycle's spine-gate fail-open events (cycle-1166),
	// surfaced verbatim so the dossier is where the epidemic becomes visible.
	SpineFailOpens []cyclestate.SpineFailOpen
	// PhaseTimings is the per-phase evidence the caller already holds — for the
	// orchestrator, the set core composed and flushed ONCE
	// (cycleRun.flushPhaseTimings), so the dossier projects exactly the record
	// the on-disk log receives. It is passed rather than re-read because that
	// log is written by a DEFERRED call in RunCycle and lands AFTER the dossier
	// is produced on the normal path (cycle-1623: twelve phases ran, one
	// synthetic phase was recorded).
	//
	// Empty ⇒ fall back to reading the workspace log. That fallback is live,
	// not vestigial: a dossier built for a workspace whose live timings were
	// never threaded has the file as its only evidence.
	PhaseTimings []phasetiming.Entry
}

// Build assembles a Dossier for the given cycle. It validates the cycle number,
// then constructs a Dossier from BuildOpts. When no per-phase evidence is
// available at all — neither live timings nor a readable log — Build records a single
// `evidence-unavailable` phase at WARN naming the degradation — the returned
// Dossier stays valid, but it never claims a phase verdict no phase produced.
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
		Cycle:                      cycle,
		Goal:                       opts.Goal,
		RunID:                      opts.RunID,
		FinalVerdict:               verdict,
		Phases:                     []PhaseRecord{evidenceUnavailablePhase()},
		SkippedPhases:              opts.SkippedPhases,
		PhasesRunVerdictNotAdopted: opts.VerdictsNotAdopted,
		SpineFailOpens:             opts.SpineFailOpens,
	}
	// Ingest the per-phase timing log when present: real per-phase records +
	// the cycle-level roll-up replace the stub, so the committed dossier carries
	// the durable latency evidence. Absent/empty log ⇒ the stub stands (the
	// always-valid back-compat skeleton).
	if records, summary, ok := timingRecords(opts.WorkspacePath, opts.PhaseTimings); ok {
		d.Phases = records
		d.Timing = summary
	}
	// Project the ship phase's own proof of delivery. Absent binding ⇒ the
	// field stays empty: the record claims no commit it cannot point at.
	if commit, tree, ok := shippedCommit(opts.WorkspacePath); ok {
		d.CommitSHA, d.TreeSHA = commit, tree
	}
	// Project the committed task set. A PRESENT decision with an empty top_n
	// records an empty (non-nil) list — "this cycle committed to nothing" is
	// the finding, not a missing field.
	if tasks, ok := committedTasks(opts.WorkspacePath); ok {
		d.Tasks = &tasks
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
			Summary:  fmt.Sprintf("cycle did not pass audit; see %s + acs-verdict.json", auditArtifactName()),
			Fix:      "address the audit findings recorded for this cycle",
		}}
		d.Carryover = []Carryover{{
			ID:       "address-audit-findings",
			Action:   fmt.Sprintf("resolve the audit findings that failed cycle %d", cycle),
			Priority: "high",
		}}
		// Ingest the failure identity the workspace already carries. RESIDUAL
		// (review MEDIUM): failure-digest.json is written when the dispatch
		// loop routes into retro, so a FAIL reclassified at finalize without a
		// retro dispatch yields reasons[] only — the block shrinks honestly
		// rather than fabricating an identity.
		if rec, ok := failureRecord(opts.WorkspacePath, cycle); ok {
			d.Failure = rec
		}
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

// evidenceUnavailablePhase is the record a dossier carries when NEITHER the
// live timings nor the on-disk log yielded per-phase evidence. It replaces the
// former synthesized "cycle-recorded" phase, which carried the CYCLE's verdict
// as though a phase had produced it: a twelve-phase cycle then read as a
// one-phase PASS, and the missing evidence was invisible precisely when the
// record mattered most. WARN keeps the marker inside the validated verdict
// vocabulary while stating plainly that this record is degraded; the cycle's
// own FinalVerdict is untouched.
func evidenceUnavailablePhase() PhaseRecord {
	return PhaseRecord{
		Name:    "evidence-unavailable",
		Verdict: VerdictWarn,
		KeyFindings: "per-phase evidence unavailable at dossier build (no live timings; " +
			phasetiming.FileName + " absent or empty) — this record is DEGRADED, not a one-phase cycle",
	}
}

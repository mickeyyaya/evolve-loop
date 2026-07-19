// Package coherence computes the verdict-coherence signal (ADR-0072 S2): does a
// cycle's recorded verdict agree with the on-disk artifacts the phases actually
// wrote? A recorded FAIL/WARN that contradicts a green audit report AND a green
// ACS verdict means the pipeline forged the verdict — "verdict-incoherence",
// the clean-exit signature (cycles 862→899). This is the one deterministic
// input that lets the Go floor and the orchestrator catch a broken pipeline
// lying about whose fault a failure is.
//
// The pure comparison (CheckVerdictCoherence) is I/O-free and table-tested; the
// artifact reader (ReadCycleVerdicts) is the thin I/O adapter that feeds it.
package coherence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// VerdictInputs is the independent evidence for the coherence check. The
// recorded verdict is NEVER trusted as ground truth on its own — the whole
// point is to compare it against the artifacts.
type VerdictInputs struct {
	Recorded string // final recorded cycle verdict: PASS/FAIL/WARN
	Audit    string // on-disk audit evolve-verdict, "" if absent
	ACS      string // acs-verdict.json verdict, "" if absent
	AuditRan bool   // audit phase ran and wrote a report
	// SubstantiveError: the negative verdict is EXPLAINED by real recorded
	// evidence — a substantive (non-infra-teardown) bridge error, or a diagnosed
	// runner-side gate downgrade (audit's CI-parity gates overrode a narrative
	// PASS; persisted as audit-fail-reason.json error-severity reasons). An
	// explained negative is a coherent task-level outcome, never forgery — the
	// cycles-930/931/932 false-HALT was this field left unpopulated at the sole
	// call site while the explanation sat in the response diagnostics.
	SubstantiveError bool
	// DeliverableValid: the on-disk audit-report passed the FULL deliverable.Verify
	// chain (challenge-token + required sections + ADR-0039 failure-context), NOT
	// the cheap ParseVerdictSentinel read. It DOWNGRADES the forgery signature to a
	// benign clean-exit-late-write race: when the recorded-negative is contradicted
	// by green artifacts AND the deliverable fully verifies, the bridge merely
	// declared the phase's clean exit before Claude Code finished its post-turn
	// async writes (the runner's ~3s settle window < the observed 60-90s dribble,
	// cycles 930/931/932/cycle-3) — self-heal, do not halt. A PASS-sentinel-tagged
	// but malformed report yields DeliverableValid=false → still forgery → halt
	// (the anti-laundering boundary). It NEVER manufactures a reconcile out of a
	// case that is coherent without it — see CheckVerdictCoherence.
	DeliverableValid bool
}

// Coherence is the result of the check. Incoherent and Reconciled are mutually
// exclusive: at most one of the two fires, and only within the forgery signature.
type Coherence struct {
	Incoherent bool
	// Reconciled marks a recorded-negative that was a benign clean-exit-late-write
	// race (green artifacts AND a fully-valid deliverable) → self-heal the recorded
	// verdict to PASS, do NOT halt. Mutually exclusive with Incoherent.
	Reconciled bool
	Category   string // "verdict-incoherence" when Incoherent; "verdict-reconciled" when Reconciled
	Evidence   string
}

// CheckVerdictCoherence is the pure signal. Incoherent ⇔ the cycle recorded a
// negative verdict while every authoritative artifact is green and no
// substantive error justified the negative — i.e. the recorded verdict was
// forged by the pipeline, not earned by the task.
func CheckVerdictCoherence(in VerdictInputs) Coherence {
	rec := strings.ToUpper(strings.TrimSpace(in.Recorded))
	if rec != "FAIL" && rec != "WARN" {
		return Coherence{} // PASS/empty recorded → nothing to contradict
	}
	if !in.AuditRan {
		return Coherence{} // audit never ran → a recorded FAIL is a genuine incomplete
	}
	if in.SubstantiveError {
		return Coherence{} // a real error justifies the negative verdict
	}
	audit := strings.ToUpper(strings.TrimSpace(in.Audit))
	acs := strings.ToUpper(strings.TrimSpace(in.ACS))
	// Both authoritative artifacts must be PRESENT and PASS to claim the
	// verdict was forged. An absent ACS (or audit) means we cannot prove
	// incoherence — err toward coherent (no false halt).
	if audit == "PASS" && acs == "PASS" {
		// The recorded-negative contradicts green artifacts. DeliverableValid is
		// the tie-breaker between a benign timing race and a genuine forgery: a
		// fully-verified deliverable means the bridge just declared clean-exit
		// before the (valid) report finished landing → self-heal to PASS. A
		// malformed report merely tagged with a PASS sentinel stays a forgery →
		// halt. DeliverableValid only ever downgrades this would-be halt; it is
		// unreachable for every coherent case guarded out above.
		if in.DeliverableValid {
			return Coherence{
				Reconciled: true,
				Category:   "verdict-reconciled",
				Evidence: "recorded=" + rec + " but on-disk audit=PASS, acs=PASS, and the audit-report passes the FULL " +
					"deliverable.Verify chain (challenge-token + required sections + ADR-0039 failure-context) — a benign " +
					"clean-exit-late-write race (the bridge declared clean exit before the valid report finished landing); " +
					"self-heal the recorded verdict to PASS, do not halt (ADR-0072)",
			}
		}
		return Coherence{
			Incoherent: true,
			Category:   "verdict-incoherence",
			Evidence: "recorded=" + rec + " but on-disk audit=PASS and acs=PASS with no substantive error and a " +
				"deliverable that does NOT fully verify — the recorded verdict contradicts the phases' own green " +
				"artifacts (pipeline-forged verdict, or an unaudited post-audit block); halt + diagnose the pipeline, " +
				"do not retry the task (ADR-0072)",
		}
	}
	return Coherence{}
}

// ReadCycleVerdicts extracts the audit evolve-verdict and the acs-verdict from a
// cycle workspace directory. The audit verdict is read via the canonical
// phasecontract.ParseVerdictSentinel (anchored to the <!-- evolve-verdict -->
// sentinel, with the placeholder-echo guard) — never a bespoke regex, which
// would re-open the cycle-603 echo bug on the very signal this gate depends on.
// Missing/malformed artifacts yield empty strings and auditRan=false — never an
// error and never a fabricated verdict (a reader that guessed would defeat the
// whole coherence check).
func ReadCycleVerdicts(workspace string) (audit, acs string, auditRan bool) {
	if b, err := os.ReadFile(filepath.Join(workspace, "audit-report.md")); err == nil {
		auditRan = true
		if v, ok := phasecontract.ParseVerdictSentinel(string(b)); ok {
			audit = v
		}
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json")); err == nil {
		var v struct {
			Verdict string `json:"verdict"`
		}
		if json.Unmarshal(b, &v) == nil {
			acs = v.Verdict
		}
	}
	return audit, acs, auditRan
}

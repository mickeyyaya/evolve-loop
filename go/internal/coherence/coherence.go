// Package coherence checks whether a cycle's recorded verdict agrees with the
// artifacts its phases wrote, and runs the advisory cross-artifact invariant stack.
// See docs/architecture/packages/internal-coherence.md.
package coherence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// VerdictInputs is the evidence the recorded verdict is checked against; the recorded verdict alone is never trusted.
type VerdictInputs struct {
	Recorded string // final recorded cycle verdict: PASS/FAIL/WARN
	Audit    string // on-disk audit evolve-verdict, "" if absent
	ACS      string // acs-verdict.json verdict, "" if absent
	AuditRan bool   // audit phase ran and wrote a report
	// SubstantiveError marks a negative verdict explained by recorded evidence (a
	// substantive bridge error or a diagnosed gate downgrade); it is never forgery.
	SubstantiveError bool
	// FailReasons carries the untruncated explanations behind SubstantiveError.
	FailReasons []string
	// DeliverableValid means the audit report passed the full deliverable.Verify
	// chain, not just the sentinel read; it only downgrades forgery to a reconcile.
	DeliverableValid bool
}

// Coherence is the check's result; Incoherent and Reconciled are mutually exclusive.
type Coherence struct {
	Incoherent bool
	// Reconciled marks a benign clean-exit-late-write race: self-heal the recorded
	// verdict to PASS instead of halting.
	Reconciled bool
	Category   string // "verdict-incoherence" when Incoherent; "verdict-reconciled" when Reconciled
	Evidence   string
}

// CheckVerdictCoherence flags a recorded FAIL/WARN that green audit and ACS artifacts contradict and no substantive error explains.
func CheckVerdictCoherence(in VerdictInputs) Coherence {
	rec := strings.ToUpper(strings.TrimSpace(in.Recorded))
	if rec != "FAIL" && rec != "WARN" {
		return Coherence{}
	}
	if !in.AuditRan {
		return Coherence{} // audit never ran → a recorded FAIL is a genuine incomplete
	}
	if in.SubstantiveError {
		return Coherence{}
	}
	audit := strings.ToUpper(strings.TrimSpace(in.Audit))
	acs := strings.ToUpper(strings.TrimSpace(in.ACS))
	// An absent audit or ACS verdict cannot prove forgery, so err toward coherent (no false halt).
	if audit == "PASS" && acs == "PASS" {
		// A fully verified report means the bridge declared clean exit before it
		// finished landing; a malformed PASS-tagged report stays a forgery.
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

// ReadCycleVerdicts returns a workspace's audit and ACS verdicts ("" when absent or unparseable, never guessed) and whether the audit report exists.
func ReadCycleVerdicts(workspace string) (audit, acs string, auditRan bool) {
	if b, err := os.ReadFile(filepath.Join(workspace, phasecontract.ArtifactFilename("audit"))); err == nil {
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

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

type VerdictInputs struct {
	Recorded         string
	Audit            string
	ACS              string
	AuditRan         bool
	SubstantiveError bool
	DeliverableValid bool
}

type Coherence struct {
	Incoherent bool
	Reconciled bool
	Category   string
	Evidence   string
}

func CheckVerdictCoherence(in VerdictInputs) Coherence {
	rec := strings.ToUpper(strings.TrimSpace(in.Recorded))
	if rec != "FAIL" && rec != "WARN" {
		return Coherence{}
	}
	if !in.AuditRan {
		return Coherence{}
	}
	if in.SubstantiveError {
		return Coherence{}
	}
	audit := strings.ToUpper(strings.TrimSpace(in.Audit))
	acs := strings.ToUpper(strings.TrimSpace(in.ACS))
	if audit == "PASS" && acs == "PASS" {
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

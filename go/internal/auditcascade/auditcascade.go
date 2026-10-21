package auditcascade

import (
	"fmt"
	"slices"
	"strings"
)

const RecordFile = "audit-cascade-shadow.json"

type Rule struct {
	RiskPaths        []string
	MaxChangedFiles  int
	EscalateVerdicts []string
}

type Inputs struct {
	Changed []string
	Verdict string
	Known   bool
}

type Decision struct {
	Escalate bool     `json:"would_escalate"`
	Reasons  []string `json:"reasons,omitempty"`
}

func Decide(in Inputs, r Rule) Decision {
	var reasons []string
	if slices.Contains(r.EscalateVerdicts, in.Verdict) {
		reasons = append(reasons, "verdict "+in.Verdict)
	}
	if !in.Known {
		reasons = append(reasons, "the diff is unknown")
	}
	if path, ok := firstRiskPath(in.Changed, r.RiskPaths); ok {
		reasons = append(reasons, "risk path "+path)
	}
	if r.MaxChangedFiles > 0 && len(in.Changed) > r.MaxChangedFiles {
		reasons = append(reasons, fmt.Sprintf("%d changed files > %d", len(in.Changed), r.MaxChangedFiles))
	}
	return Decision{Escalate: len(reasons) > 0, Reasons: reasons}
}

func firstRiskPath(changed, risk []string) (string, bool) {
	for _, path := range changed {
		for _, prefix := range risk {
			if strings.HasPrefix(path, prefix) {
				return path, true
			}
		}
	}
	return "", false
}

type Record struct {
	Stage        string `json:"stage"`
	Cycle        int    `json:"cycle"`
	Verdict      string `json:"verdict"`
	ChangedFiles int    `json:"changed_files"`
	Decision
}

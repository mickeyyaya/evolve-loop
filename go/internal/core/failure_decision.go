package core

// failure_decision.go — ADR-0072 S4 Task 2 (failure-decision-schema-reader).
// The orchestrator (retrospective agent) may emit failure-decision.json with its
// classification of a cycle's failure. This reader is the FALLBACK BOUNDARY:
// a malformed / absent / schema-invalid artifact yields (nil, nil) — the signal
// to fall back to the deterministic failureadapter — NEVER an error that aborts
// the cycle (retro_always_on_failure). Only an in-vocabulary {action, level}
// decision is honored, so a garbled artifact can never drive an unrecognized
// branch.
//
// The symbol is deliberately UNEXPORTED (JSON-tagged fields only) — no new
// apicover-gated public surface.

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// failureDecision is the orchestrator's authored classification, matching the
// schema the retrospective agent's instructions document (Task 4).
type failureDecision struct {
	Category      string `json:"category"`
	Level         string `json:"level"`
	Evidence      string `json:"evidence,omitempty"`
	Justification string `json:"justification,omitempty"`
	Action        string `json:"action"`
	FixType       string `json:"fix_type,omitempty"`
	SchemaVersion int    `json:"schema_version,omitempty"`
}

// readFailureDecision reads <workspace>/failure-decision.json. Absent, malformed,
// or schema-invalid (action/level outside the policy vocabulary) ALL return
// (nil, nil) — the deterministic-fallback signal, never a cycle-aborting error.
func readFailureDecision(workspace string) (*failureDecision, error) {
	b, err := os.ReadFile(filepath.Join(workspace, "failure-decision.json"))
	if err != nil {
		return nil, nil // absent → fall back to the deterministic adapter
	}
	var d failureDecision
	if json.Unmarshal(b, &d) != nil {
		return nil, nil // malformed JSON → fall back, do not abort
	}
	if !validDecisionAction(d.Action) || !validDecisionLevel(d.Level) {
		return nil, nil // schema-invalid → fall back rather than drive an unknown branch
	}
	return &d, nil
}

// validDecisionAction reports whether action is in the ADR-0072 action vocabulary.
func validDecisionAction(action string) bool {
	switch action {
	case policy.ActionHaltAndDiagnose, policy.ActionRetryWithFix, policy.ActionDeferOrQuarantine:
		return true
	default:
		return false
	}
}

// validDecisionLevel reports whether level is in the ADR-0072 level vocabulary.
func validDecisionLevel(level string) bool {
	return level == policy.LevelSystem || level == policy.LevelTask
}

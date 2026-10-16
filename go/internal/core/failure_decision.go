package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type failureDecision struct {
	Category      string `json:"category"`
	Level         string `json:"level"`
	Evidence      string `json:"evidence,omitempty"`
	Justification string `json:"justification,omitempty"`
	Action        string `json:"action"`
	FixType       string `json:"fix_type,omitempty"`
	SchemaVersion int    `json:"schema_version,omitempty"`
}

func readFailureDecision(workspace string) (*failureDecision, error) {
	b, err := os.ReadFile(filepath.Join(workspace, "failure-decision.json"))
	if err != nil {
		return nil, nil
	}
	var d failureDecision
	if json.Unmarshal(b, &d) != nil {
		return nil, nil
	}
	if !validDecisionAction(d.Action) || !validDecisionLevel(d.Level) {
		return nil, nil
	}
	return &d, nil
}

func validDecisionAction(action string) bool {
	switch action {
	case policy.ActionHaltAndDiagnose, policy.ActionRetryWithFix, policy.ActionDeferOrQuarantine:
		return true
	default:
		return false
	}
}

func validDecisionLevel(level string) bool {
	return level == policy.LevelSystem || level == policy.LevelTask
}

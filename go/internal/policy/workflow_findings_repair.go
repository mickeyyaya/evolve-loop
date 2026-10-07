package policy

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

type FindingsRepairPolicy struct {
	CodeReview *JudgeRepairPolicy `json:"code-review,omitempty"`
}

type JudgeRepairPolicy struct {
	Stage     string `json:"stage,omitempty"`
	Threshold string `json:"threshold,omitempty"`
}

type JudgeRepairConfig struct {
	Stage     string
	Threshold string
	Warnings  []string
}

const defaultRepairThreshold = "MEDIUM"

func (p Policy) codeReviewRepairPolicy() *JudgeRepairPolicy {
	if p.Workflow == nil || p.Workflow.FindingsRepair == nil {
		return nil
	}
	return p.Workflow.FindingsRepair.CodeReview
}

func resolveJudgeRepair(judge string, jp *JudgeRepairPolicy) JudgeRepairConfig {
	c := JudgeRepairConfig{Stage: "shadow", Threshold: defaultRepairThreshold}
	if jp == nil {
		return c
	}
	key := "workflow.findings_repair." + judge
	switch jp.Stage {
	case "", "shadow":
	case "enforce":
		c.Warnings = append(c.Warnings, fmt.Sprintf("%s.stage: \"enforce\" lands with the review loop and the audit cross-check (ADR-0124 Q3–Q5); holding \"shadow\"", key))
	default:
		c.Warnings = append(c.Warnings, fmt.Sprintf("%s.stage: unknown value %q, falling back to \"shadow\"", key, jp.Stage))
	}
	if t := strings.ToUpper(strings.TrimSpace(jp.Threshold)); t != "" {
		if knownSeverity(t) {
			c.Threshold = t
		} else {
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s.threshold: unknown severity %q, falling back to %q", key, jp.Threshold, defaultRepairThreshold))
		}
	}
	return c
}

func knownSeverity(s string) bool {
	return reportdoc.SeverityRank(s) < reportdoc.SeverityRank("")
}

package deliverable

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// CodeHandoffBudgetExceeded is the stable code for a Handoff Summary estimated over its token budget.
const CodeHandoffBudgetExceeded = "handoff_budget_exceeded"

// charsPerToken is the common ~4-characters-per-token heuristic; the repo carries no tokenizer.
const charsPerToken = 4

// EstimateTokens returns a deterministic token estimate for s at ~4 characters per token.
func EstimateTokens(s string) int {
	return len(s) / charsPerToken
}

// HandoffSectionContent returns the "## Handoff Summary" body up to the next "## " heading; ok is false when it is absent.
func HandoffSectionContent(content string) (string, bool) {
	const heading = "## Handoff Summary"
	lines := strings.Split(content, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	var body []string
	for _, ln := range lines[start+1:] {
		if strings.HasPrefix(ln, "## ") {
			break
		}
		body = append(body, ln)
	}
	return strings.Join(body, "\n"), true
}

// CheckHandoffBudget reports whether the Handoff Summary exceeds budgetTokens; an absent section is never a violation.
func CheckHandoffBudget(content string, budgetTokens int) (violated bool, estimated int) {
	body, ok := HandoffSectionContent(content)
	if !ok {
		return false, 0
	}
	estimated = EstimateTokens(body)
	return estimated > budgetTokens, estimated
}

// VerifyWithReportSize is VerifyWithStage plus the Handoff Summary budget, recorded from advisory; the Reviewer blocks on it only at enforce.
func VerifyWithReportSize(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver, phaseIO, reportSizeGate config.Stage, budgetTokens int) (Result, error) {
	res, err := VerifyWithStage(phase, roots, resolver, phaseIO)
	if err != nil {
		return res, err
	}
	if reportSizeGate < config.StageAdvisory {
		return res, nil
	}
	data, readErr := os.ReadFile(res.ArtifactPath)
	if readErr != nil {
		// VerifyWithStage already reported the missing artifact.
		return res, nil
	}
	if violated, estimated := CheckHandoffBudget(string(data), budgetTokens); violated {
		res.add(CodeHandoffBudgetExceeded, fmt.Sprintf(
			"handoff summary section is ~%d estimated tokens, over the %d-token budget — move detail out of the never-evict summary into an evictable detail section referenced by path", estimated, budgetTokens))
		res.finish()
	}
	return res, nil
}

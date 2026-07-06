package deliverable

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// reportsize.go — cycle-565 Slice S1 of report-size-contracts-jit-artifacts: a
// per-artifact token/size budget on the never-evict "## Handoff Summary" section
// (phasecontract.HandoffSummary). The section's PRESENCE rides the normal
// contract gate (CodeMissingSection); its SIZE rides a separate shadow-first
// rollout dial so a miscalibrated budget can be observed before it can ever
// block a cycle.

// CodeHandoffBudgetExceeded is the stable violation code for a Handoff Summary
// section that estimates over its token budget.
const CodeHandoffBudgetExceeded = "handoff_budget_exceeded"

// tokensPerChar approximates the common ~4-chars-per-token heuristic (no
// tokenizer dependency exists in this repo's go.mod; the same byte-length
// heuristic backs other budgets like core.salvageMaxBytes).
const charsPerToken = 4

// EstimateTokens returns a deterministic, monotonic token estimate for s using
// the ~4-chars-per-token heuristic (integer division rounds down). It is pure
// and allocation-free — same input always yields the same estimate.
func EstimateTokens(s string) int {
	return len(s) / charsPerToken
}

// HandoffSectionContent returns the body of the "## Handoff Summary" section —
// everything after the heading line up to (but not including) the next "## "
// heading, or to EOF if none follows. ok is false when the section is absent
// (its absence is CodeMissingSection's job, not the budget check's).
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
			break // next section heading — the summary ends here
		}
		body = append(body, ln)
	}
	return strings.Join(body, "\n"), true
}

// CheckHandoffBudget reports whether the Handoff Summary section's estimated
// token count exceeds budgetTokens. An absent section is never a violation
// (violated=false, estimated=0) — that is CodeMissingSection's responsibility.
func CheckHandoffBudget(content string, budgetTokens int) (violated bool, estimated int) {
	body, ok := HandoffSectionContent(content)
	if !ok {
		return false, 0
	}
	estimated = EstimateTokens(body)
	return estimated > budgetTokens, estimated
}

// VerifyWithReportSize is VerifyWithStage threaded with the report-size gate's
// own rollout stage (cycle-565 Slice S1) — exactly as VerifyWithStage was added
// as a new layer over VerifyWith rather than changing an existing signature, so
// no existing call site (cmd_phase_verify.go, reviewer.go, verifier.go,
// catalogaware.go) churns. The reportSizeGate dial is INDEPENDENT of the
// ContractGate stage: dormant at off/shadow (byte-identical Violations to
// VerifyWithStage), blocking only at enforce.
func VerifyWithReportSize(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver, phaseIO, reportSizeGate config.Stage, budgetTokens int) (Result, error) {
	res, err := VerifyWithStage(phase, roots, resolver, phaseIO)
	if err != nil {
		return res, err
	}
	// Off/shadow: observe-only. Leave Violations byte-identical to
	// VerifyWithStage so wiring the layer in cannot change existing behavior for
	// any cycle that has not opted the gate up to enforce.
	if reportSizeGate < config.StageEnforce {
		return res, nil
	}
	data, readErr := os.ReadFile(res.ArtifactPath)
	if readErr != nil {
		// An absent/unreadable artifact is already reported by VerifyWithStage
		// (CodeMissingArtifact); there is nothing to budget-check.
		return res, nil
	}
	if violated, estimated := CheckHandoffBudget(string(data), budgetTokens); violated {
		res.add(CodeHandoffBudgetExceeded, fmt.Sprintf(
			"handoff summary section is ~%d estimated tokens, over the %d-token budget — move detail out of the never-evict summary into an evictable detail section referenced by path", estimated, budgetTokens))
		res.finish()
	}
	return res, nil
}

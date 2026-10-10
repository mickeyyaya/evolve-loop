package subagent

import (
	"fmt"
	"os"
	"strconv"
)

type CheckCtxAdvisoryResult struct {
	Emit      bool
	Threshold int
	Message   string
}

func CheckCtxAdvisory(profilePath string, tokens int) (CheckCtxAdvisoryResult, error) {
	body, err := os.ReadFile(profilePath)
	if err != nil {
		return CheckCtxAdvisoryResult{}, fmt.Errorf("subagent/ctxadvisory: read profile %s: %w", profilePath, err)
	}
	rawThreshold := matchField(string(body), reFieldCtxTokens)
	if rawThreshold == "" {
		return CheckCtxAdvisoryResult{Emit: false}, nil
	}
	threshold, err := strconv.Atoi(rawThreshold)
	if err != nil {
		return CheckCtxAdvisoryResult{Emit: false}, nil
	}
	res := CheckCtxAdvisoryResult{Threshold: threshold}
	if tokens > threshold {
		res.Emit = true
		res.Message = fmt.Sprintf(
			"test-agent context at ~%d tokens; profile threshold=%d (context_clear_trigger_tokens). Agent should apply Tool-Result Hygiene before further tool calls.",
			tokens, threshold,
		)
	}
	return res, nil
}

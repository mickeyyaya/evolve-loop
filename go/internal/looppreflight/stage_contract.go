package looppreflight

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const stageContractCheck = "stage-contract"

func CheckStageContract(source func() (stageevidence.CheckResult, error)) CheckResult {
	if source == nil {
		return CheckResult{Name: stageContractCheck, Level: LevelPass, Message: "stage flip records not measured"}
	}
	res, err := source()
	if err != nil {
		return CheckResult{Name: stageContractCheck, Level: LevelWarn, Message: "the stage flip records could not be read", Detail: err.Error()}
	}
	detail := findingsDetail(res.Findings)
	switch {
	case res.Mode == stageevidence.ModeRefuse:
		return CheckResult{Name: stageContractCheck, Level: LevelHalt, Message: "genesis_required is true and the stage-genesis record is missing", Detail: detail}
	case len(res.Findings) > 0:
		return CheckResult{Name: stageContractCheck, Level: LevelWarn, Message: fmt.Sprintf("%d stage key(s) at enforce have no flip record (%s)", len(res.Findings), res.Mode), Detail: detail}
	}
	return CheckResult{Name: stageContractCheck, Level: LevelPass, Message: fmt.Sprintf("every stage key at enforce has a flip record (%s)", res.Mode)}
}

func findingsDetail(findings []stageevidence.Finding) string {
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, f.Key+": "+f.Reason)
	}
	return strings.Join(lines, "\n")
}

func checkStageContract(o resolved) CheckResult { return CheckStageContract(o.stageContract) }

package gc

import (
	"context"
	"fmt"
	"strings"
)

type CheckItem struct {
	Subject  string `json:"subject"`
	Action   string `json:"action"`
	Rule     string `json:"rule"`
	Evidence string `json:"evidence"`
}

type CheckResult struct {
	Name  string      `json:"name"`
	Items []CheckItem `json:"items"`
}

type CheckReport struct {
	Checks []CheckResult `json:"checks"`
}

type CheckContext struct {
	ProjectRoot string
	EvolveDir   string
	Policy      Policy
	Worktrees   WorktreeOptions
}

type Check interface {
	Name() string
	Scan(ctx context.Context, cc CheckContext) ([]CheckItem, error)
	Apply(ctx context.Context, cc CheckContext) error
}

func DefaultChecks() []Check {
	return []Check{worktreeCheck{}, tempCheck{}, branchCheck{}}
}

func SelectChecks(only string) ([]Check, error) {
	all := DefaultChecks()
	if only == "" {
		return all, nil
	}
	names := make([]string, 0, len(all))
	for _, chk := range all {
		names = append(names, chk.Name())
		if chk.Name() == only {
			return []Check{chk}, nil
		}
	}
	return nil, fmt.Errorf("unknown --only check %q (valid: %s)", only, strings.Join(names, ", "))
}

func RunChecks(ctx context.Context, cc CheckContext, checks []Check) (CheckReport, error) {
	results := make([]CheckResult, 0, len(checks))
	for _, chk := range checks {
		items, err := chk.Scan(ctx, cc)
		if err != nil {
			return CheckReport{}, err
		}
		if items == nil {
			items = []CheckItem{}
		}
		results = append(results, CheckResult{
			Name:  chk.Name(),
			Items: items,
		})
	}
	return CheckReport{Checks: results}, nil
}

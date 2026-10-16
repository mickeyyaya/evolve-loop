package mergegate

import (
	"context"
	"fmt"
)

type Executor interface {
	Promote(ctx context.Context, integrationBranch, target string) error
	Rollback(ctx context.Context, target string) error
}

type PromoteInput struct {
	Stage             string
	Verdict           string
	IntegrationBranch string
	TargetBranch      string
	Cadence           Cadence
}

type PromoteResult struct {
	Promoted       bool
	RolledBack     bool
	RollbackFailed bool
	Recorded       bool
	Reason         string
}

type Promoter struct {
	Exec Executor
}

func (p Promoter) Promote(ctx context.Context, in PromoteInput) PromoteResult {
	switch in.Stage {
	case "shadow", "advisory":
		return PromoteResult{Recorded: true, Reason: recordReason(in)}
	case "enforce":
		return p.enforce(ctx, in)
	default:
		return PromoteResult{Reason: "merge gate off — no promotion"}
	}
}

func (p Promoter) enforce(ctx context.Context, in PromoteInput) PromoteResult {
	if in.Verdict != "PASS" {
		return PromoteResult{Reason: fmt.Sprintf("verdict %s is not PASS — promotion blocked", verdictOrUnknown(in.Verdict))}
	}
	if p.Exec == nil {
		return PromoteResult{Reason: "no executor wired — promotion blocked"}
	}
	if err := p.Exec.Promote(ctx, in.IntegrationBranch, in.TargetBranch); err != nil {
		if rbErr := p.Exec.Rollback(ctx, in.TargetBranch); rbErr != nil {
			return PromoteResult{RollbackFailed: true,
				Reason: fmt.Sprintf("promotion of %s → %s failed (%v) AND rollback failed (%v) — %s may be inconsistent, escalate",
					in.IntegrationBranch, in.TargetBranch, err, rbErr, in.TargetBranch)}
		}
		return PromoteResult{RolledBack: true,
			Reason: fmt.Sprintf("promotion of %s → %s failed, rolled back: %v", in.IntegrationBranch, in.TargetBranch, err)}
	}
	return PromoteResult{Promoted: true,
		Reason: fmt.Sprintf("promoted %s → %s (%s cadence)", in.IntegrationBranch, in.TargetBranch, in.Cadence)}
}

func recordReason(in PromoteInput) string {
	if in.Verdict == "PASS" {
		return fmt.Sprintf("%s: would promote %s → %s", in.Stage, in.IntegrationBranch, in.TargetBranch)
	}
	return fmt.Sprintf("%s: would withhold promotion (verdict %s)", in.Stage, verdictOrUnknown(in.Verdict))
}

func verdictOrUnknown(v string) string {
	if v == "" {
		return "UNKNOWN"
	}
	return v
}

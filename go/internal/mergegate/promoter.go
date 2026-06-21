package mergegate

import (
	"context"
	"fmt"
)

// Executor performs — and undoes — the actual promotion of a completed
// milestone's integration branch onto the target branch (main). It is injected
// so the Promoter's stage/verdict policy is testable without git; the production
// implementation drives swarm's acceptance-gated merge-train + the hardened ship
// path, so the integrity floor, ship.lock, attestation, and CI-green all apply
// unchanged. Promote returns a non-nil error when the merge or its post-merge
// acceptance check fails; the Promoter then calls Rollback to restore the target.
type Executor interface {
	Promote(ctx context.Context, integrationBranch, target string) error
	Rollback(ctx context.Context, target string) error
}

// PromoteInput is one promotion request: the rollout stage, the merge-gate
// phase's verdict, and the branches involved.
type PromoteInput struct {
	Stage             string  // off | shadow | advisory | enforce (config.Stage.String())
	Verdict           string  // PASS | WARN | FAIL — the merge-to-main-gate verdict
	IntegrationBranch string  // the completed milestone's integration branch
	TargetBranch      string  // promotion target, typically "main"
	Cadence           Cadence // the advisor's cadence (recorded in the reason)
}

// PromoteResult is the outcome of a promotion request.
type PromoteResult struct {
	Promoted   bool // the merge onto TargetBranch actually happened (enforce + PASS + success)
	RolledBack bool // a failed enforce promotion was cleanly rolled back
	// RollbackFailed is set when BOTH the promotion AND its rollback failed:
	// TargetBranch may be in an indeterminate state, so the caller must escalate.
	// It is never reported alongside a clean RolledBack.
	RollbackFailed bool
	Recorded       bool   // a would-be promotion was recorded (shadow/advisory observe-only)
	Reason         string // always populated, for the ledger/dossier record
}

// Promoter is the kernel actuator for the merge-to-main gate ("kernel disposes").
// It owns the stage/verdict state machine; the dangerous git work is behind Exec.
type Promoter struct {
	Exec Executor
}

// Promote applies the rollout-stage policy to a verdict. off/unknown does nothing;
// shadow/advisory record the would-be promotion without acting; enforce promotes
// only on a PASS verdict and auto-rolls-back a failed promotion. The LLM gate's
// verdict can only ever WITHHOLD promotion here — it cannot force an unsafe merge,
// because the actual merge is acceptance-gated inside Exec.
func (p Promoter) Promote(ctx context.Context, in PromoteInput) PromoteResult {
	switch in.Stage {
	case "shadow", "advisory":
		return PromoteResult{Recorded: true, Reason: recordReason(in)}
	case "enforce":
		return p.enforce(ctx, in)
	default: // "off" or any unknown value → fail-safe no-op (gate dormant).
		return PromoteResult{Reason: "merge gate off — no promotion"}
	}
}

// enforce executes a promotion when (and only when) the verdict is PASS, auto-
// rolling-back a promotion whose merge/acceptance fails.
func (p Promoter) enforce(ctx context.Context, in PromoteInput) PromoteResult {
	if in.Verdict != "PASS" {
		return PromoteResult{Reason: fmt.Sprintf("verdict %s is not PASS — promotion blocked", verdictOrUnknown(in.Verdict))}
	}
	if p.Exec == nil {
		return PromoteResult{Reason: "no executor wired — promotion blocked"}
	}
	if err := p.Exec.Promote(ctx, in.IntegrationBranch, in.TargetBranch); err != nil {
		// A failed promotion MUST be undone. If the rollback ALSO fails, the target
		// branch may be in an indeterminate state — surface that loudly (never claim
		// a clean RolledBack) so the caller escalates instead of trusting main.
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

// recordReason describes the observe-only decision a shadow/advisory run logs.
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

package lifecycle

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
	var (
		_ func(string, LedgerAppender, ...Option) *Mover                     = New
		_ func(io.Writer) Option                                             = WithStderr
		_ func(func() time.Time) Option                                      = WithNow
		_ func(func() (string, error)) Option                                = WithActiveCycle
		_ func(func(string) (bool, error)) Option                            = WithLanded
		_ func(func(string) bool) Option                                     = WithProtectedPath
		_ func(func(string, string, string)) Option                          = WithRetire
		_ func(func(int) string) Option                                      = WithRunWorkspace
		_ func(func() *signalcenter.Center) Option                           = WithSignals
		_ func([]string) Option                                              = WithPriorityClasses
		_ func(*Mover) bool                                                  = (*Mover).SignalsWired
		_ func(*Mover, string, string) (ClaimResult, error)                  = (*Mover).Claim
		_ func(*Mover, string, string, PromoteOpts) (PromoteResult, error)   = (*Mover).Promote
		_ func(*Mover, string, string, PromoteOpts, string) (string, error)  = (*Mover).RetireUnbacked
		_ func(*Mover, string) (PromoteResult, error)                        = (*Mover).ReleaseFromQuarantine
		_ func(*Mover) (RecoverResult, error)                                = (*Mover).RecoverOrphans
		_ func(*Mover, int, string, *Policy) (RecoverResult, error)          = (*Mover).Release
		_ func(*Mover, string, string, int) (RouteResult, error)             = (*Mover).RouteConsole
		_ func(*Mover, string, string) (RouteResult, error)                  = (*Mover).RouteLane
		_ func(*Mover, RouteRequest) (RouteResult, error)                    = (*Mover).RouteConsoleRequest
		_ func(*Mover, RouteRequest) (RouteResult, error)                    = (*Mover).RouteLaneRequest
		_ func(*Mover, []byte) (FileResult, error)                           = (*Mover).File
		_ error                                                              = ErrInvalidItem
		_ string                                                             = RouteConsoleValue
		_ func(*Mover, string) (int, bool)                                   = (*Mover).ReadFailureCount
		_ func(*Mover, string) string                                        = (*Mover).ReadLastFailureClass
		_ func(*Mover, string) string                                        = (*Mover).ReadLastFailureRung
		_ func(int, int) Round                                               = PlanRound
		_ func(string, string) (Location, error)                             = Locate
		_ func(string, string) (string, error)                               = FindFileByTaskID
		_ func(int, int, bool) bool                                          = ShouldQuarantine
		_ func(string, string) (int, error)                                  = BumpFailureCount
		_ func(string, func(map[string]json.RawMessage)) error               = UpdateItemJSON
		_ func(func() (string, error)) Option                                = WithMainHead
		_ func(*Mover, string, []FieldEdit) (string, error)                  = (*Mover).Edit
		_ func(*Mover, string, string) (string, error)                       = (*Mover).Withdraw
		_ func(*Mover, string, string) (string, error)                       = (*Mover).VerifyPremise
		_ error                                                              = ErrNotWithdrawable
		_ func(func(string) (bool, error)) Option                            = WithBinding
		_ func(string) bool                                                  = IsMoverWritten
		_ string                                                             = RouteField
		_ func(string) (ClaimList, error)                                    = ListClaims
		_ func(*Mover, string, Location, string) (ClaimReleaseResult, error) = (*Mover).ReleaseClaim
		_ error                                                              = ErrClaimConflict
		_                                                                    = ClaimedItem{Duplicate: true}
		_ func(*Mover) ([]Absorbed, error)                                   = (*Mover).AbsorbRootCopies
		_ AbsorbOutcome                                                      = AbsorbParked
		_ EditOp                                                             = EditSet
	)
	if EditSet != "set" || EditAdd != "add" || EditRemove != "remove" {
		t.Errorf("the edit ops are spelled as the CLI flags: %q %q %q", EditSet, EditAdd, EditRemove)
	}
	var appender LedgerAppender = &recordingAppender{}
	if err := appender.AppendLifecycle(context.Background(), ledger.LifecycleRecord{}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []error{ErrNotFound, ErrMvFailed, ErrBadArgs, ErrBadState, ErrConsoleRouted} {
		if e == nil {
			t.Error("a sentinel is nil")
		}
	}
	for _, c := range []signalcenter.Code{CodeClaimNotFound, CodeClaimRefused, CodeClaimMoveFailed, CodePromoteNotFound, CodePromoteUnlandedSHA,
		CodePromoteMoveFailed, CodeLandedCheckFailed, CodeReleaseDoubleMove, CodeReleaseMoveFailed, CodeQuarantineFailed,
		CodeContinuationManifestUnreadable, CodeItemRewriteFailed, CodeItemRoutedConsole, CodeRouteNotFound} {
		if !c.BelongsTo(signalcenter.ModuleInbox) {
			t.Errorf("%s is not an inbox code", c)
		}
	}
	if LegacyPrefix != "[inbox-mover] " {
		t.Errorf("LegacyPrefix = %q", LegacyPrefix)
	}
	if first := PlanRound(0, 3); first.Try != 1 || first.CarryFindings || first.RaiseEffort || first.ChangeStrategy || first.Plan() != "raise-with-findings" {
		t.Errorf("PlanRound(0, 3) = %+v, want a first plain try", first)
	}
	if last := PlanRound(2, 3); last.Try != 3 || !last.CarryFindings || !last.RaiseEffort || !last.ChangeStrategy || last.Plan() != "fresh-deep-plan" {
		t.Errorf("PlanRound(2, 3) = %+v, want a third try that changes strategy", last)
	}
	if !ShouldQuarantine(2, 2, false) || ShouldQuarantine(1, 2, false) || ShouldQuarantine(2, 2, true) || ShouldQuarantine(5, 0, false) {
		t.Error("ShouldQuarantine must trip only at the ceiling for a non-system failure")
	}
}

package lifecycle

// apicover_named_test.go — every export of the leaf named in a package-local
// test (the apicover gate reads names from _test.go files and executed
// coverage from the package's own tests): the constructor, the eight Options,
// SignalsWired, the six Mover methods, the value objects, the sentinels, the
// twelve codes, the console prefix and the four exported primitives.

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
		_ func(string, LedgerAppender, ...Option) *Mover                   = New
		_ func(io.Writer) Option                                           = WithStderr
		_ func(func() time.Time) Option                                    = WithNow
		_ func(func() (string, error)) Option                              = WithActiveCycle
		_ func(func(string) (bool, error)) Option                          = WithLanded
		_ func(func(string) bool) Option                                   = WithProtectedPath
		_ func(func(string, string, string)) Option                        = WithRetire
		_ func(func(int) string) Option                                    = WithRunWorkspace
		_ func(func() *signalcenter.Center) Option                         = WithSignals
		_ func(*Mover) bool                                                = (*Mover).SignalsWired
		_ func(*Mover, string, string) (ClaimResult, error)                = (*Mover).Claim
		_ func(*Mover, string, string, PromoteOpts) (PromoteResult, error) = (*Mover).Promote
		_ func(*Mover, string) (PromoteResult, error)                      = (*Mover).ReleaseFromQuarantine
		_ func(*Mover) (RecoverResult, error)                              = (*Mover).RecoverOrphans
		_ func(*Mover, int, string, *Policy) (RecoverResult, error)        = (*Mover).Release
		_ func(*Mover, string) (int, bool)                                 = (*Mover).ReadFailureCount
		_ func(string, string) (Location, error)                           = Locate
		_ func(string, string) (string, error)                             = FindFileByTaskID
		_ func(int, int, bool) bool                                        = ShouldQuarantine
		_ func(string, string) (int, error)                                = BumpFailureCount
		_ func(string, func(map[string]json.RawMessage)) error             = UpdateItemJSON
	)
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
		CodeContinuationManifestUnreadable, CodeItemRewriteFailed} {
		if !c.BelongsTo(signalcenter.ModuleInbox) {
			t.Errorf("%s is not an inbox code", c)
		}
	}
	if LegacyPrefix != "[inbox-mover] " {
		t.Errorf("LegacyPrefix = %q", LegacyPrefix)
	}
	p := Policy{Ceiling: 2, SystemLevel: false, Committed: map[string]bool{"a": true}}
	loc := Location{Path: "p", Cycle: 1}
	cr, pr, rr, po := ClaimResult{SrcPath: "s", DestPath: "d"}, PromoteResult{NoOp: true}, RecoverResult{Recovered: 1, Paths: []string{"x"}}, PromoteOpts{Cycle: "1", CommitSHA: "s"}
	if p.Ceiling != 2 || loc.Cycle != 1 || cr.SrcPath != "s" || !pr.NoOp || rr.Recovered != 1 || po.Cycle != "1" {
		t.Error("the value objects carry their fields")
	}
}

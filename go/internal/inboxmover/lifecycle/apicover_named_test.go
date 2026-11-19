package lifecycle

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
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
		CodeContinuationManifestUnreadable, CodeItemRewriteFailed, CodeItemRoutedConsole, CodeItemFiledConsole, CodeRouteNotFound} {
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

func TestAPI_ResultTypesCarryTheMove(t *testing.T) {
	inbox := newInbox(t)
	m := New(inbox, nil)
	writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)

	var claimed ClaimResult
	claimed, err := m.Claim("t1", "7")
	if err != nil || claimed.SrcPath != filepath.Join(inbox, "t1.json") || claimed.DestPath != procPath(inbox, 7, "t1.json") {
		t.Fatalf("Claim = %+v, %v", claimed, err)
	}

	var promoted PromoteResult
	promoted, err = m.Promote("t1", "processed", PromoteOpts{Cycle: "7", CommitSHA: "deadbeef00"})
	if err != nil || promoted.NoOp || promoted.SrcPath != claimed.DestPath || !strings.Contains(promoted.DestPath, "processed/cycle-7/deadbeef-t1.json") {
		t.Fatalf("Promote = %+v, %v", promoted, err)
	}

	writeItem(t, procPath(inbox, 8, "orphan.json"), `{"id":"orphan"}`)
	var recovered RecoverResult
	recovered, err = m.RecoverOrphans()
	if err != nil || recovered.Recovered != 1 || len(recovered.Paths) != 1 {
		t.Fatalf("RecoverOrphans = %+v, %v", recovered, err)
	}

	var filed FileResult
	filed, err = newFiler(inbox, nil).File([]byte(validItem))
	if err != nil || filed.Path == "" || filed.ConsoleReason != "" {
		t.Fatalf("File = %+v, %v", filed, err)
	}

	var op EditOp = EditSet
	if _, err := m.Edit("orphan", []FieldEdit{{Op: op, Field: "weight", Value: "0.3"}}); err != nil {
		t.Fatalf("Edit with %q: %v", op, err)
	}
}

func TestAPI_AbsorbOutcomeNamesWhatBecameOfTheRootCopy(t *testing.T) {
	inbox, m, _ := claimFixture(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"a","differs":true}`)

	got, err := m.AbsorbRootCopies()
	if err != nil || len(got) != 2 {
		t.Fatalf("AbsorbRootCopies = %+v, %v", got, err)
	}
	outcomes := map[string]AbsorbOutcome{}
	for _, a := range got {
		outcomes[a.ID] = a.Outcome
	}
	if outcomes["a"] != AbsorbParked || outcomes["dup"] != AbsorbDropped {
		t.Errorf("outcomes = %v, want a parked and dup dropped", outcomes)
	}
}

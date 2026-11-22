package core

import "testing"

func TestNamesMovedMain_OnlyTheHeadMovedAndRebaseNeededCodesNameAMovedMain(t *testing.T) {
	t.Parallel()
	for code, want := range map[ShipErrorCode]bool{
		CodeAuditBindingHeadMoved: true,
		CodeGitFleetRebaseNeeded:  true,
		CodeGitPushRejected:       false,
	} {
		if got := NamesMovedMain(code); got != want {
			t.Errorf("NamesMovedMain(%s) = %v, want %v", code, got, want)
		}
	}
}

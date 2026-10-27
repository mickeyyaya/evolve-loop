package shiperr

import "testing"

func TestCodeHeldForOperatorAndCodeHeldProposalRejected_Vocab(t *testing.T) {
	for code, want := range map[ShipErrorCode]string{CodeHeldForOperator: "HELD_FOR_OPERATOR", CodeHeldProposalRejected: "HELD_PROPOSAL_REJECTED"} {
		if string(code) != want || string(SignalCode(code)) != "SHIP_"+want {
			t.Errorf("code %q projects to %q, want %q and SHIP_%s", code, SignalCode(code), want, want)
		}
	}
}

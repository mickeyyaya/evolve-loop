package shiperr

import "testing"

func TestCodeRepoContractCapacity_Vocab(t *testing.T) {
	if got, want := string(CodeRepoContractCapacity), "REPO_CONTRACT_CAPACITY"; got != want {
		t.Errorf("CodeRepoContractCapacity = %q, want %q", got, want)
	}
	for _, other := range []ShipErrorCode{CodeRepoContractInfra, CodeRepoContractGate, CodeMainRedPark, CodeUnknown} {
		if CodeRepoContractCapacity == other {
			t.Errorf("CodeRepoContractCapacity must not alias %q", other)
		}
	}
	if doc := codeDocs[CodeRepoContractCapacity]; doc == "" {
		t.Error("CodeRepoContractCapacity has no signal documentation")
	}
	if SignalCode(CodeRepoContractCapacity) != "SHIP_REPO_CONTRACT_CAPACITY" {
		t.Errorf("signal code = %q", SignalCode(CodeRepoContractCapacity))
	}
}

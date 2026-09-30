//go:build acs

package cycle308

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func TestCycle308_InboxReleaseAPIExists(t *testing.T) {
	res, err := inboxmover.ReleaseCycleProcessing(inboxmover.Options{}, 0)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing: unexpected error on absent dir: %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("absent dir: Recovered = %d, want 0", res.Recovered)
	}
}

func TestCycle308_MalformedFloorWarningsExist(t *testing.T) {
	if w := triagecap.MalformedCommittedFloorWarning("/no/such/file"); w != "" {
		t.Errorf("MalformedCommittedFloorWarning(/no/such): want \"\", got %q", w)
	}
	if w := triagecap.MalformedDeferredFloorWarning("/no/such/file"); w != "" {
		t.Errorf("MalformedDeferredFloorWarning(/no/such): want \"\", got %q", w)
	}
}

func TestCycle308_CLIVersionsFieldExists(t *testing.T) {
	var r looppreflight.Result
	if r.CLIVersions == nil {
		r.CLIVersions = map[string]string{"test": "1.0.0"}
	}
	if r.CLIVersions["test"] != "1.0.0" {
		t.Errorf("CLIVersions roundtrip: got %q, want 1.0.0", r.CLIVersions["test"])
	}
}

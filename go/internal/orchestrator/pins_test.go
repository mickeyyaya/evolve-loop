package orchestrator

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

func TestCopiedCodes_EqualTheConstantsOfTheirOwnersAndTheOwnersRegisterThem(t *testing.T) {
	for name, tc := range map[string]struct{ local, owner signalcenter.Code }{
		"bridge.CodePhaseStalled":         {CodePhaseStalled, bridge.CodePhaseStalled},
		"bridge.CodePaneExhausted":        {CodePaneExhausted, bridge.CodePaneExhausted},
		"usageevidence.CodeUsageEvidence": {CodeUsageEvidence, usageevidence.CodeUsageEvidence},
	} {
		if tc.local != tc.owner {
			t.Errorf("%s = %q but the orchestrator matches %q: the copy must follow its owner (the import ban forces a copy)", name, tc.owner, tc.local)
		}
		if _, registered := signalcenter.IsRegistered(tc.local); !registered {
			t.Errorf("%s is not registered by its owner: the oversight would match a code that no producer emits", tc.local)
		}
	}
}

package orchestrator

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestSupply_ALaneItemWithAPendingConsoleCompanionWaitsAtEnforceAndWarnsAtShadow(t *testing.T) {
	t.Parallel()
	for stage, want := range map[string]struct {
		admissible, waiting int
		warning             string
	}{
		policy.FixabilityShadow:  {1, 0, "would defer x-lane: console_companion_pending: x-lane-ac-1-console"},
		policy.FixabilityEnforce: {0, 1, ""},
	} {
		root := t.TempDir()
		inbox := filepath.Join(root, ".evolve", "inbox")
		writeItem(t, inbox, "x-lane", map[string]any{"route": "lane"})
		writeItem(t, inbox, "x-lane-ac-1-console", map[string]any{"route": "console-manual"})
		census := newCensus(t, root, nil, &eventLog{})
		census.Graph, census.Inbox.Companions = nil, policy.FixabilityConfig{Stage: stage}

		got, err := census.Count()

		hasWarning := want.warning == "" || slices.Contains(got.Warnings, want.warning)
		if err != nil || got.Counts[ClassAdmissible] != want.admissible || got.Counts[ClassWaitingDeps] != want.waiting || !hasWarning {
			t.Errorf("stage %s: Count = %+v, %v; want %d admissible, %d waiting and the warning %q", stage, got, err, want.admissible, want.waiting, want.warning)
		}
	}
}

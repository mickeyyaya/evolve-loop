package inboxbatch

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
)

func TestRepairTier_DerivesT0FromTheProtectedFilesAndReadsTheStampOtherwise(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		item Item
		want gateclass.Tier
	}{
		"protected files bind T0 over a T1 stamp": {itemWithTier("T1", []string{"go/internal/guards/role.go"}), gateclass.T0},
		"a T1 stamp without protected files":      {itemWithTier("T1", nil), gateclass.T1},
		"a T2 stamp without protected files":      {itemWithTier("T2", nil), gateclass.T2},
		"no stamp and no protected files":         {Item{}, ""},
		"a T0 stamp alone is not trusted":         {itemWithTier("T0", nil), ""},
		"a mis-cased stamp is not a tier":         {itemWithTier("t1", nil), ""},
	}
	for name, c := range cases {
		if got := c.item.RepairTier(); got != c.want {
			t.Errorf("%s: RepairTier = %q, want %q", name, got, c.want)
		}
	}
}

func TestRoleOf_TierIsStampedByTheLoopNeverAuthored(t *testing.T) {
	t.Parallel()
	if got := RoleOf(TierField); got.Owner != LoopStampOwned || !got.IsStamp() {
		t.Errorf("RoleOf(%q) = %+v, want a loop stamp", TierField, got)
	}
}

func TestItemJSON_TierRoundTripsIntoRepairTier(t *testing.T) {
	t.Parallel()
	var it Item
	if err := json.Unmarshal([]byte(`{"id":"x","tier":"T2"}`), &it); err != nil || it.RepairTier() != gateclass.T2 {
		t.Fatalf("tier read as %q, %v", it.RepairTier(), err)
	}
}

func TestConsoleRouted_ALaneOptOutOfAPipelineRepairStaysLaneRoutable(t *testing.T) {
	t.Parallel()
	raw := `{"id":"core-tests-cut-git-starts","kind":"pipeline-repair","route":"lane","files":["go/internal/core/orchestrator.go"],"title":"t","summary":"s"}`
	var it Item
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatal(err)
	}
	if routed, why := ConsoleRouted(it, protectedStub("go/internal/guards/role.go")); routed {
		t.Errorf("a pipeline-repair item that opts out with route lane and declares only unprotected files must stay lane-routable, got %q", why)
	}
	laneRoutable, console, _ := PartitionConsole([]Item{it}, nil)
	if len(laneRoutable) != 1 || len(console) != 0 {
		t.Errorf("PartitionConsole: lane %d console %d", len(laneRoutable), len(console))
	}
}

func itemWithTier(tier string, protected []string) Item {
	raw, _ := json.Marshal(tier)
	return Item{Tier: raw, ProtectedFiles: protected}
}

func TestRepairTierOf_ReadsAStampAndTheProtectedFiles(t *testing.T) {
	t.Parallel()
	if got := RepairTierOf("T2", nil); got != gateclass.T2 {
		t.Errorf("T2 stamp = %q", got)
	}
	if got := RepairTierOf("T1", []string{"a.go"}); got != gateclass.T0 {
		t.Errorf("protected files = %q", got)
	}
}

func TestRepairTier_AnObjectValuedTierKeyIsNoStamp(t *testing.T) {
	t.Parallel()
	var it Item
	if err := json.Unmarshal([]byte(`{"id":"s","tier":{"note":"x"}}`), &it); err != nil || it.RepairTier() != "" {
		t.Fatalf("tier %q err %v", it.RepairTier(), err)
	}
}

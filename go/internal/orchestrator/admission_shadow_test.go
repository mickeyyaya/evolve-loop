package orchestrator

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func coreGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	m := admission.ModulePath + "/internal/"
	g, err := admission.NewModuleGraph(m+"router|\n"+m+"core|"+m+"router\n"+admission.ModulePath+"/cmd/evolve|"+m+"core\n", 1000)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestReachShadow_RecordsEachHeldPairOnceWithItsHolder(t *testing.T) {
	events := &eventLog{}
	admit := AdmitByCell(admission.Admitter{Graph: coreGraph(t), Shadow: ReachShadow(events.publish)})
	inFlight := []fleet.Todo{todoIn("lane-core", "core")}
	pending := []fleet.Todo{todoIn("r", "router")}
	for range 3 {
		if got := admit(inFlight, pending); got != -1 {
			t.Fatalf("admit = %d, want -1: at shadow the reach rule still holds r", got)
		}
	}
	got := events.withCode(CodeAdmissionReachShadow)
	if len(got) != 1 {
		t.Fatalf("shadow events = %+v, want one for the pair r behind lane-core", got)
	}
	e := got[0]
	if e.Severity != signalcenter.SeverityInfo || e.Fields["todo"] != "r" || e.Fields["holder"] != "lane-core" || e.Fields["reason"] != "reach meets internal/router" {
		t.Errorf("shadow event = %+v, want INFO with todo r, holder lane-core and the met package", e)
	}
}

func TestAdmitByCell_TheSharedEditRuleAdmitsAnImportOfARunningLane(t *testing.T) {
	events := &eventLog{}
	admit := AdmitByCell(admission.Admitter{Graph: coreGraph(t), OnlySharedEdits: true, Shadow: ReachShadow(events.publish)})
	if got := admit([]fleet.Todo{todoIn("lane-core", "core")}, []fleet.Todo{todoIn("r", "router")}); got != 0 {
		t.Fatalf("admit = %d, want 0: r edits no package that lane-core edits", got)
	}
	if got := events.withCode(CodeAdmissionReachShadow); len(got) != 0 {
		t.Errorf("shadow events = %+v, want none at enforce", got)
	}
}

func TestCensus_CountsByTheAdmissionRuleOfTheStage(t *testing.T) {
	for stage, want := range map[bool]map[string]int{false: {ClassAdmissible: 0, ClassWaitingCell: 1}, true: {ClassAdmissible: 1, ClassWaitingCell: 0}} {
		root := t.TempDir()
		inbox := filepath.Join(root, ".evolve", "inbox")
		writeItem(t, inbox, "r", map[string]any{"files": fileIn("router")})
		c := Census{
			Inbox:           inboxmover.Options{InboxDir: inbox, Stderr: io.Discard},
			Rank:            inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: epoch},
			Protected:       guardsProtected,
			Graph:           coreGraph(t),
			InFlight:        []admission.Holding{{Lane: "lane-core", Todo: todoIn("c", "core")}},
			OnlySharedEdits: stage,
			Now:             func() time.Time { return epoch },
		}
		got, err := c.Count()
		if err != nil {
			t.Fatal(err)
		}
		if got.Counts[ClassAdmissible] != want[ClassAdmissible] || got.Counts[ClassWaitingCell] != want[ClassWaitingCell] {
			t.Errorf("only shared edits %v: counts = %v, want %v", stage, got.Counts, want)
		}
	}
}

func TestReachShadow_ThePairIsTheKeyNotTheReason(t *testing.T) {
	events := &eventLog{}
	record := ReachShadow(events.publish)
	record(admission.Shadowed{Todo: "r", Holder: "lane-core", Reason: "reach meets internal/router"})
	record(admission.Shadowed{Todo: "r", Holder: "lane-core", Reason: "reach meets internal/clihealth"})
	record(admission.Shadowed{Todo: "r", Holder: "lane-other", Reason: "reach meets internal/router"})
	if got := events.withCode(CodeAdmissionReachShadow); len(got) != 2 {
		t.Fatalf("shadow events = %+v, want 2: one for each todo and holder", got)
	}
}

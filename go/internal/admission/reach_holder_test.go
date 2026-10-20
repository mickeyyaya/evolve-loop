package admission_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func reachHolderAdmitter(t *testing.T, optimistic bool) (admission.Admitter, *[]admission.ShadowAdmission) {
	t.Helper()
	records := &[]admission.ShadowAdmission{}
	a := admitter(t, nil)
	a.Optimistic = optimistic
	a.OnOptimistic = func(s admission.ShadowAdmission) { *records = append(*records, s) }
	return a, records
}

func TestAdmit_AnOptimisticAdmissionNamesTheHolderTheReachRuleWouldHaveUsed(t *testing.T) {
	a, records := reachHolderAdmitter(t, true)
	running := []admission.Holding{{Lane: "lane-1", Todo: todo("f", "go/internal/fleet/pool.go")}}

	admitted, _ := a.Admit(running, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go"), todo("q", "go/internal/quotastate/windows.go")}, 32)

	if got := ids(admitted); !reflect.DeepEqual(got, []string{"i", "q"}) {
		t.Fatalf("admitted = %v, want [i q]", got)
	}
	if len(*records) != 1 {
		t.Fatalf("records = %+v, want one: only i is admitted by the optimistic rule alone", *records)
	}
	got := (*records)[0]
	if got.ID != "i" || got.HeldBy != "lane-1" || got.Rank != 0 || !strings.Contains(got.Reason, "inboxrank") {
		t.Errorf("record = %+v, want i held by lane-1 at rank 0 with the reach package named", got)
	}
}

func TestAdmit_TheReachRuleNamesNoHolderForItsOwnAdmissions(t *testing.T) {
	a, records := reachHolderAdmitter(t, false)
	running := []admission.Holding{{Lane: "lane-1", Todo: todo("f", "go/internal/fleet/pool.go")}}

	a.Admit(running, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go"), todo("q", "go/internal/quotastate/windows.go")}, 32)

	if len(*records) != 0 {
		t.Errorf("records = %+v, want none: the reach rule holds i and admits q without a holder", *records)
	}
}

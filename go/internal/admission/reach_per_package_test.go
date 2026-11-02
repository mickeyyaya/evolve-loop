package admission_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestAdmit_AMixedEditSetTakesReachOnlyFromItsNonLeafCodePackages(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("w149",
		"go/cmd/evolve/policy_readers_strict_test.go", "go/internal/policy/workflow.go", "go/internal/brandnew/brandnew.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("f", "go/internal/fleet/pool.go")}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"f"}) {
		t.Fatalf("admitted = %v, waiting = %+v: the leaf cmd/evolve and a package outside the graph add no reach, and policy reaches only log, so fleet, which cmd/evolve imports, is free (wave 149 ran one lane of four)", got, waiting)
	}
}

func TestAdmit_ATestOnlyEditOfAPackageHoldsNoReachOverItsDeps(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("ft", "go/internal/fleet/fleet_test.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"i"}) {
		t.Fatalf("admitted = %v, waiting = %+v: a lane that edits only tests of fleet changes no code that inboxrank callers see", got, waiting)
	}
}

func TestAdmit_ACodeEditOfAPackageStillHoldsReachOverItsDeps(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("fc", "go/internal/fleet/fleet.go", "go/internal/fleet/fleet_test.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if len(admitted) != 0 || len(waiting) != 1 {
		t.Fatalf("admitted = %v, waiting = %+v: fleet imports inboxrank, so a code edit of fleet still holds inboxrank", ids(admitted), waiting)
	}
}

func TestAdmit_ALeafCodeEditBesideANonLeafEditAddsNoReachOfItsOwn(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("lp", "go/cmd/evolve/main.go", "go/internal/policy/workflow.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("f", "go/internal/fleet/pool.go")}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"f"}) {
		t.Fatalf("admitted = %v, waiting = %+v: cmd/evolve is a leaf, so its code edit adds no reach even beside the non-leaf policy", got, waiting)
	}
}

func TestAdmit_ATodoNamedOnlyByItsTextStillHoldsTheReachOfItsPackages(t *testing.T) {
	t.Parallel()
	a := admitter(t, map[string]string{"tf": "rework go/internal/fleet/pool.go"})
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("tf", "tf")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if len(admitted) != 0 || len(waiting) != 1 {
		t.Fatalf("admitted = %v, waiting = %+v: a todo whose file list is its id edits the packages its text names, and fleet reaches inboxrank", ids(admitted), waiting)
	}
}

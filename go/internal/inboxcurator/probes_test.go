package inboxcurator

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	existingTest = "TestPool_AFreeSlotTakesTheNextTodo"
	livePath     = "go/internal/fleet/pool.go"
	removedPath  = "go/internal/fleet/legacy.go"
)

func treeOf(tests []string, paths ...string) Tree {
	return Tree{HasTest: testsIn(tests...), PathExists: func(p string) bool { return slices.Contains(paths, p) }}
}

func probe(kind, arg string) inboxbatch.Probe { return inboxbatch.Probe{Kind: kind, Arg: arg} }

func probedItem(id string, files []string, probes ...inboxbatch.Probe) inboxbatch.Item {
	it := inboxbatch.Item{ID: id, Kind: "feature", Origin: inboxbatch.OriginConsole, Files: files, Title: id, Probes: probes}
	it.Path = "2026-11-01T00-00-00Z-" + id + ".json"
	return it
}

func probeRules(tree Tree) Rules {
	return Rules{Kinds: []string{"feature"}, Thresholds: Thresholds{Refuse: 0.6, Warn: 0.2}, Tree: tree}
}

func TestAdmit_RefusesAnItemWhoseProbesAlreadyPass(t *testing.T) {
	tree := treeOf([]string{existingTest}, livePath)
	for name, probes := range map[string][]inboxbatch.Probe{
		"a test that exists":                 {probe(inboxbatch.ProbeTestExists, existingTest)},
		"a path that exists":                 {probe(inboxbatch.ProbePathExists, livePath)},
		"a path that is already gone":        {probe(inboxbatch.ProbePathAbsent, removedPath)},
		"every probe of a mixed list passes": {probe(inboxbatch.ProbeTestExists, existingTest), probe(inboxbatch.ProbePathAbsent, removedPath), probe(inboxbatch.ProbePathExists, livePath)},
	} {
		got, err := Admit(probedItem("done", []string{livePath}, probes...), nil, probeRules(tree), Override{Reason: "file it anyway", AllowOverlap: true})

		if !errors.Is(err, ErrIntake) || !errors.Is(err, ErrProbesPass) {
			t.Errorf("%s: Admit = %v, want ErrProbesPass inside ErrIntake, and no override lifts it", name, err)
			continue
		}
		for _, p := range probes {
			if !strings.Contains(err.Error(), p.Arg) {
				t.Errorf("%s: the refusal %q does not name the passing probe %q", name, err, p.Arg)
			}
		}
		if got.Record != nil {
			t.Errorf("%s: a refused item carries the record %+v", name, got.Record)
		}
	}
}

func TestAdmit_AdmitsAnItemWithOneProbeThatStillFails(t *testing.T) {
	tree := treeOf([]string{existingTest}, livePath)
	for name, probes := range map[string][]inboxbatch.Probe{
		"a test that is missing":         {probe(inboxbatch.ProbeTestExists, "TestNotWrittenYet")},
		"a path that is not there":       {probe(inboxbatch.ProbePathExists, "go/internal/fleet/new.go")},
		"a path that still exists":       {probe(inboxbatch.ProbePathAbsent, livePath)},
		"one failing probe among passes": {probe(inboxbatch.ProbeTestExists, existingTest), probe(inboxbatch.ProbeTestExists, "TestNotWrittenYet")},
	} {
		got, err := Admit(probedItem("todo", []string{livePath}, probes...), nil, probeRules(tree), Override{})

		if err != nil || got.Record != nil {
			t.Errorf("%s: Admit = %+v, %v; want the item admitted with no record", name, got, err)
		}
	}
}

func TestAdmit_AnItemWithNoProbesNeverAsksTheTree(t *testing.T) {
	asked := false
	tree := Tree{
		HasTest:    func(string) bool { asked = true; return true },
		PathExists: func(string) bool { asked = true; return true },
	}

	_, err := Admit(probedItem("plain", []string{livePath}), nil, probeRules(tree), Override{})

	if err != nil || asked {
		t.Errorf("Admit = %v, tree asked = %v; want admitted without asking the tree", err, asked)
	}
}

func TestAdmit_TheProbeRefusalComesBeforeTheOverlapJudgement(t *testing.T) {
	tree := treeOf(nil, livePath)
	done := probedItem("done", []string{livePath}, probe(inboxbatch.ProbePathExists, livePath))
	done.Title = "a free lane takes the next task"
	twin := inboxbatch.Item{ID: "twin", Files: []string{livePath}, Title: "a free lane takes the next task"}

	_, err := Admit(done, []inboxbatch.Item{twin}, probeRules(tree), Override{})

	var overlap *OverlapError
	if !errors.Is(err, ErrProbesPass) || errors.As(err, &overlap) {
		t.Errorf("Admit = %v, want the probe refusal and not the overlap refusal", err)
	}
}

func TestCheckIntake_RefusesAProbeOfAnUnknownKindOrNoArgumentOrAPathOutsideTheRepository(t *testing.T) {
	for name, tc := range map[string]struct {
		probe inboxbatch.Probe
		why   string
	}{
		"an unknown kind":             {probe("grep-matches", "x"), `"grep-matches"`},
		"no kind":                     {probe("", "x"), "probe"},
		"no argument":                 {probe(inboxbatch.ProbeTestExists, "  "), "argument"},
		"an absolute path":            {probe(inboxbatch.ProbePathExists, "/etc/passwd"), "/etc/passwd"},
		"a path that leaves the repo": {probe(inboxbatch.ProbePathAbsent, "go/../../etc/passwd"), "go/../../etc/passwd"},
	} {
		it := probedItem("p", []string{livePath}, tc.probe)

		err := CheckIntake(it, []string{"feature"})

		if !errors.Is(err, ErrIntake) || !strings.Contains(err.Error(), tc.why) {
			t.Errorf("%s: CheckIntake = %v, want ErrIntake naming %q", name, err, tc.why)
		}
	}
	valid := probedItem("p", []string{livePath}, probe(inboxbatch.ProbeTestExists, existingTest), probe(inboxbatch.ProbePathExists, livePath))
	if err := CheckIntake(valid, []string{"feature"}); err != nil {
		t.Errorf("CheckIntake of well-formed probes = %v, want nil", err)
	}
}

func TestTreeProbesPass_NeedsAtLeastOneProbeAndEveryProbeTrue(t *testing.T) {
	tree := treeOf([]string{existingTest}, livePath)
	for name, tc := range map[string]struct {
		probes []inboxbatch.Probe
		want   bool
	}{
		"no probes":                    {nil, false},
		"one passing probe":            {[]inboxbatch.Probe{probe(inboxbatch.ProbeTestExists, existingTest)}, true},
		"one failing probe":            {[]inboxbatch.Probe{probe(inboxbatch.ProbeTestExists, "TestMissing")}, false},
		"a pass and a fail":            {[]inboxbatch.Probe{probe(inboxbatch.ProbePathExists, livePath), probe(inboxbatch.ProbePathExists, removedPath)}, false},
		"an unknown kind never passes": {[]inboxbatch.Probe{probe("grep-matches", livePath)}, false},
	} {
		if got := tree.ProbesPass(probedItem("p", nil, tc.probes...)); got != tc.want {
			t.Errorf("%s: ProbesPass = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSweep_EveryDeclaredPathGoneIsAQuarantine(t *testing.T) {
	gone := probedItem("gone", []string{removedPath, "go/internal/fleet/legacy_test.go"}, probe(inboxbatch.ProbeTestExists, "TestNotWrittenYet"))
	half := probedItem("half", []string{removedPath, livePath}, probe(inboxbatch.ProbeTestExists, "TestNotWrittenYet"))
	legacy := probedItem("legacy", []string{removedPath})
	tree := treeOf(nil, livePath)

	got := Sweep([]inboxbatch.Item{gone, half, legacy}, tree, 0.5)

	if len(got) != 1 {
		t.Fatalf("Sweep = %+v, want one proposal: a probed item with every declared path gone", got)
	}
	p := got[0]
	if p.Kind != ProposeQuarantine || p.ID != "gone" || p.Path != gone.Path || p.Trigger != TriggerSweep || p.Deterministic {
		t.Errorf("proposal = %+v, want a judgment quarantine of gone by the sweep", p)
	}
	for _, path := range []string{removedPath, "go/internal/fleet/legacy_test.go"} {
		if !slices.ContainsFunc(p.Evidence, func(line string) bool { return strings.Contains(line, path) }) {
			t.Errorf("Evidence = %v, want it to name the gone path %s", p.Evidence, path)
		}
	}
}

func TestSweep_EveryProbePassingIsADeterministicConsumeThatReplacesTheTestShareAndTheQuarantine(t *testing.T) {
	done := probedItem("done", []string{removedPath}, probe(inboxbatch.ProbePathAbsent, removedPath), probe(inboxbatch.ProbeTestExists, existingTest))
	done.Acceptance = []string{"`" + existingTest + "` passes"}
	partial := probedItem("partial", []string{livePath}, probe(inboxbatch.ProbeTestExists, existingTest), probe(inboxbatch.ProbeTestExists, "TestNotWrittenYet"))
	tree := treeOf([]string{existingTest}, livePath)

	got := Sweep([]inboxbatch.Item{done, partial}, tree, 0.5)

	if len(got) != 1 {
		t.Fatalf("Sweep = %+v, want only the consume of done: a removed path must not also quarantine it, and its named test must not add a second consume", got)
	}
	p := got[0]
	if p.Kind != ProposeConsume || p.ID != "done" || p.Path != done.Path || p.Trigger != TriggerSweep || !p.Deterministic {
		t.Errorf("proposal = %+v, want a deterministic consume of done by the sweep", p)
	}
	for _, arg := range []string{removedPath, existingTest} {
		if !slices.ContainsFunc(p.Evidence, func(line string) bool { return strings.Contains(line, arg) }) {
			t.Errorf("Evidence = %v, want it to name the passing probe %s", p.Evidence, arg)
		}
	}
}

func TestSweep_AnItemWithoutProbesKeepsTheTestShareJudgmentAndIsNeverQuarantined(t *testing.T) {
	stale := item("stale", "", []string{removedPath}, "`"+existingTest+"` passes")

	got := Sweep([]inboxbatch.Item{stale}, treeOf([]string{existingTest}), 0.5)

	if len(got) != 1 || got[0].Kind != ProposeConsume || got[0].Deterministic {
		t.Errorf("Sweep = %+v, want one judgment consume from the named test", got)
	}
}

func TestCurate_TheBackstopSweepReadsTheProbesThroughThePathSeam(t *testing.T) {
	done := probedItem("done", []string{livePath}, probe(inboxbatch.ProbePathExists, livePath))
	gone := probedItem("gone", []string{removedPath}, probe(inboxbatch.ProbePathExists, "go/internal/fleet/new.go"))
	in := Inputs{
		Open: []inboxbatch.Item{done, gone}, Backstop: true, MinShare: 0.5,
		HasTest:    testsIn(),
		PathExists: func(p string) bool { return p == livePath },
	}

	byID := map[string]Proposal{}
	for _, p := range Curate(in) {
		byID[p.ID] = p
	}

	if p := byID["done"]; p.Kind != ProposeConsume || !p.Deterministic || p.Trigger != TriggerSweep {
		t.Errorf("done = %+v, want a deterministic sweep consume", p)
	}
	if p := byID["gone"]; p.Kind != ProposeQuarantine || p.Deterministic || p.Trigger != TriggerSweep {
		t.Errorf("gone = %+v, want a judgment sweep quarantine", p)
	}

	in.Backstop = false
	if got := Curate(in); len(got) != 0 {
		t.Errorf("Curate without the backstop = %+v, want no sweep proposal", got)
	}
}

func TestProposalKind_QuarantineIsTheFourthSpelling(t *testing.T) {
	if ProposeQuarantine != "quarantine" {
		t.Errorf("ProposeQuarantine = %q, want quarantine", ProposeQuarantine)
	}
	ev := Proposal{Kind: ProposeQuarantine, ID: "gone", Trigger: TriggerSweep, Reason: "every declared path is gone"}.Event("shadow", false)
	if ev.Fields["kind"] != "quarantine" || ev.Fields["applied"] != "false" {
		t.Errorf("Event fields = %v, want kind quarantine and applied false", ev.Fields)
	}
}

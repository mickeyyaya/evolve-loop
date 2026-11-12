package gateregistry

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

var firstDebt = []string{
	"./internal/phasespec/...",
	"./internal/profiles/...",
	"./internal/phasecoherence/...",
	"./internal/routingtest/...",
	"./internal/rawgitratchet/...",
	"./internal/sizeratchet/...",
	"./internal/testmainexit/...",
	"./internal/repocontract/...",
	"./internal/policy/...",
	"./internal/guards/...",
	"./internal/acssuite/...",
	"./internal/fleet/...",
	"./internal/evalqualitycheck/...",
	"./internal/inboxrank/...",
	"./internal/ipcenv/...",
}

var enforcedGates = []string{"testdup-ratchet", "size-ratchet", "raw-git-ratchet"}

var knownPhases = []string{"build", "tdd", "audit", "ship"}

func entryNamed(t *testing.T, name string) gatecontract.Entry {
	t.Helper()
	for _, e := range Entries() {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no entry named %s", name)
	return gatecontract.Entry{}
}

func packSplitProblems(pack, covered, debt []string) []string {
	var problems []string
	seen := map[string]int{}
	for _, key := range slices.Concat(covered, debt) {
		seen[key]++
	}
	for key, n := range seen {
		if n > 1 {
			problems = append(problems, key+" is in the covers and in the debt set, or twice in one")
		}
		if !slices.Contains(pack, key) {
			problems = append(problems, key+" is not in the pack")
		}
	}
	for _, key := range pack {
		if seen[key] == 0 {
			problems = append(problems, key+" has no entry and is not in the debt set")
		}
	}
	slices.Sort(problems)
	return problems
}

func TestRegistry_PackagesEqualTheCoversAndTheDebtSet(t *testing.T) {
	t.Parallel()
	var covered []string
	for _, e := range Entries() {
		covered = append(covered, e.Covers...)
	}
	if problems := packSplitProblems(repocontract.Packages(), covered, Unregistered()); len(problems) > 0 {
		t.Errorf("the pack and the registry disagree:\n%s", strings.Join(problems, "\n"))
	}
}

func TestPackSplitProblems_NamesAnOrphanAnOverlapAndAStranger(t *testing.T) {
	t.Parallel()
	got := packSplitProblems([]string{"a", "b", "c"}, []string{"a"}, []string{"a", "d"})
	want := []string{
		"a is in the covers and in the debt set, or twice in one",
		"b has no entry and is not in the debt set",
		"c has no entry and is not in the debt set",
		"d is not in the pack",
	}
	if !slices.Equal(got, want) {
		t.Errorf("problems = %q, want %q", got, want)
	}
}

func TestUnregistered_IsASubsetOfTheFirstFifteenKeys(t *testing.T) {
	t.Parallel()
	for _, key := range Unregistered() {
		if !slices.Contains(firstDebt, key) {
			t.Errorf("%s joined the debt set; the set may only shrink after G1", key)
		}
	}
	if len(Unregistered()) != len(firstDebt) {
		t.Errorf("the debt set has %d keys, want the %d first keys while no component has retired one", len(Unregistered()), len(firstDebt))
	}
}

func TestUnregistered_ReturnsACopyThatACallerCannotMutate(t *testing.T) {
	t.Parallel()
	first := Unregistered()
	first[0] = "changed"
	if Unregistered()[0] == "changed" {
		t.Error("Unregistered() shares its backing array with the caller")
	}
}

func TestEntries_ReturnFreshValuesThatACallerCannotMutate(t *testing.T) {
	t.Parallel()
	first := Entries()
	first[0].Name = "changed"
	first[0].Files[0] = "changed"
	if second := Entries(); second[0].Name == "changed" || second[0].Files[0] == "changed" {
		t.Error("Entries() shares state with the caller")
	}
}

func entryProblems(e gatecontract.Entry, phases []string) []string {
	var problems []string
	add := func(ok bool, text string) {
		if !ok {
			problems = append(problems, e.Name+": "+text)
		}
	}
	add(e.Name != "", "no name")
	add(slices.Contains(gatecontract.Kinds(), e.Kind), "unknown kind "+e.Kind)
	add(slices.Contains(gatecontract.Stages(), e.Stage), "unknown stage "+e.Stage)
	add(len(e.Files) > 0, "no file")
	add(len(e.Owners) > 0, "no owner")
	add(len(e.Inputs) > 0, "no input")
	add(e.Run != nil, "no run function")
	for _, owner := range e.Owners {
		add(slices.Contains(phases, owner.Phase), "owner phase "+owner.Phase+" is not a phase")
	}
	return problems
}

func TestEntryProblems_FlagsAMissingRunFunctionAndAnUnknownPhase(t *testing.T) {
	t.Parallel()
	bad := gatecontract.Entry{Name: "x", Kind: "scanner", Stage: "enforce", Files: []string{"f"}, Inputs: []gatecontract.Declared{{Path: "p"}}, Owners: []gatecontract.Owner{{Pattern: "*", Phase: "review"}}}
	got := strings.Join(entryProblems(bad, knownPhases), "\n")
	for _, want := range []string{"no run function", "owner phase review is not a phase"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems = %q, want a line with %q", got, want)
		}
	}
}

func TestEntries_AreUniquelyNamed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, e := range Entries() {
		if seen[e.Name] {
			t.Errorf("two entries are named %s", e.Name)
		}
		seen[e.Name] = true
	}
}

func stageProblems(entries []gatecontract.Entry, enforced []string) []string {
	var problems []string
	for _, name := range enforced {
		i := slices.IndexFunc(entries, func(e gatecontract.Entry) bool { return e.Name == name })
		switch {
		case i < 0:
			problems = append(problems, name+" is not an entry")
		case entries[i].Stage != gatecontract.StageEnforce:
			problems = append(problems, name+" is at "+entries[i].Stage+", want enforce")
		}
	}
	return problems
}

func TestEnforcedGates_EachIsAnEntryAtEnforce(t *testing.T) {
	t.Parallel()
	if problems := stageProblems(Entries(), enforcedGates); len(problems) > 0 {
		t.Errorf("an enforced gate left enforce:\n%s", strings.Join(problems, "\n"))
	}
}

func TestStageProblems_FlagsAShadowEntryAndAMissingOne(t *testing.T) {
	t.Parallel()
	entries := []gatecontract.Entry{{Name: "a", Stage: gatecontract.StageShadow}, {Name: "b", Stage: gatecontract.StageEnforce}}
	want := []string{"a is at shadow, want enforce", "c is not an entry"}
	if got := stageProblems(entries, []string{"a", "b", "c"}); !slices.Equal(got, want) {
		t.Errorf("problems = %q, want %q", got, want)
	}
}

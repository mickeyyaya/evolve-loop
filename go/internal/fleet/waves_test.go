package fleet

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/ipcenv"
)

// scopesOf reduces waves to [wave][spec][todo-id] for assertion.
func scopesOf(waves [][]CycleSpec) [][][]string {
	out := make([][][]string, len(waves))
	for i, wave := range waves {
		specs := make([][]string, len(wave))
		for j, s := range wave {
			specs[j] = s.Scope
		}
		out[i] = specs
	}
	return out
}

func td(id string, files []string, deps ...string) Todo {
	return Todo{ID: id, Files: files, DependsOn: deps}
}

func TestPlanWaves(t *testing.T) {
	cases := []struct {
		name  string
		todos []Todo
		want  [][][]string
	}{
		{
			name: "linear deps: one cycle per wave",
			todos: []Todo{
				td("a", []string{"fa"}),
				td("b", []string{"fb"}, "a"),
				td("c", []string{"fc"}, "b"),
			},
			want: [][][]string{{{"a"}}, {{"b"}}, {{"c"}}},
		},
		{
			name: "diamond, file-disjoint middle runs concurrently",
			todos: []Todo{
				td("a", []string{"fa"}),
				td("b", []string{"fb"}, "a"),
				td("c", []string{"fc"}, "a"),
				td("d", []string{"fd"}, "b", "c"),
			},
			want: [][][]string{{{"a"}}, {{"b"}, {"c"}}, {{"d"}}},
		},
		{
			name: "within a wave, file-sharing todos cluster into ONE cycle",
			todos: []Todo{
				td("a", []string{"fa"}),
				td("b", []string{"shared"}, "a"),
				td("c", []string{"shared"}, "a"),
			},
			want: [][][]string{{{"a"}}, {{"b", "c"}}},
		},
		{
			name: "all share one file: one wave, one batched cycle (the flag-reduction reality)",
			todos: []Todo{
				td("r1", []string{"registry.go"}),
				td("r2", []string{"registry.go"}),
				td("r3", []string{"registry.go"}),
			},
			want: [][][]string{{{"r1", "r2", "r3"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			waves, err := PlanWaves(tc.todos)
			if err != nil {
				t.Fatalf("PlanWaves: %v", err)
			}
			if got := scopesOf(waves); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("waves = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlanWaves_CyclicDepsRejected(t *testing.T) {
	todos := []Todo{
		td("a", []string{"fa"}, "b"),
		td("b", []string{"fb"}, "a"),
	}
	if _, err := PlanWaves(todos); err == nil {
		t.Fatal("PlanWaves: expected error for cyclic depends_on, got nil")
	}
}

func TestPlanWaves_SetsScopeEnv(t *testing.T) {
	waves, err := PlanWaves([]Todo{td("a", []string{"fa"}), td("b", []string{"fb"})})
	if err != nil {
		t.Fatal(err)
	}
	// One wave (no deps), two file-disjoint specs.
	if len(waves) != 1 || len(waves[0]) != 2 {
		t.Fatalf("want 1 wave of 2 specs, got %v", scopesOf(waves))
	}
	for _, s := range waves[0] {
		if s.Env[ipcenv.FleetScopeKey] != strings.Join(s.Scope, ",") {
			t.Errorf("spec %v: EVOLVE_FLEET_SCOPE=%q, want %q", s.Scope, s.Env[ipcenv.FleetScopeKey], strings.Join(s.Scope, ","))
		}
	}
}

func TestGroupByFiles_TransitiveSharing(t *testing.T) {
	// a—b share x, b—c share y ⇒ all three transitively connected ⇒ one group.
	groups := groupByFiles([]Todo{
		td("a", []string{"x"}),
		td("b", []string{"x", "y"}),
		td("c", []string{"y"}),
	})
	if len(groups) != 1 || len(groups[0]) != 3 {
		t.Errorf("transitive sharing: want 1 group of 3, got %d groups", len(groups))
	}
}

// TestPlanWaves_ThreadsOutputContract: a single-todo (file-disjoint) cycle
// carries that todo's OutputContract verbatim onto its CycleSpec, so it reaches
// the launched cycle as its binding goal (issue 14/22 — the contract was dropped
// Todo→CycleSpec, leaving the scout to free-choose a non-reducing task).
func TestPlanWaves_ThreadsOutputContract(t *testing.T) {
	waves, err := PlanWaves([]Todo{
		{ID: "a", Files: []string{"x.go"}, OutputContract: "delete EVOLVE_FOO; FlagCeiling toward 35"},
		{ID: "b", Files: []string{"y.go"}, OutputContract: "delete EVOLVE_BAR"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, spec := range waves[0] {
		got[spec.Scope[0]] = spec.OutputContract
	}
	if got["a"] != "delete EVOLVE_FOO; FlagCeiling toward 35" {
		t.Errorf("cycle a contract = %q", got["a"])
	}
	if got["b"] != "delete EVOLVE_BAR" {
		t.Errorf("cycle b contract = %q", got["b"])
	}
}

// TestPlanWaves_CombinesContractsForFileSharingGroup: when file-sharing todos
// merge into one cycle, each todo's contract is preserved, labeled by id.
func TestPlanWaves_CombinesContractsForFileSharingGroup(t *testing.T) {
	waves, err := PlanWaves([]Todo{
		{ID: "a", Files: []string{"shared.go"}, OutputContract: "do A"},
		{ID: "b", Files: []string{"shared.go"}, OutputContract: "do B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(waves[0]) != 1 {
		t.Fatalf("file-sharing todos must merge into 1 cycle, got %d specs", len(waves[0]))
	}
	oc := waves[0][0].OutputContract
	if !strings.Contains(oc, "[a] do A") || !strings.Contains(oc, "[b] do B") {
		t.Fatalf("combined contract must label each todo: %q", oc)
	}
}

// TestPlanCycles_ThreadsOutputContract: the flat `evolve fleet --plan` path
// (PlanCycles) carries the contract onto its specs too, not just the campaign
// PlanWaves path — both partitioners thread it (reviewer HIGH).
func TestPlanCycles_ThreadsOutputContract(t *testing.T) {
	specs, _ := PlanCycles([]Todo{
		{ID: "a", Files: []string{"x.go"}, OutputContract: "delete EVOLVE_FOO"},
	}, 4)
	if len(specs) != 1 {
		t.Fatalf("want 1 spec, got %d", len(specs))
	}
	if specs[0].OutputContract != "delete EVOLVE_FOO" {
		t.Fatalf("PlanCycles dropped the contract: %q", specs[0].OutputContract)
	}
}

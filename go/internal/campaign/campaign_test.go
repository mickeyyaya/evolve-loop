package campaign

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func validPlan() *Plan {
	return &Plan{
		Version: 1,
		Goal:    "reduce flags",
		Cycles: []fleet.Todo{
			{ID: "a", Files: []string{"fa"}},
			{ID: "b", Files: []string{"fb"}, DependsOn: []string{"a"}},
		},
	}
}

func TestLoad_RejectsUnknownFields(t *testing.T) {
	js := `{"version":1,"goal":"g","cycles":[{"id":"a","files":["f"]}],"bogus":true}`
	if _, err := Load([]byte(js)); err == nil {
		t.Fatal("Load: expected error on unknown field, got nil")
	}
}

func TestLoad_Valid(t *testing.T) {
	js := `{"version":2,"goal":"g","research":{"citations":[{"title":"ADaPT","url":"https://arxiv.org/abs/2311.05772"}]},"cycles":[{"id":"a","files":["f"],"output_contract":"done when X"}]}`
	p, err := Load([]byte(js))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Version != 2 || p.Goal != "g" || len(p.Cycles) != 1 || len(p.Research.Citations) != 1 {
		t.Errorf("parsed plan unexpected: %+v", p)
	}
	if p.Cycles[0].OutputContract != "done when X" {
		t.Errorf("output_contract not parsed: %q", p.Cycles[0].OutputContract)
	}
}

func TestVerify_Valid(t *testing.T) {
	if err := validPlan().Verify(); err != nil {
		t.Fatalf("Verify(valid): %v", err)
	}
}

func TestVerify_Errors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
		want   string
	}{
		{"zero version", func(p *Plan) { p.Version = 0 }, "version"},
		{"empty goal", func(p *Plan) { p.Goal = "  " }, "goal is empty"},
		{"no cycles", func(p *Plan) { p.Cycles = nil }, "no cycles"},
		{"empty id", func(p *Plan) { p.Cycles[1].ID = "" }, "empty id"},
		{"duplicate id", func(p *Plan) { p.Cycles[1].ID = "a" }, "duplicate cycle id"},
		{"comma in id", func(p *Plan) { p.Cycles[1].ID = "b,c" }, "invalid char"},
		{"dangling dep", func(p *Plan) { p.Cycles[1].DependsOn = []string{"ghost"} }, "invalid cycle DAG"},
		{"cyclic dep", func(p *Plan) { p.Cycles[0].DependsOn = []string{"b"} }, "invalid cycle DAG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validPlan()
			tc.mutate(p)
			err := p.Verify()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Verify: want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestWaves_DependencyOrder(t *testing.T) {
	waves, err := validPlan().Waves()
	if err != nil {
		t.Fatalf("Waves: %v", err)
	}
	// a (no deps) in wave 0; b (deps a) in wave 1.
	if len(waves) != 2 || len(waves[0]) != 1 || waves[0][0].Scope[0] != "a" || waves[1][0].Scope[0] != "b" {
		t.Errorf("unexpected waves: wave0=%v wave1=%v", waves[0], waves[1])
	}
}

func TestDiff(t *testing.T) {
	old := validPlan()
	updated := validPlan()
	updated.Cycles[1].Files = []string{"fb", "fc"}               // modify b
	updated.Cycles = append(updated.Cycles, fleet.Todo{ID: "c"}) // add c
	d := Diff(old, updated)
	if !strings.Contains(d, "added: [c]") || !strings.Contains(d, "modified: [b]") || !strings.Contains(d, "removed: []") {
		t.Errorf("Diff = %q, want added [c], modified [b], removed []", d)
	}
}

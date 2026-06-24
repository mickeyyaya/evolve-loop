package campaign

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestRender(t *testing.T) {
	p := &Plan{
		Version: 1,
		Goal:    "reduce flags",
		Research: Research{
			Summary:   "decompose as-needed; batch same-file work",
			Citations: []Citation{{Title: "ADaPT", URL: "https://arxiv.org/abs/2311.05772", Note: "as-needed decomposition"}},
		},
		Cycles: []fleet.Todo{
			{ID: "dead-sweep", Files: []string{"registry.go"}, OutputContract: "all dead flags removed"},
			{ID: "internal-batch", Files: []string{"registry.go"}, DependsOn: []string{"dead-sweep"}},
		},
	}
	out, err := p.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"# Campaign Plan (v1)",
		"reduce flags",
		"ADaPT",
		"https://arxiv.org/abs/2311.05772",
		"| dead-sweep |",
		"all dead flags removed",
		"## Execution Waves (2)",
		"Wave 1",
		"Wave 2",
		"[dead-sweep]",
		"[internal-batch]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRender_InvalidDAGErrors(t *testing.T) {
	p := &Plan{
		Version: 1,
		Goal:    "g",
		Cycles: []fleet.Todo{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}
	if _, err := p.Render(); err == nil {
		t.Fatal("Render: expected error for cyclic DAG, got nil")
	}
}

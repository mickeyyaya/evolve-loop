package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func writeInboxRecord(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, ".evolve", "inbox", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func planPromptForLane(t *testing.T, root, scope string, resolve func(projectRoot, taskID string) string) string {
	t.Helper()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	pl := &capturingPlanner{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}), WithPlanner(pl), WithScopePathResolver(resolve))
	req := CycleRequest{ProjectRoot: root, Env: map[string]string{ipcenv.FleetScopeKey: scope}}
	if _, err := o.RunCycle(context.Background(), req); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	var b strings.Builder
	advisor.WriteRoutingContext(&b, pl.got)
	return b.String()
}

func TestPlanCycle_TheAdvisorPlanPromptCarriesTheLanesScopedItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	record := writeInboxRecord(t, root, "doc-item.json",
		`{"id":"doc-item","title":"a strategy","kind":"strategy","deliverable_kind":"document","acceptance":["two distinct options","a recommendation"]}`)
	prompt := planPromptForLane(t, root, "doc-item", func(_, id string) string {
		if id == "doc-item" {
			return record
		}
		return ""
	})
	for _, want := range []string{"- id: doc-item", "deliverable_kind: document", "kind: strategy", "1. two distinct options"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the plan prompt lacks %q:\n%s", want, prompt)
		}
	}
}

func TestPlanCycle_AnUnresolvableLaneItemIsNamedInThePlanPrompt(t *testing.T) {
	t.Parallel()
	prompt := planPromptForLane(t, t.TempDir(), "lost-item", func(string, string) string { return "" })
	if !strings.Contains(prompt, "- id: lost-item (inbox record not resolved)") {
		t.Errorf("an unresolvable lane item must be named with the reason:\n%s", prompt)
	}
}

func TestLaneItem_TheDeclaredKindIsNormalizedAndAnUnknownWordFallsToTheProjectDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDomainJSON(t, root, `{"domain":"writing"}`)
	for _, tc := range []struct{ name, declared, want string }{
		{"undeclared inherits the project default", `""`, config.DeliverableKindDocument},
		{"a declared kind beats the project default", `"code"`, config.DeliverableKindCode},
		{"a declared kind is normalized", `" Code "`, config.DeliverableKindCode},
		{"an unknown word is no declaration", `"banana"`, config.DeliverableKindDocument},
	} {
		record := writeInboxRecord(t, root, tc.name+".json", `{"id":"item","deliverable_kind":`+tc.declared+`}`)
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
			WithScopePathResolver(func(string, string) string { return record }))
		if got := o.laneItem(root, "item").DeliverableKind; got != tc.want {
			t.Errorf("%s: deliverable_kind = %q, want %q", tc.name, got, tc.want)
		}
	}
}

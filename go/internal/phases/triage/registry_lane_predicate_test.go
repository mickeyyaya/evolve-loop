package triage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func writeBuilderProfileDenying(t *testing.T, root, denied string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"builder","sandbox":{"enabled":true,"deny_subpaths":["` + denied + `"]}}`
	if err := os.WriteFile(filepath.Join(dir, "builder.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryFactory_TriageJudgesWithTheBuilderSandboxPredicate(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfileDenying(t, root, ".evolve/profiles")
	writeInboxItem(t, root, "a.json", `{"id":"lane-work","weight":0.9}`)
	writeInboxItem(t, root, "b.json", `{"id":"needs-a-profile","weight":0.95,"files":[".evolve/profiles/historian.json (new)"]}`)
	factory, ok := registry.For(string(core.PhaseTriage))
	if !ok {
		t.Fatal("triage is not registered")
	}
	req := core.PhaseRequest{ProjectRoot: root}
	phase, isPhase := factory(req).(*Phase)
	if !isPhase {
		t.Fatal("the triage factory must build a *Phase")
	}
	out := phase.ComposePrompt("BODY", req)
	if !strings.Contains(excludedIDs(out), "needs-a-profile") {
		t.Fatalf("serve-phase and `evolve phase triage` must exclude what the builder sandbox denies:\n%s", out)
	}
	if strings.Contains(excludedIDs(out), "lane-work") {
		t.Fatalf("an allowed item must stay selectable:\n%s", out)
	}
}

func TestRegistryConfig_WithoutTheBuilderProfileEmitsTheRoutingFallbackCode(t *testing.T) {
	var warn bytes.Buffer
	cfg := registryConfig(core.PhaseRequest{ProjectRoot: t.TempDir()}, &warn)
	if cfg.LaneForbidden == nil {
		t.Fatal("the registry config must always carry the lane predicate")
	}
	if !cfg.LaneForbidden("go/internal/guards/role.go") || cfg.LaneForbidden(".evolve/profiles/historian.json") {
		t.Fatal("the fallback judges protected surface only")
	}
	if got := strings.Count(warn.String(), string(lanerouting.CodeRoutingFallback)); got != 1 {
		t.Fatalf("a missing builder profile must emit the registered fallback code once, got %q", warn.String())
	}
}

//go:build acs

package cycle2457

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	classFact   = "console_route_fact"
	classNoFact = "console_route_no_fact"
	classOld    = "console_route"
	protectedGo = "go/internal/guards/phase.go"
)

type census struct {
	Counts   map[string]int `json:"counts"`
	Refusals []struct {
		ID    string `json:"id"`
		Class string `json:"class"`
	} `json:"refusals"`
}

var (
	buildOnce sync.Once
	binary    string
	buildErr  error
	buildOut  []byte
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-acs-2457-")
		if err != nil {
			buildErr = err
			return
		}
		binary = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", binary, "./cmd/evolve")
		cmd.Dir = moduleDir
		buildOut, buildErr = cmd.CombinedOutput()
	})
	if buildErr != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", buildErr, buildOut)
	}
	return binary
}

func item(id string, route string, files ...string) map[string]any {
	it := map[string]any{"id": id, "weight": 0.8, "priority_class": "feature", "files": files}
	if route != "" {
		it["route"] = route
	}
	return it
}

func waveSupply(t *testing.T, items ...map[string]any) census {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "wave-goal.md"), []byte("Work the highest-weight lane-eligible inbox items.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"lastCycleNumber":1836}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		raw, err := json.Marshal(it)
		if err != nil {
			t.Fatal(err)
		}
		inboxFile := filepath.Join(evolveDir, "inbox", it["id"].(string)+".json")
		if err := os.MkdirAll(filepath.Dir(inboxFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(inboxFile, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "wave", "supply", "--json", "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("evolve wave supply --json exit=%d err=%v\nstderr:\n%s", code, err, stderr)
	}
	var envelope struct {
		Data census `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("census is not JSON: %v\n%s", err, out)
	}
	if envelope.Data.Counts == nil {
		t.Fatalf("census envelope has no data.counts:\n%s", out)
	}
	return envelope.Data
}

func refusalClass(c census, id string) string {
	for _, r := range c.Refusals {
		if r.ID == id {
			return r.Class
		}
	}
	return ""
}

func TestC2457_001_ARouteConsoleItemOfUnprotectedFilesCountsAsNoFact(t *testing.T) {
	got := waveSupply(t,
		item("lane-a", "", "go/internal/alpha/a.go"),
		item("route-only", "console", "go/internal/beta/x.go"),
	)
	if got.Counts[classNoFact] != 1 {
		t.Errorf("counts = %v, want %s = 1 for a console route that no protected file explains", got.Counts, classNoFact)
	}
	if got.Counts[classFact] != 0 {
		t.Errorf("counts = %v, want %s = 0: an unprotected file is not a checked fact", got.Counts, classFact)
	}
	if class := refusalClass(got, "route-only"); class != classNoFact {
		t.Errorf("refusal class of route-only = %q, want %q", class, classNoFact)
	}
}

func TestC2457_002_ARouteConsoleItemWithAProtectedDeclaredFileCountsAsFact(t *testing.T) {
	got := waveSupply(t,
		item("lane-a", "", "go/internal/alpha/a.go"),
		item("route-guard", "console", protectedGo),
		item("guard-no-route", "", protectedGo),
	)
	if got.Counts[classFact] != 1 {
		t.Errorf("counts = %v, want %s = 1 for a console route whose declared file is protected", got.Counts, classFact)
	}
	if got.Counts[classNoFact] != 0 {
		t.Errorf("counts = %v, want %s = 0: the protected file explains the route", got.Counts, classNoFact)
	}
	if class := refusalClass(got, "route-guard"); class != classFact {
		t.Errorf("refusal class of route-guard = %q, want %q", class, classFact)
	}
	if got.Counts["console_protected"] != 1 || refusalClass(got, "guard-no-route") != "console_protected" {
		t.Errorf("counts = %v, want the item with no route to stay console_protected, outside both route classes", got.Counts)
	}
}

func TestC2457_003_WaveSupplyJSONCarriesBothCountsAndTheirSumIsTheOldRouteCount(t *testing.T) {
	got := waveSupply(t,
		item("lane-a", "", "go/internal/alpha/a.go"),
		item("route-only", "console", "go/internal/beta/x.go"),
		item("route-guard", "console", protectedGo),
		item("route-other", "console", "go/internal/gamma/y.go"),
	)
	for _, class := range []string{classFact, classNoFact} {
		if _, ok := got.Counts[class]; !ok {
			t.Errorf("counts = %v lacks %s", got.Counts, class)
		}
	}
	if sum := got.Counts[classFact] + got.Counts[classNoFact]; sum != 3 {
		t.Errorf("%s + %s = %d, want 3: the old %s count of the three route:console items", classFact, classNoFact, sum, classOld)
	}
	if got.Counts[classFact] != 1 || got.Counts[classNoFact] != 2 {
		t.Errorf("counts = %v, want 1 fact and 2 no_fact", got.Counts)
	}
	if _, ok := got.Counts[classOld]; ok {
		t.Errorf("counts = %v still carries %s: Supply.Total would count each route item twice", got.Counts, classOld)
	}
	total := 0
	for _, n := range got.Counts {
		total += n
	}
	if total != 4 {
		t.Errorf("sum of every count = %d, want 4 items", total)
	}
}

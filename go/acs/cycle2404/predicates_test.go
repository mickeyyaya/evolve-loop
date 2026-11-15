//go:build acs

package cycle2404

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		_ = os.RemoveAll(evolveBuild.dir)
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2404-evolve-")
		if err != nil {
			evolveBuild.failure = err.Error()
			return
		}
		evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
		build.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		if out, err := build.CombinedOutput(); err != nil {
			evolveBuild.failure = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

type proposal struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Target string `json:"target"`
}

func curateRoutes(t *testing.T, items map[string]string) map[string]proposal {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".evolve/policy.json", `{"inbox":{"curator":{"stage":"shadow"}}}`)
	repo.Write("go/internal/fleet/pool.go", "package fleet\n")
	for id, body := range items {
		repo.Write(".evolve/inbox/2026-11-01T00-00-00Z-"+id+".json", body)
	}
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "open items")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), "inbox", "curate", "--json")
	cmd.Dir, cmd.WaitDelay = repo.Dir, 5*time.Second
	cmd.Env = []string{"EVOLVE_PROJECT_ROOT=" + repo.Dir, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("evolve inbox curate --json: %v\n%s", err, out)
	}
	var report struct {
		Proposals []proposal `json:"proposals"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("the curate report does not decode: %v\n%s", err, out)
	}
	reroutes := map[string]proposal{}
	for _, p := range report.Proposals {
		if p.Kind == "reroute" {
			reroutes[p.ID] = p
		}
	}
	return reroutes
}

func consoleManualItem(id, kind, files string) string {
	return fmt.Sprintf(`{"id":%q,"kind":%q,"priority_class":"stability","weight":0.5,"origin":"console","route":"console-manual",`+
		`"title":"item %s","summary":"s","fix":"f","files":%s,"acceptance":["it is done"]}`, id, kind, id, files)
}

const named = `["go/internal/fleet/pool.go"]`

func requireControlRerouted(t *testing.T, got map[string]proposal) {
	t.Helper()
	if p := got["control"]; p.Target != "lane" {
		t.Fatalf("control (a buildable bug item naming a file, routed console-manual) = %+v, want a reroute to a lane; the curator stopped proposing lane reroutes altogether", p)
	}
}

func TestC2404_001_AResearchDecisionOrTaskItemOnTheConsoleGetsNoLaneReroute(t *testing.T) {
	items := map[string]string{"control": consoleManualItem("control", "bug", named)}
	unbuildable := []string{"research", "decision", "task"}
	for _, kind := range unbuildable {
		items["item-"+kind] = consoleManualItem("item-"+kind, kind, named)
	}

	got := curateRoutes(t, items)

	requireControlRerouted(t, got)
	for _, kind := range unbuildable {
		if p, ok := got["item-"+kind]; ok {
			t.Errorf("kind %s routed console-manual drew a lane reroute proposal %+v, want none: a lane cannot build it", kind, p)
		}
	}
}

func TestC2404_002_AnItemThatNamesNoFilesGetsNoLaneReroute(t *testing.T) {
	items := map[string]string{
		"control":          consoleManualItem("control", "bug", named),
		"no-files-field":   consoleManualItem("no-files-field", "bug", `[]`),
		"only-prose-files": consoleManualItem("only-prose-files", "bug", `["none yet"]`),
	}

	got := curateRoutes(t, items)

	requireControlRerouted(t, got)
	for _, id := range []string{"no-files-field", "only-prose-files"} {
		if p, ok := got[id]; ok {
			t.Errorf("%s names no file path yet drew a lane reroute proposal %+v, want none: a lane has nothing to build", id, p)
		}
	}
}

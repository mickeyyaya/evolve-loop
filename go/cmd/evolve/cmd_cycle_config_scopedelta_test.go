package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestProductionBuildFloorChecks_ReportsAScopeDeltaThroughTheComposedFloor(t *testing.T) {
	wt := gittest.Fixture(t)
	for _, rel := range []string{"docs/alpha.md", "docs/beta.md", "docs/gamma.md", ".evolve/policy.json"} {
		wt.Write(rel, "base\n")
	}
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "base")
	base := wt.Git("rev-parse", "HEAD")
	for _, rel := range []string{"docs/alpha.md", "docs/beta.md", "docs/gamma.md", ".evolve/policy.json"} {
		wt.Write(rel, "changed\n")
	}
	projectRoot := t.TempDir()
	workspace := t.TempDir()
	writeScopeDeltaLane(t, filepath.Join(projectRoot, ".evolve"), "7", "g", "self", "docs/alpha.md")
	writeScopeDeltaLane(t, filepath.Join(projectRoot, ".evolve"), "8", "g", "sib", "docs/beta.md")
	prev := repoContractPack
	t.Cleanup(func() { repoContractPack = prev })
	repoContractPack = func(context.Context, string, string, []string) ([]string, string, error) { return nil, "", nil }
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeScopeDelta {
			events = append(events, e)
		}
	})
	var stderr bytes.Buffer

	got := core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: projectRoot, Workspace: workspace, Signals: center, Stderr: &stderr,
	})

	if len(got) != 1 || !strings.Contains(got[0], "protected control-plane path changed: .evolve/policy.json") {
		t.Fatalf("the composed floor still blocks only on the protected surface; the scope delta is a report: %v", got)
	}
	if len(events) != 1 {
		t.Fatalf("the composed floor emits one summary scope-delta signal: got %d (%+v)", len(events), events)
	}
	if events[0].Fields["total"] != "3" || events[0].Fields["disposition_carve"] != "2" || events[0].Fields["disposition_refuse"] != "1" {
		t.Fatalf("summary signal counts 3 paths (2 carve, 1 refuse): %+v", events[0].Fields)
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "scope-delta.json"))
	if err != nil {
		t.Fatalf("scope-delta.json missing from workspace: %v", err)
	}
	var doc struct {
		Entries []struct {
			Path  string `json:"path"`
			Class string `json:"class"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("scope-delta.json invalid: %v\n%s", err, raw)
	}
	classes := map[string]string{}
	for _, e := range doc.Entries {
		classes[e.Path] = e.Class
	}
	want := map[string]string{"docs/beta.md": "cross-lane", "docs/gamma.md": "opportunistic", ".evolve/policy.json": "boundary"}
	if len(classes) != len(want) {
		t.Fatalf("the live composition classifies every path outside the declared scope, through the real protected manifest, and none for the declared alpha.md: %v", classes)
	}
	for path, class := range want {
		if classes[path] != class {
			t.Errorf("%s: class %q, want %q", path, classes[path], class)
		}
	}
}

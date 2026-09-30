//go:build acs

package cycle1783

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
)

const bridgeFloorPredicates = `//go:build acs

package cycle300

import "testing"

func TestC300_020_BridgeCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/adapters/bridge/")
	if pct < 98.0 {
		t.Errorf("RED: bridge coverage = %.1f%%", pct)
	}
}
`

const triageCommittingBridge = "## top_n (commit to THIS cycle)\n- coverage-bridge: adapters/bridge coverage to >=98% — priority=H\n\n## deferred (carry to NEXT cycle's carryoverTodos)\n"

func floorBindingInput(t *testing.T, companion string) core.ReviewInput {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-300")
	wt := filepath.Join(root, "wt")
	acsDir := filepath.Join(wt, "go", "acs", "cycle300")
	for _, d := range []string{ws, acsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(ws, "triage-report.md"):       triageCommittingBridge,
		filepath.Join(acsDir, "predicates_test.go"): bridgeFloorPredicates,
		filepath.Join(ws, "triage-decision.json"):   companion,
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: root}
}

func TestC1783_001_MalformedTriageCompanionBlocksFloorBinding(t *testing.T) {
	in := floorBindingInput(t, `{"committed_floors": ["bridge"`)
	res := evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), in)
	if res.Approve {
		t.Fatal("RED: a malformed triage-decision.json was silently ignored; the floor-binding gate must fail loud")
	}
	if !strings.Contains(res.Reason, "triage-decision.json") {
		t.Errorf("RED: finding must name the malformed companion; got %q", res.Reason)
	}
}

func TestC1783_002_WellFormedCompanionStillPasses(t *testing.T) {
	in := floorBindingInput(t, `{"committed_floors":["bridge"],"deferred_floors":[]}`)
	res := evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), in)
	if !res.Approve {
		t.Errorf("a well-formed companion binding only a committed floor must pass; got %q", res.Reason)
	}
}

func TestC1783_003_MissingCompanionStillPasses(t *testing.T) {
	in := floorBindingInput(t, "")
	if err := os.Remove(filepath.Join(in.Workspace, "triage-decision.json")); err != nil {
		t.Fatal(err)
	}
	res := evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), in)
	if !res.Approve {
		t.Errorf("an absent companion is not an error and must fall back to prose; got %q", res.Reason)
	}
}

func TestC1783_004_UngradedRemediationNamesWorkspacePathNotProjectRoot(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	report := "# Scout Report\n\n## Selected Tasks\n\n- **Slug:** `some-slug`\n"
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	rootEval := filepath.Join(root, ".evolve", "evals", "some-slug.md")
	if err := os.MkdirAll(filepath.Dir(rootEval), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootEval, []byte("# Eval\n\nexistence only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := evalgate.NewReviewer(config.StageEnforce).Review(context.Background(),
		core.ReviewInput{Phase: "scout", ProjectRoot: root, Workspace: ws})
	if res.Approve {
		t.Fatal("an eval with no [code] grader must block the scout")
	}
	wsEval := filepath.Join(ws, ".evolve", "evals", "some-slug.md")
	if !strings.Contains(res.Remediation, wsEval) {
		t.Errorf("RED: remediation must name the writable workspace path %s; got %q", wsEval, res.Remediation)
	}
	if strings.Contains(res.Remediation, rootEval) {
		t.Errorf("RED: remediation names the deny-write project-root path %s", rootEval)
	}
}

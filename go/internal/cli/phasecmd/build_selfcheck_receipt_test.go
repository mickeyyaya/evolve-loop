package phasecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func boundSelfCheck(t *testing.T, worktree string, failures []string) (string, BuildSelfCheckResult, string) {
	t.Helper()
	root, ws := floorWorkspace(t)
	body, err := json.Marshal(core.CycleState{CycleID: 1788, Phase: "build", WorkspacePath: ws, ActiveWorktree: worktree})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer
	floor := &recordedFloor{failures: failures}
	got, err := BuildSelfCheck{Floor: floor.floorFor, Probe: core.BuildHandoffProbe{Workspace: ws, EvolveDir: filepath.Join(root, ".evolve"), ProjectRoot: root}}.Verify(&errb)
	if err != nil {
		t.Fatal(err)
	}
	return ws, got, errb.String()
}

func TestBuildSelfCheck_ABoundCheckWritesAReceiptForItsTree(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("x.go", "package x\n")
	repo.Git("add", "x.go")
	ws, got, stderr := boundSelfCheck(t, repo.Dir, []string{"cover-changed: Foo is at 80.0%"})
	raw, err := os.ReadFile(filepath.Join(ws, core.SelfCheckReceiptFile))
	if err != nil {
		t.Fatalf("a bound self-check writes its receipt: %v; stderr=%s", err, stderr)
	}
	var receipt struct {
		Tree  string `json:"tree"`
		Green bool   `json:"green"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.Tree == "" || receipt.Green != got.OK || got.OK {
		t.Fatalf("receipt = %s (%v); want a tree digest and green=%v", raw, err, got.OK)
	}
}

func TestBuildSelfCheck_AReceiptThatCannotBindATreeWarns(t *testing.T) {
	ws, _, stderr := boundSelfCheck(t, filepath.Join(t.TempDir(), "gone"), nil)
	if !strings.Contains(stderr, "no self-check receipt") {
		t.Fatalf("stderr = %q; want a WARN that names the missing receipt", stderr)
	}
	if _, err := os.Stat(filepath.Join(ws, core.SelfCheckReceiptFile)); !os.IsNotExist(err) {
		t.Fatalf("a receipt with no tree was written: %v", err)
	}
}

func TestBuildSelfCheck_ACheckOfAnotherWorktreeWritesNoReceipt(t *testing.T) {
	root, ws := floorWorkspace(t)
	body, err := json.Marshal(core.CycleState{CycleID: 1788, Phase: "build", WorkspacePath: ws, ActiveWorktree: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	other := gittest.Fixture(t)
	other.Write("x.go", "package x\n")
	other.Git("add", "x.go")
	var errb bytes.Buffer
	got, err := BuildSelfCheck{Floor: (&recordedFloor{}).floorFor, Probe: core.BuildHandoffProbe{Workspace: ws, EvolveDir: filepath.Join(root, ".evolve"), ProjectRoot: root, Worktree: other.Dir}}.Verify(&errb)
	if err != nil || got.Unbound == nil {
		t.Fatalf("a check of a worktree the cycle does not bind is unbound: %+v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(ws, core.SelfCheckReceiptFile)); !os.IsNotExist(err) {
		t.Fatalf("an unbound self-check predicts nothing and must write no receipt: %v", err)
	}
}

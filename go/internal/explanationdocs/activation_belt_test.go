package explanationdocs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossCheckActivation_Matrix(t *testing.T) {
	f := newFixture(t)
	f.activate(t)

	good := f.binding()
	if active, err := CrossCheckActivation(good); !active || err != nil {
		t.Fatalf("matching identity: active=%v err=%v, want (true, nil)", active, err)
	}

	dropped := f.binding()
	dropped.ContractVersion = 0
	if _, err := CrossCheckActivation(dropped); err == nil {
		t.Fatal("zero version against an ACTIVE host must fail loudly (the audit-blind case)")
	} else if !strings.Contains(err.Error(), "contract version does not match host activation") {
		t.Fatalf("mismatch must name the divergent field, got %v", err)
	}

	wrongRun := f.binding()
	wrongRun.RunID = "run-someone-else"
	if _, err := CrossCheckActivation(wrongRun); err == nil || !strings.Contains(err.Error(), "run id does not match") {
		t.Fatalf("foreign run id must fail naming the field: %v", err)
	}

	wrongBase := f.binding()
	wrongBase.BaseSHA = "0000000000000000000000000000000000000000"
	if _, err := CrossCheckActivation(wrongBase); err == nil {
		t.Fatal("stale base SHA must fail")
	}

	g := newFixture(t)
	legacy := g.binding()
	legacy.ContractVersion = 0
	if active, err := CrossCheckActivation(legacy); active || err != nil {
		t.Fatalf("version-0 with no host activation is genuine legacy: active=%v err=%v, want (false, nil)", active, err)
	}
	stale := g.binding()
	if active, err := CrossCheckActivation(stale); active || err == nil {
		t.Fatalf("live version with no host activation must fail: active=%v err=%v", active, err)
	}
}

func TestCycleFromRunWorkspaceDerivation(t *testing.T) {
	root := t.TempDir()
	if c, ok := cycleFromRunWorkspace(root, filepath.Join(root, ".evolve", "runs", "cycle-42")); !ok || c != 42 {
		t.Fatalf("canonical run workspace: got (%d,%v), want (42,true)", c, ok)
	}
	for _, ws := range []string{
		filepath.Join(root, ".evolve", "runs", "manual-release"),
		filepath.Join(root, "elsewhere", "cycle-42"),
		filepath.Join(root, ".evolve", "runs", "cycle-0"),
		filepath.Join(root, ".evolve", "runs", "cycle-x"),
	} {
		if c, ok := cycleFromRunWorkspace(root, ws); ok {
			t.Fatalf("%q must not derive a cycle, got %d", ws, c)
		}
	}
}

func TestCrossCheckActivation_AnUnsealedHostBindsNoWorktreeAndNoBase(t *testing.T) {
	f := newFixture(t)
	unsealed := f.binding()
	unsealed.Worktree, unsealed.BaseSHA = "", ""
	if err := Activate(unsealed); err != nil {
		t.Fatal(err)
	}

	provisioned := f.binding()
	if active, err := CrossCheckActivation(provisioned); !active || err != nil {
		t.Fatalf("a provisioned worktree before the Build seal must pass: active=%v err=%v", active, err)
	}
	if _, _, err := Verify(context.Background(), provisioned); err == nil || !strings.Contains(err.Error(), "not sealed") {
		t.Fatalf("ship and audit share the belt and must stay fail-closed against an unsealed host: %v", err)
	}

	if err := SealBuild(f.binding()); err != nil {
		t.Fatal(err)
	}
	cleared := f.binding()
	cleared.Worktree = ""
	if _, err := CrossCheckActivation(cleared); err == nil || !strings.Contains(err.Error(), "worktree does not match host activation") {
		t.Fatalf("a cleared worktree against a sealed host must fail naming the field: %v", err)
	}
	clearedBase := f.binding()
	clearedBase.BaseSHA = ""
	if _, err := CrossCheckActivation(clearedBase); err == nil || !strings.Contains(err.Error(), "base SHA does not match host activation") {
		t.Fatalf("a cleared base against a sealed host must fail naming the field: %v", err)
	}
}

func plantMarker(t *testing.T, root, body string) {
	t.Helper()
	if err := ensureActivationParent(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(activationPath(root, 42), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestActivationForCycle_RefusesEveryMarkerShapeThatIsNotAHostRecord(t *testing.T) {
	workspace := filepath.Join("runs", "cycle-42")
	for _, tc := range []struct {
		name    string
		plant   func(t *testing.T, root string)
		cycle   int
		wantErr string
	}{
		{name: "no project root", cycle: 42},
		{name: "no cycle", cycle: 0},
		{name: "contracts directory is a file", cycle: 42, wantErr: "inspect activation marker", plant: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "marker is a symlink", cycle: 42, wantErr: "must be a regular file", plant: func(t *testing.T, root string) {
			plantMarker(t, root, "{}")
			if err := os.Rename(activationPath(root, 42), filepath.Join(root, "real.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "real.json"), activationPath(root, 42)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "marker is not JSON", cycle: 42, wantErr: "parse activation marker", plant: func(t *testing.T, root string) {
			plantMarker(t, root, "{")
		}},
		{name: "marker names another cycle", cycle: 42, wantErr: "invalid identity or version", plant: func(t *testing.T, root string) {
			plantMarker(t, root, `{"schema_version":1,"contract_version":1,"cycle":43,"run_id":"run-42","workspace":"`+workspace+`"}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := ""
			if tc.plant != nil {
				root = t.TempDir()
				tc.plant(t, root)
			} else if tc.cycle == 0 {
				root = t.TempDir()
			}
			_, active, err := ActivationForCycle(root, tc.cycle, workspace)
			if active {
				t.Fatalf("%s must not resolve to an active host", tc.name)
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("%s: err=%v, want %q", tc.name, err, tc.wantErr)
			}
			if tc.wantErr == "" {
				return
			}
			typed := CycleBinding{ProjectRoot: root, Workspace: workspace, Cycle: tc.cycle, RunID: "run-42", ContractVersion: CurrentContractVersion}
			if _, beltErr := CrossCheckActivation(typed); beltErr == nil || !strings.Contains(beltErr.Error(), "resolve host activation") {
				t.Fatalf("%s: the belt must surface the host read failure, got %v", tc.name, beltErr)
			}
		})
	}
}

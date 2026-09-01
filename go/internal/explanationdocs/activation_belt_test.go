package explanationdocs

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCrossCheckActivation_Matrix names the belt's full decision table — the
// audit-blinding case (zero version against an ACTIVE host) is the one that
// motivated the extraction: before it, audit's gate silently no-oped.
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

	// No activation at all: legacy passes ONLY at version zero.
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

// TestCycleFromRunWorkspace pins the moved (unexported) helper's derivation contract.
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

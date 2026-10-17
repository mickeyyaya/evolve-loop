package looppreflight

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func TestResolve_DefaultDriverKnownAsksTheBridgeRegistry(t *testing.T) {
	o, err := resolve(Options{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	for cli, want := range map[string]bool{"claude-tmux": true, "agy-claude-tmux": true, "": false, "no-such-driver": false, "CLAUDE-TMUX": false} {
		if got := o.driverKnown(cli); got != want {
			t.Errorf("driverKnown(%q) = %v, want %v", cli, got, want)
		}
	}
}

func TestResolve_DefaultHostProbeHonorsTheProjectPolicyWorktreeBase(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	policyPath := filepath.Join(root, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"worktree":{"base":"`+base+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := resolve(Options{ProjectRoot: root})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	p := o.hostProbe()

	if p.Filesystem.StateDir != filepath.Join(root, ".evolve") {
		t.Errorf("StateDir = %q, want the project's .evolve", p.Filesystem.StateDir)
	}
	if p.AutoConfig.WorktreeBase != base {
		t.Errorf("WorktreeBase = %q (%s), want the policy override %q", p.AutoConfig.WorktreeBase, p.AutoConfig.WorktreeBaseReason, base)
	}
}

func TestResolve_DefaultFactoryKnownAsksThePhaseRegistry(t *testing.T) {
	t.Cleanup(registry.SnapshotForTest())
	registry.Register("preflight-probe-phase", func(core.PhaseRequest) core.PhaseRunner { return nil })
	o, err := resolve(Options{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	for name, want := range map[string]bool{"preflight-probe-phase": true, "": false, "no-such-phase": false} {
		if got := o.factoryKnown(name); got != want {
			t.Errorf("factoryKnown(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestResolve_DefaultContractKnownAsksThePhaseContractTable(t *testing.T) {
	o, err := resolve(Options{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	for name, want := range map[string]bool{"build": true, "audit": true, "": false, "no-such-phase": false} {
		if got := o.contractKnown(name); got != want {
			t.Errorf("contractKnown(%q) = %v, want %v", name, got, want)
		}
	}
}

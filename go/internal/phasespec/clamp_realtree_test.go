package phasespec

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func realTreeRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestClampDiscoveredSpecs_RealTreeKeepsEveryDeclaredWriter(t *testing.T) {
	root := realTreeRoot(t)
	specs, _ := DiscoverUserSpecs(filepath.Join(root, ".evolve", "phases"))
	profilesDir := filepath.Join(root, ".evolve", "profiles")
	clamped, warns := ClampDiscoveredSpecs(specs, SandboxedProfilePredicate(profilesDir))
	writers := 0
	for i, s := range specs {
		if !s.WritesSource {
			continue
		}
		writers++
		if !clamped[i].WritesSource {
			t.Errorf("user phase %s declares writes_source but boot strips it: its profile is not a sandboxed repo writer (warnings: %v)", s.Name, warns)
		}
	}
	if writers == 0 {
		t.Fatal("no real-tree user phase declares writes_source; the pin binds nothing")
	}
}

func TestSourceWriterProfiles_DeclareOnlyTheLaneWorktreeAndRunDir(t *testing.T) {
	loader := profiles.NewFromDir(filepath.Join(realTreeRoot(t), ".evolve", "profiles"))
	for _, name := range []string{"bug-reproduction", "test-amplification"} {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sb := p.Sandbox
		if sb == nil || !sb.Enabled || sb.ReadOnlyRepo {
			t.Fatalf("%s: sandbox must be enabled with read_only_repo false, got %+v", name, sb)
		}
		want := []string{"{worktree_path}", ".evolve/runs/cycle-*"}
		if !slices.Equal(sb.WriteSubpaths, want) {
			t.Errorf("%s: write_subpaths = %v, want exactly %v", name, sb.WriteSubpaths, want)
		}
		for _, deny := range []string{".git", ".evolve/state.json", ".evolve/ledger.jsonl", ".evolve/profiles", ".evolve/evals"} {
			if !slices.Contains(sb.DenySubpaths, deny) {
				t.Errorf("%s: deny_subpaths lacks %q", name, deny)
			}
		}
		if sb.AllowNetwork {
			t.Errorf("%s: a source-writing evaluate phase must not get network", name)
		}
	}
}

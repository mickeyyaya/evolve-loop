package paths

import (
	"path/filepath"
	"testing"
)

// TestLoopStopPath pins the brake marker's single home: the chain driver's
// `loopchain.BrakeEngaged` and the dashboard's LoopStatus.BrakeEngaged both stat
// exactly this path.
func TestLoopStopPath(t *testing.T) {
	t.Parallel()
	got := LoopStopPath(filepath.Join("root", ".evolve"))
	want := filepath.Join("root", ".evolve", LoopStopFile)
	if got != want {
		t.Fatalf("LoopStopPath = %q, want %q", got, want)
	}
	if LoopStopFile != "loop-stop" {
		t.Fatalf("LoopStopFile = %q; operators type `touch .evolve/loop-stop` — renaming breaks the documented brake", LoopStopFile)
	}
}

// TestEvolveDirOf pins the `.evolve` spelling and that Layout derives from it.
func TestEvolveDirOf(t *testing.T) {
	t.Parallel()
	if got, want := EvolveDirOf("/root"), filepath.Join("/root", ".evolve"); got != want {
		t.Fatalf("EvolveDirOf = %q, want %q", got, want)
	}
	l := Resolve(func(string) string { return "" }, "/root")
	if l.EvolveDir != EvolveDirOf("/root") {
		t.Fatalf("Layout.EvolveDir %q must derive from EvolveDirOf %q", l.EvolveDir, EvolveDirOf("/root"))
	}
}

// TestPolicyPath pins the policy file's single home: the loop's wave and
// chain engines (internal/loopwave, internal/loopchain) both resolve
// `.evolve/policy.json` through exactly this path.
func TestPolicyPath(t *testing.T) {
	t.Parallel()
	got := PolicyPath(filepath.Join("root", ".evolve"))
	want := filepath.Join("root", ".evolve", PolicyFile)
	if got != want {
		t.Fatalf("PolicyPath = %q, want %q", got, want)
	}
	if PolicyFile != "policy.json" {
		t.Fatalf("PolicyFile = %q; operators edit `.evolve/policy.json` — renaming breaks the documented layout", PolicyFile)
	}
}

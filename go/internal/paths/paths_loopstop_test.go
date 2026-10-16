package paths

import (
	"path/filepath"
	"testing"
)

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

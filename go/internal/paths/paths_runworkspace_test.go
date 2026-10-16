package paths

import (
	"path/filepath"
	"testing"
)

func TestRunWorkspace_JoinsTheCycleWorkspace(t *testing.T) {
	t.Parallel()
	if got, want := RunWorkspace("/root", 7), filepath.Join("/root", ".evolve", "runs", "cycle-7"); got != want {
		t.Fatalf("RunWorkspace = %q, want %q", got, want)
	}
	if got, want := RunWorkspace("rel", 1425), filepath.Join(EvolveDirOf("rel"), "runs", "cycle-1425"); got != want {
		t.Fatalf("RunWorkspace derives from EvolveDirOf: %q, want %q", got, want)
	}
}

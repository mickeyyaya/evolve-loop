package paths

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestCycleStateFileFor_TheOverrideGovernsOnlyItsOwnEvolveDir(t *testing.T) {
	const evolveDir = "/work/repo/.evolve"
	const own = evolveDir + "/cycle-state.json"
	cases := []struct {
		name, override, want string
	}{
		{"no override", "", own},
		{"a lane file inside", evolveDir + "/runs/cycle-9/cycle-state.json", evolveDir + "/runs/cycle-9/cycle-state.json"},
		{"another tree's lane file", "/other/.evolve/runs/cycle-9/cycle-state.json", own},
		{"a sibling sharing the prefix", "/work/repo/.evolve-old/cycle-state.json", own},
		{"the evolve dir itself", evolveDir, own},
		{"an escape through ..", evolveDir + "/../elsewhere/cycle-state.json", own},
		{"a relative override", ".evolve/runs/cycle-9/cycle-state.json", own},
	}
	for _, tc := range cases {
		if got := CycleStateFileFor(evolveDir, tc.override); got != tc.want {
			t.Errorf("%s: CycleStateFileFor(%q, %q) = %q, want %q", tc.name, evolveDir, tc.override, got, tc.want)
		}
	}
}

func TestResolve_CycleStateOverrideAppliesOnlyInsideItsEvolveDir(t *testing.T) {
	inside := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT":    "/work/repo",
		ipcenv.CycleStateFileKey: "/work/repo/.evolve/runs/cycle-9/cycle-state.json",
	}), "/tmp/cwd")
	if inside.CycleStateFile != "/work/repo/.evolve/runs/cycle-9/cycle-state.json" {
		t.Errorf("lane override inside the evolve dir: CycleStateFile=%q, want the lane's file", inside.CycleStateFile)
	}

	outside := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT":    "/tmp/fixture",
		ipcenv.CycleStateFileKey: "/work/repo/.evolve/runs/cycle-9/cycle-state.json",
	}), "/tmp/cwd")
	if outside.CycleStateFile != "/tmp/fixture/.evolve/cycle-state.json" {
		t.Errorf("override from another tree: CycleStateFile=%q, want /tmp/fixture/.evolve/cycle-state.json", outside.CycleStateFile)
	}
}

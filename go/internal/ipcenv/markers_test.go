package ipcenv_test

import (
	"path"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var classAMarkers = []string{
	"EVOLVE_PROJECT_ROOT", "EVOLVE_PLUGIN_ROOT",
	"EVOLVE_DISPATCH_ID", "EVOLVE_DISPATCH_DEPTH", "EVOLVE_MSG_TOKEN", "EVOLVE_PHASE",
	"EVOLVE_FLEET", "EVOLVE_FLEET_SCOPE", "EVOLVE_FLEET_WIDTH", "EVOLVE_FLEET_REACH_HOLDER",
	"EVOLVE_WORKTREE_ROOT", "EVOLVE_CYCLE_STATE_FILE", "EVOLVE_TMUX_SOCKET",
	"EVOLVE_FANOUT_WORKER_TOKEN", "EVOLVE_FANOUT_WORKER_NAME", "EVOLVE_FANOUT_CYCLE", "EVOLVE_FANOUT_WORKSPACE",
	"EVOLVE_FANOUT_PARENT_AGENT", "EVOLVE_FANOUT_WORKER_ARTIFACT", "EVOLVE_FANOUT_CACHE_PREFIX_FILE",
	"EVOLVE_BRIDGE_PIDFILE", "EVOLVE_PHASE_RECOVERY_STAGE", "EVOLVE_SHIP_RELEASE_NOTES", "EVOLVE_CLI",
	"BRIDGE_CLI", "BRIDGE_RUN_ID", "BRIDGE_COMPLETION", "BRIDGE_PERMISSION_MODE", "BRIDGE_SESSION_NAME",
	"BRIDGE_STREAM_OUTPUT", "BRIDGE_REQUIRE_FULL", "BRIDGE_REQUIRE_SANDBOX", "BRIDGE_HUMAN_INPUT_FLAG",
}

func TestMarkers_AreClassAOfTheControlFlagInventory(t *testing.T) {
	t.Parallel()
	got := slices.Sorted(slices.Values(ipcenv.Markers()))
	want := slices.Sorted(slices.Values(classAMarkers))
	if !slices.Equal(got, want) {
		t.Fatalf("Markers() = %v\nwant class (a) of control-flags.md = %v", got, want)
	}
	for _, key := range ipcenv.ProtocolKeys() {
		if !slices.Contains(got, key) {
			t.Errorf("protocol key %s is not a pipeline marker", key)
		}
	}
}

func TestCellMarkers_AreTheMarkersThatReachAnLLMCell(t *testing.T) {
	t.Parallel()
	want := []string{
		"EVOLVE_PROJECT_ROOT", ipcenv.WorktreeRootKey, ipcenv.CycleStateFileKey, ipcenv.TmuxSocketKey,
		ipcenv.FleetKey, ipcenv.FleetScopeKey, ipcenv.FleetWidthKey, ipcenv.FleetReachHolderKey,
		ipcenv.DispatchDepthKey, ipcenv.PhaseKey,
	}
	got := ipcenv.CellMarkers()
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
		t.Fatalf("CellMarkers() = %v, want %v", got, want)
	}
	for _, name := range []string{ipcenv.MsgTokenKey, ipcenv.DispatchIDKey, "BRIDGE_CLI", "EVOLVE_CLI"} {
		if slices.Contains(got, name) {
			t.Errorf("%s reaches an LLM cell; a token, a dispatch tag or a launch argument must not", name)
		}
	}
}

func TestCarried_IsAMarkerOrAControlNotYetMoved(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"EVOLVE_PROJECT_ROOT":              true,
		"BRIDGE_RUN_ID":                    true,
		"EVOLVE_ROUTING_MODE":              false,
		"EVOLVE_AUDITOR_CLI":               false,
		"EVOLVE_TDD_ENGINEER_MODEL":        false,
		"EVOLVE_CLI_MAX_CONCURRENT_CLAUDE": false,
		"EVOLVE_SANDBOX":                   true,
		"BRIDGE_ALLOW_BYPASS":              true,
		"EVOLVE_BYPASS_SHIP_GATE":          false,
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM": false,
		"EVOLVE_REQUIRE_INTENT":            false,
		"BRIDGE_TESTING":                   false,
		"PATH":                             false,
		"":                                 false,
	} {
		if got := ipcenv.Carried(name); got != want {
			t.Errorf("Carried(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestUnmovedControls_AreWellFormedAndMatchNoMarker(t *testing.T) {
	t.Parallel()
	for _, pattern := range ipcenv.UnmovedControls() {
		for _, name := range ipcenv.Markers() {
			matched, err := path.Match(pattern, name)
			if err != nil {
				t.Fatalf("pattern %s is malformed: %v", pattern, err)
			}
			if matched {
				t.Errorf("%s matches the pipeline marker %s", pattern, name)
			}
		}
	}
}

func TestMarkerLists_ReturnFreshSlices(t *testing.T) {
	t.Parallel()
	for name, list := range map[string]func() []string{"Markers": ipcenv.Markers, "CellMarkers": ipcenv.CellMarkers, "UnmovedControls": ipcenv.UnmovedControls} {
		first := list()
		first[0] = "MUTATED"
		if list()[0] == "MUTATED" {
			t.Errorf("%s handed out its backing array", name)
		}
	}
}

func TestRetiredInUse_NamesEachRetiredControlThatIsSetOnceWithItsReplacement(t *testing.T) {
	t.Parallel()
	got := ipcenv.RetiredInUse([]string{
		"EVOLVE_ROUTING_MODE=static", "EVOLVE_AUDITOR_CLI=claude-tmux", "EVOLVE_SCOUT_CLI=agy-tmux",
		"EVOLVE_LANE=", "EVOLVE_PROJECT_ROOT=/r", "EVOLVE_SANDBOX=off", "PATH=/bin", "EVOLVE_GO_BIN",
	})
	var names []string
	for _, r := range got {
		names = append(names, r.Name)
		if r.Use == "" {
			t.Errorf("%s names no replacement", r.Name)
		}
	}
	if want := []string{"EVOLVE_ROUTING_MODE", "EVOLVE_*_CLI"}; !slices.Equal(names, want) {
		t.Errorf("RetiredInUse = %v, want %v: a set name warns once, an empty or valueless one does not", names, want)
	}
}

func TestRetiredControls_AreNoLongerCarriedAndMatchNoMarker(t *testing.T) {
	t.Parallel()
	retired := ipcenv.RetiredControls()
	if len(retired) == 0 {
		t.Fatal("RetiredControls is empty")
	}
	for _, r := range retired {
		if slices.Contains(ipcenv.UnmovedControls(), r.Name) {
			t.Errorf("%s is both retired and not yet moved", r.Name)
		}
		for _, name := range ipcenv.Markers() {
			if matched, _ := path.Match(r.Name, name); matched {
				t.Errorf("the retired %s matches the pipeline marker %s", r.Name, name)
			}
		}
	}
	retired[0].Name = "MUTATED"
	if ipcenv.RetiredControls()[0].Name == "MUTATED" {
		t.Error("RetiredControls handed out its backing array")
	}
}

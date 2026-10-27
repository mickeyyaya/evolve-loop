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
	"BRIDGE_STREAM_OUTPUT", "BRIDGE_REQUIRE_FULL", "BRIDGE_REQUIRE_SANDBOX",
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

func TestCarried_IsOnlyAMarker(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"EVOLVE_PROJECT_ROOT":              true,
		"BRIDGE_RUN_ID":                    true,
		"EVOLVE_ROUTING_MODE":              false,
		"EVOLVE_AUDITOR_CLI":               false,
		"EVOLVE_TDD_ENGINEER_MODEL":        false,
		"EVOLVE_CLI_MAX_CONCURRENT_CLAUDE": false,
		"EVOLVE_SANDBOX":                   false,
		"BRIDGE_ALLOW_BYPASS":              false,
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

func TestMarkerLists_ReturnFreshSlices(t *testing.T) {
	t.Parallel()
	for name, list := range map[string]func() []string{"Markers": ipcenv.Markers, "CellMarkers": ipcenv.CellMarkers} {
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
	want := []ipcenv.RetiredControl{
		{Name: "EVOLVE_ROUTING_MODE", Use: "policy.json router.routing_mode"},
		{Name: "EVOLVE_*_CLI", Use: "the --cli <agent>=<driver> parameter or policy.json pins.<phase>.cli"},
		{Name: "EVOLVE_SANDBOX", Use: "policy.json sandbox.mode, or the audited --sandbox-off=<reason> parameter of evolve loop and evolve cycle run"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("RetiredInUse = %v, want %v: a set name warns once with its replacement, an empty or valueless one does not", got, want)
	}
}

func TestRetiredControls_AreNoLongerCarriedAndMatchNoMarker(t *testing.T) {
	t.Parallel()
	retired := ipcenv.RetiredControls()
	if len(retired) == 0 {
		t.Fatal("RetiredControls is empty")
	}
	for _, r := range retired {
		if ipcenv.Carried(r.Name) {
			t.Errorf("the retired %s is still carried", r.Name)
		}
		for _, name := range ipcenv.Markers() {
			matched, err := path.Match(r.Name, name)
			if err != nil {
				t.Fatalf("pattern %s is malformed: %v", r.Name, err)
			}
			if matched {
				t.Errorf("the retired %s matches the pipeline marker %s", r.Name, name)
			}
		}
	}
	retired[0].Name = "MUTATED"
	if ipcenv.RetiredControls()[0].Name == "MUTATED" {
		t.Error("RetiredControls handed out its backing array")
	}
}

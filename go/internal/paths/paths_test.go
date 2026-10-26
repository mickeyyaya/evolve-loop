package paths

import (
	"path/filepath"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolve_DefaultLayout(t *testing.T) {
	cwd := "/tmp/proj"
	got := Resolve(envMap(nil), cwd)

	want := Layout{
		ProjectRoot:    "/tmp/proj",
		PluginRoot:     "/tmp/proj",
		EvolveDir:      "/tmp/proj/.evolve",
		StateFile:      "/tmp/proj/.evolve/state.json",
		CycleStateFile: "/tmp/proj/.evolve/cycle-state.json",
		LedgerFile:     "/tmp/proj/.evolve/ledger.jsonl",
		ProfilesDir:    "/tmp/proj/.evolve/profiles",
		AdaptersDir:    "/tmp/proj/adapters",
		CapabilityDir:  "/tmp/proj/adapters",
	}
	if got != want {
		t.Errorf("Resolve default = %+v\n want %+v", got, want)
	}
}

func TestResolve_ProjectRootOverride(t *testing.T) {
	got := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT": "/work/repo",
	}), "/tmp/cwd")

	if got.ProjectRoot != "/work/repo" {
		t.Errorf("ProjectRoot=%q, want /work/repo", got.ProjectRoot)
	}
	if got.PluginRoot != "/work/repo" {
		t.Errorf("PluginRoot=%q, want /work/repo (mirror of ProjectRoot when EVOLVE_PLUGIN_ROOT unset)", got.PluginRoot)
	}
	if got.EvolveDir != "/work/repo/.evolve" {
		t.Errorf("EvolveDir=%q, want /work/repo/.evolve", got.EvolveDir)
	}
	if got.AdaptersDir != "/work/repo/adapters" {
		t.Errorf("AdaptersDir=%q, want /work/repo/adapters", got.AdaptersDir)
	}
}

func TestResolve_SplitPluginRoot(t *testing.T) {
	got := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT": "/work/repo",
		"EVOLVE_PLUGIN_ROOT":  "/opt/plugin",
	}), "")

	if got.ProjectRoot != "/work/repo" {
		t.Errorf("ProjectRoot=%q, want /work/repo", got.ProjectRoot)
	}
	if got.PluginRoot != "/opt/plugin" {
		t.Errorf("PluginRoot=%q, want /opt/plugin", got.PluginRoot)
	}
	if got.StateFile != "/work/repo/.evolve/state.json" {
		t.Errorf("StateFile=%q, want under ProjectRoot", got.StateFile)
	}
	if got.LedgerFile != "/work/repo/.evolve/ledger.jsonl" {
		t.Errorf("LedgerFile=%q, want under ProjectRoot", got.LedgerFile)
	}
	if got.ProfilesDir != "/opt/plugin/.evolve/profiles" {
		t.Errorf("ProfilesDir=%q, want under PluginRoot", got.ProfilesDir)
	}
	if got.AdaptersDir != "/opt/plugin/adapters" {
		t.Errorf("AdaptersDir=%q, want under PluginRoot", got.AdaptersDir)
	}
	if got.CapabilityDir != "/opt/plugin/adapters" {
		t.Errorf("CapabilityDir=%q, want under PluginRoot", got.CapabilityDir)
	}
}

func TestResolve_TheEnvironmentOverridesNoDir(t *testing.T) {
	got := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT":          "/repo",
		"EVOLVE_PLUGIN_ROOT":           "/opt/plugin",
		"EVOLVE_PROFILES_DIR_OVERRIDE": "/custom/profiles",
		"EVOLVE_ADAPTERS_DIR_OVERRIDE": "/tmp/fake-adapters",
		"EVOLVE_LEDGER_OVERRIDE":       "/var/log/ledger.jsonl",
	}), "")

	want := map[string][2]string{
		"ProfilesDir":   {got.ProfilesDir, "/opt/plugin/.evolve/profiles"},
		"AdaptersDir":   {got.AdaptersDir, "/opt/plugin/adapters"},
		"CapabilityDir": {got.CapabilityDir, "/opt/plugin/adapters"},
		"LedgerFile":    {got.LedgerFile, "/repo/.evolve/ledger.jsonl"},
	}
	for field, pair := range want {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q: a dir comes from the roots, never from an override variable (I-16, X18e)", field, pair[0], pair[1])
		}
	}
}

func TestResolve_EmptyEnvAndCwd(t *testing.T) {
	got := Resolve(envMap(nil), "")

	if got.ProjectRoot != "" {
		t.Errorf("ProjectRoot=%q, want empty when no env and no cwd", got.ProjectRoot)
	}
	if got.EvolveDir != ".evolve" {
		t.Errorf("EvolveDir=%q, want .evolve", got.EvolveDir)
	}
	if got.StateFile != filepath.Join(".evolve", "state.json") {
		t.Errorf("StateFile=%q, want .evolve/state.json", got.StateFile)
	}
}

func TestResolve_NilLookupPanicsNot(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Resolve(nil, cwd) panicked: %v", r)
		}
	}()
	got := Resolve(nil, "/proj")
	if got.ProjectRoot != "/proj" {
		t.Errorf("ProjectRoot=%q, want /proj (cwd fallback when lookup is nil)", got.ProjectRoot)
	}
}

func TestResolveFromEnv_IntegratesWithOsGetenv(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from/env")
	t.Setenv("EVOLVE_PLUGIN_ROOT", "/plugin/env")
	got := ResolveFromEnv()
	if got.ProjectRoot != "/from/env" {
		t.Errorf("ProjectRoot=%q, want /from/env", got.ProjectRoot)
	}
	if got.PluginRoot != "/plugin/env" {
		t.Errorf("PluginRoot=%q, want /plugin/env", got.PluginRoot)
	}
}

func TestEventChannelsRoot_IsTheChDirectoryUnderTheEventsDirectoryOfTheEvolveDir(t *testing.T) {
	t.Parallel()

	got := EventChannelsRoot(filepath.Join("proj", ".evolve"))

	if want := filepath.Join("proj", ".evolve", "events", "ch"); got != want {
		t.Errorf("EventChannelsRoot = %q, want %q: the Publisher, the reader and the gc category must name the same directory (spec §4)", got, want)
	}
}

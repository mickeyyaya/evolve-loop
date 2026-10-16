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

func TestResolve_DirOverrides(t *testing.T) {
	cases := []struct {
		name   string
		envKey string
		envVal string
		field  func(Layout) string
		want   string
	}{
		{
			name:   "profiles_override",
			envKey: "EVOLVE_PROFILES_DIR_OVERRIDE",
			envVal: "/custom/profiles",
			field:  func(l Layout) string { return l.ProfilesDir },
			want:   "/custom/profiles",
		},
		{
			name:   "adapters_override",
			envKey: "EVOLVE_ADAPTERS_DIR_OVERRIDE",
			envVal: "/custom/adapters",
			field:  func(l Layout) string { return l.AdaptersDir },
			want:   "/custom/adapters",
		},
		{
			name:   "ledger_override",
			envKey: "EVOLVE_LEDGER_OVERRIDE",
			envVal: "/var/log/ledger.jsonl",
			field:  func(l Layout) string { return l.LedgerFile },
			want:   "/var/log/ledger.jsonl",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{
				"EVOLVE_PROJECT_ROOT": "/repo",
				tc.envKey:             tc.envVal,
			}
			got := tc.field(Resolve(envMap(env), ""))
			if got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestResolve_CapabilityDirAlwaysPluginRoot(t *testing.T) {
	got := Resolve(envMap(map[string]string{
		"EVOLVE_PROJECT_ROOT":          "/repo",
		"EVOLVE_PLUGIN_ROOT":           "/opt/plugin",
		"EVOLVE_ADAPTERS_DIR_OVERRIDE": "/tmp/fake-adapters",
	}), "")

	if got.AdaptersDir != "/tmp/fake-adapters" {
		t.Errorf("AdaptersDir=%q, want override /tmp/fake-adapters", got.AdaptersDir)
	}
	if got.CapabilityDir != "/opt/plugin/adapters" {
		t.Errorf("CapabilityDir=%q, want PluginRoot/adapters (NEVER overridden)", got.CapabilityDir)
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
